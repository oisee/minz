package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestZ80PromoteHighAndLowBytes(t *testing.T) {
	for _, reg := range []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"} {
		for _, value := range []int{0, 1, 16, 128, 255} {
			t.Run(fmt.Sprintf("%s/%d", reg, value), func(t *testing.T) {
				g := &z80cg{holdsPhys: map[string]string{}, sb: &strings.Builder{}}
				pair := g.promote8toPair(reg)
				asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD BC,0xABCD\nLD DE,0xBCEF\nLD HL,0xCDEF\nLD IX,0xDEF0\nLD IY,0xEF01\nLD %s,%d\n", reg, value)
				// Unit exercises the exact emitted instructions, then reads the pair via HL.
				asm += g.sb.String() + "\n"
				if pair != "HL" {
					asm += "PUSH " + pair + "\nPOP HL\n"
				}
				asm += "DI\nHALT\n"
				result, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil || len(result.Errors) > 0 {
					t.Fatal(err, result.Errors, asm)
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				if err = z.LoadMemory(0x8000, result.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
				for n := 0; !z.IsHalted(); n++ {
					if n > 100 {
						t.Fatal("no HALT")
					}
					z.Step()
				}
				if got := z.GetRegisters().HL; got != uint16(value) {
					t.Fatalf("got %04x want %04x\n%s", got, value, asm)
				}
			})
		}
	}
}
