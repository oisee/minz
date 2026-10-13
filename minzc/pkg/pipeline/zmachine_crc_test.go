package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestZMachineCRCNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.crc\nfun main() -> u16 { return bank_crc16(7,0,0,65535) }", "crc.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
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
	oracle := func(data []byte, seed uint16) uint16 {
		crc := seed
		for _, b := range data {
			crc ^= uint16(b) << 8
			for i := 0; i < 8; i++ {
				if crc&0x8000 != 0 {
					crc = (crc << 1) ^ 0x1021
				} else {
					crc <<= 1
				}
			}
		}
		return crc
	}
	if oracle([]byte("123456789"), 65535) != 0x29b1 {
		t.Fatal("known CCITT vector")
	}
	cases := 0
	for _, bank := range []int{0, 1, 3, 4, 6, 7} {
		for _, r := range [][2]int{{0, 0}, {16384, 0}, {0, 1}, {16383, 1}, {0, 9}, {255, 257}, {0, 2204}, {14180, 2204}, {0, 16384}} {
			for _, seed := range []uint16{0xffff, 0, 0x29b1} {
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
				copy(m.Memory.RAM[bank][:9], []byte("123456789"))
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x70, false)
				before := m.Memory.RAM
				run := func(offset, count int, initial uint16) uint16 {
					m.Memory.SetPagingForce(0x10)
					m.CPU.SetPC(uint16(a.Symbols["zx__crc__bank_crc16"]))
					m.CPU.SetSP(0xbf00)
					m.CPU.SetAF(uint16(0x10|bank) << 8)
					m.CPU.SetHL(uint16(offset))
					m.CPU.SetDE(uint16(count))
					m.CPU.SetIX(initial)
					m.CPU.SetIFF1(false)
					m.CPU.SetIFF2(false)
					for i := 0; i < 2000000; i++ {
						if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
							break
						}
						m.CPU.DoOpcode()
					}
					if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
						t.Fatalf("bank%d offset%d count%d PC/SP%04x/%04x", bank, offset, count, m.CPU.PC(), m.CPU.SP())
					}
					return m.CPU.HL()
				}
				want := oracle(before[bank][r[0]:r[0]+r[1]], seed)
				if got := run(r[0], r[1], seed); got != want {
					t.Fatalf("bank%d range%v seed%x got%x want%x", bank, r, seed, got, want)
				}
				split := r[1] / 2
				first := run(r[0], split, seed)
				if got := run(r[0]+split, r[1]-split, first); got != want {
					t.Fatal("incremental CRC mismatch")
				}

				if r[1] == 2204 {
					// Actual record layout: checksum excludes its stored bytes 30/31.
					headerCRC := run(r[0], 30, seed)
					got := run(r[0]+32, r[1]-32, headerCRC)
					headerWant := oracle(before[bank][r[0]:r[0]+30], seed)
					recordWant := oracle(before[bank][r[0]+32:r[0]+r[1]], headerWant)
					if got != recordWant {
						t.Fatal("save-header gap CRC mismatch")
					}
				}
				for p := 0; p < 8; p++ {
					for i, b := range before[p] {
						if p == 2 && i >= 0x3ef0 && i < 0x3f00 {
							continue
						}
						if m.Memory.RAM[p][i] != b {
							t.Fatalf("CRC wrote bank%d offset%x", p, i)
						}
					}
				}
				cases++
			}
		}
	}
	t.Logf("%d CRC ranges plus split/incremental checks", cases)
}
