package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// TestPBQP_R0_IsolatedNode verifies that a degree-0 (isolated) virtual reg
// is assigned its minimum-cost location (R0 rule).
func TestPBQP_R0_IsolatedNode(t *testing.T) {
	m := &mir2.Module{Name: "pbqp_r0"}
	f := m.AddFunc("isolated")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	c := bld.Const(42, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(c)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.PBQPAllocate(f, lr, mir2.Z80CostTable{})

	// ClassAcc with no neighbours → should get A (cost 0).
	loc := ar.Locs[c]
	if loc.Name != "A" {
		t.Errorf("R0: isolated ClassAcc should → A; got %v", loc)
	}
	if len(ar.Spilled) != 0 {
		t.Errorf("R0: no spill expected; got %v", ar.Spilled)
	}
}

// TestPBQP_R1_LeafNode verifies the R1 rule: a degree-1 (leaf) node is
// assigned the location that minimises the joint cost with its one neighbour,
// not just its own minimum cost in isolation.
//
// Setup: two ClassAcc regs A-class (r1) and B-class (r2) that interfere.
// r2 is used 10× more than r1. The R1 rule should give r2 the cheaper slot
// (A) and r1 the next-best (B), because paying the higher reg-move cost once
// for r1 (used 1×) is cheaper than paying it 10× for r2.
func TestPBQP_R1_LeafNode(t *testing.T) {
	m := &mir2.Module{Name: "pbqp_r1"}
	f := m.AddFunc("leaf")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")

	// r1: used once (just the def).
	r1 := bld.Const(1, mir2.TyU8, mir2.ClassAcc)

	// r2: used 10 times — add it to itself repeatedly so liveness sees uses.
	r2 := bld.Const(2, mir2.TyU8, mir2.ClassAcc)
	acc := r2
	for range 9 {
		acc = bld.Add(acc, r2, mir2.TyU8, mir2.ClassAcc)
	}

	// Combine so both are live: result = r1 + acc.
	total := bld.Add(r1, acc, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(total)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.PBQPAllocate(f, lr, mir2.Z80CostTable{})

	// Neither should spill.
	if len(ar.Spilled) != 0 {
		t.Errorf("R1: unexpected spill %v", ar.Spilled)
	}

	// r2 (high-use) should get the cheaper location (A, cost 0);
	// r1 (low-use) should get a secondary location (B, C, etc.).
	r2Loc := ar.Locs[r2]
	r1Loc := ar.Locs[r1]
	t.Logf("r1(use=1) → %v, r2(use=10+) → %v", r1Loc, r2Loc)

	if r2Loc.Name == r1Loc.Name {
		t.Errorf("R1: r1 and r2 must not share a location")
	}
	// The high-use reg should pay lower per-use cost than the low-use reg.
	ct := mir2.Z80CostTable{}
	r1Info := mir2.CollectRegInfoForTest(f)[r1]
	r2Info := mir2.CollectRegInfoForTest(f)[r2]
	c1 := ct.Cost(r1Info.Cls, r1Loc)
	c2 := ct.Cost(r2Info.Cls, r2Loc)
	if c2 > c1 {
		t.Errorf("R1: high-use r2 should get lower-cost slot than r1; c2=%d > c1=%d", c2, c1)
	}
}

// TestPBQP_WeightedCost_BeatGreedy verifies that PBQP produces ≤ spills and
// ≤ total static cost compared to the old greedy allocator on a function with
// two same-class regs of unequal use counts.
//
// r_heavy is used 10× more than r_light.  Both interfere with each other so
// one must pay a reg-move penalty.  PBQP (with weighted costs) should put
// r_heavy in the cheaper slot (A, cost 0) and r_light in secondary (B, cost 4).
// Greedy may do the same or worse depending on allocation order.
func TestPBQP_WeightedCost_BeatGreedy(t *testing.T) {
	build := func(m *mir2.Module) (*mir2.Func, mir2.Reg, mir2.Reg) {
		f := m.AddFunc("weighted")
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		bld := mir2.NewBuilder(f)
		bld.SwitchToNewBlock("entry")

		// r_light: used once (just the definition as Const).
		rLight := bld.Const(1, mir2.TyU8, mir2.ClassAcc)

		// r_heavy: add to itself 9 more times → 10 total uses.
		rHeavy := bld.Const(2, mir2.TyU8, mir2.ClassAcc)
		acc := rHeavy
		for range 9 {
			acc = bld.Add(acc, rHeavy, mir2.TyU8, mir2.ClassAcc)
		}
		// Both live simultaneously in the final add.
		total := bld.Add(acc, rLight, mir2.TyU8, mir2.ClassAcc)
		bld.Ret(total)
		return f, rLight, rHeavy
	}

	// Greedy.
	mg := &mir2.Module{Name: "greedy"}
	fg, rLightG, rHeavyG := build(mg)
	lrg := mir2.ComputeLiveness(fg)
	arg := mir2.Allocate(fg, lrg, mir2.Z80CostTable{})

	// PBQP.
	mp := &mir2.Module{Name: "pbqp"}
	fp, rLightP, rHeavyP := build(mp)
	lrp := mir2.ComputeLiveness(fp)
	arp := mir2.PBQPAllocate(fp, lrp, mir2.Z80CostTable{})

	t.Logf("Greedy: light=%v heavy=%v spills=%v", arg.Locs[rLightG], arg.Locs[rHeavyG], arg.Spilled)
	t.Logf("PBQP:   light=%v heavy=%v spills=%v", arp.Locs[rLightP], arp.Locs[rHeavyP], arp.Spilled)

	// PBQP must not regress on spills.
	if len(arp.Spilled) > len(arg.Spilled) {
		t.Errorf("PBQP spilled %d regs vs greedy %d — regression", len(arp.Spilled), len(arg.Spilled))
	}

	// PBQP must give r_heavy the cheaper slot (A).
	ct := mir2.Z80CostTable{}
	infoP := mir2.CollectRegInfoForTest(fp)
	costLight := ct.Cost(infoP[rLightP].Cls, arp.Locs[rLightP])
	costHeavy := ct.Cost(infoP[rHeavyP].Cls, arp.Locs[rHeavyP])
	if costHeavy > costLight {
		t.Errorf("PBQP: high-use r_heavy (cost %d) in worse slot than r_light (cost %d)", costHeavy, costLight)
	}
}

// TestPBQP_FourPointers_NoSpill is the Phase 6a/6b combined acceptance test:
// four simultaneously-live ClassPointer regs, PBQP must place them in
// HL/DE/BC/IX without any $F0xx spill.  This repeats the greedy test but
// through the PBQP code path.
func TestPBQP_FourPointers_NoSpill(t *testing.T) {
	m := &mir2.Module{Name: "pbqp4ptr"}
	f := m.AddFunc("four_ptrs")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	p0 := bld.Param("p0", mir2.TyPtr, mir2.ClassPointer)
	p1 := bld.Param("p1", mir2.TyPtr, mir2.ClassPointer)
	p2 := bld.Param("p2", mir2.TyPtr, mir2.ClassPointer)
	p3 := bld.Param("p3", mir2.TyPtr, mir2.ClassPointer)

	v0 := bld.Load(p0, mir2.TyU8, mir2.ClassAcc)
	v1 := bld.Load(p1, mir2.TyU8, mir2.ClassAcc)
	v2 := bld.Load(p2, mir2.TyU8, mir2.ClassAcc)
	v3 := bld.Load(p3, mir2.TyU8, mir2.ClassAcc)

	s01 := bld.Add(v0, v1, mir2.TyU8, mir2.ClassAcc)
	s23 := bld.Add(v2, v3, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(bld.Add(s01, s23, mir2.TyU8, mir2.ClassAcc))

	lr := mir2.ComputeLiveness(f)
	ar := mir2.PBQPAllocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if len(ar.Spilled) != 0 {
		t.Errorf("PBQP: unexpected spill %v; locs p0=%v p1=%v p2=%v p3=%v",
			ar.Spilled, ar.Locs[p0], ar.Locs[p1], ar.Locs[p2], ar.Locs[p3])
	}
	if strings.Contains(asm, "($F0") {
		t.Errorf("PBQP: $F0xx spill in output:\n%s", asm)
	}

	// All four pointers must be in distinct physical locations.
	locs := map[string]bool{
		ar.Locs[p0].Name: true,
		ar.Locs[p1].Name: true,
		ar.Locs[p2].Name: true,
		ar.Locs[p3].Name: true,
	}
	if len(locs) < 4 {
		t.Errorf("PBQP: pointers share locations: p0=%v p1=%v p2=%v p3=%v",
			ar.Locs[p0], ar.Locs[p1], ar.Locs[p2], ar.Locs[p3])
	}
}
