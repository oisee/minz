package c89_test

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

// C90 6.3.2.2/6.6.6.4: forward aggregate values across nested calls, mutate
// only the callee's by-value copy and execute the factory effect once.
func TestC89AggregateForwardingVMZ80(t *testing.T) {
	src, err := os.ReadFile("testdata/aggregate_forwarding/forward.c")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *hir.Module {
		m, err := c89.CompileWithOpts(string(src), "forward.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := hir.Verify(m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	inputs := []int64{0, 1, 2, 10, 127, 128, 253, 254, 255}
	want := func(name string, x int64) int64 {
		if name == "count_check" {
			return 1
		}
		return int64(uint16(byte(x+2))*256 + uint16(byte(x+1)) + uint16(byte(x)))
	}
	names := []string{"direct", "copied", "count_check"}
	check := func(m *mir2.Module) {
		// These expression-level/assignment call shapes currently retain the
		// memory ABI. Prove its ownership separately from numerical results.
		for _, name := range []string{"make_pair", "forward", "transform"} {
			f := m.FuncByName(name)
			if f == nil || len(f.Contract.StructReturnBuffers) != 1 {
				t.Fatal("forwarded result lacks caller-owned memory", name)
			}
		}
		vm := mir2.NewVM(m)
		for _, name := range names {
			for _, x := range inputs {
				v, err := vm.Call(name, []mir2.Value{{I: x}})
				if err != nil || len(v) != 1 || v[0].I != want(name, x) {
					t.Fatalf("%s(%d)=%v err=%v want=%d", name, x, v, err, want(name, x))
				}
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
			for _, name := range names {
				for _, x := range inputs {
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: name, Args: []int64{x}, Expected: want(name, x), Via: "z80", Source: name})
				}
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if err != nil {
				t.Fatal(err)
			}
			check(steps.MIR2Module)
		})
	}
}
