package vir

import (
	"fmt"
	"sort"
)

// ComposeCheckedAutomatically discovers complete straight-line partitions from
// checked proposal lengths, independently admitted fragment domains and existing
// move bridges. Each partition still goes
// through exact current MIR/ABI admission. Limit bounds both the number of
// partitions and each partition's assignment search; at most eight fragments
// are permitted. Failure never modifies the output table.
func (t *RegAllocTable) ComposeCheckedAutomatically(p *LocalTableProblem, target *RegAllocTable, limit int) (int, error) {
	if t == nil || target == nil || p == nil || p.Function == nil || limit < 1 || limit > MaxLegacyCandidates {
		return 0, fmt.Errorf("invalid automatic composition input or bound")
	}
	if err := validateCompositionProblem(p); err != nil {
		return 0, err
	}
	t.mu.RLock()
	if len(t.entries) > limit {
		t.mu.RUnlock()
		return 0, fmt.Errorf("automatic composition pool exceeds bound")
	}
	lengths := map[int]bool{}
	var entries []RegAllocEntry
	for _, entry := range t.entries {
		if entry != nil {
			copy := *entry
			copy.Assignment = append([]int(nil), entry.Assignment...)
			entries = append(entries, copy)
		}
		if entry != nil && entry.Schema == checkedTableSchema && entry.CostModel == checkedTableCostModel && entry.NOps > 0 {
			lengths[entry.NOps] = true
		}
	}
	t.mu.RUnlock()
	var ordered []int
	for length := range lengths {
		ordered = append(ordered, length)
	}
	sort.Ints(ordered)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Signature < entries[j].Signature })
	byLength := map[int][]RegAllocEntry{}
	for _, entry := range entries {
		if entry.Schema == checkedTableSchema && entry.CostModel == checkedTableCostModel {
			byLength[entry.NOps] = append(byLength[entry.NOps], entry)
		}
	}
	known, viable := map[[2]int]bool{}, map[[2]int]bool{}
	admissible := func(start, end int) bool {
		key := [2]int{start, end}
		if !known[key] {
			known[key] = true
			fragment := p.Ops[start:end]
			proposals := byLength[end-start]
			if isCompositionMoveFragment(fragment) {
				proposals = entries // Existing bridges can borrow any proposed location.
			}
			viable[key] = len(compositionFragmentChoices(fragment, proposals, p.Options, true)) > 0
		}
		return viable[key]
	}
	partitions, err := discoverCompositionPartitionsMatching(p.Ops, ordered, limit, admissible)
	if err != nil {
		return 0, err
	}
	var staged RegAllocTable
	staged.Init()
	admitted := 0
	for _, cuts := range partitions {
		count, err := t.ComposeCheckedCandidatesWithMoves(p, &staged, cuts, limit)
		if err != nil {
			return 0, err
		}
		admitted += count
	}
	if admitted == 0 {
		return 0, nil
	}
	assignment, _, ok := staged.LookupChecked(p.Ops, Z80, p.Options)
	if !ok {
		return 0, fmt.Errorf("automatic composition lost admission")
	}
	if err := target.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
		return 0, err
	}
	return admitted, nil
}

func discoverCompositionPartitions(ops []VIROp, ordered []int, limit int) ([][]int, error) {
	return discoverCompositionPartitionsMatching(ops, ordered, limit, nil)
}

// A nonempty fragment domain is necessary, not sufficient: cross-fragment
// liveness/aliases and the complete MIR/ABI still decide final admission.
func discoverCompositionPartitionsMatching(ops []VIROp, ordered []int, limit int, admissible func(int, int) bool) ([][]int, error) {
	// Suffix reachability avoids exploring incomplete prefixes. The fragment
	// count is part of the state, keeping long streams bounded as well.
	n := len(ops)
	ends := func(start int) []int {
		choices := map[int]bool{}
		for _, length := range ordered {
			if length <= n-start && (admissible == nil || admissible(start, start+length)) {
				choices[start+length] = true
			}
		}
		for _, length := range []int{1, 2} {
			if start+length <= n && isCompositionMoveFragment(ops[start:start+length]) && (admissible == nil || admissible(start, start+length)) {
				choices[start+length] = true
			}
		}
		var result []int
		for end := range choices {
			result = append(result, end)
		}
		sort.Ints(result)
		return result
	}
	type state struct{ start, remaining int }
	reachable := map[state]bool{}
	known := map[state]bool{}
	var canFinish func(int, int) bool
	canFinish = func(start, remaining int) bool {
		if start == n {
			return true
		}
		if remaining == 0 {
			return false
		}
		s := state{start, remaining}
		if known[s] {
			return reachable[s]
		}
		known[s] = true
		for _, end := range ends(start) {
			if canFinish(end, remaining-1) {
				reachable[s] = true
				break
			}
		}
		return reachable[s]
	}
	var partitions [][]int
	var visit func(int, []int) error
	visit = func(start int, cuts []int) error {
		for _, end := range ends(start) {
			if end == n {
				if len(cuts) > 0 {
					partitions = append(partitions, append([]int(nil), cuts...))
					if len(partitions) > limit {
						return fmt.Errorf("automatic composition partitions exceed bound")
					}
				}
			} else if len(cuts) < 7 && canFinish(end, 7-len(cuts)) {
				if err := visit(end, append(append([]int(nil), cuts...), end)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(0, nil); err != nil {
		return nil, err
	}
	return partitions, nil
}
