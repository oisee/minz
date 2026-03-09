package mir2qbe_test

// E2E tests: real source files → HIR → MIR2 → QBE → native binary.
//
// This is the correctness oracle: if the native result matches the Z80 result
// (verified separately in pkg/mir2/examples_test.go), then:
//   - HIR→MIR2 lowering is correct
//   - MIR2 IR semantics are correct
//   - Any Z80 divergence is localised to Z80 codegen
//
// We stop the pipeline BEFORE Z80-specific steps (contract opt, register
// allocation) — those are irrelevant for QBE, which does its own RA.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2qbe"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/plm"
)

// ── pipeline helpers ──────────────────────────────────────────────────────────

// hirToMIR2 lowers a HIR module to optimised MIR2 (no Z80 alloc/contracts).
func hirToMIR2(hm *hir.Module) *mir2.Module {
	m := hir.LowerModule(hm)
	mir2.LUTGen(m)
	for _, f := range m.Funcs {
		mir2.EliminateDeadBlocks(f)
		mir2.ReorderBlocks(f)
		for {
			p := mir2.PropagateConstants(f)
			c := mir2.FoldConstants(f)
			e := mir2.ConstantCallElim(m, f)
			if !p && !c && !e {
				break
			}
		}
		mir2.DeadStoreElim(f)
	}
	return m
}

func compilePLM(src string) (*mir2.Module, error) {
	hm, err := plm.Compile(src)
	if err != nil {
		return nil, fmt.Errorf("plm.Compile: %w", err)
	}
	return hirToMIR2(hm), nil
}

func compileNanz(src, name string) (*mir2.Module, error) {
	hm, err := nanz.Parse(src, name)
	if err != nil {
		return nil, fmt.Errorf("nanz.Parse: %w", err)
	}
	return hirToMIR2(hm), nil
}

// runNativeFrom compiles m to QBE, links with a C wrapper calling funcName(args...),
// runs the binary, and returns the printed integer result.
func runNativeFrom(t *testing.T, m *mir2.Module, funcName string, args ...int) (int, error) {
	t.Helper()

	qbeIR, err := mir2qbe.Compile(m)
	if err != nil {
		return 0, fmt.Errorf("mir2qbe.Compile: %w", err)
	}

	dir := t.TempDir()
	ssaPath := filepath.Join(dir, "out.ssa")
	if err := os.WriteFile(ssaPath, []byte(qbeIR), 0644); err != nil {
		return 0, err
	}

	asmPath := filepath.Join(dir, "out.s")
	if out, err := exec.Command("qbe", "-o", asmPath, ssaPath).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("qbe:\n%s\nIR was:\n%s", out, qbeIR)
	}

	argList := make([]string, len(args))
	params := make([]string, len(args))
	for i, a := range args {
		argList[i] = strconv.Itoa(a)
		params[i] = "int"
	}
	cSrc := fmt.Sprintf(`#include <stdio.h>
extern int %s(%s);
int main(void) { printf("%%d\n", %s(%s)); return 0; }
`, funcName, strings.Join(params, ", "), funcName, strings.Join(argList, ", "))

	cPath := filepath.Join(dir, "main.c")
	if err := os.WriteFile(cPath, []byte(cSrc), 0644); err != nil {
		return 0, err
	}

	binPath := filepath.Join(dir, "bin")
	if out, err := exec.Command("cc", asmPath, cPath, "-o", binPath).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("cc: %w\n%s", err, out)
	}

	runOut, err := exec.Command(binPath).Output()
	if err != nil {
		return 0, fmt.Errorf("run: %w", err)
	}
	return strconv.Atoi(strings.TrimSpace(string(runOut)))
}

// dumpQBE prints the QBE IL for a module (for -v inspection).
func dumpQBE(t *testing.T, m *mir2.Module) {
	t.Helper()
	ir, err := mir2qbe.Compile(m)
	if err != nil {
		t.Logf("mir2qbe error: %v", err)
		return
	}
	t.Logf("=== QBE IL ===\n%s", ir)
}

// dumpDisasm runs qbe + cc + otool -tv and logs the disassembly.
func dumpDisasm(t *testing.T, m *mir2.Module) {
	t.Helper()
	qbeIR, err := mir2qbe.Compile(m)
	if err != nil {
		return
	}
	dir := t.TempDir()
	ssaPath := filepath.Join(dir, "out.ssa")
	_ = os.WriteFile(ssaPath, []byte(qbeIR), 0644)
	asmPath := filepath.Join(dir, "out.s")
	if out, err := exec.Command("qbe", "-o", asmPath, ssaPath).CombinedOutput(); err != nil {
		t.Logf("qbe disasm skip: %s", out)
		return
	}
	objPath := filepath.Join(dir, "out.o")
	if out, err := exec.Command("cc", "-c", asmPath, "-o", objPath).CombinedOutput(); err != nil {
		t.Logf("cc -c skip: %s", out)
		return
	}
	// macOS: otool -tv; Linux: objdump -d
	out, _ := exec.Command("objdump", "--disassemble", "--no-show-raw-insn", objPath).Output()
	if len(out) == 0 {
		out, _ = exec.Command("otool", "-tv", objPath).Output()
	}
	t.Logf("=== DISASM ===\n%s", out)
}

// ── PL/M demo: abs_diff + fib ─────────────────────────────────────────────────

const plmDemoSrc = `DEMO: DO;

ABS_DIFF: PROCEDURE (A, B) BYTE;
  DECLARE (A, B) BYTE;
  IF A > B THEN
    RETURN A - B;
  ELSE
    RETURN B - A;
END ABS_DIFF;

FIB: PROCEDURE (N) BYTE;
  DECLARE N BYTE;
  DECLARE (A, B, T) BYTE;
  A = 0;
  B = 1;
  DO WHILE N > 0;
    T = B;
    B = A + B;
    A = T;
    N = N - 1;
  END;
  RETURN A;
END FIB;

END DEMO;`

func TestE2E_PLM_AbsDiff(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m, err := compilePLM(plmDemoSrc)
	if err != nil {
		t.Fatal(err)
	}
	dumpQBE(t, m)
	dumpDisasm(t, m)

	cases := []struct{ a, b, want int }{
		{10, 3, 7}, {3, 10, 7}, {5, 5, 0}, {0, 255, 255}, {100, 200, 100},
	}
	for _, tc := range cases {
		got, err := runNativeFrom(t, m, "ABS_DIFF", tc.a, tc.b)
		if err != nil {
			t.Fatalf("ABS_DIFF(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("ABS_DIFF(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("ABS_DIFF(%d,%d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

func TestE2E_PLM_Fib(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m, err := compilePLM(plmDemoSrc)
	if err != nil {
		t.Fatal(err)
	}

	// fib: 0,1,1,2,3,5,8,13,21,34,55,89,144,233
	cases := []struct{ n, want int }{
		{0, 0}, {1, 1}, {2, 1}, {3, 2}, {5, 5},
		{8, 21}, {10, 55}, {13, 233},
	}
	for _, tc := range cases {
		got, err := runNativeFrom(t, m, "FIB", tc.n)
		if err != nil {
			t.Fatalf("FIB(%d): %v", tc.n, err)
		}
		if got != tc.want {
			t.Errorf("FIB(%d) = %d, want %d", tc.n, got, tc.want)
		} else {
			t.Logf("FIB(%d) = %d ✓", tc.n, got)
		}
	}
}

// ── Nanz: sum_array (while loop with ptr[i]) ──────────────────────────────────

const nanzSumArraySrc = `
fun sum_array(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    var i: u8 = 0
    while i < n {
        s = s + ptr[i]
        i = i + 1
    }
    return s
}
`

func TestE2E_Nanz_SumArray(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m, err := compileNanz(nanzSumArraySrc, "sum_array")
	if err != nil {
		t.Fatal(err)
	}
	dumpQBE(t, m)
	dumpDisasm(t, m)

	// We need to pass a real pointer — use a C helper that passes a local array.
	qbeIR, _ := mir2qbe.Compile(m)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "out.ssa"), []byte(qbeIR), 0644)
	_ = exec.Command("qbe", "-o", filepath.Join(dir, "out.s"), filepath.Join(dir, "out.ssa")).Run()

	// ptr param: u16 used as pointer → promoted to "l" (64-bit) in QBE.
	// C extern must match: long for ptr, int for n.
	cSrc := `#include <stdio.h>
extern int sum_array(long ptr, int n);
int main(void) {
    unsigned char a[] = {1, 2, 3, 4, 5};
    unsigned char b[] = {10, 20, 30};
    printf("%d\n", sum_array((long)a, 5));  // 15
    printf("%d\n", sum_array((long)b, 3));  // 60
    return 0;
}`
	cPath := filepath.Join(dir, "main.c")
	_ = os.WriteFile(cPath, []byte(cSrc), 0644)
	binPath := filepath.Join(dir, "bin")
	if out, err := exec.Command("cc", filepath.Join(dir, "out.s"), cPath, "-o", binPath).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}
	out, err := exec.Command(binPath).Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	t.Logf("sum_array output:\n%s", string(out))
	want := []int{15, 60}
	for i, w := range want {
		if i >= len(lines) {
			t.Errorf("missing output line %d", i)
			continue
		}
		got, _ := strconv.Atoi(strings.TrimSpace(lines[i]))
		if got != w {
			t.Errorf("case %d: got %d, want %d", i, got, w)
		} else {
			t.Logf("case %d: %d ✓", i, got)
		}
	}
}

// ── Nanz: abs_diff + clamp (idiomatic) ───────────────────────────────────────

const nanzAbsDiffSrc = `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b {
        return a - b
    } else {
        return b - a
    }
}

fun clamp(x: u8, lo: u8, hi: u8) -> u8 {
    if x < lo {
        return lo
    }
    if x > hi {
        return hi
    }
    return x
}
`

func TestE2E_Nanz_AbsDiff(t *testing.T) {
	if _, err := exec.LookPath("qbe"); err != nil {
		t.Skip("qbe not in PATH")
	}
	m, err := compileNanz(nanzAbsDiffSrc, "abs_diff")
	if err != nil {
		t.Fatal(err)
	}
	dumpQBE(t, m)
	dumpDisasm(t, m)

	cases := []struct{ a, b, want int }{
		{10, 3, 7}, {3, 10, 7}, {5, 5, 0}, {200, 100, 100},
	}
	for _, tc := range cases {
		got, err := runNativeFrom(t, m, "abs_diff", tc.a, tc.b)
		if err != nil {
			t.Fatalf("abs_diff(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("abs_diff(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("abs_diff(%d,%d) = %d ✓", tc.a, tc.b, got)
		}
	}
}
