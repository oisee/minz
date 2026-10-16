package c89_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

// ISO C89 aggregate return and parameter values include the current field
// values; the by-value callee receives the value, not an erased caller object.
func TestC89AggregateScalarizationBehavior(t *testing.T) {
	for _, name := range []string{"mutated_return", "struct_roundtrip", "parameter_mutation", "address_escape", "indirect_parameter", "volatile_parameter"} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", "aggregate_scalarization", name+".c"))
			if err != nil {
				t.Fatal(err)
			}
			parse := func() *hir.Module {
				m, err := c89.Compile(string(src), name+".c")
				if err != nil {
					t.Fatal(err)
				}
				if err := hir.Verify(m); err != nil {
					t.Fatalf("C source produced invalid HIR: %v", err)
				}
				return m
			}
			want := func(x int64) int64 {
				hi := byte(x)
				if name != "struct_roundtrip" {
					hi += 2
				}
				result := uint16(hi)*256 + uint16(byte(x+1))
				if name == "parameter_mutation" || name == "address_escape" || name == "indirect_parameter" || name == "volatile_parameter" {
					result += uint16(byte(x))
				}
				return int64(result)
			}
			inputs := []int64{0, 1, 2, 10, 127, 128, 253, 254, 255}
			raw := mir2.NewVM(hir.LowerModule(parse()))
			for _, x := range inputs {
				r, err := raw.Call("check", []mir2.Value{{I: x}})
				if err != nil || len(r) != 1 || r[0].I != want(x) {
					t.Fatalf("raw check(%d)=%v err=%v want=%d", x, r, err, want(x))
				}
			}
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(mode, func(t *testing.T) {
					m := parse()
					for _, x := range inputs {
						m.Asserts = append(m.Asserts, hir.Assert{FuncName: "check", Args: []int64{x}, Expected: want(x), Via: "z80"})
					}
					steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, AssertMode: "z80", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
					if err != nil {
						t.Fatal(err)
					}
					vm := mir2.NewVM(steps.MIR2Module)
					for _, x := range inputs {
						r, err := vm.Call("check", []mir2.Value{{I: x}})
						if err != nil || len(r) != 1 || r[0].I != want(x) {
							t.Fatalf("final check(%d)=%v err=%v want=%d", x, r, err, want(x))
						}
					}
				})
			}
		})
	}
}

// Optimization evidence is separate from value semantics: the caller/callee
// ABI must really carry scalar fields, not keep an invisible struct copy.
func TestC89AggregateScalarizationABI(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "aggregate_scalarization", "struct_roundtrip.c"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c89.Compile(string(src), "struct_roundtrip.c")
	if err != nil {
		t.Fatal(err)
	}
	var maker, consumer *hir.Func
	for _, f := range m.Funcs {
		switch f.Name {
		case "make_pair":
			maker = f
		case "consume":
			consumer = f
		}
	}
	if maker == nil || len(maker.RetTys) != 2 {
		t.Fatal("eligible Pair return not decomposed")
	}
	if consumer == nil || len(consumer.Params) != 2 || consumer.Params[0].Ty != mir2.TyU8 || consumer.Params[1].Ty != mir2.TyU8 {
		t.Fatal("eligible by-value Pair input not decomposed into scalar arguments")
	}
	checkMIR := func(stage string, mm *mir2.Module) {
		for _, f := range mm.Funcs {
			if f.Name == "make_pair" && len(f.Contract.Returns) != 2 {
				t.Fatalf("%s: return tuple lost before code generation", stage)
			}
			if f.Name == "consume" && len(f.Contract.Params) != 2 {
				t.Fatalf("%s: scalar parameters lost before code generation", stage)
			}
			for _, b := range f.Blocks {
				for _, inst := range b.Insts {
					if inst.Op == mir2.OpAlloca {
						t.Fatalf("%s %s: eligible aggregate still allocates memory", stage, f.Name)
					}
				}
			}
		}
	}
	checkMIR("raw", hir.LowerModule(m))
	steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true})
	if err != nil {
		t.Fatal(err)
	}
	checkMIR("final", steps.MIR2Module)

}

func TestC89AggregateParameterMemoryFallback(t *testing.T) {
	for _, name := range []string{"address_escape", "indirect_parameter", "volatile_parameter"} {
		src, err := os.ReadFile(filepath.Join("testdata", "aggregate_scalarization", name+".c"))
		if err != nil {
			t.Fatal(err)
		}
		m, err := c89.Compile(string(src), name+".c")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range m.Funcs {
			if f.Name == "consume" {
				if len(f.Params) != 1 || f.Params[0].Ty != mir2.TyPtr {
					t.Fatalf("%s: incompatible scalar ABI at address/indirect boundary", name)
				}
			}
		}
	}
}
