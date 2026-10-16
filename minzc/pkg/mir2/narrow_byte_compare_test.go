package mir2

import (
	"reflect"
	"testing"
)

func byteCompareFixture(cond CmpCond) (*Module, *Func, *Inst) {
	m := &Module{}
	f := m.AddFunc("compare")
	f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassGeneral}, {Reg: 2, Ty: TyU8, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassAcc}, {Ty: TyI16, Class: ClassGeneral}}
	cmp := &Inst{Op: OpCmp, Dst: 5, Src: [2]Reg{3, 4}, Ty: TyBool, Cond: cond, Cls: ClassAcc, SrcFile: "promoted.c", SrcLine: 7}
	f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpExt, Dst: 3, Src: [2]Reg{1}, SrcTy: TyU8, Ty: TyI16},
		{Op: OpExt, Dst: 4, Src: [2]Reg{2}, SrcTy: TyU8, Ty: TyI16}, cmp,
	}, Term: &TermRet{Vals: []Reg{5, 3}}}}
	return m, f, cmp
}
func byteCompareExpected(c CmpCond, x, y int64) bool {
	switch c {
	case CmpEq:
		return x == y
	case CmpNe:
		return x != y
	case CmpLt, CmpUlt:
		return x < y
	case CmpLe, CmpUle:
		return x <= y
	case CmpGt, CmpUgt:
		return x > y
	case CmpGe, CmpUge:
		return x >= y
	}
	panic("unknown condition")
}
func TestNarrowByteCompareAllPairs(t *testing.T) {
	for _, c := range []CmpCond{CmpEq, CmpNe, CmpLt, CmpLe, CmpGt, CmpGe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
		m, f, i := byteCompareFixture(c)
		ext := *f.Blocks[0].Insts[0]
		if !SimplifyIdentities(f) || i.Src != [2]Reg{1, 2} || i.SrcLine != 7 || i.SrcFile != "promoted.c" || !reflect.DeepEqual(ext, *f.Blocks[0].Insts[0]) {
			t.Fatal("rewrite or surviving extension", c)
		}
		if err := Verify(m); err != nil {
			t.Fatal(err)
		}
		vm := NewVM(m)
		for x := int64(0); x < 256; x++ {
			for y := int64(0); y < 256; y++ {
				out, err := vm.Call("compare", []Value{{I: x}, {I: y}})
				want := int64(0)
				if byteCompareExpected(c, x, y) {
					want = 1
				}
				if err != nil || len(out) != 2 || out[0].I != want || out[1].I != x {
					t.Fatal(c, x, y, out, err)
				}
			}
		}
	}
}
func TestNarrowByteCompareRefusals(t *testing.T) {
	for _, kind := range []string{"sign-extend", "signed-byte", "mixed-wide", "late-definition", "hard-compare", "hard-extension", "flag-return", "fallible", "asm", "redefined-source", "redefined-extension"} {
		t.Run(kind, func(t *testing.T) {
			_, f, i := byteCompareFixture(CmpLt)
			b := f.Blocks[0]
			switch kind {
			case "sign-extend":
				b.Insts[0].Op = OpSext
			case "signed-byte":
				b.Insts[0].SrcTy = TyI8
			case "mixed-wide":
				b.Insts[0].Ty = TyU16
			case "late-definition":
				b.Insts = []*Inst{i, b.Insts[0], b.Insts[1]}
			case "hard-compare":
				i.ClsHard = true
			case "hard-extension":
				b.Insts[0].ClsHard = true
			case "flag-return":
				f.Contract.Returns[0].Class = ClassFlag
			case "fallible":
				f.Attrs.FallibleCY = true
			case "asm":
				b.Insts = append(b.Insts, &Inst{Op: OpAsm, Asm: &AsmExtra{Template: "ADC A,0"}})
			case "redefined-source":
				b.Insts = append(b.Insts[:2], &Inst{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 17}, i)
			case "redefined-extension":
				b.Insts = append(b.Insts[:2], &Inst{Op: OpConst, Dst: 3, Ty: TyI16, Imm: 300}, i)
			}
			before := *i
			if narrowZeroExtendedByteComparisons(f) || !reflect.DeepEqual(before, *i) {
				t.Fatal("unsafe rewrite", kind)
			}
		})
	}
}

// The declared range is storage-identical to u8. This identity remains valid
// for the entire byte domain, and does not rely on trusting a smaller promise.
func TestNarrowByteCompareRangedSource(t *testing.T) {
	m, f, i := byteCompareFixture(CmpGt)
	ranged := NewRanged(TyU8, 0, 64)
	f.Contract.Params[0].Ty = ranged
	f.Blocks[0].Insts[0].SrcTy = ranged
	if !narrowZeroExtendedByteComparisons(f) || i.Src[0] != 1 || i.Cond != CmpUgt {
		t.Fatal("ranged source")
	}
	vm := NewVM(m)
	for x := int64(0); x < 256; x++ {
		for y := int64(0); y < 256; y++ {
			out, e := vm.Call("compare", []Value{{I: x}, {I: y}})
			want := int64(0)
			if x > y {
				want = 1
			}
			if e != nil || out[0].I != want || out[1].I != x {
				t.Fatal(x, y, out, e)
			}
		}
	}
}
