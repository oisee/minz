package nanz_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestFunImportFixture(t *testing.T) {
	root := findRepoRoot(t)
	fixtureDir := filepath.Join(root, "fun", "import_demo")
	mainPath := filepath.Join(fixtureDir, "main.nanz")

	src, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	hm, err := nanz.ParseWithOpts(string(src), "main.nanz", nanz.ParseOpts{
		BaseDir: fixtureDir,
	})
	if err != nil {
		t.Fatalf("parse with import fixture: %v", err)
	}
	if len(hm.Asserts) != 2 {
		t.Fatalf("asserts: want 2, got %d", len(hm.Asserts))
	}

	asmText, err := pipeline.CompileHIR(hm)
	if err != nil {
		t.Fatalf("compile import fixture: %v", err)
	}
	if !strings.Contains(asmText, "compute:") {
		t.Fatalf("compiled asm missing compute label:\n%s", asmText)
	}
	if strings.Contains(asmText, "lib__math__double:") {
		t.Fatalf("compiled asm still contains dead imported function label lib__math__double:\n%s", asmText)
	}
	if strings.Contains(asmText, "lib__math__add:") {
		t.Fatalf("compiled asm still contains dead imported function label lib__math__add:\n%s", asmText)
	}
}

func TestFunImportFixtureVIRSelfAddRegression(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not found")
	}

	root := findRepoRoot(t)
	fixtureDir := filepath.Join(root, "fun", "import_demo")
	mainPath := filepath.Join(fixtureDir, "main.nanz")

	src, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	hm, err := nanz.ParseWithOpts(string(src), "main.nanz", nanz.ParseOpts{
		BaseDir: fixtureDir,
	})
	if err != nil {
		t.Fatalf("parse with import fixture: %v", err)
	}

	steps, err := pipeline.CompileHIRSteps(hm, pipeline.Options{
		ContractOpt: true,
		UseVIR:      true,
		UseVIRDSE:   true,
		AssertMode:  "mir2",
	})
	if err != nil {
		t.Fatalf("compile import fixture via VIR: %v", err)
	}

	if !strings.Contains(steps.Assembly, "compute:\n    ADD A, A\n    INC A\n    RET") {
		t.Fatalf("compute did not collapse to ADD A, A sequence:\n%s", steps.Assembly)
	}
}
