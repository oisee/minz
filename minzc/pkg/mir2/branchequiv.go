package mir2

// BranchEquiv eliminates redundant equality branches in pure functions with
// at most two u8 parameters. This is the input domain the VM check exhausts.
//
// # Target pattern
//
// A TermBrIf guarded by CmpEq where, for every input that makes the equality
// true, the else-path produces the same output as the then-path.  In that case
// the TermBrIf is replaced by an unconditional TermJmp to the else-block.
//
// # Motivating example — abs_diff(a, b: u8) → u8
//
//	entry:  eq = CmpEq(a, b)
//	        BrIf(eq, @zero, @diff)       ← this BrIf is redundant
//	zero:   ret 0
//	diff:   lt = CmpLt(a, b)
//	        BrIf(lt, @b_minus_a, @a_minus_b)
//	b_minus_a: ret b-a                   ← when a==b: b-a = 0
//	a_minus_b: ret a-b                   ← when a==b: a-b = 0
//
// Both paths in @diff return 0 when a==b, so the @zero shortcut is dead code.
// Removing the BrIf saves 10T + 3 bytes on Z80 (JP Z omitted).
//
// # Check method
//
// Run original and patched functions on every input satisfying the equality.
// This establishes agreement under the MIR2 VM semantics for the admitted
// domain; it is not an independent proof of those semantics.
func BranchEquiv(m *Module, f *Func) bool {
	// The comparator observes returns only. Reject memory, calls, I/O, asm,
	// patches and any future opcode until its effects are explicitly handled.
	if !beqAdmittedPureU8(f) {
		return false
	}
	vm := NewVM(m)
	changed := false

	for _, blk := range f.Blocks {
		brif, ok := blk.Term.(*TermBrIf)
		if !ok {
			continue
		}

		// Find the OpCmp(CmpEq, ...) that defines the condition register.
		cmpInst := beqFindDef(f, brif.Cond)
		if cmpInst == nil || cmpInst.Op != OpCmp || cmpInst.Cond != CmpEq {
			continue
		}

		// Determine how to generate boundary inputs where CmpEq holds.
		gen := beqMakeBoundaryGen(f, cmpInst)
		if gen == nil {
			continue
		}
		if gen.rhsIdx < 0 && (gen.rhsConst < 0 || gen.rhsConst > 255) {
			continue
		}

		// Patch: replace TermBrIf with unconditional Jmp to the else-branch.
		patchedJmp := &TermJmp{Target: brif.Else, Args: brif.ElseArgs}
		blk.Term = patchedJmp

		if beqTestEquivalent(f, blk, gen, vm, brif, patchedJmp) {
			// Optimization accepted — leave blk.Term = patchedJmp.
			changed = true
		} else {
			// Not proven equivalent — restore.
			blk.Term = brif
		}
	}

	return changed
}

// ── boundary input generator ──────────────────────────────────────────────────

// beqBoundaryGen describes how to construct function arguments where the
// CmpEq condition is satisfied (lhs == rhs).
type beqBoundaryGen struct {
	nParams  int
	lhsIdx   int   // param index of the lhs operand
	rhsIdx   int   // param index of the rhs operand; -1 if rhs is a constant
	rhsConst int64 // rhs constant value (used when rhsIdx == -1)
}

// beqMakeBoundaryGen returns a generator for boundary inputs, or nil if the
// operands of cmpEq cannot be traced to function parameters or constants.
func beqMakeBoundaryGen(f *Func, cmpEq *Inst) *beqBoundaryGen {
	n := len(f.Contract.Params)
	if n == 0 {
		return nil
	}

	lhsIdx := beqTraceToParam(f, cmpEq.Src[0])
	rhsIdx := beqTraceToParam(f, cmpEq.Src[1])

	if lhsIdx >= 0 && rhsIdx >= 0 {
		if lhsIdx == rhsIdx {
			// Comparing a register to itself — always true, degenerate.
			return nil
		}
		return &beqBoundaryGen{nParams: n, lhsIdx: lhsIdx, rhsIdx: rhsIdx}
	}

	// Try one side as param, the other as constant.
	if lhsIdx >= 0 {
		if c, ok := beqTraceToConst(f, cmpEq.Src[1]); ok {
			return &beqBoundaryGen{nParams: n, lhsIdx: lhsIdx, rhsIdx: -1, rhsConst: c}
		}
	}
	if rhsIdx >= 0 {
		if c, ok := beqTraceToConst(f, cmpEq.Src[0]); ok {
			// Swap so lhsIdx is always the param.
			return &beqBoundaryGen{nParams: n, lhsIdx: rhsIdx, rhsIdx: -1, rhsConst: c}
		}
	}

	return nil
}

// beqTraceToParam returns the Contract.Params index for register r, following
// chains of OpMove.  Returns -1 if not traceable to a param.
func beqTraceToParam(f *Func, r Reg) int {
	for i, p := range f.Contract.Params {
		if p.Reg == r {
			return i
		}
	}
	def := beqFindDef(f, r)
	if def != nil && def.Op == OpMove {
		return beqTraceToParam(f, def.Src[0])
	}
	return -1
}

// beqTraceToConst returns the immediate value if r traces to an OpConst.
func beqTraceToConst(f *Func, r Reg) (int64, bool) {
	def := beqFindDef(f, r)
	if def != nil && def.Op == OpConst {
		return def.Imm, true
	}
	return 0, false
}

// beqFindDef returns the Inst in f whose Dst == r, or nil.
func beqFindDef(f *Func, r Reg) *Inst {
	for _, blk := range f.Blocks {
		for _, inst := range blk.Insts {
			if inst.Dst == r {
				return inst
			}
		}
	}
	return nil
}

// ── equivalence proof ─────────────────────────────────────────────────────────

// beqTestEquivalent returns true if the original function (restored temporarily
// from origBrif) and the patched function (blk.Term == patchedJmp) produce
// identical outputs for all boundary inputs where lhs == rhs.
//
// The caller admits only pure functions with at most two u8 parameters, so
// these inputs exhaust the equality boundary under the VM's u8 domain.
func beqTestEquivalent(
	f *Func,
	blk *Block,
	gen *beqBoundaryGen,
	vm *VM,
	origBrif *TermBrIf,
	patchedJmp *TermJmp,
) bool {
	inputs := beqBoundaryInputs(gen)
	for _, args := range inputs {
		// Run patched version (blk.Term already = patchedJmp).
		patchedResult, err := vm.CallFunc(f, args)
		if err != nil {
			return false
		}

		// Run original version.
		blk.Term = origBrif
		origResult, err2 := vm.CallFunc(f, args)
		blk.Term = patchedJmp // restore patch
		if err2 != nil {
			return false
		}

		if !beqResultsEqual(origResult, patchedResult) {
			return false
		}
	}
	return true
}

// beqBoundaryInputs returns argument vectors that satisfy the equality condition.
//
// For param-vs-param (rhsIdx >= 0): iterate v over 0..255 and set both
// equality operands to v. Admission guarantees there are no other params.
//
// For param-vs-const: fix lhs param = rhsConst and sweep the one optional
// other u8 param over 0..255.
func beqBoundaryInputs(gen *beqBoundaryGen) [][]Value {
	n := gen.nParams

	if gen.rhsIdx < 0 {
		other := -1
		for i := 0; i < n; i++ {
			if i != gen.lhsIdx {
				other = i
			}
		}
		if other < 0 {
			args := make([]Value, n)
			args[gen.lhsIdx] = Value{I: gen.rhsConst}
			return [][]Value{args}
		}
		inputs := make([][]Value, 0, 256)
		for v := 0; v < 256; v++ {
			args := make([]Value, n)
			args[gen.lhsIdx] = Value{I: gen.rhsConst}
			args[other] = Value{I: int64(v)}
			inputs = append(inputs, args)
		}
		return inputs
	}

	inputs := make([][]Value, 0, 256)
	for v := 0; v < 256; v++ {
		args := make([]Value, n)
		args[gen.lhsIdx] = Value{I: int64(v)}
		args[gen.rhsIdx] = Value{I: int64(v)}
		inputs = append(inputs, args)
	}
	return inputs
}

// beqAdmittedPureU8 limits the return-only comparison to a finite, fully
// enumerated input domain and instructions without observable effects.
func beqAdmittedPureU8(f *Func) bool {
	if len(f.Contract.Params) == 0 || len(f.Contract.Params) > 2 {
		return false
	}
	for _, p := range f.Contract.Params {
		if p.Ty != TyU8 {
			return false
		}
	}
	for _, blk := range f.Blocks {
		for _, inst := range blk.Insts {
			switch inst.Op {
			case OpConst, OpMove, OpAdd, OpSub, OpMul, OpDiv, OpSDiv,
				OpMod, OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar,
				OpBitGet, OpBitSet, OpBitReset, OpNeg, OpNot,
				OpExt, OpSext, OpTrunc, OpCmp:
				// Pure scalar operations, including ones that may trap in the VM.
			default:
				return false
			}
		}
	}
	return true
}

// beqResultsEqual compares two return-value slices for equality.
func beqResultsEqual(a, b []Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].I != b[i].I {
			return false
		}
	}
	return true
}
