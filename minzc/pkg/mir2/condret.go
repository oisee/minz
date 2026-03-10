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
