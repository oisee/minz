package mir2

import (
	"fmt"
	"strings"
	"testing"
)

// Every primitive that reads or writes a byte or word spill slot must leave
// A and F as it found them: callers do not know whether A holds a live value.
// Each case runs the emitted code on the emulator with A=$A5, F=$C3 and checks
// the moved value, both flag/accumulator bytes, and the slot neighbours.
func TestSpillTransitPreservesAccumulatorAndFlags(t *testing.T) {
	const (
		b1 = "_spill_t_r1" // u8 slot, initial $3C
		w2 = "_spill_t_r2" // u16 slot, initial $FFFF
		b3 = "_spill_t_r3" // u8 slot after b1, initial $77
		w4 = "_spill_t_r4" // u16 slot, initial $1234
	)
	cases := []struct {
		name  string
		setup string
		emit  func(g *z80cg)
		check string // loads the observed value into IY (A and F untouched)
		want  uint16
	}{
		{"emitMov spill->C", "LD C,0", func(g *z80cg) { g.emitMov("C", b1, 8) }, "LD B,0\nPUSH BC\nPOP IY", 0x3C},
		{"emitMov E->spill", "LD E,$5E", func(g *z80cg) { g.emitMov(b1, "E", 8) }, "LD IY,(" + b1 + ")", 0x775E},
		{"loadSpill8 B", "LD B,0", func(g *z80cg) { g.loadSpill8("B", b1) }, "LD C,B\nLD B,0\nPUSH BC\nPOP IY", 0x3C},
		{"storeSpill8 D", "LD D,$D1", func(g *z80cg) { g.storeSpill8(b1, "D") }, "LD IY,(" + b1 + ")", 0x77D1},
		{"storeSpill8 IXH", "LD IX,$9900", func(g *z80cg) { g.storeSpill8(b1, "IXH") }, "LD IY,(" + b1 + ")", 0x7799},
		{"emitLD8 spill->spill", "", func(g *z80cg) { g.emitLD8(b3, b1) }, "LD IY,(" + b1 + ")", 0x3C3C},
		{"emitLD8 spill->HL widen", "LD HL,$FFFF", func(g *z80cg) { g.emitLD8("HL", b1) }, "PUSH HL\nPOP IY", 0x003C},
		{"emitMovViaAltA H<-IXL", "LD IX,$0042", func(g *z80cg) { g.emitMovViaAltA("H", "IXL") }, "LD L,0\nPUSH HL\nPOP IY", 0x4200},
		{"emitSingleCopy u8 C->i16 spill", "LD C,$7B", func(g *z80cg) { g.emitSingleCopy("C", w2, TyI16) }, "LD IY,(" + w2 + ")", 0x007B},
		{"emitSingleCopy spill->B u8", "LD B,0", func(g *z80cg) { g.emitSingleCopy(b1, "B", TyU8) }, "LD C,B\nLD B,0\nPUSH BC\nPOP IY", 0x3C},
		{"emitSingleCopy word spill->C (truncate)", "LD C,0", func(g *z80cg) { g.emitSingleCopy(w4, "C", TyU16) }, "LD B,0\nPUSH BC\nPOP IY", 0x34},
		{"loadSpill16 byte home", "LD DE,$FFFF", func(g *z80cg) { g.loadSpill16("DE", b1) }, "PUSH DE\nPOP IY", 0x003C},
		{"emitMov C->word spill", "LD C,$C7", func(g *z80cg) { g.emitMov(w2, "C", 16) }, "LD IY,(" + w2 + ")", 0x00C7},
		{"emitMov C->byte home (word move)", "LD C,$C7", func(g *z80cg) { g.emitMov(b1, "C", 16) }, "LD IY,(" + b1 + ")", 0x77C7},
		{"emitMov HL->byte home", "LD HL,$ABCD", func(g *z80cg) { g.emitMov(b1, "HL", 16) }, "LD IY,(" + b1 + ")", 0x77CD},
		{"emitMov word spill->byte home", "", func(g *z80cg) { g.emitMov(b1, w4, 16) }, "LD IY,(" + b1 + ")", 0x7734},
		{"emitMov byte home->word spill", "", func(g *z80cg) { g.emitMov(w2, b1, 16) }, "LD IY,(" + w2 + ")", 0x003C},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &Func{Name: "t"}
			b := &Block{Label: "entry"}
			f.Blocks = []*Block{b}
			var out strings.Builder
			g := &z80cg{fn: f, curBlock: b, sb: &out, ar: &AllocResult{Locs: map[Reg]PhysLoc{}},
				physOverride: map[Reg]string{}, holdsPhys: map[string]string{},
				regInfo: map[Reg]RegInfo{1: {Ty: TyU8}, 2: {Ty: TyU16}, 3: {Ty: TyU8}, 4: {Ty: TyU16}}}
			tc.emit(g)
			if g.err != nil {
				t.Fatal(g.err)
			}
			asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nLD BC,$A5C3\nPUSH BC\nPOP AF\n%s\n%s\n%s\nHALT\n"+
				"%s: DB $3C\n%s: DB $77\n%s: DW $FFFF\n%s: DW $1234\n", tc.setup, out.String(), tc.check, b1, b3, w2, w4)
			got := runExternJudge(t, asm)
			if got.A != 0xA5 || got.F != 0xC3 {
				t.Fatalf("A/F not preserved: A=%02x F=%02x\n%s", got.A, got.F, asm)
			}
			if got.IY != tc.want {
				t.Fatalf("moved value: got %04x want %04x\n%s", got.IY, tc.want, asm)
			}
		})
	}
}
