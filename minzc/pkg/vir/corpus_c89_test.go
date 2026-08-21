package vir_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/vir"
)

// TestVIR_C89Corpus runs the VIR unified solver on all C89 example files.
// This is the corpus used for SDCC comparison in the paper.
func TestVIR_C89Corpus(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found, skipping corpus test")
	}

	exDir := filepath.Join("..", "..", "..", "examples", "c89")
	files, err := filepath.Glob(filepath.Join(exDir, "*.c"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .c examples found")
	}
	sort.Strings(files)

	opts := vir.SolverOptions{Timeout: 30 * time.Second}

	totalFiles := 0
	passFiles := 0
	totalFuncs := 0
	passFuncs := 0
	var failDetails []string
	var timings []time.Duration

	for _, f := range files {
		base := filepath.Base(f)
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		// C89 → HIR
		hm, err := c89.CompileWithOpts(string(src), base, c89.CompileOpts{})
		if err != nil {
			t.Logf("skip %s: compile: %v", base, err)
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
			t.Logf("%-35s %d/%d funcs OK (%.0fms)",
				base, filePass, filePass+fileFail, float64(elapsed.Microseconds())/1000)
		}
	}

	// Summary
	t.Logf("\n========== VIR C89 Corpus Summary ==========")
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

	cats := categorizeFailures(failDetails)
	if len(cats) > 0 {
		t.Logf("\nFailure categories:")
		for cat, count := range cats {
			t.Logf("  %-30s %d", cat, count)
		}
	}
}

// lowerHIR_c89 is identical to lowerHIR for Nanz — shared via corpus_test.go
