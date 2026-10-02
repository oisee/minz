package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// Parse afresh on every iteration: lowering mutates HIR, and map iteration
// must vary in the frontend as well as in allocation and code generation.
func TestProductionDeterminism(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "946684801")
	for _, input := range []string{
		"nanz/self_tokenizer.nanz", "nanz/01_sum_array.nanz",
		"nanz/09_function_pointers.nanz", "nanz/08_arena_allocator.nanz",
		"nanz/hello_cpm.nanz", "c89/bench.c", "objc/inherit.m",
	} {
		t.Run(input, func(t *testing.T) {
			path, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", input))
			if err != nil {
				t.Fatal(err)
			}
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var want string
			for run := 0; run < 20; run++ {
				var hm *hir.Module
				if filepath.Ext(path) == ".c" || filepath.Ext(path) == ".m" {
					hm, err = c89.CompileWithOpts(string(src), filepath.Base(path), c89.CompileOpts{BaseDir: filepath.Dir(path), IncludePaths: []string{filepath.Dir(path)}})
				} else {
					hm, err = nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(filepath.Dir(path), "..", "..", "stdlib")})
				}
				if err != nil {
					t.Fatal(err)
				}
				opts := DefaultOptions()
				opts.AssertMode = "none"
				steps, err := CompileHIRSteps(hm, opts)
				if err != nil {
					t.Fatal(err)
				}
				if run == 0 {
					want = steps.Assembly
				} else if steps.Assembly != want {
					t.Fatalf("run %d differs from run 0 (%d vs %d assembly bytes)", run, len(steps.Assembly), len(want))
				}
			}
		})
	}
}

// Explicit optional backends retain the MIR2 checks for via mir2 assertions.
func TestOptionalBackendRetainsMIR2Assertions(t *testing.T) {
	for _, mode := range []string{"wasm", "llvm"} {
		t.Run(mode, func(t *testing.T) {
			hm, err := nanz.Parse("fun answer() -> u8 { return 42 }\nassert answer() == 43 via mir2\n", "test.nanz")
			if err != nil {
				t.Fatal(err)
			}
			_, err = CompileHIRSteps(hm, Options{AssertMode: mode})
			if err == nil || !strings.Contains(err.Error(), "[mir2]") {
				t.Fatalf("expected MIR2 failure, got %v", err)
			}
		})
	}
}

func TestLLVMModeRetainsZ80Assertions(t *testing.T) {
	hm, err := nanz.Parse("fun answer() -> u8 { return 42 }\nassert answer() == 43 via z80\n", "test.nanz")
	if err != nil {
		t.Fatal(err)
	}
	hm.AssertStats = &hir.AssertStats{}
	steps, err := CompileHIRSteps(hm, Options{AssertMode: "llvm"})
	if steps.Assembly == "" {
		t.Fatal("expected emitted Z80 assembly")
	}
	if err == nil || !strings.Contains(err.Error(), "[z80]") {
		t.Fatalf("expected Z80 failure before LLVM, got %v", err)
	}
	if *hm.AssertStats != (hir.AssertStats{Executed: 1, Failed: 1}) {
		t.Fatalf("stats=%+v", hm.AssertStats)
	}
}
