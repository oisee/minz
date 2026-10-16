package nanz_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func TestCImportedAssertNativeVM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "arith.c"), []byte(`unsigned long step(unsigned long x){return x+1UL;}unsigned char valid(unsigned long x){return x!=0;}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, form := range []struct{ name, imp, step, valid string }{
		{"selected", "import arith {step,valid}", "step", "valid"},
		{"aliased", "import arith {step as next,valid as check}", "next", "check"},
		{"qualified", "import arith", "arith.step", "arith.valid"},
		{"glob", "import arith {*}", "step", "valid"},
	} {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(form.name+"/"+mode, func(t *testing.T) {
				source := form.imp + fmt.Sprintf("\nassert %s(65535)==65536 via z80\nassert %s(4294967295)==0 via z80\nassert %s(1) via z80\nassert not %s(0) via z80\n", form.step, form.step, form.valid, form.valid)
				parse := func() *hir.Module {
					m, err := nanz.ParseWithOpts(source, "asserts.nanz", nanz.ParseOpts{BaseDir: dir})
					if err != nil {
						t.Fatal(err)
					}
					return m
				}
				m := parse()
				for _, a := range m.Asserts {
					if a.FuncName != "arith__step" && a.FuncName != "arith__valid" {
						t.Fatal("unresolved", a.FuncName)
					}
				}
				steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "z80", ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if err != nil {
					t.Fatal(err)
				}
				for _, ir := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
					for _, a := range m.Asserts {
						args := make([]mir2.Value, len(a.Args))
						for i, v := range a.Args {
							args[i] = mir2.Value{I: v}
						}
						got, err := mir2.NewVM(ir).Call(a.FuncName, args)
						if err != nil || len(got) != 1 || got[0].I != a.Expected {
							t.Fatalf("%s VM got=%v err=%v want=%d", a.Source, got, err, a.Expected)
						}
					}
				}
			})
		}
	}
}

// Execute the checked-in example itself with an oracle independent of its asserts.
func TestSystemCInteropExampleNativeVM(t *testing.T) {
	path := "../../../examples/system-c/interop/main.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *hir.Module {
		m, err := nanz.ParseWithOpts(string(source), path, nanz.ParseOpts{BaseDir: filepath.Dir(path)})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	cases := []hir.Assert{{FuncName: "main", Expected: 65536, Via: "z80"}, {FuncName: "arith__step", Args: []int64{4294967295}, Expected: 0, Via: "z80"}}
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			m := parse()
			m.Asserts = append([]hir.Assert(nil), cases...)
			steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "z80", ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if err != nil {
				t.Fatal(err)
			}
			for _, ir := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
				for _, a := range cases {
					args := make([]mir2.Value, len(a.Args))
					for i, v := range a.Args {
						args[i] = mir2.Value{I: v}
					}
					got, err := mir2.NewVM(ir).Call(a.FuncName, args)
					if err != nil || len(got) != 1 || got[0].I != a.Expected {
						t.Fatalf("%s got=%v err=%v want=%d", a.FuncName, got, err, a.Expected)
					}
				}
			}
		})
	}
}
