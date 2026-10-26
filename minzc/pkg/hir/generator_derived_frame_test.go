package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"strings"
	"testing"
)

// A frontend can defer frame layout to shared expansion. Verify/dump/lowering
// must agree on this valid pre-expansion representation.
func TestGeneratorForDerivedFrame(t *testing.T) {
	m := &hir.Module{Name: "derived_frame", NonNullObjects: true, Funcs: []*hir.Func{
		{Name: "source", IsGen: true, ElemTy: mir2.TyU8, RetTy: mir2.TyVoid, Body: hir.Blk(&hir.YieldStmt{Val: hir.U8(7)})},
		{Name: "probe", RetTy: mir2.TyU8, Body: hir.Blk(
			&hir.VarDeclStmt{Name: "result", Ty: mir2.TyU8, Init: hir.U8(0)},
			&hir.GenForStmt{Var: "value", Gen: "source", ElemTy: mir2.TyU8, Body: hir.Blk(&hir.AssignStmt{Target: hir.Var("result", mir2.TyU8), Val: hir.Var("value", mir2.TyU8)})},
			&hir.ReturnStmt{Val: hir.Var("result", mir2.TyU8)},
		)},
	}, Asserts: []hir.Assert{{FuncName: "probe", Expected: 7, Source: "frontend defers frame"}}}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	if text := m.Dump(); !strings.Contains(text, "via frame <derived>") {
		t.Fatalf("dump omitted derived frame: %s", text)
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
