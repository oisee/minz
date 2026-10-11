package nanz

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestPrimitiveUnsignedResultWidth(t *testing.T) {
	types := []mir2.Ty{mir2.TyU8, mir2.TyU16, mir2.TyU24, mir2.TyU32}
	for _, l := range types {
		for _, r := range types {
			for _, op := range []string{"+", "-", "*", "/", "%", "&", "|", "^"} {
				want := l
				if r.Width() > l.Width() {
					want = r
				}
				if got := resultTy(l, r, op); got != want {
					t.Fatal(l, r, op, got, want)
				}
			}
		}
	}
}

// These are existing policy controls, not assertions of a newly chosen C-style
// common type. Keep mixed-signedness, shift-count and pointer behavior stable.
func TestUnsignedWidthRepairPolicyBoundaries(t *testing.T) {
	for _, c := range []struct {
		l, r mir2.Ty
		op   string
		want mir2.Ty
	}{
		{mir2.TyI8, mir2.TyU8, "+", mir2.TyI8}, {mir2.TyU8, mir2.TyI8, "+", mir2.TyU8},
		{mir2.TyU16, mir2.TyI32, "+", mir2.TyU16}, {mir2.TyI32, mir2.TyU16, "+", mir2.TyU16},
		{mir2.TyU8, mir2.TyU32, "<<", mir2.TyU8}, {mir2.TyU32, mir2.TyU8, ">>", mir2.TyU32},
		{mir2.TyU8, mir2.TyU16, "<<", mir2.TyU16}, // historical count-driven u16 rule: unresolved
		{mir2.TyU16, mir2.TyU32, "<", mir2.TyBool}, {mir2.TyU32, mir2.TyU16, "==", mir2.TyBool},
		{mir2.PointerTo(mir2.TyU16), mir2.TyU32, "+", mir2.PointerTo(mir2.TyU16)},
	} {
		if got := resultTy(c.l, c.r, c.op); got != c.want {
			t.Fatal(c, got)
		}
	}
	src := `fun *(a:u16,b:u32)->u8 {return 7}
 fun run(a:u16,b:u32)->u8{return a*b}`
	m, e := Parse(src, "overloaded")
	if e != nil {
		t.Fatal(e)
	}
	ret := m.FuncByName("run").Body.Body[0].(*hir.ReturnStmt)
	call, ok := ret.Val.(*hir.CallExpr)
	if !ok || call.Ty != mir2.TyU8 {
		t.Fatal("exact overload bypasses primitive rule", ret.Val)
	}
}
