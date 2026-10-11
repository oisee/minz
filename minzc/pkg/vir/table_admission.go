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
		live := make(map[int]bool)
		for i := len(ops) - 1; i >= 0; i-- {
			op := ops[i]
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
					if loc == clobber || desc.Locs[loc].Alias.Has(clobber) || desc.Locs[clobber].Alias.Has(loc) ||
						(desc.Name == "z80" && z80spec.Overlaps(desc.Locs[loc].Name, desc.Locs[clobber].Name)) {
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
		}
	}
	return nil
}
