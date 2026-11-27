package pipeline

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// TestUnrollGeneratedVMOracle compiles every C file of $MINZ_UNROLL_ORACLE_DIR
// with unrolling ungated (every admitted loop unrolled) and checks each
// two-byte-argument function on the Z80 against the raw MIR VM of the
// ordinary (not unrolled) lowering. Generated corpora are not checked in;
// the test is skipped without the variable.
func TestUnrollGeneratedVMOracle(t *testing.T) {
	dir := os.Getenv("MINZ_UNROLL_ORACLE_DIR")
	if dir == "" {
		t.Skip("MINZ_UNROLL_ORACLE_DIR not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.c"))
	sort.Strings(files)
	args := [][]int64{{0, 0}, {1, 2}, {28, 166}, {216, 103}, {255, 255}, {137, 50}, {90, 3}}
	funcs, unrolledFuncs, wrong := 0, 0, 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parse := func() *hir.Module {
			h, err := c89.CompileWithOpts(string(src), path, c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			return h
		}
		oracle := hir.LowerModuleWithOptions(parse(), hir.LowerOptions{})
		h := parse()
		o := DefaultOptions()
		o.AssertMode, o.unrollUngated = "none", true
		s, err := CompileHIRSteps(h, o)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		for _, f := range oracle.Funcs {
			if len(f.Contract.Params) != 2 || strings.Contains(f.Name, "$") {
				continue
			}
			funcs++
			if s.unrolled[f.Name] > 0 {
				unrolledFuncs++
			}
			for _, a := range args {
				want, err := mir2.NewVM(oracle).Call(f.Name, []mir2.Value{{I: a[0]}, {I: a[1]}})
				if err != nil || len(want) != 1 {
					t.Fatalf("%s %s oracle: %v", path, f.Name, err)
				}
				_, got, err := RunZ80Cycles(h, s.MIR2Module, s.Allocation, s.Assembly, f.Name, a)
				if err != nil || got&255 != want[0].I&255 {
					wrong++
					t.Errorf("%s %s%v: Z80 %d %v want %d", filepath.Base(path), f.Name, a, got&255, err, want[0].I&255)
					break
				}
			}
		}
	}
	t.Logf("%d functions, %d with unrolled loops, %d wrong", funcs, unrolledFuncs, wrong)
}
