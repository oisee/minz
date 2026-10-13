package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Independent Inform bytecode exercises actual VAR10/11 dispatch, capability
// flag, upper reselection and lower scrolling. No interpreter output is stubbed.
func TestZMachineV3WindowStoryNative(t *testing.T) { testZMachineV3ScreenStory(t, "windows", false) }
func TestZMachineV3StatusStoryNative(t *testing.T) {
	t.Run("score", func(t *testing.T) { testZMachineV3ScreenStory(t, "status", false) })
	t.Run("time", func(t *testing.T) { testZMachineV3ScreenStory(t, "status", true) })
}
func testZMachineV3ScreenStory(t *testing.T, fixture string, timeMode bool) {
	root := filepath.Clean("../../..")
	dir := t.TempDir()
	story, e := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures/"+fixture+".z3"))
	if e != nil {
		t.Fatal(e)
	}
	if timeMode {
		globals := int(story[12])<<8 | int(story[13])
		story[1] |= 2
		story[globals+2] = 0
		story[globals+3] = 17
		story[globals+4] = 0
		story[globals+5] = 4
	}
	source := filepath.Join(dir, "core.nanz")
	cmd := exec.Command("python3", filepath.Join(root, "scripts/zmachine/prepare_native_core.py"), source, "--story-bytes", fmt.Sprint(len(story)), "--console", "--windows")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	src, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	h, e := nanz.ParseWithOpts(string(src), source, nanz.ParseOpts{BaseDir: dir, StdlibDir: filepath.Join(root, "stdlib")})
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + s.Assembly)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	m, e := spectrum.NewPentagon128(nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x5b00+uint16(i), b, false)
	}
	copy(m.Memory.RAM[0][:], story)
	m.Memory.SetPagingForce(0x10)
	m.Memory.Write(0xbf10, 0x76, false)
	m.Memory.Write(0xbf00, 0x10, false)
	m.Memory.Write(0xbf01, 0xbf, false)
	m.CPU.SetPC(uint16(a.Symbols["main"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	var output []byte
	m.SetPCTrap(uint16(a.Symbols["zx__console__z3_window_put"]), func() { output = append(output, byte(m.CPU.AF()>>8)); m.CPU.DoOpcode() })
	for i := 0; i < 2000; i++ {
		m.RunFrameFast()
		if m.CPU.Halted() {
			break
		}
	}
	if !m.CPU.Halted() || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 {
		t.Fatalf("boundary %04x/%04x output %q", m.CPU.PC(), m.CPU.SP(), output)
	}
	if fixture == "windows" {
		want := []byte("UPPER" + string(bytes.Repeat([]byte("lower\n"), 25)) + "RESETdone\n")
		if !bytes.Equal(output, want) {
			t.Fatalf("output %q != %q", output, want)
		}
		if m.Memory.RAM[0][1]&32 == 0 {
			t.Fatal("capability missing")
		}
		for x, ch := range []byte("RESET") {
			for line := 0; line < 8; line++ {
				adr := uint16(0x4020 + line*256 + x)
				font := m.Memory.Read(uint16(0x3d00 + int(ch-32)*8 + line))
				if got := m.Memory.Read(adr); got != font {
					t.Fatalf("upper pixels x%d line%d: %02x != %02x", x, line, got, font)
				}
			}
		}
	} else {
		if !bytes.Equal(output, []byte("X")) {
			t.Fatalf("status polluted window/stream %q", output)
		}
		for name, want := range map[string]byte{"con_row": 1, "con_col": 1, "z3_selected_window": 1, "z3_lower_row": 23, "z3_lower_col": 0} {
			if got := m.Memory.Read(uint16(a.Symbols["zx__console__"+name])); got != want {
				t.Fatalf("status changed %s: %d != %d", name, got, want)
			}
		}
		text := "-7/42"
		if timeMode {
			text = "17:04"
		}
		for x, ch := range []byte("Status room") {
			for line := 0; line < 8; line++ {
				adr := uint16(0x4000 + line*256 + x)
				font := m.Memory.Read(uint16(0x3d00 + int(ch-32)*8 + line))
				if m.Memory.Read(adr) != font {
					t.Fatal("room status pixels", x, line)
				}
			}
		}
		for x, ch := range []byte(text) {
			for line := 0; line < 8; line++ {
				adr := uint16(0x4000 + line*256 + 20 + x)
				font := m.Memory.Read(uint16(0x3d00 + int(ch-32)*8 + line))
				if m.Memory.Read(adr) != font {
					t.Fatalf("numeric status %s pixels %d/%d", text, x, line)
				}
			}
		}
		if m.Memory.RAM[0][1]&16 != 0 {
			t.Fatal("status unavailable flag set")
		}
	}
	t.Logf("V3 %s screen bytecode/pixels/return PASS; %d resident bytes", fixture, len(a.Binary))
}
