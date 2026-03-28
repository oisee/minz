// accsaves.go — Post-allocation pass that inserts explicit save/restore
// instructions when A (accumulator) holds a live value that would be clobbered
// by an upcoming instruction.
//
// Z80's accumulator architecture means most ALU/CMP/CALL instructions require
// or clobber A. The PBQP register allocator assigns vregs to A based on live
// range analysis, but doesn't insert spills when multiple A-allocated vregs
// overlap through destructive instructions.
//
// This pass walks each block in instruction order, simulates which vreg
// currently owns A, and when an instruction will destroy A while a live vreg
// is in it, inserts an OpMove to save it to a scratch register.
package mir2

import "fmt"

var _ = fmt.Sprintf // keep import for debug

// InsertAccSaves walks the function after register allocation and inserts
// OpMove instructions to save A before it's clobbered by ALU/CMP/CALL ops.
//
// Specifically, it detects when:
//  1. A vreg V is allocated to A and its value is "currently in A"
//  2. An upcoming instruction I will overwrite A (ALU, CMP, CALL, or loads a
//     different vreg into A for its operand)
//  3. V is still used after I (live across I)
//
// When detected, it inserts an OpMove(V → scratch) before I, where scratch
// is a free GPR that will be preserved across I (via PUSH/POP if needed).
func InsertAccSaves(f *Func, ar *AllocResult) {
	if f == nil || ar == nil {
		return
	}
	for _, blk := range f.Blocks {
		insertAccSavesBlock(f, blk, ar)
	}
	_ = f.Name
}

func insertAccSavesBlock(f *Func, blk *Block, ar *AllocResult) {
	// Track which vreg currently "owns" A.
	accOwner := NoReg

	// Initialize from params (entry block)
	for _, cp := range f.Contract.Params {
		if cp.Reg == NoReg {
			continue
		}
		loc := ar.Loc(cp.Reg)
		_ = loc // debug removed
		if loc.Kind == LocReg && loc.Name == "A" {
			accOwner = cp.Reg
		}
	}
	// Also check block params
	for _, bp := range blk.Params {
		loc := ar.Loc(bp.Dst)
		if loc.Kind == LocReg && loc.Name == "A" {
			accOwner = bp.Dst
		}
	}
	// For non-entry blocks: check if a vreg allocated to A is live-in.
	// Build the set of vregs that belong to THIS function.
	funcVregs := make(map[Reg]bool)
	for _, cp := range f.Contract.Params {
		if cp.Reg != NoReg {
			funcVregs[cp.Reg] = true
		}
	}
	for _, b := range f.Blocks {
		for _, bp := range b.Params {
			funcVregs[bp.Dst] = true
		}
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				funcVregs[inst.Dst] = true
			}
		}
	}

	// Look for a vreg that: belongs to this function, is allocated to A,
	// is used in this block, and is defined OUTSIDE this block (live-in).
	if accOwner == NoReg {
		for _, inst := range blk.Insts {
			for _, u := range inst.Uses() {
				if u == NoReg || !funcVregs[u] {
					continue
				}
				loc := ar.Loc(u)
				if loc.Kind == LocReg && loc.Name == "A" {
					definedHere := false
					for _, other := range blk.Insts {
						if other.Dst == u {
							definedHere = true
							break
						}
					}
					for _, bp := range blk.Params {
						if bp.Dst == u {
							definedHere = true
							break
						}
					}
					if !definedHere {
						accOwner = u
						break
					}
				}
			}
			if accOwner != NoReg {
				break
			}
		}
	}

	var newInsts []*Inst
	changed := false

	for i, inst := range blk.Insts {
		// Check if this instruction will clobber A.
		willClobberA := instClobbersA(inst, ar)

		if willClobberA && accOwner != NoReg {
			// Check if accOwner is still needed AFTER this instruction.
			needSave := isUsedAfterIdx(blk, i, accOwner)

			if needSave {
				// Insert OpMove to save accOwner to a scratch register,
				// AND rename all future uses of accOwner to the scratch.
				scratch := pickAccSaveScratch(f, blk, i, ar, accOwner)
				if scratch != NoReg {
					saveInst := &Inst{
						Op:  OpMove,
						Dst: scratch,
						Src: [2]Reg{accOwner, NoReg},
						Ty:  TyU8,
						Cls: classFromLoc(ar.Loc(scratch)),
					}
					newInsts = append(newInsts, saveInst)

					// Rename all subsequent uses of accOwner to scratch.
					renameUses(blk, i, accOwner, scratch)
					changed = true
					_ = blk.Label
				}
			}
		}

		newInsts = append(newInsts, inst)

		// Update accOwner: if this instruction defines a vreg allocated to A,
		// that vreg now owns A.
		if inst.Dst != NoReg && inst.Dst > 0 {
			loc := ar.Loc(inst.Dst)
			if loc.Kind == LocReg && loc.Name == "A" {
				accOwner = inst.Dst
			}
		}
	}

	if changed {
		blk.Insts = newInsts
	}
}

// instClobbersA returns true if the instruction will overwrite A.
func instClobbersA(inst *Inst, ar *AllocResult) bool {
	switch inst.Op {
	case OpAdd, OpSub, OpMul, OpDiv, OpSDiv, OpMod,
		OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar,
		OpNeg, OpNot:
		// 8-bit ALU always uses A as working register on Z80
		if inst.Ty.Width() <= 8 {
			return true
		}
	case OpCmp:
		// CMP loads operand into A for CP instruction
		return true
	case OpCall, OpCallIndirect:
		// CALL clobbers A (return value register)
		return true
	case OpMove:
		// Move TO A clobbers A
		if inst.Dst != NoReg {
			loc := ar.Loc(inst.Dst)
			if loc.Kind == LocReg && loc.Name == "A" {
				return true
			}
		}
	case OpConst:
		// LD A, imm clobbers A if const is allocated to A
		if inst.Dst != NoReg {
			loc := ar.Loc(inst.Dst)
			if loc.Kind == LocReg && loc.Name == "A" {
				return true
			}
		}
	}
	return false
}

// isLiveAfterIdx checks if vreg is used by any instruction at index > idx
// or by the block's terminator.
func isLiveAfterIdx(blk *Block, idx int, vreg Reg) bool {
	for j := idx + 1; j < len(blk.Insts); j++ {
		for _, u := range blk.Insts[j].Uses() {
			if u == vreg {
				return true
			}
		}
	}
	if blk.Term != nil {
		for _, u := range blk.Term.termUses() {
			if u == vreg {
				return true
			}
		}
	}
	return false
}

// isUsedAfterIdx is like isLiveAfterIdx but also checks if the vreg is
// redefined (shadowed) before its next use. Returns false if redefined first.
func isUsedAfterIdx(blk *Block, idx int, vreg Reg) bool {
	for j := idx + 1; j < len(blk.Insts); j++ {
		// Check if used
		for _, u := range blk.Insts[j].Uses() {
			if u == vreg {
				return true
			}
		}
		// Check if redefined (shadows the value)
		if blk.Insts[j].Dst == vreg {
			return false
		}
	}
	if blk.Term != nil {
		for _, u := range blk.Term.termUses() {
			if u == vreg {
				return true
			}
		}
	}
	return false
}

// renameTermReg replaces oldReg with newReg in a terminator's register uses.
func renameTermReg(term Term, oldReg, newReg Reg) {
	switch t := term.(type) {
	case *TermRet:
		for i, v := range t.Vals {
			if v == oldReg {
				t.Vals[i] = newReg
			}
		}
	case *TermBrIf:
		if t.Cond == oldReg {
			t.Cond = newReg
		}
		for i, a := range t.ThenArgs {
			if a == oldReg {
				t.ThenArgs[i] = newReg
			}
		}
		for i, a := range t.ElseArgs {
			if a == oldReg {
				t.ElseArgs[i] = newReg
			}
		}
	case *TermCondRet:
		if t.Cond == oldReg {
			t.Cond = newReg
		}
		for i, v := range t.Vals {
			if v == oldReg {
				t.Vals[i] = newReg
			}
		}
		for i, a := range t.ThenArgs {
			if a == oldReg {
				t.ThenArgs[i] = newReg
			}
		}
	case *TermJmp:
		for i, a := range t.Args {
			if a == oldReg {
				t.Args[i] = newReg
			}
		}
	}
}

// renameUses replaces all uses of oldReg with newReg in instructions at
// index > idx and in the block's terminator.
func renameUses(blk *Block, idx int, oldReg, newReg Reg) {
	for j := idx + 1; j < len(blk.Insts); j++ {
		inst := blk.Insts[j]
		for k := range inst.Src {
			if inst.Src[k] == oldReg {
				inst.Src[k] = newReg
			}
		}
		for k, a := range inst.Args {
			if a == oldReg {
				inst.Args[k] = newReg
			}
		}
	}
	if blk.Term != nil {
		renameTermReg(blk.Term, oldReg, newReg)
	}
}

func classFromLoc(loc PhysLoc) RegClass {
	switch loc.Name {
	case "A":
		return ClassAcc
	case "B":
		return ClassCounter
	case "D", "E", "H", "L":
		return ClassGeneral
	case "C":
		return ClassRegC
	}
	return ClassGeneral
}

// pickAccSaveScratch finds a free vreg allocated to a GPR (not A) that can
// hold the saved A value. Creates a new synthetic vreg if needed.
func pickAccSaveScratch(f *Func, blk *Block, idx int, ar *AllocResult, accOwner Reg) Reg {
	// Find a GPR that's not A and not used by the current or nearby instructions.
	// Strategy: create a new synthetic vreg allocated to a free GPR.
	usedRegs := make(map[string]bool)
	usedRegs["A"] = true // A is what we're saving FROM
	// Collect regs used by instructions around idx
	for j := idx; j < len(blk.Insts) && j < idx+3; j++ {
		for _, u := range blk.Insts[j].Uses() {
			loc := ar.Loc(u)
			if loc.Kind == LocReg {
				usedRegs[loc.Name] = true
			}
		}
	}

	for _, name := range []string{"D", "E", "H", "L", "B", "C"} {
		if !usedRegs[name] {
			// Create synthetic vreg
			synth := f.AllocReg()
			ar.Locs[synth] = PhysLoc{Kind: LocReg, Name: name}
			return synth
		}
	}
	return NoReg
}
