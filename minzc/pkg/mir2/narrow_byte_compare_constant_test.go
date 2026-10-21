package mir2

import (
	"fmt"
	"reflect"
	"testing"
)

func byteConstantCompareFixture(c CmpCond, k int64, reverse bool, ty Ty) (*Module, *Func, *Inst) {
	m := &Module{}
	f := m.AddFunc("compare")
	x, ext, constant, result := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
	f.Contract.Params = []Param{{Reg: x, Ty: TyU8, Class: ClassGeneral}}
	f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassAcc}, {Ty: ty, Class: ClassGeneral}, {Ty: ty, Class: ClassGeneral}}
	src := [2]Reg{ext, constant}
	if reverse {
		src = [2]Reg{constant, ext}
	}
	cmp := &Inst{Op: OpCmp, Dst: result, Src: src, Ty: TyBool, SrcTy: ty, Cond: c, Cls: ClassAcc, SrcFile: "constant.c", SrcLine: 9}
	f.Blocks = []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpExt, Dst: ext, Src: [2]Reg{x}, SrcTy: TyU8, Ty: ty},
		{Op: OpConst, Dst: constant, Ty: ty, Imm: k}, cmp,
	}, Term: &TermRet{Vals: []Reg{result, ext, constant}}}}
	return m, f, cmp
}

// Every byte input, both operand orders, all signed/unsigned predicates, and
// constants around byte, sign and word boundaries. The VM before the rewrite
// is an independent reference, including word truncation and sign extension.
func TestNarrowByteCompareConstants(t *testing.T) {
	for _, ty := range []Ty{TyI16, TyU16} {
		for _, cond := range []CmpCond{CmpEq, CmpNe, CmpLt, CmpLe, CmpGt, CmpGe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
			for _, k := range []int64{-65536, -32769, -32768, -256, -5, -1, 0, 1, 8, 127, 128, 254, 255, 256, 300, 32767, 32768, 65535, 65536, 65537} {
				for _, reverse := range []bool{false, true} {
					t.Run(fmt.Sprintf("%v/%d/%d/%t", ty, cond, k, reverse), func(t *testing.T) {
						before, _, _ := byteConstantCompareFixture(cond, k, reverse, ty)
						m, f, cmp := byteConstantCompareFixture(cond, k, reverse, ty)
						ext, constant := *f.Blocks[0].Insts[0], *f.Blocks[0].Insts[1]
						if !narrowZeroExtendedByteComparisons(f) {
							t.Fatal("comparison was not narrowed or folded")
						}
						if !reflect.DeepEqual(ext, *f.Blocks[0].Insts[0]) || !reflect.DeepEqual(constant, *f.Blocks[0].Insts[1]) {
							t.Fatal("changed shared operand")
						}
						if cmp.SrcFile != "constant.c" || cmp.SrcLine != 9 {
							t.Fatal("lost source location")
						}
						if k&65535 <= 255 {
							if cmp.Op != OpCmp || cmp.SrcTy != TyU8 || cmp.Cond == CmpLt || cmp.Cond == CmpLe || cmp.Cond == CmpGt || cmp.Cond == CmpGe {
								t.Fatalf("not an unsigned byte compare: %+v", cmp)
							}
						} else if cmp.Op != OpConst || cmp.Ty != TyBool {
							t.Fatalf("not folded: %+v", cmp)
						}
						if err := Verify(m); err != nil {
							t.Fatal(err)
						}
						a, b := NewVM(before), NewVM(m)
						for x := int64(0); x < 256; x++ {
							want, e1 := a.Call("compare", []Value{{I: x}})
							got, e2 := b.Call("compare", []Value{{I: x}})
							if e1 != nil || e2 != nil || !reflect.DeepEqual(want, got) {
								t.Fatalf("x=%d got=%v want=%v errors=%v/%v", x, got, want, e1, e2)
							}
						}
					})
				}
			}
		}
	}
}

func TestNarrowByteCompareConstantRefusals(t *testing.T) {
	for _, kind := range []string{"sext", "signed-byte", "hard-constant", "hard-compare", "hard-ext", "redefined-constant", "redefined-source", "redefined-ext", "late-constant", "byte-constant", "wide-param", "flags-only", "unknown-predicate", "wrong-width", "missing-width"} {
		t.Run(kind, func(t *testing.T) {
			_, f, cmp := byteConstantCompareFixture(CmpLt, 8, false, TyI16)
			b := f.Blocks[0]
			switch kind {
			case "sext":
				b.Insts[0].Op = OpSext
			case "signed-byte":
				b.Insts[0].SrcTy = TyI8
			case "hard-constant":
				b.Insts[1].ClsHard = true
			case "hard-compare":
				cmp.ClsHard = true
			case "hard-ext":
				b.Insts[0].ClsHard = true
			case "redefined-constant":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: cmp.Src[1], Ty: TyI16, Imm: 300})
			case "redefined-source":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: f.Contract.Params[0].Reg, Ty: TyU8, Imm: 7})
			case "redefined-ext":
				b.Insts = append(b.Insts, &Inst{Op: OpConst, Dst: cmp.Src[0], Ty: TyI16, Imm: 300})
			case "late-constant":
				b.Insts = []*Inst{b.Insts[0], cmp, b.Insts[1]}
			case "byte-constant":
				b.Insts[1].Ty = TyU8
			case "wide-param":
				b.Insts = b.Insts[1:]
				b.Params = []BlockParam{{Dst: cmp.Src[0], Ty: TyI16}}
			case "flags-only":
				cmp.FlagsOnly = true
			case "unknown-predicate":
				cmp.Cond = CmpSubCarry
			case "missing-width":
				cmp.SrcTy = nil
			case "wrong-width":
				cmp.SrcTy = TyU8
			}
			old := *cmp
			if narrowZeroExtendedByteComparisons(f) || !reflect.DeepEqual(old, *cmp) {
				t.Fatal("unsafe rewrite", kind)
			}
		})
	}
}
