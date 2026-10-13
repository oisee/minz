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
	return t.composeCheckedCandidates(p, target, cuts, limit, false)
}

// ComposeCheckedCandidatesWithMoves additionally assigns existing move (optionally followed by RET)
// fragments as explicit bridges between proposals. It never inserts operations,
// changes SSA value identities, or invents parallel-copy schedules. The bridge
// cost, scratch effects and actual return ABI enter whole-function admission.
func (t *RegAllocTable) ComposeCheckedCandidatesWithMoves(p *LocalTableProblem, target *RegAllocTable, cuts []int, limit int) (int, error) {
	return t.composeCheckedCandidates(p, target, cuts, limit, true)
}

func (t *RegAllocTable) composeCheckedCandidates(p *LocalTableProblem, target *RegAllocTable, cuts []int, limit int, moves bool) (int, error) {
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
	// Build and independently admit each fragment's finite assignment domain.
	domains := make([][]map[int]int, 0, len(ends))
	start = 0
	for _, end := range ends {
		ops := p.Ops[start:end]
		var ids []int
		for v := range buildProblem(ops, Z80).vregs {
			ids = append(ids, v)
		}
		sort.Ints(ids)
		choices := []map[int]int{}
		seen := map[string]bool{}
		for _, entry := range entries {
			if entry.Schema != checkedTableSchema || entry.CostModel != checkedTableCostModel || entry.NOps != len(ops) || entry.NVregs != len(ids) || len(entry.Assignment) != len(ids) {
				continue
			}
			fragment := make(map[int]int, len(ids))
			for i, v := range ids {
				fragment[v] = entry.Assignment[i]
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
			key := fmt.Sprint(entry.Assignment)
			if !seen[key] {
				choices = append(choices, fragment)
				seen[key] = true
			}
		}
		if moves && isCompositionMoveFragment(ops) {
			for _, bridge := range compositionMoveChoices(ops, entries, p.Options) {
				key := fmt.Sprint(bridge)
				duplicate := false
				for _, choice := range choices {
					if fmt.Sprint(choice) == key {
						duplicate = true
						break
					}
				}
				if !duplicate {
					choices = append(choices, bridge)
				}
			}
		}
		domains = append(domains, choices)
		start = end
	}
	domains = propagateCompositionDomains(domains)
	candidates := []map[int]int{{}}
	attempts := 0
	for _, choices := range domains {
		if len(choices) == 0 {
			return 0, nil
		}
		next := []map[int]int{}
		for _, fragment := range choices {
			for _, prefix := range candidates {
				if !compositionAssignmentsCompatible(prefix, fragment) {
					continue
				}
				attempts++
				if attempts > limit {
					return 0, fmt.Errorf("composition combinations exceed bound")
				}
				combined := make(map[int]int, len(prefix)+len(fragment))
				for v, loc := range prefix {
					combined[v] = loc
				}
				for v, loc := range fragment {
					combined[v] = loc
				}
				next = append(next, combined)
			}
		}
		candidates = next
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
