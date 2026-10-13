package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestZMachineRLEPackNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.rle\nglobal data:[u8;1]\nfun main() -> u16 { return bank_pack_rle(7, &data,0,0,0) }", "pack.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	// Independent greedy host encoder. Native bytes must match, including ties.
	pack := func(data []byte) []byte {
		var out []byte
		for i := 0; i < len(data); {
			run := 1
			for i+run < len(data) && data[i+run] == data[i] && run < 130 {
				run++
			}
			if run >= 3 {
				out = append(out, byte(128+run-3), data[i])
				i += run
				continue
			}
			start := i
			i += run
			for i < len(data) && i-start < 128 {
				run = 1
				for i+run < len(data) && data[i+run] == data[i] && run < 130 {
					run++
				}
				if run >= 3 {
					break
				}
				n := run
				if n > 128-(i-start) {
					n = 128 - (i - start)
				}
				i += n
			}
			out = append(out, byte(i-start-1))
			out = append(out, data[start:i]...)
		}
		return out
	}
	cases := 0
	for _, length := range []int{0, 1, 2, 3, 4, 127, 128, 129, 130, 131, 255, 256, 1024} {
		for mode := 0; mode < 4; mode++ {
			data := make([]byte, length)
			for i := range data {
				switch mode {
				case 0:
					data[i] = 0x5a
				case 1:
					data[i] = byte(i)
				case 2:
					data[i] = byte(i / 2)
				case 3:
					if i%257 < 133 {
						data[i] = 17
					} else {
						data[i] = byte(i)
					}
				}
			}
			want := pack(data)
			for _, capacity := range []int{0, max(0, len(want)-1), len(want), len(want) + 1} {
				for _, offset := range []int{0, 16384 - capacity} {
					for _, bank := range []int{0, 1, 3, 4, 6, 7} {
						m, err := spectrum.NewPentagon128(nil, nil)
						if err != nil {
							t.Fatal(err)
						}
						for p := 0; p < 8; p++ {
							for i := range m.Memory.RAM[p] {
								m.Memory.RAM[p][i] = byte(i*37 + p*17)
							}
						}
						for i, b := range a.Binary {
							m.Memory.Write(0x9000+uint16(i), b, false)
						}
						for i, b := range data {
							m.Memory.Write(0x6000+uint16(i), b, false)
						}
						m.Memory.Write(0xbf00, 0, false)
						m.Memory.Write(0xbf01, 0x70, false)
						before := m.Memory.RAM
						m.Memory.SetPagingForce(0x10)
						m.CPU.SetPC(uint16(a.Symbols["zx__rle__bank_pack_rle"]))
						m.CPU.SetSP(0xbf00)
						m.CPU.SetAF(uint16(0x10|bank) << 8)
						m.CPU.SetHL(0x6000)
						m.CPU.SetDE(uint16(length))
						m.CPU.SetIX(uint16(offset))
						m.CPU.SetIY(uint16(capacity))
						m.CPU.SetIFF1(false)
						m.CPU.SetIFF2(false)
						for i := 0; i < 500000; i++ {
							if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
								break
							}
							m.CPU.DoOpcode()
						}
						expected := uint16(len(want))
						if len(want) > capacity {
							expected = 65535
						}
						if m.CPU.HL() != expected || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
							t.Fatalf("len%d mode%d cap%d off%d got%d want%d PC/SP%04x/%04x", length, mode, capacity, offset, m.CPU.HL(), expected, m.CPU.PC(), m.CPU.SP())
						}
						if expected != 65535 && !bytes.Equal(m.Memory.RAM[bank][offset:offset+len(want)], want) {
							t.Fatalf("encoding mismatch len%d mode%d cap%d got%x want%x", length, mode, capacity, m.Memory.RAM[bank][offset:offset+len(want)], want)
						}
						for p := 0; p < 8; p++ {
							for i, b := range before[p] {
								if p == bank && i >= offset && i < offset+capacity {
									continue
								}
								if p == 2 && i >= 0x3ee0 && i < 0x3f00 {
									continue
								}
								// Three declared bytes of resident encoder scratch.
								adr := uint16(0x8000 + i)
								scratch := uint16(a.Symbols["zx__rle__rle_begin"])
								literal := uint16(a.Symbols["zx__rle__rle_literals"])
								if p == 2 && ((adr >= scratch && adr < scratch+2) || adr == literal) {
									continue
								}
								if m.Memory.RAM[p][i] != b {
									t.Fatalf("out of bounds write bank%d off%04x", p, i)
								}
							}
						}
						cases++
					}
				}
			}
		}
	}
	t.Logf("%d bounded encoding cases", cases)
}
