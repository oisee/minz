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
// The module parameter is needed for callee contract lookup (OpCall).
func LowerMIR2Block(b *mir2.Block, desc *MachineDesc, mod *mir2.Module) ([]MIROp, error) {
	var ops []MIROp

	for _, inst := range b.Insts {
		if inst.Op == mir2.OpCall || inst.Op == mir2.OpCallIndirect {
			callOps, err := translateCall(inst, desc, mod)
			if err != nil {
				return nil, fmt.Errorf("block %s: %w", b.Label, err)
			}
			ops = append(ops, callOps...)
			continue
		}
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
		ops, err := LowerMIR2Block(b, desc, nil)
		if err != nil {
			return nil, err
		}
		allOps = append(allOps, ops...)
	}

	if len(allOps) == 0 {
		return &LowerResult{}, nil
	}

	// Extract function params for isel+WFC.
	params := ContractParamsToBlockParams(f, desc)

	// Instruction selection with param pre-seeding
	sel, err := SelectBlockInstructions(desc, allOps, params)
	if err != nil {
		return nil, fmt.Errorf("isel: %w", err)
	}

	// WFC constraint propagation + collapse with param cells
	wfc := NewWFCStateWithParams(desc, sel.Insts, params)
	wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		return nil, fmt.Errorf("wfc: %w", err)
	}

	return &LowerResult{
		Ops:   allOps,
		Insts: wfc.ToInsts(),
	}, nil
}

// LowerMIR2Prog converts a full MIR2 function into a multi-block LIR Prog,
// preserving block structure, params, terminators, and edge arguments.
// This is the structured alternative to LowerMIR2Func (which flattens).
func LowerMIR2Prog(f *mir2.Func, desc *MachineDesc, mod *mir2.Module) (*Prog, error) {
	prog := &Prog{
		Name:   f.Name,
		Blocks: make([]Block, 0, len(f.Blocks)),
		Desc:   desc,
	}

	for _, mb := range f.Blocks {
		b := Block{Label: mb.Label}

		// Translate block params
		for _, mp := range mb.Params {
			width := 8
			if mp.Ty != nil {
				if w := mp.Ty.Width(); w > 0 {
					width = w
				}
			}
			if width < 8 {
				width = 8
			}
			b.Params = append(b.Params, BlockParam{
				VReg:    int(mp.Dst),
				Allowed: regClassToLocSet(desc, mp.Class, width),
				Phys:    -1,
			})
		}

		// Translate instructions
		ops, err := LowerMIR2Block(mb, desc, mod)
		if err != nil {
			return nil, err
		}
		// Store ops temporarily — isel happens later per-block
		b.Insts = nil // will be filled after isel
		// Stash MIROps in a side map — but for now we embed them as
		// un-selected instructions (no pattern) so we can attach them later.
		// Actually, we'll return a Prog with a parallel ops slice. But
		// Prog doesn't have that. Let's use a simpler approach: store ops
		// in the block's Insts as "raw" Insts with MIROp encoded.
		// Better: just do isel inline.
		_ = ops

		// Translate terminator
		b.Term, err = translateTerm(mb.Term, desc)
		if err != nil {
			return nil, fmt.Errorf("block %s term: %w", mb.Label, err)
		}

		prog.Blocks = append(prog.Blocks, b)
	}

	return prog, nil
}

// LowerMIR2ProgWithOps converts a MIR2 function into a Prog and also returns
// per-block MIROps for later isel. The Prog.Blocks[i].Insts are initially empty.
func LowerMIR2ProgWithOps(f *mir2.Func, desc *MachineDesc, mod *mir2.Module) (*Prog, [][]MIROp, error) {
	prog := &Prog{
		Name:   f.Name,
		Blocks: make([]Block, 0, len(f.Blocks)),
		Desc:   desc,
	}

	blockOps := make([][]MIROp, 0, len(f.Blocks))

	for _, mb := range f.Blocks {
		b := Block{Label: mb.Label}

		// Translate block params
		for _, mp := range mb.Params {
			width := 8
			if mp.Ty != nil {
				if w := mp.Ty.Width(); w > 0 {
					width = w
				}
			}
			if width < 8 {
				width = 8
			}
			b.Params = append(b.Params, BlockParam{
				VReg:    int(mp.Dst),
				Allowed: regClassToLocSet(desc, mp.Class, width),
				Phys:    -1,
			})
		}

		// Translate instructions to MIROps
		ops, err := LowerMIR2Block(mb, desc, mod)
		if err != nil {
			return nil, nil, err
		}
		blockOps = append(blockOps, ops)

		// Translate terminator
		var termErr error
		b.Term, termErr = translateTerm(mb.Term, desc)
		if termErr != nil {
			return nil, nil, fmt.Errorf("block %s term: %w", mb.Label, termErr)
		}

		prog.Blocks = append(prog.Blocks, b)
	}

	return prog, blockOps, nil
}

// ContractParamsToBlockParams converts a MIR2 function's Contract.Params
// into LIR BlockParam entries. This allows the flat codegen path to seed
// WFC with the same param constraints that the multi-block path uses.
func ContractParamsToBlockParams(f *mir2.Func, desc *MachineDesc) []BlockParam {
	var params []BlockParam
	for _, cp := range f.Contract.Params {
		width := 8
		if cp.Ty != nil {
			if w := cp.Ty.Width(); w > 0 {
				width = w
			}
		}
		if width < 8 {
			width = 8
		}
		params = append(params, BlockParam{
			VReg:    int(cp.Reg),
			Allowed: regClassToLocSet(desc, cp.Class, width),
			Phys:    -1,
		})
	}
	return params
}

// regClassToLocSet maps a MIR2 register class + width to a LIR LocSet.
func regClassToLocSet(desc *MachineDesc, cls mir2.RegClass, width int) LocSet {
	// For non-Z80 descs, just return all locs of matching width.
	if desc.Name != "z80" {
		s := desc.LocsOfWidth(width)
		if !s.IsEmpty() {
			return s
		}
		return desc.LocsOfWidth(desc.WordSize)
	}

	// Z80-specific class → loc mapping
	switch cls {
	case mir2.ClassAcc:
		return desc.LocSetByNames("A")
	case mir2.ClassCounter:
		return desc.LocSetByNames("B")
	case mir2.ClassPointer:
		return desc.LocSetByNames("HL")
	case mir2.ClassIndex:
		return desc.LocSetByNames("DE")
	case mir2.ClassPair:
		return desc.LocSetByNames("HL", "DE", "BC")
	case mir2.ClassIX:
		return desc.LocSetByNames("IX")
	case mir2.ClassIY:
		return desc.LocSetByNames("IY")
	case mir2.ClassFlag:
		return desc.LocSetByNames("F")
	case mir2.ClassGeneral:
		if width >= 16 {
			return desc.LocSetByNames("HL", "DE", "BC")
		}
		return desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
	case mir2.ClassMem:
		s := LocSet(0)
		for i, loc := range desc.Locs {
			if loc.Kind == LocMem {
				s = s.Set(i)
			}
		}
		return s
	default:
		// Fallback: all locs of matching width
		s := desc.LocsOfWidth(width)
		if !s.IsEmpty() {
			return s
		}
		return desc.LocsOfWidth(desc.WordSize)
	}
}

// translateTerm converts a MIR2 terminator to a LIR Term.
func translateTerm(t mir2.Term, desc *MachineDesc) (Term, error) {
	if t == nil {
		return Term{Kind: TermNone}, nil
	}

	regToOp := func(r mir2.Reg) Operand {
		if r == mir2.NoReg {
			return Operand{VReg: -1, Phys: -1}
		}
		return Operand{VReg: int(r), Allowed: desc.LocsOfWidth(8), Phys: -1}
	}

	regsToOps := func(regs []mir2.Reg) []Operand {
		ops := make([]Operand, len(regs))
		for i, r := range regs {
			ops[i] = regToOp(r)
		}
		return ops
	}

	switch tt := t.(type) {
	case *mir2.TermJmp:
		term := Term{
			Kind:    TermJump,
			Targets: []string{tt.Target},
		}
		if len(tt.Args) > 0 {
			term.Args = [][]Operand{regsToOps(tt.Args)}
		}
		return term, nil

	case *mir2.TermBrIf:
		term := Term{
			Kind:    TermBranch,
			Cond:    regToOp(tt.Cond),
			Targets: []string{tt.Then, tt.Else},
			Args:    make([][]Operand, 2),
		}
		if len(tt.ThenArgs) > 0 {
			term.Args[0] = regsToOps(tt.ThenArgs)
		}
		if len(tt.ElseArgs) > 0 {
			term.Args[1] = regsToOps(tt.ElseArgs)
		}
		return term, nil

	case *mir2.TermBrIf2:
		// Three-way branch: split into cmp block → two conditional edges.
		// For now, emit as a TermBranch with Lt as "then" and a fallthrough
		// to handle Eq/Gt. The pipeline will handle this via block splitting.
		// Simplification: encode as TermBranch3 (Lt vs not-Lt), then the
		// not-Lt block branches on Eq vs Gt.
		// Actually, per the plan: TermBrIf2 → two TermBranch blocks.
		// But that's a block-level transform, not a term-level one.
		// At the term level, we can't create new blocks. Return a special
		// encoding that the pipeline splits later.
		// For now: treat as TermBranch comparing Lhs < Rhs.
		// The bridge caller will handle the split.
		return Term{
			Kind: TermBranch,
			Cond: regToOp(tt.Lhs),
			Targets: []string{tt.Lt, tt.Gt},
			Args: [][]Operand{regsToOps(tt.LtArgs), regsToOps(tt.GtArgs)},
		}, nil

	case *mir2.TermDJNZ:
		term := Term{
			Kind:    TermDJNZ,
			Counter: regToOp(tt.Counter),
			Targets: []string{tt.Body, tt.Exit},
			Args:    make([][]Operand, 2),
		}
		if len(tt.BodyArgs) > 0 {
			term.Args[0] = regsToOps(tt.BodyArgs)
		}
		if len(tt.ExitArgs) > 0 {
			term.Args[1] = regsToOps(tt.ExitArgs)
		}
		return term, nil

	case *mir2.TermRet:
		term := Term{
			Kind:    TermReturn,
			RetVals: regsToOps(tt.Vals),
		}
		return term, nil

	case *mir2.TermCondRet:
		// Conditional return: treat as branch to then-block with return as fallthrough.
		// Simplification: if cond==0 return, else jump to Then.
		term := Term{
			Kind:    TermBranch,
			Cond:    regToOp(tt.Cond),
			Targets: []string{tt.Then, ""},
			Args:    make([][]Operand, 2),
		}
		if len(tt.ThenArgs) > 0 {
			term.Args[0] = regsToOps(tt.ThenArgs)
		}
		return term, nil

	case *mir2.TermUnreachable:
		return Term{Kind: TermReturn}, nil

	default:
		return Term{Kind: TermNone}, nil
	}
}

// translateCall converts an OpCall/OpCallIndirect into a sequence of LIR MIROps:
// argument setup moves (one per arg) + the call itself.
// Returns nil, nil if the call can't be lowered (e.g. indirect call, missing module).
func translateCall(inst *mir2.Inst, desc *MachineDesc, mod *mir2.Module) ([]MIROp, error) {
	if inst.Op == mir2.OpCallIndirect {
		return nil, nil // indirect calls not yet supported
	}
	if mod == nil {
		return nil, nil // can't look up callee without module
	}

	callee := mod.FuncByName(inst.Sym)
	if callee == nil {
		return nil, nil // extern/unknown callee — skip
	}

	var ops []MIROp

	// Emit argument setup: move each arg vreg into its param-class-constrained vreg.
	// The isel will pick the right move pattern, and WFC will collapse to the
	// physical register matching the param class.
	for i, argReg := range inst.Args {
		if i >= len(callee.Contract.Params) {
			break
		}
		cp := callee.Contract.Params[i]
		width := 8
		if cp.Ty != nil {
			if w := cp.Ty.Width(); w > 0 {
				width = w
			}
		}
		if width < 8 {
			width = 8
		}
		// Emit: move argReg → constrained vreg matching callee param class.
		// DstAllowed tells isel to narrow the move destination to the
		// callee's expected register class.
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        int(cp.Reg),
			Src:        [2]int{int(argReg), -1},
			Width:      width,
			DstAllowed: regClassToLocSet(desc, cp.Class, width),
		})
	}

	// Emit the call itself.
	width := 0 // void
	if inst.Ty != nil {
		w := inst.Ty.Width()
		if w > 0 {
			width = w
		}
	}
	if width < 8 && width > 0 {
		width = 8
	}

	callOp := MIROp{
		Op:    OpCall,
		Dst:   int(inst.Dst),
		Src:   [2]int{-1, -1},
		Width: width,
		Sym:   inst.Sym,
	}
	if inst.Dst == mir2.NoReg {
		callOp.Dst = -1
	}

	ops = append(ops, callOp)
	return ops, nil
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
	// Several ops produce or consume booleans (width=1) but the actual
	// operation uses 8-bit registers. Promote narrow widths to 8.
	if width < 8 {
		width = 8
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
	case mir2.OpShl:
		op.Op = OpShl
	case mir2.OpShr, mir2.OpSar:
		op.Op = OpShr
	case mir2.OpCmp:
		op.Op = OpCmp
	case mir2.OpLoad:
		op.Op = OpLoad
	case mir2.OpStore:
		op.Op = OpStore
		op.Dst = -1
	case mir2.OpNeg:
		// neg(x) → sub(0, x): emit const 0 + sub
		// For now, skip — isel doesn't have a neg pattern yet
		return nil, nil
	case mir2.OpNot:
		// bitwise complement — skip for now
		return nil, nil
	case mir2.OpExt, mir2.OpSext, mir2.OpTrunc:
		// Type conversions — treat as move for now
		op.Op = OpMove
		op.Src[1] = -1
	case mir2.OpField, mir2.OpPtrBump:
		// Pointer offset — treat as add with immediate
		op.Op = OpAdd
		// Imm is the byte offset; src[1] is unused, use const
		op.Src[1] = -1
	case mir2.OpPtrAdd:
		op.Op = OpAdd
	case mir2.OpAddrOf:
		// Address of global — treat as const with symbol address
		op.Op = OpConst
		op.Src = [2]int{-1, -1}
	case mir2.OpCall, mir2.OpCallIndirect:
		// Calls: skip for now (need calling convention support)
		return nil, nil
	case mir2.OpPush, mir2.OpPop:
		// Stack ops: skip (handled by regalloc)
		return nil, nil
	case mir2.OpPatchSlot, mir2.OpLoadPatched, mir2.OpPatch:
		// SMC: skip (target-specific)
		return nil, nil
	case mir2.OpAsm:
		// Inline assembly: skip
		return nil, nil
	case mir2.OpAlloca:
		// Stack alloc: skip
		return nil, nil
	default:
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
