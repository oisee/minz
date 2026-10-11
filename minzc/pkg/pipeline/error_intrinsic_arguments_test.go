package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// Build malformed HIR directly: source parser already refuses @(args) on
// check/propagate. Compiler callers must not have their arguments ignored.
func TestErrorIntrinsicArgumentsRejectedByBothAPIs(t *testing.T) {
	for _, kind := range []string{"check-arg", "propagate-arg", "code-pointer", "code-word", "code-negative", "code-large", "missing-code"} {
		for _, steps := range []bool{false, true} {
			one := &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8}
			errcall := &hir.CallExpr{Fn: "@error", Args: []hir.Expr{one}, Ty: mir2.TyVoid}
			body := []hir.Stmt{&hir.ExprStmt{Expr: errcall}}
			switch kind {
			case "code-pointer":
				errcall.Args = []hir.Expr{&hir.ConstPtrExpr{Addr: 1}}
			case "code-word":
				errcall.Args = []hir.Expr{&hir.IntLitExpr{Val: 256, Ty: mir2.TyU16}}
			case "code-negative":
				errcall.Args = []hir.Expr{&hir.IntLitExpr{Val: -1, Ty: mir2.TyU8}}
			case "code-large":
				errcall.Args = []hir.Expr{&hir.IntLitExpr{Val: 256, Ty: mir2.TyU8}}
			case "missing-code":
				errcall.Args = nil
			case "check-arg", "propagate-arg":
				name := "@check"
				if kind == "propagate-arg" {
					name = "@propagate"
				}
				body = []hir.Stmt{&hir.VarDeclStmt{Name: "x", Ty: mir2.TyU8, Init: &hir.CallExpr{Fn: "callee", Ty: mir2.TyU8}}, &hir.ExprStmt{Expr: &hir.CallExpr{Fn: name, Args: []hir.Expr{one}, Ty: mir2.TyVoid}}, &hir.ReturnStmt{Val: one}}
			}
			h := &hir.Module{Funcs: []*hir.Func{{Name: "test", RetTy: mir2.TyU8, Body: &hir.Block{Body: body}}}}
			if kind == "check-arg" || kind == "propagate-arg" {
				h.Funcs = append(h.Funcs, &hir.Func{Name: "callee", RetTy: mir2.TyU8, Body: &hir.Block{Body: []hir.Stmt{&hir.ExprStmt{Expr: errcall}}}})
			}
			var err error
			if steps {
				_, err = CompileHIRSteps(h)
			} else {
				_, err = CompileHIRWithOptions(h, Options{})
			}
			if err == nil {
				t.Fatalf("ignored malformed %s steps=%v", kind, steps)
			}
		}
	}
}
