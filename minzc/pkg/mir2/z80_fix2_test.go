package mir2

import (
	"strings"
	"testing"
)

func fix2CG(inst *Inst, names map[Reg]string, later ...*Inst) *z80cg {
	b := &Block{Label: "entry", Insts: append([]*Inst{inst}, later...)}
	f := &Func{Name: "arith", Blocks: []*Block{b}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
	for r, n := range names {
		ar.Locs[r] = PhysLoc{Name: n}
	}
	return &z80cg{sb: &strings.Builder{}, fn: f, curBlock: b, ar: ar, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}, cmpSwapped: map[Reg]bool{}, cmpNeedsTwo: map[Reg]bool{}}
}

func TestFix2SelectedScratchSaves(t *testing.T) {
	t.Run("add does not write live DE", func(t *testing.T) {
		i := &Inst{Op: OpAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
		g := fix2CG(i, map[Reg]string{1: "HL", 2: "DE", 3: "HL"})
		g.curBlock.Term = &TermRet{Vals: []Reg{2, 3}}
		g.genBinOp("ADD", i)
		if strings.Contains(g.sb.String(), "PUSH") {
			t.Fatal(g.sb.String())
		}
	})
	t.Run("carry clear does not overwrite live accumulator", func(t *testing.T) {
		i := &Inst{Op: OpSub, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
		g := fix2CG(i, map[Reg]string{1: "HL", 2: "DE", 3: "HL", 4: "A"})
		g.curBlock.Term = &TermRet{Vals: []Reg{3, 4}}
		g.genBinOp("SUB", i)
		if strings.Contains(g.sb.String(), "PUSH") {
			t.Fatal(g.sb.String())
		}
	})
	t.Run("later definitions do not require saves", func(t *testing.T) {
		i := &Inst{Op: OpAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
		next := &Inst{Op: OpConst, Dst: 4, Ty: TyU16, Imm: 123}
		g := fix2CG(i, map[Reg]string{1: "HL", 2: "DE", 3: "BC", 4: "HL"}, next)
		g.curBlock.Term = &TermRet{Vals: []Reg{3, 4}}
		g.genBinOp("ADD", i)
		if strings.Contains(g.sb.String(), "POP HL") {
			t.Fatal(g.sb.String())
		}
	})
	t.Run("power of two does not write live pairs", func(t *testing.T) {
		i := &Inst{Op: OpDiv, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
		g := fix2CG(i, map[Reg]string{1: "D", 2: "C", 3: "A", 4: "HL", 5: "BC"})
		g.constVals[2] = 2
		g.curBlock.Term = &TermRet{Vals: []Reg{3, 4, 5}}
		g.genDivMod(i)
		if strings.Contains(g.sb.String(), "PUSH") {
			t.Fatal(g.sb.String())
		}
	})
	t.Run("compare does not write live DE or BC", func(t *testing.T) {
		i := &Inst{Op: OpCmp, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyBool, Cond: CmpUlt, FlagsOnly: true}
		g := fix2CG(i, map[Reg]string{1: "HL", 2: "DE", 3: "F", 4: "BC"})
		g.curBlock.Term = &TermRet{Vals: []Reg{2, 3, 4}}
		g.genCmp16(i)
		if strings.Contains(g.sb.String(), "PUSH") {
			t.Fatal(g.sb.String())
		}
	})
	t.Run("dead comparison lhs needs no restoration", func(t *testing.T) {
		i := &Inst{Op: OpCmp, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyBool, Cond: CmpEq}
		g := fix2CG(i, map[Reg]string{1: "HL", 2: "DE", 3: "F"})
		g.curBlock.Term = &TermRet{Vals: []Reg{3}}
		g.genCmp16(i)
		if strings.Contains(g.sb.String(), "ADD HL") || strings.Contains(g.sb.String(), "PUSH") {
			t.Fatal(g.sb.String())
		}
	})
	t.Run("cross block live HL still saved", func(t *testing.T) {
		i := &Inst{Op: OpAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
		g := fix2CG(i, map[Reg]string{1: "HL", 2: "DE", 3: "BC"})
		g.curBlock.Term = &TermJmp{Target: "exit"}
		g.fn.Blocks = append(g.fn.Blocks, &Block{Label: "exit", Term: &TermRet{Vals: []Reg{1, 3}}})
		g.liveness = ComputeLiveness(g.fn)
		g.genBinOp("ADD", i)
		if !strings.Contains(g.sb.String(), "PUSH HL") || !strings.Contains(g.sb.String(), "POP HL") {
			t.Fatal(g.sb.String())
		}
	})
}

func TestFix2BytePointerOffset(t *testing.T) {
	// D overlaps the high byte of the offset pair: copy D to E before zeroing D.
	i := &Inst{Op: OpPtrAdd, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyPtr}
	g := fix2CG(i, map[Reg]string{1: "HL", 2: "D", 3: "HL"})
	g.genInst(i)
	asm := g.sb.String()
	if !strings.Contains(asm, "LD E, D\n    LD D, 0") || strings.Contains(asm, "PUSH") {
		t.Fatal(asm)
	}
}
