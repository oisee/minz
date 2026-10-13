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
	"strings"
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
func TestZMachineBankedZorkExploreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY")
	}
	if os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_VM for original-interpreter transcript reference")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-explore")
}
func TestZMachineBankedZorkRestartNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-restart")
}
func TestZMachineBankedZorkSaveRestoreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-save")
}
func TestZMachineBankedZorkCellarSaveRestoreNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" {
		t.Skip("set MINZ_Z3_STORY and MINZ_Z3_VM")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-cellar-save")
}

// A separately built full V3 story, not the Zork I workload used earlier.
func TestZMachineBankedZorkIIGameplayNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" || os.Getenv("MINZ_Z3_FROTZ") == "" {
		t.Skip("set MINZ_Z3_ZORK2_STORY, MINZ_Z3_VM and MINZ_Z3_FROTZ")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2")
}
func TestZMachineBankedZorkIICompressedNative(t *testing.T) {
	path := os.Getenv("MINZ_Z3_ZORK2_STORY")
	if path == "" || os.Getenv("MINZ_Z3_VM") == "" || os.Getenv("MINZ_Z3_FROTZ") == "" {
		t.Skip("set MINZ_Z3_ZORK2_STORY, MINZ_Z3_VM and MINZ_Z3_FROTZ")
	}
	testZMachineBankedInterpreterNative(t, false, path, "@console-zork2-compressed")
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
	compressedStory := commands == "@console-zork2-compressed"
	zorkII := commands == "@console-zork2" || compressedStory
	cellarSave := commands == "@console-cellar-save"
	saveRestore := commands == "@console-save" || cellarSave
	restart := commands == "@console-restart"
	explore := commands == "@console-explore" || restart || saveRestore || zorkII
	keyboard := commands == "@console-keyboard" || explore
	console := commands == "@console-startup" || keyboard
	var restartArchive []byte
	if restart || saveRestore {
		archivePath := filepath.Join(t.TempDir(), "restart.rle")
		pack := exec.Command("python3", filepath.Join(root, "scripts/zmachine/restart_rle.py"), externalStory, archivePath)
		if out, err := pack.CombinedOutput(); err != nil {
			t.Fatalf("pack %v %s", err, out)
		}
		var err error
		restartArchive, err = os.ReadFile(archivePath)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Args = append(cmd.Args, "--restart-bytes", strconv.Itoa(int(story[14])<<8|int(story[15])), "--restart-rle-bytes", strconv.Itoa(len(restartArchive)))
	}

	if saveRestore {
		cmd.Args = append(cmd.Args, "--save-story", externalStory)
	}
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
	var storageImage []byte
	if compressedStory {
		layoutDir := t.TempDir()
		layout := exec.Command("python3", filepath.Join(root, "scripts/zmachine/story_lz_layout.py"), externalStory, layoutDir)
		if out, err := layout.CombinedOutput(); err != nil {
			t.Fatalf("layout: %v %s", err, out)
		}
		storageImage, err = os.ReadFile(filepath.Join(layoutDir, "layout-2048.bin"))
		if err != nil {
			t.Fatal(err)
		}
		cmd.Args = append(cmd.Args, "--story-layout", filepath.Join(layoutDir, "layout-2048.json"))
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
	if compressedStory {
		for i, b := range storageImage {
			m.Memory.RAM[pages[i/16384]][i%16384] = b
		}
	}
	if restart || saveRestore {
		for i, b := range restartArchive {
			at := len(story) + i
			m.Memory.RAM[pages[at/16384]][at%16384] = b
		}
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
	var reference []byte
	if explore {
		route := []byte("north\neast\nopen window\nenter window\nwest\ntake lamp\ntake sword\ninventory\nturn on lamp\nmove rug\nopen trap door\ndown\nlook\nup\nquit\ny\n")
		if zorkII {
			route = []byte("look\ninventory\ntake lamp\nturn on lamp\nnorth\nlook\nsouth\nquit\ny\n")
		}
		if restart {
			route = []byte("open mailbox\ntake leaflet\nrestart\ny\nlook\ninventory\nquit\ny\n")
		}
		if saveRestore {
			route = []byte("open mailbox\nsave\ntake leaflet\ninventory\nrestore\ninventory\nquit\ny\n")
		}
		if cellarSave {
			route = []byte("north\neast\nopen window\nenter window\nwest\ntake lamp\ntake sword\nturn on lamp\nmove rug\nopen trap door\ndown\nsave\ndrop sword\ninventory\nrestore\ninventory\ndrop lamp\nrestore\ninventory\nquit\ny\n")
		}
		d := t.TempDir()
		source, err := os.ReadFile(filepath.Join(root, "examples/zmachine/zork-west-demo/zvm.nanz"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "zvm.nanz"), source, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "zork-west-demo.z3"), story, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(d, "text"), 0700); err != nil {
			t.Fatal(err)
		}
		printSource, err := os.ReadFile(filepath.Join(root, "stdlib/text/print.nanz"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "text/print.nanz"), printSource, 0600); err != nil {
			t.Fatal(err)
		}
		vm := exec.Command(os.Getenv("MINZ_Z3_VM"), "-H", filepath.Join(d, "zvm.nanz"))
		vm.Stdin = bytes.NewReader(route)
		reference, err = vm.Output()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(reference, []byte("Z3 error")) {
			t.Fatalf("reference failed%q", reference)
		}
		if tool := os.Getenv("MINZ_Z3_FROTZ"); tool != "" && !restart && !saveRestore {
			referenceCmd := exec.Command(tool, "-m", "-w", "255", externalStory)
			referenceCmd.Stdin = bytes.NewReader(route)
			external, err := referenceCmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			normalize := func(data []byte) string {
				var lines []string
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimRight(line, " \r")
					line = strings.TrimLeft(line, "> ")
					if line == "" || strings.Contains(line, "Score:") && strings.Contains(line, "Moves:") {
						continue
					}
					lines = append(lines, line)
				}
				// Ignore terminal wrapping while retaining every word and punctuation.
				return strings.Join(strings.Fields(strings.Join(lines, "\n")), " ")
			}
			if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
				if err := os.MkdirAll(d, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(d, "explore-frotz.txt"), external, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if normalize(reference) != normalize(external) {
				t.Fatalf("VM/Frotz exploration differs VM%q Frotz%q", reference, external)
			}
			t.Log("exploration VM text matches independently implemented Frotz")
		}
		keyText = bytes.ReplaceAll(route, []byte{10}, []byte{13})
	}
	keys := []byte{0, 'z', 'x', 'c', 'v', 'a', 's', 'd', 'f', 'g', 'q', 'w', 'e', 'r', 't', '1', '2', '3', '4', '5', '0', '9', '8', '7', '6', 'p', 'o', 'i', 'u', 'y', 13, 'l', 'k', 'j', 'h', 32, 0, 'm', 'n', 'b'}
	keyAt := 0
	ready := false
	minSP := uint16(0xbf00)
	var rendered []byte
	var observedSaveLengths []int
	inputEcho := false
	var inputReturn, inputSP uint16

	for i := 0; i < 200000000; i++ {
		if console && !keyboard && m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
			break
		}
		if m.CPU.PC() == 0x5000 && m.CPU.SP() == 0xbf02 {
			break
		}
		if saveRestore && m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
			addr := uint16(a.Symbols["native_save_length"])
			length := int(m.Memory.Read(addr)) | int(m.Memory.Read(addr+1))<<8
			if length > 0 && (len(observedSaveLengths) == 0 || observedSaveLengths[len(observedSaveLengths)-1] != length) {
				observedSaveLengths = append(observedSaveLengths, length)
			}
		}
		if keyboard {
			if m.CPU.PC() == uint16(a.Symbols["tui_read_line_or_eof"]) {
				inputEcho = true
				inputSP = m.CPU.SP()
				inputReturn = uint16(m.Memory.Read(inputSP)) | uint16(m.Memory.Read(inputSP+1))<<8
			}
			if inputEcho && m.CPU.PC() == inputReturn && m.CPU.SP() == inputSP+2 {
				inputEcho = false
			}
			if !inputEcho && m.CPU.PC() == uint16(a.Symbols["native_putchar"]) {
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
		if explore {
			if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
				if err := os.MkdirAll(d, 0755); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"explore-native.txt": rendered, "explore-vm.txt": reference} {
					if err := os.WriteFile(filepath.Join(d, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !bytes.Equal(rendered, reference) {
				t.Fatalf("exploration text differs: native %q reference %q", rendered, reference)
			}
			phrases := []string{"Kitchen", "Living Room", "Cellar", "A brass lantern", "A sword"}
			if zorkII {
				phrases = []string{"ZORK II: The Wizard of Frobozz", "Inside the Barrow", "You are empty-handed.", "Taken.", "The lamp is now on.", "Narrow Tunnel", "total of 400 points"}
			}
			if saveRestore {
				phrases = []string{"Ok.", "A leaflet", "You are empty-handed."}
				if cellarSave {
					phrases = []string{"Cellar", "A sword", "A brass lantern", "Dropped."}
				}
			}
			if restart {
				phrases = []string{"You are empty-handed.", "West of House"}
			}
			for _, phrase := range phrases {
				if !bytes.Contains(rendered, []byte(phrase)) {
					t.Fatalf("exploration missing %q: %q", phrase, rendered)
				}
			}
			if cellarSave {
				restored := []byte("You are carrying:\n  A sword\n  A brass lantern (providing light)")
				if bytes.Count(rendered, restored) != 2 {
					t.Fatal("both restores must recover sword and lit lamp", string(rendered))
				}
				if len(observedSaveLengths) != 1 || observedSaveLengths[0] > 2204 {
					t.Fatal("missing or oversized native save", observedSaveLengths)
				}
				t.Logf("cellar native save %d bytes; both inventory restores independently checked", observedSaveLengths[0])
			}
			if compressedStory {
				at := uint16(a.Symbols["native_story_cache_misses"])
				misses := int(m.Memory.Read(at)) | int(m.Memory.Read(at+1))<<8
				t.Logf("compressed resident %d bytes; cache misses %d (u16 counter)", len(a.Binary), misses)
			}
			t.Logf("exploration %d keys; %d output bytes; %d T; native stack %d bytes", keyAt, len(rendered), m.CPU.Tstates(), 0xbf00-int(minSP))
			return
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
