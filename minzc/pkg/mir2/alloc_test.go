package mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// ── InterferenceGraph tests ───────────────────────────────────────────────────

func TestIGraphSimple(t *testing.T) {
	// fun @f(%r1: u8, %r2: u8) -> u8
	//   block @entry:
	//     %r3 = add %r1, %r2 : u8    ; born when live={%r3} → no new edges
	//     %r4 = add %r2, %r3 : u8    ; born when live={%r4} → no new edges;
	//                                ;   after removal, adds %r2 and %r3 → live={%r2,%r3}
	//     ret %r4
	//
	// Live ranges:
	//   %r1: [param..add(%r1,%r2)]      → dead after first add
	//   %r2: [param..add(%r2,%r3)]      → still needed for second add
	//   %r3: [add(%r1,%r2)..add(%r2,%r3)] → live during second add
	//   %r4: [add(%r2,%r3)..ret]
	//
	// Interference (simultaneous live):
	//   %r2 ↔ %r3   (both alive at second add's birth point)
	//   %r1 ↔ %r2   (simultaneous function params)

	m := &mir2.Module{Name: "f"}
	f := m.AddFunc("f")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("a", mir2.TyU8, mir2.ClassGeneral)
	r2 := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	r3 := bld.Add(r1, r2, mir2.TyU8, mir2.ClassGeneral) // a+b
	r4 := bld.Add(r2, r3, mir2.TyU8, mir2.ClassAcc)    // b+(a+b): uses r2 AND r3
	bld.Ret(r4)

	lr := mir2.ComputeLiveness(f)
	ig := mir2.BuildInterferenceGraph(f, lr)

	// Function params interfere with each other (defined simultaneously).
	if !ig.Neighbors(r1).Has(r2) {
		t.Errorf("%%r%d and %%r%d should interfere (simultaneous params)", r1, r2)
	}
	// %r2 and %r3 are simultaneously live during second add → must interfere.
	if !ig.Neighbors(r2).Has(r3) {
		t.Errorf("%%r%d and %%r%d should interfere (both live at second add)", r2, r3)
	}
	// Symmetry.
	if !ig.Neighbors(r3).Has(r2) {
		t.Errorf("%%r%d and %%r%d should interfere (symmetry)", r3, r2)
	}
}

func TestIGraphNoSelfEdge(t *testing.T) {
	// A register must never interfere with itself.
	m := &mir2.Module{Name: "self"}
	f := m.AddFunc("self")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("x", mir2.TyU8, mir2.ClassGeneral)
	bld.Ret(r1)

	lr := mir2.ComputeLiveness(f)
	ig := mir2.BuildInterferenceGraph(f, lr)

	if ig.Neighbors(r1).Has(r1) {
		t.Errorf("%%r%d must not interfere with itself", r1)
	}
}

func TestIGraphFibNoConflict(t *testing.T) {
	// In fibonacci, loop_head block params (%r8, %r9, %r10) are defined simultaneously
	// → they must interfere with each other.
	m := &mir2.Module{Name: "fib_ig"}
	f := buildFib(m)

	lr := mir2.ComputeLiveness(f)
	ig := mir2.BuildInterferenceGraph(f, lr)

	var headBlock *mir2.Block
	for _, b := range f.Blocks {
		if b.Label == "loop_head" {
			headBlock = b
			break
		}
	}
	if headBlock == nil {
		t.Fatal("loop_head block not found")
	}

	params := headBlock.Params
	if len(params) != 3 {
		t.Fatalf("loop_head has %d params, want 3", len(params))
	}

	// All three block params must pairwise interfere.
	for i, p := range params {
		for j, q := range params {
			if i == j {
				continue
			}
			if !ig.Neighbors(p.Dst).Has(q.Dst) {
				t.Errorf("loop_head block params %%r%d and %%r%d should interfere", p.Dst, q.Dst)
			}
		}
	}
}

// ── Allocator tests ───────────────────────────────────────────────────────────

func TestAllocateSimple(t *testing.T) {
	// fun @id(%r1: u8 [acc]) -> u8
	//   block @entry:
	//     ret %r1
	//
	// Only one register; should get ClassAcc's best location = "A".
	m := &mir2.Module{Name: "id"}
	f := m.AddFunc("id")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("n", mir2.TyU8, mir2.ClassAcc)
	bld.Ret(r1)

	lr := mir2.ComputeLiveness(f)
	result := mir2.Allocate(f, lr, mir2.Z80CostTable{})

	loc := result.Loc(r1)
	if loc.Name != "A" {
		t.Errorf("ClassAcc register should get loc A; got %q", loc.Name)
	}
	if len(result.Spilled) != 0 {
		t.Errorf("unexpected spills: %v", result.Spilled)
	}
}

func TestAllocateNoConflict(t *testing.T) {
	// After allocation, no two interfering registers share the same PhysLoc.
	m := &mir2.Module{Name: "fib_alloc"}
	f := buildFib(m)

	lr := mir2.ComputeLiveness(f)
	ig := mir2.BuildInterferenceGraph(f, lr)
	result := mir2.Allocate(f, lr, mir2.Z80CostTable{})

	// Check every edge in the interference graph.
	info := mir2.CollectRegInfoForTest(f)
	for r, neighbors := range mir2.IGAdjForTest(ig) {
		locR := result.Loc(r)
		neighbors.Each(func(n mir2.Reg) {
			locN := result.Loc(n)
			if locR == locN {
				t.Errorf("interfering regs %%r%d and %%r%d both assigned to %q",
					r, n, locR.Name)
			}
		})
		_ = info
	}
}

func TestAllocateCounterGetsB(t *testing.T) {
	// A ClassCounter register with no interference should get B (cost 0).
	m := &mir2.Module{Name: "counter"}
	f := m.AddFunc("counter")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	cnt := bld.Const(10, mir2.TyU8, mir2.ClassCounter)
	bld.Ret(cnt)

	lr := mir2.ComputeLiveness(f)
	result := mir2.Allocate(f, lr, mir2.Z80CostTable{})

	loc := result.Loc(cnt)
	if loc.Name != "B" {
		t.Errorf("ClassCounter with no interference should get B; got %q", loc.Name)
	}
}

func TestAllocatePointerGetsHL(t *testing.T) {
	// A ClassPointer u16 register with no interference should get HL (cost 0).
	m := &mir2.Module{Name: "ptr"}
	f := m.AddFunc("ptr")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	p := bld.Const(0x8000, mir2.TyU16, mir2.ClassPointer)
	bld.Ret(p)

	lr := mir2.ComputeLiveness(f)
	result := mir2.Allocate(f, lr, mir2.Z80CostTable{})

	loc := result.Loc(p)
	if loc.Name != "HL" {
		t.Errorf("ClassPointer u16 with no interference should get HL; got %q", loc.Name)
	}
}

func TestAllocateFlagGetsCPUFlag(t *testing.T) {
	// A ClassFlag bool register with no interference should get the F location.
	m := &mir2.Module{Name: "flag"}
	f := m.AddFunc("flag")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassFlag}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	r2 := bld.Param("b", mir2.TyU8, mir2.ClassAcc)
	cmp := bld.Cmp(mir2.CmpLt, r1, r2, mir2.ClassFlag, false)
	bld.Ret(cmp)

	lr := mir2.ComputeLiveness(f)
	result := mir2.Allocate(f, lr, mir2.Z80CostTable{})

	loc := result.Loc(cmp)
	if loc.Kind != mir2.LocFlag {
		t.Errorf("ClassFlag bool should get LocFlag; got kind=%d name=%q", loc.Kind, loc.Name)
	}
}

func TestAllocateCoversAllRegs(t *testing.T) {
	// Every virtual register in fibonacci must have a PhysLoc assigned.
	m := &mir2.Module{Name: "fib_complete"}
	f := buildFib(m)

	lr := mir2.ComputeLiveness(f)
	result := mir2.Allocate(f, lr, mir2.Z80CostTable{})

	info := mir2.CollectRegInfoForTest(f)
	for r := range info {
		if _, ok := result.Locs[r]; !ok {
			t.Errorf("%%r%d has no allocated location", r)
		}
	}
}
