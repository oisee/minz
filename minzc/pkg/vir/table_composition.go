package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"sort"
)

// ComposeCheckedCandidates combines bounded straight-line assignment proposals.
// cuts partition the complete current operation sequence. Shared vregs must have
// identical locations; incompatible boundaries are refused, never repaired with
// unmodelled moves. Every full assignment is admitted again including the actual
// return ABI, scratch writes, overlap, immediates and all explicit operations.
// This preparation does not invoke a solver or alter production IR.
func (t *RegAllocTable) ComposeCheckedCandidates(p *LocalTableProblem, target *RegAllocTable, cuts []int, limit int) (int, error) {
	if t == nil || target == nil || p == nil || p.Function == nil || limit < 1 || limit > MaxLegacyCandidates || len(cuts) < 1 || len(cuts) > 7 {
		return 0, fmt.Errorf("invalid composition input or bounds")
	}
	context, err := localFunctionContext(p.Function)
	if err != nil || context != p.Options.CheckedContext {
		return 0, fmt.Errorf("stale composition function binding")
	}
	// Rebuild the lowering from the real MIR and its entry placements. A
	// caller may mutate the prepared ops without changing the function hash.
	allocation := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, param := range p.Function.Contract.Params {
		index, exists := p.Options.ParamLocs[int(param.Reg)]
		if exists {
			if index < 0 || index >= len(Z80.Locs) {
				return 0, fmt.Errorf("invalid composition ABI placement")
			}
			name := Z80.Locs[index].Name
			for _, location := range mir2.Z80PhysLocs {
				if location.Name == name {
					allocation.Locs[param.Reg] = location
					break
				}
			}
		} else {
			for _, location := range mir2.Z80PhysLocs {
				if localPhysicalRegister(location) && (mir2.Z80CostTable{}).Cost(param.Class, location) < mir2.InfCost && Z80.LocByName(location.Name) >= 0 && Z80.Locs[Z80.LocByName(location.Name)].Width == param.Ty.Width() {
					allocation.Locs[param.Reg] = location
					break
				}
			}
		}
	}
	rebuilt, err := PrepareLocalTableProblem(p.Function, allocation)
	if err != nil || !reflect.DeepEqual(rebuilt.Ops, p.Ops) || !reflect.DeepEqual(rebuilt.Options, p.Options) {
		return 0, fmt.Errorf("composition problem differs from current MIR")
	}
	ends := append(append([]int(nil), cuts...), len(p.Ops))
	start := 0
	for _, end := range ends {
		if end <= start || end > len(p.Ops) {
			return 0, fmt.Errorf("invalid composition partition")
		}
		start = end
	}
	t.mu.RLock()
	if len(t.entries) > limit {
		t.mu.RUnlock()
		return 0, fmt.Errorf("composition pool exceeds bound")
	}
	entries := make([]RegAllocEntry, 0, len(t.entries))
	for _, entry := range t.entries {
		if entry != nil {
			copy := *entry
			copy.Assignment = append([]int(nil), entry.Assignment...)
			entries = append(entries, copy)
		}
	}
	t.mu.RUnlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Signature < entries[j].Signature })
	candidates := []map[int]int{{}}
	start = 0
	attempts := 0
	for _, end := range ends {
		ops := p.Ops[start:end]
		var ids []int
		for v := range buildProblem(ops, Z80).vregs {
			ids = append(ids, v)
		}
		sort.Ints(ids)
		next := []map[int]int{}
		for _, entry := range entries {
			if entry.Schema != checkedTableSchema || entry.CostModel != checkedTableCostModel || entry.NOps != len(ops) || entry.NVregs != len(ids) || len(entry.Assignment) != len(ids) {
				continue
			}
			for _, prefix := range candidates {
				attempts++
				if attempts > limit {
					return 0, fmt.Errorf("composition combinations exceed bound")
				}
				combined := make(map[int]int, len(prefix)+len(ids))
				for v, loc := range prefix {
					combined[v] = loc
				}
				valid := true
				fragment := make(map[int]int, len(ids))
				for i, v := range ids {
					loc := entry.Assignment[i]
					if old, exists := combined[v]; exists && old != loc {
						valid = false
						break
					}
					combined[v] = loc
					fragment[v] = loc
				}
				if !valid {
					continue
				}
				opts := p.Options
				opts.ParamLocs = map[int]int{}
				for v, loc := range p.Options.ParamLocs {
					if _, exists := fragment[v]; exists {
						opts.ParamLocs[v] = loc
					}
				}
				if _, err := checkedLocalCost(ops, fragment, Z80, opts); err != nil {
					continue
				}
				next = append(next, combined)
			}
		}
		candidates = next
		start = end
		if len(candidates) == 0 {
			return 0, nil
		}
	}
	// Stage admission privately: bounds failure never partially changes target.
	var staged RegAllocTable
	staged.Init()
	admitted := 0
	for _, assignment := range candidates {
		if err := staged.AddChecked(p.Ops, assignment, Z80, p.Options); err == nil {
			admitted++
		}
	}
	if admitted == 0 {
		return 0, nil
	}
	assignment, _, ok := staged.LookupChecked(p.Ops, Z80, p.Options)
	if !ok {
		return 0, fmt.Errorf("composed candidate lost admission")
	}
	if err := target.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
		return 0, err
	}
	return admitted, nil
}
