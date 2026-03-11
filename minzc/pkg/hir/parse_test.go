package hir_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

// TestParseHIR_Add verifies that a simple function signature round-trips correctly.
func TestParseHIR_Add(t *testing.T) {
	src := `; HIR module: test

fun @add(a: u16, b: u16) -> u16
  return ((a:u16) + (b:u16)):u16
`
	m, err := hir.ParseHIR(src, "test")
	if err != nil {
		t.Fatalf("ParseHIR: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "add" {
		t.Errorf("func name: got %q, want %q", f.Name, "add")
	}
	if len(f.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(f.Params))
	}
	if f.Params[0].Name != "a" || f.Params[0].Ty != mir2.TyU16 {
		t.Errorf("param 0: got %s:%s, want a:u16", f.Params[0].Name, f.Params[0].Ty)
	}
	if f.Params[1].Name != "b" || f.Params[1].Ty != mir2.TyU16 {
		t.Errorf("param 1: got %s:%s, want b:u16", f.Params[1].Name, f.Params[1].Ty)
	}
	if f.RetTy != mir2.TyU16 {
		t.Errorf("ret type: got %s, want u16", f.RetTy)
	}
	if f.Body == nil {
		t.Fatal("expected non-nil body")
	}
	if len(f.Body.Body) == 0 {
		t.Fatal("expected at least one statement in body")
	}
}

// TestParseHIR_RoundTrip checks that Dump() output can be re-parsed with
// matching function names, parameter counts, and struct definitions.
func TestParseHIR_RoundTrip(t *testing.T) {
	// Build a module manually.
	st := &mir2.StructTy{
		Name: "Point",
		Fields: []mir2.StructField{
			{Name: "x", Ty: mir2.TyU8},
			{Name: "y", Ty: mir2.TyU8},
		},
	}
	m := &hir.Module{
		Name:    "roundtrip",
		Structs: []*mir2.StructTy{st},
		Globals: []mir2.Global{
			{Name: "counter", Ty: mir2.TyU16},
			{Name: "TABLE", Ty: mir2.NewArray(mir2.TyU8, 256), IsConst: true},
		},
		Strings: []string{"hello", "world"},
		Funcs: []*hir.Func{
			{
				Name:   "add",
				Params: []hir.Param{{Name: "a", Ty: mir2.TyU16}, {Name: "b", Ty: mir2.TyU16}},
				RetTy:  mir2.TyU16,
				Body: hir.Blk(
					hir.Ret(hir.Add(hir.Var("a", mir2.TyU16), hir.Var("b", mir2.TyU16), mir2.TyU16)),
				),
			},
			{
				Name:   "noop",
				Params: nil,
				RetTy:  mir2.TyVoid,
				Body:   hir.Blk(hir.RetVoid()),
			},
		},
	}

	dumped := m.Dump()
	t.Logf("Dump output:\n%s", dumped)

	m2, err := hir.ParseHIR(dumped, "roundtrip")
	if err != nil {
		t.Fatalf("ParseHIR: %v", err)
	}

	// Check module name
	if m2.Name != "roundtrip" {
		t.Errorf("module name: got %q want %q", m2.Name, "roundtrip")
	}

	// Check structs
	if len(m2.Structs) != 1 {
		t.Fatalf("structs: got %d, want 1", len(m2.Structs))
	}
	if m2.Structs[0].Name != "Point" {
		t.Errorf("struct name: got %q, want Point", m2.Structs[0].Name)
	}
	if len(m2.Structs[0].Fields) != 2 {
		t.Errorf("struct fields: got %d, want 2", len(m2.Structs[0].Fields))
	}

	// Check globals
	if len(m2.Globals) != 2 {
		t.Fatalf("globals: got %d, want 2", len(m2.Globals))
	}
	if m2.Globals[0].Name != "counter" {
		t.Errorf("global 0 name: got %q, want counter", m2.Globals[0].Name)
	}
	if !m2.Globals[1].IsConst {
		t.Errorf("global 1 should be const")
	}

	// Check strings
	if len(m2.Strings) != 2 {
		t.Fatalf("strings: got %d, want 2", len(m2.Strings))
	}
	if m2.Strings[0] != "hello" || m2.Strings[1] != "world" {
		t.Errorf("strings: got %v, want [hello world]", m2.Strings)
	}

	// Check functions
	if len(m2.Funcs) != 2 {
		t.Fatalf("funcs: got %d, want 2", len(m2.Funcs))
	}
	if m2.Funcs[0].Name != "add" {
		t.Errorf("func 0 name: got %q, want add", m2.Funcs[0].Name)
	}
	if len(m2.Funcs[0].Params) != 2 {
		t.Errorf("func 0 params: got %d, want 2", len(m2.Funcs[0].Params))
	}
	if m2.Funcs[0].RetTy != mir2.TyU16 {
		t.Errorf("func 0 retTy: got %s, want u16", m2.Funcs[0].RetTy)
	}
	if m2.Funcs[1].Name != "noop" {
		t.Errorf("func 1 name: got %q, want noop", m2.Funcs[1].Name)
	}
}

// TestParseHIR_Compile parses a simple HIR source and compiles through the pipeline.
func TestParseHIR_Compile(t *testing.T) {
	src := `; HIR module: simple

fun @double(n: u8) -> u8
  return ((n:u8) + (n:u8)):u8
`
	m, err := hir.ParseHIR(src, "simple")
	if err != nil {
		t.Fatalf("ParseHIR: %v", err)
	}

	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("CompileHIR: %v", err)
	}
	if asm == "" {
		t.Fatal("CompileHIR returned empty assembly")
	}
	t.Logf("Assembly:\n%s", asm)

	// Sanity: should contain the function label
	if !strings.Contains(asm, "double") {
		t.Errorf("assembly should contain 'double', got:\n%s", asm)
	}
}

// TestParseHIR_IfWhile checks that if/while structures are parsed.
func TestParseHIR_IfWhile(t *testing.T) {
	src := `; HIR module: ctrl

fun @fib(n: u8) -> u8
  if ((n:u8) <= 1:u8):bool
    return (n:u8)
  var x: u8 = 0:u8
  while ((x:u8) < (n:u8)):bool
    (x:u8) = ((x:u8) + 1:u8):u8
  return (x:u8)
`
	m, err := hir.ParseHIR(src, "ctrl")
	if err != nil {
		t.Fatalf("ParseHIR: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Body == nil || len(f.Body.Body) == 0 {
		t.Fatal("expected non-empty body")
	}
	// First stmt should be IfStmt
	if _, ok := f.Body.Body[0].(*hir.IfStmt); !ok {
		t.Errorf("first stmt should be IfStmt, got %T", f.Body.Body[0])
	}
}

// TestParseHIR_VoidFunc checks that a void function parses correctly.
func TestParseHIR_VoidFunc(t *testing.T) {
	src := `; HIR module: void_test

fun @nothing()
  return
`
	m, err := hir.ParseHIR(src, "void_test")
	if err != nil {
		t.Fatalf("ParseHIR: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.RetTy != mir2.TyVoid {
		t.Errorf("ret type: got %s, want void", f.RetTy)
	}
}

// TestParseHIR_ForRange checks for-range statements.
func TestParseHIR_ForRange(t *testing.T) {
	src := `; HIR module: fortest

fun @sum(n: u8) -> u16
  var s: u16 = 0:u16
  for i in 0:u8..(n:u8)
    (s:u16) = ((s:u16) + cast((i:u8)):u16):u16
  return (s:u16)
`
	m, err := hir.ParseHIR(src, "fortest")
	if err != nil {
		t.Fatalf("ParseHIR: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Body == nil {
		t.Fatal("nil body")
	}
	// Find the for statement
	found := false
	for _, s := range f.Body.Body {
		if _, ok := s.(*hir.ForRangeStmt); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ForRangeStmt in body")
	}
}
