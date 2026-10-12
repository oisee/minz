package mir2

import "testing"

func TestTSMCSpillCrossCall(t *testing.T) {
	for _, op := range []Op{OpCall, OpCallIndirect, OpCallCond, OpAsm} {
		for _, successor := range []bool{false, true} {
			m := &Module{}
			f := m.AddFunc("caller")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			r := b.Const(7, TyU8, ClassGeneral)
			b.Cur.Append(&Inst{Op: op, Dst: NoReg, Sym: "other", Ty: TyVoid})
			if successor {
				b.Jmp("exit")
				b.SwitchToNewBlock("exit")
			}
			copy := b.Move(r, TyU8, ClassAcc)
			b.Ret(copy)
			ar := &AllocResult{Locs: map[Reg]PhysLoc{r: {Kind: LocMem}, copy: {Kind: LocReg, Name: "A"}}}
			if pairs := scanTSMCSpillPairs(f, ar); len(pairs) != 0 {
				t.Fatalf("op%v successor%v: cross-call home patched", op, successor)
			}
		}
	}
}

func TestTSMCSpillNewResultAndDeadArgumentRemainEligible(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("caller")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	r := b.Call("other", nil, TyU8, ClassGeneral, CallAttrs{})
	copy := b.Move(r, TyU8, ClassAcc)
	b.Ret(copy)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{r: {Kind: LocMem}, copy: {Kind: LocReg, Name: "A"}}}
	if pairs := scanTSMCSpillPairs(f, ar); len(pairs) != 1 || pairs[0].reg != r {
		t.Fatal("new call result unnecessarily disabled")
	}
	// A home passed to a call but never used afterward needs no snapshot.
	f = m.AddFunc("last_argument")
	b = NewBuilder(f)
	b.SwitchToNewBlock("entry")
	r = b.Const(9, TyU8, ClassGeneral)
	b.CallVoid("other", []Reg{r}, CallAttrs{})
	b.Ret()
	ar = &AllocResult{Locs: map[Reg]PhysLoc{r: {Kind: LocMem}}}
	if pairs := scanTSMCSpillPairs(f, ar); len(pairs) != 1 {
		t.Fatal("last-use argument unnecessarily disabled")
	}
}
