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

// Literal identity includes encoding. Duplicate CString text across source
// modules must preserve addresses, while SString text stays length-prefixed.
func TestC89StringImportVMZ80(t *testing.T) {
	dir, e := filepath.Abs("testdata/language_admission")
	if e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile(filepath.Join(dir, "string_entry.c"))
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
					path := filepath.Join(dir, "string_entry.c")
					m, e = c89.CompileWithOpts(string(source), path, c89.CompileOpts{BaseDir: dir, Resolver: nanz.CSourceResolver(path, nanz.ParseOpts{})})
				} else {
					m, e = nanz.ParseWithOpts(`fun seed()->u8 { let s:^u8="packet"
 return s[0] }
 import strings { library_string }
 fun check(x:u8)->u16 {
 let a:^u8=c"other"
 let b:^u8=c"packet"
 return library_string(x)+(a[x%6] as u16)+(b[x%7] as u16)+(seed() as u16)
 }`, filepath.Join(dir, "entry.nanz"), nanz.ParseOpts{BaseDir: dir})
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
			want := func(x int64) int64 {
				v := 2*int64("packet\x00"[x%7]) + int64("Z\n\x00"[x%3]) + int64("other\x00"[x%6])
				if front == "nanz" {
					v += 6
				}
				return v
			}
			check := func(t *testing.T, mm *mir2.Module) {
				t.Helper()
				if e := mir2.Verify(mm); e != nil {
					t.Fatal(e)
				}
				for _, x := range inputs {
					v, e := mir2.NewVM(mm).Call("check", []mir2.Value{{I: x}})
					if e != nil || len(v) != 1 || v[0].I != want(x) {
						t.Fatalf("check(%d)=%v err=%v want=%d", x, v, e, want(x))
					}
				}
			}
			t.Run("raw", func(t *testing.T) { check(t, hir.LowerModule(parse(t))) })
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(mode, func(t *testing.T) {
					m := parse(t)
					for _, x := range inputs {
						m.Asserts = append(m.Asserts, hir.Assert{FuncName: "check", Args: []int64{x}, Expected: want(x), Via: "z80", Source: fmt.Sprintf("check(%d)", x)})
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
