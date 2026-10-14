package pipeline

import (
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
)

func checkedPartitionOracleProblem(t *testing.T, n int) *vir.LocalTableProblem {
	t.Helper()
	f := (&mir2.Module{}).AddFunc("memory_parts")
	b := f.NewBlock("entry")
	for i := 0; i < n; i++ {
		address, value := f.AllocReg(), f.AllocReg()
		b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: address, Ty: mir2.TyPtr, Imm: int64(0x6000 + i)})
		b.Append(&mir2.Inst{Op: mir2.OpLoad, Dst: value, Ty: mir2.TyU8, Src: [2]mir2.Reg{address}})
		b.Append(&mir2.Inst{Op: mir2.OpStore, Ty: mir2.TyU8, Src: [2]mir2.Reg{address, value}})
	}
	b.Seal(&mir2.TermRet{})
	p, e := vir.PrepareLocalTableProblem(f, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func checkedPartitionOracleAssignment(ops []vir.VIROp) map[int]int {
	// Fixtures define every address as canonical ptr and every loaded value as
	// byte. Recovering that type here does not depend on production partitioning.
	assignment := map[int]int{}
	for _, op := range ops {
		for _, v := range append([]int{op.Dst}, op.Src[:]...) {
			if v <= 0 {
				continue
			}
			name := "A"
			if v%2 == 1 {
				name = "HL"
			}
			assignment[v] = vir.Z80.LocByName(name)
		}
	}
	return assignment
}
func TestCheckedCompositionPartitionDomainsOracle(t *testing.T) {
	fixtures, successful := 0, 0
	for n := 1; n <= 3; n++ {
		for mask := 1; mask < 8; mask++ {
			p := checkedPartitionOracleProblem(t, n)
			var pool, manual, automatic vir.RegAllocTable
			pool.Init()
			manual.Init()
			automatic.Init()
			for start := 0; start < len(p.Ops); start++ {
				for length := 1; length <= 3 && start+length <= len(p.Ops); length++ {
					if mask&(1<<(length-1)) == 0 {
						continue
					}
					ops := p.Ops[start : start+length]
					// Malformed local proposals are absent, rather than admitted by a guessed
					// signature. Every retained fragment passes independent current cost checks.
					_ = pool.AddChecked(ops, checkedPartitionOracleAssignment(ops), vir.Z80, p.Options)
				}
			}
			manualCount := 0
			// Enumerate every possible partition of the operation stream independently.
			for mask := 1; mask < (1 << (len(p.Ops) - 1)); mask++ {
				cuts := []int{}
				for end := 1; end < len(p.Ops); end++ {
					if mask&(1<<(end-1)) != 0 {
						cuts = append(cuts, end)
					}
				}
				if len(cuts) > 7 {
					continue
				}
				count, e := pool.ComposeCheckedCandidatesWithMoves(p, &manual, cuts, 1024)
				if e != nil {
					t.Fatal(n, mask, e)
				}
				manualCount += count
			}
			autoCount, e := pool.ComposeCheckedAutomatically(p, &automatic, 1024)
			if e != nil {
				t.Fatal(n, mask, e)
			}
			if (manualCount > 0) != (autoCount > 0) {
				t.Fatal("partition filter lost a feasible full assignment", n, mask, manualCount, autoCount)
			}
			ma, mc, mok := manual.LookupChecked(p.Ops, vir.Z80, p.Options)
			aa, ac, aok := automatic.LookupChecked(p.Ops, vir.Z80, p.Options)
			if mok != aok || mok && (mc != ac || !reflect.DeepEqual(ma, aa)) {
				t.Fatal("different minimum admitted assignment", n, mask, mc, ac, ma, aa)
			}
			if mok {
				successful++
				if _, e = p.Emit(&automatic); e != nil {
					t.Fatal(e)
				}
			}
			fixtures++
		}
	}
	if successful == 0 {
		t.Fatal("oracle had no feasible compositions")
	}
	t.Logf("%d byte-memory pool configurations: full independent cut-mask enumeration agrees on feasibility and best checked assignment; %d feasible", fixtures, successful)
}
