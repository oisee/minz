package mir2

// DeadStoreElim removes MIR2 instructions whose result register has no uses
// and whose opcode is pure (no observable side effects).
//
// The pass is iterated to fixpoint: removing one dead instruction may expose
// another (e.g. a constant that fed only a dead CP instruction).
//
// Ops considered pure (safe to delete when result is unused):
//   OpConst, OpMove, OpAdd/Sub/Mul/Div/SDig/Mod, OpAnd/Or/Xor/Shl/Shr/Sar,
//   OpNeg, OpNot, OpExt, OpSext, OpTrunc, OpCmp,
//   OpAddrOf, OpField, OpPtrBump.
//
// Ops NOT deleted even if result unused:
//   OpStore, OpPatch, OpCall, OpCallIndirect, OpAsm — observable side effects.
//   OpLoad — may be volatile (I/O mapped memory on Z80).
//   OpAlloca — changes the stack frame layout.
//   OpPatchSlot, OpLoadPatched — SMC slot lifecycle.
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
	case OpConst, OpMove,
		OpAdd, OpSub, OpMul, OpDiv, OpSDiv, OpMod,
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
	return uses
}

// termRegUses returns all registers read by a terminator.
func termRegUses(t Term) []Reg {
	return t.termUses()
}
