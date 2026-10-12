package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestHIRFactsFixedPoint(t *testing.T) {
	m := &hir.Module{Funcs: []*hir.Func{
		{Name: "a", Body: hir.Blk(&hir.ExprStmt{Expr: hir.Call("b", mir2.TyVoid)})},
		{Name: "b", Body: hir.Blk(&hir.ExprStmt{Expr: hir.Call("a", mir2.TyVoid)}, &hir.StoreStmt{Ptr: hir.Addr("g"), Val: hir.U8(1)})},
		{Name: "caller", Body: hir.Blk(&hir.ExprStmt{Expr: hir.Call("a", mir2.TyVoid)})},
		{Name: "ext", IsExtern: true},
		{Name: "unknown", Body: hir.Blk(&hir.ExprStmt{Expr: hir.Call("ext", mir2.TyVoid)})},
		{Name: "missing", Body: hir.Blk(&hir.ExprStmt{Expr: hir.Call("absent", mir2.TyVoid)})},
		{Name: "pure", Body: hir.Blk(hir.Ret(hir.U8(1)))},
	}}
	f := hir.AnalyzeFacts(m)
	for _, i := range []int{0, 1, 2} {
		if f.Functions[i].Effects&hir.WritesMemory == 0 {
			t.Fatalf("missing propagated write: %+v", f.Functions[i])
		}
	}
	if !f.Functions[0].Recursive || !f.Functions[1].Recursive || f.Functions[2].Recursive {
		t.Fatal("recursive cycle classification")
	}
	for _, i := range []int{3, 4, 5} {
		if f.Functions[i].Effects&hir.UnknownEffect == 0 {
			t.Fatal("unknown callee inferred pure")
		}
	}
	if f.Functions[6].Effects != 0 || f.DB.Count("hir-recursive") != 2 {
		t.Fatal("unexpected facts")
	}
	m.Funcs[1].Body = hir.Blk()
	if hir.AnalyzeFacts(m).Functions[0].Effects != 0 {
		t.Fatal("facts did not rebuild")
	}
}

func TestGraceHIRLetInBindingGuard(t *testing.T) {
	m := &hir.Module{Funcs: []*hir.Func{{Name: "test", Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(
		hir.If(hir.Bool(true), hir.Blk(&hir.ExprStmt{Expr: &hir.LetInExpr{Name: "x", Ty: mir2.TyU8, Init: hir.U8(9), Body: hir.Var("x", mir2.TyU8)}}), nil), hir.Ret(hir.Var("x", mir2.TyU8)))}}}
	before := graceLower(t, m).Dump()
	r, err := hir.RunGrace(m, 0)
	if err != nil || r.Applied != 0 || graceLower(t, m).Dump() != before {
		t.Fatalf("binding guard: %+v %v", r, err)
	}
}

func TestHIRFactsAllocationAndSymbols(t *testing.T) {
	m := &hir.Module{Funcs: []*hir.Func{
		{Name: "symbol", Body: hir.Blk(hir.Ret(hir.Addr("@mir2.str.0")))},
		{Name: "struct", Body: hir.Blk(hir.Decl("s", &mir2.StructTy{Name: "S"}, nil))},
	}}
	f := hir.AnalyzeFacts(m)
	for _, ff := range f.Functions {
		if ff.Effects&hir.MayTrap == 0 {
			t.Fatalf("missing possible allocation trap: %+v", ff)
		}
	}
}
