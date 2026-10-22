package mir2

import "testing"

func TestAdjacentPointerWidthProof(t *testing.T) {
	for _, kind := range []string{"word", "byte", "wide", "large", "different-base", "far"} {
		t.Run(kind, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("addresses")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			var ptrTy, idxTy Ty = TyPtr, TyU16
			if kind == "byte" {
				idxTy = TyU8
			}
			if kind == "wide" {
				ptrTy = PtrFor(32)
				idxTy = TyU32
			}
			p := b.Param("p", ptrTy, ClassPointer)
			q := b.Param("q", ptrTy, ClassPointer)
			idx := b.Param("idx", idxTy, ClassPair)
			first := b.PtrAdd(p, idx, ClassPointer)
			f.Entry().Insts[len(f.Entry().Insts)-1].Ty = ptrTy
			b.Load(first, TyU8, ClassGeneral)
			step := int64(1)
			if kind == "large" {
				step = 4
			}
			k := b.Const(step, idxTy, ClassPair)
			nextIdx := b.BinOp(OpAdd, idx, k, idxTy, ClassPair)
			if kind == "far" {
				for j := 0; j < 20; j++ {
					b.Const(int64(j), TyU16, ClassPair)
				}
			}
			base := p
			if kind == "different-base" {
				base = q
			}
			next := b.PtrAdd(base, nextIdx, ClassPointer)
			f.Entry().Insts[len(f.Entry().Insts)-1].Ty = ptrTy
			x := b.Load(next, TyU8, ClassGeneral)
			b.Ret(x)
			n := ReuseAdjacentPointers(f)
			want := 0
			if kind == "word" {
				want = 1
			}
			if n != want {
				t.Fatalf("got%d want%d", n, want)
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
		})
	}
}
