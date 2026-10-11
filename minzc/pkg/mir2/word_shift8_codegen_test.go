package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

// Cross every source/result pair and spill destination with independent word
// arithmetic. A live byte exercises accumulator scratch-save paths.
func TestZ80UnsignedWordByteExtraction(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpShr, mir2.OpDiv} {
		for _, src := range []string{"HL", "DE", "BC", "IX", "IY"} {
			for _, dst := range []string{"HL", "DE", "BC", "IX", "IY", "spill"} {
				for _, value := range []uint16{0, 1, 255, 256, 257, 32767, 32768, 0xABCD, 65535} {
					t.Run(fmt.Sprintf("%s_%s_%s_%04x", op, src, dst, value), func(t *testing.T) {
						m := &mir2.Module{Name: "extract"}
						f := m.AddFunc("extract")
						b := mir2.NewBuilder(f)
						b.SwitchToNewBlock("entry")
						x := b.Param("x", mir2.TyU16, mir2.ClassPair)
						liveA := b.Param("liveA", mir2.TyU8, mir2.ClassAcc)
						k := int64(8)
						if op == mir2.OpDiv {
							k = 256
						}
						count := b.Const(k, mir2.TyU16, mir2.ClassGeneral)
						out := b.BinOp(op, x, count, mir2.TyU16, mir2.ClassPair)
						// Consume live A after extraction and retain it in the return result.
						wide := b.Ext(liveA, mir2.TyU8, mir2.TyU16, mir2.ClassPair)
						result := b.Add(out, wide, mir2.TyU16, mir2.ClassPair)
						b.Ret(result)
						f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
						outLoc := mir2.PhysLoc{Kind: mir2.LocReg, Name: dst}
						if dst == "spill" {
							outLoc = mir2.PhysLoc{Kind: mir2.LocMem}
						}
						// Keep the addition scratch separate from the live source/result.
						scratch := "DE"
						if dst == "DE" {
							scratch = "BC"
						}
						ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: src}, liveA: {Kind: mir2.LocReg, Name: "A"}, count: {Kind: mir2.LocReg, Name: "C"}, out: outLoc, wide: {Kind: mir2.LocReg, Name: scratch}, result: {Kind: mir2.LocReg, Name: "HL"}}}
						asm := "ORG 0x8000\nLD SP,0xFF00\n" + fmt.Sprintf("LD %s,%d\n", src, value) + "LD A,103\nCALL extract\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
						got := fix1Execute(t, asm).HL
						want := (value >> 8) + 103
						if got != want {
							t.Fatalf("got%04x want%04x\n%s", got, want, asm)
						}
						if strings.Contains(asm, "SRL H\n    RR  L\n    SRL H") || strings.Contains(asm, "SRL H\n    RR L\n    SRL H") {
							t.Fatal("byte extraction still emits bit loop")
						}
					})
				}
			}
		}
	}
}

func TestZ80WordByteExtractionPreservesLiveSourceAndPair(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpShr, mir2.OpDiv} {
		m := &mir2.Module{Name: "preserve"}
		f := m.AddFunc("extract")
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		x := b.Param("x", mir2.TyU16, mir2.ClassPair)
		other := b.Param("other", mir2.TyU16, mir2.ClassPair)
		count := int64(8)
		if op == mir2.OpDiv {
			count = 256
		}
		k := b.Const(count, mir2.TyU16, mir2.ClassGeneral)
		out := b.BinOp(op, x, k, mir2.TyU16, mir2.ClassPair)
		sum := b.Add(out, x, mir2.TyU16, mir2.ClassPair)
		result := b.Add(sum, other, mir2.TyU16, mir2.ClassPair)
		b.Ret(result)
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
		ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "DE"}, other: {Kind: mir2.LocReg, Name: "HL"}, k: {Kind: mir2.LocReg, Name: "C"}, out: {Kind: mir2.LocReg, Name: "BC"}, sum: {Kind: mir2.LocReg, Name: "DE"}, result: {Kind: mir2.LocReg, Name: "HL"}}}
		asm := "ORG 0x8000\nLD SP,0xFF00\nLD DE,0xABCD\nLD HL,0x1234\nCALL extract\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
		regs := fix1Execute(t, asm)
		if regs.HL != 0xAB+0xABCD+0x1234 {
			t.Fatalf("%s live source/pair got%04x\n%s", op, regs.HL, asm)
		}
	}
}
