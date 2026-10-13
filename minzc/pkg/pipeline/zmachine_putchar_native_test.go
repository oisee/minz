package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Independent cell/pixel model covers each cursor location, wrap, scroll,
// CR/LF and backspace including the top-left boundary.
func TestZMachinePutcharNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.console\nfun main() { con_putchar(65) }", "putchar.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	pixel := func(row, line, col int) int { return 0x4000 | ((row & 24) << 8) | (line << 8) | ((row & 7) << 5) | col }
	base := make([]byte, 65536)
	for i := range base {
		base[i] = byte(i*31 + 17)
	}
	copy(base[0x8000:], a.Binary)
	for i := 0; i < 768; i++ {
		base[0x5800+i] = byte(i)
	}
	colAddr, rowAddr, attrAddr := a.Symbols["zx__console__con_col"], a.Symbols["zx__console__con_row"], a.Symbols["zx__console__con_attr"]
	target := a.Symbols["zx__console__con_putchar"]
	copy(base[0x7000:], []byte{0xcd, byte(target), byte(target >> 8), 0x76})
	cases := 0
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			for _, ch := range []byte{65, 13, 10, 8} {
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				copy(z.Memory(), base)
				z.Memory()[colAddr] = byte(col)
				z.Memory()[rowAddr] = byte(row)
				z.Memory()[attrAddr] = 7
				want := append([]byte(nil), z.Memory()...)
				r, c := row, col
				draw := func(ch byte) {
					for line := 0; line < 8; line++ {
						want[pixel(r, line, c)] = base[0x3d00+(int(ch)-32)*8+line]
					}
					want[0x5800+r*32+c] = 7
				}
				newline := func() {
					c = 0
					r++
					if r == 24 {
						for y := 0; y < 23; y++ {
							for l := 0; l < 8; l++ {
								for x := 0; x < 32; x++ {
									want[pixel(y, l, x)] = want[pixel(y+1, l, x)]
								}
							}
							copy(want[0x5800+y*32:0x5800+(y+1)*32], want[0x5800+(y+1)*32:0x5800+(y+2)*32])
						}
						for l := 0; l < 8; l++ {
							for x := 0; x < 32; x++ {
								want[pixel(23, l, x)] = 0
							}
						}
						for x := 0; x < 32; x++ {
							want[0x5800+23*32+x] = 7
						}
						r = 23
					}
				}
				switch ch {
				case 13, 10:
					newline()
				case 8:
					if c > 0 {
						c--
						draw(32)
					} else if r > 0 {
						r--
						c = 31
						draw(32)
					}
				default:
					draw(ch)
					c++
					if c == 32 {
						newline()
					}
				}
				want[colAddr] = byte(c)
				want[rowAddr] = byte(r)
				z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: ch})
				minSP := uint16(0xf000)
				for step := 0; step < 100000; step++ {
					if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
						break
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 {
					t.Fatal("return", row, col, ch, z.GetPC(), z.GetSP())
				}
				for i, b := range want {
					if i >= int(minSP) && i < 0xf000 {
						continue
					}
					if z.Memory()[i] != b {
						t.Fatalf("row%d col%d ch%d memory%04x got%02x want%02x", row, col, ch, i, z.Memory()[i], b)
					}
				}
				cases++
			}
		}
	}
	t.Logf("%d cursor/control cases; whole 64KB memory and PC/SP verified", cases)
}
