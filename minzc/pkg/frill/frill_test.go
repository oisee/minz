package frill

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
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

func TestLetIn(t *testing.T) {
	m := compile(t, `
let foo (x : u8) : u8 =
  let y = x + 1 in
  let z = y * 2 in
  z
assert foo 3 == 8
assert foo 10 == 22
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

func TestDo(t *testing.T) {
	m := compile(t, `
let side (x : u8) : IO u8 = x
let test (x : u8) : IO u8 =
  do side x
  x + 1
assert test 5 == 6
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

func TestCompose(t *testing.T) {
	m := compile(t, `
let double (x : u8) : u8 = x + x
let inc (x : u8) : u8 = x + 1
let double_then_inc = double >> inc
assert double_then_inc 5 == 11
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

func TestRecursion(t *testing.T) {
	m := compile(t, `
let factorial (n : u8) : u8 = if n == 0 then 1 else n * factorial (n - 1)
assert factorial 0 == 1
assert factorial 5 == 120
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

func TestMatch(t *testing.T) {
	m := compile(t, `
let classify (x : u8) : u8 =
  match x with
  | 0 -> 10
  | 1 -> 20
  | _ -> 30
assert classify 0 == 10
assert classify 1 == 20
assert classify 5 == 30
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

func TestTupleReturn(t *testing.T) {
	m := compile(t, `
let swap (a : u8) (b : u8) : (u8, u8) = (b, a)
`)
	if len(m.Funcs) != 1 {
		t.Fatalf("expected 1 func, got %d", len(m.Funcs))
	}
}

func TestMathLib(t *testing.T) {
	m := compile(t, `
let add (a : u8) (b : u8) : u8 = a + b
let double (x : u8) : u8 = x + x
let square (x : u8) : u8 = x * x
let max (a : u8) (b : u8) : u8 = if a > b then a else b
let min (a : u8) (b : u8) : u8 = if a < b then a else b
let abs_diff (a : u8) (b : u8) : u8 = if a > b then a - b else b - a
let gcd (a : u8) (b : u8) : u8 = if b == 0 then a else gcd b (a % b)
assert add 3 5 == 8
assert square 4 == 16
assert max 5 3 == 5
assert max 3 5 == 5
assert abs_diff 10 3 == 7
assert abs_diff 3 10 == 7
assert gcd 12 8 == 4
assert gcd 12 0 == 12
`)
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

func TestGcdE2E(t *testing.T) {
	m := compile(t, `
let gcd (a : u8) (b : u8) : u8 = if b == 0 then a else gcd b (a % b)
assert gcd 12 0 == 12
assert gcd 12 8 == 4
assert gcd 8 12 == 4
assert gcd 255 0 == 255
`)
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	t.Log("\n" + asm)
}
