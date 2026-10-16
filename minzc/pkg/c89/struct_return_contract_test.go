package c89_test

import (
	c89 "github.com/minz/minzc/pkg/c89"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// Designated initialization assigns named fields independently of their order.
// The expectations are the literal field values in the unchanged C source.
func TestDesignatedStructReturnContractVMZ80(t *testing.T) {
	source, err := os.ReadFile("../../../examples/c89/c99_designated.c")
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]int64{
		"desig_basic_a": 10, "desig_basic_b": 20,
		"desig_rev_a": 3, "desig_rev_b": 7,
		"desig_mixed_a": 5, "desig_mixed_b": 15,
		"desig_triple_x": 1, "desig_triple_y": 50, "desig_triple_z": 100,
	}
	names := make([]string, 0, len(wants))
	for name := range wants {
		names = append(names, name)
	}
	sort.Strings(names)
	parse := func() *hir.Module {
		h, err := c89.CompileWithOpts(string(source), "c99_designated.c", c89.CompileOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if err := hir.Verify(h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	check := func(vm *mir2.VM) {
		for _, name := range names {
			want := wants[name]
			got, err := vm.Call(name, nil)
			if err != nil || len(got) != 1 || got[0].I != want {
				t.Fatalf("%s: got %v, want %d, err %v", name, got, want, err)
			}
		}
	}
	check(mir2.NewVM(hir.LowerModule(parse())))
	for mode, opts := range []pipeline.Options{
		{ContractOpt: true},
		{ContractOpt: true, UseHIRGrace: true, UseGrace: true},
		{ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true},
	} {
		h := parse()
		for _, name := range names {
			want := wants[name]
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: name, Expected: want, Via: "z80"})
		}
		steps, err := pipeline.CompileHIRSteps(h, opts)
		if err != nil {
			t.Fatal(err)
		}
		check(mir2.NewVM(steps.MIR2Module))
		if dir := os.Getenv("MINZ_STRUCT_RETURN_EVIDENCE"); dir != "" {
			dir = filepath.Join(dir, []string{"ordinary", "speed", "size"}[mode])
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{"source.c": source, "final.a80": []byte(steps.Assembly), "final.mir2": []byte(steps.MIR2Opt), "input.hir": []byte(steps.HIR)} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

// Both lexical branches of the original function must return the ordered pair.
func TestMinmaxNestedStructReturnVMZ80(t *testing.T) {
	source, err := os.ReadFile("../../../examples/c89/minmax_pair.c")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *hir.Module {
		h, err := c89.Compile(string(source), "minmax_pair.c")
		if err != nil {
			t.Fatal(err)
		}
		if err := hir.Verify(h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	check := func(vm *mir2.VM) {
		for a := int64(0); a < 256; a++ {
			for b := int64(0); b < 256; b++ {
				lo, hi := a, b
				if a > b {
					lo, hi = b, a
				}
				for _, tc := range []struct {
					name string
					want int64
				}{{"smaller", lo}, {"larger", hi}} {
					got, err := vm.Call(tc.name, []mir2.Value{{I: a}, {I: b}})
					if err != nil || len(got) != 1 || got[0].I != tc.want {
						t.Fatalf("%s(%d,%d): %v, want %d, err %v", tc.name, a, b, got, tc.want, err)
					}
				}
			}
		}
	}
	check(mir2.NewVM(hir.LowerModule(parse())))
	for _, opts := range []pipeline.Options{{ContractOpt: true}, {ContractOpt: true, UseHIRGrace: true, UseGrace: true}, {ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true}} {
		h := parse()
		for _, a := range []int64{0, 1, 255, 256, 32767, 32768, 65535} {
			for _, b := range []int64{0, 1, 255, 256, 32767, 32768, 65535} {
				lo, hi := a, b
				if a > b {
					lo, hi = b, a
				}
				h.Asserts = append(h.Asserts, hir.Assert{FuncName: "smaller", Args: []int64{a, b}, Expected: lo, Via: "z80"}, hir.Assert{FuncName: "larger", Args: []int64{a, b}, Expected: hi, Via: "z80"})
			}
		}
		steps, err := pipeline.CompileHIRSteps(h, opts)
		if err != nil {
			t.Fatal(err)
		}
		check(mir2.NewVM(steps.MIR2Module))
	}
}

func TestC89ExampleHIRContracts(t *testing.T) {
	paths, err := filepath.Glob("../../../examples/c89/*.c")
	if err != nil || len(paths) == 0 {
		t.Fatalf("C89 examples: %v, %d files", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(err)
			}
			h, err := c89.CompileWithOpts(string(source), abs, c89.CompileOpts{BaseDir: filepath.Dir(abs), IncludePaths: []string{filepath.Dir(abs)}, Resolver: nanz.CSourceResolver(abs, nanz.ParseOpts{})})
			if err != nil {
				t.Fatal(err)
			}
			if err := hir.Verify(h); err != nil {
				t.Fatal(err)
			}
			hir.LowerModule(h)
		})
	}
}
