package vir_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
)

// TestVIR_vs_SDCC compares the unified VIR solver output against SDCC reference
// instruction counts for the 5 paper benchmark programs.
func TestVIR_vs_SDCC(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	// SDCC 4.2.0 instruction counts (from research/abi-paper/sdcc-output/)
	sdccCounts := map[string]int{
		"abs_diff": 12,
		"gcd":      17,
		"minmax":   60, // min + max combined
		"fib":      22,
		"select_b": 20, // dead-code elimination: `let t = a; return b` — SDCC can't optimize
	}

	nanzSources := map[string]string{
		"abs_diff": `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b { return a - b }
    return b - a
}`,
		"gcd": `
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}`,
		"minmax": `
fun min(a: u8, b: u8) -> u8 {
    if a < b { return a }
    return b
}
fun max(a: u8, b: u8) -> u8 {
    if a > b { return a }
    return b
}`,
		"fib": `
fun fib(n: u8) -> u16 {
    if n < 2 { return n }
    let a: u16 = 0
    let b: u16 = 1
    for i in 2..n+1 {
        let t: u16 = a + b
        a = b
        b = t
    }
    return b
}`,
		"select_b": `
fun select_b(a: u8, b: u8) -> u8 {
    let t: u8 = a
    return b
}`,
	}

	opts := vir.SolverOptions{Timeout: 30 * time.Second}

	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════════════════╗")
	t.Log("║      MinZ VIR Solver vs SDCC — Instruction Count Comparison            ║")
	t.Log("║      Z3 joint isel+regalloc (provably optimal per basic block)         ║")
	t.Log("╚══════════════════════════════════════════════════════════════════════════╝")
	t.Log("")
	t.Logf("%-15s %6s %6s %6s %8s %s", "Program", "SDCC", "PBQP", "VIR", "vs SDCC", "Winner")
	t.Logf("%-15s %6s %6s %6s %8s %s", "───────────────", "──────", "──────", "──────", "────────", "──────")

	totalSDCC, totalPBQP, totalVIR := 0, 0, 0
	virWins, sdccWins, ties := 0, 0, 0

	for _, name := range []string{"abs_diff", "gcd", "minmax", "fib", "select_b"} {
		src := nanzSources[name]
		sdcc := sdccCounts[name]

		// Compile through PBQP (production)
		pbqpInsts, pbqpAsm := compilePBQP(t, src, name)

		// Compile through VIR solver
		virInsts, virAsm := compileVIR(t, src, name, opts)

		best := virInsts
		vsSDCC := best - sdcc
		winner := "tie"
		if best < sdcc {
			winner = "VIR ✓"
			virWins++
		} else if best > sdcc {
			winner = "SDCC"
			sdccWins++
		} else {
			ties++
		}

		sign := "+"
		if vsSDCC < 0 {
			sign = ""
		}
		t.Logf("%-15s %6d %6d %6d %7s%d %s",
			name, sdcc, pbqpInsts, virInsts, sign, vsSDCC, winner)

		totalSDCC += sdcc
		totalPBQP += pbqpInsts
		totalVIR += virInsts

		_ = pbqpAsm
		_ = virAsm
	}

	t.Log("")
	totalVsSDCC := totalVIR - totalSDCC
	sign := "+"
	if totalVsSDCC < 0 {
		sign = ""
	}
	t.Logf("%-15s %6d %6d %6d %7s%d", "TOTAL", totalSDCC, totalPBQP, totalVIR, sign, totalVsSDCC)
	t.Log("")
	t.Logf("VIR wins: %d  SDCC wins: %d  Ties: %d", virWins, sdccWins, ties)
	t.Log("")

	// Show VIR assembly for each function
	for _, name := range []string{"abs_diff", "gcd", "minmax", "fib", "select_b"} {
		src := nanzSources[name]
		_, virAsm := compileVIR(t, src, name, opts)
		t.Logf("── %s (VIR) ──", name)
		for _, line := range strings.Split(virAsm, "\n") {
			if strings.TrimSpace(line) != "" {
				t.Logf("  %s", line)
			}
		}
		t.Log("")
	}
}

// TestVIR_C89_SDCC_Benchmark compiles sdcc_benchmark.c through VIR and
// reports per-function instruction counts.
func TestVIR_C89_SDCC_Benchmark(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	benchFile := filepath.Join("..", "..", "..", "examples", "c89", "sdcc_benchmark.c")
	src, err := os.ReadFile(benchFile)
	if err != nil {
		t.Skip("sdcc_benchmark.c not found")
	}

	hm, err := c89.CompileWithOpts(string(src), "sdcc_benchmark.c", c89.CompileOpts{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	m, err := lowerHIR(hm)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}

	opts := vir.SolverOptions{Timeout: 30 * time.Second}
	asm, results := vir.CodegenModule(m, opts)

	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════╗")
	t.Log("║      sdcc_benchmark.c — VIR Solver Results                 ║")
	t.Log("╚══════════════════════════════════════════════════════════════╝")
	t.Log("")

	ok, fail := 0, 0
	totalInsts := 0
	for _, r := range results {
		insts := countInstructions(r.ASM)
		totalInsts += insts
		if r.OK {
			ok++
			t.Logf("  %-25s %3d insts  OK", r.Name, insts)
		} else {
			fail++
			t.Logf("  %-25s FAIL: %s", r.Name, r.Error)
		}
	}
	t.Log("")
	t.Logf("  %d/%d functions OK, %d total instructions", ok, ok+fail, totalInsts)
	t.Logf("  Total ASM: %d bytes", len(asm))
}

func compilePBQP(t *testing.T, src, name string) (int, string) {
	t.Helper()
	hm, err := nanz.ParseWithOpts(src, name+".nanz", nanz.ParseOpts{})
	if err != nil {
		t.Logf("PBQP parse %s: %v", name, err)
		return 0, ""
	}
	m := hir.LowerModule(hm)
	var alloc *mir2.AllocResult
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
		lr := mir2.ComputeLiveness(f)
		ct := mir2.Z80CostTable{}
		alloc = mir2.Allocate(f, lr, ct)
	}
	asm := mir2.Z80Codegen(m, alloc, mir2.Z80CodegenOptions{})
	return countInstructions(asm), asm
}

func compileVIR(t *testing.T, src, name string, opts vir.SolverOptions) (int, string) {
	t.Helper()
	hm, err := nanz.ParseWithOpts(src, name+".nanz", nanz.ParseOpts{})
	if err != nil {
		t.Logf("VIR parse %s: %v", name, err)
		return 0, ""
	}
	m, err := lowerHIR(hm)
	if err != nil {
		t.Logf("VIR lower %s: %v", name, err)
		return 0, ""
	}
	asm, results := vir.CodegenModule(m, opts)
	for _, r := range results {
		if !r.OK {
			t.Logf("VIR %s/%s: %s", name, r.Name, r.Error)
		}
	}
	return countInstructions(asm), asm
}

func countInstructions(asm string) int {
	count := 0
	for _, line := range strings.Split(asm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasSuffix(line, ":") {
			continue
		}
		// Skip directives
		if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "EQU") ||
			strings.HasPrefix(line, "ORG") || strings.HasPrefix(line, "SECTION") {
			continue
		}
		count++
	}
	return count
}

func formatDelta(d int) string {
	if d > 0 {
		return fmt.Sprintf("+%d", d)
	}
	return fmt.Sprintf("%d", d)
}
