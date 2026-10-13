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

// Transport-only reduction: the encoder publishes a known 1932-byte record.
// The actual disk loops, paging, sector API and return stack execute as Z80.
func TestZMachineDiskMultiSectorTransportNative(t *testing.T) {
	root := filepath.Clean("../../..")
	storyPath := filepath.Join(root, "examples/zmachine/z3-fixtures/save_restore.z3")
	story, err := os.ReadFile(storyPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "restart.rle")
	pack := exec.Command("python3", filepath.Join(root, "scripts/zmachine/restart_rle.py"), storyPath, archivePath)
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("pack %v %s", err, out)
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "native.nanz")
	gen := exec.Command("python3", filepath.Join(root, "scripts/zmachine/prepare_native_core.py"), source, "--story-bytes", strconv.Itoa(len(story)), "--restart-offset", "16384", "--restart-bytes", strconv.Itoa(int(story[14])<<8|int(story[15])), "--restart-rle-bytes", strconv.Itoa(len(archive)), "--save-story", storyPath, "--disk-save", "--extended-save", "--console", "--windows")
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate %v %s", err, out)
	}
	src, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), source, nanz.ParseOpts{BaseDir: dir, StdlibDir: filepath.Join(root, "stdlib")})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, a.Errors)
	}
	m, err := spectrum.NewScorpionMemory256(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x5b00+uint16(i), b, false)
	}
	record := make([]byte, 1932)
	for i := range record {
		record[i] = byte(i*37 + (i>>8)*19 + 11)
	}
	copy(record, []byte("MZ3R"))
	copy(m.Memory.ExtendedRAM[0][:], record)
	addr := uint16(a.Symbols["native_save_length"])
	m.Memory.Write(addr, byte(len(record)&255), false)
	m.Memory.Write(addr+1, byte(len(record)>>8), false)
	m.SetPCTrap(uint16(a.Symbols["native_save"]), func() {
		sp := m.CPU.SP()
		pc := uint16(m.Memory.Read(sp)) | uint16(m.Memory.Read(sp+1))<<8
		m.CPU.SetSP(sp + 2)
		m.CPU.SetPC(pc)
		m.CPU.SetAF(0x0100)
	})
	disk := &formats.TRDFile{Path: filepath.Join(dir, "save.trd"), Writeback: true}
	if err := disk.Flush(); err != nil {
		t.Fatal(err)
	}
	m.SetPCTrap(uint16(a.Symbols["zx__scorpion_disk9__scorpion_disk_sector"]), func() {
		t.Logf("sector op%d offset%04x track%d sec%d staging %x", m.CPU.AF()>>8, m.CPU.HL(), m.CPU.DE()>>8, m.CPU.DE()&255, m.Memory.ExtendedRAM[1][:8])
		m.CPU.DoOpcode()
	})
	formats.InstallTRDTraps(m, disk, false)
	m.Memory.SetPagingForce(0x10)
	m.Memory.Write(0xbf00, 0x10, false)
	m.Memory.Write(0xbf01, 0xbf, false)
	m.CPU.SetPC(uint16(a.Symbols["native_disk_save"]))
	m.CPU.SetSP(0xbf00)
	m.CPU.SetIFF1(false)
	m.CPU.SetIFF2(false)
	for i := 0; i < 2000000; i++ {
		if m.CPU.PC() == 0xbf10 && m.CPU.SP() == 0xbf02 {
			break
		}
		if !m.HandlePCTrap() {
			m.CPU.DoOpcode()
		}
		if m.CPU.SP() < 0xbe00 {
			t.Fatalf("stack escape PC/SP %04x/%04x", m.CPU.PC(), m.CPU.SP())
		}
	}
	if m.CPU.PC() != 0xbf10 || m.CPU.SP() != 0xbf02 || byte(m.CPU.AF()>>8) != 1 {
		t.Fatalf("disk transport PC/SP %04x/%04x A%d", m.CPU.PC(), m.CPU.SP(), m.CPU.AF()>>8)
	}
	persisted, err := os.ReadFile(disk.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(persisted, record) {
		at := bytes.Index(persisted, record[:32])
		first := -1
		if at >= 0 {
			for i, b := range record {
				if persisted[at+i] != b {
					first = i
					break
				}
			}
		}
		t.Fatalf("full multisector record not persisted header at %d first mismatch %d slot head %x", at, first, persisted[8192:8208])
	}
	if m.Memory.PagingState() != 0x10 || m.Memory.ScorpionPagingState() != 0 {
		t.Fatal("paging not restored")
	}
}
