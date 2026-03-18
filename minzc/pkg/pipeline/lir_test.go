package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/nanz"
)

// TestLIR_C89Corpus runs the LIR convergence check on the C89 test corpus.
// For each .c file, it compiles through C89→HIR→MIR2, then runs
// CheckModuleConvergence to verify LIR can process the functions.
func TestLIR_C89Corpus(t *testing.T) {
	exDir := filepath.Join("..", "..", "..", "examples", "c89")
	files, err := filepath.Glob(filepath.Join(exDir, "*.c"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no C89 examples found")
	}

	totalFuncs := 0
	passedFuncs := 0
	failedFuncs := 0
	var failDetails []string

	for _, f := range files {
		base := filepath.Base(f)
		src, err := os.ReadFile(f)
		if err != nil {
			t.Logf("skip %s: %v", base, err)
			continue
		}

		hm, err := c89.CompileWithOpts(string(src), base, c89.CompileOpts{})
		if err != nil {
			t.Logf("skip %s: compile error: %v", base, err)
			continue
		}

		steps, err := CompileHIRSteps(hm, Options{
			ContractOpt: true,
			LIRCheck:    true,
		})
		if err != nil {
			t.Logf("skip %s: pipeline error: %v", base, err)
			continue
		}

		// Count results
		filePass, fileFail := 0, 0
		for _, r := range steps.LIRResults {
			totalFuncs++
			if r.Match && r.Error == "" {
				passedFuncs++
				filePass++
			} else {
				failedFuncs++
				fileFail++
				if r.Error != "" {
					failDetails = append(failDetails,
						base+"/"+r.FuncName+"/"+r.Machine+": "+r.Error)
				}
			}
		}

		if fileFail > 0 {
			t.Logf("%s: %d/%d funcs passed LIR", base, filePass, filePass+fileFail)
		}
	}

	t.Logf("\n=== LIR Convergence Summary ===")
	t.Logf("Files: %d", len(files))
	t.Logf("Functions (×3 machines): %d total, %d passed, %d failed",
		totalFuncs, passedFuncs, failedFuncs)
	if totalFuncs > 0 {
		pct := float64(passedFuncs) / float64(totalFuncs) * 100
		t.Logf("Pass rate: %.1f%%", pct)
	}

	// Show first 20 failures
	if len(failDetails) > 0 {
		t.Logf("\nFirst failures:")
		for i, d := range failDetails {
			if i >= 20 {
				t.Logf("  ... and %d more", len(failDetails)-20)
				break
			}
			t.Logf("  %s", d)
		}
	}
}

// TestLIR_SingleFunc tests LIR on a single hand-written C function.
func TestLIR_SingleFunc(t *testing.T) {
	src := `int add(int a, int b) { return a + b; }`
	hm, err := c89.CompileWithOpts(src, "test.c", c89.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	steps, err := CompileHIRSteps(hm, Options{
		ContractOpt: true,
		LIRCheck:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range steps.LIRResults {
		if r.Error != "" {
			t.Logf("%s/%s: %s", r.FuncName, r.Machine, r.Error)
		} else {
			t.Logf("%s/%s: ✓ ops=%d→%d insts=%d",
				r.FuncName, r.Machine, r.OpCount, r.Combined, r.InstCount)
		}
	}

	// Verify at least one machine passes
	anyPass := false
	for _, r := range steps.LIRResults {
		if r.Match && r.Error == "" {
			anyPass = true
			break
		}
	}
	if !anyPass {
		t.Error("no machine passed for simple add function")
	}
}

// TestLIR_CodegenOutput tests that LIR produces readable Z80 assembly.
func TestLIR_CodegenOutput(t *testing.T) {
	src := `unsigned char double_val(unsigned char x) { return x + x; }`
	hm, err := c89.CompileWithOpts(src, "test.c", c89.CompileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	m := hir.LowerModule(hm)
	if len(m.Funcs) == 0 {
		t.Fatal("no functions")
	}

	asm, err := lir.LIRCodegenFunc(m.Funcs[0], m)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("LIR Z80 assembly:\n%s", asm)
}

// ── Multi-frontend corpus tests ──────────────────────────────────────────────

// TestLIR_NanzCorpus runs LIR convergence on all .nanz examples.
func TestLIR_NanzCorpus(t *testing.T) {
	runFrontendCorpus(t, "nanz", filepath.Join("..", "..", "..", "examples", "nanz"), "*.nanz",
		func(src, name string) (*hir.Module, error) {
			return nanz.ParseWithOpts(src, name, nanz.ParseOpts{})
		})
}

// TestLIR_LanzCorpus runs LIR convergence on all .lanz examples.
func TestLIR_LanzCorpus(t *testing.T) {
	// Lanz examples live alongside lizp examples
	runFrontendCorpus(t, "lanz", filepath.Join("..", "..", "..", "examples", "lizp"), "*.lanz",
		func(src, name string) (*hir.Module, error) {
			return lanz.Compile(src, name)
		})
}

// TestLIR_LizpCorpus runs LIR convergence on all .lizp examples.
func TestLIR_LizpCorpus(t *testing.T) {
	runFrontendCorpus(t, "lizp", filepath.Join("..", "..", "..", "examples", "lizp"), "*.lizp",
		func(src, name string) (*hir.Module, error) {
			return lizp.Compile(src, name)
		})
}

// runFrontendCorpus is a generic corpus runner for any frontend that produces *hir.Module.
func runFrontendCorpus(t *testing.T, lang, dir, glob string, compile func(string, string) (*hir.Module, error)) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, glob))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skipf("no %s examples found in %s", lang, dir)
	}

	totalFuncs := 0
	passedFuncs := 0
	var failSummary []string

	for _, f := range files {
		base := filepath.Base(f)
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		hm, err := compile(string(src), base)
		if err != nil {
			t.Logf("skip %s: %v", base, err)
			continue
		}

		steps, err := CompileHIRSteps(hm, Options{ContractOpt: true, LIRCheck: true})
		if err != nil {
			t.Logf("skip %s: pipeline: %v", base, err)
			continue
		}

		filePass, fileFail := 0, 0
		for _, r := range steps.LIRResults {
			totalFuncs++
			if r.Match && r.Error == "" {
				passedFuncs++
				filePass++
			} else {
				fileFail++
				if len(failSummary) < 10 && r.Error != "" {
					failSummary = append(failSummary,
						base+"/"+r.FuncName+"/"+r.Machine+": "+r.Error)
				}
			}
		}
		if fileFail > 0 {
			t.Logf("%s: %d/%d passed", base, filePass, filePass+fileFail)
		}
	}

	t.Logf("\n=== %s LIR Summary ===", lang)
	t.Logf("Files: %d, Functions(×3): %d, Passed: %d", len(files), totalFuncs, passedFuncs)
	if totalFuncs > 0 {
		t.Logf("Pass rate: %.1f%%", float64(passedFuncs)/float64(totalFuncs)*100)
	}
	for _, s := range failSummary {
		t.Logf("  %s", s)
	}
}
