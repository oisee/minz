package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestReadmissionRecomputesCostsAndRejectsWrongABI(t *testing.T) {
	f, allocation := productionTableFunction(mir2.ClassAcc)
	p, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	assignment := map[int]int{}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = Z80.LocByName("A")
		}
	}
	var source, destination RegAllocTable
	source.Init()
	destination.Init()
	if err := source.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
		t.Fatal(err)
	}
	// Source JSON costs and identities are provenance, not a current certificate.
	for _, entry := range source.entries {
		entry.Cost = -12345
	}
	n, err := source.ReadmitCheckedCandidates(p, &destination, 1)
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	_, cost, hit := destination.LookupChecked(p.Ops, Z80, p.Options)
	if !hit || cost < 0 {
		t.Fatal("claimed cost trusted", cost, hit)
	}
	f.Contract.Returns[0].Class = mir2.ClassRegC
	q, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	var rejected RegAllocTable
	rejected.Init()
	n, err = source.ReadmitCheckedCandidates(q, &rejected, 1)
	if err != nil || n != 0 || len(rejected.entries) != 0 {
		t.Fatal("wrong return ABI admitted", n, err)
	}
	if _, err := source.ReadmitCheckedCandidates(p, &rejected, 1); err == nil {
		t.Fatal("stale function admitted")
	}
	// Validate bounds before any destination mutation.
	source.entries["extra"] = &RegAllocEntry{}
	if _, err := source.ReadmitCheckedCandidates(q, &rejected, 1); err == nil || len(rejected.entries) != 0 {
		t.Fatal("oversize pool partially admitted")
	}
}
