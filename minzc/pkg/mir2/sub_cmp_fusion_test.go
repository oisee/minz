package mir2

import "testing"

// Borrow from subtraction is unsigned ordering; S xor V is signed ordering.
// Keep legacy CmpLt/Ge with no signed SrcTy admitted for existing unsigned IR.
func TestSubCmpFusionPredicateContract(t *testing.T) {
	cases := []struct {
		name  string
		cond  CmpCond
		ty    Ty
		want  CmpCond
		fused bool
	}{
		{"signed_lt", CmpLt, TyI8, CmpLt, false}, {"signed_ge", CmpGe, TyI8, CmpGe, false},
		{"equal", CmpEq, TyU8, CmpEq, false}, {"not_equal", CmpNe, TyU8, CmpNe, false},
		{"signed_gt", CmpGt, TyI8, CmpGt, false}, {"signed_le", CmpLe, TyI8, CmpLe, false},
		{"unsigned_lt", CmpUlt, TyU8, CmpSubCarry, true}, {"unsigned_ge", CmpUge, TyU8, CmpSubCarryNot, true},
		{"legacy_lt", CmpLt, nil, CmpSubCarry, true}, {"legacy_ge", CmpGe, nil, CmpSubCarryNot, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &Inst{Op: OpSub, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
			cmp := &Inst{Op: OpCmp, Dst: 4, Src: [2]Reg{1, 2}, Ty: TyBool, SrcTy: tc.ty, Cond: tc.cond}
			blk := &Block{Insts: []*Inst{sub, cmp}}
			FusionSubCmpInBlock(blk)
			if cmp.Cond != tc.want {
				t.Fatalf("condition=%v want=%v", cmp.Cond, tc.want)
			}
			wantLHS := Reg(1)
			if tc.fused {
				wantLHS = 3
			}
			if cmp.Src[0] != wantLHS {
				t.Fatalf("lhs=%d want=%d", cmp.Src[0], wantLHS)
			}
		})
	}
}

func TestHoistSubCmpPreservesSignedPredicate(t *testing.T) {
	for _, signed := range []bool{false, true} {
		ty := Ty(TyU8)
		if signed {
			ty = TyI8
		}
		cmp := &Inst{Op: OpCmp, Dst: 4, Src: [2]Reg{1, 2}, Ty: TyBool, SrcTy: ty, Cond: CmpLt}
		sub := &Inst{Op: OpSub, Dst: 3, Src: [2]Reg{1, 2}, Ty: ty}
		blk := &Block{Insts: []*Inst{cmp, sub}}
		hoistReorderSubBeforeCmp(blk)
		if signed {
			if blk.Insts[0] != cmp || cmp.Cond != CmpLt || cmp.Src[0] != 1 {
				t.Fatal("signed compare was rewritten as unsigned borrow")
			}
		} else if blk.Insts[0] != sub || cmp.Cond != CmpSubCarry || cmp.Src[0] != 3 {
			t.Fatal("legacy unsigned borrow fusion lost")
		}
	}
}
