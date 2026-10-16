package c89_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func TestC89GlobalRelocationImportVMZ80(t *testing.T) {
	dir, e := filepath.Abs("testdata/global_relocations")
	if e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile(filepath.Join(dir, "entry.c"))
	if e != nil {
		t.Fatal(e)
	}
	for _, front := range []string{"c", "nanz"} {
		t.Run(front, func(t *testing.T) {
			parse := func(t *testing.T) *hir.Module {
				t.Helper()
				var m *hir.Module
				var e error
				if front == "c" {
					path := filepath.Join(dir, "entry.c")
					m, e = c89.CompileWithOpts(string(source), path, c89.CompileOpts{BaseDir: dir, Resolver: nanz.CSourceResolver(path, nanz.ParseOpts{})})
				} else {
					m, e = nanz.ParseWithOpts(`fun seed()->u8 { let s:^u8="yz"
 return s[0] }
 import library { library }
 fun check(x:u8)->u16 { let p:^u8=c"yz"
 return library(x)+(p[x%3] as u16)+(seed() as u16) }`, filepath.Join(dir, "entry.nanz"), nanz.ParseOpts{BaseDir: dir})
				}
				if e != nil {
					t.Fatal(e)
				}
				if e = hir.Verify(m); e != nil {
					t.Fatal(e)
				}
				return m
			}
			inputs := []int64{0, 1, 2, 3, 5, 6, 7, 127, 128, 254, 255}
			model := func(x int64) int64 {
				v := int64([]byte{5, 7, 11}[x%3]) + 2*int64("yz\x00"[x%3]) + x + 17
				if front == "nanz" {
					v += 2
				}
				return v
			}
			check := func(t *testing.T, mm *mir2.Module) {
				t.Helper()
				if e := mir2.Verify(mm); e != nil {
					t.Fatal(e)
				}
				vm := mir2.NewVM(mm)
				for _, x := range inputs {
					v, e := vm.Call("check", []mir2.Value{{I: x}})
					if e != nil || len(v) != 1 || v[0].I != model(x) {
						t.Fatalf("VM check(%d)=%v err=%v want=%d", x, v, e, model(x))
					}
				}
			}
			t.Run("raw", func(t *testing.T) { check(t, hir.LowerModule(parse(t))) })
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(mode, func(t *testing.T) {
					m := parse(t)
					for _, x := range inputs {
						m.Asserts = append(m.Asserts, hir.Assert{FuncName: "check", Args: []int64{x}, Expected: model(x), Via: "z80", Source: fmt.Sprintf("check(%d)", x)})
					}
					s, e := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
					if e != nil {
						t.Fatal(e)
					}
					check(t, s.MIR2Module)
				})
			}
		})
	}
}
