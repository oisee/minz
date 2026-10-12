package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestLambdaNameDoesNotAuthorizeUnknownReferences(t *testing.T) {
	m := verifierModule(&ReturnStmt{Val: Var("typo", mir2.TyU8)})
	m.Funcs[0].Name = "lambda_forged"
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "unknown variable") {
		t.Fatalf("got %v", err)
	}
}
func TestExplicitCaptureContract(t *testing.T) {
	fn := &Func{Name: "closure", Captures: []Param{{Name: "x", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: Blk(&ReturnStmt{Val: Var("x", mir2.TyU8)})}
	m := verifierModule(&ExprStmt{Expr: Var("closure", mir2.TyU8)}, &ReturnStmt{Val: U8(0)})
	m.Funcs = append(m.Funcs, fn)
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "needs capture") {
		t.Fatalf("got %v", err)
	}
	m.Funcs[0].Body = Blk(Decl("x", mir2.TyU8, U8(3)), &ExprStmt{Expr: Var("closure", mir2.TyU8)}, &ReturnStmt{Val: U8(0)})
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	fn.Captures = append(fn.Captures, fn.Captures[0])
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "duplicate capture") {
		t.Fatalf("got %v", err)
	}
}
func TestAliasAndIndirectSignatureContract(t *testing.T) {
	f := &Func{Name: "target", Params: []Param{{Name: "x", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: Blk(&ReturnStmt{Val: Var("x", mir2.TyU8)})}
	call := &CallIndirectExpr{FnPtr: Var("alias", mir2.TyU8), Args: []Expr{U8(7)}, Ty: mir2.TyU8}
	m := verifierModule(Decl("alias", mir2.TyU8, Var("target", mir2.TyU8)), &ReturnStmt{Val: call})
	m.Funcs = append(m.Funcs, f)
	if err := ResolveCallSignatures(m); err != nil {
		t.Fatal(err)
	}
	if call.Signature == nil || len(call.Signature.Params) != 1 {
		t.Fatal("signature missing")
	}
	call.Args = nil
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "arity") {
		t.Fatalf("got %v", err)
	}
	call.Args = []Expr{U8(7)}
	call.Signature = &CallSignature{Returns: []mir2.Ty{mir2.TyU8}}
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("got %v", err)
	}
	call.Signature = nil
	m.Funcs[0].Body.Body = append(m.Funcs[0].Body.Body[:1], &AssignStmt{Target: Var("alias", mir2.TyU8), Val: U8(3)}, &ReturnStmt{Val: call})
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "reassignment") {
		t.Fatalf("got %v", err)
	}
}
func TestExplicitRawIndirectSignatureRejectsWrongArguments(t *testing.T) {
	call := &CallIndirectExpr{FnPtr: Var("p", mir2.TyPtr), Ty: mir2.TyU8, Signature: &CallSignature{Params: []mir2.Ty{mir2.TyU8}, Returns: []mir2.Ty{mir2.TyU8}}}
	m := verifierModule(&ReturnStmt{Val: call})
	m.Funcs[0].Params = []Param{{Name: "p", Ty: mir2.TyPtr}}
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "arity") {
		t.Fatalf("got %v", err)
	}
}
func TestTypedCallArgumentsRejectDifferentPointees(t *testing.T) {
	f := &Func{Name: "target", Params: []Param{{Name: "p", Ty: mir2.PointerTo(mir2.TyU16)}}, RetTy: mir2.TyU8, Body: Blk(&ReturnStmt{Val: U8(1)})}
	m := verifierModule(&ReturnStmt{Val: Call("target", mir2.TyU8, Var("p", mir2.PointerTo(mir2.TyI16)))})
	m.Funcs[0].Params = []Param{{Name: "p", Ty: mir2.PointerTo(mir2.TyI16)}}
	m.Funcs = append(m.Funcs, f)
	if err := Verify(m); err == nil || !strings.Contains(err.Error(), "argument") {
		t.Fatalf("got %v", err)
	}
}
