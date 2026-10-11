package mir2

import "testing"

func TestAccSaveScratchProtectsPairAndForeignIDs(t *testing.T) {
	f := &Func{Name: "standalone"}
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8}, {Op: OpConst, Dst: 2, Ty: TyU8}, {Op: OpConst, Dst: 3, Ty: TyU8}, {Op: OpConst, Dst: 4, Ty: TyU8}, {Op: OpLoad, Dst: 5, Src: [2]Reg{6, NoReg}, Ty: TyU8}}
	b.Term = &TermRet{Vals: []Reg{1}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "A"}, 2: {Kind: LocReg, Name: "B"}, 3: {Kind: LocReg, Name: "C"}, 4: {Kind: LocReg, Name: "B"}, 5: {Kind: LocReg, Name: "C"}, 6: {Kind: LocReg, Name: "DE"}}}
	r := pickAccSaveScratch(f, b, 0, ar, 1)
	if r <= 6 {
		t.Fatalf("foreign allocation identity overwritten: %d", r)
	}
	if ar.Loc(r).Name != "H" {
		t.Fatalf("future pair clobbered by scratch %v", ar.Loc(r))
	}
	if ar.Loc(6).Name != "DE" {
		t.Fatal("foreign pointer changed")
	}
}

func TestAccSaveRenameLifetimeAndCanonicalOperands(t *testing.T) {
	b := &Block{Insts: []*Inst{{Op: OpConst, Dst: 2}, {Op: OpAsm, Asm: &AsmExtra{Ins: []Reg{1}}}, {Op: OpMove, Dst: 1, Src: [2]Reg{1, NoReg}}, {Op: OpMove, Dst: 3, Src: [2]Reg{1, NoReg}}}, Term: &TermRet{Vals: []Reg{1}}}
	renameUses(b, 0, 1, 9)
	if b.Insts[1].Asm.Ins[0] != 9 || b.Insts[2].Src[0] != 9 {
		t.Fatal("old value uses not renamed")
	}
	if b.Insts[3].Src[0] != 1 || b.Term.(*TermRet).Vals[0] != 1 {
		t.Fatal("new definition renamed")
	}
	for _, term := range []Term{&TermDJNZ{Counter: 1, BodyArgs: []Reg{1}, ExitArgs: []Reg{1}}, &TermBrIf2{Lhs: 1, Rhs: 1, EqArgs: []Reg{1}, LtArgs: []Reg{1}, GtArgs: []Reg{1}}} {
		renameTermReg(term, 1, 9)
		for _, r := range term.termUses() {
			if r != 9 {
				t.Fatal("terminator use not renamed")
			}
		}
	}
	for _, inst := range []*Inst{{ExtraRets: []Reg{1}}, {Asm: &AsmExtra{Outs: []Reg{1}}}} {
		b := &Block{Insts: []*Inst{{}, inst, {Op: OpMove, Src: [2]Reg{1, NoReg}}}, Term: &TermRet{Vals: []Reg{1}}}
		renameUses(b, 0, 1, 9)
		if b.Insts[2].Src[0] != 1 || b.Term.(*TermRet).Vals[0] != 1 {
			t.Fatal("extra definition not respected")
		}
		if isUsedAfterIdx(b, 0, 1) {
			t.Fatal("redefined value considered live")
		}
	}
}
