package c89_test

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

// The nil-initializer scalarization changes this existing example's assembly.
// Check its results on native Z80 as well as VM. Its C99/C11 constructs are
// regression coverage for extensions, not evidence of ISO C89 coverage.
func TestCAggregateExampleStorageNativeVM(t *testing.T) {
	name := "../../../examples/c/c99_structs.c"
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	// Expectations come from the literal field values and arithmetic in the
	// source; they are independent of its MIR and register assignments.
	specs := []struct {
		name string
		want int64
	}{
		{"point_x", 10}, {"point_y", 20}, {"color_green", 20},
		{"compound_point", 42}, {"manhattan", (15 - 10) + (30 - 20)},
		{"entity_x", 42}, {"entity_id", 7}, {"vec3_z", 30},
		{"vec3_sum", 10 + 20 + 30}, {"test_return_struct", 42},
	}
	parse := func() *hir.Module {
		m, err := c89.Compile(string(src), name)
		if err != nil {
			t.Fatal(err)
		}
		if err := hir.Verify(m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	check := func(m *mir2.Module) {
		vm := mir2.NewVM(m)
		for _, s := range specs {
			v, err := vm.Call(s.name, nil)
			if err != nil || len(v) != 1 || v[0].I != s.want {
				t.Fatalf("%s: result=%v err=%v want=%d", s.name, v, err, s.want)
			}
		}
	}
	raw := hir.LowerModule(parse())
	if err := mir2.Verify(raw); err != nil {
		t.Fatal(err)
	}
	check(raw)
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			m := parse()
			for _, s := range specs {
				m.Asserts = append(m.Asserts, hir.Assert{FuncName: s.name, Expected: s.want, Via: "z80", Source: s.name})
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if err != nil {
				t.Fatal(err)
			}
			check(steps.MIR2Module)
		})
	}
}
