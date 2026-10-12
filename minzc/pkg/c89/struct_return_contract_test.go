package c89

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
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
		h, err := CompileWithOpts(string(source), "c99_designated.c", CompileOpts{})
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
