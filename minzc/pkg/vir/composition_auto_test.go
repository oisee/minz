package vir

import (
	"math"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestCheckedCompositionAutomaticFindsMovePartition(t *testing.T) {
	f, a := productionTableFunction(mir2.ClassAcc)
	p, err := PrepareLocalTableProblem(f, a)
	if err != nil {
		t.Fatal(err)
	}
	var pool, target RegAllocTable
	pool.Init()
	target.Init()
	if err := pool.AddChecked(p.Ops[:1], map[int]int{p.Ops[0].Dst: Z80.LocByName("C")}, Z80, p.Options); err != nil {
		t.Fatal(err)
	}
	count, err := pool.ComposeCheckedAutomatically(p, &target, 32)
	if err != nil || count == 0 {
		t.Fatal(count, err)
	}
	asm, err := p.Emit(&target)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("automatic constant/move/return partition", asm)
	before := target.Size()
	// Even an overflow-sized untrusted proposal must be a clean miss rather
	// than a negative slice endpoint during partition discovery.
	pool.entries["oversized"] = &RegAllocEntry{Schema: checkedTableSchema, CostModel: checkedTableCostModel, NOps: math.MaxInt, Assignment: []int{Z80.LocByName("A")}}
	if _, err := pool.ComposeCheckedAutomatically(p, &target, 32); err != nil {
		t.Fatal(err)
	}
	delete(pool.entries, "oversized")
	if _, err := pool.ComposeCheckedAutomatically(p, &target, 1); err == nil || target.Size() != before {
		t.Fatal("partition bound changed target", err)
	}
	p.Ops = nil
	if _, err := pool.ComposeCheckedAutomatically(p, &target, 32); err == nil || target.Size() != before {
		t.Fatal("empty forged problem accepted", err)
	}
}
