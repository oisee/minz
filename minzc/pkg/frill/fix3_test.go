package frill

import (
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestAdoptedFunctionsEmittedOnce(t *testing.T) {
	for _, name := range []string{"functional_demo", "pipe", "showcase", "stdlib_demo"} {
		t.Run(name, func(t *testing.T) {
			path := "../../../examples/frill/" + name + ".frl"
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hm, err := CompileWithOpts(string(src), path, CompileOpts{BaseDir: filepath.Dir(path)})
			if err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, f := range hm.Funcs {
				if seen[f.Name] {
					t.Fatalf("duplicate function %q", f.Name)
				}
				seen[f.Name] = true
			}
			opts := pipeline.DefaultOptions()
			opts.AssertMode = "none"
			steps, err := pipeline.CompileHIRSteps(hm, opts)
			if err != nil {
				t.Fatal(err)
			}
			res, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Errors) > 0 {
				t.Fatal(res.Errors)
			}
			// MIR2 assertions are independent of emission and register allocation.
			if err := pipeline.RunAssertsMIR2(hm, steps.MIR2Module); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestImportsKeepFirstFunctionBinding(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{
		"first.frl":  "let step (x : u8) = x + 1",
		"second.frl": "let step (x : u8) (y : u8) = x + y",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
	}
	hm, err := CompileWithOpts(`import "first.frl"
import "second.frl"
let main (x : u8) = step x
assert main 4 == 5`, "imports.frl", CompileOpts{BaseDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(hm.Funcs) != 2 || len(hm.Funcs[0].Params) != 1 {
		t.Fatalf("functions = %+v", hm.Funcs)
	}
	if _, err := pipeline.CompileHIR(hm); err != nil {
		t.Fatal(err)
	}
}
