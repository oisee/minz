package hir_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// A descriptor built through the shared HIR API is consumed by separately
// parsed Nanz code. This pins field ABI and element reads across frontends,
// rather than comparing the helper's field list to itself.
func TestBorrowedViewHIRToNanzNative(t *testing.T) {
	m, err := nanz.Parse(`
struct ByteView { data: ^u8, len: u16 }
fun consume(view: ^ByteView) -> u16 {
    var result: u16 = 0
    for value in view.data[0..view.len] {
        result = result + u16(value)
    }
    return result
}
`, "borrowed-view-consumer.nanz")
	if err != nil {
		t.Fatal(err)
	}
	m.NonNullObjects = true
	st, err := hir.BorrowedViewType("ByteView", mir2.TyU8)
	if err != nil {
		t.Fatal(err)
	}
	array := &mir2.ArrayTy{Elem: mir2.TyU8, Len: 3}
	data := &hir.AddrOfExpr{X: hir.Var("storage", array), Ty: mir2.TyPtr}
	view := &hir.AddrOfExpr{X: hir.Var("view", st), Ty: mir2.PointerTo(st)}
	body := hir.Blk(&hir.VarDeclStmt{Name: "storage", Ty: array}, &hir.VarDeclStmt{Name: "view", Ty: st})
	for i := int64(0); i < 3; i++ {
		body.Body = append(body.Body, &hir.AssignStmt{Target: &hir.IndexExpr{Base: data, Idx: hir.U16(i), ElemTy: mir2.TyU8}, Val: &hir.BinExpr{Op: "+", L: hir.Var("seed", mir2.TyU8), R: &hir.IntLitExpr{Val: i, Ty: mir2.TyU8}, Ty: mir2.TyU8}})
	}
	body.Body = append(body.Body,
		&hir.AssignStmt{Target: &hir.FieldExpr{X: view, Field: hir.BorrowedViewData, Offset: st.ByteOffset(0), Ty: mir2.TyPtr}, Val: data},
		&hir.AssignStmt{Target: &hir.FieldExpr{X: view, Field: hir.BorrowedViewLen, Offset: st.ByteOffset(1), Ty: mir2.TyU16}, Val: hir.U16(3)},
		&hir.ReturnStmt{Val: &hir.CallExpr{Fn: "consume", Args: []hir.Expr{view}, Ty: mir2.TyU16}},
	)
	m.Funcs = append(m.Funcs, &hir.Func{Name: "probe", Params: []hir.Param{{Name: "seed", Ty: mir2.TyU8}}, RetTy: mir2.TyU16, Body: body})
	for x := int64(0); x < 256; x++ {
		want := x + ((x + 1) & 255) + ((x + 2) & 255)
		m.Asserts = append(m.Asserts, hir.Assert{FuncName: "probe", Args: []int64{x}, Expected: want, Source: "shared HIR view -> Nanz"})
	}
	if err = hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	if err = pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err = pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
