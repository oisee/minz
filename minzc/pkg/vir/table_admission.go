package vir

import "fmt"

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
	return nil
}
