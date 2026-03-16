package c89

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/interop"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
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
		for i, g := range m.Globals {
			t.Logf("global[%d] = %q ty=%v", i, g.Name, g.Ty)
		}
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

func TestCompile_CommentAsserts(t *testing.T) {
	src := `
int twice(int x) { return x + x; }
int add(int a, int b) { return a + b; }
// assert twice(5) == 10
// assert add(3, 4) == 7
// assert twice(0) == 0
`
	m, err := Compile(src, "assert.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Asserts) != 3 {
		t.Fatalf("expected 3 asserts, got %d", len(m.Asserts))
	}
	a := m.Asserts[0]
	if a.FuncName != "twice" {
		t.Errorf("assert[0] func = %q, want twice", a.FuncName)
	}
	if len(a.Args) != 1 || a.Args[0] != 5 {
		t.Errorf("assert[0] args = %v, want [5]", a.Args)
	}
	if a.Expected != 10 {
		t.Errorf("assert[0] expected = %d, want 10", a.Expected)
	}
	a1 := m.Asserts[1]
	if a1.FuncName != "add" || len(a1.Args) != 2 || a1.Args[0] != 3 || a1.Args[1] != 4 || a1.Expected != 7 {
		t.Errorf("assert[1] = %+v, want add(3,4)==7", a1)
	}
}

func TestCompile_AssertVia(t *testing.T) {
	src := `
int f(int x) { return x; }
// assert f(42) == 42 via mir2
`
	m, err := Compile(src, "via.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(m.Asserts) != 1 {
		t.Fatalf("expected 1 assert, got %d", len(m.Asserts))
	}
	if m.Asserts[0].Via != "mir2" {
		t.Errorf("via = %q, want mir2", m.Asserts[0].Via)
	}
}

func TestCompile_Sandbox(t *testing.T) {
	src := `
int counter = 0;
int inc(void) { counter = counter + 1; return counter; }
int get(void) { return counter; }
// sandbox "inc test"
// assert inc() == 1
// assert inc() == 2
// assert get() == 2
// end sandbox
// assert inc() == 1
`
	m, err := Compile(src, "sandbox.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// 1 top-level assert (outside sandbox).
	if len(m.Asserts) != 1 {
		t.Fatalf("expected 1 top-level assert, got %d", len(m.Asserts))
	}
	if m.Asserts[0].FuncName != "inc" {
		t.Errorf("top-level assert func = %q, want inc", m.Asserts[0].FuncName)
	}
	// 1 sandbox with 3 asserts.
	if len(m.Sandboxes) != 1 {
		t.Fatalf("expected 1 sandbox, got %d", len(m.Sandboxes))
	}
	sb := m.Sandboxes[0]
	if sb.Name != "inc test" {
		t.Errorf("sandbox name = %q, want %q", sb.Name, "inc test")
	}
	if len(sb.Asserts) != 3 {
		t.Errorf("sandbox asserts = %d, want 3", len(sb.Asserts))
	}
}

func TestAssert_E2E_Pipeline(t *testing.T) {
	src := `
int twice(int x) { return x + x; }
int add(int a, int b) { return a + b; }
// assert twice(5) == 10 via mir2
// assert twice(0) == 0 via mir2
// assert add(3, 4) == 7 via mir2
// assert add(0, 0) == 0 via mir2
`
	hm, err := Compile(src, "e2e.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if len(hm.Asserts) != 4 {
		t.Fatalf("expected 4 asserts, got %d", len(hm.Asserts))
	}
	_, err = pipeline.CompileHIR(hm)
	if err != nil {
		t.Fatalf("pipeline (asserts should pass): %v", err)
	}
	t.Logf("C89 E2E: %d asserts passed via MIR2 VM", len(hm.Asserts))
}

func TestObjCAssert_E2E(t *testing.T) {
	src := `
@interface Box { int value; }
-(int)get;
-(int)addN:(int)n;
@end

@implementation Box
-(int)get { return self->value; }
-(int)addN:(int)n { return self->value + n; }
@end

int identity(int x) { return x; }
// assert identity(7) == 7

// assert-objc Box{value:0}.get() == 0
// assert-objc Box{value:42}.get() == 42
// assert-objc Box{value:10}.addN(5) == 15
`
	hm, err := Compile(src, "objc_test.m")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// 1 plain C assert + 3 ObjC asserts = 4 total
	if len(hm.Asserts) != 4 {
		t.Fatalf("expected 4 asserts, got %d", len(hm.Asserts))
	}
	// Check wrapper functions were generated.
	wrapperCount := 0
	for _, f := range hm.Funcs {
		if strings.HasPrefix(f.Name, "__objc_test_") {
			wrapperCount++
		}
	}
	if wrapperCount != 3 {
		t.Fatalf("expected 3 ObjC wrapper functions, got %d", wrapperCount)
	}
	// Run through pipeline (asserts execute on MIR2 VM).
	_, err = pipeline.CompileHIR(hm)
	if err != nil {
		t.Fatalf("pipeline (ObjC asserts should pass): %v", err)
	}
	t.Logf("ObjC E2E: %d asserts passed (1 C + 3 ObjC)", len(hm.Asserts))
}

func TestObjCMethodChaining(t *testing.T) {
	src := `
@interface Counter {
    int count;
}
-(Counter*)inc;
-(Counter*)add:(int)n;
-(int)value;
-(int)incAndGet;
-(int)addAndGet:(int)n;
-(int)incTwiceAndGet;
@end

@implementation Counter
-(Counter*)inc {
    self->count = self->count + 1;
    return self;
}
-(Counter*)add:(int)n {
    self->count = self->count + n;
    return self;
}
-(int)value { return self->count; }

// Method chaining inside ObjC: [[self inc] value]
-(int)incAndGet {
    return [[self inc] value];
}

// [[self add:n] value]
-(int)addAndGet:(int)n {
    return [[self add:n] value];
}

// [[[self inc] inc] value]
-(int)incTwiceAndGet {
    return [[[self inc] inc] value];
}

@end

int id(int x) { return x; }
// assert id(1) == 1 via mir2

// Method chaining via assert-objc:
// assert-objc Counter{count:0}.incAndGet() == 1
// assert-objc Counter{count:5}.addAndGet(10) == 15
// assert-objc Counter{count:0}.incTwiceAndGet() == 2
`
	hm, err := Compile(src, "chain_test.m")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	t.Logf("funcs=%d asserts=%d", len(hm.Funcs), len(hm.Asserts))

	_, err = pipeline.CompileHIR(hm)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	t.Logf("method chaining: %d asserts passed", len(hm.Asserts))
}

func TestCorpus_AllExamples(t *testing.T) {
	corpusDir := filepath.Join("..", "..", "..", "examples", "c89")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Skipf("corpus dir not found: %v", err)
	}
	totalAsserts := 0
	passedAsserts := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".c" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(corpusDir, e.Name()))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var hm *hir.Module
			if strings.Contains(string(src), "#import") {
				absDir, _ := filepath.Abs(corpusDir)
				hm, err = CompileWithOpts(string(src), e.Name(), CompileOpts{
					BaseDir: absDir,
					Resolver: &interop.Resolver{
						BaseDir: absDir,
						Compilers: map[string]interop.CompileFunc{
							".c": Compile,
						},
					},
				})
			} else {
				hm, err = Compile(string(src), e.Name())
			}
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			t.Logf("  %d funcs, %d asserts, %d sandboxes",
				len(hm.Funcs), len(hm.Asserts), len(hm.Sandboxes))

			mir2Asserts := 0
			for _, a := range hm.Asserts {
				if a.Via == "mir2" {
					mir2Asserts++
				}
			}
			totalAsserts += mir2Asserts
			if mir2Asserts == 0 {
				return
			}
			_, err = pipeline.CompileHIR(hm)
			if err != nil {
				t.Logf("  pipeline: %v", err)
			} else {
				passedAsserts += mir2Asserts
				t.Logf("  %d/%d mir2 asserts passed", mir2Asserts, mir2Asserts)
			}
		})
	}
	t.Logf("TOTAL: %d/%d mir2 asserts passed across corpus", passedAsserts, totalAsserts)
}

func TestCorpus_ObjCExamples(t *testing.T) {
	corpusDir := filepath.Join("..", "..", "..", "examples", "objc")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Skipf("objc corpus dir not found: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".m" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(corpusDir, e.Name()))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			hm, err := Compile(string(src), e.Name())
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			t.Logf("  %d funcs, %d asserts", len(hm.Funcs), len(hm.Asserts))
			if len(hm.Asserts) == 0 {
				return
			}
			_, err = pipeline.CompileHIR(hm)
			if err != nil {
				t.Fatalf("pipeline: %v", err)
			}
			t.Logf("  all %d asserts passed", len(hm.Asserts))
		})
	}
}
