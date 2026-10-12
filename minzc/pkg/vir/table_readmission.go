package vir

import (
	"fmt"
	"sort"
)

// ReadmitCheckedCandidates treats saved assignments as untrusted proposals for
// one current problem. Old keys and costs never authorize current emission.
// This is bounded offline preparation, not automatic production lookup or proof
// that the old and current operation graphs have the same shape.
func (t *RegAllocTable) ReadmitCheckedCandidates(p *LocalTableProblem, target *RegAllocTable, limit int) (int, error) {
	if t == nil || target == nil || p == nil || p.Function == nil || limit < 1 || limit > MaxLegacyCandidates {
		return 0, fmt.Errorf("invalid readmission source, target, problem or limit")
	}
	context, err := localFunctionContext(p.Function)
	if err != nil || context != p.Options.CheckedContext {
		return 0, fmt.Errorf("readmission stale function binding")
	}
	t.mu.RLock()
	if len(t.entries) > limit {
		t.mu.RUnlock()
		return 0, fmt.Errorf("readmission candidate limit exceeded")
	}
	var entries []RegAllocEntry
	for _, entry := range t.entries {
		if entry == nil {
			continue
		}
		copy := *entry
		copy.Assignment = append([]int(nil), entry.Assignment...)
		entries = append(entries, copy)
	}
	t.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Signature < entries[j].Signature })
	var ids []int
	for v := range buildProblem(p.Ops, Z80).vregs {
		ids = append(ids, v)
	}
	sort.Ints(ids)
	admitted := 0
	for _, entry := range entries {
		if entry.Schema != checkedTableSchema || entry.CostModel != checkedTableCostModel || entry.NOps != len(p.Ops) || entry.NVregs != len(ids) || len(entry.Assignment) != len(ids) {
			continue
		}
		assignment := make(map[int]int, len(ids))
		for i, v := range ids {
			assignment[v] = entry.Assignment[i]
		}
		if err := target.AddChecked(p.Ops, assignment, Z80, p.Options); err == nil {
			admitted++
		}
	}
	return admitted, nil
}
