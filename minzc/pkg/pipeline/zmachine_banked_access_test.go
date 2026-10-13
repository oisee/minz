package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestZXBankedMemoryNative(t *testing.T) {
	src := `import zx.banked
 fun read_test() -> u8 { return bank_read(3, 0x123, 0x10) }
 fun write_test() -> void { bank_write(3,0x123,0x5a,0x10) }
 `
	h, err := nanz.ParseWithOpts(src, "bank.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x8000+uint16(i), b, false)
	}
	m.Memory.RAM[3][0x123] = 0xab
	for _, name := range []string{"read_test", "write_test"} {
		m.Memory.SetPagingForce(0x10)
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		m.CPU.SetPC(uint16(a.Symbols[name]))
		m.CPU.SetSP(0xbf00)
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		for i := 0; i < 1000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 {
			t.Fatalf("return PC/SP %04x/%04x", m.CPU.PC(), m.CPU.SP())
		}
		if m.Memory.PagingState() != 0x10 {
			t.Fatal("paging restoration", m.Memory.PagingState())
		}
		if name == "read_test" && byte(m.CPU.AF()>>8) != 0xab {
			t.Fatalf("read %02x", m.CPU.AF()>>8)
		}
		if name == "write_test" && m.Memory.RAM[3][0x123] != 0x5a {
			t.Fatalf("write %02x", m.Memory.RAM[3][0x123])
		}
	}
}

// Raw imported ABI is executed on physical bank arrays. Code and stack stay
// below C000; each call must restore ROM/screen/paging bits and leave IFF false.
func TestZXBankedMemoryAllPagesNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.banked\nfun main() -> u8 { bank_write(0,0,0,0) return bank_read(0,0,0) }", "bank-all.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
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
	for bank := 0; bank < 8; bank++ {
		for _, offset := range []int{0, 1, 255, 256, 16382, 16383} {
			for _, restore := range []byte{0, 0x10, 0x18, 0x1f} {
				m, err := spectrum.NewPentagon128(nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				for page := 0; page < 8; page++ {
					for i := range m.Memory.RAM[page] {
						m.Memory.RAM[page][i] = byte(i*37 + page*17)
					}
				}
				for i, b := range a.Binary {
					m.Memory.Write(0x9000+uint16(i), b, false)
				}
				// Destination on page2 can alias resident code: these offsets are chosen
				// outside its instruction bytes. Page5 offsets near zero are screen RAM.
				expected := m.Memory.RAM[bank][offset]
				for _, op := range []string{"read", "write"} {
					m.Memory.SetPagingForce(restore)
					m.Memory.Write(0xbf00, 0, false)
					m.Memory.Write(0xbf01, 0x70, false)
					before := m.Memory.RAM
					m.CPU.SetPC(uint16(a.Symbols["zx__banked__bank_"+op]))
					m.CPU.SetSP(0xbf00)
					m.CPU.SetAF(uint16((restore&0x18)|byte(bank)) << 8)
					m.CPU.SetHL(uint16(offset))
					m.CPU.SetDE(0x5a00 | uint16(restore))
					m.CPU.SetIFF1(false)
					m.CPU.SetIFF2(false)
					for i := 0; i < 100; i++ {
						if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
							break
						}
						m.CPU.DoOpcode()
					}
					if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.CPU.IFF1() || m.CPU.IFF2() {
						t.Fatalf("return/IFF bank%d offset%d op%s PC=%04x SP=%04x entry=%04x", bank, offset, op, m.CPU.PC(), m.CPU.SP(), a.Symbols["zx__banked__bank_"+op])
					}
					if m.Memory.PagingState() != restore {
						t.Fatal("paging restored incorrectly")
					}
					if op == "read" && byte(m.CPU.AF()>>8) != expected {
						t.Fatal("read wrong physical bank byte")
					}
					for page := 0; page < 8; page++ {
						for i, want := range before[page] {
							if op == "write" && page == bank && i == offset {
								want = 0x5a
							}
							if m.Memory.RAM[page][i] != want {
								t.Fatalf("bank%d offset%d op%s modified page%d[%04x]", bank, offset, op, page, i)
							}
						}
					}
				}
			}
		}
	}
}
