package hir_test

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"strings"
	"testing"
)

func pointerFixture(mutate func(*hir.CallIndirectExpr) []hir.Stmt) (*hir.Module, *hir.CallIndirectExpr) {
	one := &hir.Func{Name: "one", Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(&hir.ReturnStmt{Val: hir.Var("x", mir2.TyU8)})}
	two := &hir.Func{Name: "two", Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}, {Name: "y", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(&hir.ReturnStmt{Val: hir.Var("y", mir2.TyU8)})}
	call := &hir.CallIndirectExpr{FnPtr: hir.Var("p", mir2.TyPtr), Args: []hir.Expr{hir.U8(7)}, Ty: mir2.TyU8}
	body := []hir.Stmt{hir.Decl("p", mir2.TyPtr, &hir.AddrOfExpr{Sym: "one"})}
	body = append(body, mutate(call)...)
	body = append(body, &hir.ReturnStmt{Val: call})
	main := &hir.Func{Name: "main", RetTy: mir2.TyU8, Body: hir.Blk(body...)}
	return &hir.Module{Funcs: []*hir.Func{main, one, two}}, call
}
func TestMutableRawFunctionPointerSignatures(t *testing.T) {
	assign := func() hir.Stmt {
		return &hir.AssignStmt{Target: hir.Var("p", mir2.TyPtr), Val: &hir.AddrOfExpr{Sym: "two"}}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*hir.CallIndirectExpr) []hir.Stmt
	}{
		{"straight", func(*hir.CallIndirectExpr) []hir.Stmt { return []hir.Stmt{assign()} }},
		{"branch", func(*hir.CallIndirectExpr) []hir.Stmt {
			return []hir.Stmt{&hir.IfStmt{Cond: hir.U8(1), Then: hir.Blk(assign())}}
		}},
		{"loop", func(call *hir.CallIndirectExpr) []hir.Stmt {
			return []hir.Stmt{&hir.WhileStmt{Cond: hir.U8(0), Body: hir.Blk(&hir.ExprStmt{Expr: call}, assign())}}
		}},
		{"goto", func(*hir.CallIndirectExpr) []hir.Stmt {
			return []hir.Stmt{&hir.LabelStmt{Name: "again"}, assign(), &hir.IfStmt{Cond: hir.U8(0), Then: hir.Blk(&hir.GotoStmt{Label: "again"})}}
		}},
		{"pointer_copy", func(call *hir.CallIndirectExpr) []hir.Stmt {
			call.FnPtr = hir.Var("q", mir2.TyPtr)
			return []hir.Stmt{hir.Decl("q", mir2.TyPtr, hir.Var("p", mir2.TyPtr)), &hir.AssignStmt{Target: hir.Var("q", mir2.TyPtr), Val: &hir.AddrOfExpr{Sym: "two"}}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, call := pointerFixture(tc.mutate)
			if e := hir.ResolveCallSignatures(m); e != nil {
				t.Fatal(e)
			}
			if call.Signature != nil {
				t.Fatal("mutable pointer inherited a declaration target signature")
			}
			if e := hir.ResolveCallSignatures(m); e != nil {
				t.Fatal(e)
			}
			call.Signature = &hir.CallSignature{Params: []mir2.Ty{mir2.TyU8, mir2.TyU8}, Returns: []mir2.Ty{mir2.TyU8}}
			if e := hir.Verify(m); e == nil || !strings.Contains(e.Error(), "arity") {
				t.Fatalf("explicit signature ignored: %v", e)
			}
		})
	}
}
func TestImmutableAddressOfPointerRetainsKnownSignature(t *testing.T) {
	m, call := pointerFixture(func(*hir.CallIndirectExpr) []hir.Stmt { return nil })
	if e := hir.ResolveCallSignatures(m); e != nil {
		t.Fatal(e)
	}
	if call.Signature == nil || len(call.Signature.Params) != 1 {
		t.Fatal("known unmodified pointer signature missing")
	}
	call.Args = nil
	if e := hir.Verify(m); e == nil || !strings.Contains(e.Error(), "arity") {
		t.Fatalf("known pointer check lost: %v", e)
	}
}
func TestMutableFunctionPointerCaptureCannotEscape(t *testing.T) {
	m, _ := pointerFixture(func(*hir.CallIndirectExpr) []hir.Stmt {
		return []hir.Stmt{&hir.AssignStmt{Target: hir.Var("p", mir2.TyPtr), Val: &hir.AddrOfExpr{Sym: "two"}}}
	})
	m.Funcs[1].Captures = []hir.Param{{Name: "outer", Ty: mir2.TyU8}}
	m.Funcs[0].Body.Body = append([]hir.Stmt{hir.Decl("outer", mir2.TyU8, hir.U8(9))}, m.Funcs[0].Body.Body...)
	if e := hir.Verify(m); e == nil || !strings.Contains(e.Error(), "capturing callable") {
		t.Fatalf("capturing raw pointer escaped: %v", e)
	}
}
func TestC89MutableFunctionPointersMIRAssertions(t *testing.T) {
	src, e := os.ReadFile("../../../examples/c89/func_ptr.c")
	if e != nil {
		t.Fatal(e)
	}
	m, e := c89.Compile(string(src), "func_ptr.c")
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Asserts) != 6 {
		t.Fatalf("want six independent MIR annotations, got %d", len(m.Asserts))
	}
	s, e := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, SkipZ80Emission: true, AssertMode: "mir2"})
	if e != nil {
		t.Fatal(e)
	}
	if s.MIR2Module == nil || s.Assembly != "" {
		t.Fatal("expected MIR2-only product")
	}
}

func TestMutableFunctionPointerAssignmentCaptureCannotEscape(t *testing.T) {
	m, _ := pointerFixture(func(*hir.CallIndirectExpr) []hir.Stmt {
		return []hir.Stmt{&hir.AssignStmt{Target: hir.Var("p", mir2.TyPtr), Val: &hir.AddrOfExpr{Sym: "two"}}}
	})
	m.Funcs[2].Captures = []hir.Param{{Name: "outer", Ty: mir2.TyU8}}
	m.Funcs[0].Body.Body = append([]hir.Stmt{hir.Decl("outer", mir2.TyU8, hir.U8(9))}, m.Funcs[0].Body.Body...)
	if e := hir.Verify(m); e == nil || !strings.Contains(e.Error(), "escape through assignment") {
		t.Fatalf("capturing assignment escaped: %v", e)
	}
}
