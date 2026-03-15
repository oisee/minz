package objc

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestObjC_InterfaceToStruct(t *testing.T) {
	src := `
@interface Point {
    int x;
    int y;
}
-(int)getX;
-(int)getY;
@end

int test(void) { return 42; }
`
	m, err := Compile(src, "test.m")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Check struct was created.
	if len(m.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(m.Structs))
	}
	st := m.Structs[0]
	if st.Name != "Point" {
		t.Errorf("struct name = %q, want Point", st.Name)
	}
	if len(st.Fields) != 2 {
		t.Errorf("struct fields = %d, want 2", len(st.Fields))
	}
	t.Logf("@interface Point → struct Point { %d fields }", len(st.Fields))

	// Check C function still works.
	if len(m.Funcs) != 1 {
		t.Errorf("expected 1 func (test), got %d", len(m.Funcs))
	}
}

func TestObjC_ImplementationToFunctions(t *testing.T) {
	src := `
@interface Counter {
    int count;
}
-(int)value;
-(void)increment;
@end

@implementation Counter
-(int)value {
    return self->count;
}
-(void)increment {
    self->count = self->count + 1;
}
@end
`
	m, err := Compile(src, "test.m")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Should have 2 methods as HIR functions.
	if len(m.Funcs) < 2 {
		t.Fatalf("expected >=2 funcs, got %d", len(m.Funcs))
	}

	for _, f := range m.Funcs {
		t.Logf("func %s(", f.Name)
		for _, p := range f.Params {
			t.Logf("  %s: %v", p.Name, p.Ty)
		}
		t.Logf(") -> %v", f.RetTy)
	}

	// Check Counter_value exists with self param.
	getValue := findFunc(m, "Counter_value")
	if getValue == nil {
		t.Fatal("Counter_value not found")
	}
	if len(getValue.Params) < 1 || getValue.Params[0].Name != "self" {
		t.Error("Counter_value should have self as first param")
	}
	if getValue.Params[0].Ty != mir2.TyPtr {
		t.Error("self should be TyPtr")
	}
	if getValue.RetTy != mir2.TyI16 {
		t.Errorf("Counter_value retTy = %v, want i16", getValue.RetTy)
	}

	// Check Counter_increment exists.
	incr := findFunc(m, "Counter_increment")
	if incr == nil {
		t.Fatal("Counter_increment not found")
	}
	if incr.RetTy != mir2.TyVoid {
		t.Errorf("Counter_increment retTy = %v, want void", incr.RetTy)
	}
}

func TestObjC_MessageToStaticCall(t *testing.T) {
	src := `
@interface Adder {
    int base;
}
-(int)value;
-(int)add:(int)x;
@end

@implementation Adder
-(int)value {
    return self->base;
}
-(int)add:(int)x {
    return self->base + x;
}
@end

int test(void) {
    int r = [self value];
    return r;
}
`
	m, err := Compile(src, "test.m")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Adder_value, Adder_add, test
	if len(m.Funcs) < 3 {
		t.Fatalf("expected >=3 funcs, got %d", len(m.Funcs))
	}

	for _, f := range m.Funcs {
		t.Logf("func %s — %d params, ret=%v", f.Name, len(f.Params), f.RetTy)
	}

	// Check that test() contains a call (message lowered to static call).
	test := findFunc(m, "test")
	if test == nil {
		t.Fatal("test function not found")
	}
	found := false
	walkExprs(test.Body, func(e hir.Expr) {
		if call, ok := e.(*hir.CallExpr); ok {
			t.Logf("found call: %s(%d args)", call.Fn, len(call.Args))
			found = true
		}
	})
	if !found {
		t.Error("[self value] should have been lowered to a function call")
	}
}

func TestObjC_MultiKeywordMessage(t *testing.T) {
	src := `
@interface Calc {
    int val;
}
-(int)addX:(int)x andY:(int)y;
@end

@implementation Calc
-(int)addX:(int)x andY:(int)y {
    return x + y;
}
@end
`
	m, err := Compile(src, "test.m")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	f := findFunc(m, "Calc_addX_andY")
	if f == nil {
		t.Fatal("Calc_addX_andY not found")
	}
	// self + x + y = 3 params
	if len(f.Params) != 3 {
		t.Errorf("params = %d, want 3 (self, x, y)", len(f.Params))
	}
	t.Logf("-(int)addX:(int)x andY:(int)y → func %s(%v)", f.Name, paramNames(f))
}

func TestObjC_NameMangling(t *testing.T) {
	tests := []struct {
		class, sel, want string
	}{
		{"Foo", "value", "Foo_value"},
		{"Foo", "add:", "Foo_add"},
		{"Foo", "addX:andY:", "Foo_addX_andY"},
		{"Foo", "initWith:label:count:", "Foo_initWith_label_count"},
	}
	for _, tt := range tests {
		got := mangleName(tt.class, tt.sel)
		if got != tt.want {
			t.Errorf("mangle(%s, %s) = %q, want %q", tt.class, tt.sel, got, tt.want)
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func findFunc(m *hir.Module, name string) *hir.Func {
	for _, f := range m.Funcs {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func paramNames(f *hir.Func) []string {
	var names []string
	for _, p := range f.Params {
		names = append(names, p.Name)
	}
	return names
}

func walkExprs(blk *hir.Block, fn func(hir.Expr)) {
	if blk == nil {
		return
	}
	for _, s := range blk.Body {
		switch st := s.(type) {
		case *hir.VarDeclStmt:
			if st.Init != nil {
				fn(st.Init)
			}
		case *hir.ReturnStmt:
			if st.Val != nil {
				fn(st.Val)
			}
		case *hir.ExprStmt:
			fn(st.Expr)
		case *hir.IfStmt:
			fn(st.Cond)
			walkExprs(st.Then, fn)
			walkExprs(st.Else, fn)
		}
	}
}
