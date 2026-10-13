package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// The original header is needed on the nonmatching successor after AND 31.
// Independent byte oracle: matching headers return 1, others 2+(header>>5).
func TestAccumulatorHeaderSuccessorNative(t *testing.T) {
	src := `global header: u8
 fun probe() -> u16 {
  let h: u8 = header
  if u16(h & 31) == 5 { return 1 }
  return 2 + u16(h / 32)
 }`
	h, err := nanz.Parse(src, "header.nanz")
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
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range a.Binary {
		m.Memory.Write(0x8000+uint16(i), v, false)
	}
	for value := 0; value < 256; value++ {
		m.Memory.Write(uint16(a.Symbols["header"]), byte(value), false)
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		m.CPU.SetPC(uint16(a.Symbols["probe"]))
		m.CPU.SetSP(0xbf00)
		for i := 0; i < 2000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			m.CPU.DoOpcode()
		}
		want := uint16(2 + (value >> 5))
		if value&31 == 5 {
			want = 1
		}
		if m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.CPU.HL() != want {
			t.Fatalf("header%02x HL=%04x want=%04x PC/SP=%04x/%04x", value, m.CPU.HL(), want, m.CPU.PC(), m.CPU.SP())
		}
	}
}
