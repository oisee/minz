package mir2

// CondRetSink transforms a BrIf whose else-branch is a trivial return block
// into a TermCondRet, enabling conditional-return instructions (RET CC on Z80).
//
// Pattern recognised:
//
//	block @cur:
//	    ...
//	    br_if %cond, @then(%ta...), @else()     ← ElseArgs empty
//
//	block @else:                                 ← no block params
//	    [zero or more pure instructions]
//	    ret %v...
//
// Transformation:
//
//	block @cur:
//	    ...
//	    [instructions hoisted from @else]
//	    cond_ret %cond, [%v...], @then(%ta...)
//
//	block @else:   ← now unreachable; removed by EliminateDeadBlocks
//
// Safety requirements:
//   - @else has no block params (verified: len(ElseArgs)==0)
//   - @else has exactly one predecessor (only reachable from this BrIf)
//   - All instructions in @else are DSE-pure (no side effects)
//   - @else.Term is *TermRet
//
// Returns true if any transformation was made.
func CondRetSink(f *Func) bool {
	changed := false
	for _, blk := range f.Blocks {
		brif, ok := blk.Term.(*TermBrIf)
		if !ok {
			continue
		}
		// Require: no args on the else edge (else block has no params).
		if len(brif.ElseArgs) != 0 {
			continue
		}
		elseBlock := f.BlockByLabel(brif.Else)
		if elseBlock == nil {
			continue
		}
		// Require: else block has no params.
		if len(elseBlock.Params) != 0 {
			continue
		}
		// Require: else block has exactly one predecessor (this block).
		if countPredecessors(f, brif.Else) != 1 {
			continue
		}
		// Require: else block terminates with TermRet.
		elseRet, ok := elseBlock.Term.(*TermRet)
		if !ok {
			continue
		}
		// Require: all instructions in else block are pure.
		if !allPure(elseBlock.Insts) {
			continue
		}
		// Require: hoisted instructions must not clobber the carry flag when
		// the branch condition is carry-based (CmpLt/CmpGe/CmpSubCarry…).
		// If they do, Z80 codegen would use the wrong carry for cond_ret.
		if condIsCarryBased(f, brif.Cond) && anyFlagClobberer(elseBlock.Insts) {
			continue
		}
		// Hoist instructions from @else into the current block.
		blk.Insts = append(blk.Insts, elseBlock.Insts...)
		// Replace BrIf with TermCondRet.
		blk.Term = &TermCondRet{
			Cond:     brif.Cond,
			Vals:     elseRet.Vals,
			Then:     brif.Then,
			ThenArgs: brif.ThenArgs,
		}
		// Sub-swap optimisation: if we hoisted `r = sub(x, y)` and the Then-block
		// contains `r' = sub(y, x)` (same operands, reversed), replace the Then-block
		// instruction with `r' = neg(r)`.  This avoids a live-range conflict on the
		// accumulator (A would otherwise be clobbered before the Then-block can use it).
		thenBlock := f.BlockByLabel(brif.Then)
		if thenBlock != nil {
			applySubSwapNeg(elseBlock.Insts, thenBlock)
		}
		// Sub+CmpLt flag fusion: if the block's cond_ret condition is from a
		// CmpLt(x, y) and we hoisted Sub(x, y) with the same operands, reorder
		// so Sub comes just before CmpLt.  At Z80 codegen time, a "last flags"
		// peephole will then suppress the redundant CP instruction.
		hoistReorderSubBeforeCmp(blk)
		changed = true
	}
	// Also apply fusionSubCmpInBlock to ALL blocks (not just CondRetSink ones):
	// this fuses Sub(x,y) immediately followed by Cmp(CmpGe/CmpLt, x,y) into
	// CmpSubCarry/CmpSubCarryNot, removing the original operands from the Cmp's
	// use set before register allocation.
	for _, blk := range f.Blocks {
		fusionSubCmpInBlock(blk)
	}
	return changed
}

// hoistReorderSubBeforeCmp reorders instructions in blk: for each
// OpCmp(CmpLt/CmpUlt, x, y) that has a later OpSub(x, y) with the same
// operands, move the Sub to the position just before the Cmp.
//
// After CondRetSink, a hoisted Sub ends up AFTER an existing CmpLt that
// tests the same operands.  After the reorder, the Cmp is converted to
// CmpSubCarry(subResult, y): carry from the Sub already encodes x<y, so
// Z80 codegen emits no CP instruction.  Crucially, the original lhs x
// is no longer a live source of the Cmp, which removes the interference
// between x (→A) and subResult (→A) in the PBQP allocator — allowing
// subResult to live in A with no spill.
//
// CmpSubCarry semantics (VM): (r + b) >= 2^width, where r = x−y and b = y.
// This equals the unsigned borrow from x−y, i.e. x < y.  See ops.go.
//
// This reorder is always safe: neither instruction depends on the other.
func hoistReorderSubBeforeCmp(blk *Block) {
	insts := blk.Insts
	for i := 0; i < len(insts); i++ {
		inst := insts[i]
		if inst.Op != OpCmp || inst.Dst == NoReg {
			continue
		}
		if inst.Cond != CmpLt && inst.Cond != CmpUlt {
			continue
		}
		cx, cy := inst.Src[0], inst.Src[1]
		// Find Sub(cx, cy) at a LATER position in the block.
		for j := i + 1; j < len(insts); j++ {
			s := insts[j]
			if s.Op != OpSub || s.Src[0] != cx || s.Src[1] != cy {
				continue
			}
			// Safety: no instruction between i+1 and j-1 redefines cx or cy.
			safe := true
			for k := i + 1; k < j; k++ {
				if insts[k].Dst == cx || insts[k].Dst == cy {
					safe = false
					break
				}
			}
			if !safe {
				continue
			}
			// Move Sub from position j to position i (just before Cmp at i).
			// Shift instructions in [i, j-1] one position right, then insert Sub at i.
			copy(insts[i+1:j+1], insts[i:j])
			insts[i] = s
			// The Cmp is now at position i+1.  Convert it to CmpSubCarry:
			//   - Src[0] = s.Dst  (the sub result register, in A after SUB)
			//   - Cond   = CmpSubCarry
			// This removes cx from the Cmp's use set, eliminating the
			// PBQP interference between cx (→A) and s.Dst (→A).
			// Z80 codegen: CmpSubCarry emits nothing (carry already in F).
			// VM: evaluates as (s.Dst + cy) >= 2^width = (cx < cy unsigned).
			cmp := insts[i+1]
			cmp.Src[0] = s.Dst
			cmp.Cond = CmpSubCarry
			cmp.SrcTy = s.Ty // width of the sub operands (needed by VM for carry check)
			break
		}
	}
}

// fusionSubCmpInBlock fuses a Sub(x,y) that IMMEDIATELY PRECEDES a Cmp(x,y) in
// the same block into a CmpSubCarry / CmpSubCarryNot.
//
// This handles the IAR abs_diff pattern:
//
//	r = sub(a, b)           // carry set by subtraction
//	cmp = CmpGe(a, b)       // same operands — carry already encodes a >= b (NC)
//
// After fusion:
//
//	r = sub(a, b)
//	cmp = CmpSubCarryNot(r, b)  // Z80 codegen emits nothing; BrIf uses NC
//
// This removes `a` from the Cmp's use set, which eliminates the live-range
// interference between `a` (original param, was in A) and `r` (sub result,
// also wants A) — allowing the allocator to put `r` in A without spilling `a`.
func fusionSubCmpInBlock(blk *Block) {
	insts := blk.Insts
	for i := 0; i+1 < len(insts); i++ {
		sub := insts[i]
		if sub.Op != OpSub {
			continue
		}
		cmp := insts[i+1]
		if cmp.Op != OpCmp || cmp.Dst == NoReg {
			continue
		}
		if cmp.Src[0] != sub.Src[0] || cmp.Src[1] != sub.Src[1] {
			continue
		}
		// Sub and Cmp reference the same original operands.
		// Replace Cmp's lhs with the sub result (removes `a` from use set).
		cmp.Src[0] = sub.Dst
		cmp.SrcTy = sub.Ty
		switch cmp.Cond {
		case CmpLt, CmpUlt:
			cmp.Cond = CmpSubCarry
		case CmpGe, CmpUge:
			cmp.Cond = CmpSubCarryNot
		}
		// Apply mutation (slice elements are pointers — already mutated above
		// since cmp is a pointer; no re-assign needed).
	}
}

// applySubSwapNeg rewrites instructions in thenBlock that are the operand-swapped
// form of a hoisted Sub instruction.
//
// For each hoisted `r_h = sub(x, y)` in hoisted:
//   - If thenBlock contains `r' = sub(y, x)` (same regs, reversed), replace with
//     `r' = neg(r_h)`.
//
// This eliminates the need for `x` to remain live into thenBlock, which would
// otherwise conflict with the accumulator register holding the hoisted sub result.
func applySubSwapNeg(hoisted []*Inst, thenBlock *Block) {
	for _, h := range hoisted {
		if h.Op != OpSub || h.Dst == NoReg {
			continue
		}
		hx, hy := h.Src[0], h.Src[1]
		for _, inst := range thenBlock.Insts {
			if inst.Op != OpSub {
				continue
			}
			if inst.Src[0] == hy && inst.Src[1] == hx {
				// Replace sub(y, x) with neg(r_h).
				// Force ClassAcc: NEG always reads/writes A, so the result must
				// be in A.  This avoids a spurious round-trip through ClassGeneral.
				inst.Op = OpNeg
				inst.Src[0] = h.Dst
				inst.Src[1] = NoReg
				inst.Cls = ClassAcc
			}
		}
	}
}

// allPure reports whether every instruction in the slice is DSE-pure
// (safe to hoist because it has no observable side effects).
func allPure(insts []*Inst) bool {
	for _, inst := range insts {
		if !isDSEPure(inst.Op) {
			return false
		}
	}
	return true
}

// countPredecessors counts how many blocks in f have label as a successor.
func countPredecessors(f *Func, label string) int {
	n := 0
	for _, blk := range f.Blocks {
		if blk.Term == nil {
			continue
		}
		for _, succ := range blk.Term.Successors() {
			if succ == label {
				n++
			}
		}
	}
	return n
}

// condIsCarryBased reports whether the register cond is produced by a carry-
// based comparison (CmpLt, CmpGe, CmpUlt, CmpUge, CmpSubCarry, CmpSubCarryNot).
// These comparisons leave a result in the CPU carry flag; any subsequent
// instruction that modifies carry will invalidate the condition.
func condIsCarryBased(f *Func, cond Reg) bool {
	for _, blk := range f.Blocks {
		for _, inst := range blk.Insts {
			if inst.Dst != cond {
				continue
			}
			switch inst.Cond {
			case CmpLt, CmpUlt, CmpGe, CmpUge,
				CmpSubCarry, CmpSubCarryNot:
				return true
			}
		}
	}
	return false
}

// anyFlagClobberer reports whether any instruction in the slice would clobber
// the carry flag in a way that breaks a preceding carry-based condition.
//
// The specific pattern we block is sub(const_0, x) — i.e. "0 - x" — which
// the Z80 backend emits as OR A (clears carry) + SBC HL, rr.  A plain
// sub(y, x) where y is not const_0 is handled by applySubSwapNeg (-> Neg)
// which does not suffer from the same flag-clobber issue.
func anyFlagClobberer(insts []*Inst) bool {
	// Collect const-zero registers defined in the slice.
	constZero := map[Reg]bool{}
	for _, inst := range insts {
		if inst.Op == OpConst && inst.Imm == 0 {
			constZero[inst.Dst] = true
		}
		// sub(const_0, x) emits OR A on Z80, clobbering carry.
		if inst.Op == OpSub && len(inst.Src) >= 1 && constZero[inst.Src[0]] {
			return true
		}
		// NEG emits Z80 NEG which clobbers carry.
		if inst.Op == OpNeg {
			return true
		}
	}
	return false
}
