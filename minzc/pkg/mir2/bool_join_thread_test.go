package mir2

import (
	"strings"
	"testing"
)

// boolJoinShape builds the C/Nanz lowering of `if (x > 10 && x < 200)` with
// a two-valued result, then lets each test perturb one proof condition.
//
//	entry: c1 = cmp.ugt x, 10 ; br_if c1, @rhs, @no
//	rhs:   c2 = cmp.ult x, 200 ; jmp @J(c2, x)
//	no:    z  = const 0 : bool ; jmp @J(z, x)
//	J(p: bool, q: u8): z0 = const 0 ; t = cmp.ne p, z0 ; br_if t, @T(q), @F()
//	T(v: u8): ret v + 1      F: ret 7
type boolJoinShape struct {
	m                      *Module
	f                      *Func
	entry, rhs, no, j      *Block
	tBlk, fBlk             *Block
	x, c2, zero, p, q, cmp Reg
}

func newBoolJoinShape(t *testing.T) *boolJoinShape {
	t.Helper()
	s := &boolJoinShape{m: &Module{}}
	s.f = s.m.AddFunc("bj")
	b := NewBuilder(s.f)
	s.entry = b.SwitchToNewBlock("entry")
	s.x = b.Param("x", TyU8, ClassAcc)
	ten := b.Const(10, TyU8, ClassGeneral)
	c1 := b.Cmp(CmpUgt, s.x, ten, ClassFlag, false)
	b.BrIf(c1, "rhs", nil, "no", nil)
	s.rhs = b.SwitchToNewBlock("rhs")
	k := b.Const(200, TyU8, ClassGeneral)
	s.c2 = b.Cmp(CmpUlt, s.x, k, ClassFlag, false)
	b.Jmp("J", s.c2, s.x)
	s.no = b.SwitchToNewBlock("no")
	s.zero = b.Const(0, TyBool, ClassGeneral)
	b.Jmp("J", s.zero, s.x)
	s.j = b.SwitchToNewBlock("J")
	s.p = b.BlockParam(s.j, TyBool, ClassGeneral)
	s.q = b.BlockParam(s.j, TyU8, ClassGeneral)
	z0 := b.Const(0, TyBool, ClassGeneral)
	s.cmp = b.Cmp(CmpNe, s.p, z0, ClassFlag, false)
	b.BrIf(s.cmp, "T", []Reg{s.q}, "F", nil)
	s.tBlk = b.SwitchToNewBlock("T")
	v := b.BlockParam(s.tBlk, TyU8, ClassGeneral)
	one := b.Const(1, TyU8, ClassGeneral)
	b.Ret(b.Add(v, one, TyU8, ClassGeneral))
	s.fBlk = b.SwitchToNewBlock("F")
	b.Ret(b.Const(7, TyU8, ClassGeneral))
	s.f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	return s
}

// results runs the function on every byte.
func boolJoinResults(t *testing.T, m *Module, name string) []int64 {
	t.Helper()
	if err := Verify(m); err != nil {
		t.Fatalf("verify: %v\n%s", err, m.Dump())
	}
	vm := NewVM(m)
	out := make([]int64, 256)
	for x := 0; x < 256; x++ {
		got, err := vm.Call(name, []Value{{I: int64(x)}})
		if err != nil || len(got) != 1 {
			t.Fatalf("%s(%d): %v %v", name, x, got, err)
		}
		out[x] = got[0].I
	}
	return out
}

// checkThread runs the pass and asserts the expected outcome and unchanged
// behaviour over the whole u8 domain.
func checkThread(t *testing.T, m *Module, f *Func, wantChange bool) {
	t.Helper()
	before := boolJoinResults(t, m, f.Name)
	dump := m.Dump()
	if got := ThreadBoolJoins(f); got != wantChange {
		t.Fatalf("ThreadBoolJoins = %v, want %v\nbefore:\n%s\nafter:\n%s", got, wantChange, dump, m.Dump())
	}
	after := boolJoinResults(t, m, f.Name)
	for x := range before {
		if before[x] != after[x] {
			t.Fatalf("x=%d: %d before, %d after\nbefore:\n%s\nafter:\n%s", x, before[x], after[x], dump, m.Dump())
		}
	}
	if !wantChange && m.Dump() != dump {
		t.Fatalf("refused but changed:\n%s", m.Dump())
	}
}

func TestThreadBoolJoinAndNe(t *testing.T) {
	s := newBoolJoinShape(t)
	checkThread(t, s.m, s.f, true)
	if s.f.BlockByLabel("J") != nil {
		t.Fatalf("join kept:\n%s", s.m.Dump())
	}
	br, ok := s.rhs.Term.(*TermBrIf)
	if !ok || br.Cond != s.c2 || br.Then != "T" || br.Else != "F" || len(br.ThenArgs) != 1 || br.ThenArgs[0] != s.x {
		t.Fatalf("rhs term %s", TermString(s.rhs.Term))
	}
	if j, ok := s.no.Term.(*TermJmp); !ok || j.Target != "F" || len(j.Args) != 0 {
		t.Fatalf("no term %s", TermString(s.no.Term))
	}
	if ThreadBoolJoins(s.f) {
		t.Fatal("not idempotent")
	}
}

func TestThreadBoolJoinEqInverts(t *testing.T) {
	s := newBoolJoinShape(t)
	s.j.Insts[1].Cond = CmpEq
	checkThread(t, s.m, s.f, true)
	if br := s.rhs.Term.(*TermBrIf); br.Then != "F" || br.Else != "T" {
		t.Fatalf("rhs term %s", TermString(s.rhs.Term))
	}
	if j := s.no.Term.(*TermJmp); j.Target != "T" || len(j.Args) != 1 || j.Args[0] != s.x {
		t.Fatalf("no term %s", TermString(s.no.Term))
	}
}

func TestThreadBoolJoinZeroOnLeft(t *testing.T) {
	s := newBoolJoinShape(t)
	c := s.j.Insts[1]
	c.Src[0], c.Src[1] = c.Src[1], c.Src[0]
	checkThread(t, s.m, s.f, true)
}

// Nanz lowers `if a && b` to br_if on the join parameter itself.
func TestThreadBoolJoinDirectParam(t *testing.T) {
	s := newBoolJoinShape(t)
	s.j.Insts = nil
	s.j.Term.(*TermBrIf).Cond = s.p
	checkThread(t, s.m, s.f, true)
}

// `x = a && b; if (x)` with x otherwise dead: ext then cmp.
func TestThreadBoolJoinThroughExt(t *testing.T) {
	s := newBoolJoinShape(t)
	e := &Inst{Op: OpExt, Dst: s.f.AllocReg(), Src: [2]Reg{s.p}, Ty: TyU8, SrcTy: TyBool, Cls: ClassGeneral}
	z := &Inst{Op: OpConst, Dst: s.f.AllocReg(), Ty: TyU8, Cls: ClassGeneral}
	c := s.j.Insts[1]
	c.Src = [2]Reg{e.Dst, z.Dst}
	s.j.Insts = []*Inst{e, z, c}
	checkThread(t, s.m, s.f, true)
}

// An ext that drops bits of the parameter can turn a non-zero value into 0.
func TestThreadBoolJoinRefusesNarrowingExt(t *testing.T) {
	s := newBoolJoinShape(t)
	s.j.Params[0].Ty = TyU16
	// 256 reaches J; its low byte is zero.
	s.no.Insts[0].Imm, s.no.Insts[0].Ty = 256, TyU16
	e := &Inst{Op: OpExt, Dst: s.f.AllocReg(), Src: [2]Reg{s.p}, Ty: TyU8, Cls: ClassGeneral}
	c := s.j.Insts[1]
	c.Src[0] = e.Dst
	s.j.Insts = []*Inst{e, s.j.Insts[0], c}
	checkThread(t, s.m, s.f, false)
}

// The predecessor's constant is truncated by the edge to the parameter type.
func TestThreadBoolJoinMasksConstantToParam(t *testing.T) {
	s := newBoolJoinShape(t)
	s.j.Params[0].Ty = TyU8
	s.no.Insts[0].Imm, s.no.Insts[0].Ty = 256, TyU16
	checkThread(t, s.m, s.f, true)
	if j := s.no.Term.(*TermJmp); j.Target != "F" {
		t.Fatalf("256 into u8 is false; got %s", TermString(s.no.Term))
	}
}

// The value is also used after the branch: it must stay materialised.
func TestThreadBoolJoinRefusesParamUsedElsewhere(t *testing.T) {
	s := newBoolJoinShape(t)
	b := NewBuilder(s.f)
	b.SwitchTo(s.fBlk)
	s.fBlk.Insts = nil
	s.fBlk.Term = nil
	b.Ret(b.Ext(s.p, TyBool, TyU8, ClassGeneral))
	checkThread(t, s.m, s.f, false)
}

func TestThreadBoolJoinRefusesParamPassedOn(t *testing.T) {
	s := newBoolJoinShape(t)
	br := s.j.Term.(*TermBrIf)
	br.ThenArgs = []Reg{s.p}
	checkThread(t, s.m, s.f, false)
}

// A test-chain value used outside J would lose its definition.
func TestThreadBoolJoinRefusesChainUsedElsewhere(t *testing.T) {
	s := newBoolJoinShape(t)
	b := NewBuilder(s.f)
	b.SwitchTo(s.fBlk)
	s.fBlk.Insts = nil
	s.fBlk.Term = nil
	b.Ret(b.Ext(s.cmp, TyBool, TyU8, ClassGeneral))
	checkThread(t, s.m, s.f, false)
}

// The other parameter read directly (not via J's edge arguments) below J.
func TestThreadBoolJoinRefusesOtherParamUsedBelow(t *testing.T) {
	s := newBoolJoinShape(t)
	s.j.Term.(*TermBrIf).ThenArgs = []Reg{s.x}
	b := NewBuilder(s.f)
	b.SwitchTo(s.fBlk)
	s.fBlk.Insts = nil
	s.fBlk.Term = nil
	b.Ret(s.q)
	checkThread(t, s.m, s.f, false)
}

func TestThreadBoolJoinRefusesEffect(t *testing.T) {
	s := newBoolJoinShape(t)
	s.m.Globals = append(s.m.Globals, Global{Name: "g", Ty: TyU8})
	addr := &Inst{Op: OpAddrOf, Dst: s.f.AllocReg(), Sym: "g", Ty: TyPtr, Cls: ClassPointer}
	st := &Inst{Op: OpStore, Src: [2]Reg{addr.Dst, s.x}, Ty: TyU8}
	s.j.Insts = append([]*Inst{addr, st}, s.j.Insts...)
	checkThread(t, s.m, s.f, false)
}

// A pure instruction outside the test chain (its value used on an edge).
func TestThreadBoolJoinRefusesOtherInstruction(t *testing.T) {
	s := newBoolJoinShape(t)
	add := &Inst{Op: OpAdd, Dst: s.f.AllocReg(), Src: [2]Reg{s.q, s.q}, Ty: TyU8, Cls: ClassGeneral}
	s.j.Insts = append([]*Inst{add}, s.j.Insts...)
	s.j.Term.(*TermBrIf).ThenArgs = []Reg{add.Dst}
	checkThread(t, s.m, s.f, false)
}

// The test is against a non-zero constant: not a truth test.
func TestThreadBoolJoinRefusesNonZeroTest(t *testing.T) {
	s := newBoolJoinShape(t)
	s.j.Insts[0].Imm = 1
	checkThread(t, s.m, s.f, false)
}

func TestThreadBoolJoinRefusesOrderingTest(t *testing.T) {
	s := newBoolJoinShape(t)
	s.j.Insts[1].Cond = CmpUgt
	checkThread(t, s.m, s.f, false)
}

// A conditional predecessor is left alone; the jmp predecessors are threaded
// and J stays for the conditional edge.
func TestThreadBoolJoinKeepsConditionalPredecessor(t *testing.T) {
	s := newBoolJoinShape(t)
	// entry: br_if c1, @rhs, @J(zero, x) — the no-block folded into the edge.
	s.entry.Insts = append(s.entry.Insts, s.no.Insts...)
	br := s.entry.Term.(*TermBrIf)
	br.Else, br.ElseArgs = "J", []Reg{s.zero, s.x}
	s.f.removeBlock("no")
	checkThread(t, s.m, s.f, true)
	if s.f.BlockByLabel("J") == nil {
		t.Fatal("J removed while a conditional edge still reaches it")
	}
	if br := s.entry.Term.(*TermBrIf); br.Else != "J" {
		t.Fatalf("conditional predecessor rewritten: %s", TermString(s.entry.Term))
	}
}

// The predecessor computes the comparison and then does more work: the
// flags would not belong to that comparison at the branch. That predecessor
// stays; the constant one is threaded and J survives for it.
func TestThreadBoolJoinRefusesCmpNotLast(t *testing.T) {
	s := newBoolJoinShape(t)
	extra := &Inst{Op: OpAdd, Dst: s.f.AllocReg(), Src: [2]Reg{s.x, s.x}, Ty: TyU8, Cls: ClassGeneral}
	s.rhs.Insts = append(s.rhs.Insts, extra)
	s.rhs.Term.(*TermJmp).Args[1] = extra.Dst
	checkThread(t, s.m, s.f, true)
	if j, ok := s.rhs.Term.(*TermJmp); !ok || j.Target != "J" {
		t.Fatalf("rhs rewritten: %s", TermString(s.rhs.Term))
	}
	if s.f.BlockByLabel("J") == nil {
		t.Fatal("J removed")
	}
}

// The argument is not a constant and not a comparison of the predecessor.
func TestThreadBoolJoinRefusesOpaqueArgument(t *testing.T) {
	s := newBoolJoinShape(t)
	tr := &Inst{Op: OpTrunc, Dst: s.f.AllocReg(), Src: [2]Reg{s.x}, Ty: TyBool, SrcTy: TyU8, Cls: ClassGeneral}
	s.rhs.Insts = append(s.rhs.Insts, tr)
	s.rhs.Term.(*TermJmp).Args[0] = tr.Dst
	s.no.Insts = []*Inst{{Op: OpTrunc, Dst: s.zero, Src: [2]Reg{s.x}, Ty: TyBool, SrcTy: TyU8, Cls: ClassGeneral}}
	checkThread(t, s.m, s.f, false)
}

// J heads a loop: a predecessor is dominated by J.
//
//	entry: jmp @J(1, x)
//	J(p, q): br_if p, @T(q), @F()
//	T(v): c = cmp.ult v, 200 ; v1 = v + 1 ... ; jmp @J(c, v1)   (back edge)
func TestThreadBoolJoinRefusesLoopHeader(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("loop")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassAcc)
	one := b.Const(1, TyBool, ClassGeneral)
	b.Jmp("J", one, x)
	j := b.SwitchToNewBlock("J")
	p := b.BlockParam(j, TyBool, ClassGeneral)
	q := b.BlockParam(j, TyU8, ClassGeneral)
	b.BrIf(p, "T", []Reg{q}, "F", []Reg{q})
	tb := b.SwitchToNewBlock("T")
	v := b.BlockParam(tb, TyU8, ClassGeneral)
	k := b.Const(1, TyU8, ClassGeneral)
	v1 := b.Add(v, k, TyU8, ClassGeneral)
	lim := b.Const(200, TyU8, ClassGeneral)
	c := b.Cmp(CmpUlt, v1, lim, ClassFlag, false)
	b.Jmp("J", c, v1)
	fb := b.SwitchToNewBlock("F")
	r := b.BlockParam(fb, TyU8, ClassGeneral)
	b.Ret(r)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	checkThread(t, m, f, false)
}

// J's br_if returns to a block that dominates J (J is a latch).
func TestThreadBoolJoinRefusesLatch(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("latch")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU8, ClassAcc)
	b.Jmp("H", x)
	h := b.SwitchToNewBlock("H")
	v := b.BlockParam(h, TyU8, ClassGeneral)
	k := b.Const(3, TyU8, ClassGeneral)
	v1 := b.Add(v, k, TyU8, ClassGeneral)
	ten := b.Const(10, TyU8, ClassGeneral)
	c1 := b.Cmp(CmpUgt, v1, ten, ClassFlag, false)
	b.BrIf(c1, "rhs", nil, "no", nil)
	b.SwitchToNewBlock("rhs")
	lim := b.Const(200, TyU8, ClassGeneral)
	c2 := b.Cmp(CmpUlt, v1, lim, ClassFlag, false)
	b.Jmp("J", c2)
	b.SwitchToNewBlock("no")
	z := b.Const(0, TyBool, ClassGeneral)
	b.Jmp("J", z)
	j := b.SwitchToNewBlock("J")
	p := b.BlockParam(j, TyBool, ClassGeneral)
	b.BrIf(p, "H", []Reg{v1}, "X", nil)
	b.SwitchToNewBlock("X")
	b.Ret(v1)
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	checkThread(t, m, f, false)
}

func TestThreadBoolJoinRefusesOtherTerminators(t *testing.T) {
	t.Run("cond_ret", func(t *testing.T) {
		s := newBoolJoinShape(t)
		s.j.Term = &TermCondRet{Cond: s.cmp, Vals: []Reg{s.q}, Then: "T", ThenArgs: []Reg{s.q}}
		checkThread(t, s.m, s.f, false)
	})
	t.Run("br_if2", func(t *testing.T) {
		s := newBoolJoinShape(t)
		s.j.Insts = s.j.Insts[:1]
		s.j.Term = &TermBrIf2{Lhs: s.p, Rhs: s.j.Insts[0].Dst, Eq: "F", Lt: "F", Gt: "T", GtArgs: []Reg{s.q}}
		checkThread(t, s.m, s.f, false)
	})
	t.Run("djnz", func(t *testing.T) {
		s := newBoolJoinShape(t)
		s.j.Insts = nil
		s.tBlk.Params = append([]BlockParam{{Dst: s.f.AllocReg(), Ty: TyU8, Class: ClassCounter}}, s.tBlk.Params...)
		s.j.Term = &TermDJNZ{Counter: s.q, Body: "T", BodyArgs: []Reg{s.p}, Exit: "F"}
		if ThreadBoolJoins(s.f) {
			t.Fatal("threaded a DJNZ join")
		}
	})
}

func TestThreadBoolJoinRefusesFunctionShapes(t *testing.T) {
	t.Run("fallible", func(t *testing.T) {
		s := newBoolJoinShape(t)
		s.f.Attrs.FallibleCY = true
		if ThreadBoolJoins(s.f) {
			t.Fatal("threaded a fallible function")
		}
	})
	t.Run("asm", func(t *testing.T) {
		s := newBoolJoinShape(t)
		s.tBlk.Insts = append([]*Inst{{Op: OpAsm, Sym: "NOP"}}, s.tBlk.Insts...)
		if ThreadBoolJoins(s.f) {
			t.Fatal("threaded a function with inline asm")
		}
	})
	t.Run("loop_origin", func(t *testing.T) {
		s := newBoolJoinShape(t)
		s.f.LoopOrigins = map[string]LoopOrigin{"J": {SiteID: "1", Role: "body"}}
		checkThread(t, s.m, s.f, false)
	})
	t.Run("static_loop", func(t *testing.T) {
		s := newBoolJoinShape(t)
		s.f.StaticLoops = map[string]StaticLoop{"J": {ID: 1, Trips: -1}}
		checkThread(t, s.m, s.f, false)
	})
}

// A flag-returning function: the returned flag is computed below the join
// and is untouched by the rewrite.
func TestThreadBoolJoinFlagReturn(t *testing.T) {
	s := newBoolJoinShape(t)
	b := NewBuilder(s.f)
	b.SwitchTo(s.tBlk)
	s.tBlk.Insts = nil
	s.tBlk.Term = nil
	lim := b.Const(100, TyU8, ClassGeneral)
	b.Ret(b.Cmp(CmpUlt, s.tBlk.Params[0].Dst, lim, ClassFlag, false))
	b.SwitchTo(s.fBlk)
	s.fBlk.Insts = nil
	s.fBlk.Term = nil
	b.Ret(b.Const(0, TyBool, ClassGeneral))
	s.f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassFlag, FlagCond: CmpUlt}}
	checkThread(t, s.m, s.f, true)
}

// Three-way `a && b && c`: the inner join's result feeds an outer join; the
// pass reaches a fixpoint with both removed.
func TestThreadBoolJoinChained(t *testing.T) {
	s := newBoolJoinShape(t)
	// Retarget J's branch into a second join: T computes c, F passes 0.
	b := NewBuilder(s.f)
	s.j.Term.(*TermBrIf).Then, s.j.Term.(*TermBrIf).ThenArgs = "c3", []Reg{s.q}
	s.j.Term.(*TermBrIf).Else, s.j.Term.(*TermBrIf).ElseArgs = "c0", nil
	c3 := b.SwitchToNewBlock("c3")
	w := b.BlockParam(c3, TyU8, ClassGeneral)
	fifty := b.Const(50, TyU8, ClassGeneral)
	b.Jmp("J2", b.Cmp(CmpNe, w, fifty, ClassFlag, false))
	b.SwitchToNewBlock("c0")
	b.Jmp("J2", b.Const(0, TyBool, ClassGeneral))
	j2 := b.SwitchToNewBlock("J2")
	p2 := b.BlockParam(j2, TyBool, ClassGeneral)
	z := b.Const(0, TyBool, ClassGeneral)
	b.BrIf(b.Cmp(CmpNe, p2, z, ClassFlag, false), "T", []Reg{s.x}, "F", nil)
	checkThread(t, s.m, s.f, true)
	if strings.Contains(s.m.Dump(), "@J(") || strings.Contains(s.m.Dump(), "@J2(") {
		t.Fatalf("join left:\n%s", s.m.Dump())
	}
}

// Dead pure instructions left by constant folding (the C frontend's
// `cmp.ne %c, 0` copy after the comparison, a dead use of %p in J) are
// ignored as DeadStoreElim would remove them.
func TestThreadBoolJoinIgnoresDeadCopies(t *testing.T) {
	s := newBoolJoinShape(t)
	z := &Inst{Op: OpConst, Dst: s.f.AllocReg(), Ty: TyBool, Cls: ClassGeneral}
	copyCmp := &Inst{Op: OpCmp, Cond: CmpNe, Dst: s.f.AllocReg(), Src: [2]Reg{s.c2, z.Dst}, Ty: TyBool, Cls: ClassFlag}
	s.rhs.Insts = append(s.rhs.Insts, z, copyCmp)
	deadExt := &Inst{Op: OpExt, Dst: s.f.AllocReg(), Src: [2]Reg{s.p}, Ty: TyU8, SrcTy: TyBool, Cls: ClassGeneral}
	s.j.Insts = append([]*Inst{deadExt}, s.j.Insts...)
	checkThread(t, s.m, s.f, true)
	if br, ok := s.rhs.Term.(*TermBrIf); !ok || br.Cond != s.c2 {
		t.Fatalf("rhs term %s", TermString(s.rhs.Term))
	}
}
