package mir2

import (
	"strings"
	"testing"
)

func TestAccumulatorPreservationExhaustedScratchNative(t *testing.T) {
	for _, mode := range []string{"operand", "overwrite"} {
		t.Run(mode, func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("exhausted")
			f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassAcc}}
			inst := &Inst{Op: OpAdd, Dst: 2, Src: [2]Reg{6, 7}, Ty: TyU8}
			b := &Block{Label: "entry", Insts: []*Inst{inst}, Term: &TermRet{Vals: []Reg{1, 3, 4, 5, 8, 9}}}
			f.Blocks = []*Block{b}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: "A"}, 2: {Name: "A"}, 3: {Name: "BC"}, 4: {Name: "DE"}, 5: {Name: "HL"}, 6: {Name: "B"}, 7: {Name: "C"}, 8: {Name: "IX", Kind: LocIXY}, 9: {Name: "IY", Kind: LocIXY}}}
			var out strings.Builder
			g := &z80cg{fn: f, mod: m, curBlock: b, ar: ar, sb: &out, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
			if mode == "operand" {
				g.saveAccOperandIfLive(1, inst)
			} else {
				g.saveABeforeOverwrite(inst)
			}
			asm := "ORG 0x8000\nLD SP,0xff00\nLD A,37\nLD DE,0x1234\n" + out.String() + "\nLD A,0\nLD A,(_spill_exhausted_r1)\nHALT\n_spill_exhausted_r1: DB 0\n"
			got := runExternJudge(t, asm)
			if got.A != 37 {
				t.Fatalf("saved accumulator lost: got %04x", got.A)
			}
			if got.DE != 0x1234 {
				t.Fatalf("live DE corrupted: got %04x want1234\n%s", got.DE, asm)
			}
			if g.loc(1) != "_spill_exhausted_r1" {
				t.Fatalf("exhaustion must use spill, got %s", g.loc(1))
			}
		})
	}
}
