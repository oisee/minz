package mir2wasm

import (
	"context"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/tetratelabs/wazero"
)

func compileNanz(t *testing.T, src string) string {
	t.Helper()
	hirMod, err := nanz.Parse(src, "test.nanz")
	if err != nil {
		t.Fatalf("nanz compile: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	if mir2Mod == nil {
		t.Fatal("mir2 lower failed")
	}
	wat, err := Compile(mir2Mod)
	if err != nil {
		t.Fatalf("wasm compile: %v", err)
	}
	return wat
}

func TestAdd(t *testing.T) {
	wat := compileNanz(t, `
fun add(a: u8, b: u8) -> u8 { return a + b }
fun main() -> u8 { return add(3, 4) }
`)
	t.Log(wat)
	if !strings.Contains(wat, "i32.add") {
		t.Error("expected i32.add in output")
	}
	if !strings.Contains(wat, "(module") {
		t.Error("expected (module wrapper")
	}
	if !strings.Contains(wat, "func $add") {
		t.Error("expected func $add")
	}
}

func TestDouble(t *testing.T) {
	wat := compileNanz(t, `
fun double(x: u8) -> u8 { return x + x }
`)
	t.Log(wat)
	if !strings.Contains(wat, "i32.add") {
		t.Error("expected i32.add")
	}
}

func TestSwap(t *testing.T) {
	wat := compileNanz(t, `
fun swap(a: u8, b: u8) -> (u8, u8) { return (b, a) }
`)
	t.Log(wat)
	if !strings.Contains(wat, "func $swap") {
		t.Error("expected func $swap")
	}
}

func TestBinaryRun(t *testing.T) {
	// Compile to WASM binary and actually RUN it via wazero
	hirMod, err := nanz.Parse(`
fun add(a: u8, b: u8) -> u8 { return a + b }
fun double(x: u8) -> u8 { return x + x }
`, "test.nanz")
	if err != nil {
		t.Fatal(err)
	}
	mir2Mod := hir.LowerModule(hirMod)

	wasmBin, err := CompileBinary(mir2Mod)
	if err != nil {
		t.Fatalf("compile binary: %v", err)
	}
	t.Logf("WASM binary: %d bytes", len(wasmBin))

	// Run via wazero
	ctx := context.Background()
	rt := wazero.NewRuntime(ctx)
	defer rt.Close(ctx)

	mod, err := rt.Instantiate(ctx, wasmBin)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	defer mod.Close(ctx)

	// Test add(3, 4) = 7
	addFn := mod.ExportedFunction("add")
	if addFn == nil {
		t.Fatal("add function not exported")
	}
	results, err := addFn.Call(ctx, 3, 4)
	if err != nil {
		t.Fatalf("add call: %v", err)
	}
	if results[0] != 7 {
		t.Errorf("add(3, 4) = %d, want 7", results[0])
	}

	// Test double(21) = 42
	dblFn := mod.ExportedFunction("double")
	if dblFn == nil {
		t.Fatal("double function not exported")
	}
	results, err = dblFn.Call(ctx, 21)
	if err != nil {
		t.Fatalf("double call: %v", err)
	}
	if results[0] != 42 {
		t.Errorf("double(21) = %d, want 42", results[0])
	}

	t.Logf("add(3,4) = %d, double(21) = %d — WASM execution verified!", results[0], 42)
}

func TestConditional(t *testing.T) {
	wat := compileNanz(t, `
fun max_byte(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}
`)
	t.Log(wat)
	if !strings.Contains(wat, "i32.gt_u") {
		t.Error("expected i32.gt_u for unsigned comparison")
	}
}
