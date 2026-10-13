package pipeline

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
)

// Independent Inform story: SAVE branches again after RESTORE, recovering
// value=1 even though the restore command sets it to 7 first.
func TestZMachineDiskSaveAcrossRestart(t *testing.T) {
	root := filepath.Clean("../../..")
	original, err := os.ReadFile(filepath.Join(root, "examples/zmachine/z3-fixtures/save_restore.z3"))
	if err != nil {
		t.Fatal(err)
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
			if err := os.WriteFile(commands, []byte("s\nq\n"), 0600); err != nil {
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
			h, err := nanz.ParseWithOpts(string(src), source, nanz.ParseOpts{StdlibDir: filepath.Join(root, "stdlib")})
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
					m.Memory.Write(uint16(a.Symbols["native_commands"]), 'r', false)
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
				for i := 0; i < 10000; i++ {
					m.RunFrameFast()
					if m.CPU.Halted() {
						break
					}
				}
				if !m.CPU.Halted() || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 || m.Memory.ScorpionPagingState() != 0 {
					t.Fatal("return boundary", m.CPU.PC(), m.CPU.SP())
				}
				addr := uint16(a.Symbols["native_output_length"])
				n := int(m.Memory.Read(addr)) | int(m.Memory.Read(addr+1))<<8
				output := make([]byte, n)
				for i := range output {
					output[i] = m.Memory.Read(uint16(a.Symbols["native_output"]) + uint16(i))
				}
				return output
			}
			first := run(disk, false)
			if !bytes.Equal(first, []byte(">saved 1\n>\n")) {
				t.Fatalf("save %q", first)
			}
			reopened, err := formats.LoadTRD(diskPath)
			if err != nil {
				t.Fatal(err)
			}
			second := run(reopened, true)
			if !bytes.Equal(second, first) {
				t.Fatalf("fresh restore %q save %q", second, first)
			}
			// Corrupt the stored header: restore must reject it before applying state.
			reopened.Data[2*2*16*256] ^= 1
			rejected := run(reopened, true)
			if !bytes.Equal(rejected, []byte(">restore failed\n>\n")) {
				t.Fatalf("corrupt restore %q", rejected)
			}
			t.Logf("MZ3R disk save and fresh machine restore PASS; resident %d bytes", len(a.Binary))

		})
	}
}
