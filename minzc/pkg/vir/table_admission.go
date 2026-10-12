package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80spec"
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
		check := func(values map[int]bool, i int) error {
			for a := range values {
				for b := range values {
					if a < b && overlaps(assignment[a], assignment[b]) {
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
					if v != d && overlaps(assignment[d], assignment[v]) && !(op.Op == OpMove && v == op.Src[0] && assignment[d] == assignment[v]) {
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
