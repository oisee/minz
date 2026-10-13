package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestCheckedCompositionRevalidatesWholeFunction(t *testing.T) {
	f, allocation := productionTableFunction(mir2.ClassAcc)
	p, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	var pool, target RegAllocTable
	pool.Init()
	target.Init()
	// Two smaller fragments: constant, then actual return bridge and RET.
	for _, ops := range [][]VIROp{p.Ops[:1], p.Ops[1:]} {
		assignment := map[int]int{}
		for v := range buildProblem(ops, Z80).vregs {
			assignment[v] = Z80.LocByName("A")
		}
		if err = pool.AddChecked(ops, assignment, Z80, p.Options); err != nil {
			t.Fatal(err)
		}
	}
	n, err := pool.ComposeCheckedCandidates(p, &target, []int{1}, 16)
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err = p.Emit(&target); err != nil {
		t.Fatal(err)
	}
	// A claimed negative cost is irrelevant; proposals are fully readmitted.
	for _, entry := range pool.entries {
		entry.Cost = -1000
	}
	var again RegAllocTable
	again.Init()
	if n, err = pool.ComposeCheckedCandidates(p, &again, []int{1}, 16); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	// Different return ABI invalidates the composition even if fragments match.
	f.Contract.Returns[0].Class = mir2.ClassRegC
	q, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	var refused RegAllocTable
	refused.Init()
	if n, err = pool.ComposeCheckedCandidates(q, &refused, []int{1}, 16); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	for _, cuts := range [][]int{{0}, {len(p.Ops)}, {2, 1}} {
		if _, err = pool.ComposeCheckedCandidates(p, &refused, cuts, 16); err == nil {
			t.Fatal("bad partition accepted")
		}
	}
}

func TestCheckedCompositionRejectsCrossBoundaryLiveClobber(t *testing.T) {
	f, allocation := productionTableFunction(mir2.ClassAcc)
	p, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	// Each fragment alone permits reusing A. Across the boundary v1 must survive
	// the definition of v2: full admission must reject, even when local admission passes.
	p.Ops = []VIROp{
		{Op: OpConst, Dst: 1, Imm: 7, Width: 8},
		{Op: OpConst, Dst: 2, Imm: 9, Width: 8},
		{Op: OpMove, Dst: 3, Src: [2]int{1, -1}, Width: 8},
		{Op: OpRet, Src: [2]int{3, -1}, Width: 8},
	}
	var pool, target RegAllocTable
	pool.Init()
	target.Init()
	for _, ops := range [][]VIROp{p.Ops[:1], p.Ops[1:2], p.Ops[2:]} {
		assignment := map[int]int{}
		for v := range buildProblem(ops, Z80).vregs {
			assignment[v] = Z80.LocByName("A")
		}
		if err = pool.AddChecked(ops, assignment, Z80, p.Options); err != nil {
			t.Fatal(err)
		}
	}
	complete := map[int]int{1: Z80.LocByName("A"), 2: Z80.LocByName("A"), 3: Z80.LocByName("A")}
	if _, err := checkedLocalCost(p.Ops, complete, Z80, p.Options); err == nil {
		t.Fatal("full admission missed cross-boundary live clobber")
	}
	// Composition additionally refuses the altered prepared operation list.
	n, err := pool.ComposeCheckedCandidates(p, &target, []int{1, 2}, 16)
	if err == nil || n != 0 || target.Size() != 0 {
		t.Fatal("cross-boundary clobber admitted", n, err)
	}
	// A work-limit error is atomic, including an existing target entry.
	original, err := PrepareLocalTableProblem(f, allocation)
	if err != nil {
		t.Fatal(err)
	}
	good := map[int]int{}
	for v := range buildProblem(original.Ops, Z80).vregs {
		good[v] = Z80.LocByName("A")
	}
	if err = target.AddChecked(original.Ops, good, Z80, original.Options); err != nil {
		t.Fatal(err)
	}
	before := target.Size()
	if _, err = pool.ComposeCheckedCandidates(p, &target, []int{1}, 1); err == nil || target.Size() != before {
		t.Fatal("limit failure mutated target", err)
	}
}
