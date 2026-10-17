package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestInternalFunctionsAndExplicitEntries(t *testing.T) {
	literal := func(n int64) *hir.Block {
		return &hir.Block{Body: []hir.Stmt{&hir.ReturnStmt{Val: &hir.IntLitExpr{Val: n, Ty: mir2.TyU8}}}}
	}
	m := &hir.Module{Globals: []mir2.Global{{Name: "answer", Ty: mir2.TyU8}}, Funcs: []*hir.Func{
		{Name: "main", RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{
			&hir.AssignStmt{Target: &hir.VarRefExpr{Name: "answer", Ty: mir2.TyU8}, Val: &hir.CallExpr{Fn: "internal_used", Ty: mir2.TyU8}}, &hir.ReturnStmt{},
		}}},
		{Name: "public_value", RetTy: mir2.TyU8, Body: literal(7)},
		{Name: "internal_used", Internal: true, RetTy: mir2.TyU8, Body: literal(8)},
		{Name: "internal_assert", Internal: true, RetTy: mir2.TyU8, Body: literal(9)},
		{Name: "internal_dead", Internal: true, RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: "CALL public_value"}, &hir.ReturnStmt{}}}},
	}, Asserts: []hir.Assert{{FuncName: "internal_assert", Expected: 9}}}
	opts := DefaultOptions()
	opts.AssertMode = "none"
	steps, err := CompileHIRSteps(m, opts)
	if err != nil {
		t.Fatal(err)
	}
	if steps.MIR2Module.FuncByName("internal_dead") != nil {
		t.Fatal("dead internal ASM remains an implicit root")
	}
	if steps.MIR2Module.FuncByName("public_value") == nil || steps.MIR2Module.FuncByName("internal_assert") == nil {
		t.Fatal("explicit public/assert entry removed")
	}
	v := mir2.NewVM(steps.MIR2Module)
	if _, err := v.Call("main", nil); err != nil {
		t.Fatal(err)
	}
	if got := v.ReadHeap(v.GlobalAddr("answer"), 1)[0]; got != 8 {
		t.Fatalf("internal call result %d", got)
	}
	for name, want := range map[string]int64{"public_value": 7, "internal_assert": 9} {
		got, err := v.Call(name, nil)
		if err != nil || len(got) != 1 || got[0].I != want {
			t.Fatalf("entry %s: %v %v", name, got, err)
		}
	}
}
