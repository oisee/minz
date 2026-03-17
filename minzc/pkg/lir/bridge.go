// bridge.go — MIR2 → LIR conversion.
//
// Converts a MIR2 function into a sequence of MIROps (the LIR input format),
// then runs isel + WFC to produce allocated LIR instructions.
// This is the bridge between the existing compiler and the new LIR backend.
//
// Usage:
//
//	mir2Func := ... // from HIR lowering
//	lirResult, err := lir.LowerMIR2(mir2Func, lir.CISC)
//	// lirResult.Insts are allocated LIR instructions
//	// Execute on LIR-VM and compare with MIR2-VM for convergence
package lir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// LowerResult holds the output of MIR2→LIR lowering.
type LowerResult struct {
	Ops   []MIROp // translated MIR2 ops
	Insts []Inst  // allocated LIR instructions (after isel + WFC)
}

// LowerMIR2Block converts one MIR2 basic block into LIR MIROps.
// This is a straightforward 1:1 translation — isel handles the rest.
func LowerMIR2Block(b *mir2.Block, desc *MachineDesc) ([]MIROp, error) {
	var ops []MIROp

	for _, inst := range b.Insts {
		op, err := translateInst(inst, desc)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", b.Label, err)
		}
		if op != nil {
			ops = append(ops, *op)
		}
	}

	return ops, nil
}

// LowerMIR2Func converts a full MIR2 function (all blocks, straight-line)
// into LIR and runs isel + WFC allocation.
func LowerMIR2Func(f *mir2.Func, desc *MachineDesc) (*LowerResult, error) {
	var allOps []MIROp

	for _, b := range f.Blocks {
		ops, err := LowerMIR2Block(b, desc)
		if err != nil {
			return nil, err
		}
		allOps = append(allOps, ops...)
	}

	if len(allOps) == 0 {
		return &LowerResult{}, nil
	}

	// Instruction selection
	sel, err := SelectInstructions(desc, allOps)
	if err != nil {
		return nil, fmt.Errorf("isel: %w", err)
	}

	// WFC constraint propagation + collapse
	wfc := NewWFCState(desc, sel.Insts)
	wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		return nil, fmt.Errorf("wfc: %w", err)
	}

	return &LowerResult{
		Ops:   allOps,
		Insts: wfc.ToInsts(),
	}, nil
}

// translateInst converts one MIR2 instruction to a LIR MIROp.
func translateInst(inst *mir2.Inst, desc *MachineDesc) (*MIROp, error) {
	if inst.Dst == mir2.NoReg && inst.Op != mir2.OpStore {
		return nil, nil // skip side-effect-free instructions with no result
	}

	width := 8
	if inst.Ty != nil {
		w := inst.Ty.Width()
		if w > 0 {
			width = w
		}
	}

	op := &MIROp{
		Dst:   int(inst.Dst),
		Src:   [2]int{int(inst.Src[0]), int(inst.Src[1])},
		Imm:   inst.Imm,
		Width: width,
	}

	// Fix NoReg → -1
	if inst.Dst == mir2.NoReg {
		op.Dst = -1
	}
	if inst.Src[0] == mir2.NoReg {
		op.Src[0] = -1
	}
	if inst.Src[1] == mir2.NoReg {
		op.Src[1] = -1
	}

	switch inst.Op {
	case mir2.OpConst:
		op.Op = OpConst
	case mir2.OpMove:
		op.Op = OpMove
	case mir2.OpAdd:
		op.Op = OpAdd
	case mir2.OpSub:
		op.Op = OpSub
	case mir2.OpMul:
		op.Op = OpMul
	case mir2.OpAnd:
		op.Op = OpAnd
	case mir2.OpOr:
		op.Op = OpOr
	case mir2.OpXor:
		op.Op = OpXor
	case mir2.OpCmp:
		op.Op = OpCmp
	case mir2.OpLoad:
		op.Op = OpLoad
	case mir2.OpStore:
		op.Op = OpStore
		op.Dst = -1
	default:
		// Skip unsupported ops (calls, branches, etc.) for now
		return nil, nil
	}

	return op, nil
}

// RunConvergence executes the same MIR2 function on MIR2-VM and LIR-VM,
// comparing results. Returns error on divergence.
func RunConvergence(f *mir2.Func, m *mir2.Module, desc *MachineDesc, args []mir2.Value) error {
	// MIR2-VM execution
	vm := mir2.NewVM(m)
	mir2Result, err := vm.Call(f.Name, args)
	if err != nil {
		return fmt.Errorf("mir2-vm: %w", err)
	}

	// LIR lowering + execution
	lirResult, err := LowerMIR2Func(f, desc)
	if err != nil {
		return fmt.Errorf("lir lower: %w", err)
	}

	lirVM := NewVM(desc)
	for i := range lirResult.Insts {
		if err := lirVM.ExecInst(&lirResult.Insts[i]); err != nil {
			return fmt.Errorf("lir-vm inst %d: %w", i, err)
		}
	}

	// Compare: MIR2 return value vs LIR last instruction's destination
	if len(mir2Result) == 0 {
		return nil // void function, nothing to compare
	}
	mir2Val := mir2Result[0].I

	if len(lirResult.Insts) == 0 {
		return fmt.Errorf("lir produced no instructions")
	}
	lastInst := lirResult.Insts[len(lirResult.Insts)-1]
	if lastInst.Dst.Phys < 0 {
		return fmt.Errorf("lir last instruction has no physical destination")
	}
	lirVal := int64(lirVM.Get(lastInst.Dst.Phys))

	if mir2Val != lirVal {
		return fmt.Errorf("DIVERGENCE: mir2-vm=%d, lir-vm(%s)=%d",
			mir2Val, desc.Name, lirVal)
	}

	return nil
}
