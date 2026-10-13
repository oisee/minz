package mir2

import "testing"

func TestBitCmpKeepsLoadUsedBySuccessor(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("probe")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	p := b.Param("p", TyPtr, ClassPointer)
	v := b.Load(p, TyU8, ClassGeneral)
	mask := b.Const(16, TyU8, ClassGeneral)
	x := b.And(v, mask, TyU8, ClassGeneral)
	zero := b.Const(0, TyU8, ClassGeneral)
	c := b.Cmp(CmpNe, x, zero, ClassFlag, false)
	b.BrIf(c, "yes", nil, "no", nil)
	b.SwitchToNewBlock("yes")
	b.Ret(v)
	b.SwitchToNewBlock("no")
	b.Ret(v)
	g := &z80cg{fn: f, ar: &AllocResult{Locs: map[Reg]PhysLoc{p: {Kind: LocReg, Name: "HL"}, v: {Kind: LocReg, Name: "C"}}}, bitCmpPat: map[Reg]bitCmpPat{}, bitCmpSkip: map[Reg]bool{}}
	g.scanBitCmpPatterns(f.Blocks[0])
	if g.bitCmpSkip[v] {
		t.Fatal("cross-block live load removed")
	}
}

func TestBitCmpMemoryFusionRejectsInterveningStore(t *testing.T) {
	load := &Inst{Op: OpLoad, Dst: 2, Ty: TyU8, Src: [2]Reg{1, 0}}
	store := &Inst{Op: OpStore, Ty: TyU8, Src: [2]Reg{1, 3}}
	and := &Inst{Op: OpAnd, Dst: 4, Ty: TyU8, Src: [2]Reg{2, 5}}
	cmp := &Inst{Op: OpCmp, Dst: 6, Src: [2]Reg{4, 7}}
	b := &Block{Insts: []*Inst{load, store, and, cmp}}
	if bitCmpMemoryAdjacent(b, cmp, bitCmpPat{useMem: true, ptrReg: 1}, []Reg{2, 4}, 7) {
		t.Fatal("load was moved across memory write")
	}
}
