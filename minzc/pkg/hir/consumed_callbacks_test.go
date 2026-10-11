package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestConsumedCallbackContracts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		arg, ret mir2.Ty
		arity    int
		wantErr  bool
	}{{"byte", mir2.TyU8, mir2.TyU8, 1, false}, {"word", mir2.TyU16, mir2.TyU16, 1, false}, {"arg-width", mir2.TyU16, mir2.TyU8, 1, true}, {"ret-width", mir2.TyU8, mir2.TyU16, 1, true}, {"arity", mir2.TyU8, mir2.TyU8, 2, true}, {"signed", mir2.TyI8, mir2.TyU8, 1, true}, {"fallible", mir2.TyU8, mir2.TyU8, 1, true}} {
		t.Run(tc.name, func(t *testing.T) {
			expected := mir2.TyU8
			if tc.name == "word" {
				expected = mir2.TyU16
			}
			apply := &hir.Func{Name: "apply", Params: []hir.Param{{Name: "f", Ty: mir2.TyPtr}, {Name: "x", Ty: expected}}, RetTy: expected, Body: hir.Blk(hir.Ret(&hir.CallIndirectExpr{FnPtr: hir.Var("f", mir2.TyPtr), Args: []hir.Expr{hir.Var("x", expected)}, Ty: expected}))}
			params := []hir.Param{{Name: "x", Ty: tc.arg}}
			if tc.arity == 2 {
				params = append(params, hir.Param{Name: "y", Ty: tc.arg})
			}
			target := &hir.Func{Name: "target", Params: params, RetTy: tc.ret, Body: hir.Blk(hir.Ret(&hir.IntLitExpr{Val: 0, Ty: tc.ret}))}
			caller := &hir.Func{Name: "caller", RetTy: expected, Body: hir.Blk(hir.Ret(&hir.CallExpr{Fn: "apply", Args: []hir.Expr{&hir.AddrOfExpr{Sym: "target"}, &hir.IntLitExpr{Val: 5, Ty: expected}}, Ty: expected}))}
			if tc.name == "fallible" {
				target.FallibleCY = true
			}
			m := &hir.Module{Funcs: []*hir.Func{caller, apply, target}}
			e := hir.Verify(m)
			if (e != nil) != tc.wantErr {
				t.Fatalf("got%v wantErr%v", e, tc.wantErr)
			}
			if tc.wantErr && !strings.Contains(e.Error(), "callback") {
				t.Fatal(e)
			}
		})
	}
}
func TestConsumedCallbacksForwardingConflictAndPointerUse(t *testing.T) {
	u8 := mir2.TyU8
	ptr := mir2.TyPtr
	apply := &hir.Func{Name: "apply", Params: []hir.Param{{Name: "f", Ty: ptr}, {Name: "x", Ty: u8}}, RetTy: u8, Body: hir.Blk(hir.Ret(&hir.CallIndirectExpr{FnPtr: hir.Var("f", ptr), Args: []hir.Expr{hir.Var("x", u8)}, Ty: u8}))}
	wrap := &hir.Func{Name: "wrap", Params: []hir.Param{{Name: "p", Ty: ptr}, {Name: "x", Ty: u8}}, RetTy: u8, Body: hir.Blk(hir.Decl("alias", ptr, hir.Var("p", ptr)), hir.Ret(&hir.CallExpr{Fn: "apply", Args: []hir.Expr{hir.Var("alias", ptr), hir.Var("x", u8)}, Ty: u8}))}
	bad := &hir.Func{Name: "bad", Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, RetTy: u8, Body: hir.Blk(hir.Ret(hir.U8(0)))}
	caller := &hir.Func{Name: "caller", RetTy: u8, Body: hir.Blk(hir.Ret(&hir.CallExpr{Fn: "wrap", Args: []hir.Expr{&hir.AddrOfExpr{Sym: "bad"}, hir.U8(5)}, Ty: u8}))}
	if e := hir.Verify(&hir.Module{Funcs: []*hir.Func{caller, wrap, apply, bad}}); e == nil || !strings.Contains(e.Error(), "callback") {
		t.Fatal("forwarding mismatch accepted", e)
	}
	// Explicit annotation cannot authorize a different physical argument width.
	apply.Body = hir.Blk(hir.Ret(&hir.CallIndirectExpr{FnPtr: hir.Var("f", ptr), Args: []hir.Expr{hir.Var("x", u8)}, Ty: u8, Signature: &hir.CallSignature{Params: []mir2.Ty{mir2.TyU16}, Returns: []mir2.Ty{u8}}}))
	if e := hir.Verify(&hir.Module{Funcs: []*hir.Func{apply}}); e == nil || !strings.Contains(e.Error(), "adapter") {
		t.Fatal("forged explicit ABI accepted", e)
	}
	// Same pointer parameter consumed with incompatible argument types.
	apply.Body = hir.Blk(&hir.ExprStmt{Expr: &hir.CallIndirectExpr{FnPtr: hir.Var("f", ptr), Args: []hir.Expr{hir.Var("x", u8)}, Ty: u8}}, hir.Ret(&hir.CallIndirectExpr{FnPtr: hir.Var("f", ptr), Args: []hir.Expr{hir.U16(1)}, Ty: u8}))
	if e := hir.Verify(&hir.Module{Funcs: []*hir.Func{apply}}); e == nil || !strings.Contains(e.Error(), "conflicting") {
		t.Fatal("conflict accepted", e)
	}
	// A function address passed as ordinary data ptr, never invoked, is unaffected.
	sink := &hir.Func{Name: "sink", Params: []hir.Param{{Name: "p", Ty: ptr}}, RetTy: u8, Body: hir.Blk(hir.Ret(hir.U8(0)))}
	caller.Body = hir.Blk(hir.Ret(&hir.CallExpr{Fn: "sink", Args: []hir.Expr{&hir.AddrOfExpr{Sym: "bad"}}, Ty: u8}))
	if e := hir.Verify(&hir.Module{Funcs: []*hir.Func{caller, sink, bad}}); e != nil {
		t.Fatal(e)
	}
	// Truly anonymous raw function pointer remains unresolved.
	apply.Body = hir.Blk(hir.Ret(&hir.CallIndirectExpr{FnPtr: hir.Var("f", ptr), Args: []hir.Expr{hir.Var("x", u8)}, Ty: u8}))
	caller.Body = hir.Blk(hir.Ret(&hir.CallExpr{Fn: "apply", Args: []hir.Expr{&hir.IntLitExpr{Val: 0x8000, Ty: ptr}, hir.U8(5)}, Ty: u8}))
	if e := hir.Verify(&hir.Module{Funcs: []*hir.Func{caller, apply}}); e != nil {
		t.Fatal(e)
	}
}
