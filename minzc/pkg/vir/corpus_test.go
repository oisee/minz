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

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/vir"
)

// TestVIR_NanzCorpus runs the VIR unified solver on all .nanz example files.
// For each file: Nanz→HIR→MIR2→VIR→Z3→PIR→ASM.
// Reports per-function and per-file pass/fail rates.
func TestVIR_NanzCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found, skipping corpus test")
	}

	exDir := filepath.Join("..", "..", "..", "examples", "nanz")
	files, err := filepath.Glob(filepath.Join(exDir, "*.nanz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .nanz examples found")
	}
	sort.Strings(files)

	opts := vir.SolverOptions{Timeout: 5 * time.Second}

	totalFiles := 0
	passFiles := 0
	totalFuncs := 0
	passFuncs := 0
	failFuncs := 0
	var failDetails []string
	var timings []time.Duration

	for _, f := range files {
		base := filepath.Base(f)
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		// Parse Nanz → HIR
		hm, err := nanz.ParseWithOpts(string(src), base, nanz.ParseOpts{})
		if err != nil {
			t.Logf("skip %s: parse: %v", base, err)
			continue
		}

		// HIR → MIR2
		m, err := lowerHIR(hm)
		if err != nil {
			t.Logf("skip %s: lower: %v", base, err)
			continue
		}

		totalFiles++
		start := time.Now()

		// VIR codegen on all functions
		_, results := vir.CodegenModule(m, opts)

		elapsed := time.Since(start)
		timings = append(timings, elapsed)

		filePass, fileFail := 0, 0
		for _, r := range results {
			totalFuncs++
			if r.OK {
				passFuncs++
				filePass++
			} else {
				failFuncs++
				fileFail++
				if len(failDetails) < 30 {
					failDetails = append(failDetails,
						fmt.Sprintf("%s/%s: %s", base, r.Name, r.Error))
				}
			}
		}

		if fileFail == 0 {
			passFiles++
		} else {
			t.Logf("%-30s %d/%d funcs OK (%.0fms)",
				base, filePass, filePass+fileFail, float64(elapsed.Microseconds())/1000)
		}
	}

	// Summary
	t.Logf("\n========== VIR Corpus Summary ==========")
	t.Logf("Files:     %d/%d (%.1f%%)", passFiles, totalFiles,
		pct(passFiles, totalFiles))
	t.Logf("Functions: %d/%d (%.1f%%)", passFuncs, totalFuncs,
		pct(passFuncs, totalFuncs))

	if len(timings) > 0 {
		var total time.Duration
		for _, d := range timings {
			total += d
		}
		t.Logf("Total Z3 time: %v (avg %.1fms/file)", total,
			float64(total.Microseconds())/float64(len(timings))/1000)
	}

	if len(failDetails) > 0 {
		t.Logf("\nFirst %d failures:", len(failDetails))
		for _, d := range failDetails {
			t.Logf("  %s", d)
		}
	}

	// Categorize failures
	cats := categorizeFailures(failDetails)
	if len(cats) > 0 {
		t.Logf("\nFailure categories:")
		for cat, count := range cats {
			t.Logf("  %-30s %d", cat, count)
		}
	}
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func categorizeFailures(details []string) map[string]int {
	cats := make(map[string]int)
	for _, d := range details {
		// Extract error portion after the last ":"
		parts := strings.SplitN(d, ": ", 2)
		if len(parts) < 2 {
			cats["unknown"]++
			continue
		}
		errMsg := parts[1]
		switch {
		case strings.Contains(errMsg, "unsatisfiable"):
			cats["unsat (need more patterns/moves)"]++
		case strings.Contains(errMsg, "z3 not found"):
			cats["z3 missing"]++
		case strings.Contains(errMsg, "timeout"):
			cats["z3 timeout"]++
		case strings.Contains(errMsg, "unsupported MIR2 op"):
			// Extract the op name
			cats["unsupported op"]++
		case strings.Contains(errMsg, "no pattern"):
			cats["no matching pattern"]++
		default:
			cats["other: "+truncate(errMsg, 40)]++
		}
	}
	return cats
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// lowerHIR performs HIR → MIR2 lowering with standard optimization passes.
func lowerHIR(hm *hir.Module) (*mir2.Module, error) {
	m := hir.LowerModule(hm)

	// Standard MIR2 optimization passes
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}

	// Liveness + allocation (needed for contract info used by bridge)
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ct := mir2.Z80CostTable{}
		mir2.Allocate(f, lr, ct)
	}

	return m, nil
}
