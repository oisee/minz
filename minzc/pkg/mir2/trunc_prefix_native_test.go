package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// A 16->8 conversion crossing DD/FD prefix groups must leave the main
// accumulator and flags intact when its destination is another register.
func TestTruncPrefixBridgePreservesLiveAccumulatorNative(t *testing.T) {
	for _, src := range []string{"HL", "IX", "IY"} {
		for _, dst := range []string{"H", "L", "IXH", "IXL", "IYH", "IYL"} {
			if src == "HL" && (dst == "H" || dst == "L") || src == "IX" && strings.HasPrefix(dst, "IX") || src == "IY" && strings.HasPrefix(dst, "IY") {
				continue
			}
			f := &Func{Name: "trunc_live"}
			b := f.NewBlock("entry")
			inst := &Inst{Op: OpTrunc, Dst: 2, Src: [2]Reg{1}, Ty: TyU8, SrcTy: TyU16}
			b.Insts = []*Inst{inst}
			b.Term = &TermRet{Vals: []Reg{2, 3}}
			var out strings.Builder
			g := &z80cg{fn: f, curBlock: b, sb: &out, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: src}, 2: {Kind: LocReg, Name: dst}, 3: {Kind: LocReg, Name: "A"}}}, regInfo: map[Reg]RegInfo{1: {Ty: TyU16}, 2: {Ty: TyU8}, 3: {Ty: TyU8}}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, constVals: map[Reg]int64{}, liveness: ComputeLiveness(f)}
			g.genInst(inst)
			image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + out.String() + "RET\n")
			if e != nil {
				t.Fatal(e)
			}
			if len(image.Errors) > 0 {
				t.Fatal(image.Errors)
			}
			z := emulator.NewRemogattoZ80()
			if e := z.LoadMemory(0x8000, image.Binary); e != nil {
				t.Fatal(e)
			}
			for _, word := range []uint16{0, 1, 127, 128, 255, 256, 0x1234, 0xffff} {
				for _, a := range []byte{0, 1, 0x55, 0xff} {
					for _, flags := range []byte{0, 1, 0x45, 0xff} {
						r := emulator.Registers{PC: 0x8000, SP: 0xeffe, A: a, F: flags, BC: 0xb1c2, DE: 0xd3e4, HL: 0x5678, IX: 0x9abc, IY: 0xdef0}
						switch src {
						case "HL":
							r.HL = word
						case "IX":
							r.IX = word
						case "IY":
							r.IY = word
						}
						want := r
						lo := uint16(byte(word))
						switch dst {
						case "H":
							want.HL = lo<<8 | want.HL&255
						case "L":
							want.HL = want.HL&0xff00 | lo
						case "IXH":
							want.IX = lo<<8 | want.IX&255
						case "IXL":
							want.IX = want.IX&0xff00 | lo
						case "IYH":
							want.IY = lo<<8 | want.IY&255
						case "IYL":
							want.IY = want.IY&0xff00 | lo
						}
						z.Memory()[0xeffe] = 0
						z.Memory()[0xefff] = 0xf1
						z.SetRegisters(r)
						for n := 0; n < 100 && z.GetPC() != 0xf100; n++ {
							z.Step()
						}
						got := z.GetRegisters()
						if got.PC != 0xf100 || got.SP != 0xf000 || got.A != want.A || got.F != want.F || got.BC != want.BC || got.DE != want.DE || got.HL != want.HL || got.IX != want.IX || got.IY != want.IY {
							t.Fatal(fmt.Sprintf("%s->%s word%04x A%02x F%02x: got%v want%v\n%s", src, dst, word, a, flags, got, want, out.String()))
						}
					}
				}
			}
		}
	}
}
