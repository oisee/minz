package mir2

import "testing"

func TestCoalescedBranchEdgeNeedsNoTrampoline(t *testing.T) {
	f := &Func{Name: "edge"}
	g := &z80cg{}
	copies := []parallelCopy{{srcName: "D", dstName: "D", ty: TyU8}, {srcName: "HL", dstName: "HL", ty: TyU16}}
	if got := g.branchLabel(f, "loop", copies); got != blockLabel("edge", "loop") || len(g.trampolines) != 0 {
		t.Fatalf("coalesced edge got%s trampolines%d", got, len(g.trampolines))
	}
	// Keep the coalesced destinations in mixed edges: the copy resolver must
	// still see that D and HL cannot serve as scratch registers.
	mixed := append(copies, parallelCopy{srcName: "B", dstName: "C", ty: TyU8})
	g.branchLabel(f, "loop", mixed)
	if len(g.trampolines) != 1 || len(g.trampolines[0].copies) != 3 {
		t.Fatal("mixed edge lost scratch reservations")
	}
	if !hasEffectiveBlockCopies([]parallelCopy{{dstName: "D", ty: TyU8, isImm: true, immVal: 0}}) {
		t.Fatal("immediate edge omitted")
	}
}
