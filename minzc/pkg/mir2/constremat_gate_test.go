package mir2

import "testing"

// The rematerialisation gate's third clause: a dead constant's reserved home is
// contested not only when the function spills or the constant survives a call,
// but also when the home is what pushed a live byte into an index half.  These
// tests are the two sides of that line.

// buildFoldLoop builds the post-F5 `execute` shape of a fused fold: a counted
// loop whose body loads a byte and compares it against a loop-carried
// accumulator, with the two loop-invariant constants 0 (bound) and 1
// (decrement) live across the back edge and no call anywhere.
//
// The second return value holds the two loop-invariant constants.
func buildFoldLoop() (*Module, *Func, [2]Reg) {
	m := &Module{Name: "remat_gate_fold"}
	f := m.AddFunc("execute")
	b := NewBuilder(f)

	b.SwitchToNewBlock("entry")
	acc0 := b.Const(0, TyU8, ClassGeneral)
	base := b.AddrOf("source", ClassPointer)
	bound := b.AddrOf("n", ClassPointer)
	n := b.Load(bound, TyU16, ClassPointer)
	one := b.Const(1, TyU16, ClassIndex)
	zero := b.Const(0, TyU16, ClassIndex)
	b.Jmp("head", base, n, acc0)

	head := b.SwitchToNewBlock("head")
	p := b.BlockParam(head, TyPtr, ClassPointer)
	i := b.BlockParam(head, TyU16, ClassIndex)
	acc := b.BlockParam(head, TyU8, ClassGeneral)
	more := b.Cmp(CmpNe, i, zero, ClassFlag, false)
	b.BrIf(more, "body", nil, "exit", []Reg{acc})

	b.SwitchToNewBlock("body")
	elem := b.Load(p, TyU8, ClassGeneral)
	next := b.PtrBump(p, 1, ClassPointer)
	left := b.Sub(i, one, TyU16, ClassIndex)
	take := b.Cmp(CmpUgt, elem, acc, ClassFlag, false)
	b.BrIf(take, "head", []Reg{next, left, elem}, "head", []Reg{next, left, acc})

	exit := b.SwitchToNewBlock("exit")
	final := b.BlockParam(exit, TyU8, ClassGeneral)
	dst := b.AddrOf("result", ClassPointer)
	b.Store(dst, final, TyU8)
	b.Ret()

	if err := Verify(m); err != nil {
		panic(err)
	}
	return m, f, [2]Reg{zero, one}
}

// TestConstRematGateFreesPrimaryRegisterBlockedByDeadConstant: with the two
// dead constants reserving BC and IX in the trial allocation, the loaded byte
// has no primary register left and would take an index half.  The gate must
// recognise that, and the re-solve must both move the byte into the primary
// file and lower the modelled cost — otherwise the trial allocation stands.
func TestConstRematGateFreesPrimaryRegisterBlockedByDeadConstant(t *testing.T) {
	_, f, consts := buildFoldLoop()
	lr := ComputeLiveness(f)
	ar := PBQPAllocate(f, lr, Z80CostTable{})

	for i, c := range consts {
		if got := ar.Locs[c]; got.Kind != LocRemat {
			t.Errorf("loop constant %d (%%%d): home %v, want LocRemat — its reserved pair is what blocks B and C", i, c, got)
		}
	}
	info := collectRegInfo(f)
	for r, ri := range info {
		if ri.Ty != TyU8 || ar.Locs[r].Kind != LocIXY8 {
			continue
		}
		t.Errorf("%%%d (u8) allocated to index half %s: a primary register must be reachable once the dead constants hold no home", r, ar.Locs[r].Name)
	}
}

// TestRematDecisionAcceptsWhenBlockersReachPrimaryFile pins the post-condition
// on the fold shape: the index-half clause fires, every vreg it was justified
// with reaches the primary register file in the re-solve, and the re-solve is
// therefore the allocation that is used.  The LocReg check is the whole guard —
// a modelled-cost comparison was measured to reject nothing in 502 candidate
// re-solves across the benchmark corpus and examples/, and was dropped.
//
// This is the one test here that names the round-2 API (constRematCandidates'
// second result); the two tests around it use only PBQPAllocate, so they
// cherry-pick back to a tree without the decision.
func TestRematDecisionAcceptsWhenBlockersReachPrimaryFile(t *testing.T) {
	_, f, _ := buildFoldLoop()
	lr := ComputeLiveness(f)
	ct := Z80CostTable{}
	locs := ct.Locs()
	costs := nodeCosts(f, collectRegInfo(f), ct, locs)
	trial := pbqpSolve(f, lr, ct, nil)
	remat, blockers := constRematCandidates(f, trial, lr, ct, costs, locs)
	if len(remat) == 0 || len(blockers) == 0 {
		t.Fatalf("fold loop: remat=%v blockers=%v, want both non-empty", remat, blockers)
	}
	for v := range blockers {
		if loc := trial.Locs[v]; loc.Kind != LocIXY8 {
			t.Fatalf("blocker %%%d is at %s in the trial, want an index half: the premise must only fire on index halves", v, loc.Name)
		}
		if loc := pbqpSolve(f, lr, ct, remat).Locs[v]; loc.Kind != LocReg {
			t.Fatalf("blocker %%%d reached %s in the re-solve, want a primary register", v, loc.Name)
		}
	}
	got, first := rematDecision(f, lr, ct)
	if len(got) != len(remat) || first == nil {
		t.Fatalf("rematDecision returned (%v, %v), want the accepted candidate", got, first)
	}
	for v := range blockers {
		if first.Locs[v].Kind != LocReg {
			t.Errorf("returned allocation parks blocker %%%d in %s", v, first.Locs[v].Name)
		}
	}
}

// TestConstRematGateKeepsUncontestedConstantHome is the `hu` side of the line
// (pkg/pipeline/testdata/o8c/p1.nanz): a straight-line function with no spills
// and no calls, where rematerialising the shift-count constant merely re-ties
// the parameters — the displaced compare operand moves from BC to HL, both
// primary pairs, and the emitted code gets slower.  No vreg is pushed out of
// the primary register file, so the gate must not fire.
func TestConstRematGateKeepsUncontestedConstantHome(t *testing.T) {
	m, f, shiftCounts := buildHu()
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	lr := ComputeLiveness(f)
	ar := PBQPAllocate(f, lr, Z80CostTable{})
	for _, r := range shiftCounts {
		if got := ar.Locs[r]; got.Kind == LocRemat {
			t.Errorf("const %%%d rematerialised: no spill, no call and no live value pushed out of the primary file, so the home is free for the whole function", r)
		}
	}
}

// buildHu builds the `hu` shape — two u16 parameters, a compare, and a shift
// by a constant in each arm — and returns the two shift-count constants.
func buildHu() (*Module, *Func, [2]Reg) {
	m := &Module{Name: "remat_gate_hu"}
	f := m.AddFunc("hu")
	f.Contract.Params = []Param{{Name: "c", Ty: TyU16, Class: ClassPointer}, {Name: "a", Ty: TyU16, Class: ClassIndex}}
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	c := f.Contract.Params[0].Reg
	a := f.Contract.Params[1].Reg
	gt := b.Cmp(CmpUgt, c, a, ClassFlag, false)
	b.BrIf(gt, "then", nil, "else", nil)

	b.SwitchToNewBlock("then")
	oneT := b.Const(1, TyU16, ClassPointer)
	b.Ret(b.Add(b.Shl(a, oneT, TyU16, ClassPointer), c, TyU16, ClassPointer))

	b.SwitchToNewBlock("else")
	oneE := b.Const(1, TyU16, ClassPointer)
	b.Ret(b.Sub(b.Shl(a, oneE, TyU16, ClassPointer), c, TyU16, ClassPointer))

	return m, f, [2]Reg{oneT, oneE}
}

// TestConstRematGatePerConstantForLiveAcrossCall pins the F6 rule on the
// live-across-a-call branch: a constant whose own live range does not cover the
// call keeps its home even when a sibling's does.  Rematerialising the whole
// eligible set here would re-tie the entry block for no reason — the entry
// constant's home is not contested by anything.
func TestConstRematGatePerConstantForLiveAcrossCall(t *testing.T) {
	m := &Module{Name: "remat_gate_per_constant"}
	f := m.AddFunc("execute")
	callee := m.AddFunc("sink")
	cb := NewBuilder(callee)
	cb.SwitchToNewBlock("entry")
	cb.Ret()

	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	preamble := b.Const(7, TyU8, ClassGeneral) // dead; live range ends in this block
	seed := b.Load(b.AddrOf("buf", ClassPointer), TyU8, ClassGeneral)
	masked := b.Xor(seed, preamble, TyU8, ClassGeneral)
	b.Store(b.AddrOf("result", ClassPointer), masked, TyU8)
	base := b.AddrOf("source", ClassPointer)
	n := b.Load(b.AddrOf("n", ClassPointer), TyU16, ClassPointer)
	one := b.Const(1, TyU16, ClassIndex)
	zero := b.Const(0, TyU16, ClassIndex)
	b.Jmp("head", base, n)

	head := b.SwitchToNewBlock("head")
	p := b.BlockParam(head, TyPtr, ClassPointer)
	i := b.BlockParam(head, TyU16, ClassIndex)
	more := b.Cmp(CmpNe, i, zero, ClassFlag, false)
	b.BrIf(more, "body", nil, "exit", nil)

	b.SwitchToNewBlock("body")
	// The call sits between the loop constants' definitions and their uses, so
	// both are live across it and a callee clobber set would PUSH/POP for them.
	b.CallVoid("sink", nil, CallAttrs{})
	next := b.PtrBump(p, 1, ClassPointer)
	left := b.Sub(i, one, TyU16, ClassIndex)
	b.Jmp("head", next, left)

	b.SwitchToNewBlock("exit")
	b.Store(b.AddrOf("result", ClassPointer), b.Load(p, TyU8, ClassGeneral), TyU8)
	b.Ret()

	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	lr := ComputeLiveness(f)
	ct := Z80CostTable{}
	locs := ct.Locs()
	costs := nodeCosts(f, collectRegInfo(f), ct, locs)
	trial := pbqpSolve(f, lr, ct, nil)
	if len(trial.Spilled) != 0 {
		t.Skipf("fixture spilled; the spill clause would admit everything: %v", trial.Spilled)
	}
	eligible := eligibleRematConstants(f, trial, computeDeadConsts(f, trial))
	if !eligible[preamble] || !eligible[zero] || !eligible[one] {
		t.Fatalf("fixture eligibility wrong: want all three constants eligible, got %v", eligible)
	}
	if blockers := indexHalfBlockers(f, trial, lr, eligible, costs, locs); len(blockers) != 0 {
		t.Skipf("fixture has index-half blockers %v; the third clause would admit everything", blockers)
	}
	remat, justifiers := constRematCandidates(f, trial, lr, ct, costs, locs)
	if justifiers != nil {
		t.Fatalf("justifiers=%v, want nil: the index-half clause must not be what admits these", justifiers)
	}
	if remat[preamble] {
		t.Errorf("entry constant %%%d rematerialised: its live range ends before the call, so F6's per-constant rule keeps its home", preamble)
	}
	if !remat[zero] || !remat[one] {
		t.Errorf("loop constants zero=%%%d one=%%%d must be rematerialised (live across the call), got %v", zero, one, remat)
	}
	if got := len(remat); got != 2 {
		t.Errorf("rematerialised %d constants, want exactly the two that span the call", got)
	}
}
