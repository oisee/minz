package mir2llvm

import (
	"bytes"
	"os"
	"os/exec"
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

func TestLLI(t *testing.T) {
	if _, err := exec.LookPath("lli"); err != nil {
		t.Skip("lli not found, skipping native LLVM test")
	}

	tests := []struct {
		name   string
		src    string
		expect int
	}{
		{"add", `fun main() -> u8 { return 3 + 4 }`, 7},
		{"mul", `fun main() -> u8 { return 6 * 7 }`, 42},
		{"call", `
fun double(x: u8) -> u8 { return x + x }
fun main() -> u8 { return double(5) }`, 10},
		{"chain", `
fun leaf(x: u8) -> u8 { return x + x }
fun middle(y: u8) -> u8 { return leaf(y) }
fun main() -> u8 { return middle(5) }`, 10},
		{"conditional", `
fun max_byte(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}
fun main() -> u8 { return max_byte(10, 20) }`, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ll := compileNanz(t, tt.src)

			// Wrap main() to return i32 for lli (process exit code)
			wrapper := ll + "\ndefine i32 @_main_wrapper() {\n" +
				"  %r = call i8 @main()\n" +
				"  %ext = zext i8 %r to i32\n" +
				"  ret i32 %ext\n}\n"

			// Write to temp file
			tmpFile := t.TempDir() + "/test.ll"
			if err := os.WriteFile(tmpFile, []byte(wrapper), 0644); err != nil {
				t.Fatal(err)
			}

			// Run via lli
			cmd := exec.Command("lli", "-entry-function=_main_wrapper", tmpFile)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("lli failed: %v\n%s", err, stderr.String())
				}
			}

			if exitCode != tt.expect {
				t.Errorf("lli returned %d, want %d\nIR:\n%s\nstderr: %s",
					exitCode, tt.expect, wrapper, stderr.String())
			} else {
				t.Logf("lli: main() = %d (correct)", exitCode)
			}
		})
	}
}
