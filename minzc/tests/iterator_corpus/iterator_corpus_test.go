package iterator_corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/codegen"
	"github.com/minz/minzc/pkg/parser"
	"github.com/minz/minzc/pkg/semantic"
)

// knownZ80CodegenFailures lists files that pass parse+semantic but fail Z80 codegen.
// As of v0.19.2, OpPush/OpPop are implemented — enumerate and reduce compile to Z80.
var knownZ80CodegenFailures = map[string]bool{}

// TestIteratorCorpus runs all .minz files in the iterator corpus through the
// full compile pipeline (parse -> semantic -> codegen), verifying each compiles
// without errors.
func TestIteratorCorpus(t *testing.T) {
	corpusDir := "."
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("Failed to read corpus dir: %v", err)
	}

	var minzFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".minz") {
			minzFiles = append(minzFiles, entry.Name())
		}
	}

	if len(minzFiles) == 0 {
		t.Fatal("No .minz files found in iterator corpus")
	}

	t.Logf("Found %d .minz files in corpus", len(minzFiles))

	for _, file := range minzFiles {
		t.Run(strings.TrimSuffix(file, ".minz"), func(t *testing.T) {
			filePath := filepath.Join(corpusDir, file)
			if knownZ80CodegenFailures[file] {
				// These pass parse+semantic but fail Z80 codegen (pre-existing OpPush issue)
				testCompileToMIR(t, filePath)
			} else {
				testCompileFile(t, filePath)
			}
		})
	}
}

// testCompileFile compiles a single .minz file through the full pipeline
func testCompileFile(t *testing.T, path string) {
	t.Helper()

	// Read source
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}

	// Parse
	p := parser.New()
	decls, err := p.ParseString(string(source), path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	astFile := &ast.File{
		Name:         path,
		Declarations: decls,
	}

	// Semantic analysis
	analyzer := semantic.NewAnalyzer()
	module, err := analyzer.Analyze(astFile)
	if err != nil {
		t.Fatalf("Semantic error: %v", err)
	}

	// Code generation (Z80)
	var buf strings.Builder
	gen := codegen.NewZ80Generator(&buf)
	gen.SetTargetPlatform("cpm")
	if err := gen.Generate(module); err != nil {
		t.Fatalf("Codegen error: %v", err)
	}

	asmOutput := buf.String()
	if len(asmOutput) == 0 {
		t.Error("Empty assembly output")
	}

	// Basic sanity: should have at least a RET instruction
	if !strings.Contains(asmOutput, "RET") {
		t.Error("Expected RET instruction in output")
	}

	t.Logf("Compiled successfully: %d bytes of assembly", len(asmOutput))
}

// testCompileToMIR compiles a .minz file through parse + semantic only (no Z80 codegen).
// Used for programs that use IR features not yet in the Z80 backend.
func testCompileToMIR(t *testing.T, path string) {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}

	p := parser.New()
	decls, err := p.ParseString(string(source), path)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	astFile := &ast.File{
		Name:         path,
		Declarations: decls,
	}

	analyzer := semantic.NewAnalyzer()
	module, err := analyzer.Analyze(astFile)
	if err != nil {
		t.Fatalf("Semantic error: %v", err)
	}

	if len(module.Functions) == 0 {
		t.Error("Expected at least one function in module")
	}

	t.Logf("Compiled to MIR successfully: %d functions (Z80 codegen skipped — known OpPush issue)", len(module.Functions))
}
