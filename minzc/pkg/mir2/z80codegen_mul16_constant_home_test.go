package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// deadConstsForFunc elides the RHS load. Its allocated HL is therefore not a
// resident multiplier. Force that nominal home and independently check the
// complete immediate/shift/fallback paths rather than an allocator coincidence.
func TestNativeMul16ElidedConstantHLHome(t *testing.T) {
	values := []uint16{0, 1, 2, 3, 127, 255, 256, 32768, 65534, 65535}
	constants := []int64{0, 1, 2, 3, 5, 6, 9, 10, 12, 32, 255, 256, 257, 4096, 32768, 65535, -1}
	getPair := func(r emulator.Registers, name string) uint16 {
		switch name {
		case "HL":
			return r.HL
		case "DE":
			return r.DE
		case "BC":
			return r.BC
		case "IX":
			return r.IX
		case "IY":
			return r.IY
		}
		panic(name)
	}
	for _, lhs := range []string{"DE", "BC", "IX", "IY"} {
		for _, dst := range []string{"HL", "DE", "BC", "IX", "IY"} {
			for _, cv := range constants {
				t.Run(fmt.Sprintf("%s-constant%d-HL-%s", lhs, cv, dst), func(t *testing.T) {
					inst := &Inst{Op: OpMul, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU16}
					constant := &Inst{Op: OpConst, Dst: 2, Ty: TyU16, Imm: cv}
					f := &Func{Name: "multiply", Contract: Contract{Params: []Param{{Reg: 1, Ty: TyU16}}}, Blocks: []*Block{{Label: "entry", Insts: []*Inst{constant, inst}}}}
					ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: lhs}, 2: {Kind: LocReg, Name: "HL"}, 3: {Kind: LocReg, Name: dst}}}
					var out strings.Builder
					g := &z80cg{sb: &out, fn: f, ar: ar, physOverride: map[Reg]string{}, constVals: map[Reg]int64{2: cv}}
					g.emit("multiply:")
					// No LD HL,cv: match the real folded-constant emission.
					g.genMul16(inst)
					g.emit("RET")
					bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + out.String())
					if err != nil || len(bin.Errors) > 0 {
						t.Fatal(err, out.String())
					}
					for _, a := range values {
						z := emulator.NewRemogattoZ80()
						if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
							t.Fatal(err)
						}
						r := emulator.Registers{PC: 0x8000, SP: 0xeff0, HL: 0xa55a}
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
						want := uint16(uint32(a) * uint32(uint16(cv)))
						if !done || r.SP != 0xeff2 || getPair(r, dst) != want {
							t.Fatalf("%d * %d: got %d want %d PC=%04x SP=%04x\n%s", a, cv, getPair(r, dst), want, r.PC, r.SP, out.String())
						}
					}
				})
			}
		}
	}
}
