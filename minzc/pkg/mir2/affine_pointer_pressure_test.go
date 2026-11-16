package mir2

import "testing"

// affineSineFixture is the shape of plasma's init_sine:
//
//	for (i: u16 = 0; i < 256; i++) sine[i] = trunc((i >> 3) & 7)
//
// The constant bound reaches the header as a loop parameter forwarded
// unchanged by the backedge, so it holds a pair for the whole loop; the index
// stays live for the body arithmetic, which also needs a word temporary.
func affineSineFixture() (*Module, *Func) {
	m := &Module{Globals: []Global{{Name: "sine", Ty: NewArray(TyU8, 256)}}}
	f := m.AddFunc("init_sine")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	zero := b.Const(0, TyU16, ClassPointer)
	bound := b.Const(256, TyU16, ClassPointer)
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	n := b.BlockParam(head, TyU16, ClassPointer)
	i := b.BlockParam(head, TyU16, ClassPointer)
	b.Jmp("head", bound, zero)
	b.SwitchTo(head)
	cond := b.CmpWithSrcTy(CmpUlt, i, n, ClassFlag, false, TyU16)
	head.Term = &TermCondRet{Cond: cond, Then: "body"}
	b.SwitchTo(body)
	sh := b.Shr(i, b.Const(3, TyU16, ClassPointer), TyU16, ClassPointer)
	v := b.And(sh, b.Const(7, TyU16, ClassPointer), TyU16, ClassPointer)
	s := b.Trunc(v, TyU16, TyU8, ClassGeneral)
	p := b.PtrAdd(b.AddrOf("sine", ClassPointer), i, ClassPointer)
	b.Store(p, s, TyU8)
	next := b.Add(i, b.Const(1, TyU16, ClassPointer), TyU16, ClassPointer)
	b.Jmp("head", n, next)
	return m, f
}

// affineAttrFixture is the shape of tetris's pixel_actor_attributes: a byte
// index (no counter: the body reads it), two walks taking HL and DE and a
// word temporary. withAcc adds a byte accumulator, the second byte of loop
// state that no longer fits beside them.
func affineAttrFixture(withAcc bool) (*Module, *Func) {
	m := &Module{Globals: []Global{{Name: "src", Ty: NewArray(TyU8, 16)}, {Name: "dst", Ty: NewArray(TyU8, 16)}}}
	f := m.AddFunc("attrs")
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	zero := b.Const(0, TyU8, ClassGeneral)
	head := f.NewBlock("head")
	body := f.NewBlock("body")
	exit := f.NewBlock("exit")
	i := b.BlockParam(head, TyU8, ClassGeneral)
	var acc Reg
	if withAcc {
		acc = b.BlockParam(head, TyU8, ClassGeneral)
		b.Jmp("head", zero, zero)
	} else {
		b.Jmp("head", zero)
	}
	b.SwitchTo(head)
	cond := b.CmpWithSrcTy(CmpUlt, i, b.Const(8, TyU8, ClassGeneral), ClassFlag, false, TyU8)
	b.BrIf(cond, "body", nil, "exit", nil)
	b.SwitchTo(body)
	wi := b.Ext(i, TyU8, TyU16, ClassPointer)
	x := b.Load(b.PtrAdd(b.AddrOf("src", ClassPointer), wi, ClassPointer), TyU8, ClassGeneral)
	w := b.Shl(wi, b.Const(3, TyU16, ClassPointer), TyU16, ClassPointer)
	y := b.Add(x, b.Trunc(w, TyU16, TyU8, ClassGeneral), TyU8, ClassGeneral)
	b.Store(b.PtrAdd(b.AddrOf("dst", ClassPointer), wi, ClassPointer), y, TyU8)
	next := b.Add(i, b.Const(1, TyU8, ClassGeneral), TyU8, ClassGeneral)
	if withAcc {
		b.Jmp("head", next, b.Add(acc, x, TyU8, ClassGeneral))
	} else {
		b.Jmp("head", next)
	}
	b.SwitchTo(exit)
	if withAcc {
		b.Ret(acc)
	} else {
		b.Ret(zero)
	}
	return m, f
}

// The walk must not fire when the state it leaves does not fit HL/DE/BC plus
// A: a constant bound held as a header parameter is a pair, and byte loop
// state counts against the byte budget. Benchmark-shaped loops still walk.
func TestAffinePointerWalkPressureGate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func() (*Module, *Func)
		walk  bool
	}{
		// walk + index + constant bound = 3 pairs (peak 7 -> 9 live bytes).
		{"init_sine", affineSineFixture, false},
		// 2 walks + word temporary + byte index + byte accumulator = 8 bytes.
		{"attrs+acc", func() (*Module, *Func) { return affineAttrFixture(true) }, false},
		// The same loop without the accumulator is 7 bytes and fits.
		{"attrs", func() (*Module, *Func) { return affineAttrFixture(false) }, true},
		// fold_max: walk + u16 counter (BC) + byte maximum.
		{"fold_max", func() (*Module, *Func) { return affineFoldFixture("") }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, f := tc.build()
			if err := Verify(m); err != nil {
				t.Fatal(err, m.Dump())
			}
			before := m.Dump()
			n := StrengthReduceCountedPointers(f)
			if err := Verify(m); err != nil {
				t.Fatal(err, m.Dump())
			}
			if tc.walk != (n > 0) {
				t.Fatalf("rewrites=%d, want walk=%v\n%s", n, tc.walk, m.Dump())
			}
			if !tc.walk && m.Dump() != before {
				t.Fatalf("refusal mutated the function\n%s", m.Dump())
			}
		})
	}
}
