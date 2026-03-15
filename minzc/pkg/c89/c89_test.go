package c89

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestCompile_VoidFunc(t *testing.T) {
	m, err := Compile("void noop(void) {}", "test.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if m.Name != "test" {
		t.Errorf("Name = %q, want %q", m.Name, "test")
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "noop" {
		t.Errorf("func name = %q, want %q", f.Name, "noop")
	}
	if f.RetTy != mir2.TyVoid {
		t.Errorf("retTy = %v, want void", f.RetTy)
	}
}

func TestCompile_AddFunction(t *testing.T) {
	src := `
int add(int a, int b) {
    return a + b;
}
`
	m, err := Compile(src, "add.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "add" {
		t.Errorf("func name = %q, want %q", f.Name, "add")
	}
	if len(f.Params) != 2 {
		t.Errorf("params = %d, want 2", len(f.Params))
	}
	if f.RetTy != mir2.TyI16 {
		t.Errorf("retTy = %v, want i16", f.RetTy)
	}
}

func TestCompile_Global(t *testing.T) {
	src := `
int counter = 0;
void inc(void) { counter = counter + 1; }
`
	m, err := Compile(src, "glob.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Globals) != 1 {
		t.Fatalf("got %d globals, want 1", len(m.Globals))
	}
	if m.Globals[0].Name != "counter" {
		t.Errorf("global name = %q, want %q", m.Globals[0].Name, "counter")
	}
}

func TestCompile_IfElse(t *testing.T) {
	src := `
int max(int a, int b) {
    if (a > b) {
        return a;
    } else {
        return b;
    }
}
`
	m, err := Compile(src, "max.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(m.Funcs))
	}
	if m.Funcs[0].Name != "max" {
		t.Errorf("func name = %q, want %q", m.Funcs[0].Name, "max")
	}
}

func TestCompile_WhileLoop(t *testing.T) {
	src := `
int sum_to(int n) {
    int total = 0;
    int i = 0;
    while (i < n) {
        total = total + i;
        i = i + 1;
    }
    return total;
}
`
	m, err := Compile(src, "loop.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(m.Funcs))
	}
}

func TestCompile_ForLoop(t *testing.T) {
	src := `
int sum_for(int n) {
    int total = 0;
    for (int i = 0; i < n; i = i + 1) {
        total = total + i;
    }
    return total;
}
`
	m, err := Compile(src, "forloop.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(m.Funcs))
	}
}

func TestCompile_Pointer(t *testing.T) {
	src := `
void store(int *p, int val) {
    *p = val;
}
`
	m, err := Compile(src, "ptr.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("got %d funcs, want 1", len(m.Funcs))
	}
	if len(m.Funcs[0].Params) != 2 {
		t.Errorf("params = %d, want 2", len(m.Funcs[0].Params))
	}
	// First param should be pointer type.
	if m.Funcs[0].Params[0].Ty != mir2.TyPtr {
		t.Errorf("param[0] ty = %v, want ptr", m.Funcs[0].Params[0].Ty)
	}
}

func TestCompile_TypeMapping(t *testing.T) {
	src := `
unsigned char byte_fn(unsigned char a) { return a; }
int word_fn(int a) { return a; }
`
	m, err := Compile(src, "types.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Funcs) != 2 {
		t.Fatalf("got %d funcs, want 2", len(m.Funcs))
	}
	if m.Funcs[0].RetTy != mir2.TyU8 {
		t.Errorf("byte_fn retTy = %v, want u8", m.Funcs[0].RetTy)
	}
	if m.Funcs[1].RetTy != mir2.TyI16 {
		t.Errorf("word_fn retTy = %v, want i16", m.Funcs[1].RetTy)
	}
}

func TestCompile_MultipleReturns(t *testing.T) {
	src := `
int abs(int x) {
    if (x < 0) return -x;
    return x;
}
`
	_, err := Compile(src, "abs.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
}
