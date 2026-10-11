package mir2

import "testing"

func TestInsertAccSavesModuleRegisterIdentity(t *testing.T) {
	m := &Module{Name: "acc_save_identity"}
	f := m.AddFunc("first")
	a, c := f.AllocReg(), f.AllocReg()
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpConst, Dst: a, Ty: TyU8, Imm: 7}, {Op: OpConst, Dst: c, Ty: TyU8, Imm: 9}}
	b.Term = &TermRet{Vals: []Reg{a}}
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	g := m.AddFunc("second")
	p, v := g.AllocReg(), g.AllocReg()
	g.Contract.Params = []Param{{Name: "ptr", Reg: p, Ty: TyU16, Class: ClassPointer}}
	g.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	gb := g.NewBlock("entry")
	gb.Insts = []*Inst{{Op: OpLoad, Dst: v, Src: [2]Reg{p, NoReg}, Ty: TyU8}}
	gb.Term = &TermRet{Vals: []Reg{v}}
	m.RenumberRegs()
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	a, c = b.Insts[0].Dst, b.Insts[1].Dst
	p, v = g.Contract.Params[0].Reg, gb.Insts[0].Dst
	ar := &AllocResult{Locs: map[Reg]PhysLoc{a: {Kind: LocReg, Name: "A"}, c: {Kind: LocReg, Name: "A"}, p: {Kind: LocReg, Name: "DE"}, v: {Kind: LocReg, Name: "A"}}}
	before := ar.Loc(p)
	InsertAccSaves(f, ar)
	if ar.Loc(p) != before {
		t.Fatalf("foreign pointer overwritten: %v -> %v", before, ar.Loc(p))
	}
	if len(b.Insts) != 3 {
		t.Fatal("save not inserted")
	}
	saved := b.Insts[1].Dst
	if saved == p || saved == v || saved == a || saved == c {
		t.Fatal("synthetic identity collision")
	}
	if b.Term.(*TermRet).Vals[0] != saved {
		t.Fatal("saved return missing")
	}
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	x, y, z := f.AllocReg(), g.AllocReg(), f.AllocReg()
	if !(saved < x && x < y && y < z) {
		t.Fatal("alternating allocations collide")
	}
}
