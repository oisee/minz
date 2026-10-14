package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestZXDirectPrimitivesNative(t *testing.T) {
	const source = `import zx.sprite_address
import zx.tile_cell
fun down(@z80_hl address:u16)->u16 {return sprite_next_line(address)}
fun fill(@z80_hl source:^u8,@z80_de dst:^u8) {tile_row_fill(source,dst)}`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "direct-primitives.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for mode, opt := range []Options{{AssertMode: "none", ContractOpt: true}, {AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			steps, err := CompileHIRSteps(parse(), opt)
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil || len(a.Errors) != 0 {
				t.Fatal(err, a.Errors)
			}
			m, err := spectrum.New48K(make([]byte, 16384))
			if err != nil {
				t.Fatal(err)
			}
			for at := 0x4000; at < 65536; at++ {
				m.Memory.Write(uint16(at), byte(at*37+at/256+11), false)
			}
			for i, b := range a.Binary {
				m.Memory.Write(0x8000+uint16(i), b, false)
			}
			call := func(name string, hl, de uint16) {
				m.CPU.SetHL(hl)
				m.CPU.SetDE(de)
				m.CPU.SetPC(uint16(a.Symbols[name]))
				m.CPU.SetSP(0xeff0)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				m.Memory.Write(0xeff0, 0, false)
				m.Memory.Write(0xeff1, 0x70, false)
				for count := 0; m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xeff2; count++ {
					if count > 10000 {
						t.Fatalf("%s did not return: PC=%04x SP=%04x", name, m.CPU.PC(), m.CPU.SP())
					}
					m.CPU.DoOpcode()
					if m.CPU.SP() < 0xefb0 {
						t.Fatal("stack exceeded 64-byte bound")
					}
				}
				if m.CPU.IFF1() || m.CPU.IFF2() {
					t.Fatal("primitive changed interrupt mode")
				}
			}
			vms := []*mir2.VM{mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(steps.MIR2Module)}
			// Oracle is raster coordinates, independent of the bit-field DOWN_HL code.
			address := func(x, y int) uint16 { return uint16(0x4000 + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + x) }
			for y := 0; y < 191; y++ {
				for x := 0; x < 32; x++ {
					input, want := address(x, y), address(x, y+1)
					call("down", input, 0)
					if got := m.CPU.HL(); got != want {
						t.Fatalf("down(%d,%d)=%04x want%04x", x, y, got, want)
					}
					for _, vm := range vms {
						got, err := vm.Call("down", []mir2.Value{{I: int64(input)}})
						if err != nil || len(got) != 1 || got[0].I != int64(want) {
							t.Fatalf("VM down(%d,%d): %v %v", x, y, got, err)
						}
					}
				}
			}
			// Every ten-column occupancy mask; cycle all three screen thirds, the
			// first/last character rows, and both left/right legal column boundaries.
			rows := []int{0, 7, 8, 15, 16, 23}
			for mask := 0; mask < 1024; mask++ {
				x := 0
				if mask%2 != 0 {
					x = 22
				}
				y := rows[(mask/2)%len(rows)] * 8
				dst := address(x, y)
				for col := 0; col < 10; col++ {
					value := byte(0)
					if mask&(1<<col) != 0 {
						value = byte(1 + (mask+col*17)%255)
					}
					m.Memory.Write(0xc000+uint16(col), value, false)
				}
				m.Memory.Write(0xeff0, 0, false)
				m.Memory.Write(0xeff1, 0x70, false)
				want := make([]byte, 65536)
				for at := range want {
					want[at] = m.Memory.ReadByteInternal(uint16(at))
				}
				for col := 0; col < 10; col++ {
					for line := 0; line < 8; line++ {
						value := byte(0)
						if mask&(1<<col) != 0 {
							value = 255
						}
						want[address(x+col, y+line)] = value
					}
				}
				call("fill", 0xc000, dst)
				for at, value := range want {
					if at >= 0xefb0 && at < 0xeff0 {
						continue
					}
					if got := m.Memory.ReadByteInternal(uint16(at)); got != value {
						t.Fatalf("mask%03x (%d,%d) RAM%04x=%02x want%02x", mask, x, y, at, got, value)
					}
				}
			}
			t.Log("6112 DOWN_HL raster positions: native/raw/final VM; 1024 tile masks: full native RAM, PC/SP and INT PASS")
		})
	}
}
