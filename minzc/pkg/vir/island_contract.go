package vir

import (
	"fmt"
	"sort"
)

// islandLiveIns returns uses before a local definition, including asm operands.
func islandLiveIns(ops []VIROp) []int {
	defined, inputs := map[int]bool{}, map[int]bool{}
	for _, op := range ops {
		for _, src := range append(append([]int(nil), op.Src[:]...), op.AsmIns...) {
			if src > 0 && !defined[src] {
				inputs[src] = true
			}
		}
		if op.Dst > 0 {
			defined[op.Dst] = true
		}
		for _, dst := range op.AsmOuts {
			if dst > 0 {
				defined[dst] = true
			}
		}
	}
	result := make([]int, 0, len(inputs))
	for v := range inputs {
		result = append(result, v)
	}
	sort.Ints(result)
	return result
}

// pinIslandEntry establishes the actual incoming locations before any real op.
// A scheduling use is a hard source hint, not a claim about later placement.
func pinIslandEntry(ops []VIROp, incoming map[int]int, desc *MachineDesc) ([]VIROp, error) {
	pins := make([]VIROp, 0)
	for _, v := range islandLiveIns(ops) {
		loc, ok := incoming[v]
		if !ok || loc < 0 || loc >= len(desc.Locs) || desc.NonAllocatable.Has(loc) {
			return nil, fmt.Errorf("island input v%d has no valid incoming location", v)
		}
		pins = append(pins, VIROp{Op: OpKeepAlive, Src: [2]int{v, -1}, Width: desc.Locs[loc].Width, SrcHint: [2]LocSet{Singleton(loc)}})
	}
	return append(pins, ops...), nil
}
