package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// A multiplier resident in HL must survive staging the multiplicand.
// Force physical homes rather than depending on a particular allocator choice.
func TestNativeMul16HLSourceOverlap(t *testing.T) {
	values := []uint16{0, 1, 2, 3, 127, 255, 256, 32768, 65534, 65535}
	for _, lhs := range []string{"DE", "BC", "IX", "IY"} {
		for _, dst := range []string{"HL", "DE", "BC", "IX", "IY"} {
			t.Run(lhs+"-HL-"+dst, func(t *testing.T) {
				inst := &Inst{Op: OpMul, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
				f := &Func{Name: "multiply", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU16}, {Reg: 2, Ty: TyU16}}}, Blocks: []*Block{{Label: "entry", Insts: []*Inst{inst}}}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: lhs}, 2: {Kind: LocReg, Name: "HL"}, 3: {Kind: LocReg, Name: dst}}}
				var out strings.Builder
				g := &z80cg{sb: &out, fn: f, ar: ar, physOverride: map[Reg]string{}, constVals: map[Reg]int64{}}
				g.emit("multiply:")
				g.genMul16(inst)
				g.emit("RET")
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + out.String())
				if e != nil {
					t.Fatal(e, out.String())
				}
				if len(bin.Errors) > 0 {
					t.Fatal(bin.Errors, out.String())
				}
				for _, a := range values {
					for _, b := range values {
						z := emulator.NewRemogattoZ80()
						if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
							t.Fatal(err)
						}
						r := emulator.Registers{PC: 0x8000, SP: 0xeff0, HL: b}
						switch lhs {
						case "DE":
							r.DE = a
						case "BC":
							r.BC = a
						case "IX":
							r.IX = a
						case "IY":
							r.IY = a
						}
						z.Memory()[0xeff0] = 0
						z.Memory()[0xeff1] = 0x70
						z.SetRegisters(r)
						done := false
						for n := 0; n < 10000; n++ {
							if z.GetRegisters().PC == 0x7000 {
								done = true
								break
							}
							z.Step()
						}
						r = z.GetRegisters()
						var got uint16
						switch dst {
						case "HL":
							got = r.HL
						case "DE":
							got = r.DE
						case "BC":
							got = r.BC
						case "IX":
							got = r.IX
						case "IY":
							got = r.IY
						}
						if !done || r.SP != 0xeff2 || got != uint16(uint32(a)*uint32(b)) {
							t.Fatal(fmt.Sprintf("%d * %d: got %d PC=%04x SP=%04x", a, b, got, r.PC, r.SP), out.String())
						}
					}
				}
			})
		}
	}
}
