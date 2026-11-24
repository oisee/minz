package hir

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// CheckIterChains runs before Verify, so every shape Verify exists to reject
// must pass through it without a panic and without an error of its own.
func TestCheckIterChainsToleratesMalformedHIR(t *testing.T) {
	u8 := mir2.Ty(mir2.TyU8)
	fn := func(body ...Stmt) *Module {
		return &Module{Funcs: []*Func{{Name: "bad", RetTy: u8, Body: Blk(body...)}}}
	}
	chain := func(args ...Expr) *CallExpr { return &CallExpr{Fn: "sum", Args: args, Ty: mir2.TyU16, Method: true} }
	cases := map[string]*Module{
		"nil module":           nil,
		"nil func":             {Funcs: []*Func{nil}},
		"nil body":             {Funcs: []*Func{{Name: "bad", RetTy: u8}}},
		"nil return stmt":      fn((*ReturnStmt)(nil)),
		"nil stmt interface":   fn(nil),
		"absent variable":      fn(&ReturnStmt{Val: Var("absent", u8)}),
		"nil expr stmt":        fn(&ExprStmt{}),
		"typed nil expr":       fn(&ExprStmt{Expr: (*CallExpr)(nil)}),
		"typed nil arg":        fn(&ReturnStmt{Val: chain((*VarRefExpr)(nil), (*IntLitExpr)(nil))}),
		"nil arg":              fn(&ReturnStmt{Val: chain(nil)}),
		"nil if blocks":        fn(&IfStmt{Cond: (*BinExpr)(nil), Then: nil, Else: (*Block)(nil)}),
		"nil while":            fn(&WhileStmt{}),
		"nil method call line": {Funcs: []*Func{{Name: "bad", RetTy: u8, Body: Blk()}}, MethodCallLines: map[*CallExpr]int{nil: 3}},
		"nil store":            fn(&StoreStmt{}),
		"nil assign":           fn(&AssignStmt{}),
		"nil var decl init":    fn(&VarDeclStmt{Name: "x", Ty: u8}),
		"nil switch":           fn(&SwitchStmt{Cases: []*SwitchCase{nil}}),
		"nil foreach body":     fn(&ForEachStmt{}),
		"nil range for body":   fn(&ForRangeStmt{}),
		"nil block stmt":       fn((*Block)(nil)),
	}
	// Arity errors on these are diagnostics, not crashes: only require that
	// CheckIterChains returns.
	shapes := map[string]*Module{
		"nil callback function": {Funcs: []*Func{nil, {Name: "bad", RetTy: u8, Body: Blk(&ExprStmt{Expr: &CallExpr{Fn: "forEach", Args: []Expr{Var("a", u8), Var("cb", u8), nil}, Method: true}})}}},
		"nil nested stage":      fn(&ReturnStmt{Val: chain(&CallExpr{Fn: "take", Method: true}, nil)}),
		"method call no args":   fn(&ReturnStmt{Val: chain()}),
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			if err := CheckIterChains(m); err != nil {
				t.Fatalf("CheckIterChains reported %v; malformed HIR is Verify's to report", err)
			}
		})
	}
	for name, m := range shapes {
		t.Run(name, func(t *testing.T) { _ = CheckIterChains(m) })
	}
}

// CallExpr.Method is absent from the reflective HIR fingerprints while it is
// false, so function-site and loop-profile identities of programs that never
// set it are those of the compiler before the field existed.
func TestCallMethodMarkFingerprint(t *testing.T) {
	fp := func(e Expr) string {
		var b bytes.Buffer
		if err := fingerprintValue(&b, reflect.ValueOf(e), map[uintptr]bool{}); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	plain := fp(&CallExpr{Fn: "sum", Ty: mir2.TyU16})
	if strings.Contains(plain, "Method") {
		t.Fatalf("unset Method appears in the fingerprint: %s", plain)
	}
	if marked := fp(&CallExpr{Fn: "sum", Ty: mir2.TyU16, Method: true}); marked == plain || !strings.Contains(marked, `"Method"=`) {
		t.Fatalf("a set Method must change the fingerprint: %s", marked)
	}
}
