package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

type carryCost struct{ T, Bytes, Calls int }

func carryProbeParse(t *testing.T, file string) *hir.Module {
	t.Helper()
	path := "testdata/o8c/" + file
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var h *hir.Module
	if strings.HasSuffix(file, ".c") {
		h, err = c89.Compile(string(src), path)
	} else {
		h, err = nanz.Parse(string(src), path)
	}
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// Frozen origin/main (2b70eaa74) costs use exactly the same fixture assertions,
// contract options and function-entry-through-RET measurement as the candidate.
func carryCheckCost(t *testing.T, key string, got carryCost) {
	t.Helper()
	data, err := os.ReadFile("testdata/o8c/origin-costs.json")
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]carryCost{}
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatal(err)
	}
	t.Logf("CARRY_COST %s %d %d %d", key, got.T, got.Bytes, got.Calls)
	before, ok := baseline[key]
	if !ok {
		t.Errorf("missing origin cost for %s", key)
		return
	}
	if got.Calls == 0 || got.Calls != before.Calls || got.T > before.T || got.Bytes > before.Bytes {
		t.Errorf("%s: T %d -> %d, module bytes %d -> %d, calls %d -> %d", key, before.T, got.T, before.Bytes, got.Bytes, before.Calls, got.Calls)
	}
}

func carryMeasureProbes(t *testing.T, file string, grace, isolated bool) {
	t.Helper()
	h := carryProbeParse(t, file)
	checks := h.Asserts
	if isolated {
		selected := strings.Split(t.Name(), "/")
		name := selected[len(selected)-1]
		h.Funcs = []*hir.Func{h.FuncByName(name)}
	}
	h.Asserts = nil
	raw := hir.LowerModule(h)
	steps, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: grace})
	if err != nil {
		t.Fatal(err)
	}
	if isolated {
		name := h.Funcs[0].Name
		if name != "hu" && name != "both" && !strings.Contains(steps.Assembly, "    BIT ") {
			t.Errorf("unfusible diamond lost its BIT predicate:\n%s", steps.Assembly)
		}
	}
	costs := map[string]carryCost{}
	for _, a := range checks {
		if a.Via != "z80" || h.FuncByName(a.FuncName) == nil {
			continue
		}
		args := make([]mir2.Value, len(a.Args))
		for i, v := range a.Args {
			args[i] = mir2.Value{I: v}
		}
		for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
			got, err := mir2.NewVM(m).Call(a.FuncName, args)
			// The VM represents signed results as width-masked integers.
			width := m.FuncByName(a.FuncName).Contract.Returns[0].Ty.Width()
			mask := int64((uint64(1) << width) - 1)
			if err != nil || len(got) != 1 || got[0].I&mask != a.Expected&mask {
				t.Fatalf("%s%v: VM %v, err %v, want %d", a.FuncName, a.Args, got, err, a.Expected)
			}
		}
		cycles, size := measureGraceZ80Compiled(t, h, steps, a.FuncName, a.Args, nil, a.Expected, 2000000)
		cost := costs[a.FuncName]
		cost.T += cycles
		cost.Bytes = size
		cost.Calls++
		costs[a.FuncName] = cost
	}
	if len(costs) != len(h.Funcs) {
		t.Fatalf("assertion inventory covers %d of %d functions", len(costs), len(h.Funcs))
	}
	mode := "module"
	if isolated {
		mode = "isolated"
	}
	for _, f := range h.Funcs {
		carryCheckCost(t, fmt.Sprintf("%s/%s/grace%v/%s", mode, file, grace, f.Name), costs[f.Name])
	}
}

// These seven diamonds must retain their original predicate/arm layout when
// carry cannot replace the test. Separate modules also guard each code size.
func TestShiftCarryProfitabilityVMZ80(t *testing.T) {
	for _, grace := range []bool{false, true} {
		for _, name := range []string{"wb", "wb2", "wb3", "hu", "both", "sar", "sh2"} {
			t.Run(fmt.Sprintf("grace%v/%s", grace, name), func(t *testing.T) { carryMeasureProbes(t, "p1.nanz", grace, true) })
		}
	}
}

func TestShiftCarryCriticProbesVMZ80(t *testing.T) {
	for _, file := range []string{"p1.nanz", "p2.nanz", "p3.nanz", "c1.c"} {
		for _, grace := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/grace%v", file, grace), func(t *testing.T) { carryMeasureProbes(t, file, grace, false) })
		}
	}
}

func TestShiftCarryCarryKernelsVMZ80(t *testing.T) {
	for _, name := range []string{"crc8", "mul8", "crc16", "lfsr"} {
		for _, grace := range []bool{false, true} {
			t.Run(fmt.Sprintf("grace%v/%s", grace, name), func(t *testing.T) {
				h := carryProbeParse(t, "p1.nanz")
				h.Funcs = []*hir.Func{h.FuncByName(name)}
				h.Asserts = nil
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: grace})
				if err != nil {
					t.Fatal(err)
				}
				asm := s.Assembly
				if !strings.Contains(asm, "JRS C,") && !strings.Contains(asm, "JRS NC,") && !strings.Contains(asm, "RET C") && !strings.Contains(asm, "RET NC") {
					t.Errorf("no carry branch:\n%s", asm)
				}
				if strings.Contains(asm, "    BIT ") {
					t.Errorf("separate bit predicate remains:\n%s", asm)
				}
			})
		}
	}
}
