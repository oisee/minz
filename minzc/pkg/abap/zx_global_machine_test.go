package abap

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

// Execute the real report on the full 48K machine, without intercepting its
// output routines. Expected pixels come from the source font, not emitted data.
func TestSAPZXDemoFontFullMachine(t *testing.T) {
	src, err := os.ReadFile("../../../examples/abap/sap_zx_demo.abap")
	if err != nil {
		t.Fatal(err)
	}
	h, err := Compile(string(src), "sap_zx_demo", hir.TargetZXSpectrum)
	if err != nil {
		t.Fatal(err)
	}
	var font []byte
	for _, g := range h.Globals {
		if g.Name == "_zx_font" {
			font = append([]byte(nil), g.Init...)
		}
	}
	if len(font) != 768 {
		t.Fatalf("source font has %d bytes", len(font))
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	steps, err := pipeline.CompileHIRSteps(h, opts)
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	rom, err := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if err != nil {
		t.Fatal(err)
	}
	m, err := spectrum.New48K(rom)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x8000+uint16(i), b, false)
	}
	main, ok := a.Symbols["main"]
	if !ok {
		t.Fatal("missing main")
	}
	// Use the CLI's ordinary bare-main contract: completion is HALT.
	m.CPU.SetPC(uint16(main))
	m.CPU.SetSP(0xeff0)
	m.CPU.SetIY(0x5c3a)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	for frame := 0; frame < 100 && !m.CPU.Halted(); frame++ {
		m.RunFrame()
	}
	if !m.CPU.Halted() {
		t.Fatalf("report did not finish: PC=%04x SP=%04x", m.CPU.PC(), m.CPU.SP())
	}
	// Character WRITE streams bytes without an implicit newline. The first
	// WRITE fills row 0; row 1 holds the title and the first four bytes of
	// the following separator. Compare all eight bitmap scans of both rows.
	for line, text := range []string{"================================", "  SAP Report on ZX Spectrum!===="} {
		row := line
		for col := 0; col < 32; col++ {
			ch := byte(' ')
			if col < len(text) {
				ch = text[col]
			}
			for scan := 0; scan < 8; scan++ {
				addr := uint16(0x4000 | (row&0x18)<<8 | scan<<8 | (row&7)<<5 | col)
				want := font[int(ch-32)*8+scan]
				if got := m.Memory.Read(addr); got != want {
					t.Fatalf("line %d col %d scan %d at %04x: got %02x want %02x", line, col, scan, addr, got, want)
				}
			}
		}
	}
}
