package mir2

import (
	"strings"
	"testing"
)

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

// Fusion commits to skipping the mask and zero. With A, DE and HL live,
// ordinary CP preservation chooses an index half, which BIT cannot address.
// The fused test must run before that preservation can relocate its source.
func TestBitCmpLiveAccumulatorFusion(t *testing.T) {
	for _, bitget := range []bool{false, true} {
		for _, cond := range []CmpCond{CmpEq, CmpNe} {
			m := &Module{}
			f := m.AddFunc("probe")
			f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			x := b.Param("x", TyU8, ClassAcc)
			p := b.Param("p", TyU16, ClassPair)
			q := b.Param("q", TyU16, ClassPointer)
			var mask, test Reg
			if bitget {
				test = b.BitGet(x, 0, TyU8, ClassGeneral)
			} else {
				mask = b.Const(1, TyU8, ClassGeneral)
				test = b.And(x, mask, TyU8, ClassGeneral)
			}
			zero := b.Const(0, TyU8, ClassGeneral)
			c := b.Cmp(cond, test, zero, ClassFlag, false)
			b.BrIf(c, "yes", nil, "no", nil)
			b.SwitchToNewBlock("yes")
			xe := b.Ext(x, TyU8, TyU16, ClassPair)
			r := b.Add(p, xe, TyU16, ClassPointer)
			b.Ret(r)
			b.SwitchToNewBlock("no")
			xe2 := b.Ext(x, TyU8, TyU16, ClassPair)
			r2 := b.Add(q, xe2, TyU16, ClassPointer)
			b.Ret(r2)
			ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
			for reg, name := range map[Reg]string{x: "A", p: "DE", q: "HL", mask: "B", test: "C", zero: "B", c: "F", xe: "BC", xe2: "BC", r: "HL", r2: "HL"} {
				ar.Locs[reg] = PhysLoc{Kind: LocReg, Name: name}
			}
			asm, e := Z80Codegen(m, ar)
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(asm, "BIT 0, A") {
				t.Fatalf("bitget=%t cond=%v: fused producers skipped without BIT:\n%s", bitget, cond, asm)
			}
		}
	}
}
