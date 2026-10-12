package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestLiteralOverwriteNativeState(t *testing.T) {
	for _, body := range []string{
		"LD A, 102\nLD C, 110\n; call argument rematerialization\nLD C, 102\nLD A, 110\n",
		"LD B, 0\nLD D, 7\nLD B, 255\nLD E, 11\nLD D, 19\n",
		"LD H, 3\nLD L, 5\nLD H, 9\nLD L, 17\nLD H, 255\n",
	} {
		optimized := strings.Join(elimOverwrittenByteLiterals(strings.Split(body, "\n")), "\n")
		if optimized == body {
			t.Fatal("fixture did not fold")
		}
		binaries := make([][]byte, 2)
		for i, asm := range []string{body, optimized} {
			b, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm + "RET\n")
			if e != nil || len(b.Errors) > 0 {
				t.Fatal(e, b.Errors)
			}
			binaries[i] = b.Binary
		}
		if len(binaries[1]) >= len(binaries[0]) {
			t.Fatal("no assembled size reduction")
		}
		for x := 0; x < 256; x++ {
			var final [2]emulator.Registers
			for i, bin := range binaries {
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				if e := z.LoadMemory(0x8000, bin); e != nil {
					t.Fatal(e)
				}
				if e := z.LoadMemory(0xe000, []byte{0, 0x70}); e != nil {
					t.Fatal(e)
				}
				r := checksumCopyRegisters()
				r.F = byte(x)
				r.A = byte(x)
				r.BC = uint16(x) * 257
				r.DE = uint16(x) * 193
				r.HL = uint16(x) * 71
				r.SP = 0xe000
				r.PC = 0x8000
				z.SetRegisters(r)
				for steps := 0; steps < 30 && z.GetRegisters().PC != 0x7000; steps++ {
					z.Step()
				}
				final[i] = z.GetRegisters()
				if final[i].PC != 0x7000 || final[i].SP != 0xe002 || final[i].F != r.F || final[i].IX != r.IX || final[i].IY != r.IY {
					t.Fatalf("boundary %+v", final[i])
				}
			}
			if final[0] != final[1] {
				t.Fatalf("x%d original%+v optimized%+v", x, final[0], final[1])
			}
		}
	}
}

func TestLiteralOverwriteBarriers(t *testing.T) {
	for _, barrier := range []string{"label:", "CALL helper", "LD B, A", "LD HL, 1234", "ADD A, B", "EXX", "LD (HL), A", "LD A, symbol", "LD IXH, 3", "DB 0", "LD A, -1"} {
		src := "LD A, 7\n" + barrier + "\nLD A, 11\n"
		if got := strings.Join(elimOverwrittenByteLiterals(strings.Split(src, "\n")), "\n"); got != src {
			t.Fatalf("crossed %q barrier: %s", barrier, got)
		}
	}
}
