package hir

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func verifierModule(body ...Stmt) *Module {
	return &Module{Funcs: []*Func{{Name: "test", RetTy: mir2.TyU8, Body: Blk(body...)}}}
}

func TestVerifyClosedNamesPreservesDefaultAndLexicalBindings(t *testing.T) {
	implicit := verifierModule(&AssignStmt{Target: Var("fault", mir2.TyU8), Val: U8(0)}, &ReturnStmt{Val: Var("fault", mir2.TyU8)})
	if e := Verify(implicit); e != nil {
		t.Fatal("default implicit assignment changed", e)
	}
	if e := VerifyClosedNames(implicit); e == nil || !strings.Contains(e.Error(), "undeclared local") {
		t.Fatalf("free write admitted: %v", e)
	}
	explicit := verifierModule(Decl("fault", mir2.TyU8, U8(1)), &AssignStmt{Target: Var("fault", mir2.TyU8), Val: U8(2)}, &ReturnStmt{Val: Var("fault", mir2.TyU8)})
	if e := VerifyClosedNames(explicit); e != nil {
		t.Fatal("explicit local rejected", e)
	}
	parameter := verifierModule(&AssignStmt{Target: Var("fault", mir2.TyU8), Val: U8(2)}, &ReturnStmt{Val: Var("fault", mir2.TyU8)})
	parameter.Funcs[0].Params = []Param{{Name: "fault", Ty: mir2.TyU8}}
	if e := VerifyClosedNames(parameter); e != nil {
		t.Fatal("parameter assignment rejected", e)
	}
	sibling := verifierModule(If(Bool(true), Blk(Decl("fault", mir2.TyU8, U8(1))), nil), &AssignStmt{Target: Var("fault", mir2.TyU8), Val: U8(2)}, &ReturnStmt{Val: U8(0)})
	if e := VerifyClosedNames(sibling); e == nil {
		t.Fatal("sibling declaration bound a free write")
	}
	global := verifierModule(&AssignStmt{Target: Var("fault", mir2.TyU8), Val: U8(2)}, &ReturnStmt{Val: Var("fault", mir2.TyU8)})
	global.Globals = []mir2.Global{{Name: "fault", Ty: mir2.TyU8}}
	if e := VerifyClosedNames(global); e != nil {
		t.Fatal("declared global rejected", e)
	}
	address := verifierModule(&ReturnStmt{Val: &AddrOfExpr{Sym: "foreign"}})
	if e := Verify(address); e != nil {
		t.Fatal("default lazy address changed", e)
	}
	if e := VerifyClosedNames(address); e == nil || !strings.Contains(e.Error(), "unknown address symbol") {
		t.Fatalf("foreign address admitted: %v", e)
	}
	address.Globals = []mir2.Global{{Name: "foreign", Ty: mir2.TyU8}}
	if e := VerifyClosedNames(address); e != nil {
		t.Fatal("declared global address rejected", e)
	}
	stringAddress := verifierModule(&ReturnStmt{Val: &AddrOfExpr{Sym: "@mir2.str.0"}})
	stringAddress.Strings = []string{"x"}
	if e := VerifyClosedNames(stringAddress); e != nil {
		t.Fatal("interned string address rejected", e)
	}
}

func TestVerifyRejectsMalformedHIR(t *testing.T) {
	var nilExpr *IntLitExpr
	cycle := &BinExpr{Op: "+", Ty: mir2.TyU8}
	cycle.L = cycle
	cycle.R = U8(1)
	cyclicBlock := Blk()
	cyclicBlock.Body = []Stmt{cyclicBlock}
	cases := []struct {
		name string
		m    *Module
		want string
	}{
		{"nil module", nil, "nil module"},
		{"nil function", &Module{Funcs: []*Func{nil}}, "nil function"},
		{"typed nil", verifierModule(&ReturnStmt{Val: nilExpr}), "missing"},
		{"expression cycle", verifierModule(&ReturnStmt{Val: cycle}), "cyclic"},
		{"block cycle", verifierModule(cyclicBlock), "cyclic"},
		{"unknown variable", verifierModule(&ReturnStmt{Val: Var("missing", mir2.TyU8)}), "unknown variable"},
		{"sibling scope", verifierModule(If(Bool(true), Blk(Decl("x", mir2.TyU8, U8(1))), nil), &ReturnStmt{Val: Var("x", mir2.TyU8)}), "unknown variable"},
		{"missing type", verifierModule(&ReturnStmt{Val: &IntLitExpr{Val: 1}}), "missing type"},
		{"bad lvalue", verifierModule(&AssignStmt{Target: U8(1), Val: U8(2)}), "assignment target"},
		{"break", verifierModule(&BreakStmt{}), "outside loop"},
		{"continue", verifierModule(&ContinueStmt{}), "outside loop"},
		{"unknown label", verifierModule(&GotoStmt{Label: "absent"}), "unknown label"},
		{"duplicate label", verifierModule(&LabelStmt{Name: "x"}, &LabelStmt{Name: "x"}), "duplicate label"},
		{"missing then", verifierModule(If(Bool(true), nil, nil)), "missing"},
		{"bad tuple", verifierModule(&TupleLetStmt{Names: []string{"x"}, Tys: nil}), "length mismatch"},
		{"bad return arity", verifierModule(&ReturnStmt{Vals: []Expr{U8(1), U8(2)}}), "return arity"},
		{"bad address", verifierModule(&ReturnStmt{Val: &LoadExpr{Ptr: Bool(true), Ty: mir2.TyU8}}), "non-address"},
		{"wrong pointee", verifierModule(Decl("p", mir2.PointerTo(mir2.TyU16), U16(100)), &ReturnStmt{Val: &LoadExpr{Ptr: Var("p", mir2.PointerTo(mir2.TyU16)), Ty: mir2.TyU8}}), "pointee"},
		{"self-binding", verifierModule(&AssignStmt{Target: Var("x", mir2.TyU8), Val: Var("x", mir2.TyU8)}), "unknown variable"},
		{"forged pointer type", verifierModule(Decl("p", mir2.PointerTo(mir2.TyU16), U16(100)), &ReturnStmt{Val: &LoadExpr{Ptr: Var("p", mir2.TyPtr), Ty: mir2.TyU8}}), "declared"},
		{"wrong indexed pointee", verifierModule(Decl("p", mir2.PointerTo(mir2.TyU16), U16(100)), &ReturnStmt{Val: &IndexExpr{Base: Var("p", mir2.PointerTo(mir2.TyU16)), Idx: U8(0), ElemTy: mir2.TyU8}}), "pointee"},
		{"partial typed store", verifierModule(Decl("p", mir2.PointerTo(mir2.TyU16), U16(100)), &StoreStmt{Ptr: Var("p", mir2.PointerTo(mir2.TyU16)), Val: U8(1)}), "pointee"},
		{"bad call arity", verifierModule(&ExprStmt{Expr: Call("test", mir2.TyU8, U8(1))}), "arity"},
		{"bad call type", verifierModule(&ExprStmt{Expr: Call("test", mir2.TyVoid)}), "return type"},
		{"bad bit", verifierModule(&ReturnStmt{Val: &BitExpr{X: U8(1), Bit: 8}}), "bit index"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Verify(tc.m)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestVerifyValidHIRCompatibility(t *testing.T) {
	m := verifierModule(
		Decl("n", mir2.TyU8, U16(255)),
		&AssignStmt{Target: Var("implicit", mir2.TyU8), Val: U8(2)},
		&VarDeclStmt{Name: "array", Ty: mir2.TyU8, ArrayLen: 2, Initial: []Expr{nil, U8(1)}},
		&WhileStmt{Cond: Var("n", mir2.TyU8), Body: Blk(&ContinueStmt{}, &BreakStmt{})},
		&GotoStmt{Label: "later"}, &LabelStmt{Name: "later"},
		&ReturnStmt{Val: U16(1)},
	)
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyTupleCallUsesAllResultSlots(t *testing.T) {
	makeModule := func() *Module {
		return &Module{Funcs: []*Func{
			{Name: "pair", RetTy: mir2.TyVoid, RetTys: []mir2.Ty{mir2.TyU8, mir2.TyU16}, Body: Blk(&ReturnStmt{Vals: []Expr{U8(10), U16(300)}})},
			{Name: "caller", RetTy: mir2.TyVoid, Body: Blk(&TupleLetStmt{Names: []string{"a", "b"}, Tys: []mir2.Ty{mir2.TyU8, mir2.TyU16}, Call: Call("pair", mir2.TyVoid)})},
		}}
	}
	if err := Verify(makeModule()); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*TupleLetStmt)
		want string
	}{
		{"second result", func(s *TupleLetStmt) { s.Tys[1] = mir2.TyU8 }, "result 1 type"},
		{"arity", func(s *TupleLetStmt) { s.Names = s.Names[:1]; s.Tys = s.Tys[:1] }, "arity"},
		{"arguments", func(s *TupleLetStmt) { s.Call.Args = []Expr{U8(1)} }, "arity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := makeModule()
			tc.edit(m.Funcs[1].Body.Body[0].(*TupleLetStmt))
			if err := Verify(m); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestGraceRejectsBadInputBeforeMutation(t *testing.T) {
	branch := If(Bool(true), Blk(&ReturnStmt{Val: U8(1)}), Blk(&ReturnStmt{Val: U8(2)}))
	m := verifierModule(branch, &BreakStmt{})
	result, err := RunGrace(m, 10)
	if err == nil || result.Applied != 0 || m.Funcs[0].Body.Body[0] != branch {
		t.Fatalf("invalid input rewritten: result=%+v err=%v", result, err)
	}
}
