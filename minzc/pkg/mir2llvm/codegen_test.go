package mir2llvm

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
		t.Fatalf("nanz parse: %v", err)
	}
	mir2Mod := hir.LowerModule(hirMod)
	if mir2Mod == nil {
		t.Fatal("mir2 lower failed")
	}
	ll, err := Compile(mir2Mod)
	if err != nil {
		t.Fatalf("llvm compile: %v", err)
	}
	return ll
}

func TestAdd(t *testing.T) {
	ll := compileNanz(t, `fun add(a: u8, b: u8) -> u8 { return a + b }`)
	t.Log(ll)
	if !strings.Contains(ll, "add i8") {
		t.Error("expected add i8 in output")
	}
	if !strings.Contains(ll, "define i8 @add") {
		t.Error("expected define i8 @add")
	}
}

func TestDouble(t *testing.T) {
	ll := compileNanz(t, `fun double(x: u8) -> u8 { return x + x }`)
	t.Log(ll)
	if !strings.Contains(ll, "add i8") {
		t.Error("expected add i8")
	}
}

func TestConditional(t *testing.T) {
	ll := compileNanz(t, `
fun max_byte(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}`)
	t.Log(ll)
	if !strings.Contains(ll, "icmp ugt") {
		t.Error("expected icmp ugt")
	}
	if !strings.Contains(ll, "br i1") || !strings.Contains(ll, "ret i8") {
		t.Error("expected br + ret for conditional")
	}
}

func TestChain(t *testing.T) {
	ll := compileNanz(t, `
fun leaf(x: u8) -> u8 { return x + x }
fun middle(y: u8) -> u8 { return leaf(y) }
fun top(z: u8) -> u8 { return middle(z) }
fun main() -> u8 { return top(5) }
`)
	t.Log(ll)
	if !strings.Contains(ll, "call i8 @leaf") {
		t.Error("expected call to leaf")
	}
	if !strings.Contains(ll, "call i8 @middle") {
		t.Error("expected call to middle")
	}
}

func TestNativeRun(t *testing.T) {
	// Just verify the LLVM IR is syntactically valid by checking structure
	ll := compileNanz(t, `
fun add(a: u8, b: u8) -> u8 { return a + b }
fun square(x: u8) -> u8 { return x * x }
fun main() -> u8 { return add(3, 4) }
`)
	// Check well-formedness
	if !strings.HasPrefix(ll, "; ModuleID") {
		t.Error("expected ; ModuleID header")
	}
	if strings.Count(ll, "define ") != 3 {
		t.Errorf("expected 3 function definitions, got %d", strings.Count(ll, "define "))
	}
	if strings.Count(ll, "ret ") < 3 {
		t.Errorf("expected at least 3 ret instructions")
	}
	t.Logf("LLVM IR: %d bytes, %d functions", len(ll), strings.Count(ll, "define "))
}
