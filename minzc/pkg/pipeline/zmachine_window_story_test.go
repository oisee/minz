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
func TestZMachineV3VerifyLifecycleNative(t *testing.T) {
	t.Run("original", func(t *testing.T) { testZMachineV3ScreenStory(t, "verify_lifecycle", false) })
	t.Run("corrupt", func(t *testing.T) { testZMachineV3ScreenStory(t, "verify_corrupt", false) })
}
func TestZMachineNativeOpcodeDiagnostic(t *testing.T) {
	testZMachineV3ScreenStory(t, "bad_opcode", false)
}
func TestZMachineV3UpperInputNative(t *testing.T) {
	for _, c := range []struct{ height, window, count byte }{{1, 1, 40}, {0, 1, 79}, {2, 1, 79}, {23, 1, 79}, {23, 0, 79}} {
		t.Run(fmt.Sprintf("height%d/window%d/count%d", c.height, c.window, c.count), func(t *testing.T) {
			testZMachineV3ScreenStoryInput(t, "window_input", false, c.height, c.window, c.count)
		})
	}
}
func testZMachineV3ScreenStory(t *testing.T, fixture string, timeMode bool) {
	testZMachineV3ScreenStoryInput(t, fixture, timeMode, 1, 1, 40)
}
func testZMachineV3ScreenStoryInput(t *testing.T, fixture string, timeMode bool, height, window, count byte) {
	root := filepath.Clean("../../..")
	dir := t.TempDir()
	sourceFixture := fixture
	if fixture == "bad_opcode" {
		sourceFixture = "windows"
	}
	if fixture == "verify_corrupt" {
		sourceFixture = "verify_lifecycle"
	}
	story, e := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures/"+sourceFixture+".z3"))
	if e != nil {
		t.Fatal(e)
	}
	if fixture == "verify_corrupt" {
		globals := int(story[12])<<8 | int(story[13])
		story[globals+3] ^= 1 // score is safe to change; original checksum remains.
	}
	initialPC := int(story[6])<<8 | int(story[7])
	if fixture == "bad_opcode" {
		story[initialPC] = 0xbf
	}
	if fixture == "window_input" {
		globals := int(story[12])<<8 | int(story[13])
		// Inform globals 3..5 configure the independent story, leaving its
		// opcode bodies intact; every typed letter is checked by the story.
		story[globals+7] = height
		story[globals+9] = window
		story[globals+11] = count
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
	keyAt := 0
	if fixture == "window_input" {
		ready := false
		keys := []byte{0, 'z', 'x', 'c', 'v', 'a', 's', 'd', 'f', 'g', 'q', 'w', 'e', 'r', 't', '1', '2', '3', '4', '5', '0', '9', '8', '7', '6', 'p', 'o', 'i', 'u', 'y', 13, 'l', 'k', 'j', 'h', 32, 0, 'm', 'n', 'b'}
		m.SetPCTrap(uint16(a.Symbols["zx__console__con_getchar"]), func() { m.Keyboard.Reset(); ready = true; m.CPU.DoOpcode() })
		m.SetPCTrap(uint16(a.Symbols["zx__console__scan_row_key"]), func() {
			if ready {
				ch := byte('a')
				if keyAt == int(count) {
					ch = 13
				}
				if keyAt > int(count) {
					t.Fatal("extra input")
				}
				for idx, key := range keys {
					if ch == key {
						m.Keyboard.KeyPress(idx/5, idx%5)
						break
					}
				}
				keyAt++
				ready = false
			}
			m.CPU.DoOpcode()
		})
	}
	if fixture == "window_input" {
		editing := false
		m.SetPCTrap(uint16(a.Symbols["zx__console__con_input"]), func() { editing = true; m.CPU.DoOpcode() })
		m.SetPCTrap(uint16(a.Symbols["zx__console__con_draw_char"]), func() {
			row := byte(m.CPU.DE() >> 8)
			if editing && (row > 23 || (row != 0 && ((window == 1 && row > height) || (window == 0 && row <= height)))) {
				t.Fatalf("draw escaped window: row %d height %d window %d", row, height, window)
			}
			m.CPU.DoOpcode()
		})
	}
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
	if fixture == "verify_lifecycle" || fixture == "verify_corrupt" {
		want := "verified\nverified\n"
		if fixture == "verify_corrupt" {
			want = "rejected\nrejected\n"
		}
		if string(output) != want {
			t.Fatalf("verify lifecycle %q != %q", output, want)
		}
		scoreAddr := (int(story[12])<<8 | int(story[13])) + 2
		if m.Memory.RAM[0][scoreAddr] != 0 || m.Memory.RAM[0][scoreAddr+1] != 123 {
			t.Fatal("story did not mutate score")
		}
	} else if fixture == "bad_opcode" {
		faultPC := initialPC + 1
		want := fmt.Sprintf("Z3 error 1:0:15:%d:%d:%d\n", faultPC>>16, (faultPC>>8)&255, faultPC&255)
		if string(output) != want {
			t.Fatalf("diagnostic %q != %q", output, want)
		}
	} else if fixture == "windows" {
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
	} else if fixture == "window_input" {
		if keyAt != int(count)+1 || !bytes.Equal(output, []byte("123456789012345678901234567890\r")) {
			t.Fatalf("input not preserved: keys %d output %q", keyAt, output)
		}
		first, last := int(height)+1, 24
		if window == 0 {
			first, last = 1, 24
		}
		for row := first; row < last; row++ {
			for line := 0; line < 8; line++ {
				for x := 0; x < 32; x++ {
					adr := uint16(0x4000 | ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | x)
					if m.Memory.Read(adr) != 0 {
						t.Fatalf("upper input wrote lower row %d", row)
					}
				}
			}
		}
		wantRow, wantCol := byte(1), byte(31)
		if height == 0 {
			wantRow, wantCol = 0, 31
		} else if height > 1 {
			wantRow, wantCol = 5, 0
			if height < 4 {
				wantRow, wantCol = height, 31
			}
		}
		if window == 0 {
			wantRow, wantCol = 23, 15
		}
		for name, want := range map[string]byte{"con_row": wantRow, "con_col": wantCol, "z3_selected_window": window, "z3_lower_row": 23, "z3_lower_col": 0} {
			if got := m.Memory.Read(uint16(a.Symbols["zx__console__"+name])); got != want {
				t.Fatalf("upper input changed %s: %d != %d", name, got, want)
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
