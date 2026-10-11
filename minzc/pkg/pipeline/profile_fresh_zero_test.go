package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestFreshHIRZeroSizeNodesHaveNoMutableStorage(t *testing.T) {
	fresh := func() *hir.Module {
		return &hir.Module{Funcs: []*hir.Func{{Name: "entry", RetTy: mir2.TyVoid, Body: hir.Blk(hir.While(&hir.BoolLitExpr{Val: true}, hir.Blk(&hir.BreakStmt{})))}}}
	}
	seen := map[uintptr]any{}
	first, second := fresh(), fresh()
	if e := jointCheckFreshHIR(first, seen); e != nil {
		t.Fatal(e)
	}
	if e := jointCheckFreshHIR(second, seen); e != nil {
		t.Fatal("independent AST with zero-size break rejected", e)
	}
	shared := &hir.BoolLitExpr{Val: true}
	a, b := fresh(), fresh()
	a.Funcs[0].Body.Body[0].(*hir.WhileStmt).Cond = shared
	b.Funcs[0].Body.Body[0].(*hir.WhileStmt).Cond = shared
	seen = map[uintptr]any{}
	if e := jointCheckFreshHIR(a, seen); e != nil {
		t.Fatal(e)
	}
	if e := jointCheckFreshHIR(b, seen); e == nil {
		t.Fatal("shared mutable condition accepted")
	}
}
