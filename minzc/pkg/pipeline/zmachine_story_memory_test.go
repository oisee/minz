package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Physical RAM is the oracle for real compiled big-endian Z3 reads.
func TestZXStoryLogicalMemoryNative(t *testing.T) {
	src := `import zx.story_memory
 global requested: u32
 global prior_paging: u8
 fun read_test() -> u8 { return story_byte(requested, prior_paging) }
 fun word_test() -> u16 { return story_word(requested, prior_paging) }
 `
	h, err := nanz.ParseWithOpts(src, "story-memory.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	if len(a.Binary) >= 0x3d00 {
		t.Fatal("reader collides with native stack")
	}
	pages := []int{0, 1, 3, 4, 6, 7}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for page := 0; page < 8; page++ {
		for i := range m.Memory.RAM[page] {
			m.Memory.RAM[page][i] = byte(i*37 + (i>>8)*11 + page*19)
		}
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x8000+uint16(i), b, false)
	}
	byteAt := func(address int) byte { return m.Memory.RAM[pages[address/16384]][address%16384] }
	addresses := []int{0, 1, 255, 256, 86926, 86927, 98302, 98303}
	for boundary := 16384; boundary < 98304; boundary += 16384 {
		addresses = append(addresses, boundary-2, boundary-1, boundary, boundary+1)
	}
	cases := 0
	for _, address := range addresses {
		for _, restore := range []byte{0, 0x10, 0x18, 0x1f} {
			for _, word := range []bool{false, true} {
				if word && address == 98303 {
					continue
				}
				cases++
				m.Memory.SetPagingForce(restore)
				req := uint16(a.Symbols["requested"])
				for i := 0; i < 4; i++ {
					m.Memory.Write(req+uint16(i), byte(address>>(8*i)), false)
				}
				m.Memory.Write(uint16(a.Symbols["prior_paging"]), restore, false)
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x70, false)
				before := m.Memory.RAM
				name := "read_test"
				if word {
					name = "word_test"
				}
				m.CPU.SetPC(uint16(a.Symbols[name]))
				m.CPU.SetSP(0xbf00)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				for i := 0; i < 4000; i++ {
					if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
						break
					}
					m.CPU.DoOpcode()
				}
				if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.CPU.IFF1() || m.CPU.IFF2() {
					t.Fatalf("address%d word%v PC/SP=%04x/%04x", address, word, m.CPU.PC(), m.CPU.SP())
				}
				if m.Memory.PagingState() != restore {
					t.Fatal("paging state lost")
				}
				want := uint16(byteAt(address))
				got := uint16(m.CPU.AF() >> 8)
				if word {
					want = want<<8 | uint16(byteAt(address+1))
					got = m.CPU.HL()
				}
				if got != want {
					t.Fatalf("address%d word%v restore%02x got%04x want%04x", address, word, restore, got, want)
				}
				for _, page := range pages {
					if m.Memory.RAM[page] != before[page] {
						t.Fatalf("read modified story bank%d", page)
					}
				}
			}
		}
	}
	t.Logf("%d physical-bank cases; generated module %d bytes", cases, len(a.Binary))
}
