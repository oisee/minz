package mir2wasm

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
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
