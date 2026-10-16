package c89_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestC89AggregateLinkOriginAndIdempotence(t *testing.T) {
	src := `struct Pair { unsigned char a, b; };
struct Pair marked(unsigned char x) { struct Pair p = {0, 0}; p.a=x; return p; }
struct Pair unrelated(unsigned char x) { struct Pair p = {0, 0}; p.a=x; return p; }`
	m, err := c89.CompileWithOpts(src, "origin.c", c89.CompileOpts{DeferAggregateABI: true})
	if err != nil {
		t.Fatal(err)
	}
	// Same lowered shape, but only the C-origin function is eligible for the
	// C pointer-return convention. Other frontends retain their own ABI.
	m.FuncByName("unrelated").CValueABI = false
	hir.OptimizeCAggregateABI(m)
	if len(m.FuncByName("marked").RetTys) != 2 || len(m.FuncByName("unrelated").RetTys) != 0 || m.FuncByName("unrelated").RetTy != mir2.TyPtr {
		t.Fatal("aggregate eligibility ignored source ABI origin")
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	before := m.Dump()
	hir.OptimizeCAggregateABI(m)
	if m.Dump() != before {
		t.Fatal("repeated pipeline admission changed the selected ABI")
	}
}

// Imported prototypes retain the C value ABI until all calls are visible.
// Optimization must then decompose both input and return at the whole-module
// boundary, including a Nanz caller. This is not merely a memory fallback test.
func TestC89AggregateImportVMZ80(t *testing.T) {
	dir, err := filepath.Abs("testdata/aggregate_import")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := os.ReadFile(filepath.Join(dir, "entry.c"))
	if err != nil {
		t.Fatal(err)
	}
	inputs := []int64{0, 1, 2, 127, 128, 253, 254, 255}
	want := func(x int64) int64 { return int64(uint16(byte(x))*256 + uint16(byte(x+1))) }
	for _, frontend := range []string{"c", "nanz"} {
		t.Run(frontend, func(t *testing.T) {
			parse := func() *hir.Module {
				var m *hir.Module
				var err error
				if frontend == "c" {
					path := filepath.Join(dir, "entry.c")
					m, err = c89.CompileWithOpts(string(entry), path, c89.CompileOpts{BaseDir: dir, Resolver: nanz.CSourceResolver(path, nanz.ParseOpts{})})
				} else {
					m, err = nanz.ParseWithOpts("import library { own }\nfun check(x:u8)->u16{return own(x)}", filepath.Join(dir, "entry.nanz"), nanz.ParseOpts{BaseDir: dir})
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = hir.Verify(m); err != nil {
					t.Fatal(err)
				}
				if frontend == "nanz" {
					maker, consumer := m.FuncByName("library__make_pair"), m.FuncByName("library__consume")
					if maker == nil || maker.RetTy != mir2.TyPtr || consumer == nil || len(consumer.Params) != 1 {
						t.Fatal("library ABI changed before caller source was parsed")
					}
				}
				return m
			}
			raw := hir.LowerModule(parse())
			if err := mir2.Verify(raw); err != nil {
				t.Fatal(err)
			}
			check := func(vm *mir2.VM) {
				t.Helper()
				for _, x := range inputs {
					v, err := vm.Call("check", []mir2.Value{{I: x}})
					if err != nil || len(v) != 1 || v[0].I != want(x) {
						t.Fatalf("check(%d)=%v err=%v want=%d", x, v, err, want(x))
					}
				}
			}
			check(mir2.NewVM(raw))
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(mode, func(t *testing.T) {
					m := parse()
					for _, x := range inputs {
						m.Asserts = append(m.Asserts, hir.Assert{FuncName: "check", Args: []int64{x}, Expected: want(x), Via: "z80"})
					}
					steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
					if err != nil {
						t.Fatal(err)
					}
					prefix := ""
					if frontend == "nanz" {
						prefix = "library__"
					}
					maker, consumer := steps.MIR2Module.FuncByName(prefix+"make_pair"), steps.MIR2Module.FuncByName(prefix+"consume")
					if maker == nil || len(maker.Contract.Returns) != 2 || consumer == nil || len(consumer.Contract.Params) != 2 {
						t.Fatal("linked aggregate chain not actually decomposed")
					}
					check(mir2.NewVM(steps.MIR2Module))
				})
			}
		})
	}
}
