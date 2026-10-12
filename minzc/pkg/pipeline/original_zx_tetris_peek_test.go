package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"testing"
)

// zx_peek is unused by original main: its inline load needs separate evidence.
func TestOriginalZXTetrisUnusedPeekNative(t *testing.T) {
	src, e := os.ReadFile("../../../examples/zx/tetris.nanz")
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/zx-tetris-initial-main/frozen/current.nanz")
	if e != nil || !bytes.Equal(src, frozen) {
		t.Fatal("freeze", e)
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.Parse(string(src), "examples/zx/tetris.nanz")
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || len(image.Errors) > 0 {
				t.Fatal(e, image.Errors)
			}
			entry, ok := image.Symbols["zx_peek"]
			if !ok {
				t.Fatal("missing peek")
			}
			f := s.MIR2Module.FuncByName("zx_peek")
			if f == nil || len(f.Contract.Params) != 1 || s.Allocation.Locs[f.Contract.Params[0].Reg].Name != "HL" || len(f.Contract.Returns) != 1 || mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty) != "A" {
				t.Fatal("unexpected peek ABI", f)
			}
			for _, addr := range []uint16{0, 0x6000, 0x7fff, 0xffff} {
				for value := 0; value < 256; value++ {
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					z.Hooks = nil
					mem := z.Memory()
					copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
					copy(mem[0x8000:], image.Binary)
					mem[addr] = byte(value)
					mem[0xeff0] = 3
					mem[0xeff1] = 0x70
					want := append([]byte(nil), mem...)
					z.SetIOHandlers(func(p uint16) byte { t.Fatal("unexpected IN", p); return 0 }, func(p uint16, v byte) { t.Fatal("unexpected OUT", p, v) })
					z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeff0, HL: addr, A: byte(value) ^ 0xff, F: 0x85, BC: 0x1234, DE: 0x5678, IX: 0x2468, IY: 0x1357})
					done := false
					minSP := uint16(0xeff0)
					for step := 0; step < 1000; step++ {
						if z.GetPC() == 0x7003 && z.GetSP() == 0xeff2 {
							done = true
							break
						}
						if z.GetPC() < 0x8000 || int(z.GetPC()) >= 0x8000+len(image.Binary) {
							t.Fatal("foreign fetch", z.GetPC())
						}
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
					}
					if !done || z.GetRegisters().A != byte(value) {
						t.Fatal("peek", addr, value, z.GetRegisters())
					}
					copy(want[minSP:0xeff0], mem[minSP:0xeff0])
					if !bytes.Equal(want, mem) {
						t.Fatal("peek wrote RAM", addr, value)
					}
				}
			}
			t.Log("1024 native byte reads, HL/A ABI, PC/SP and full RAM PASS")
		})
	}
}
