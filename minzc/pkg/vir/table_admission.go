package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80spec"
	"sort"
)

// validateTableAssignment checks the input contract independently of table
// metadata and pattern selection. An absent vreg must never become loc -1
// (which the pattern matcher interprets as an unconstrained operand).
func validateTableAssignment(ops []VIROp, assignment map[int]int, desc *MachineDesc) error {
	require := func(v int) error {
		if v <= 0 {
			return nil
		}
		loc, ok := assignment[v]
		if !ok {
			return fmt.Errorf("missing assignment for vreg %d", v)
		}
		if loc < 0 || (desc != nil && loc >= len(desc.Locs)) {
			return fmt.Errorf("invalid location %d for vreg %d", loc, v)
		}
		return nil
	}
	for _, op := range ops {
		if err := require(op.Dst); err != nil {
			return err
		}
		if desc != nil && op.Dst > 0 && op.Width > 0 {
			width := op.Width
			if op.Op == OpCmp || op.Op == OpCmpImm || op.Op == OpBitTest {
				width = 1
			}
			loc := assignment[op.Dst]
			if desc.Locs[loc].Width != width {
				return fmt.Errorf("location %s has width %d for vreg %d, require %d", desc.Locs[loc].Name, desc.Locs[loc].Width, op.Dst, width)
			}
		}
		for _, v := range op.Src {
			if err := require(v); err != nil {
				return err
			}
		}
		for _, v := range op.AsmIns {
			if err := require(v); err != nil {
				return err
			}
		}
		for _, v := range op.AsmOuts {
			if err := require(v); err != nil {
				return err
			}
		}
	}
	if desc != nil {
		// Walk backwards independently of the solver's range approximation.
		// Inputs consumed here may still be needed later; outputs are produced
		// after the clobber and must not be mistaken for live-through values.
		overlaps := func(a, b int) bool {
			return a == b || desc.Locs[a].Alias.Has(b) || desc.Locs[b].Alias.Has(a) ||
				(desc.Name == "z80" && z80spec.Overlaps(desc.Locs[a].Name, desc.Locs[b].Name))
		}
		// Track value epochs, rather than equating vreg names permanently.
		// Exact-width moves copy a value; every other definition creates a new
		// epoch. This also handles non-SSA research inputs conservatively.
		type valueEpoch struct{ position, value int }
		history := map[int][]valueEpoch{}
		current := map[int]int{}
		nextValue := 0
		for v := range assignment {
			nextValue++
			current[v] = nextValue
			history[v] = []valueEpoch{{0, nextValue}}
		}
		exactMove := func(op VIROp) bool {
			return op.Op == OpMove && op.Dst > 0 && op.Src[0] > 0 && op.Width > 0 &&
				desc.Locs[assignment[op.Dst]].Width == op.Width && desc.Locs[assignment[op.Src[0]]].Width == op.Width &&
				len(op.AsmOuts) == 0 && op.Clobbers.IsEmpty()
		}
		for i, op := range ops {
			value := 0
			if exactMove(op) {
				value = current[op.Src[0]]
			}
			for _, d := range append([]int{op.Dst}, op.AsmOuts...) {
				if d <= 0 {
					continue
				}
				nextValue++
				newValue := nextValue
				if d == op.Dst && value != 0 {
					newValue = value
				}
				current[d] = newValue
				history[d] = append(history[d], valueEpoch{i + 1, newValue})
			}
		}
		sameValue := func(a, b, position int) bool {
			if assignment[a] != assignment[b] {
				return false
			}
			valueAt := func(v int) int {
				epochs := history[v]
				end := sort.Search(len(epochs), func(j int) bool { return epochs[j].position > position })
				if end == 0 {
					return 0
				}
				return epochs[end-1].value
			}
			aValue, bValue := valueAt(a), valueAt(b)
			return aValue != 0 && aValue == bValue
		}
		check := func(values map[int]bool, i int) error {
			for a := range values {
				for b := range values {
					if a < b && overlaps(assignment[a], assignment[b]) && !sameValue(a, b, i) {
						return fmt.Errorf("op %d overlapping live vregs %d and %d", i, a, b)
					}
				}
			}
			return nil
		}
		live := make(map[int]bool)
		for i := len(ops) - 1; i >= 0; i-- {
			op := ops[i]
			// Definitions write their locations even if the result is dead.
			// Only a move from an equal value permits overlapping live storage.
			for _, d := range append([]int{op.Dst}, op.AsmOuts...) {
				if d <= 0 {
					continue
				}
				for v := range live {
					if v != d && overlaps(assignment[d], assignment[v]) && !(exactMove(op) && assignment[d] == assignment[v] && sameValue(op.Src[0], v, i)) {
						return fmt.Errorf("op %d definition vreg %d overwrites live vreg %d", i, d, v)
					}
				}
			}
			delete(live, op.Dst)
			for _, v := range op.AsmOuts {
				delete(live, v)
			}
			for v := range live {
				loc := assignment[v]
				var conflict bool
				op.Clobbers.ForEach(func(clobber int) bool {
					if clobber >= len(desc.Locs) {
						conflict = true
						return false
					}
					if overlaps(loc, clobber) {
						conflict = true
						return false
					}
					return true
				})
				if conflict {
					return fmt.Errorf("op %d clobbers live-through vreg %d at %s", i, v, desc.Locs[loc].Name)
				}
			}
			for _, v := range op.Src {
				if v > 0 {
					live[v] = true
				}
			}
			for _, v := range op.AsmIns {
				if v > 0 {
					live[v] = true
				}
			}
			if err := check(live, i); err != nil {
				return err
			}
		}
	}
	return nil
}
