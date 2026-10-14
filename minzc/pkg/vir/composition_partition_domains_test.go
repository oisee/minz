package vir

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func partitionMemoryFixture(t *testing.T, n int) *LocalTableProblem {
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
	p, e := PrepareLocalTableProblem(f, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func partitionAssignment(ops []VIROp) map[int]int {
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
			assignment[v] = Z80.LocByName(name)
		}
	}
	return assignment
}
func TestCompositionPartitionDomainsAvoidSpuriousSingletons(t *testing.T) {
	p := partitionMemoryFixture(t, 5)
	var pool, target RegAllocTable
	pool.Init()
	target.Init()
	for start := 0; start < len(p.Ops)-1; start += 3 {
		ops := p.Ops[start : start+3]
		if e := pool.AddChecked(ops, partitionAssignment(ops), Z80, p.Options); e != nil {
			t.Fatal(e)
		}
	}
	// A void RET entry has one op and zero vregs: it cannot match arbitrary
	// singleton CONST/LOAD/STORE operations, despite its length matching them.
	tail := p.Ops[len(p.Ops)-1:]
	if e := pool.AddChecked(tail, map[int]int{}, Z80, p.Options); e != nil {
		t.Fatal(e)
	}
	if _, e := discoverCompositionPartitions(p.Ops, []int{1, 3}, 64); e == nil {
		t.Fatal("fixture must exceed old length-only partition bound")
	}
	n, e := pool.ComposeCheckedAutomatically(p, &target, 64)
	if e != nil || n == 0 {
		t.Fatal("necessary-domain filter did not recover valid composition", n, e)
	}
	asm, e := p.Emit(&target)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("length-only exceeds 64; admitted-domain discovery emits one checked assignment: %s", asm)
}
