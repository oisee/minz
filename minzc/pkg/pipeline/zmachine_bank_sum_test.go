package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// A host byte sum is the oracle; every physical RAM byte outside the
// bounded native stack must remain unchanged.
func TestZMachineBankSumNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.banked\nfun main() -> u16 { return bank_sum(0,0,0,0) }", "sum.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + s.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	for _, bank := range []int{0, 1, 3, 4, 6, 7} {
		for _, restore := range []byte{0, 0x10, 0x18, 0x1f} {
			for _, r := range [][2]int{{0, 0}, {0, 1}, {255, 2}, {256, 257}, {0, 16384}, {64, 16320}, {16383, 1}, {16384, 0}} {
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
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x70, false)
				before := m.Memory.RAM
				var want uint16
				for i := r[0]; i < r[0]+r[1]; i++ {
					want += uint16(before[bank][i])
				}
				m.Memory.SetPagingForce(restore)
				m.CPU.SetPC(uint16(a.Symbols["zx__banked__bank_sum"]))
				m.CPU.SetSP(0xbf00)
				m.CPU.SetAF(uint16(byte(bank)|(restore&0x18)) << 8)
				m.CPU.SetHL(uint16(r[0]))
				m.CPU.SetDE(uint16(r[1]))
				m.CPU.SetBC(uint16(restore) << 8)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				for i := 0; i < 300000; i++ {
					if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
						break
					}
					m.CPU.DoOpcode()
				}
				if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.CPU.HL() != want || m.Memory.PagingState() != restore || m.CPU.IFF1() || m.CPU.IFF2() {
					t.Fatalf("bank%d restore%02x offset%d count%d got%04x want%04x PC/SP%04x/%04x", bank, restore, r[0], r[1], m.CPU.HL(), want, m.CPU.PC(), m.CPU.SP())
				}
				// Only the native stack may change; no story/screen/code writes.
				for p := 0; p < 8; p++ {
					for i, v := range before[p] {
						if p == 2 && i >= 0x3efc && i < 0x3f00 {
							continue
						}
						if m.Memory.RAM[p][i] != v {
							t.Fatalf("unexpected RAM write bank%d offset%04x", p, i)
						}
					}
				}
			}
		}
	}
}

func TestZMachineBankRestorePrefixNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.banked\nfun main() { bank_restore_prefix(7,0,0,0x10) }", "restore.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x9000\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	for _, r := range [][2]int{{0, 0}, {0, 1}, {255, 257}, {509, 11288}, {0, 16384}, {16383, 1}, {16384, 0}} {
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
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		before := m.Memory.RAM
		want := before
		copy(want[0][:r[1]], before[7][r[0]:r[0]+r[1]])
		m.Memory.SetPagingForce(0x10)
		m.CPU.SetPC(uint16(a.Symbols["zx__banked__bank_restore_prefix"]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetAF(0x1700)
		m.CPU.SetHL(uint16(r[0]))
		m.CPU.SetDE(uint16(r[1]))
		m.CPU.SetBC(0x1000)
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		for i := 0; i < 500000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || m.CPU.IFF2() {
			t.Fatalf("offset%d count%d PC/SP%04x/%04x", r[0], r[1], m.CPU.PC(), m.CPU.SP())
		}
		for p := 0; p < 8; p++ {
			for i, v := range want[p] {
				if p == 2 && i >= 0x3efc && i < 0x3f00 {
					continue
				}
				if m.Memory.RAM[p][i] != v {
					t.Fatalf("restore unexpected RAM bank%d offset%04x got%02x want%02x", p, i, m.Memory.RAM[p][i], v)
				}
			}
		}
	}
}
