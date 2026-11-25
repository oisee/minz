package mir2

import (
	"strings"
	"testing"
)

func useCountCacheFunc() *Func {
	return &Func{Name: "ucc", Blocks: []*Block{{Label: "entry", Insts: []*Inst{
		{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 3},
		{Op: OpAdd, Dst: 2, Src: [2]Reg{1, 1}, Ty: TyU8},
	}, Term: &TermRet{Vals: []Reg{2}}}}}
}

// Outside genFunc nothing is cached: a mutation between two queries is seen.
func TestFnUseCountsOutsideGenFuncRecounts(t *testing.T) {
	f := useCountCacheFunc()
	g := &z80cg{fn: f}
	if n := g.fnUseCounts()[1]; n != 2 {
		t.Fatalf("uses of r1 = %d, want 2", n)
	}
	f.Blocks[0].Insts[1].Src[1] = NoReg
	if n := g.fnUseCounts()[1]; n != 1 {
		t.Fatalf("after mutation uses of r1 = %d, want 1 (stale cache)", n)
	}
}

// genFunc caches use counts only while it runs: after it returns, and on the
// next genFunc of the same generator, a mutation made in between is seen.
func TestFnUseCountsCacheEndsWithGenFunc(t *testing.T) {
	f := useCountCacheFunc()
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "A"}, 2: {Kind: LocReg, Name: "A"}}}
	g := &z80cg{mod: &Module{Funcs: []*Func{f}}, fn: f, ar: ar, sb: &strings.Builder{}}
	g.genFunc(f)
	if g.err != nil {
		t.Fatal(g.err)
	}
	if g.useCountsOn || g.useCounts != nil {
		t.Fatal("use-count cache still active after genFunc returned")
	}
	// Populate a cache as genFunc would, then let genFunc start afresh.
	g.useCountsOn, g.useCounts = true, nil
	if n := g.fnUseCounts()[1]; n != 2 {
		t.Fatalf("uses of r1 = %d, want 2", n)
	}
	f.Blocks[0].Insts[1].Src[1] = NoReg
	g.genFunc(f) // must reset the cache on entry and clear it on return
	if n := g.fnUseCounts()[1]; n != 1 {
		t.Fatalf("after mutation and genFunc, uses of r1 = %d, want 1 (stale cache)", n)
	}
}
