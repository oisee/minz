package mir2

// DeadStoreElim removes MIR2 instructions whose result register has no uses
// and whose opcode is pure (no observable side effects).
//
// The pass is iterated to fixpoint: removing one dead instruction may expose
// another (e.g. a constant that fed only a dead CP instruction).
//
// Ops considered pure (safe to delete when result is unused):
//
//	OpConst, OpMove, OpAdd/Sub/Mul, OpAnd/Or/Xor/Shl/Shr/Sar,
//	OpNeg, OpNot, OpExt, OpSext, OpTrunc, OpCmp,
//	OpAddrOf, OpField, OpPtrBump.
//
// Ops NOT deleted even if result unused:
//
//	OpStore, OpPatch, OpCall, OpCallIndirect, OpAsm — observable side effects.
//	OpLoad — may be volatile (I/O mapped memory on Z80).
//	OpAlloca — changes the stack frame layout.
//	OpPatchSlot, OpLoadPatched — SMC slot lifecycle.
//	OpDiv, OpSDiv, OpMod, OpSMod — may trap on a zero divisor. A dead
//	  result does not make the operation unobservable. Proven constant
//	  nonzero divisions can still be folded before this pass.
func DeadStoreElim(f *Func) {
	for {
		uses := countRegUses(f)
		changed := false
		for _, b := range f.Blocks {
			keep := b.Insts[:0]
			for _, inst := range b.Insts {
				if inst.Dst != NoReg && uses[inst.Dst] == 0 && isDSEPure(inst.Op) {
					changed = true
					continue // drop dead instruction
				}
				keep = append(keep, inst)
			}
			b.Insts = keep
		}
		if !changed {
			break
		}
	}
}

// isDSEPure reports whether op can be safely deleted when its result is unused.
func isDSEPure(op Op) bool {
	switch op {
	case OpConst, OpMove, OpPackBytes,
		OpAdd, OpSub, OpMul,
		OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar,
		OpNeg, OpNot, OpExt, OpSext, OpTrunc,
		OpCmp,
		OpAddrOf, OpField, OpPtrBump, OpPtrAdd:
		return true
	}
	return false
}

// countRegUses returns a map from Reg → number of uses across the entire function.
// Block params are NOT counted as uses (they are definitions).
// Terminator args ARE counted as uses.
func countRegUses(f *Func) map[Reg]int {
	uses := make(map[Reg]int)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for _, r := range inst.Uses() {
				uses[r]++
			}
		}
		if b.Term != nil {
			for _, r := range termRegUses(b.Term) {
				uses[r]++
			}
		}
	}
	// DJNZ supplies the body counter implicitly. Its first block parameter
	// must remain present even when no instruction reads it directly.
	for _, b := range f.Blocks {
		if t, ok := b.Term.(*TermDJNZ); ok {
			if body := f.BlockByLabel(t.Body); body != nil && len(body.Params) > 0 {
				uses[body.Params[0].Dst]++
			}
		}
	}

	return uses
}

// termRegUses returns all registers read by a terminator.
func termRegUses(t Term) []Reg {
	return t.termUses()
}
