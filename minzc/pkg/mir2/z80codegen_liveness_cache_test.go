package mir2

import (
	"strings"
	"testing"
)

// r1 is defined in entry and read in "use"; whether it is live after the
// call in entry depends on that read. Removing the read between two queries
// must change the answer.
func livenessCacheFunc() (*Func, *Inst) {
	call := &Inst{Op: OpCall, Sym: "ext", Ty: TyVoid}
	return &Func{Name: "lcc", Blocks: []*Block{
		{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8, Imm: 3}, call}, Term: &TermJmp{Target: "use"}},
		{Label: "use", Term: &TermRet{Vals: []Reg{1}}},
	}}, call
}

// Outside genFunc regsLiveAfterInst recomputes liveness on every call.
func TestRegsLiveAfterInstOutsideGenFuncRecomputes(t *testing.T) {
	f, call := livenessCacheFunc()
	g := &z80cg{fn: f, curBlock: f.Blocks[0]}
	if !g.regsLiveAfterInst(call)[1] {
		t.Fatal("r1 should be live after the call")
	}
	f.Blocks[1].Term = &TermRet{}
	if g.regsLiveAfterInst(call)[1] {
		t.Fatal("r1 still live after its only read was removed (stale liveness)")
	}
}

func emitLivenessCacheFunc(g *z80cg, f *Func) string {
	var sb strings.Builder
	g.sb = &sb
	g.genFunc(f)
	return sb.String()
}

func newLivenessCacheGen(f *Func) *z80cg {
	ext := &Func{Name: "ext", Attrs: FuncAttrs{IsExtern: true}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "A"}}}
	return &z80cg{mod: &Module{Funcs: []*Func{f, ext}}, fn: f, ar: ar}
}

// genFunc caches liveness only while it runs. A generator that still holds
// liveness of the unmutated function must not use it when it emits the
// mutated one: the caller save around the call (PUSH AF / POP AF while r1
// is live in A) has to disappear exactly as with a fresh generator.
func TestRegsLiveAfterInstCacheEndsWithGenFunc(t *testing.T) {
	f, call := livenessCacheFunc()
	g := newLivenessCacheGen(f)
	before := emitLivenessCacheFunc(g, f)
	if !strings.Contains(before, "PUSH AF") {
		t.Fatalf("expected a caller save of live r1:\n%s", before)
	}
	if g.useCountsOn {
		t.Fatal("liveness cache still active after genFunc returned")
	}
	// Leave a populated cache behind, as if emission had been interrupted.
	g.curBlock, g.useCountsOn = f.Blocks[0], true
	if !g.regsLiveAfterInst(call)[1] {
		t.Fatal("r1 should be live after the call")
	}
	f.Blocks[1].Term = &TermRet{}
	got := emitLivenessCacheFunc(g, f)
	want := emitLivenessCacheFunc(newLivenessCacheGen(f), f)
	if got != want || strings.Contains(got, "PUSH AF") {
		t.Fatalf("stale liveness reached emission:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
