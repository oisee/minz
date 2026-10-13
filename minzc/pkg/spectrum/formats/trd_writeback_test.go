package formats

import (
	"bytes"
	"github.com/minz/minzc/pkg/spectrum"
	"os"
	"path/filepath"
	"testing"
)

func TestTRDFlushPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.trd")
	disk := &TRDFile{Path: path}
	for i := range disk.Data {
		disk.Data[i] = byte(i*17 + 31)
	}
	if err := disk.Flush(); err != nil {
		t.Fatal(err)
	}
	reopened, err := LoadTRD(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Data != disk.Data {
		t.Fatal("initial image differs")
	}
	payload := bytes.Repeat([]byte{0xa7}, 256)
	disk.WriteSector(2, 1, 15, payload)
	if err := disk.Flush(); err != nil {
		t.Fatal(err)
	}
	reopened, err = LoadTRD(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Data != disk.Data {
		t.Fatal("modified image differs after reopen")
	}
	if !bytes.Equal(reopened.ReadSector(2, 1, 15), payload) {
		t.Fatal("sector not persisted")
	}
	if reopened.Writeback {
		t.Fatal("read-only load unexpectedly enabled writes")
	}
}

func TestTRDFlushFailureLeavesPriorImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save.trd")
	previous := []byte("prior image")
	if err := os.WriteFile(path, previous, 0600); err != nil {
		t.Fatal(err)
	}
	disk := &TRDFile{Path: filepath.Join(path, "invalid-child")}
	if err := disk.Flush(); err == nil {
		t.Fatal("expected invalid destination failure")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, previous) {
		t.Fatal("prior file changed")
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatal("temporary file leaked")
	}
}

func TestTRDSectorTrapWriteback(t *testing.T) {
	for _, mode := range []string{"persist", "read-only", "failure"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "save.trd")
			disk := &TRDFile{Path: path, Writeback: mode != "read-only"}
			if err := disk.Flush(); err != nil {
				t.Fatal(err)
			}
			before := disk.Data
			if mode == "failure" {
				disk.Path = filepath.Join(path, "invalid")
			}
			m, err := spectrum.NewPentagon128(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 256; i++ {
				m.Memory.Write(0x8000+uint16(i), byte(i^0xa5), false)
			}
			m.Memory.Write(0x7000, 0x76, false)
			m.Memory.Write(0xbf00, 0, false)
			m.Memory.Write(0xbf01, 0x70, false)
			m.CPU.SetPC(TRDOSEntryPoint)
			m.CPU.SetSP(0xbf00)
			m.CPU.SetHL(0x8000)
			m.CPU.SetBC(0x0106)
			m.CPU.SetDE(0x0201)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			InstallTRDTraps(m, disk, false)
			m.RunFrameFast()
			if m.CPU.SP() != 0xbf02 {
				t.Fatal("stack", m.CPU.SP())
			}
			reopened, err := LoadTRD(path)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "persist":
				if disk.LastWriteError != nil || reopened.Data != disk.Data || disk.Data == before {
					t.Fatal("write not persisted")
				}
			case "read-only":
				if reopened.Data != before || disk.Data == before {
					t.Fatal("read-only policy")
				}
			case "failure":
				if disk.LastWriteError == nil || disk.Data != before || reopened.Data != before || m.CPU.BC() != 0x0100 {
					t.Fatal("failed write not rolled back")
				}
			}
		})
	}
}
