package vir

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// LocalTableProblem is the conservative bridge from the production MIR ABI to
// local tables. Preparation copies instructions and never mutates production IR.
// Its binding includes the whole MIR function (contract and return terminator).
type LocalTableProblem struct {
	Ops      []VIROp
	Options  SolverOptions
	Function *mir2.Func
}

func PrepareLocalTableProblem(f *mir2.Func, allocation *mir2.AllocResult) (*LocalTableProblem, error) {
	if f == nil || allocation == nil || len(f.Blocks) != 1 || f.Attrs.IsExtern || len(f.Contract.Returns) != 1 {
		return nil, fmt.Errorf("local table requires single-result leaf block")
	}
	block := f.Blocks[0]
	term, ok := block.Term.(*mir2.TermRet)
	if !ok || len(term.Vals) != 1 {
		return nil, fmt.Errorf("local table requires unconditional single return")
	}
	ret := f.Contract.Returns[0]
	if ret.Ty == nil || ret.Class == mir2.ClassFlag || ret.Ty.Width() != 8 && ret.Ty.Width() != 16 {
		return nil, fmt.Errorf("unsupported return ABI")
	}
	for _, p := range block.Params {
		if p.Dst == term.Vals[0] {
			return nil, fmt.Errorf("return block parameter not yet certified")
		}
	}
	returnName := mir2.ReturnLocation(ret.Class, ret.Ty)
	returnLoc := Z80.LocByName(returnName)
	if returnLoc < 0 {
		return nil, fmt.Errorf("unsupported return location")
	}
	opts := SolverOptions{OptSize: true, CheckedReturnLocation: returnName, CheckedRawEmission: true, ParamLocs: map[int]int{}}
	for _, p := range f.Contract.Params {
		loc, exists := allocation.Locs[p.Reg]
		if !exists {
			return nil, fmt.Errorf("missing parameter allocation")
		}
		index := Z80.LocByName(loc.Name)
		if index < 0 || p.Ty == nil || Z80.Locs[index].Width != p.Ty.Width() {
			return nil, fmt.Errorf("unsupported parameter ABI")
		}
		opts.ParamLocs[int(p.Reg)] = index
	}
	var ops []VIROp
	maxReg := 0
	for _, inst := range block.Insts {
		if inst.Ty == nil || inst.Ty.Width() != 8 && inst.Ty.Width() != 16 {
			return nil, fmt.Errorf("unsupported instruction type")
		}
		// A deliberately small independently executable domain. Calls, effects,
		// casts, comparisons/flags and CFG stay in the production backend.
		switch inst.Op {
		case mir2.OpConst, mir2.OpMove, mir2.OpAdd, mir2.OpSub, mir2.OpAnd, mir2.OpOr, mir2.OpXor:
		default:
			return nil, fmt.Errorf("unsupported local opcode %s", inst.Op)
		}
		copyInst := *inst
		lowered, err := translateInst(&copyInst, Z80, nil)
		if err != nil {
			return nil, err
		}
		ops = append(ops, lowered...)
		if int(inst.Dst) > maxReg {
			maxReg = int(inst.Dst)
		}
		for _, r := range inst.Src {
			if int(r) > maxReg {
				maxReg = int(r)
			}
		}
	}
	// Pin only the explicit return move, irrespective of ALU or param guesses.
	abiReg := maxReg + 1
	ops = append(ops, VIROp{Op: OpMove, Dst: abiReg, Src: [2]int{int(term.Vals[0]), -1}, Width: ret.Ty.Width(), DstHint: Singleton(returnLoc)})
	ops = append(ops, VIROp{Op: OpRet, Src: [2]int{abiReg, -1}, Width: ret.Ty.Width()})
	for v := range opts.ParamLocs {
		used := false
		for _, op := range ops {
			for _, src := range op.Src {
				if src == v {
					used = true
				}
			}
		}
		if !used {
			delete(opts.ParamLocs, v)
		}
	}
	ops = foldConstIntoALU(ops)
	ops = EliminateDeadConsts(ops)
	// Preserve values that survive a destructive tied accumulator operation.
	// Saves are explicit problem operations and are included in admission/cost.
	ops = insertSaveMoves(append([]VIROp(nil), ops...), Z80)
	contract, err := json.Marshal(f.Contract)
	if err != nil {
		return nil, err
	}
	binding := (&mir2.Module{Funcs: []*mir2.Func{f}}).Dump() + string(contract)
	opts.CheckedContext = fmt.Sprintf("production-local-v2:%x", sha256.Sum256([]byte(binding)))
	return &LocalTableProblem{Ops: ops, Options: opts, Function: f}, nil
}

// Emit emits raw direct instructions. No solver, caller adapters, unchecked
// peepholes, runtime helpers or extra module storage are introduced.
func (p *LocalTableProblem) Emit(table *RegAllocTable) (string, error) {
	if p == nil || table == nil {
		return "", fmt.Errorf("nil local table problem")
	}
	assignment, _, ok := table.LookupChecked(p.Ops, Z80, p.Options)
	if !ok {
		return "", fmt.Errorf("checked local table miss")
	}
	vf := &Func{Name: p.Function.Name, Blocks: []Block{{Ops: p.Ops}}}
	// OpRet is already part of the admitted ops; no outside terminator is emitted.
	return emitFromTable(&mir2.Func{Name: p.Function.Name}, vf, p.Ops, assignment, Z80, p.Options)
}
