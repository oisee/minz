package vir

import "sort"

// An existing move followed immediately by its RET may share one fragment.
func isCompositionMoveFragment(ops []VIROp) bool {
	return (len(ops) == 1 || len(ops) == 2 && ops[1].Op == OpRet && ops[1].Src[0] == ops[0].Dst) && ops[0].Op == OpMove
}

// Only locations proposed by the bounded pool, the ABI or current hints are
// considered. Pattern matching checks the unchanged operation and its effects.
func compositionMoveChoices(ops []VIROp, entries []RegAllocEntry, opts SolverOptions) []map[int]int {
	op := ops[0]
	locations := map[int]bool{}
	include := func(loc int) {
		if loc >= 0 && loc < len(Z80.Locs) && Z80.Locs[loc].Width == op.Width && !Z80.NonAllocatable.Has(loc) {
			locations[loc] = true
		}
	}
	for _, entry := range entries {
		if entry.Schema == checkedTableSchema && entry.CostModel == checkedTableCostModel {
			for _, loc := range entry.Assignment {
				include(loc)
			}
		}
	}
	for _, loc := range opts.ParamLocs {
		include(loc)
	}
	include(Z80.LocByName(opts.CheckedReturnLocation))
	op.DstHint.ForEach(func(loc int) bool { include(loc); return true })
	op.SrcHint[0].ForEach(func(loc int) bool { include(loc); return true })
	var ordered []int
	for loc := range locations {
		ordered = append(ordered, loc)
	}
	sort.Ints(ordered)
	localOpts := opts
	localOpts.ParamLocs = map[int]int{}
	for v, loc := range opts.ParamLocs {
		if v == op.Dst || v == op.Src[0] {
			localOpts.ParamLocs[v] = loc
		}
	}
	var choices []map[int]int
	for _, src := range ordered {
		for _, dst := range ordered {
			if op.Dst == op.Src[0] && src != dst {
				continue
			}
			assignment := map[int]int{op.Src[0]: src, op.Dst: dst}
			if _, err := checkedLocalCost(ops, assignment, Z80, localOpts); err == nil {
				choices = append(choices, assignment)
			}
		}
	}
	return choices
}
