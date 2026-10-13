package nanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestFoldReturnCarriesAccumulatorType(t *testing.T) {
	m, err := nanz.Parse(`fun f(n: u8) -> u16 { return range(n..0).fold(0, |acc: u16, i: u8| { return acc + i }) }`, "fold_type")
	if err != nil {
		t.Fatal(err)
	}
	call := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt).Val.(*hir.CallExpr)
	if call.Ty != mir2.TyU16 {
		t.Fatalf("fold type %v, want u16", call.Ty)
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	got, err := vm.Call("f", []mir2.Value{{I: 255}})
	if err != nil || len(got) != 1 || got[0].I != 32640 {
		t.Fatalf("got %v err %v", got, err)
	}
}
