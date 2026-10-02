package hir_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// Independent Python-model assertions from the critic exercise caller restores,
// returned tuples and scratch-register reuse through the default PBQP pipeline.
func TestProductionCallFixRound1(t *testing.T) {
	paths, err := filepath.Glob("testdata/call_fix1/*.nanz")
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixtures: %v", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hm, err := nanz.Parse(string(src), path)
			if err != nil {
				t.Fatal(err)
			}
			opts := pipeline.DefaultOptions()
			opts.AssertMode = "all"
			steps, err := pipeline.CompileHIRSteps(hm, opts)
			if filepath.Base(path) == "indirect.nanz" {
				if err == nil || !strings.Contains(err.Error(), "indirect call in apply has live values") {
					t.Fatalf("want unsupported indirect ABI error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(steps.Assembly, "_call_result_") {
				t.Fatal("call result uses writable inline code storage")
			}
		})
	}
}

func TestProductionCallResultStorage(t *testing.T) {
	hm, err := nanz.Parse(`
fun retword(a:u16,b:u16)->u16 { if a<b { return a } return b }
fun romcall(a:u16,b:u16,c:u16)->u16 { return retword(a,b)+a+c }
`, "rom_calls")
	if err != nil {
		t.Fatal(err)
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	steps, err := pipeline.CompileHIRSteps(hm, opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(steps.Assembly, "_call_result_") {
		t.Fatal("call results must not write to inline code storage")
	}
}
