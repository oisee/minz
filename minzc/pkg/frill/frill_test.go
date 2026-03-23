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

func TestLetIn(t *testing.T) {
	m := compile(t, `
let dist (a : u8) (b : u8) : u8 =
  let sum = a + b in
  let diff = a - b in
  sum + diff

assert dist 10 3 == 20
assert dist 5 5 == 10
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

func TestLetInChain(t *testing.T) {
	m := compile(t, `
let compute (x : u8) : u8 =
  let a = x + 1 in
  let b = a + a in
  let c = b * 2 in
  c

assert compute 3 == 16
assert compute 0 == 4
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
let describe (x : u8) : u8 =
  match x with
  | 0 -> 10
  | 1 -> 20
  | 2 -> 30
  | _ -> 99
  end

assert describe 0 == 10
assert describe 1 == 20
assert describe 2 == 30
assert describe 5 == 99
assert describe 255 == 99
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

func TestMatchExhaustive(t *testing.T) {
	m := compile(t, `
let day_type (d : u8) : u8 =
  match d with
  | 0 -> 0
  | 6 -> 0
  | _ -> 1
  end

assert day_type 0 == 0
assert day_type 6 == 0
assert day_type 1 == 1
assert day_type 3 == 1
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

func TestADT(t *testing.T) {
	m := compile(t, `
type Color = Red | Green | Blue

let is_warm (c : u8) : u8 =
  match c with
  | Red -> 1
  | _ -> 0
  end

assert is_warm 0 == 1
assert is_warm 1 == 0
assert is_warm 2 == 0
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

func TestADTConstructorExpr(t *testing.T) {
	m := compile(t, `
type Direction = North | South | East | West

let go_north (x : u8) : u8 = North
let go_south (x : u8) : u8 = South
let is_vertical (d : u8) : u8 =
  match d with
  | North -> 1
  | South -> 1
  | _ -> 0
  end

assert go_north 0 == 0
assert go_south 0 == 1
assert is_vertical 0 == 1
assert is_vertical 1 == 1
assert is_vertical 2 == 0
assert is_vertical 3 == 0
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
