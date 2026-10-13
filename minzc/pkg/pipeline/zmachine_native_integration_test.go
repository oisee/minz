package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// Real Nanz interpreter executes an independent Inform-built Z3 story from
// physical bank0. Host I/O is replaced by explicit native capture boundaries.
func TestZMachineBankedInterpreterNative(t *testing.T) {
	testZMachineBankedInterpreterNative(t, false, "", "")
}
func TestZMachineBankedHighMemoryNative(t *testing.T) {
	testZMachineBankedInterpreterNative(t, true, "", "")
}
func TestZMachineBankedZorkStartupNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to an independently supplied Zork I story")
	}
	testZMachineBankedInterpreterNative(t, false, path, "")
}
func TestZMachineBankedZorkGameplayNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to independently supplied Zork I story")
	}
	testZMachineBankedInterpreterNative(t, false, path, "look\nopen mailbox\ntake leaflet\nread leaflet\ninventory\nquit\ny\n")
}
func TestZMachineBankedZorkConsoleNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-startup")
}
func TestZMachineBankedZorkKeyboardNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-keyboard")
}
func testZMachineBankedInterpreterNative(t *testing.T, highMemory bool, externalStory string, commands string) {
	root := filepath.Clean("../../..")
	story, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures/stream3.z3"))
	if err != nil {
		t.Fatal(err)
	}
	if highMemory {
		fixture := filepath.Join(t.TempDir(), "high.z3")
		cmd := exec.Command("python3", filepath.Join(root, "examples/zmachine/z3-fixtures/high_memory.py"), fixture, "--bytes", "98302")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v %s", err, out)
		}
		story, err = os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
	}
	if externalStory != "" {
		story, err = os.ReadFile(externalStory)
		if err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "core.nanz")
	cmd := exec.Command("python3", filepath.Join(root, "scripts/zmachine/prepare_native_core.py"), path, "--story-bytes", strconv.Itoa(len(story)))
	keyboard := commands == "@console-keyboard"
	console := commands == "@console-startup" || keyboard
	if console {
		commands = ""
		cmd.Args = append(cmd.Args, "--console")
	}
	if commands != "" {
		commandPath := filepath.Join(t.TempDir(), "commands.txt")
		if err := os.WriteFile(commandPath, []byte(commands), 0600); err != nil {
			t.Fatal(err)
		}
		cmd.Args = append(cmd.Args, "--commands", commandPath)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{StdlibDir: filepath.Join(root, "stdlib")})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MINZ_Z3_ASM") != "" {
		if err := os.WriteFile(os.Getenv("MINZ_Z3_ASM"), []byte(steps.Assembly), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + steps.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	if 0x5b00+len(a.Binary) > 0xbe00 {
		t.Fatalf("resident core reaches %04x; native stack starts be00", 0x5b00+len(a.Binary))
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if console {
		for ch := 32; ch < 128; ch++ {
			for line := 0; line < 8; line++ {
				m.Memory.ROM[1][0x3d00+(ch-32)*8+line] = byte((ch*17 + line*29) ^ 0x5a)
			}
		}
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x5b00+uint16(i), b, false)
	}
	pages := []int{0, 1, 3, 4, 6, 7}
	for i, b := range story {
		m.Memory.RAM[pages[i/16384]][i%16384] = b
	}
	m.Memory.SetPagingForce(0x10)
	m.Memory.Write(0xbf00, 0, false)
	m.Memory.Write(0xbf01, 0x50, false)
	m.CPU.SetPC(uint16(a.Symbols["main"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	m.CPU.SetTstates(0)
	if keyboard {
		snapshot := filepath.Join(t.TempDir(), "keyboard.sna")
		if err := formats.SaveSNA(snapshot, m); err != nil {
			t.Fatal(err)
		}
		saved, err := formats.LoadSNA(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := spectrum.NewPentagon128(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		fresh.Memory.ROM = m.Memory.ROM
		formats.ApplySnapshot(fresh, saved)
		if fresh.Memory.RAM != m.Memory.RAM || fresh.CPU.PC() != m.CPU.PC() || fresh.CPU.SP() != m.CPU.SP() {
			t.Fatal("snapshot before keyboard differs")
		}
		m = fresh
	}
	keyText := []byte("look\ropen mailbox\rtake leaflet\rread leaflet\rinventory\rquit\ry\r")
	keys := []byte{0, 'z', 'x', 'c', 'v', 'a', 's', 'd', 'f', 'g', 'q', 'w', 'e', 'r', 't', '1', '2', '3', '4', '5', '0', '9', '8', '7', '6', 'p', 'o', 'i', 'u', 'y', 13, 'l', 'k', 'j', 'h', 32, 0, 'm', 'n', 'b'}
	keyAt := 0
	ready := false
	minSP := uint16(0xbf00)
	var rendered []byte

	for i := 0; i < 100000000; i++ {
		if console && !keyboard && m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
			break
		}
		if m.CPU.PC() == 0x5000 && m.CPU.SP() == 0xbf02 {
			break
		}
		if keyboard {
			if m.CPU.PC() == uint16(a.Symbols["native_putchar"]) {
				rendered = append(rendered, byte(m.CPU.AF()>>8))
			}
			if m.CPU.PC() == uint16(a.Symbols["zx__console__con_getchar"]) {
				m.Keyboard.Reset()
				ready = true
			}
			if ready && m.CPU.PC() == uint16(a.Symbols["zx__console__scan_row_key"]) {
				if keyAt == len(keyText) {
					t.Fatal("unexpected additional keyboard input")
				}
				found := false
				for idx, ch := range keys {
					if ch == keyText[keyAt] {
						m.Keyboard.KeyPress(idx/5, idx%5)
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("unmapped key %q", keyText[keyAt])
				}
				keyAt++
				ready = false
			}
		}
		m.CPU.DoOpcode()
		if m.CPU.SP() < minSP {
			minSP = m.CPU.SP()
		}

	}
	if keyboard {
		if m.CPU.PC() != 0x5000 || m.CPU.SP() != 0xbf02 || keyAt != len(keyText) || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() || minSP < 0xbe00 {
			t.Fatalf("keyboard keys%d/%d PC/SP%04x/%04x minSP%04x", keyAt, len(keyText), m.CPU.PC(), m.CPU.SP(), minSP)
		}
		for _, phrase := range []string{"Opening the small mailbox reveals a leaflet.", "ZORK is a game of adventure, danger, and low cunning.", "You are carrying:\n  A leaflet", "Your score is 0 (total of 350 points), in 5 moves."} {
			if !bytes.Contains(rendered, []byte(phrase)) {
				t.Fatalf("keyboard missing%q output%q", phrase, rendered)
			}
		}
		if bytes.Contains(rendered, []byte("Z3 error")) {
			t.Fatalf("keyboard error%q", rendered)
		}
		t.Logf("keyboard %d keys; %d bytes; %d T; native stack %d bytes", keyAt, len(a.Binary), m.CPU.Tstates(), 0xbf00-int(minSP))
		return
	}

	if console {
		if m.CPU.PC() != uint16(a.Symbols["tui_read_line_or_eof"]) {
			t.Fatalf("console did not reach input PC%04x", m.CPU.PC())
		}
		// Independent 32x24 terminal model including wrap and scrolling.
		cells := [24][32]byte{}
		row, col := 0, 0
		newline := func() {
			col = 0
			row++
			if row == 24 {
				for r := 0; r < 23; r++ {
					cells[r] = cells[r+1]
				}
				cells[23] = [32]byte{}
				row = 23
			}
		}
		text := "ZORK I: The Great Underground Empire\nInfocom interactive fiction - a fantasy story\nCopyright (c) 1981, 1982, 1983, 1984, 1985, 1986 Infocom, Inc. All rights reserved.\nZORK is a registered trademark of Infocom, Inc.\nRelease 1 / Serial number 000001\n\nWest of House\nYou are standing in an open field west of a white house, with a boarded front door.\nThere is a small mailbox here.\n\n>"
		for _, ch := range []byte(text) {
			if ch == 10 || ch == 13 {
				newline()
				continue
			}
			cells[row][col] = ch
			col++
			if col == 32 {
				newline()
			}
		}
		for r := 0; r < 24; r++ {
			for c := 0; c < 32; c++ {
				ch := cells[r][c]
				for l := 0; l < 8; l++ {
					want := byte(0)
					if ch >= 32 {
						want = byte((int(ch)*17 + l*29) ^ 0x5a)
					}
					addr := uint16(0x4000 | ((r & 0x18) << 8) | (l << 8) | ((r & 7) << 5) | c)
					if m.Memory.Read(addr) != want {
						t.Fatalf("screen r%d c%d l%d got%02x want%02x", r, c, l, m.Memory.Read(addr), want)
					}
				}
				if m.Memory.Read(uint16(0x5800+r*32+c)) != 7 {
					t.Fatal("attribute mismatch")
				}
			}
		}
		if m.Memory.Read(uint16(a.Symbols["zx__console__con_col"])) != byte(col) || m.Memory.Read(uint16(a.Symbols["zx__console__con_row"])) != byte(row) || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() {
			t.Fatal("console cursor/paging/IFF mismatch")
		}
		t.Logf("console startup %d bytes; %d T; SP%04x cursor%d/%d", len(a.Binary), m.CPU.Tstates(), m.CPU.SP(), row, col)
		return
	}
	if m.CPU.PC() != 0x5000 || m.CPU.SP() != 0xbf02 {
		t.Fatalf("PC/SP=%04x/%04x vmPC=%02x%02x%02x steps=%02x%02x", m.CPU.PC(), m.CPU.SP(), m.Memory.Read(uint16(a.Symbols["pc"])+2), m.Memory.Read(uint16(a.Symbols["pc"])+1), m.Memory.Read(uint16(a.Symbols["pc"])), m.Memory.Read(uint16(a.Symbols["steps"])+1), m.Memory.Read(uint16(a.Symbols["steps"])))
	}
	if m.CPU.IFF1() || m.CPU.IFF2() || m.Memory.PagingState() != 0x10 {
		t.Fatal("native boundary state lost")
	}
	lengthAt := uint16(a.Symbols["native_output_length"])
	length := int(m.Memory.Read(lengthAt)) | int(m.Memory.Read(lengthAt+1))<<8
	if length > 1024 {
		t.Fatal("invalid capture length", length)
	}
	got := make([]byte, length)
	for i := range got {
		got[i] = m.Memory.Read(uint16(a.Symbols["native_output"]) + uint16(i))
	}
	wantPath := "stream3.out"
	if highMemory {
		wantPath = "high_memory.out"
	}
	want, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures", wantPath))
	if err != nil {
		t.Fatal(err)
	}
	if !highMemory && externalStory == "" {
		for _, expected := range []struct {
			address int
			data    []byte
		}{
			{0x492, []byte{0, 3, 65, 13, 66}}, {0x4b2, []byte{0, 2, 88, 90}}, {0x4d2, []byte{0, 1, 89}},
		} {
			if !bytes.Equal(m.Memory.RAM[0][expected.address:expected.address+len(expected.data)], expected.data) {
				t.Fatalf("stream RAM mismatch at %04x", expected.address)
			}
		}
	}
	if externalStory != "" {
		for _, phrase := range []string{"ZORK I: The Great Underground Empire", "West of House", "There is a small mailbox here."} {
			if !bytes.Contains(got, []byte(phrase)) {
				t.Fatalf("missing %q: output %q", phrase, got)
			}
		}
		if commands != "" {
			at := uint16(a.Symbols["native_command_at"])
			consumed := int(m.Memory.Read(at)) | int(m.Memory.Read(at+1))<<8
			if consumed != len(commands) {
				t.Fatalf("consumed %d of %d command bytes", consumed, len(commands))
			}
			for _, phrase := range []string{"Opening the small mailbox reveals a leaflet.", "ZORK is a game of adventure, danger, and low cunning.", "You are carrying:\n  A leaflet", "Your score is 0 (total of 350 points), in 5 moves.", "Do you wish to leave the game?"} {
				if !bytes.Contains(got, []byte(phrase)) {
					t.Fatalf("gameplay missing %q: %q", phrase, got)
				}
			}
			if bytes.Count(got, []byte("There is a small mailbox here.")) != 2 {
				t.Fatalf("LOOK did not repeat room: %q", got)
			}
		}
		if bytes.Contains(got, []byte("Z3 error")) {
			t.Fatalf("interpreter failure %q", got)
		}
	} else if !bytes.Equal(got, want) {
		t.Fatalf("output got %q want %q; vmPC=%02x%02x%02x steps=%d", got, want, m.Memory.Read(uint16(a.Symbols["pc"])+2), m.Memory.Read(uint16(a.Symbols["pc"])+1), m.Memory.Read(uint16(a.Symbols["pc"])), int(m.Memory.Read(uint16(a.Symbols["steps"]))))
	}
	t.Logf("native interpreter %d bytes; %d T; output %q", len(a.Binary), m.CPU.Tstates(), got)
}
