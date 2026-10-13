package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
	"path/filepath"
	"testing"
)

func TestScorpionDiskSectorAcrossRestart(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.scorpion_disk\nfun save()->u8{return scorpion_disk_sector(6,256,2,1)}\nfun load()->u8{return scorpion_disk_sector(5,256,2,1)}", "disk.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	path := filepath.Join(t.TempDir(), "save.trd")
	disk := &formats.TRDFile{Path: path, Writeback: true}
	if err = disk.Flush(); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i*17 + 91)
	}
	run := func(d *formats.TRDFile, name string, initial []byte) *spectrum.Machine {
		m, err := spectrum.NewScorpionMemory256(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range a.Binary {
			m.Memory.Write(0x8000+uint16(i), b, false)
		}
		for i, b := range initial {
			m.Memory.WriteRAMDirect(8, 256+uint16(i), b)
		}
		entry := a.Symbols[name]
		for i, b := range []byte{0xcd, byte(entry), byte(entry >> 8), 0x76} {
			m.Memory.Write(0x7000+uint16(i), b, false)
		}
		m.Memory.SetPaging(0x10)
		m.CPU.SetPC(0x7000)
		m.CPU.SetSP(0xbf00)
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		formats.InstallTRDTraps(m, d, false)
		m.RunFrameFast()
		if m.CPU.SP() != 0xbf00 || !m.CPU.Halted() || byte(m.CPU.AF()>>8) != 1 || m.Memory.PagingState() != 0x10 || m.Memory.ScorpionPagingState() != 0 || m.CPU.IFF1() || m.CPU.IFF2() {
			t.Fatal("sector boundary", name, m.CPU.PC(), m.CPU.SP(), m.CPU.AF())
		}
		return m
	}
	run(disk, "save", payload)
	reopened, err := formats.LoadTRD(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reopened.ReadSector(2, 0, 0), payload) {
		t.Fatal("disk content")
	}
	fresh := run(reopened, "load", nil)
	for i, b := range payload {
		if fresh.Memory.ReadRAMDirect(8, 256+uint16(i)) != b {
			t.Fatal("fresh machine load", i)
		}
	}
	t.Log("compiled Nanz SAVE-sector, reopened TRD, fresh machine LOAD-sector PASS")
}
