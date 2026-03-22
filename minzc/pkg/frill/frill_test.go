package frill

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func compile(t *testing.T, src string) *hir.Module {
	t.Helper()
	m, err := Compile(src, "test")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return m
}

func TestBasicAdd(t *testing.T) {
	m := compile(t, `let add (a : u8) (b : u8) : u8 = a + b`)
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "add" {
		t.Errorf("name = %q", f.Name)
	}
	if len(f.Params) != 2 {
		t.Errorf("params = %d", len(f.Params))
	}
	if f.RetTy != mir2.TyU8 {
		t.Errorf("retty = %v", f.RetTy)
	}
}

func TestIfExpr(t *testing.T) {
	m := compile(t, `let max (a : u8) (b : u8) : u8 = if a > b then a else b`)
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
}

func TestAssert(t *testing.T) {
	m := compile(t, `
let double (x : u8) : u8 = x + x
assert double 5 == 10
assert double 0 == 0
`)
	if len(m.Funcs) != 1 {
		t.Errorf("expected 1 func, got %d", len(m.Funcs))
	}
	if len(m.Asserts) != 2 {
		t.Errorf("expected 2 asserts, got %d", len(m.Asserts))
	}
	if m.Asserts[0].Expected != 10 {
		t.Errorf("assert[0] expected=%d", m.Asserts[0].Expected)
	}
}

func TestMultipleFuncs(t *testing.T) {
	m := compile(t, `
let inc (x : u8) : u8 = x + 1
let dec (x : u8) : u8 = x - 1
let double (x : u8) : u8 = x + x
assert inc 5 == 6
assert dec 5 == 4
assert double 7 == 14
`)
	if len(m.Funcs) != 3 {
		t.Errorf("expected 3 funcs, got %d", len(m.Funcs))
	}
	if len(m.Asserts) != 3 {
		t.Errorf("expected 3 asserts, got %d", len(m.Asserts))
	}
}

func TestPipe(t *testing.T) {
	m := compile(t, `
let double (x : u8) : u8 = x + x
let inc (x : u8) : u8 = x + 1
let pipe_test (x : u8) : u8 = x |> double |> inc
assert pipe_test 3 == 7
assert pipe_test 10 == 21
`)
	mir2mod := hir.LowerModule(m)
	for _, a := range m.Asserts {
		result, err := mir2.Eval1(mir2mod, a.FuncName, a.Args...)
		if err != nil {
			t.Errorf("assert %s: %v", a.Source, err)
			continue
		}
		if result != a.Expected {
			t.Errorf("assert %s: got %d, want %d", a.Source, result, a.Expected)
		}
	}
}

func TestPipeWithExtraArgs(t *testing.T) {
	m := compile(t, `
let add (a : u8) (b : u8) : u8 = a + b
let test (x : u8) : u8 = x |> add 10
assert test 3 == 13
assert test 0 == 10
`)
	mir2mod := hir.LowerModule(m)
	for _, a := range m.Asserts {
		result, err := mir2.Eval1(mir2mod, a.FuncName, a.Args...)
		if err != nil {
			t.Errorf("assert %s: %v", a.Source, err)
			continue
		}
		if result != a.Expected {
			t.Errorf("assert %s: got %d, want %d", a.Source, result, a.Expected)
		}
	}
}

func TestE2E_MIR2(t *testing.T) {
	m := compile(t, `
let add (a : u8) (b : u8) : u8 = a + b
let square (x : u8) : u8 = x * x
assert add 3 5 == 8
assert square 4 == 16
`)
	// Lower to MIR2 and run asserts via VM
	mir2mod := hir.LowerModule(m)
	for _, a := range m.Asserts {
		result, err := mir2.Eval1(mir2mod, a.FuncName, a.Args...)
		if err != nil {
			t.Errorf("assert %s: VM error: %v", a.Source, err)
			continue
		}
		if result != a.Expected {
			t.Errorf("assert %s: got %d, want %d", a.Source, result, a.Expected)
		}
	}
}
