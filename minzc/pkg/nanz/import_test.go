package nanz_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// setupModuleDir creates a temporary directory with a module file for import testing.
func setupModuleDir(t *testing.T) (baseDir string, cleanup func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "nanz_import_test")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	// Create mylib/math.nanz
	libDir := filepath.Join(dir, "mylib")
	os.MkdirAll(libDir, 0o755)
	os.WriteFile(filepath.Join(libDir, "math.nanz"), []byte(`
fun add(a: u8, b: u8) -> u8 {
    return (a + b)
}

fun double(x: u8) -> u8 {
    return (x + x)
}
`), 0o644)

	// Create mylib/enums.nanz
	os.WriteFile(filepath.Join(libDir, "enums.nanz"), []byte(`
enum Color {
    RED = 1,
    GREEN = 2,
    BLUE = 4
}

fun get_blue() -> u8 {
    return Color.BLUE
}
`), 0o644)

	return dir, func() { os.RemoveAll(dir) }
}

func TestImportUnqualified(t *testing.T) {
	dir, cleanup := setupModuleDir(t)
	defer cleanup()

	src := `import mylib.math { add, double }

fun test() -> u8 {
    return add(double(10), 5)
}
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Should have 3 funcs: mylib$math$add, mylib$math$double, test
	if len(m.Funcs) != 3 {
		names := make([]string, len(m.Funcs))
		for i, f := range m.Funcs {
			names[i] = f.Name
		}
		t.Fatalf("funcs: want 3, got %d: %v", len(m.Funcs), names)
	}

	// Compile should succeed
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestImportGlob(t *testing.T) {
	dir, cleanup := setupModuleDir(t)
	defer cleanup()

	src := `import mylib.math { * }

fun test() -> u8 {
    return add(double(3), 1)
}
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestImportAlias(t *testing.T) {
	dir, cleanup := setupModuleDir(t)
	defer cleanup()

	src := `import mylib.math { add as plus, double as dbl }

fun test() -> u8 {
    return plus(dbl(10), 5)
}
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestImportQualified(t *testing.T) {
	dir, cleanup := setupModuleDir(t)
	defer cleanup()

	src := `import mylib.math

fun test() -> u8 {
    return mylib.math.add(10, 5)
}
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestImportQualifiedNested(t *testing.T) {
	dir, cleanup := setupModuleDir(t)
	defer cleanup()

	src := `import mylib.math

fun test() -> u8 {
    return mylib.math.add(mylib.math.double(10), 5)
}
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestImportCircularDetection(t *testing.T) {
	dir, err := os.MkdirTemp("", "nanz_circular_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// a.nanz imports b.nanz, b.nanz imports a.nanz
	os.WriteFile(filepath.Join(dir, "a.nanz"), []byte(`import b
fun fa() -> u8 { return 1 }
`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.nanz"), []byte(`import a
fun fb() -> u8 { return 2 }
`), 0o644)

	src := `import a
fun test() -> u8 { return fa() }
`
	_, err = nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err == nil {
		t.Fatal("expected circular import error")
	}
	t.Logf("got expected error: %v", err)
}

func TestImportNotFound(t *testing.T) {
	_, err := nanz.ParseWithOpts(`import nonexistent.module
fun test() -> u8 { return 0 }
`, "test.nanz", nanz.ParseOpts{
		BaseDir: "/tmp",
	})
	if err == nil {
		t.Fatal("expected module not found error")
	}
	t.Logf("got expected error: %v", err)
}

func TestImportWithAssert(t *testing.T) {
	dir, cleanup := setupModuleDir(t)
	defer cleanup()

	src := `import mylib.math { add, double }

fun compute(x: u8) -> u8 {
    return add(double(x), 1)
}

assert compute(5) == 11
assert compute(10) == 21
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Asserts) != 2 {
		t.Fatalf("asserts: want 2, got %d", len(m.Asserts))
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile with asserts: %v", err)
	}
}

func TestImportLanzModule(t *testing.T) {
	dir, err := os.MkdirTemp("", "nanz_lanz_import")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a Lanz module
	os.WriteFile(filepath.Join(dir, "lanzmath.lanz"), []byte(`
(fun lanz_double ((x u8)) u8 (return (+ x x)))
(fun lanz_inc ((x u8)) u8 (return (+ x 1)))
`), 0o644)

	// Import from Nanz
	src := `import lanzmath { lanz_double, lanz_inc }

fun compute(x: u8) -> u8 {
    return lanz_double(x)
}

fun inc_wrap(x: u8) -> u8 {
    return lanz_inc(x)
}

assert compute(5) == 10
assert inc_wrap(9) == 10
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Verify functions were imported
	found := 0
	for _, f := range m.Funcs {
		if f.Name == "lanzmath$lanz_double" || f.Name == "lanzmath$lanz_inc" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected 2 imported lanz functions, found %d", found)
	}

	// Full pipeline with asserts
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t.Log("Nanz → Lanz cross-language import: OK")
}

func TestImportPLMModule(t *testing.T) {
	dir, err := os.MkdirTemp("", "nanz_plm_import")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a PL/M-80 module
	os.WriteFile(filepath.Join(dir, "legacy.plm"), []byte(`
plm_add: PROCEDURE(a, b) BYTE;
    DECLARE (a, b) BYTE;
    RETURN a + b;
END plm_add;
`), 0o644)

	// Import from Nanz — PL/M names are uppercased
	src := `import legacy { PLM_ADD }

fun use_plm(a: u8, b: u8) -> u8 {
    return PLM_ADD(a, b)
}

assert use_plm(5, 1) == 6
`
	m, err := nanz.ParseWithOpts(src, "test.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Full pipeline
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	t.Log("Nanz → PL/M-80 cross-language import: OK")
}
