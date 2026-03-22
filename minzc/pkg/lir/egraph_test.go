package lir

import "testing"

func TestEGraph_Basic(t *testing.T) {
	eg := NewEGraph()

	// Class 0: two variants for truncation
	id0 := eg.AddClass(
		EVariant{Ops: []MIROp{{Op: OpMove, Width: 8}}, Cost: 4, Tag: "trunc_L"},
		EVariant{Ops: []MIROp{{Op: OpMove, Width: 8}}, Cost: 8, Tag: "trunc_via_A"},
	)

	// Class 1: single variant (no alternatives)
	id1 := eg.SingleClass([]MIROp{{Op: OpAdd, Width: 8}}, 4, "add")

	if id0 != 0 || id1 != 1 {
		t.Fatalf("expected ids 0,1 got %d,%d", id0, id1)
	}

	classes, variants, avg := eg.Stats()
	if classes != 2 || variants != 3 {
		t.Errorf("stats: classes=%d variants=%d, want 2,3", classes, variants)
	}
	if avg < 1.4 || avg > 1.6 {
		t.Errorf("avg=%.2f, want ~1.5", avg)
	}

	// SelectCheapest should pick the cheaper variant
	eg.SelectCheapest()
	cls := eg.Class(0)
	if cls.Best != 0 {
		t.Errorf("class 0 best=%d, want 0 (cost 4 < cost 8)", cls.Best)
	}
}

func TestEGraph_Merge(t *testing.T) {
	eg := NewEGraph()

	id0 := eg.AddClass(
		EVariant{Cost: 10, Tag: "A"},
	)
	id1 := eg.AddClass(
		EVariant{Cost: 5, Tag: "B"},
		EVariant{Cost: 15, Tag: "C"},
	)

	eg.Merge(id0, id1)

	// After merge, root class should have all 3 variants
	root := eg.Class(id0)
	if len(root.Variants) != 3 {
		t.Errorf("merged class has %d variants, want 3", len(root.Variants))
	}

	eg.SelectCheapest()
	if root.Variants[root.Best].Tag != "B" {
		t.Errorf("cheapest=%s, want B (cost 5)", root.Variants[root.Best].Tag)
	}
}

func TestEGraph_Extract(t *testing.T) {
	eg := NewEGraph()

	eg.AddClass(
		EVariant{Ops: []MIROp{{Op: OpMove, Imm: 1}}, Cost: 10, Tag: "slow"},
		EVariant{Ops: []MIROp{{Op: OpMove, Imm: 2}}, Cost: 4, Tag: "fast"},
	)
	eg.AddClass(
		EVariant{Ops: []MIROp{{Op: OpAdd, Imm: 3}}, Cost: 7, Tag: "only"},
	)

	eg.SelectCheapest()
	ops := eg.Extract()

	if len(ops) != 2 {
		t.Fatalf("extracted %d ops, want 2", len(ops))
	}
	// First class: cheapest is "fast" (Imm=2)
	if ops[0].Imm != 2 {
		t.Errorf("first op imm=%d, want 2 (fast variant)", ops[0].Imm)
	}
	// Second class: only variant (Imm=3)
	if ops[1].Imm != 3 {
		t.Errorf("second op imm=%d, want 3", ops[1].Imm)
	}
}
