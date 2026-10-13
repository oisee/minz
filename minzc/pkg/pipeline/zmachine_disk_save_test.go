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
	testZMachineDiskSave(t, path)
}

func TestZMachineDiskSaveAcrossRestart(t *testing.T) { testZMachineDiskSave(t, "") }

// Independent Inform story: SAVE branches again after RESTORE, recovering
// value=1 even though the restore command sets it to 7 first.
func testZMachineDiskSave(t *testing.T, externalStory string) {
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
			if out, err := gen.CombinedOutput(); err != nil {
				t.Fatalf("generate: %v %s", err, out)
			}
			src, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			// Capture output by observing entry without allocating a bounded output buffer.
			capture := string(src)
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
			h, err := nanz.ParseWithOpts(capture, source, nanz.ParseOpts{StdlibDir: filepath.Join(root, "stdlib")})
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
				if restore {
					restoreRoute := []byte("r\nq\n")
					if externalStory != "" {
						restoreRoute = []byte("restore\ninventory\nquit\ny\n")
					}
					for i, b := range restoreRoute {
						m.Memory.Write(uint16(a.Symbols["native_commands"])+uint16(i), b, false)
					}
				}
				m.Memory.SetPagingForce(0x10)
				m.Memory.Write(0x5000, 0x76, false)
				m.Memory.Write(0xbf00, 0, false)
				m.Memory.Write(0xbf01, 0x50, false)
				m.CPU.SetPC(uint16(a.Symbols["main"]))
				m.CPU.SetSP(0xbf00)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				formats.InstallTRDTraps(m, disk, false)
				var output []byte
				m.SetPCTrap(uint16(a.Symbols["native_putchar"]), func() { output = append(output, byte(m.CPU.AF()>>8)); m.CPU.DoOpcode() })
				for i := 0; i < 10000; i++ {
					m.RunFrameFast()
					if m.CPU.Halted() {
						break
					}
				}
				if !m.CPU.Halted() || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.Memory.ScorpionPagingState() != 0 {
					t.Fatal("return boundary", m.CPU.PC(), m.CPU.SP())
				}

				return output
			}
			first := run(disk, false)
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
