package mir2c_test

// Crazy round-trip test: run the same functions through every possible path
// and assert all results agree.
//
// Path 1 — MIR2 VM:
//   Nanz → HIR → MIR2 → mir2.VM.Call()
//
// Path 2 — C oracle (mir2c):
//   Nanz → HIR → MIR2 → mir2c.Compile → cc → native binary
//
// Path 3 — QBE oracle (mir2qbe → qbe tool → cc):
//   Nanz → HIR → MIR2 → mir2qbe.Compile → qbe → cc → native binary
//
// Path 4 — QBE round-trip (mir2qbe → qbe2mir2 → mir2c → cc):
//   Nanz → HIR → MIR2 → mir2qbe.Compile → qbe2mir2.Parse → mir2c.Compile → cc → native
//
// Path 5 — Z80 assembly (show only):
//   Nanz → HIR → pipeline.CompileHIR → Z80 asm text (logged, not executed)
//
// Path 3 requires `qbe` in PATH (skipped if absent).
// Paths 2 and 4 require `cc` in PATH (present everywhere).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2qbe"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/qbe2mir2"
)

// ── path implementations ──────────────────────────────────────────────────────

// path1_vm: run via the MIR2 interpreter VM.
func path1_vm(m *mir2.Module, funcName string, args ...int64) (int64, error) {
	vm := mir2.NewVM(m)
	vals := make([]mir2.Value, len(args))
	for i, a := range args {
		vals[i] = mir2.Value{I: a}
	}
	res, err := vm.Call(funcName, vals)
	if err != nil {
		return 0, fmt.Errorf("VM: %w", err)
	}
	if len(res) == 0 {
		return 0, nil
	}
	return res[0].I, nil
}

// path2_c: Nanz HIR-MIR2 → mir2c → cc → native.
func path2_c(t *testing.T, m *mir2.Module, funcName string, args ...int) (int, error) {
	t.Helper()
	return runC(t, m, funcName, args...)
}

// path3_qbe: MIR2 → QBE IL → qbe tool → cc → native.
func path3_qbe(t *testing.T, m *mir2.Module, funcName string, args ...int) (int, error) {
	t.Helper()
	qbeIR, err := mir2qbe.Compile(m)
	if err != nil {
		return 0, fmt.Errorf("mir2qbe: %w", err)
	}

	dir := t.TempDir()
	ssaPath := filepath.Join(dir, "out.ssa")
	if err := os.WriteFile(ssaPath, []byte(qbeIR), 0644); err != nil {
		return 0, err
	}
	asmPath := filepath.Join(dir, "out.s")
	if out, err := exec.Command("qbe", "-o", asmPath, ssaPath).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("qbe:\n%s", out)
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
	out, err := exec.Command(binPath).Output()
	if err != nil {
		return 0, fmt.Errorf("run: %w", err)
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// path4_qbeRoundTrip: MIR2 → QBE IL → qbe2mir2 → mir2c → cc → native.
// This is the "crazy double-translation" path.
func path4_qbeRoundTrip(t *testing.T, m *mir2.Module, funcName string, args ...int) (int, error) {
	t.Helper()

	// Step A: MIR2 → QBE IL
	qbeIR, err := mir2qbe.Compile(m)
	if err != nil {
		return 0, fmt.Errorf("mir2qbe: %w", err)
	}
	t.Logf("[path4] QBE IL:\n%s", qbeIR)

	// Step B: QBE IL → MIR2 (parse back)
	m2, err := qbe2mir2.Parse(qbeIR)
	if err != nil {
		return 0, fmt.Errorf("qbe2mir2.Parse: %w", err)
	}

	// Step C: MIR2 → C99 → cc → native
	return runC(t, m2, funcName, args...)
}

// path5_z80asm: Nanz → HIR → full pipeline → Z80 assembly text (no execute, just show).
func path5_z80asm(t *testing.T, nanzSrc, modName string) string {
	t.Helper()
	hm, err := nanz.Parse(nanzSrc, modName)
	if err != nil {
		t.Logf("[path5] nanz.Parse: %v", err)
		return ""
	}
	asm, err := pipeline.CompileHIR(hm)
	if err != nil {
		t.Logf("[path5] CompileHIR: %v", err)
		return ""
	}
	return asm
}

// ── round-trip runner ─────────────────────────────────────────────────────────

type rtCase struct {
	args []int
	want int
}

// runRoundTrip runs all available paths for a single function and compares results.
func runRoundTrip(t *testing.T, m *mir2.Module, funcName string, cases []rtCase) {
	t.Helper()

	hasQBE := func() bool {
		_, err := exec.LookPath("qbe")
		return err == nil
	}()
	hasCC := func() bool {
		_, err := exec.LookPath("cc")
		return err == nil
	}()

	for _, tc := range cases {
		name := fmt.Sprintf("%s(%v)", funcName, tc.args)
		t.Run(name, func(t *testing.T) {
			results := map[string]int64{}
			errs := map[string]string{}

			// Path 1: MIR2 VM
			args64 := make([]int64, len(tc.args))
			for i, a := range tc.args {
				args64[i] = int64(a)
			}
			if v, err := path1_vm(m, funcName, args64...); err != nil {
				errs["1:VM"] = err.Error()
			} else {
				results["1:VM"] = v
			}

			// Path 2: mir2c → cc
			if hasCC {
				if v, err := path2_c(t, m, funcName, tc.args...); err != nil {
					errs["2:mir2c→cc"] = err.Error()
				} else {
					results["2:mir2c→cc"] = int64(v)
				}
			}

			// Path 3: mir2qbe → qbe → cc
			if hasQBE && hasCC {
				if v, err := path3_qbe(t, m, funcName, tc.args...); err != nil {
					errs["3:QBE→native"] = err.Error()
				} else {
					results["3:QBE→native"] = int64(v)
				}
			}

			// Path 4: mir2qbe → qbe2mir2 → mir2c → cc
			if hasCC {
				if v, err := path4_qbeRoundTrip(t, m, funcName, tc.args...); err != nil {
					errs["4:QBE↔MIR2→C"] = err.Error()
				} else {
					results["4:QBE↔MIR2→C"] = int64(v)
				}
			}

			// Report any errors
			for path, errMsg := range errs {
				t.Errorf("path %s error: %s", path, errMsg)
			}

			// All results must match
			want := int64(tc.want)
			allMatch := true
			for path, got := range results {
				if got != want {
					t.Errorf("path %-18s: got %d, want %d", path, got, want)
					allMatch = false
				}
			}
			if allMatch && len(results) > 0 {
				// Print a nice summary line
				pathNames := make([]string, 0, len(results))
				for p := range results {
					pathNames = append(pathNames, p)
				}
				t.Logf("✓ all %d paths agree: %s(%v) = %d",
					len(results), funcName, tc.args, want)
			}
		})
	}
}

// ── test cases ────────────────────────────────────────────────────────────────

const rtAbsDiffSrc = `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b {
        return a - b
    } else {
        return b - a
    }
}
`

func TestRoundTrip_AbsDiff(t *testing.T) {
	m, err := compileNanz(rtAbsDiffSrc, "abs_diff")
	if err != nil {
		t.Fatal(err)
	}

	// Show Z80 assembly for fun
	if asm := path5_z80asm(t, rtAbsDiffSrc, "abs_diff"); asm != "" {
		t.Logf("=== Z80 assembly (path 5) ===\n%s", asm)
	}

	runRoundTrip(t, m, "abs_diff", []rtCase{
		{[]int{10, 3}, 7},
		{[]int{3, 10}, 7},
		{[]int{5, 5}, 0},
		{[]int{200, 100}, 100},
		{[]int{0, 255}, 255},
	})
}

const rtFibSrc = `
fun fib(n: u8) -> u8 {
    var a: u8 = 0
    var b: u8 = 1
    var i: u8 = 0
    while i < n {
        var t: u8 = b
        b = a + b
        a = t
        i = i + 1
    }
    return a
}
`

func TestRoundTrip_Fib(t *testing.T) {
	m, err := compileNanz(rtFibSrc, "fib")
	if err != nil {
		t.Fatal(err)
	}

	if asm := path5_z80asm(t, rtFibSrc, "fib"); asm != "" {
		t.Logf("=== Z80 assembly (path 5) ===\n%s", asm)
	}

	runRoundTrip(t, m, "fib", []rtCase{
		{[]int{0}, 0},
		{[]int{1}, 1},
		{[]int{5}, 5},
		{[]int{8}, 21},
		{[]int{10}, 55},
	})
}

const rtClampSrc = `
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

func TestRoundTrip_Clamp(t *testing.T) {
	m, err := compileNanz(rtClampSrc, "clamp")
	if err != nil {
		t.Fatal(err)
	}

	if asm := path5_z80asm(t, rtClampSrc, "clamp"); asm != "" {
		t.Logf("=== Z80 assembly (path 5) ===\n%s", asm)
	}

	runRoundTrip(t, m, "clamp", []rtCase{
		{[]int{5, 0, 10}, 5},
		{[]int{0, 5, 10}, 5},  // below lo → clamp to lo
		{[]int{15, 5, 10}, 10}, // above hi → clamp to hi
		{[]int{5, 5, 5}, 5},
		{[]int{128, 10, 200}, 128},
	})
}

const rtMaxSrc = `
fun max3(a: u8, b: u8, c: u8) -> u8 {
    var m: u8 = a
    if b > m {
        m = b
    }
    if c > m {
        m = c
    }
    return m
}
`

func TestRoundTrip_Max3(t *testing.T) {
	m, err := compileNanz(rtMaxSrc, "max3")
	if err != nil {
		t.Fatal(err)
	}

	if asm := path5_z80asm(t, rtMaxSrc, "max3"); asm != "" {
		t.Logf("=== Z80 assembly (path 5) ===\n%s", asm)
	}

	runRoundTrip(t, m, "max3", []rtCase{
		{[]int{1, 2, 3}, 3},
		{[]int{3, 2, 1}, 3},
		{[]int{2, 3, 1}, 3},
		{[]int{7, 7, 7}, 7},
		{[]int{100, 200, 150}, 200},
	})
}
