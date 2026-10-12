package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each physical byte has an independent accessor; expected writes are applied
// to the original register state rather than interpreting emitted instructions.
func TestAdapterMixedCopiesNative(t *testing.T) {
	bytes := []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"}
	pairs := []string{"BC", "DE", "HL", "IX", "IY"}
	read := func(r emulator.Registers, n string) uint16 {
		switch n {
		case "A":
			return uint16(r.A)
		case "BC":
			return r.BC
		case "DE":
			return r.DE
		case "HL":
			return r.HL
		case "IX":
			return r.IX
		case "IY":
			return r.IY
		}
		var v uint16
		switch n[:1] {
		case "B", "C":
			v = r.BC
		case "D", "E":
			v = r.DE
		case "H", "L":
			v = r.HL
		case "I":
			if strings.HasPrefix(n, "IX") {
				v = r.IX
			} else {
				v = r.IY
			}
		}
		if n == "B" || n == "D" || n == "H" || strings.HasSuffix(n, "H") {
			return v >> 8
		}
		return v & 255
	}
	write := func(r *emulator.Registers, n string, v uint16) {
		if n == "A" {
			r.A = byte(v)
			return
		}
		var p *uint16
		switch n {
		case "BC", "B", "C":
			p = &r.BC
		case "DE", "D", "E":
			p = &r.DE
		case "HL", "H", "L":
			p = &r.HL
		case "IX", "IXH", "IXL":
			p = &r.IX
		case "IY", "IYH", "IYL":
			p = &r.IY
		}
		if len(n) == 2 {
			*p = v
			return
		}
		if n == "B" || n == "D" || n == "H" || strings.HasSuffix(n, "H") {
			*p = (*p & 255) | (v&255)<<8
		} else {
			*p = (*p & 0xff00) | (v & 255)
		}
	}
	for _, b := range bytes {
		for _, p := range pairs {
			for _, ends := range [][2]string{{b, p}, {p, b}} {
				from, to := ends[0], ends[1]
				t.Run(fmt.Sprintf("%s_to_%s", from, to), func(t *testing.T) {
					asm := "ORG 0x8000\n" + strings.Join(emitAdapterMove(Z80.Locs[Z80.LocByName(from)], Z80.Locs[Z80.LocByName(to)]), "\n") + "\nRET\n"
					ar, err := z80asm.NewAssembler().AssembleString(asm)
					if err != nil || len(ar.Errors) > 0 {
						t.Fatalf("assembly %v %v: %s", err, ar.Errors, asm)
					}
					if dir := os.Getenv("VIR_MIXED_ADAPTER_EVIDENCE"); dir != "" {
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						name := from + "_to_" + to
						if err := os.WriteFile(filepath.Join(dir, name+".a80"), []byte(asm), 0644); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(dir, name+".bin"), ar.Binary, 0644); err != nil {
							t.Fatal(err)
						}
					}
					z := emulator.NewRemogattoZ80()
					if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
						t.Fatal(err)
					}
					limit := 256
					if Z80.Locs[Z80.LocByName(from)].Width == 16 {
						limit = 65536
					}
					for x := 0; x < limit; x++ {
						in := emulator.Registers{A: byte(x*31 + 1), F: 0x45, BC: uint16(x*257 + 13), DE: uint16(x*509 + 27), HL: uint16(x*997 + 39), IX: uint16(x*2053 + 51), IY: uint16(x*4093 + 63), PC: 0x8000, SP: 0xeffe}
						want := in
						write(&want, to, read(in, from))
						want.PC = 0xf100
						want.SP = 0xf000
						if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
							t.Fatal(err)
						}
						z.SetRegisters(in)
						for i := 0; i < 64 && z.GetRegisters().PC != 0xf100; i++ {
							z.Step()
						}
						if got := z.GetRegisters(); got != want {
							t.Fatalf("x=%d got=%+v want=%+v ASM=%s", x, got, want, asm)
						}
					}
				})
			}
		}
	}
}
