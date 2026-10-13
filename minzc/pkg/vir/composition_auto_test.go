package vir

import (
	"fmt"
	"math"
	"reflect"
	"sort"
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

func TestCompositionAutomaticPartitionsIndependentOracle(t *testing.T) {
	cases := 0
	for n := 2; n <= 10; n++ {
		for mask := 0; mask < 8; mask++ {
			for withMoves := 0; withMoves < 2; withMoves++ {
				ops := make([]VIROp, n)
				for i := range ops {
					ops[i] = VIROp{Op: OpAdd, Dst: i + 2, Src: [2]int{i + 1, -1}, Width: 8}
				}
				if withMoves == 1 {
					for i := 1; i < n; i += 3 {
						ops[i].Op = OpMove
					}
				}
				var lengths []int
				for k := 1; k <= 3; k++ {
					if mask&(1<<(k-1)) != 0 {
						lengths = append(lengths, k)
					}
				}
				got, err := discoverCompositionPartitions(ops, lengths, MaxLegacyCandidates)
				if err != nil {
					t.Fatal(n, mask, withMoves, err)
				}
				// Enumerate every possible cut mask independently. Each segment
				// needs a proposed length or a real singleton move, at most 8 pieces.
				var want []string
				for cutsMask := 1; cutsMask < (1 << (n - 1)); cutsMask++ {
					var cuts []int
					for i := 1; i < n; i++ {
						if cutsMask&(1<<(i-1)) != 0 {
							cuts = append(cuts, i)
						}
					}
					if len(cuts) > 7 {
						continue
					}
					start, valid := 0, true
					for _, end := range append(append([]int(nil), cuts...), n) {
						length := end - start
						proposed := length <= 3 && mask&(1<<(length-1)) != 0
						bridge := length == 1 && ops[start].Op == OpMove
						if !proposed && !bridge {
							valid = false
							break
						}
						start = end
					}
					if valid {
						want = append(want, fmt.Sprint(cuts))
					}
				}
				var actual []string
				for _, cuts := range got {
					actual = append(actual, fmt.Sprint(cuts))
				}
				sort.Strings(want)
				sort.Strings(actual)
				if !reflect.DeepEqual(want, actual) {
					t.Fatal(n, mask, withMoves, want, actual)
				}
				cases++
			}
		}
	}
	t.Logf("%d operation/length configurations: all bounded partitions equal independent cut-mask oracle", cases)
}
