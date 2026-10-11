package vir

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestLegacyCandidateImportRecomputesAndRefusesABI(t *testing.T) {
	f, alloc := productionTableFunction(mir2.ClassAcc)
	p, e := PrepareLocalTableProblem(f, alloc)
	if e != nil {
		t.Fatal(e)
	}
	assignment := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = Z80.LocByName("A")
		}
	}
	// A supplied cost is never accepted as the checked current cost.
	data, _ := json.Marshal([]RegAllocEntry{{Signature: "old", Cost: -999, Assignment: []int{0, 0}}})
	table := &RegAllocTable{}
	table.Init()
	ev, e := ImportLegacyCandidates(data, "legacy-native-json", p.Ops, Z80, p.Options, table)
	if e != nil || ev.Admitted != 1 {
		t.Fatal(e, ev)
	}
	_, cost, hit := table.LookupChecked(p.Ops, Z80, p.Options)
	if !hit || cost < 0 {
		t.Fatal("claimed cost trusted", cost)
	}
	f.Contract.Returns[0].Class = mir2.ClassRegC
	q, e := PrepareLocalTableProblem(f, alloc)
	if e != nil {
		t.Fatal(e)
	}
	other := &RegAllocTable{}
	other.Init()
	ev, e = ImportLegacyCandidates(data, "legacy-native-json", q.Ops, Z80, q.Options, other)
	if e != nil || ev.Admitted != 0 || other.Size() != 0 {
		t.Fatal("ABI mismatch admitted", e, ev)
	}
	for _, format := range []string{"unknown"} {
		if _, e = ImportLegacyCandidates(data, format, p.Ops, Z80, p.Options, table); e == nil {
			t.Fatal("badformat")
		}
	}
	bad, _ := json.Marshal([]RegAllocEntry{{Signature: "old", Assignment: []int{0, 0}}, {Signature: "forged", Schema: "vir-local-direct-v1", Assignment: []int{0, 0}}})
	empty := &RegAllocTable{}
	empty.Init()
	if _, e = ImportLegacyCandidates(bad, "legacy-native-json", p.Ops, Z80, p.Options, empty); e == nil || empty.Size() != 0 {
		t.Fatal("partial schema relabel")
	}
	if _, e = ImportLegacyCandidatesWithRemap(data, "legacy-native-json", p.Ops, Z80, p.Options, empty, 7); e == nil {
		t.Fatal("unboundedremap")
	}
}
