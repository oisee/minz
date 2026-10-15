package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// Exhaust every byte for each index half and shift direction. Native output
// must assemble, preserve the other half and AF, and return with exact PC/SP.
func TestIndexHalfConstantShiftNative(t *testing.T) {
	for _, dst := range []string{"IXH", "IXL", "IYH", "IYL"} {
		for _, mnemonic := range []string{"SLA", "SRL", "SRA"} {
			for _, count := range []int{0, 1, 3, 7, 8, 15} {
				var out strings.Builder
				g := &z80cg{sb: &out,
					ar:           &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocIXY, Name: dst}, 2: {Kind: LocReg, Name: "C"}, 3: {Kind: LocIXY, Name: dst}}},
					physOverride: map[Reg]string{}, holdsPhys: map[string]string{},
					constVals: map[Reg]int64{2: int64(count)}, regInfo: map[Reg]RegInfo{1: {Ty: TyU8}, 2: {Ty: TyU8}, 3: {Ty: TyU8}}}
				g.genShift(mnemonic, &Inst{Op: OpShr, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8})
				body := out.String()
				assembly, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + body + "\nRET\n")
				if err != nil || len(assembly.Errors) != 0 {
					t.Fatalf("%s %s %d: %v %v\n%s", dst, mnemonic, count, err, assembly.Errors, body)
				}
				for value := 0; value < 256; value++ {
					z := emulator.NewRemogattoZ80()
					if err := z.LoadMemory(0x8000, assembly.Binary); err != nil {
						t.Fatal(err)
					}
					r := emulator.Registers{PC: 0x8000, SP: 0xff00, A: 0x5a, F: 0x45, IX: 0x1234, IY: 0x5678}
					if dst == "IXH" {
						r.IX = uint16(value)<<8 | 0x34
					} else if dst == "IXL" {
						r.IX = 0x1200 | uint16(value)
					} else if dst == "IYH" {
						r.IY = uint16(value)<<8 | 0x78
					} else {
						r.IY = 0x5600 | uint16(value)
					}
					z.Memory()[0xff00], z.Memory()[0xff01] = 0x00, 0x70
					z.SetRegisters(r)
					for n := 0; z.GetPC() != 0x7000 && n < 100; n++ {
						z.Step()
					}
					want := byte(value << count)
					if mnemonic == "SRL" {
						want = byte(value >> count)
					} else if mnemonic == "SRA" {
						want = byte(int8(value) >> count)
					}
					expected := r
					if dst == "IXH" {
						expected.IX = uint16(want)<<8 | 0x34
					} else if dst == "IXL" {
						expected.IX = 0x1200 | uint16(want)
					} else if dst == "IYH" {
						expected.IY = uint16(want)<<8 | 0x78
					} else {
						expected.IY = 0x5600 | uint16(want)
					}
					got := z.GetRegisters()
					if got.IX != expected.IX || got.IY != expected.IY || got.A != r.A || got.F != r.F || got.PC != 0x7000 || got.SP != 0xff02 {
						t.Fatalf("%s %s %d value %02x: %s\n%s", dst, mnemonic, count, value, fmt.Sprint(got), body)
					}
				}
			}
		}
	}
}
