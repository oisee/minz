package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// TestGraceShowcase compiles the 9 official showcase programs through both
// Go and Grace paths, comparing assembly output, instruction count, and
// binary size. Produces a detailed report.
func TestGraceShowcase(t *testing.T) {
	nanzDir := filepath.Join("..", "..", "..", "examples", "nanz")

	// The 9 official showcase programs (from Report 096)
	showcase := []string{
		"01_sum_array.nanz",
		"02_sum_array_idiomatic.nanz",
		"03_filter_map_chain.nanz",
		"04_lut_popcount.nanz",
		"05_four_pointers.nanz",
		"06_pbqp_weighted.nanz",
		"07_ix_load_store.nanz",
		"08_arena_allocator.nanz",
		"09_function_pointers.nanz",
	}

	stats := mir2.NewGraceStats()

	type result struct {
		name      string
		goAsm     string
		graceAsm  string
		goInsts   int
		graceInsts int
		goErr     error
		graceErr  error
		identical bool
		goFuncs   int
		graceFuncs int
	}

	var results []result

	for _, name := range showcase {
		path := filepath.Join(nanzDir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Logf("SKIP %s: %v", name, err)
			continue
		}

		r := result{name: name}

		// Go path
		hm1, err := nanz.ParseWithOpts(string(src), name, nanz.ParseOpts{})
		if err != nil {
			r.goErr = fmt.Errorf("parse: %v", err)
			results = append(results, r)
			continue
		}
		goSteps, goErr := CompileHIRSteps(hm1, Options{ContractOpt: true})
		r.goErr = goErr
		if goErr == nil {
			r.goAsm = goSteps.Assembly
			r.goInsts = countInstructions(goSteps.Assembly)
			r.goFuncs = countFunctions(goSteps.Assembly)
		}

		// Grace path
		hm2, err := nanz.ParseWithOpts(string(src), name, nanz.ParseOpts{})
		if err != nil {
			r.graceErr = fmt.Errorf("parse: %v", err)
			results = append(results, r)
			continue
		}
		graceSteps, graceErr := CompileHIRSteps(hm2, Options{
			ContractOpt: true,
			UseGrace:    true,
			GraceStats:  stats,
		})
		r.graceErr = graceErr
		if graceErr == nil {
			r.graceAsm = graceSteps.Assembly
			r.graceInsts = countInstructions(graceSteps.Assembly)
			r.graceFuncs = countFunctions(graceSteps.Assembly)
		}

		r.identical = r.goAsm == r.graceAsm
		results = append(results, r)
	}

	// ── Report ──────────────────────────────────────────────────────────
	t.Log("")
	t.Log("╔═══════════════════════════════════════════════════════════════════╗")
	t.Log("║           Grace vs Go — Showcase Compilation Report             ║")
	t.Log("╚═══════════════════════════════════════════════════════════════════╝")
	t.Log("")
	t.Logf("%-35s %6s %6s %6s %s", "Program", "Go", "Grace", "Delta", "Status")
	t.Logf("%-35s %6s %6s %6s %s", strings.Repeat("─", 35), "──────", "──────", "──────", "──────")

	totalGo, totalGrace := 0, 0
	identical, diverged, goFail, graceFail := 0, 0, 0, 0

	for _, r := range results {
		status := ""
		delta := ""

		if r.goErr != nil {
			status = fmt.Sprintf("GO FAIL: %v", r.goErr)
			goFail++
		} else if r.graceErr != nil {
			status = fmt.Sprintf("GRACE FAIL: %v", r.graceErr)
			graceFail++
		} else if r.identical {
			status = "✓ identical"
			identical++
			delta = "0"
		} else {
			diff := r.graceInsts - r.goInsts
			if diff > 0 {
				delta = fmt.Sprintf("+%d", diff)
			} else {
				delta = fmt.Sprintf("%d", diff)
			}
			status = "≠ diverged"
			diverged++

			// Find first difference
			goLines := strings.Split(r.goAsm, "\n")
			grLines := strings.Split(r.graceAsm, "\n")
			for i := 0; i < len(goLines) && i < len(grLines); i++ {
				if goLines[i] != grLines[i] {
					t.Logf("  → first diff at line %d:", i+1)
					t.Logf("    Go:    %s", goLines[i])
					t.Logf("    Grace: %s", grLines[i])
					break
				}
			}
		}

		shortName := strings.TrimSuffix(r.name, ".nanz")
		t.Logf("%-35s %6d %6d %6s %s", shortName, r.goInsts, r.graceInsts, delta, status)
		totalGo += r.goInsts
		totalGrace += r.graceInsts
	}

	t.Log("")
	t.Logf("%-35s %6d %6d %6d", "TOTAL", totalGo, totalGrace, totalGrace-totalGo)
	t.Log("")
	t.Logf("Identical: %d/%d  Diverged: %d  Go fail: %d  Grace fail: %d",
		identical, len(results), diverged, goFail, graceFail)
	t.Log("")
	t.Logf("Grace rule applications: %s", stats)
	t.Log("")

	// Per-rule breakdown
	t.Log("Rule fire counts:")
	for name, count := range stats.ByRule {
		t.Logf("  %-25s %d", name, count)
	}
}

// countInstructions counts Z80 instructions in assembly output.
func countInstructions(asm string) int {
	count := 0
	for _, line := range strings.Split(asm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, ".") {
			continue
		}
		// Skip labels (end with :)
		if strings.HasSuffix(line, ":") {
			continue
		}
		// Skip directives
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "ORG") || strings.HasPrefix(upper, "DB") ||
			strings.HasPrefix(upper, "DW") || strings.HasPrefix(upper, "DS") ||
			strings.HasPrefix(upper, "EQU") || strings.HasPrefix(upper, "DEFW") ||
			strings.HasPrefix(upper, "DEFB") || strings.HasPrefix(upper, "DEFS") ||
			strings.HasPrefix(upper, "SECTION") || strings.HasPrefix(upper, "GLOBAL") {
			continue
		}
		count++
	}
	return count
}

// countFunctions counts function labels in assembly.
func countFunctions(asm string) int {
	count := 0
	for _, line := range strings.Split(asm, "\n") {
		if strings.HasPrefix(line, "; fun ") || strings.HasPrefix(line, "; fn ") {
			count++
		}
	}
	return count
}
