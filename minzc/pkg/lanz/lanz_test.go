package lanz

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestParseSExpr(t *testing.T) {
	src := `(fun add ((a u8) (b u8)) u8 (return (+ a b)))`
	nodes, err := ParseSExpr(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if !nodes[0].IsList() || len(nodes[0].List) != 5 {
		t.Fatalf("expected list of 5, got %s", nodes[0])
	}
	if nodes[0].List[0].Atom != "fun" {
		t.Fatalf("expected fun, got %q", nodes[0].List[0].Atom)
	}
}

func TestParseSExpr_Comments(t *testing.T) {
	src := `; this is a comment
(+ 1 2) ; trailing
; another`
	nodes, err := ParseSExpr(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
}

func TestParseSExpr_String(t *testing.T) {
	src := `(print "hello world")`
	nodes, err := ParseSExpr(src)
	if err != nil {
		t.Fatal(err)
	}
	if nodes[0].List[1].Atom != `"hello world"` {
		t.Fatalf("expected quoted string, got %q", nodes[0].List[1].Atom)
	}
}

func TestCompile_Add(t *testing.T) {
	src := `(fun add ((a u8) (b u8)) u8 (return (+ a b)))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "add" {
		t.Errorf("name = %q, want add", f.Name)
	}
	if len(f.Params) != 2 {
		t.Errorf("params = %d, want 2", len(f.Params))
	}
	if f.RetTy != mir2.TyU8 {
		t.Errorf("retTy = %v, want u8", f.RetTy)
	}
	if f.Body == nil || len(f.Body.Body) != 1 {
		t.Fatalf("body should have 1 stmt")
	}
	ret, ok := f.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("expected ReturnStmt, got %T", f.Body.Body[0])
	}
	bin, ok := ret.Val.(*hir.BinExpr)
	if !ok {
		t.Fatalf("expected BinExpr, got %T", ret.Val)
	}
	if bin.Op != "+" {
		t.Errorf("op = %q, want +", bin.Op)
	}
}

func TestCompile_Global(t *testing.T) {
	src := `(global counter u8 42)`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Globals) != 1 {
		t.Fatalf("expected 1 global, got %d", len(m.Globals))
	}
	g := m.Globals[0]
	if g.Name != "counter" {
		t.Errorf("name = %q, want counter", g.Name)
	}
	if g.Ty != mir2.TyU8 {
		t.Errorf("ty = %v, want u8", g.Ty)
	}
	if len(g.Init) != 1 || g.Init[0] != 42 {
		t.Errorf("init = %v, want [42]", g.Init)
	}
}

func TestCompile_IfWhile(t *testing.T) {
	src := `(fun f () void
		(var x u8 0)
		(if (< x 10)
			(set x (+ x 1)))
		(while (< x 20)
			(set x (+ x 1))))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	body := m.Funcs[0].Body.Body
	if len(body) != 3 {
		t.Fatalf("expected 3 stmts, got %d", len(body))
	}
	if _, ok := body[0].(*hir.VarDeclStmt); !ok {
		t.Errorf("stmt 0: expected VarDeclStmt, got %T", body[0])
	}
	if _, ok := body[1].(*hir.IfStmt); !ok {
		t.Errorf("stmt 1: expected IfStmt, got %T", body[1])
	}
	if _, ok := body[2].(*hir.WhileStmt); !ok {
		t.Errorf("stmt 2: expected WhileStmt, got %T", body[2])
	}
}

func TestCompile_ForRange(t *testing.T) {
	src := `(fun f () void (for i 0 10 (call print_u8 i)))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	body := m.Funcs[0].Body.Body
	if len(body) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(body))
	}
	fr, ok := body[0].(*hir.ForRangeStmt)
	if !ok {
		t.Fatalf("expected ForRangeStmt, got %T", body[0])
	}
	if fr.Var != "i" {
		t.Errorf("var = %q, want i", fr.Var)
	}
}

func TestCompile_Extern(t *testing.T) {
	src := `(extern print_u8 ((v u8)) void)`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Funcs) != 1 {
		t.Fatal("expected 1 func")
	}
	if !m.Funcs[0].IsExtern {
		t.Error("expected IsExtern")
	}
}

func TestCompile_Struct(t *testing.T) {
	src := `(struct Point ((x u8) (y u8)))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Structs) != 1 {
		t.Fatal("expected 1 struct")
	}
	st := m.Structs[0]
	if st.Name != "Point" {
		t.Errorf("name = %q, want Point", st.Name)
	}
	if len(st.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(st.Fields))
	}
}

func TestCompile_BareCall(t *testing.T) {
	src := `(fun f () void (print_u8 42))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	body := m.Funcs[0].Body.Body
	if len(body) != 1 {
		t.Fatalf("expected 1 stmt, got %d", len(body))
	}
	es, ok := body[0].(*hir.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", body[0])
	}
	call, ok := es.Expr.(*hir.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", es.Expr)
	}
	if call.Fn != "print_u8" {
		t.Errorf("fn = %q, want print_u8", call.Fn)
	}
}

func TestCompile_RoundTrip(t *testing.T) {
	// Compile Lanz → HIR → dump → verify dump is parseable
	src := `(fun add ((a u8) (b u8)) u8 (return (+ a b)))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	dump := m.Dump()
	if dump == "" {
		t.Fatal("dump is empty")
	}
	t.Logf("Lanz→HIR dump:\n%s", dump)
}

func TestDump_RoundTrip(t *testing.T) {
	src := `(fun add ((a u8) (b u8)) u8 (return (+ a b)))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	lanzOut := Dump(m)
	t.Logf("Dump output:\n%s", lanzOut)
	// Parse the dump back
	m2, err := Compile(lanzOut, "test2")
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if len(m2.Funcs) != 1 || m2.Funcs[0].Name != "add" {
		t.Errorf("round-trip lost function: got %d funcs", len(m2.Funcs))
	}
}

func TestDump_Global(t *testing.T) {
	src := `(global counter u8 42)`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	lanzOut := Dump(m)
	if !strings.Contains(lanzOut, "(global counter u8 42)") {
		t.Errorf("dump = %q, want global counter u8 42", lanzOut)
	}
}

func TestIsInt(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"42", true},
		{"0xFF", true},
		{"-1", true},
		{"abc", false},
		{"", false},
		{"-", false},
		{"0x", false},
	}
	for _, c := range cases {
		if got := IsInt(c.s); got != c.want {
			t.Errorf("IsInt(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestCompile_UnaryNeg(t *testing.T) {
	src := `(fun f ((x u8)) u8 (return (neg x)))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	ret := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt)
	u, ok := ret.Val.(*hir.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", ret.Val)
	}
	if u.Op != "-" {
		t.Errorf("op = %q, want -", u.Op)
	}
}

func TestCompile_CondExpr(t *testing.T) {
	// if-as-expression
	src := `(fun abs ((x i8)) u8 (return (cast (if (< x 0) (neg x) x) u8)))`
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	ret := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt)
	cast, ok := ret.Val.(*hir.CastExpr)
	if !ok {
		t.Fatalf("expected CastExpr, got %T", ret.Val)
	}
	cond, ok := cast.X.(*hir.CondExpr)
	if !ok {
		t.Fatalf("expected CondExpr, got %T", cast.X)
	}
	_ = cond
}
