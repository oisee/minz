package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/plm"
)

// TestGrace_AllFrontends compiles ALL examples across ALL frontends through
// both Go and Grace paths. Reports per-file: compile status, instruction count,
// convergence with Go path.
func TestGrace_AllFrontends(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus-scale test; runs under `make test-all`, skipped by -short")
	}
	examplesRoot := filepath.Join("..", "..", "..", "examples")

	type frontend struct {
		name    string
		dir     string
		glob    string
		compile func(string, string) (*hir.Module, error)
	}

	frontends := []frontend{
		{"nanz", filepath.Join(examplesRoot, "nanz"), "*.nanz",
			func(src, name string) (*hir.Module, error) {
				return nanz.ParseWithOpts(src, name, nanz.ParseOpts{})
			}},
		{"lanz", filepath.Join(examplesRoot, "lizp"), "*.lanz",
			func(src, name string) (*hir.Module, error) {
				return lanz.Compile(src, name)
			}},
		{"lizp", filepath.Join(examplesRoot, "lizp"), "*.lizp",
			func(src, name string) (*hir.Module, error) {
				return lizp.Compile(src, name)
			}},
		{"plm", filepath.Join(examplesRoot, "nanz"), "*.plm",
			func(src, name string) (*hir.Module, error) {
				return plm.Compile(src)
			}},
		{"pascal", filepath.Join(examplesRoot, "pascal"), "*.pas",
			func(src, name string) (*hir.Module, error) {
				return pascal.Compile(src, name)
			}},
		{"c89", filepath.Join(examplesRoot, "c89"), "*.c",
			func(src, name string) (*hir.Module, error) {
				return c89.Compile(src, name)
			}},
	}

	stats := mir2.NewGraceStats()
	totalFiles, totalPass, totalIdentical, totalDiverge, totalFail := 0, 0, 0, 0, 0
	totalGoInsts, totalGraceInsts := 0, 0

	t.Log("")
	t.Log("╔════════════════════════════════════════════════════════════════════════════╗")
	t.Log("║          Grace vs Go — ALL Frontends Compilation Report                  ║")
	t.Log("╚════════════════════════════════════════════════════════════════════════════╝")

	for _, fe := range frontends {
		files, err := filepath.Glob(filepath.Join(fe.dir, fe.glob))
		if err != nil || len(files) == 0 {
			t.Logf("\n── %s: no files found ──", fe.name)
			continue
		}

		t.Logf("\n── %s (%d files) ──", strings.ToUpper(fe.name), len(files))
		t.Logf("%-35s %6s %6s %6s %s", "File", "Go", "Grace", "Delta", "Status")
		t.Logf("%-35s %6s %6s %6s %s", strings.Repeat("─", 35), "──────", "──────", "──────", "──────")

		fePass, feFail, feIdentical, feDiverge := 0, 0, 0, 0

		for _, f := range files {
			base := filepath.Base(f)
			src, err := os.ReadFile(f)
			if err != nil {
				continue
			}

			// Go path
			hm1, err := fe.compile(string(src), base)
			if err != nil {
				t.Logf("%-35s %6s %6s %6s PARSE FAIL", base, "-", "-", "-")
				continue
			}
			goSteps, goErr := CompileHIRSteps(hm1, Options{ContractOpt: true})
			if goErr != nil {
				t.Logf("%-35s %6s %6s %6s GO FAIL: %v", base, "-", "-", "-", goErr)
				continue
			}
			totalFiles++
			goInsts := countInstructions(goSteps.Assembly)

			// Grace path
			hm2, err := fe.compile(string(src), base)
			if err != nil {
				t.Logf("%-35s %6d %6s %6s GRACE PARSE FAIL", base, goInsts, "-", "-")
				feFail++
				totalFail++
				continue
			}
			graceSteps, graceErr := CompileHIRSteps(hm2, Options{
				ContractOpt: true,
				UseGrace:    true,
				GraceStats:  stats,
			})
			if graceErr != nil {
				t.Logf("%-35s %6d %6s %6s GRACE FAIL: %v", base, goInsts, "-", "-", graceErr)
				feFail++
				totalFail++
				continue
			}

			graceInsts := countInstructions(graceSteps.Assembly)
			totalGoInsts += goInsts
			totalGraceInsts += graceInsts
			fePass++
			totalPass++

			delta := graceInsts - goInsts
			deltaStr := "0"
			if delta > 0 {
				deltaStr = "+" + strings.Repeat("", 0) + string(rune('0'+delta%10))
				deltaStr = formatDelta(delta)
			} else if delta < 0 {
				deltaStr = formatDelta(delta)
			}

			if goSteps.Assembly == graceSteps.Assembly {
				t.Logf("%-35s %6d %6d %6s ✓ identical", base, goInsts, graceInsts, deltaStr)
				feIdentical++
				totalIdentical++
			} else {
				t.Logf("%-35s %6d %6d %6s ≠ diverged", base, goInsts, graceInsts, deltaStr)
				feDiverge++
				totalDiverge++
			}
		}

		t.Logf("%-35s Pass: %d  Identical: %d  Diverged: %d  Fail: %d",
			fe.name+" subtotal:", fePass, feIdentical, feDiverge, feFail)
	}

	t.Log("")
	t.Log("════════════════════════════════════════════════════════════════════════════")
	t.Logf("TOTAL FILES: %d  Pass: %d  Identical: %d  Diverged: %d  Fail: %d",
		totalFiles, totalPass, totalIdentical, totalDiverge, totalFail)
	t.Logf("TOTAL INSTRUCTIONS: Go=%d  Grace=%d  Delta=%d",
		totalGoInsts, totalGraceInsts, totalGraceInsts-totalGoInsts)
	if totalFiles > 0 {
		t.Logf("Convergence rate: %.1f%%", float64(totalIdentical)/float64(totalFiles)*100)
	}
	t.Log("")
	t.Logf("Grace stats: %s", stats)
	t.Log("")
	t.Log("Rule fire counts:")
	for name, count := range stats.ByRule {
		t.Logf("  %-25s %d", name, count)
	}
}

func formatDelta(d int) string {
	if d > 0 {
		return "+" + itoa(d)
	}
	return itoa(d)
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
