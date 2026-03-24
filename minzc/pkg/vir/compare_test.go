package vir_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// TestVIR_Standalone_vs_Constrained compares per-function instruction counts
// when solved with PBQP-derived param constraints vs unconstrained (standalone).
// This measures the potential gain from the "solve free + adapter" strategy.
func TestVIR_Standalone_vs_Constrained(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	exDir := filepath.Join("..", "..", "..", "examples", "nanz")
	files, err := filepath.Glob(filepath.Join(exDir, "*.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)

	opts := vir.SolverOptions{Timeout: 5 * time.Second}

	type funcScore struct {
		file           string
		name           string
		constrained    int // insts with PBQP param constraints
		standalone     int // insts without param constraints
		delta          int // constrained - standalone (positive = standalone wins)
		nParams        int
		adapterCost    int // estimated LD moves for adapter (= nParams, worst case)
	}
	var scores []funcScore
	var totalConst, totalStand int
	var standaloneWins, constrainedWins, ties int

	for _, fpath := range files {
		base := filepath.Base(fpath)
		src, err := os.ReadFile(fpath)
		if err != nil {
			continue
		}

		hm, err := nanz.ParseWithOpts(string(src), base, nanz.ParseOpts{})
		if err != nil {
			continue
		}

		m, err := lowerHIR(hm)
		if err != nil {
			continue
		}

		// Build PBQP-derived param locations (like assert_test.go)
		funcParamLocs := make(map[string]map[int]int)
		for _, f := range m.Funcs {
			lr := mir2.ComputeLiveness(f)
			ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
			if ar == nil {
				continue
			}
			pl := make(map[int]int)
			for _, cp := range f.Contract.Params {
				if loc, ok := ar.Locs[cp.Reg]; ok {
					idx := vir.Z80.LocByName(loc.Name)
					if idx >= 0 {
						pl[int(cp.Reg)] = idx
					}
				}
			}
			funcParamLocs[f.Name] = pl
		}

		// Run constrained (with PBQP param locs)
		optsConst := opts
		optsConst.FuncParamLocs = funcParamLocs
		_, resultsConst := vir.CodegenModule(m, optsConst)

		// Run standalone (no param constraints)
		_, resultsStand := vir.CodegenModule(m, opts)

		// Build lookup maps
		constMap := make(map[string]int)
		for _, r := range resultsConst {
			if r.OK {
				constMap[r.Name] = countInstructions(r.ASM)
			}
		}
		standMap := make(map[string]int)
		for _, r := range resultsStand {
			if r.OK {
				standMap[r.Name] = countInstructions(r.ASM)
			}
		}

		for _, f := range m.Funcs {
			ci, cOK := constMap[f.Name]
			si, sOK := standMap[f.Name]
			if !cOK || !sOK {
				continue
			}

			nParams := len(f.Contract.Params)
			delta := ci - si
			scores = append(scores, funcScore{
				file: base, name: f.Name,
				constrained: ci, standalone: si,
				delta: delta, nParams: nParams,
				adapterCost: nParams, // worst case: 1 LD per param
			})
			totalConst += ci
			totalStand += si
			if delta > 0 {
				standaloneWins++
			} else if delta < 0 {
				constrainedWins++
			} else {
				ties++
			}
		}
	}

	// Sort by delta descending (biggest standalone wins first)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].delta > scores[j].delta
	})

	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════════════════════╗")
	t.Log("║   VIR: Constrained (PBQP ABI) vs Standalone (free regs) + Adapter         ║")
	t.Log("╚══════════════════════════════════════════════════════════════════════════════╝")
	t.Log("")
	t.Logf("%-30s %-20s %6s %6s %6s %5s %7s",
		"File", "Function", "Const", "Stand", "Delta", "Param", "Net")
	t.Logf("%-30s %-20s %6s %6s %6s %5s %7s",
		"──────────────────────────────", "────────────────────",
		"──────", "──────", "──────", "─────", "───────")

	// Show functions where standalone wins (delta > 0)
	shown := 0
	for _, s := range scores {
		if s.delta <= 0 {
			break
		}
		net := s.delta - s.adapterCost // net gain after adapter cost
		netStr := formatDelta(net)
		if net > 0 {
			netStr += " ✓"
		}
		t.Logf("%-30s %-20s %6d %6d %6d %5d %7s",
			s.file, s.name, s.constrained, s.standalone, s.delta, s.nParams, netStr)
		shown++
	}

	if shown == 0 {
		t.Log("  (no functions benefit from standalone mode)")
	}

	t.Log("")
	t.Logf("Total: constrained=%d  standalone=%d  delta=%s",
		totalConst, totalStand, formatDelta(totalConst-totalStand))
	t.Logf("Standalone wins: %d  Constrained wins: %d  Ties: %d  (of %d funcs)",
		standaloneWins, constrainedWins, ties, len(scores))

	// Count net wins (after adapter cost)
	netWins := 0
	totalNetSaved := 0
	for _, s := range scores {
		net := s.delta - s.adapterCost
		if net > 0 {
			netWins++
			totalNetSaved += net
		}
	}
	t.Logf("Net wins (after adapter cost): %d funcs, %d instructions saved", netWins, totalNetSaved)
	t.Log("")
}
