package vir

import "testing"

func TestCheckedCopyEqualityEpochs(t *testing.T) {
	a, hl := Z80.LocByName("A"), Z80.LocByName("HL")
	positive := []VIROp{
		{Op: OpMove, Dst: 2, Src: [2]int{1, -1}, Width: 8},
		{Op: OpMove, Dst: 3, Src: [2]int{2, -1}, Width: 8},
		{Op: OpStore, Src: [2]int{4, 1}, Width: 8},
		{Op: OpStore, Src: [2]int{4, 2}, Width: 8},
		{Op: OpRet, Src: [2]int{3, -1}, Width: 8},
	}
	assignment := map[int]int{1: a, 2: a, 3: a, 4: hl}
	if e := validateTableAssignment(positive, assignment, Z80); e != nil {
		t.Fatal("equal transitive copies rejected", e)
	}
	for _, kind := range []string{"redefine-source", "redefine-copy", "different-values", "partial-width", "asm-output", "clobber", "stale-epoch-before-copy"} {
		t.Run(kind, func(t *testing.T) {
			ops := append([]VIROp(nil), positive...)
			switch kind {
			case "redefine-source":
				ops = append(ops[:2], append([]VIROp{{Op: OpConst, Dst: 1, Imm: 9, Width: 8}}, positive[2:]...)...)
			case "redefine-copy":
				ops = append(ops[:2], append([]VIROp{{Op: OpConst, Dst: 2, Imm: 9, Width: 8}}, positive[2:]...)...)
			case "different-values":
				ops[0] = VIROp{Op: OpConst, Dst: 2, Imm: 9, Width: 8}
			case "partial-width":
				ops[0].Width = 16
			case "asm-output":
				ops[0].AsmOuts = []int{2}
			case "clobber":
				ops[0].Clobbers = Z80_A
			case "stale-epoch-before-copy":
				ops = append([]VIROp{{Op: OpStore, Src: [2]int{4, 2}, Width: 8}}, ops...)
			}
			if e := validateTableAssignment(ops, assignment, Z80); e == nil {
				t.Fatal("unproven equal values admitted", kind)
			}
		})
	}
}
