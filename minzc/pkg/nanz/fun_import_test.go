package nanz_test

import (
	"os"
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
	if !strings.Contains(asmText, "lib__math__double:") {
		t.Fatalf("compiled asm missing imported function label lib__math__double:\n%s", asmText)
	}
	if !strings.Contains(asmText, "lib__math__add:") {
		t.Fatalf("compiled asm missing imported function label lib__math__add:\n%s", asmText)
	}
}
