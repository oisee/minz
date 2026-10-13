package mir2

import "testing"

func typedOffsetFusionModule(hi int64, ty Ty) *Module {
	m := &Module{}
	f := m.AddFunc("word")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.Param("p", TyPtr, ClassPointer)
	loP := b.PtrAdd(p, b.Const(255, TyU8, ClassGeneral), ClassPointer)
	hiP := b.PtrAdd(p, b.Const(hi, ty, ClassGeneral), ClassPointer)
	lo := b.Ext(b.Load(loP, TyU8, ClassGeneral), TyU8, TyU16, ClassPair)
	h := b.Ext(b.Load(hiP, TyU8, ClassGeneral), TyU8, TyU16, ClassPair)
	eight := b.Const(8, TyU8, ClassGeneral)
	b.Ret(b.Or(lo, b.Shl(h, eight, TyU16, ClassPair), TyU16, ClassPair))
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPair}}
	return m
}

func TestFusionIndependentTypedOffsetCounterexample(t *testing.T) {
	for _, tc := range []struct {
		hi        int64
		ty        Ty
		wantFused int
	}{{256, TyU8, 0}, {256, TyU16, 1}, {65792, TyU16, 1}, {-65280, TyU16, 1}} {
		for _, base := range []int64{0, 65280, 65535} {
			m := typedOffsetFusionModule(tc.hi, tc.ty)
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			low := uint16(base + 255)
			highOffset := uint16(tc.hi)
			if tc.ty == TyU8 {
				highOffset = uint16(uint8(tc.hi))
			}
			high := uint16(base + int64(highOffset))
			execute := func(m *Module) int64 {
				vm := NewVM(m)
				vm.EnsureHeap(65536)
				vm.WriteHeap(int64(low), 0x34)
				vm.WriteHeap(int64(high), 0x12)
				r, e := vm.Call("word", []Value{{I: base}})
				if e != nil {
					t.Fatal(e)
				}
				return r[0].I
			}
			if got := execute(m); got != 0x1234 {
				t.Fatal("raw", got)
			}
			if n := FuseSequentialByteWords(m.Funcs[0]); n != tc.wantFused {
				t.Fatalf("hi=%d ty=%s fusion=%d", tc.hi, tc.ty, n)
			}
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			if got := execute(m); got != 0x1234 {
				t.Fatalf("typed constant truncation lost hi=%d ty=%s base=%d got=%x", tc.hi, tc.ty, base, got)
			}
		}
	}
	// Independently constructed forged metadata must not bypass typed adjacency.
	m := typedOffsetFusionModule(256, TyU16)
	FuseSequentialByteWords(m.Funcs[0])
	var wide *Inst
	for _, i := range m.Funcs[0].Blocks[0].Insts {
		if i.SequentialBytes {
			wide = i
		}
	}
	if wide == nil {
		t.Fatal("not fused")
	}
	for _, d := range wide.SequentialAddressDefs {
		if d.Op == OpConst && d.Imm == 256 {
			d.Ty = TyU8

		}
	}

	if e := Verify(m); e == nil {
		t.Fatal("typed-invalid adjacency metadata accepted")
	}
}

func TestFusionConstantsUseStoredBits(t *testing.T) {
	for _, store := range []bool{false, true} {
		m := wordFusionFixture(store)
		f := m.Funcs[0]
		for _, i := range f.Blocks[0].Insts {
			if i.Op == OpConst && i.Imm == 8 {
				i.Ty = TyU8
				i.Imm = 264
			}
		}
		if n := FuseSequentialByteWords(f); n != 1 {
			t.Fatal("stored shift count8 not matched", n)
		}
		if e := Verify(m); e != nil {
			t.Fatal(e)
		}
		vm := NewVM(m)
		p := vm.AllocHeap([]byte{0x34, 0x12}).I
		args := []Value{{I: p}}
		if store {
			args = append(args, Value{I: 0xa55a})
		}
		r, e := vm.Call("word", args)
		if e != nil {
			t.Fatal(e)
		}
		if !store && r[0].I != 0x1234 {
			t.Fatal(r)
		}
		if store {
			v := vm.ReadHeap(p, 2)
			if v[0] != 0x5a || v[1] != 0xa5 {
				t.Fatal(v)
			}
		}
	}
}
