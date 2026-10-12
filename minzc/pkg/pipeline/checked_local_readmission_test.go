package pipeline

import (
	"github.com/minz/minzc/pkg/vir"
	"testing"
)

func TestModuleTableReadmissionWithoutSolver(t *testing.T) {
	_, path := localTableFixture(t)
	var source vir.RegAllocTable
	source.Init()
	if err := source.Load(path); err != nil {
		t.Fatal(err)
	}
	text := jointSource + checkedTableSource
	options := Options{OptSize: true, AssertMode: "none"}
	table, notes, err := ReadmitCheckedLocalTables(graceMeasureParse(t, text), options, &source, 1)
	if err != nil || table.Size() == 0 {
		t.Fatal(notes, err)
	}
	if err := table.Save(path); err != nil {
		t.Fatal(err)
	}
	options.CheckedLocalTablePath = path
	selected, err := CompileHIRSteps(graceMeasureParse(t, text), options)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("readmitted leaf not selected")
	}
	for x := 0; x < 256; x++ {
		executeLocalTableFunction(t, selected.Assembly, byte(x))
	}
	for _, bad := range []Options{{}, {OptSize: true, SkipZ80Emission: true}, {OptSize: true, CheckedLocalTablePath: path}, {OptSize: true, CheckedLocalTableSHA256: "abc"}, {OptSize: true, Backend: "llvm"}} {
		if out, _, err := ReadmitCheckedLocalTables(graceMeasureParse(t, text), bad, &source, 1); err == nil || out != nil {
			t.Fatal("invalid mode accepted", bad)
		}
	}
	t.Log(notes)
}
