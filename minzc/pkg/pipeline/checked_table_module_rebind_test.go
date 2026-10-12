package pipeline

import (
	"sort"
	"testing"

	"github.com/minz/minzc/pkg/vir"
)

// A register assignment can be proposed again, but its old certificate binds
// the exact MIR function and must not authorize a different module identity.
func TestCheckedTableModuleRelocationRequiresReadmission(t *testing.T) {
	isolated, path := localTableFixture(t)
	joined, err := CompileHIRSteps(graceMeasureParse(t, jointSource+checkedTableSource), Options{OptSize: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := vir.PrepareLocalTableProblem(isolated.MIR2Module.FuncByName("add5"), isolated.Allocation)
	if err != nil {
		t.Fatal(err)
	}
	current, err := vir.PrepareLocalTableProblem(joined.MIR2Module.FuncByName("add5"), joined.Allocation)
	if err != nil {
		t.Fatal(err)
	}
	oldOptions, currentOptions := old.Options, current.Options
	oldOptions.CheckedContext, currentOptions.CheckedContext = "", ""
	a, err := vir.CheckedTableKey(old.Ops, vir.Z80, oldOptions)
	if err != nil {
		t.Fatal(err)
	}
	b, err := vir.CheckedTableKey(current.Ops, vir.Z80, currentOptions)
	if err != nil || a != b {
		t.Fatalf("different canonical problem: %s / %s: %v", a, b, err)
	}
	if old.Options.CheckedContext == current.Options.CheckedContext {
		t.Fatal("exact binding did not change")
	}
	var table vir.RegAllocTable
	table.Init()
	if err := table.Load(path); err != nil {
		t.Fatal(err)
	}
	_, oldCost, ok := table.LookupChecked(old.Ops, vir.Z80, old.Options)
	if !ok {
		t.Fatal("original certificate missing")
	}
	if _, err := current.Emit(&table); err == nil {
		t.Fatal("original certificate reused without readmission")
	}
	ids := func(p *vir.LocalTableProblem) []int {
		set := map[int]bool{}
		for _, op := range p.Ops {
			for _, v := range []int{op.Dst, op.Src[0], op.Src[1]} {
				if v > 0 {
					set[v] = true
				}
			}
		}
		var out []int
		for v := range set {
			out = append(out, v)
		}
		sort.Ints(out)
		return out
	}
	before, after := ids(old), ids(current)
	if len(before) != len(after) || before[0] == after[0] {
		t.Fatal("fixture did not relocate vregs", before, after)
	}
	var readmitted vir.RegAllocTable
	readmitted.Init()
	if _, err := table.ReadmitCheckedCandidates(current, &readmitted, 0); err == nil {
		t.Fatal("invalid bound admitted")
	}
	count, err := table.ReadmitCheckedCandidates(current, &readmitted, 1)
	if err != nil || count != 1 {
		t.Fatal("readmission failed", count, err)
	}
	_, newCost, ok := readmitted.LookupChecked(current.Ops, vir.Z80, current.Options)
	if !ok || newCost != oldCost {
		t.Fatal("readmission changed cost", oldCost, newCost, ok)
	}
	if err := readmitted.Save(path); err != nil {
		t.Fatal(err)
	}
	selected, err := CompileHIRSteps(graceMeasureParse(t, jointSource+checkedTableSource), Options{OptSize: true, AssertMode: "none", CheckedLocalTablePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("readmitted candidate not selected")
	}
	for x := 0; x < 256; x++ {
		executeLocalTableFunction(t, selected.Assembly, byte(x))
	}
	t.Logf("vregs %v -> %v; readmitted cost %d; native all 256 inputs PASS", before, after, newCost)
}
