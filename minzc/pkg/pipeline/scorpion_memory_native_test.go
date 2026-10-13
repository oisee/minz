package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestScorpionLibraryNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.scorpion_memory\nfun main()->u8 { scorpion_write(13,123,167,16,0) return scorpion_read(13,123,16,0) }", "scorpion.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
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
	cases := 0
	for page := 0; page < 16; page++ {
		for _, offset := range []uint16{0, 1, 255, 256, 16382, 16383} {
			for _, prior := range []uint16{0x0010, 0x101f, 0x0218} {
				m, err := spectrum.NewScorpionMemory256(nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				for i, b := range a.Binary {
					m.Memory.Write(0x8000+uint16(i), b, false)
				}
				p7, p1 := byte(prior), byte(prior>>8)
				m.Memory.SetPaging(p7)
				m.Memory.SetScorpionPaging(p1)
				m.Memory.WriteRAMDirect(page, offset, 0xa7)
				for _, name := range []string{"zx__scorpion_memory__scorpion_read", "zx__scorpion_memory__scorpion_write"} {
					m.Memory.Write(0xbf00, 0, false)
					m.Memory.Write(0xbf01, 0x70, false)
					m.CPU.SetPC(uint16(a.Symbols[name]))
					m.CPU.SetSP(0xbf00)
					m.CPU.SetAF(uint16(page) << 8)
					m.CPU.SetHL(offset)
					m.CPU.SetDE(uint16(p1)<<8 | uint16(p7))
					m.CPU.SetBC(0x5a00)
					m.CPU.SetIFF1(false)
					m.CPU.SetIFF2(false)
					for i := 0; i < 1000; i++ {
						if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
							break
						}
						m.CPU.DoOpcode()
					}
					if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != p7 || m.Memory.ScorpionPagingState() != p1 || m.CPU.IFF1() || m.CPU.IFF2() {
						t.Fatal("return", page, offset, prior, name)
					}
					if name == "zx__scorpion_memory__scorpion_read" && byte(m.CPU.AF()>>8) != 0xa7 {
						t.Fatal("read", page, offset)
					}
					if name == "zx__scorpion_memory__scorpion_write" && m.Memory.ReadRAMDirect(page, offset) != 0x5a {
						t.Fatal("write", page, offset)
					}
					cases++
				}
			}
		}
	}
	// Compiled caller checks argument allocation, not only raw ASM entry ABI.
	m, err := spectrum.NewScorpionMemory256(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x8000+uint16(i), b, false)
	}
	m.Memory.Write(0xbf00, 0, false)
	m.Memory.Write(0xbf01, 0x70, false)
	m.CPU.SetPC(uint16(a.Symbols["main"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	for i := 0; i < 1000; i++ {
		if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
			break
		}
		m.CPU.DoOpcode()
	}
	if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || byte(m.CPU.AF()>>8) != 167 || m.Memory.ReadRAMDirect(13, 123) != 167 {
		t.Fatal("compiled caller")
	}
	t.Logf("%d raw ABI cases and compiled caller PASS", cases)
}
