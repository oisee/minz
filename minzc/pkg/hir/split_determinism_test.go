package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"testing"
)

func TestSplitInterfaceDeterministic(t *testing.T) {
	f := &Func{Name: "split", Params: []Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}}, Body: Blk(
		&ExprStmt{Expr: Var("a", mir2.TyU8)}, &ExprStmt{Expr: Var("b", mir2.TyU8)},
		&ExprStmt{Expr: Var("a", mir2.TyU8)}, &ReturnStmt{Vals: []Expr{Var("b", mir2.TyU8)}},
	)}
	for run := 0; run < 100; run++ {
		candidates := FindSplitPoints(f, []int{9, 9, 9, 9})
		if len(candidates) == 0 {
			t.Fatal("no split candidates")
		}
		if !reflect.DeepEqual(candidates[0].inputs, []string{"a", "b"}) {
			t.Fatalf("unstable split ABI: %v", candidates[0].inputs)
		}
	}
}
