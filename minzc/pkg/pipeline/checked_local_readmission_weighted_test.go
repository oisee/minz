package pipeline

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
)

func TestSavedWeightedTableReadmissionNativeCaller(t *testing.T) {
	testSavedWeightedReadmission(t, Options{ContractOpt: true, OptSize: true, AssertMode: "none"})
}

func TestSavedWeightedReadmissionContractGraceMatrix(t *testing.T) {
	for _, strategy := range []string{"off", "greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			name := strategy + "/plain"
			if grace {
				name = strategy + "/grace"
			}
			t.Run(name, func(t *testing.T) {
				opts := Options{ContractOpt: strategy != "off", OptSize: true, AssertMode: "none", UseGrace: grace, UseHIRGrace: grace}
				if strategy == "pbqp" {
					opts.ContractStrategy = "pbqp"
				}
				testSavedWeightedReadmission(t, opts)
			})
		}
	}
}

func testSavedWeightedReadmission(t *testing.T, opts Options) {
	t.Helper()
	src, err := os.ReadFile("../../../examples/nanz/06_pbqp_weighted.nanz")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src) + "\nfun weighted_caller(x:u8)->u8{return weighted(x)+x}\n"
	var saved vir.RegAllocTable
	saved.Init()
	if err := saved.Load("../../../docs/grace-real-leaf-local-tables/caller-table.json"); err != nil {
		t.Fatal(err)
	}
	baseline, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
	if err != nil {
		t.Fatal(err)
	}
	table, notes, err := ReadmitCheckedLocalTables(graceMeasureParse(t, text), opts, &saved, 1)
	if err != nil || table.Size() == 0 {
		t.Fatal(notes, err)
	}
	path := t.TempDir() + "/current.json"
	if err := table.Save(path); err != nil {
		t.Fatal(err)
	}
	opts.CheckedLocalTablePath = path
	selected, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["weighted"].Backend != "checked-local-table" {
		t.Fatal(notes)
	}
	before, err := checkedAsmSize(baseline.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	after, err := checkedAsmSize(selected.Assembly)
	if err != nil || after >= before {
		t.Fatal(before, after, err)
	}
	vm := mir2.NewVM(selected.MIR2Module)
	var beforeT, afterT int
	for x := 0; x < 256; x++ {
		want := byte(11*x + 1)
		values, err := vm.Call("weighted_caller", []mir2.Value{{I: int64(x)}})
		if err != nil || len(values) != 1 || values[0].I != int64(want) {
			t.Fatal(x, values, err)
		}
		beforeT += runCheckedRealByte(t, baseline.Assembly, baseline.MIR2Module.FuncByName("weighted_caller"), baseline.Allocation, byte(x), want)
		afterT += runCheckedRealByte(t, selected.Assembly, selected.MIR2Module.FuncByName("weighted_caller"), selected.Allocation, byte(x), want)
	}
	if afterT >= beforeT {
		t.Fatal("caller gain lost", beforeT, afterT)
	}
	t.Logf("saved proposal without solver: module %d -> %d B; all256caller %d -> %d T; %v", before, after, beforeT, afterT, notes)
}
