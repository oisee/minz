package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// TestGraceConvergence compiles the Nanz corpus through BOTH the Go and Grace
// optimization paths and compares assembly output. Any difference is a
// convergence failure — the Grace rules didn't replicate the Go behavior.
func TestGraceConvergence(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	nanzDir := filepath.Join("..", "..", "..", "examples", "nanz")
	entries, err := os.ReadDir(nanzDir)
	if err != nil {
		t.Skipf("nanz examples not found: %v", err)
	}

	stats := mir2.NewGraceStats()
	total, matched, failed := 0, 0, 0

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".nanz") {
			continue
		}
		path := filepath.Join(nanzDir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Compile with Go path
		hm1, err := nanz.ParseWithOpts(string(src), e.Name(), nanz.ParseOpts{})
		if err != nil {
			continue // skip files that don't parse
		}
		goSteps, goErr := CompileHIRSteps(hm1, Options{ContractOpt: true})
		if goErr != nil {
			continue
		}
		total++

		// Compile with Grace path (re-parse to get fresh module)
		hm2, err := nanz.ParseWithOpts(string(src), e.Name(), nanz.ParseOpts{})
		if err != nil {
			continue
		}
		graceSteps, graceErr := CompileHIRSteps(hm2, Options{
			ContractOpt: true,
			UseGrace:    true,
			GraceStats:  stats,
		})
		if graceErr != nil {
			t.Logf("GRACE FAIL %s: %v", e.Name(), graceErr)
			failed++
			continue
		}

		// Compare assembly output
		if goSteps.Assembly == graceSteps.Assembly {
			matched++
		} else {
			failed++
			// Show first divergence
			goLines := strings.Split(goSteps.Assembly, "\n")
			grLines := strings.Split(graceSteps.Assembly, "\n")
			diffLine := -1
			maxLen := len(goLines)
			if len(grLines) < maxLen {
				maxLen = len(grLines)
			}
			for i := 0; i < maxLen; i++ {
				if goLines[i] != grLines[i] {
					diffLine = i
					break
				}
			}
			if diffLine >= 0 && diffLine < len(goLines) && diffLine < len(grLines) {
				t.Logf("DIVERGE %s line %d:", e.Name(), diffLine+1)
				t.Logf("  Go:    %s", goLines[diffLine])
				t.Logf("  Grace: %s", grLines[diffLine])
			} else {
				t.Logf("DIVERGE %s: Go=%d lines, Grace=%d lines", e.Name(), len(goLines), len(grLines))
			}
		}
	}

	t.Logf("\n=== Grace Convergence ===")
	t.Logf("Files: %d, Matched: %d, Diverged: %d", total, matched, failed)
	if total > 0 {
		t.Logf("Rate: %.1f%%", float64(matched)/float64(total)*100)
	}
	t.Logf("Grace stats: %s", stats)

	// Don't fail the test on divergences — this is informational.
	// As we add more rules, convergence should increase.
}

// TestGraceVerify runs the Grace path on the full Nanz corpus and checks
// that MIR2 verification passes (no structural corruption).
func TestGraceVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	nanzDir := filepath.Join("..", "..", "..", "examples", "nanz")
	entries, err := os.ReadDir(nanzDir)
	if err != nil {
		t.Skipf("nanz examples not found: %v", err)
	}

	total, pass, fail := 0, 0, 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".nanz") {
			continue
		}
		path := filepath.Join(nanzDir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		hm, err := nanz.ParseWithOpts(string(src), e.Name(), nanz.ParseOpts{})
		if err != nil {
			continue
		}
		total++

		_, graceErr := CompileHIRSteps(hm, Options{
			ContractOpt: true,
			UseGrace:    true,
		})
		if graceErr != nil {
			t.Logf("FAIL %s: %v", e.Name(), graceErr)
			fail++
		} else {
			pass++
		}
	}

	t.Logf("\n=== Grace Verification ===")
	t.Logf("Files: %d, Pass: %d, Fail: %d", total, pass, fail)
	if total > 0 {
		t.Logf("Rate: %.1f%%", float64(pass)/float64(total)*100)
	}

	if fail > 0 {
		t.Errorf("%d files failed Grace path verification", fail)
	}
}
