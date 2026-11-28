package mir2

import "testing"

// (y+y) > z in C int arithmetic: %d = add %e, %e : i16 with %e = ext y. The
// sum reaches 510, so a comparison must see it at full width; narrowing it
// to u8 wrapped y=128 to 0 (generated program seed 41, f2). A bitwise op of
// zero-extended bytes stays in 0..255 and may still be narrowed there.
func TestNarrowArithKeepsComparedSum(t *testing.T) {
	for _, tc := range []struct {
		op   Op
		ext  Op
		want bool
	}{{OpAdd, OpExt, false}, {OpXor, OpExt, true}, {OpXor, OpSext, false}} {
		srcTy := TyU8
		if tc.ext == OpSext {
			srcTy = TyI8
		}
		e1 := &Inst{Op: tc.ext, Dst: 3, Src: [2]Reg{1}, Ty: TyI16, SrcTy: srcTy}
		e2 := &Inst{Op: tc.ext, Dst: 4, Src: [2]Reg{2}, Ty: TyI16, SrcTy: srcTy}
		rhs := Reg(4)
		if tc.op == OpAdd {
			rhs = 3
		}
		x := &Inst{Op: tc.op, Dst: 5, Src: [2]Reg{3, rhs}, Ty: TyI16}
		cmp := &Inst{Op: OpCmp, Dst: 6, Src: [2]Reg{5, 4}, Ty: TyBool, SrcTy: TyI16, Cond: CmpGt}
		f := &Func{Name: "f", Contract: Contract{Params: []Param{{Reg: 1, Ty: srcTy}, {Reg: 2, Ty: srcTy}}},
			Blocks: []*Block{{Label: "entry", Insts: []*Inst{e1, e2, x, cmp}, Term: &TermRet{Vals: []Reg{6}}}}}
		if got := isNarrowable(x, f); got != tc.want {
			t.Fatalf("%v of %v bytes compared: narrowable=%t want %t", tc.op, tc.ext, got, tc.want)
		}
	}
}
