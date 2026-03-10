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
// Uses a type switch because Term.termUses() is package-private.
func termRegUses(t Term) []Reg {
	switch v := t.(type) {
	case *TermJmp:
		return v.Args
	case *TermBrIf:
		out := make([]Reg, 0, 1+len(v.ThenArgs)+len(v.ElseArgs))
		if v.Cond != NoReg {
			out = append(out, v.Cond)
		}
		out = append(out, v.ThenArgs...)
		out = append(out, v.ElseArgs...)
		return out
	case *TermBrIf2:
		out := make([]Reg, 0, 2+len(v.EqArgs)+len(v.LtArgs)+len(v.GtArgs))
		if v.Lhs != NoReg {
			out = append(out, v.Lhs)
		}
		if v.Rhs != NoReg {
			out = append(out, v.Rhs)
		}
		out = append(out, v.EqArgs...)
		out = append(out, v.LtArgs...)
		out = append(out, v.GtArgs...)
		return out
	case *TermDJNZ:
		uses := make([]Reg, 0, 1+len(v.BodyArgs)+len(v.ExitArgs))
		if v.Counter != NoReg {
			uses = append(uses, v.Counter)
		}
		uses = append(uses, v.BodyArgs...)
		uses = append(uses, v.ExitArgs...)
		return uses
	case *TermCondRet:
		out := make([]Reg, 0, 1+len(v.Vals)+len(v.ThenArgs))
		if v.Cond != NoReg {
			out = append(out, v.Cond)
		}
		out = append(out, v.Vals...)
		out = append(out, v.ThenArgs...)
		return out
	case *TermRet:
		return v.Vals
	}
	return nil
}
