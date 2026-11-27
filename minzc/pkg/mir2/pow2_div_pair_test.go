package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// An unsigned word division by a power of two whose quotient lives in BC or
// DE shifts that pair in place; it no longer stages the dividend through HL.
// Every source pair, both destinations and several divisors run natively.
func TestPow2DivIntoPairInPlaceNative(t *testing.T) {
	for _, lhs := range []string{"HL", "DE", "BC"} {
		for _, dst := range []string{"BC", "DE"} {
			for _, k := range []int64{1, 2, 4, 16, 256, 32768} {
				name := fmt.Sprintf("%s_to_%s_div%d", lhs, dst, k)
				m := &Module{}
				f := m.AddFunc("q")
				b := f.NewBlock("entry")
				f.Contract.Params = []Param{{Reg: 1, Ty: TyU16, Class: ClassPair}}
				f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
				b.Insts = []*Inst{{Op: OpConst, Dst: 2, Imm: k, Ty: TyU16}, {Op: OpDiv, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}}
				b.Term = &TermRet{Vals: []Reg{3}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: lhs}, 2: {Kind: LocRemat, Name: RematLocName}, 3: {Name: dst}}}
				asm, err := Z80Codegen(m, ar)
				if err != nil {
					t.Fatal(name, err)
				}
				if lhs != "HL" && strings.Contains(asm, "SRL H") {
					t.Fatalf("%s: dividend staged through HL\n%s", name, asm)
				}
				bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL q\nDI\nHALT\n" + asm)
				if err != nil || len(bin.Errors) > 0 {
					t.Fatal(name, err, bin.Errors)
				}
				z := emulator.NewRemogattoZ80()
				if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
					t.Fatal(err)
				}
				for _, x := range []uint16{0, 1, 2, 3, 255, 256, 257, 0x7fff, 0x8000, 0x8001, 0xabcd, 0xfffe, 0xffff} {
					r := emulator.Registers{A: 0x5a, BC: 0x1111, DE: 0x2222, HL: 0x3333, SP: 0xf000, PC: 0x8000}
					switch lhs {
					case "HL":
						r.HL = x
					case "DE":
						r.DE = x
					case "BC":
						r.BC = x
					}
					z.Unhalt()
					z.SetRegisters(r)
					for i := 0; i < 400 && !z.IsHalted(); i++ {
						z.Step()
					}
					got := z.GetRegisters()
					if got.HL != x/uint16(k) || got.SP != 0xf000 {
						t.Fatalf("%s x=%04x: HL=%04x want %04x\n%s", name, x, got.HL, x/uint16(k), asm)
					}
				}
			}
		}
	}
}
