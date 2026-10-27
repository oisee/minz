package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// foldLetType parses `fun execute() { <body> }` over the shared globals and
// returns the type the `let s` binding and the fold call itself were given.
func foldLetType(t *testing.T, body string) (letTy, callTy mir2.Ty) {
	t.Helper()
	src := "global a: [u8; 4]\nglobal n: u16\nglobal r: u16\n" +
		"fun add16(acc: u16, x: u8) -> u16 { return acc + u16(x) }\n" +
		"fun execute() {\n" + body + "\n}"
	m, err := nanz.Parse(src, "fold_lambda")
	if err != nil {
		t.Fatal(err)
	}
	ex := m.FuncByName("execute")
	if ex == nil || len(ex.Body.Body) == 0 {
		t.Fatalf("execute body missing: %v", m.Funcs)
	}
	decl, ok := ex.Body.Body[0].(*hir.VarDeclStmt)
	if !ok {
		t.Fatalf("first statement is %T, want *hir.VarDeclStmt", ex.Body.Body[0])
	}
	call, ok := decl.Init.(*hir.CallExpr)
	if !ok {
		t.Fatalf("let init is %T, want *hir.CallExpr", decl.Init)
	}
	return decl.Ty, call.Ty
}

// The lambda literal and the named callback must infer the same accumulator
// type, for fold and for reduce, and for both the `let` binding and the call.
func TestFoldLambdaLetBindingKeepsAccumulatorType(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"fold_lambda", "let s = a.filter(|x: u8| x >= 128).fold(u16(0), |acc: u16, x: u8| acc + u16(x), n)"},
		{"fold_lambda_block", "let s = a.filter(|x: u8| x >= 128).fold(u16(0), |acc: u16, x: u8| { return acc + u16(x) }, n)"},
		{"fold_lambda_untyped_init", "let s = a.filter(|x: u8| x >= 128).fold(0, |acc: u16, x: u8| acc + u16(x), n)"},
		// A bare function name used as a value is an AddrOfExpr, the spelling
		// the named-callback path always used.
		{"fold_named", "let s = a.filter(|x: u8| x >= 128).fold(u16(0), add16, n)"},
		{"reduce_lambda", "let s = a.filter(|x: u8| x >= 128).reduce(u16(0), |acc: u16, x: u8| acc + u16(x), n)"},
		{"reduce_named", "let s = a.filter(|x: u8| x >= 128).reduce(u16(0), add16, n)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			letTy, callTy := foldLetType(t, c.body+"\nr = s")
			if letTy != mir2.TyU16 {
				t.Fatalf("let s type %v, want u16", letTy)
			}
			if callTy != mir2.TyU16 {
				t.Fatalf("fold call type %v, want u16", callTy)
			}
		})
	}
}

// An untyped u8 lambda keeps inferring u8 — the fix must not widen every fold.
func TestFoldLambdaU8AccumulatorStillU8(t *testing.T) {
	letTy, callTy := foldLetType(t, "let s = a.fold(u8(0), |acc: u8, x: u8| acc + x, n)\nr = s")
	if letTy != mir2.TyU8 || callTy != mir2.TyU8 {
		t.Fatalf("accumulator type let=%v call=%v, want u8", letTy, callTy)
	}
}

// A callback that returns no value and an init that names no type is a source
// error, not a silent u8 accumulator.
func TestFoldUnknownAccumulatorIsRejected(t *testing.T) {
	for _, src := range []string{
		"global a: [u8; 4]\nglobal n: u16\nglobal r: u16\nfun execute() { let s = a.fold(0, |acc: u8, x: u8| { r = u16(x) }, n) }",
		"global a: [u8; 4]\nglobal n: u16\nglobal r: u16\nfun nop(acc: u8, x: u8) { }\nglobal z: u8\nfun execute() { let s = a.fold(nop(z), nop, n) }",
		// Same refusal when the chain sits in a return rather than a let.
		"global a: [u8; 4]\nglobal n: u16\nfun f() -> u16 { return a.fold(0, |acc: u8, x: u8| { }, n) }",
	} {
		if _, err := nanz.Parse(src, "fold_lambda"); err == nil {
			t.Fatalf("parse succeeded, want accumulator-type diagnostic:\n%s", src)
		} else if !strings.Contains(err.Error(), "accumulator type") {
			t.Fatalf("error %q does not name the accumulator type", err)
		}
	}
}

// Return-position folds get the accumulator type from the same helper.
func TestFoldLambdaReturnPositionType(t *testing.T) {
	m, err := nanz.Parse(`fun add16(acc: u16, x: u8) -> u16 { return acc + u16(x) }
global a: [u8; 4]
global n: u16
fun f() -> u16 { return a.filter(|x: u8| x >= 128).fold(u16(0), |acc: u16, x: u8| acc + u16(x), n) }
fun g() -> u16 { return a.filter(|x: u8| x >= 128).fold(u16(0), add16, n) }`, "fold_lambda")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"f", "g"} {
		call := m.FuncByName(name).Body.Body[0].(*hir.ReturnStmt).Val.(*hir.CallExpr)
		if call.Ty != mir2.TyU16 {
			t.Fatalf("%s: fold type %v, want u16", name, call.Ty)
		}
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
}
