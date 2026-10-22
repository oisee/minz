package mir2

import "testing"

func TestNaturalLoopFactsIndependentOfLayout(t *testing.T) {
	_, f := invariantFixture()
	// Layout is deliberately unrelated to dominance/backedge order.
	f.Blocks[1], f.Blocks[3] = f.Blocks[3], f.Blocks[1]
	loops := AnalyzeNaturalLoops(f)
	if len(loops) != 1 {
		t.Fatalf("loops = %d", len(loops))
	}
	l := loops[0]
	if l.Header.Label != "head" || l.Preheader.Label != "entry" || l.Latch.Label != "body" || len(l.Blocks) != 2 || len(l.Exits) != 1 {
		t.Fatalf("incorrect CFG facts: %+v", l)
	}
	if l.Invariant[4] != 1 || len(l.Loads) != 1 || len(l.Stores) != 0 || l.Opaque {
		t.Fatalf("incorrect memory/invariance facts: %+v", l)
	}
	if len(l.Inductions) != 1 || l.Inductions[0].Param.Dst != 5 || l.Inductions[0].Step != 1 || l.Inductions[0].Initial != 3 {
		t.Fatalf("incorrect affine recurrence: %+v", l.Inductions)
	}
	if len(l.LiveOut) != 1 || !l.LiveOut[5] {
		t.Fatalf("live-outs: %v", l.LiveOut)
	}
	f.BlockByLabel("body").Insts = append(f.BlockByLabel("body").Insts, &Inst{Op: OpCall, Sym: "opaque", Ty: TyVoid})
	if !AnalyzeNaturalLoops(f)[0].Opaque {
		t.Fatal("call effects lost")
	}
}
