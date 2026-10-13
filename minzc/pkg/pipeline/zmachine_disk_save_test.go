package pipeline

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestZMachineZorkDiskSaveAcrossRestart(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to Zork I")
	}
	testZMachineDiskSave(t, path, true, false, false)
}

func TestZMachineDiskSaveAcrossRestart(t *testing.T) { testZMachineDiskSave(t, "", true, false, false) }

func TestZMachineDiskSaveLegacyStaging(t *testing.T) {
	testZMachineDiskSave(t, "", false, false, false)
}

func TestZMachineZorkDiskConsoleSaveAcrossRestart(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to Zork I")
	}
	testZMachineDiskSave(t, path, true, true, false)
}

func TestZMachineZorkWindowDiskConsoleSaveAcrossRestart(t *testing.T) {
	path := os.Getenv("MINZ_Z3_STORY")
	if path == "" {
		t.Skip("set MINZ_Z3_STORY to Zork I")
	}
	testZMachineDiskSave(t, path, true, true, true)
}

// Independent Inform story: SAVE branches again after RESTORE, recovering
// value=1 even though the restore command sets it to 7 first.
func testZMachineDiskSave(t *testing.T, externalStory string, extended, console, windows bool) {
	root := filepath.Clean("../../..")
	original, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures/save_restore.z3"))
	if err != nil {
		t.Fatal(err)
	}
	if externalStory != "" {
		original, err = os.ReadFile(externalStory)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, size := range []int{len(original)} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			story := append([]byte(nil), original...)
			if size > len(story) {
				story = append(story, make([]byte, size-len(story))...)
			}
			dir := t.TempDir()
			storyPath := filepath.Join(dir, "story.z3")
			if err := os.WriteFile(storyPath, story, 0600); err != nil {
				t.Fatal(err)
			}
			archivePath := filepath.Join(dir, "restart.rle")
			pack := exec.Command("python3", filepath.Join(root, "scripts/zmachine/restart_rle.py"), storyPath, archivePath)
			if out, err := pack.CombinedOutput(); err != nil {
				t.Fatalf("pack: %v %s", err, out)
			}
			archive, err := os.ReadFile(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			offset := len(story)
			if offset < 16384 {
				offset = 16384
			}
			if offset/16384 != (offset+len(archive)-1)/16384 {
				offset = (offset/16384 + 1) * 16384
			}
			commands := filepath.Join(dir, "commands.txt")
			route := []byte("s\nq\n")
			if externalStory != "" {
				route = []byte("open mailbox\ntake leaflet\nsave\ninventory\nquit\ny\n")
			}
			if err := os.WriteFile(commands, route, 0600); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(dir, "native.nanz")
			gen := exec.Command("python3", filepath.Join(root, "scripts/zmachine/prepare_native_core.py"), source, "--story-bytes", strconv.Itoa(size), "--restart-offset", strconv.Itoa(offset), "--restart-bytes", strconv.Itoa(int(story[14])<<8|int(story[15])), "--restart-rle-bytes", strconv.Itoa(len(archive)), "--save-story", storyPath, "--disk-save", "--commands", commands)
			if console {
				gen.Args = gen.Args[:len(gen.Args)-2]
				gen.Args = append(gen.Args, "--console")
			}
			if extended {
				gen.Args = append(gen.Args, "--extended-save")
			}
			if windows {
				gen.Args = append(gen.Args, "--windows")
			}
			if out, err := gen.CombinedOutput(); err != nil {
				t.Fatalf("generate: %v %s", err, out)
			}
			src, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			// Capture output by observing entry without allocating a bounded output buffer.
			capture := string(src)
			if !console {
				start := strings.Index(capture, "fun native_putchar(")
				brace := strings.Index(capture[start:], "{") + start
				end := brace + 1
				depth := 1
				for depth > 0 {
					if capture[end] == '{' {
						depth++
					}
					if capture[end] == '}' {
						depth--
					}
					end++
				}
				capture = capture[:start] + "fun native_putchar(ch:u8) { asm z80 { NOP } }" + capture[end:]
				capture = strings.ReplaceAll(capture, "global native_output: [u8; 1024]\n", "")
				capture = strings.ReplaceAll(capture, "global native_output_length: u16 = 0\n", "")
			}
			h, err := nanz.ParseWithOpts(capture, source, nanz.ParseOpts{BaseDir: filepath.Dir(source), StdlibDir: filepath.Join(root, "stdlib")})
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + steps.Assembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble: %v %v", err, a.Errors)
			}
			if 0x5b00+len(a.Binary) > 0xbe00 {
				t.Fatal("resident overlaps stack")
			}

			diskPath := filepath.Join(dir, "save.trd")
			disk := &formats.TRDFile{Path: diskPath, Writeback: true}
			if err := disk.Flush(); err != nil {
				t.Fatal(err)
			}
			failWriteAt, writeCalls := 0, 0
			var stagingMagic []byte
			run := func(disk *formats.TRDFile, restore bool) []byte {
				m, err := spectrum.NewScorpionMemory256(nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				for i, b := range a.Binary {
					m.Memory.Write(0x5b00+uint16(i), b, false)
				}
				pages := []int{0, 1, 3, 4, 6, 7}
				for i, b := range story {
					m.Memory.RAM[pages[i/16384]][i%16384] = b
				}
				for i, b := range archive {
					at := offset + i
					m.Memory.RAM[pages[at/16384]][at%16384] = b
				}
				keyText := bytes.ReplaceAll(route, []byte("\n"), []byte("\r"))
				if restore {
					restoreRoute := []byte("r\nq\n")
					if externalStory != "" {
						restoreRoute = []byte("restore\ninventory\nquit\ny\n")
					}
					keyText = bytes.ReplaceAll(restoreRoute, []byte("\n"), []byte("\r"))
					if !console {
						for i, b := range restoreRoute {
							m.Memory.Write(uint16(a.Symbols["native_commands"])+uint16(i), b, false)
						}
					}
				}
				m.Memory.SetPagingForce(0x10)
				m.Memory.Write(0xbf10, 0x76, false)
				m.Memory.Write(0xbf00, 0x10, false)
				m.Memory.Write(0xbf01, 0xbf, false)
				m.CPU.SetPC(uint16(a.Symbols["main"]))
				m.CPU.SetSP(0xbf00)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				writeCalls = 0
				actualPath := disk.Path
				defer func() { disk.Path = actualPath }()
				sectorSymbol := "zx__scorpion_disk__scorpion_disk_sector"
				if extended {
					sectorSymbol = "zx__scorpion_disk9__scorpion_disk_sector"
				}
				m.SetPCTrap(uint16(a.Symbols[sectorSymbol]), func() {
					if byte(m.CPU.AF()>>8) == 6 {
						writeCalls++
						if failWriteAt == writeCalls {
							disk.Path = filepath.Join(actualPath, "invalid-child")
						}
					}
					m.CPU.DoOpcode()
				})
				formats.InstallTRDTraps(m, disk, false)
				var output []byte
				keyAt, ready := 0, false
				if console {
					keys := []byte{0, 'z', 'x', 'c', 'v', 'a', 's', 'd', 'f', 'g', 'q', 'w', 'e', 'r', 't', '1', '2', '3', '4', '5', '0', '9', '8', '7', '6', 'p', 'o', 'i', 'u', 'y', 13, 'l', 'k', 'j', 'h', 32, 0, 'm', 'n', 'b'}
					m.SetPCTrap(uint16(a.Symbols["zx__console__con_getchar"]), func() {
						m.Keyboard.Reset()
						ready = true
						m.CPU.DoOpcode()
					})
					m.SetPCTrap(uint16(a.Symbols["zx__console__scan_row_key"]), func() {
						if ready {
							if keyAt >= len(keyText) {
								t.Fatal("unexpected keyboard input")
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
						m.CPU.DoOpcode()
					})
				}
				putchar := a.Symbols["native_putchar"]
				if windows {
					putchar = a.Symbols["zx__console__z3_window_put"]
				} else if console {
					putchar = a.Symbols["zx__console__con_putchar"]
				}
				if putchar == 0 {
					t.Fatal("missing output symbol")
				}
				m.SetPCTrap(uint16(putchar), func() { output = append(output, byte(m.CPU.AF()>>8)); m.CPU.DoOpcode() })
				for i := 0; i < 10000; i++ {
					m.RunFrameFast()
					if m.CPU.Halted() {
						break
					}
				}
				if !m.CPU.Halted() || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.Memory.ScorpionPagingState() != 0 {
					t.Fatalf("return boundary PC/SP %04x/%04x keys %d/%d output %q", m.CPU.PC(), m.CPU.SP(), keyAt, len(keyText), output)
				}

				if windows {
					if m.Memory.RAM[0][1]&32 == 0 {
						t.Fatal("window capability lost")
					}
					if restore && m.Memory.Read(uint16(a.Symbols["zx__console__z3_upper_height"])) != 0 {
						t.Fatal("upper window not collapsed after restore")
					}
				}
				if console && keyAt != len(keyText) {
					t.Fatalf("keys %d/%d", keyAt, len(keyText))
				}
				dynamic := int(story[14])<<8 | int(story[15])
				for i := dynamic; i < len(story); i++ {
					if got := m.Memory.RAM[pages[i/16384]][i%16384]; got != story[i] {
						t.Fatalf("static story changed at %d: %02x != %02x", i, got, story[i])
					}
				}
				for i, expected := range archive {
					at := offset + i
					if got := m.Memory.RAM[pages[at/16384]][at%16384]; got != expected {
						t.Fatalf("restart archive changed at %d", at)
					}
				}
				stagingMagic = append([]byte(nil), m.Memory.ExtendedRAM[0][:4]...)
				return output
			}
			first := run(disk, false)
			if extended && !bytes.Equal(stagingMagic, []byte("MZ3R")) {
				t.Fatalf("save not staged in physical bank8: %x", stagingMagic)
			}
			if externalStory == "" && !bytes.Equal(first, []byte(">saved 1\n>\n")) {
				t.Fatalf("save %q", first)
			}
			reopened, err := formats.LoadTRD(diskPath)
			if err != nil {
				t.Fatal(err)
			}
			second := run(reopened, true)
			if externalStory == "" && !bytes.Equal(second, first) {
				t.Fatalf("fresh restore %q save %q", second, first)
			}
			if externalStory != "" {
				for _, data := range [][]byte{first, second} {
					for _, needle := range []string{"Ok.", "You are carrying:\n  A leaflet"} {
						if !bytes.Contains(data, []byte(needle)) {
							t.Fatalf("missing %q: %q", needle, data)
						}
					}
					if bytes.Contains(data, []byte("Z3 error")) {
						t.Fatal("interpreter error")
					}
				}
				if d := os.Getenv("MINZ_Z3_TRANSCRIPTS"); d != "" {
					if err := os.MkdirAll(d, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string][]byte{"disk-save.txt": first, "disk-restore-fresh.txt": second} {
						if err := os.WriteFile(filepath.Join(d, name), data, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			if console {
				t.Logf("console keyboard disk SAVE/fresh RESTORE PASS; resident %d bytes", len(a.Binary))
				return
			}
			// Failed overwrite must leave the previously committed record usable.
			savedPath := disk.Path
			disk.Path = filepath.Join(savedPath, "invalid-child")
			failed := run(disk, false)
			disk.Path = savedPath
			if !bytes.Contains(failed, []byte("failed")) && !bytes.Contains(failed, []byte("Failed.")) {
				t.Fatalf("write failure not reported %q", failed)
			}
			retained, err := formats.LoadTRD(savedPath)
			if err != nil {
				t.Fatal(err)
			}
			retainedOutput := run(retained, true)
			if !bytes.Equal(retainedOutput, second) {
				t.Fatalf("prior save lost %q", retainedOutput)
			}
			{
				// Measure the complete overwrite and inject failures at each write,
				// including the final marker publication. Start from the same disk.
				baseline, err := os.ReadFile(savedPath)
				if err != nil {
					t.Fatal(err)
				}
				if output := run(retained, false); !bytes.Equal(output, first) {
					t.Fatalf("successful overwrite %q", output)
				}
				writes := writeCalls
				if writes < 2 {
					t.Fatal("missing header/marker writes", writes)
				}
				for at := 1; at <= writes; at++ {
					if err := os.WriteFile(savedPath, baseline, 0644); err != nil {
						t.Fatal(err)
					}
					candidate, err := formats.LoadTRD(savedPath)
					if err != nil {
						t.Fatal(err)
					}
					candidate.Writeback = true
					failWriteAt = at
					output := run(candidate, false)
					if !bytes.Contains(output, []byte("save failed")) && !bytes.Contains(output, []byte("Failed.")) {
						t.Fatalf("write%d not rejected %q", at, output)
					}
					failWriteAt = 0
					prior, err := formats.LoadTRD(savedPath)
					if err != nil {
						t.Fatal(err)
					}
					if prior.Data[8192+2] != baseline[8192+2] {
						t.Fatal("marker changed on failure", at)
					}
					if output := run(prior, true); !bytes.Equal(output, second) {
						t.Fatalf("write%d lost prior record %q", at, output)
					}
				}
				t.Logf("all %d write failure positions preserve prior committed save", writes)
			}
			// Corrupt the stored header: restore must reject it before applying state.
			reopened.Data[2*2*16*256] ^= 1
			rejected := run(reopened, true)
			if externalStory == "" && !bytes.Equal(rejected, []byte(">restore failed\n>\n")) {
				t.Fatalf("corrupt restore %q", rejected)
			}
			if externalStory != "" && (bytes.Contains(rejected, []byte("You are carrying:\n  A leaflet")) || !bytes.Contains(rejected, []byte("Failed."))) {
				t.Fatalf("corrupt Zork restore %q", rejected)
			}
			t.Logf("MZ3R disk save and fresh machine restore PASS; resident %d bytes", len(a.Binary))

		})
	}
}
