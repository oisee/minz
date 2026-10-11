package pipeline

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

func TestIndexedQuotientWeightedNative(t *testing.T) {
	asset := os.Getenv("INDEXED_ENRT4")
	if asset == "" {
		t.Skip("set INDEXED_ENRT4")
	}
	source, err := os.ReadFile("../../../examples/nanz/06_pbqp_weighted.nanz")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("original-factor10", func(t *testing.T) { runIndexedWeighted(t, asset, source, 10, "original") })
	variant := []byte(strings.Replace(string(source), "    heavy = heavy + x\n", "", 1))
	t.Run("same-shape-factor9", func(t *testing.T) { runIndexedWeighted(t, asset, variant, 9, "factor9") })
}
func runIndexedWeighted(t *testing.T, asset string, source []byte, baseFactor int, label string) {
	// Exact corpus helper plus an explicit harness that keeps x live over CALL.
	text := string(source) + "\nfun weighted_caller(x: u8) -> u8 { return weighted(x) + x }\n"
	opts := Options{ContractOpt: true, OptSize: true, AssertMode: "none"}
	baseline, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
	if err != nil {
		t.Fatal(err)
	}
	table := &vir.RegAllocTable{}
	table.Init()
	problem, err := vir.PrepareLocalTableProblem(baseline.MIR2Module.FuncByName("weighted"), baseline.Allocation)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(problem.Ops)
	notes, err := vir.ImportIndexedENRTQuotient(asset, problem, table)
	if err != nil || notes.Import == nil || notes.Import.Admitted != 1 {
		t.Fatal(err, notes)
	}
	afterOps, _ := json.Marshal(problem.Ops)
	if !bytes.Equal(original, afterOps) {
		t.Fatal("original operations changed")
	}
	nv := 0
	for _, g := range notes.QuotientGroups {
		nv += len(g)
	}
	if notes.RecordIndex != 5 || notes.Shape.NVregs != 2 {
		t.Fatal("lost historical shape reuse")
	}
	t.Logf("%d original vregs → %d historical variables, record%d prefix%dB", nv, notes.Shape.NVregs, notes.RecordIndex, notes.BytesRead)
	path := t.TempDir() + "/real.json"
	if err := table.Save(path); err != nil {
		t.Fatal(err)
	}
	opts.CheckedLocalTablePath = path
	selected, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["weighted"].Backend != "checked-local-table" || selected.Traces["weighted_caller"].Backend != "PBQP" {
		t.Fatal("wrong leaf/caller selection", notes)
	}
	before, err := checkedAsmSize(baseline.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	after, err := checkedAsmSize(selected.Assembly)
	if err != nil || after >= before {
		t.Fatal("no measured byte benefit", before, after, err)
	}
	vm := mir2.NewVM(selected.MIR2Module)
	rawVM := mir2.NewVM(hir.LowerModule(graceMeasureParse(t, text)))
	var helperBefore, helperAfter, callerBefore, callerAfter int
	for x := 0; x < 256; x++ {
		for _, name := range []string{"weighted", "weighted_caller"} {
			factor := baseFactor
			if name == "weighted_caller" {
				factor = baseFactor + 1
			}
			want := byte(factor*x + 1)
			raw, rawErr := rawVM.Call(name, []mir2.Value{{I: int64(x)}})
			if rawErr != nil || len(raw) != 1 || byte(raw[0].I) != want {
				t.Fatal("raw source VM oracle", name, x, raw, rawErr)
			}
			values, err := vm.Call(name, []mir2.Value{{I: int64(x)}})
			if err != nil || len(values) != 1 || byte(values[0].I) != want {
				t.Fatal("VM oracle", name, x, values, err)
			}
			b := runCheckedRealByte(t, baseline.Assembly, baseline.MIR2Module.FuncByName(name), baseline.Allocation, byte(x), want)
			a := runCheckedRealByte(t, selected.Assembly, selected.MIR2Module.FuncByName(name), selected.Allocation, byte(x), want)
			if name == "weighted" {
				helperBefore, helperAfter = b, a
			} else {
				callerBefore, callerAfter = b, a
			}
		}
	}
	if helperAfter >= helperBefore || callerAfter >= callerBefore {
		t.Fatal("no native timing benefit", helperBefore, helperAfter, callerBefore, callerAfter)
	}
	t.Logf("weighted exact corpus + caller harness all 256 inputs: module %d→%d B helper %d→%d T caller %d→%d T; actual ABI/SP verified; generation %v", before, after, helperBefore, helperAfter, callerBefore, callerAfter, notes)
	if dir := os.Getenv("VIR_REAL_TABLE_EXPORT"); dir != "" {
		dir = dir + "/" + label
		os.MkdirAll(dir, 0755)
		ev, _ := json.MarshalIndent(notes, "", "  ")
		os.WriteFile(dir+"/quotient.json", ev, 0644)
		for name, steps := range map[string]Steps{"baseline": baseline, "selected": selected} {
			bin, _ := z80asm.NewAssembler().AssembleString(steps.Assembly)
			os.WriteFile(dir+"/"+name+".bin", bin.Binary, 0644)
		}
		for name, data := range map[string][]byte{"corpus.nanz": source, "caller-harness.nanz": []byte(text), "caller-baseline.a80": []byte(baseline.Assembly), "caller-selected.a80": []byte(selected.Assembly)} {
			if err := os.WriteFile(dir+"/"+name, data, 0644); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/caller-table.json", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
