package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestIndirectCallEmissionScope(t *testing.T) {
	source, err := filepath.Abs("../../../examples/c89/func_ptr.c")
	if err != nil {
		t.Fatal(err)
	}
	oldOutput, oldBackend, oldMode, oldForce, oldEmit := outputFile, backend, assertMode, assertForce, emitFormat
	defer func() {
		outputFile, backend, assertMode, assertForce, emitFormat = oldOutput, oldBackend, oldMode, oldForce, oldEmit
	}()
	for _, tc := range []struct {
		name, mode, be, emit, out string
		reject                    bool
	}{
		{"mir2", "mir2", "z80", "", "", false},
		{"all_mir2_annotations", "all", "z80", "", "", false},
		{"c", "all", "c", "", "output.c", false},
		{"mir2_dump", "all", "z80", "mir2", "output.mir2", false},
		{"hir_dump", "all", "z80", "hir", "output.hir", false},
		{"z80_assembly", "mir2", "z80", "", "output.a80", true},
		{"z80_binary", "none", "z80", "", "output.bin", true},
		{"default_z80_product", "none", "z80", "", "", true},
		{"z80_assert", "z80", "z80", "mir2", "output.mir2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outputFile, backend, assertMode, assertForce, emitFormat = "", tc.be, tc.mode, "", tc.emit
			if tc.out != "" {
				outputFile = filepath.Join(t.TempDir(), tc.out)
			}
			input := source
			if tc.name == "z80_assert" {
				src, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				input = filepath.Join(t.TempDir(), "func_ptr.c")
				if err := os.WriteFile(input, []byte(strings.ReplaceAll(string(src), "via mir2", "via z80")), 0644); err != nil {
					t.Fatal(err)
				}
			}
			err := compileViaHIR(input)
			if tc.reject {
				if err == nil || !strings.Contains(err.Error(), "caller preservation for the indirect ABI is unsupported") {
					t.Fatalf("want explicit Z80 rejection, got %v", err)
				}
				if outputFile != "" {
					if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
						t.Fatal("rejected compile wrote output")
					}
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if outputFile != "" {
					data, err := os.ReadFile(outputFile)
					if err != nil || len(data) == 0 {
						t.Fatalf("missing product: %v", err)
					}
					if tc.be == "c" && !strings.Contains(string(data), "#include") {
						t.Fatal("C backend did not emit C")
					}
				}
			}
		})
	}
}

// Skipping a product must still execute MIR2 assertions, including sandboxes.
// A Z80 sandbox assertion must force guarded emission despite the skip option.
func TestIndirectIntermediateAsserts(t *testing.T) {
	src, err := os.ReadFile("../../../examples/c89/func_ptr.c")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		bad, z80 bool
	}{
		{"mir2_sandbox", false, false},
		{"bad_mir2_sandbox", true, false},
		{"z80_sandbox", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hm, err := c89.Compile(string(src), "func_ptr.c")
			if err != nil {
				t.Fatal(err)
			}
			hm.Sandboxes = []hir.Sandbox{{Name: "indirect", Asserts: hm.Asserts}}
			hm.Asserts = nil
			if tc.bad {
				hm.Sandboxes[0].Asserts[0].Expected++
			}
			if tc.z80 {
				hm.Sandboxes[0].Asserts[0].Via = "z80"
			}
			steps, err := pipeline.CompileHIRSteps(hm, pipeline.Options{ContractOpt: true, SkipZ80Emission: true})
			switch {
			case tc.z80:
				if err == nil || !strings.Contains(err.Error(), "indirect ABI is unsupported") {
					t.Fatalf("want guarded Z80 emission, got %v", err)
				}
			case tc.bad:
				if err == nil || !strings.Contains(err.Error(), "sandbox") {
					t.Fatalf("MIR2 assertion was skipped: %v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if steps.Assembly != "" || steps.MIR2Module == nil {
					t.Fatal("wanted MIR2 without Z80 emission")
				}
			}
		})
	}
}
