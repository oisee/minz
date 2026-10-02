package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// Parse afresh on every iteration: lowering mutates HIR, and map iteration
// must vary in the frontend as well as in allocation and code generation.
func TestProductionDeterminism(t *testing.T) {
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
