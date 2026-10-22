package c89_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"testing"
)

// C89 3.6.4.2 switch enters the selected label and then follows statement
// order; 3.6.6.2/3.6.6.3 continue/break target the nearest applicable construct.
// All arithmetic here is unsigned and fits the target 16-bit result.
func TestC89ControlMatrixVMZ80(t *testing.T) {
	src, err := os.ReadFile("testdata/control_matrix/switch.c")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *hir.Module {
		m, err := c89.CompileWithOpts(string(src), "switch.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = hir.Verify(m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	cases := []struct {
		name string
		want func(int64) int64
	}{
		{"join_unused", func(x int64) int64 {
			if x&1 != 0 {
				return 1
			}
			if x == 0 {
				return 2
			}
			return 4
		}},
		{"fallthrough", func(x int64) int64 { return []int64{3, 2, 12, 8}[x&3] }},
		{"loop_in_switch", func(x int64) int64 {
			if x&1 == 0 {
				return 12
			}
			return 20
		}},
		{"goto_scan", func(x int64) int64 { return x * (x - 1) / 2 }},
		{"default_middle", func(x int64) int64 {
			switch x {
			case 0:
				return 7
			case 1:
				return 4
			case 2:
				return 8
			}
			return 6
		}},
		{"chained_labels", func(x int64) int64 {
			if x < 2 {
				return 3
			}
			return 7
		}},
		{"nested_switch", func(x int64) int64 {
			if x&3 != 0 {
				return 30
			}
			if x&4 == 0 {
				return 11
			}
			return 12
		}},
		{"switch_continue", func(x int64) int64 {
			var v int64
			for i := int64(0); i < 6; i++ {
				switch (i + x) & 3 {
				case 1:
					continue
				case 2:
					v += 2
				default:
					v += 3
				}
				v += 10
			}
			return v
		}},
		{"selector_once", func(x int64) int64 {
			v := int64(3)
			if x < 2 {
				v = x + 1
			}
			return v + int64(byte(x+1))*16
		}},
		{"byte_case", func(x int64) int64 {
			if x == 255 {
				return 1
			}
			return 3
		}},
	}
	inputs := []int64{0, 1, 2, 3, 4, 5, 7, 31, 254, 255}
	check := func(vm *mir2.VM) {
		t.Helper()
		for _, c := range cases {
			for _, x := range inputs {
				got, err := vm.Call(c.name, []mir2.Value{{I: x}})
				if err != nil || len(got) != 1 || got[0].I != c.want(x) {
					t.Fatalf("%s(%d)=%v err=%v want=%d", c.name, x, got, err, c.want(x))
				}
			}
		}
	}
	raw := hir.LowerModule(parse())
	if err := mir2.Verify(raw); err != nil {
		t.Fatal(err)
	}
	check(mir2.NewVM(raw))
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			m := parse()
			for _, c := range cases {
				for _, x := range inputs {
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: c.name, Args: []int64{x}, Expected: c.want(x), Via: "z80", Source: fmt.Sprintf("%s(%d)", c.name, x)})
				}
			}
			s, err := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if err != nil {
				t.Fatal(err)
			}
			check(mir2.NewVM(s.MIR2Module))
		})
	}
}
