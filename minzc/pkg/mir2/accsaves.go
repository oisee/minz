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
func InsertAccSaves(f *Func, ar *AllocResult) error {
	if f == nil || ar == nil {
		return nil
	}
	for _, blk := range f.Blocks {
		if err := insertAccSavesBlock(f, blk, ar); err != nil {
			return err
		}
	}
	return nil
}

func insertAccSavesBlock(f *Func, blk *Block, ar *AllocResult) error {
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
				if scratch == NoReg {
					return fmt.Errorf("accumulator save in recursive function %s requires a frame-backed spill", f.Name)
				}
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
	return nil
}

// instClobbersA returns true if the instruction will overwrite A.
func instClobbersA(inst *Inst, ar *AllocResult) bool {
	switch inst.Op {
	case OpAdd, OpSub, OpMul, OpDiv, OpSDiv, OpSMod, OpMod,
		OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar,
		OpNeg, OpNot:
		// 8-bit ALU always uses A as working register on Z80
		if inst.Ty.Width() <= 8 {
			return true
		}
	case OpCmp:
		// CMP loads operand into A for CP instruction
		return true
	case OpCall, OpCallIndirect, OpCallCond:
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
		if accSaveDefines(blk.Insts[j], vreg) {
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
	mr := func(r Reg) Reg {
		if r == oldReg {
			return newReg
		}
		return r
	}
	mrs := func(rs []Reg) {
		for i := range rs {
			rs[i] = mr(rs[i])
		}
	}
	remapTerm(term, mr, mrs)
}

func accSaveDefines(inst *Inst, r Reg) bool {
	if inst.Dst == r {
		return true
	}
	for _, d := range inst.ExtraRets {
		if d == r {
			return true
		}
	}
	if inst.Asm != nil {
		for _, d := range inst.Asm.Outs {
			if d == r {
				return true
			}
		}
	}
	return false
}

// renameUses replaces all uses of oldReg with newReg in instructions at
// index > idx and in the block's terminator.
func renameUses(blk *Block, idx int, oldReg, newReg Reg) {
	for j := idx + 1; j < len(blk.Insts); j++ {
		inst := blk.Insts[j]
		inst.ReplaceUse(oldReg, newReg)
		// Uses in a redefining instruction consume the old value; later uses do not.
		if accSaveDefines(inst, oldReg) {
			return
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
	// Reserve physical bytes throughout the saved value's remaining lifetime,
	// including successor live-ins. A pair occupies both of its halves.
	usedRegs := map[string]bool{"A": true}
	reserve := func(r Reg) {
		loc := ar.Loc(r)
		if loc.Kind != LocReg {
			return
		}
		usedRegs[loc.Name] = true
		for _, name := range regBytes[loc.Name] {
			usedRegs[name] = true
		}
	}
	for _, r := range ComputeLiveness(f).LiveOutOf(f, blk).Slice() {
		reserve(r)
	}
	forceSpill := false
	for j := idx; j < len(blk.Insts); j++ {
		inst := blk.Insts[j]
		if inst.Op == OpCallIndirect || inst.Op == OpAsm {
			forceSpill = true
		}
		for _, r := range inst.Uses() {
			reserve(r)
		}
		reserve(inst.Dst)
		for _, r := range inst.ExtraRets {
			reserve(r)
		}
		if inst.Asm != nil {
			for _, r := range inst.Asm.Outs {
				reserve(r)
			}
		}
		if j > idx && accSaveDefines(inst, accOwner) {
			break
		}
	}
	if blk.Term != nil {
		for _, r := range blk.Term.termUses() {
			reserve(r)
		}
	}
	for _, name := range []string{"D", "E", "H", "L", "B", "C"} {
		if !forceSpill && !usedRegs[name] {
			synth := f.AllocReg()
			// Standalone functions can be used with a caller-owned combined map too.
			for {
				if _, exists := ar.Locs[synth]; !exists {
					break
				}
				synth = f.AllocReg()
			}
			ar.Locs[synth] = PhysLoc{Kind: LocReg, Name: name}
			return synth
		}
	}
	// Live spill homes are snapshotted by codegen around calls, including reentry.
	synth := f.AllocReg()
	for {
		if _, exists := ar.Locs[synth]; !exists {
			break
		}
		synth = f.AllocReg()
	}
	ar.Locs[synth] = PhysLoc{Kind: LocMem}
	ar.Spilled = append(ar.Spilled, synth)
	return synth
}
