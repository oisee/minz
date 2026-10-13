package nanz_test

import (
	"encoding/json"
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

// Known failures are exact repository-relative paths. Basenames are not unique:
// showcase.lizp must not hide showcase.plm, nor factorial.lizp factorial.pas.
// MINZ_SHOWCASE_CHECK_KNOWN=1 executes these cases for an explicit failure audit.
var knownFailures = map[string]string{
	"examples/zmachine/spectrum128/zvm_core.nanz": "host core template: compile through scripts/zmachine/prepare_native_core.py for explicit banked Z80 memory",
	"examples/zmachine/zork-west-demo/zvm.nanz":   "host-only ZVM: story[131072] exceeds ordinary Z80 16-bit address space; native u32 spills are supported",
}

// TestShowcaseCompileAssemble compiles every .nanz/.lanz/.plm/.lizp/.pas file
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
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				t.Fatal(err)
			}
			stage := "skip"
			defer func() {
				status := "PASS"
				if t.Skipped() {
					status = "SKIP"
				} else if t.Failed() {
					status = "FAIL"
				}
				row := struct{ Path, Case, Status, Stage string }{filepath.ToSlash(relPath), t.Name(), status, stage}
				encoded, _ := json.Marshal(row)
				t.Logf("SHOWCASE_CASE %s", encoded)
			}()
			t.Logf("SOURCE_PATH %s", filepath.ToSlash(relPath))
			if reason, ok := knownFailures[filepath.ToSlash(relPath)]; ok && os.Getenv("MINZ_SHOWCASE_CHECK_KNOWN") != "1" {
				t.Skipf("known failure: %s", reason)
			}

			stage = "read"
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			stage = "frontend"
			// Step 1: Parse to HIR via the appropriate frontend
			var hm *hir.Module
			switch ext {
			case ".nanz":
				hm, err = nanz.ParseWithOpts(string(src), base, nanz.ParseOpts{
					BaseDir:   filepath.Dir(path),
					StdlibDir: filepath.Join(root, "stdlib"),
				})
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

			stage = "pipeline"
			// Step 2: HIR → MIR2 → Z80 asm
			asmText, err := pipeline.CompileHIR(hm)
			if err != nil {
				t.Fatalf("pipeline: %v", err)
			}

			// Opt-in diagnostics retain compiler output before assembly.
			if dir := os.Getenv("MINZ_SHOWCASE_ARTIFACTS"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, base+ext+".a80"), []byte(asmText), 0644); err != nil {
					t.Fatal(err)
				}
			}

			stage = "assembler"
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

			stage = "sanity"
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

			stage = "complete"
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
