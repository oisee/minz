package nanz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/z80asm"
)

// Known failures — skip these until the underlying bugs are fixed.
// Each entry is the base filename (without extension).
var knownFailures = map[string]string{
	// Nanz compile failures
	"ex4a_abs_diff_iar": "compile: IAR-style function not supported",
	"ex14_fold_assert":  "compile: fold assertion not wired in showcase context",
	// Nanz asm failures
	"ex20_arena_allocator": "asm: BUG-008 arena codegen (LD IXL, (IX+d))",
	"08_arena_allocator":   "asm: BUG-008 arena codegen (LD IXL, (IX+d))",
	"ex34_tetris":          "asm: tetris needs clobber save/restore (WIP)",
	"tetris":               "asm: tetris v1 needs clobber save/restore (WIP)",
	"tetris_v2":            "asm: tetris v2 needs clobber save/restore (WIP)",
	"plasma":               "asm: new example, not yet verified",
	// Import tests need module search path
	"ex23_import_unqualified": "compile: import needs module search path",
	"ex24_import_qualified":   "compile: import needs module search path",
	"ex25_import_alias_glob":  "compile: import needs module search path",
	"nc":                      "compile: import needs module search path (fs.fat12)",
	// Lizp asm failures (MZA encoding gaps)
	"cond_test":        "asm: lizp codegen produces instructions MZA can't encode yet",
	"factorial":        "asm: lizp codegen produces instructions MZA can't encode yet",
	"showcase":         "asm: lizp codegen produces instructions MZA can't encode yet",
	"zx_checkerboard":  "asm: lizp codegen produces instructions MZA can't encode yet",
	"zx_rainbow":       "asm: lizp codegen produces instructions MZA can't encode yet",
	// PLM asm failures
	"01_sum_array":      "asm: PLM codegen produces instructions MZA can't encode yet",
	"01_sum_plm_origin": "asm: PLM codegen produces instructions MZA can't encode yet",
	// Pascal asm failures (new frontend, codegen gaps)
	"casetest":  "asm: Pascal case codegen not fully wired",
	"sieve":     "asm: Pascal array codegen not fully wired",
	// "factorial" already listed under lizp — covers both .lizp and .pas
}

// TestShowcaseCompileAssemble compiles every .nanz/.lanz/.plm/.lizp file
// found in reports/showcase-src/ and examples/ through the full pipeline:
//
//	source → HIR → MIR2 → Z80 asm → assemble
//
// This catches regressions in the compiler, allocator, codegen, and assembler
// across all frontends.
func TestShowcaseCompileAssemble(t *testing.T) {
	root := findRepoRoot(t)

	dirs := []string{
		filepath.Join(root, "reports", "showcase-src"),
		filepath.Join(root, "examples"),
	}

	var files []string
	for _, dir := range dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := filepath.Ext(path)
			if ext == ".nanz" || ext == ".lanz" || ext == ".plm" || ext == ".lizp" || ext == ".pas" {
				files = append(files, path)
			}
			return nil
		})
	}

	if len(files) == 0 {
		t.Skip("no showcase files found")
	}

	t.Logf("Found %d showcase files", len(files))

	for _, path := range files {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		ext := filepath.Ext(path)

		t.Run(base+ext, func(t *testing.T) {
			if reason, ok := knownFailures[base]; ok {
				t.Skipf("known failure: %s", reason)
			}

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			// Step 1: Parse to HIR via the appropriate frontend
			var hm *hir.Module
			switch ext {
			case ".nanz":
				hm, err = nanz.Parse(string(src), base)
			case ".lanz":
				hm, err = lanz.Compile(string(src), base)
			case ".plm":
				hm, err = plm.Compile(string(src))
			case ".lizp":
				hm, err = lizp.Compile(string(src), base)
			case ".pas":
				hm, err = pascal.Compile(string(src), base)
			default:
				t.Skipf("unsupported extension: %s", ext)
				return
			}
			if err != nil {
				t.Fatalf("frontend %s: %v", ext, err)
			}

			// Step 2: HIR → MIR2 → Z80 asm
			asmText, err := pipeline.CompileHIR(hm)
			if err != nil {
				t.Fatalf("pipeline: %v", err)
			}

			// Step 3: Assemble
			assembler := z80asm.NewAssembler()
			result, err := assembler.AssembleString(asmText)
			if err != nil {
				t.Fatalf("assemble: %v", err)
			}
			if len(result.Errors) > 0 {
				for _, e := range result.Errors {
					t.Errorf("asm error: %v", e)
				}
				t.Fatalf("assembly produced %d errors", len(result.Errors))
			}

			// Step 4: Sanity checks — catch common codegen bugs
			for _, line := range strings.Split(asmText, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, ";") {
					continue
				}
				if strings.Contains(trimmed, "AND stack") {
					t.Errorf("codegen bug: 'AND stack' (LocStack allocated): %s", trimmed)
				}
				if strings.Contains(trimmed, "LD A, ?") {
					t.Errorf("codegen bug: 'LD A, ?' (unresolved forward ref): %s", trimmed)
				}
			}

			t.Logf("OK: %d bytes asm, %d bytes binary", len(asmText), len(result.Binary))
		})
	}
}

// findRepoRoot walks up from the test file to find the repo root.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (CLAUDE.md)")
		}
		dir = parent
	}
}
