package c89_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

// ISO C90 6.3.2.2/6.6.6.4: returning an aggregate yields a value, not a
// pointer into an expired activation. Direct use and local copying must agree.
func TestC89AggregateReturnLifetimeVMZ80(t *testing.T) {
	for _, family := range []string{"large_return", "indirect_return"} {
		t.Run(family, func(t *testing.T) {
			src, e := os.ReadFile(filepath.Join("testdata/aggregate_lifetime", family+".c"))
			if e != nil {
				t.Fatal(e)
			}
			parse := func() *hir.Module {
				m, e := c89.CompileWithOpts(string(src), family+".c", c89.CompileOpts{NoLibc: true})
				if e != nil {
					t.Fatal(e)
				}
				if e = hir.Verify(m); e != nil {
					t.Fatal(e)
				}
				return m
			}
			want := func(x int64) int64 {
				if family == "large_return" {
					return 9*x + 13
				}
				return int64(uint16(byte(x))*256 + uint16(byte(x+1)))
			}
			inputs := []int64{0, 1, 2, 127, 128, 253, 254, 255}
			check := func(t *testing.T, mm *mir2.Module) {
				t.Helper()
				if e := mir2.Verify(mm); e != nil {
					t.Fatal(e)
				}
				maker := "make_trio"
				if family == "indirect_return" {
					maker = "make_pair"
				}
				if f := mm.FuncByName(maker); f == nil || len(f.Contract.StructReturnBuffers) != 1 {
					t.Fatal("memory aggregate return lacks caller-owned result buffer")
				}
				for _, f := range []string{"direct", "copied"} {
					for _, x := range inputs {
						v, e := mir2.NewVM(mm).Call(f, []mir2.Value{{I: x}})
						if e != nil || len(v) != 1 || v[0].I != want(x) {
							t.Fatalf("%s(%d)=%v err=%v want=%d", f, x, v, e, want(x))
						}
					}
				}
			}
			t.Run("raw", func(t *testing.T) { check(t, hir.LowerModule(parse())) })
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(mode, func(t *testing.T) {
					m := parse()
					for _, f := range []string{"direct", "copied"} {
						for _, x := range inputs {
							m.Asserts = append(m.Asserts, hir.Assert{FuncName: f, Args: []int64{x}, Expected: want(x), Via: "z80", Source: fmt.Sprintf("%s(%d)", f, x)})
						}
					}
					steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
					if e != nil {
						t.Fatal(e)
					}
					check(t, steps.MIR2Module)
				})
			}
		})
	}
}
