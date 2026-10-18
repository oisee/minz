package abap

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func zxFleetDisk(t *testing.T, payload []byte, cross bool) *formats.TRDFile {
	t.Helper()
	d := &formats.TRDFile{}
	extra, e := os.ReadFile("../../../examples/abap/fleet/EXTRA.CSV")
	if e != nil {
		t.Fatal(e)
	}
	for i, n := range []string{"EXTRA", "FLEET"} {
		data := extra
		start := 16
		if i == 1 {
			data = payload
			start = 17
			if cross {
				start = 31
			}
		}
		p := i * 16
		copy(d.Data[p:p+8], []byte(fmt.Sprintf("%-8s", n)))
		copy(d.Data[p+8:p+11], []byte("CSV"))
		binary.LittleEndian.PutUint16(d.Data[p+11:], uint16(len(data)))
		d.Data[p+13] = byte((len(data) + 255) / 256)
		d.Data[p+14] = byte(start % 16)
		d.Data[p+15] = byte(start / 16)
		copy(d.Data[start*256:], data)
	}
	return d
}
func zxFleetExpected(t *testing.T, seats int) []byte {
	t.Helper()
	fontHex, e := os.ReadFile("testdata/zx-grid/font.hex")
	if e != nil {
		t.Fatal(e)
	}
	font, e := hex.DecodeString(strings.TrimSpace(string(fontHex)))
	if e != nil {
		t.Fatal(e)
	}
	var rows [24]string
	rows[0] = "ALV Rows: 2 First: 1"
	border := "+--------+--------+-----------+"
	rows[1] = border
	rows[2] = "|NAME    |ROUTE   |      SEATS|"
	rows[3] = border
	// Headings are left aligned, unlike signed numeric values.
	rows[2] = "|NAME    |ROUTE   |SEATS      |"
	rows[4] = fmt.Sprintf("|%-8s|%-8s|%11d|", "Atlas", "North", seats)
	rows[5] = "|Beacon  |South   |         32|"
	rows[22] = border
	rows[23] = "Arrows: scroll  Q: Back"
	bitmap := make([]byte, 6144)
	for y, row := range rows {
		row += strings.Repeat(" ", 32-len(row))
		for x, ch := range []byte(row) {
			for line := 0; line < 8; line++ {
				pixelY := y*8 + line
				addr := ((pixelY & 0xc0) << 5) | ((pixelY & 7) << 8) | ((pixelY & 0x38) << 2) | x
				bitmap[addr] = font[(int(ch)-32)*8+line]
			}
		}
	}
	return bitmap
}

func TestZXFrontendFleetNative(t *testing.T) {
	source, e := os.ReadFile("../../../examples/abap/zfleet_csv_alv.abap")
	if e != nil {
		t.Fatal(e)
	}
	hm, e := Compile(string(source), "zfleet_csv_alv.abap", hir.TargetZXSpectrum)
	if e != nil {
		t.Fatal(e)
	}
	steps, e := pipeline.CompileHIRSteps(hm, pipeline.Options{Backend: "z80", AssertMode: "none", ContractOpt: true})
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString(steps.Assembly)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	payload, e := os.ReadFile("../../../examples/abap/fleet/FLEET.CSV")
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name                     string
		model48, cross, packaged bool
		data                     []byte
		seats, total, status     int
	}{
		{name: "128k", data: payload, seats: 40, total: 72}, {name: "48k", model48: true, data: payload, seats: 40, total: 72},
		{name: "packaged-sna", model48: true, packaged: true, data: payload, seats: 40, total: 72},
		{name: "packaged-bad-extent", model48: true, packaged: true, data: payload, status: 8},
		{name: "cross-logical-track", cross: true, data: []byte("Atlas,North,40" + strings.Repeat(" ", 130) + "\r\nBeacon,South,32" + strings.Repeat(" ", 130) + "\r\n"), seats: 40, total: 72},
		{name: "changed-disk-data", data: bytes.Replace(payload, []byte("40"), []byte("41"), 1), seats: 41, total: 73},
		{name: "bad-number", data: []byte("Atlas,North,nope\n"), status: 4},
		{name: "bad-extent", data: payload, status: 8},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m *spectrum.Machine
			if c.model48 {
				m, e = spectrum.New48K(nil)
			} else {
				m, e = spectrum.NewPentagon128(nil, nil)
			}
			if e != nil {
				t.Fatal(e)
			}
			for i, b := range a.Binary {
				m.Memory.Write(uint16(0x6000+i), b, false)
			}
			// A real native caller invokes the report, stores returned status and halts.
			boot := []byte{0xf3, 0x31, 0, 0xff, 0xdd, 0x21, 0x34, 0x12, 0xfd, 0x21, 0x78, 0x56, 0xcd, 0, 0x60, 0x32, 0x20, 0x5b, 0x76, 0xc3, 0x12, 0x5b}
			for i, b := range boot {
				m.Memory.Write(0x5b00+uint16(i), b, false)
			}
			m.CPU.SetPC(0x5b00)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			if c.packaged {
				packaged, errs := pipeline.Assemble(steps.Assembly, "zxspectrum")
				if len(errs) > 0 {
					t.Fatal(errs)
				}
				path := filepath.Join(t.TempDir(), "fleet.sna")
				if e := os.WriteFile(path, packaged, 0600); e != nil {
					t.Fatal(e)
				}
				snap, e := formats.LoadSNA(path)
				if e != nil {
					t.Fatal(e)
				}
				formats.ApplySnapshot(m, snap)
			}

			disk := zxFleetDisk(t, c.data, c.cross)
			if strings.HasSuffix(c.name, "bad-extent") {
				disk.Data[16+13] = 0
			}
			formats.InstallTRDTraps(m, disk, false)
			var profile *spectrum.Profiler
			if c.name == "128k" {
				profile = spectrum.NewProfiler()
				m.SetProfiler(profile)
			}
			// The only host boundary is the sector trap. Keyboard input travels through
			// the actual matrix; parsing, typed rows, numerics and pixels execute on Z80.
			var grid []byte
			for frame := 0; frame < 500; frame++ {
				m.Keyboard.Reset()
				switch {
				case frame >= 50 && frame < 60:
					m.Keyboard.KeyPress(1, 3)
				case frame >= 110 && frame < 120:
					m.Keyboard.KeyPress(3, 1)
				case frame >= 170 && frame < 180:
					m.Keyboard.KeyPress(6, 0)
				case frame >= 230 && frame < 240:
					m.Keyboard.KeyPress(6, 0)
				case frame >= 300 && frame < 310:
					m.Keyboard.KeyPress(1, 3)
				case frame >= 400 && frame < 410:
					m.Keyboard.KeyPress(2, 0)
				}
				m.RunFrameFast()
				if frame == 40 {
					fontHex, _ := os.ReadFile("testdata/zx-grid/font.hex")
					font, _ := hex.DecodeString(strings.TrimSpace(string(fontHex)))
					for x, ch := range []byte("P_FILE [EXTRA.CSV]: ") {
						for line := 0; line < 8; line++ {
							if got, want := m.Memory.Read(uint16(0x4000+line*256+x)), font[(int(ch)-32)*8+line]; got != want {
								t.Fatalf("initial prompt cell%d line%d got%02x want%02x", x, line, got, want)
							}
						}
					}
				}

				if frame == 350 {
					if c.status == 0 && (m.CPU.PC() == 0x5b12 || m.CPU.PC() == 0x5b1b) {
						t.Fatal("value-help key unexpectedly exited ALV")
					}
					grid = make([]byte, 6912)
					for i := range grid {
						grid[i] = m.Memory.ReadScreen(uint16(i))
					}
				}
			}
			expectedPC := uint16(0x5b12)
			if c.packaged {
				expectedPC = 0x5b1b
				expectedBorder := byte(7)
				if c.status != 0 {
					expectedBorder = 2
				}
				if m.ULA.BorderColor() != expectedBorder {
					t.Fatal("caller return border")
				}
			}
			status := int(m.Memory.Read(0x5b20))
			if status != c.status || m.CPU.SP() != 0xff00 || m.CPU.IX() != 0x1234 || m.CPU.IY() != 0x5678 || m.CPU.PC() != expectedPC {
				t.Fatalf("status=%d want=%d PC=%04x SP=%04x IX=%04x IY=%04x", status, c.status, m.CPU.PC(), m.CPU.SP(), m.CPU.IX(), m.CPU.IY())
			}
			if c.status == 0 {
				total := uint32(0)
				for i := 0; i < 4; i++ {
					total |= uint32(m.Memory.Read(uint16(a.Symbols["total"]+i))) << uint(8*i)
				}
				if int(total) != c.total {
					t.Fatal("total", total, c.total)
				}
				if !bytes.Equal(grid[:6144], zxFleetExpected(t, c.seats)) {
					for i, b := range grid[:6144] {
						if b != zxFleetExpected(t, c.seats)[i] {
							t.Fatalf("independent grid bitmap mismatch at %04x got%02x want%02x", 0x4000+i, b, zxFleetExpected(t, c.seats)[i])
						}
					}
				}
				for _, v := range grid[6144:] {
					if v != 0x47 {
						t.Fatal("attribute", v)
					}
				}
				base := a.Symbols["fleet"]
				got := make([]byte, 88)
				for i := range got {
					got[i] = m.Memory.Read(uint16(base + i))
				}
				want := append([]byte(fmt.Sprintf("%-20s%-20s", "Atlas", "North")), byte(c.seats), 0, 0, 0)
				want = append(want, []byte(fmt.Sprintf("%-20s%-20s", "Beacon", "South"))...)
				want = append(want, 32, 0, 0, 0)
				if !bytes.Equal(got, want) {
					t.Fatal("typed rows", got, want)
				}
			}
			minSP := uint16(0)
			if profile != nil {
				minSP = profile.StackDepth
				if int(minSP) < a.Symbols["__zx_app_stack_bottom"]+2 || int(minSP) >= a.Symbols["__zx_app_stack_top"] {
					t.Fatal("resident stack boundary", minSP)
				}
				for pc := 0; pc < 0x4000; pc++ {
					if profile.ExecCount[pc] != 0 {
						t.Fatalf("unexpected ROM execution at %04x", pc)
					}
				}
			}
			if root := os.Getenv("MINZ_ABAP_ZX_ARTIFACTS"); root != "" {
				os.MkdirAll(root, 0755)
				meta, _ := json.MarshalIndent(map[string]any{"case": c.name, "bytes": len(a.Binary), "origin": 0x6000, "image_end": 0x6000 + len(a.Binary), "status": status, "minimum_sp": minSP, "stack_bottom": a.Symbols["__zx_app_stack_bottom"], "stack_top": a.Symbols["__zx_app_stack_top"], "pc": m.CPU.PC(), "sp": m.CPU.SP(), "IX": m.CPU.IX(), "IY": m.CPU.IY(), "total": c.total, "symbols": a.Symbols}, "", "  ")
				for ext, data := range map[string][]byte{"json": meta, "bin": a.Binary, "a80": []byte(steps.Assembly), "hir": []byte(steps.HIR), "raw.mir": []byte(steps.MIR2Raw), "final.mir": []byte(steps.MIR2Opt), "scr": grid} {
					if e := os.WriteFile(filepath.Join(root, c.name+"."+ext), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
				disk.Path = filepath.Join(root, c.name+".trd")
				if e := disk.Flush(); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
