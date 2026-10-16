package c89_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

// ISO C90 6.7.1 old-style definitions, 6.3.3.4 sizeof (no operand evaluation),
// and 6.8 conditional inclusion, stringification and token concatenation.
func TestC89LanguageAdmissionVMZ80(t *testing.T) {
	src, e := os.ReadFile(filepath.Join("testdata/language_admission", "core.c"))
	if e != nil {
		t.Fatal(e)
	}
	parse := func(t *testing.T) *hir.Module {
		t.Helper()
		m, e := c89.Compile(string(src), "core.c")
		if e != nil {
			t.Fatal(e)
		}
		if e = hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	inputs := []int64{0, 1, 2, 5, 6, 127, 128, 254, 255}
	want := func(f string, x int64) int64 {
		switch f {
		case "knr":
			return 3*x + 71
		case "unevaluated":
			return 7
		case "stringbytes":
			return 2 * int64([]byte{65, 10, 65, 0, 90, 0}[x%6])
		case "preprocessing":
			return 19 + int64("packet"[x%6])
		}
		panic(f)
	}
	check := func(t *testing.T, mm *mir2.Module) {
		t.Helper()
		if e := mir2.Verify(mm); e != nil {
			t.Fatal(e)
		}
		for _, f := range []string{"knr", "unevaluated", "preprocessing", "stringbytes"} {
			for _, x := range inputs {
				v, e := mir2.NewVM(mm).Call(f, []mir2.Value{{I: x}})
				if e != nil || len(v) != 1 || v[0].I != want(f, x) {
					t.Fatalf("%s(%d)=%v err=%v want=%d", f, x, v, e, want(f, x))
				}
			}
		}
	}
	t.Run("raw", func(t *testing.T) { check(t, hir.LowerModule(parse(t))) })
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			m := parse(t)
			for _, f := range []string{"knr", "unevaluated", "preprocessing", "stringbytes"} {
				for _, x := range inputs {
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: f, Args: []int64{x}, Expected: want(f, x), Via: "z80", Source: fmt.Sprintf("%s(%d)", f, x)})
				}
			}
			s, e := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if e != nil {
				t.Fatal(e)
			}
			check(t, s.MIR2Module)
		})
	}
}
