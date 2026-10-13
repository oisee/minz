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
	if f != nil && f.Attrs.FixedIndirectABI {
		return nil, fmt.Errorf("checked local table does not model the canonical indirect entry bridge")
	}
	if f == nil || allocation == nil || len(f.Blocks) != 1 || f.Attrs.IsExtern || len(f.Contract.Returns) != 1 {
		return nil, fmt.Errorf("local table requires single-result leaf block")
	}
	block := f.Blocks[0]
	if block == nil {
		return nil, fmt.Errorf("nil entry block")
	}
	parameters := map[mir2.Reg]mir2.Param{}
	for _, p := range f.Contract.Params {
		if _, duplicate := parameters[p.Reg]; p.Reg <= 0 || duplicate || !localIntegerType(p.Ty) {
			return nil, fmt.Errorf("invalid or duplicate contract parameter")
		}
		parameters[p.Reg] = p
	}
	hasWordExpansion := false
	constants := map[mir2.Reg]int64{}
	defined := map[mir2.Reg]mir2.Ty{}
	entryParams := map[mir2.Reg]bool{}
	for _, p := range block.Params {
		contract, exists := parameters[p.Dst]
		if !exists || entryParams[p.Dst] || p.Ty != contract.Ty || p.Class != contract.Class {
			return nil, fmt.Errorf("duplicate or mismatched entry block parameter")
		}
		entryParams[p.Dst] = true
	}
	for r, p := range parameters {
		defined[r] = p.Ty
	}
	for _, inst := range block.Insts {
		if inst == nil || inst.Dst <= 0 || inst.ClsHard || !localIntegerType(inst.Ty) {
			return nil, fmt.Errorf("hard class or invalid local definition unsupported")
		}
		arity := 0
		switch inst.Op {
		case mir2.OpConst:
		case mir2.OpMove:
			arity = 1
		case mir2.OpShl:
			arity = 2
			if inst.Ty.Width() != 16 {
				return nil, fmt.Errorf("local constant shift requires word width")
			}
		case mir2.OpMul:
			arity = 2
			if inst.Ty.Width() != 16 {
				return nil, fmt.Errorf("local constant multiply requires word width")
			}
		case mir2.OpAdd, mir2.OpSub, mir2.OpAnd, mir2.OpOr, mir2.OpXor:
			arity = 2
		default:
			return nil, fmt.Errorf("unsupported local opcode %s", inst.Op)
		}
		for i, r := range inst.Src {
			if i >= arity {
				if r != 0 {
					return nil, fmt.Errorf("unexpected local operand")
				}
			} else if r <= 0 || defined[r] != inst.Ty {
				return nil, fmt.Errorf("missing, unbound or mistyped local input vreg %d", r)
			}
		}
		if _, exists := defined[inst.Dst]; exists {
			return nil, fmt.Errorf("redefined local vreg %d", inst.Dst)
		}
		if inst.Op == mir2.OpMul {
			hasWordExpansion = true
			k, ok := constants[inst.Src[1]]
			if !ok || k < 2 || k > 255 {
				return nil, fmt.Errorf("local word multiply requires RHS constant2..255")
			}
		}
		if inst.Op == mir2.OpShl {
			hasWordExpansion = true
			amount, ok := constants[inst.Src[1]]
			if !ok || amount < 1 || amount > 15 {
				return nil, fmt.Errorf("local word shift requires RHS constant1..15")
			}
		}
		if inst.Op == mir2.OpConst {
			constants[inst.Dst] = inst.Imm
		}
		defined[inst.Dst] = inst.Ty
	}

	term, ok := block.Term.(*mir2.TermRet)
	if !ok || len(term.Vals) != 1 {
		return nil, fmt.Errorf("local table requires unconditional single return")
	}
	ret := f.Contract.Returns[0]
	if !localIntegerType(ret.Ty) || ret.Class == mir2.ClassFlag {
		return nil, fmt.Errorf("unsupported return ABI")
	}
	for _, p := range block.Params {
		if p.Dst == term.Vals[0] {
			return nil, fmt.Errorf("return block parameter not yet certified")
		}
	}
	if defined[term.Vals[0]] != ret.Ty {
		return nil, fmt.Errorf("unbound or mistyped return value")
	}
	returnName := mir2.ReturnLocation(ret.Class, ret.Ty)
	returnLoc := Z80.LocByName(returnName)
	if returnLoc < 0 || Z80.Locs[returnLoc].Width != ret.Ty.Width() {
		return nil, fmt.Errorf("unsupported return location")
	}
	opts := SolverOptions{OptSize: true, CheckedReturnLocation: returnName, CheckedRawEmission: true, ParamLocs: map[int]int{}}
	for _, p := range f.Contract.Params {
		loc, exists := allocation.Locs[p.Reg]
		if !exists {
			return nil, fmt.Errorf("missing parameter allocation")
		}
		index := Z80.LocByName(loc.Name)
		if index < 0 || !localPhysicalRegister(loc) || (mir2.Z80CostTable{}).Cost(p.Class, loc) >= mir2.InfCost || Z80.Locs[index].Width != p.Ty.Width() || Z80.NonAllocatable.Has(index) {
			return nil, fmt.Errorf("unsupported parameter ABI")
		}
		opts.ParamLocs[int(p.Reg)] = index
	}
	var ops []VIROp
	maxReg := 0
	if hasWordExpansion {
		for r := range defined {
			if int(r) > maxReg {
				maxReg = int(r)
			}
		}
	}
	for _, inst := range block.Insts {
		copyInst := *inst
		var lowered []VIROp
		var err error
		if inst.Op == mir2.OpShl {
			// Exact modulo-word doubling chain; no implicit repeat-count pattern.
			maxReg++
			acc := maxReg
			lowered = append(lowered, VIROp{Op: OpMove, Dst: acc, Src: [2]int{int(inst.Src[0]), -1}, Width: 16, DstHint: Z80.LocSetByNames("HL")})
			for count := int64(0); count < constants[inst.Src[1]]; count++ {
				maxReg++
				lowered = append(lowered, VIROp{Op: OpAdd, Dst: maxReg, Src: [2]int{acc, acc}, Width: 16, DstHint: Z80.LocSetByNames("HL")})
				acc = maxReg
			}
			lowered = append(lowered, VIROp{Op: OpMove, Dst: int(inst.Dst), Src: [2]int{acc, -1}, Width: 16, DstHint: Z80.LocSetByNames("HL")})
		} else if inst.Op == mir2.OpMul {
			// Binary Horner chain. Original MIR remains bound; every shift/add is
			// explicit, so scratch/liveness/ABI is independently admitted and costed.
			k := constants[inst.Src[1]]
			x := int(inst.Src[0])
			acc := x
			top := int64(128)
			for top > k {
				top >>= 1
			}
			for bit := top >> 1; bit > 0; bit >>= 1 {
				maxReg++
				dst := maxReg
				lowered = append(lowered, VIROp{Op: OpAdd, Dst: dst, Src: [2]int{acc, acc}, Width: 16, DstHint: Z80.LocSetByNames("HL")})
				acc = dst
				if k&bit != 0 {
					maxReg++
					dst = maxReg
					lowered = append(lowered, VIROp{Op: OpAdd, Dst: dst, Src: [2]int{acc, x}, Width: 16, DstHint: Z80.LocSetByNames("HL")})
					acc = dst
				}
			}
			lowered = append(lowered, VIROp{Op: OpMove, Dst: int(inst.Dst), Src: [2]int{acc, -1}, Width: 16, DstHint: Z80.LocSetByNames("HL")})
		} else {
			lowered, err = translateInst(&copyInst, Z80, nil)
		}
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
	beforeSaves := maxReg + 1
	ops = insertSaveMoves(append([]VIROp(nil), ops...), Z80)
	for i := range ops {
		if hasWordExpansion && ops[i].Op == OpMove && ops[i].Width == 16 && ops[i].Dst > beforeSaves {
			// This save must copy the live HL value, not exchange and destroy it.
			ops[i].DstHint = Z80.LocSetByNames("BC")
		}
	}
	context, err := localFunctionContext(f)
	if err != nil {
		return nil, err
	}
	opts.CheckedContext = context
	return &LocalTableProblem{Ops: ops, Options: opts, Function: f}, nil
}

// Emit emits raw direct instructions. No solver, caller adapters, unchecked
// peepholes, runtime helpers or extra module storage are introduced.
func (p *LocalTableProblem) Emit(table *RegAllocTable) (string, error) {
	if p == nil || table == nil || p.Function == nil {
		return "", fmt.Errorf("nil local table problem")
	}
	context, err := localFunctionContext(p.Function)
	if err != nil || context != p.Options.CheckedContext {
		return "", fmt.Errorf("changed local function binding")
	}
	assignment, _, ok := table.LookupChecked(p.Ops, Z80, p.Options)
	if !ok {
		return "", fmt.Errorf("checked local table miss")
	}
	vf := &Func{Name: p.Function.Name, Blocks: []Block{{Ops: p.Ops}}}
	// OpRet is already part of the admitted ops; no outside terminator is emitted.
	return emitFromTable(&mir2.Func{Name: p.Function.Name}, vf, p.Ops, assignment, Z80, p.Options)
}

// Width alone is not a semantic type proof (e.g. bool, BCD and fixed point).
func localIntegerType(ty mir2.Ty) bool {
	return ty == mir2.TyU8 || ty == mir2.TyI8 || ty == mir2.TyU16 || ty == mir2.TyI16
}

func localPhysicalRegister(loc mir2.PhysLoc) bool {
	if loc.Offset != 0 {
		return false
	}
	switch loc.Name {
	case "IX", "IY":
		return loc.Kind == mir2.LocIXY
	case "IXH", "IXL", "IYH", "IYL":
		return loc.Kind == mir2.LocIXY8
	default:
		return loc.Kind == mir2.LocReg
	}
}

func localFunctionContext(f *mir2.Func) (string, error) {
	if f == nil || len(f.Blocks) != 1 || f.Blocks[0] == nil || f.Blocks[0].Term == nil {
		return "", fmt.Errorf("invalid local function binding")
	}
	for _, inst := range f.Blocks[0].Insts {
		if inst == nil {
			return "", fmt.Errorf("nil local instruction binding")
		}
	}
	contract, err := json.Marshal(f.Contract)
	if err != nil {
		return "", err
	}
	binding := (&mir2.Module{Funcs: []*mir2.Func{f}}).Dump() + string(contract)
	return fmt.Sprintf("production-local-v4:%x", sha256.Sum256([]byte(binding))), nil
}
