package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// TestGrace_SDCC_Comparison compares MinZ (Go path vs Grace path) against SDCC
// on equivalent C/Nanz programs. Uses SDCC instruction counts from
// research/abi-paper/sdcc-output/ as reference.
func TestGrace_SDCC_Comparison(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	// SDCC instruction counts (from pre-compiled .asm files)
	sdccCounts := map[string]int{
		"abs_diff": 12,
		"gcd":      17,
		"minmax":   60,
		"fib":      22,
		"swap":     20,
	}

	// Equivalent Nanz source for each program
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
		"swap": `
fun swap(a: u8, b: u8) -> u8 {
    let t: u8 = a
    return b
}`,
	}

	stats := mir2.NewGraceStats()

	t.Log("")
	t.Log("╔════════════════════════════════════════════════════════════════════╗")
	t.Log("║         MinZ vs SDCC — Instruction Count Comparison              ║")
	t.Log("╚════════════════════════════════════════════════════════════════════╝")
	t.Log("")
	t.Logf("%-15s %6s %6s %6s %6s %6s", "Program", "SDCC", "Go", "Grace", "vsSDCC", "Winner")
	t.Logf("%-15s %6s %6s %6s %6s %6s", strings.Repeat("─", 15), "──────", "──────", "──────", "──────", "──────")

	totalSDCC, totalGo, totalGrace := 0, 0, 0
	minzWins, sdccWins, ties := 0, 0, 0

	for _, name := range []string{"abs_diff", "gcd", "minmax", "fib", "swap"} {
		src := nanzSources[name]
		sdcc := sdccCounts[name]

		goInsts := compileAndCount(t, src, name, false, nil)
		graceInsts := compileAndCount(t, src, name, true, stats)

		best := graceInsts
		if goInsts < best {
			best = goInsts
		}

		vsSDCC := formatDelta(best - sdcc)
		winner := "tie"
		if best < sdcc {
			winner = "MinZ ✓"
			minzWins++
		} else if best > sdcc {
			winner = "SDCC"
			sdccWins++
		} else {
			ties++
		}

		t.Logf("%-15s %6d %6d %6d %6s %s", name, sdcc, goInsts, graceInsts, vsSDCC, winner)

		totalSDCC += sdcc
		totalGo += goInsts
		totalGrace += graceInsts
	}

	t.Log("")
	t.Logf("%-15s %6d %6d %6d %6s", "TOTAL", totalSDCC, totalGo, totalGrace, formatDelta(min(totalGo, totalGrace)-totalSDCC))
	t.Log("")
	t.Logf("MinZ wins: %d  SDCC wins: %d  Ties: %d", minzWins, sdccWins, ties)
	t.Log("")
	t.Logf("Grace stats: %s", stats)
}

// TestGrace_FatFS compiles FatFS C89 test files through Grace and reports.
func TestGrace_FatFS(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	c89Dir := filepath.Join("..", "..", "..", "examples", "c89")

	fatfsFiles := []string{
		"fatfs_constructs.c",
		"fatfs_lowlevel.c",
	}

	stats := mir2.NewGraceStats()

	t.Log("")
	t.Log("╔════════════════════════════════════════════════════════════════════╗")
	t.Log("║              FatFS C89 → Grace Compilation Report                ║")
	t.Log("╚════════════════════════════════════════════════════════════════════╝")
	t.Log("")
	t.Logf("%-30s %6s %6s %6s %6s %s", "File", "Go", "Grace", "Delta", "Funcs", "Status")
	t.Logf("%-30s %6s %6s %6s %6s %s", strings.Repeat("─", 30), "──────", "──────", "──────", "──────", "──────")

	totalGo, totalGrace := 0, 0

	for _, name := range fatfsFiles {
		path := filepath.Join(c89Dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Logf("%-30s SKIP: %v", name, err)
			continue
		}

		// Go path
		hm1, err := c89.Compile(string(src), name)
		if err != nil {
			t.Logf("%-30s PARSE FAIL: %v", name, err)
			continue
		}
		goSteps, err := CompileHIRSteps(hm1, Options{ContractOpt: true})
		if err != nil {
			t.Logf("%-30s GO FAIL: %v", name, err)
			continue
		}
		goInsts := countInstructions(goSteps.Assembly)
		goFuncs := countFunctions(goSteps.Assembly)

		// Grace path
		hm2, err := c89.Compile(string(src), name)
		if err != nil {
			t.Logf("%-30s GRACE PARSE FAIL", name)
			continue
		}
		graceSteps, err := CompileHIRSteps(hm2, Options{
			ContractOpt: true,
			UseGrace:    true,
			GraceStats:  stats,
		})
		if err != nil {
			t.Logf("%-30s GRACE FAIL: %v", name, err)
			continue
		}
		graceInsts := countInstructions(graceSteps.Assembly)

		delta := graceInsts - goInsts
		status := "✓ identical"
		if goSteps.Assembly != graceSteps.Assembly {
			status = "≠ diverged"
		}

		t.Logf("%-30s %6d %6d %6s %6d %s", name, goInsts, graceInsts, formatDelta(delta), goFuncs, status)
		totalGo += goInsts
		totalGrace += graceInsts
	}

	t.Log("")
	t.Logf("%-30s %6d %6d %6s", "TOTAL", totalGo, totalGrace, formatDelta(totalGrace-totalGo))
	t.Log("")
	t.Logf("Grace stats: %s", stats)
}

func compileAndCount(t *testing.T, src, name string, useGrace bool, stats *mir2.GraceStats) int {
	t.Helper()
	hm, err := nanz.ParseWithOpts(src, name+".nanz", nanz.ParseOpts{})
	if err != nil {
		t.Fatalf("%s parse: %v", name, err)
	}
	opts := Options{ContractOpt: true}
	if useGrace {
		opts.UseGrace = true
		opts.GraceStats = stats
	}
	steps, err := CompileHIRSteps(hm, opts)
	if err != nil {
		t.Fatalf("%s compile: %v", name, err)
	}
	return countInstructions(steps.Assembly)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func compileC89(src, name string) (*hir.Module, error) {
	return c89.Compile(src, name)
}
