package pipeline

import (
	"encoding/binary"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Full imported console executes real ASM; expected pixels follow the Spectrum
// address specification, not the console helpers. All 64KB writes are checked.
func TestZMachineConsoleScrollNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.console\nfun main() { con_scroll() }", "scroll.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + calledMainTestAssembly(t, s))
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	for i := range z.Memory() {
		z.Memory()[i] = byte(i*41 + 29)
	}
	z.LoadMemory(0x8000, a.Binary)
	attr := a.Symbols["zx__console__con_attr"]
	z.Memory()[attr] = 0x47
	addr := func(row, line, col int) int {
		return 0x4000 | ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | col
	}
	for row := 0; row < 24; row++ {
		for line := 0; line < 8; line++ {
			for col := 0; col < 32; col++ {
				z.Memory()[addr(row, line, col)] = byte(row*11 + line*7 + col)
			}
		}
		for col := 0; col < 32; col++ {
			z.Memory()[0x5800+row*32+col] = byte(row*5 + col)
		}
	}
	target := a.Symbols["main"]
	z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
	before := append([]byte(nil), z.Memory()...)
	want := append([]byte(nil), before...)
	for row := 0; row < 24; row++ {
		for line := 0; line < 8; line++ {
			for col := 0; col < 32; col++ {
				v := byte(0)
				if row < 23 {
					v = before[addr(row+1, line, col)]
				}
				want[addr(row, line, col)] = v
			}
		}
		for col := 0; col < 32; col++ {
			v := byte(0x47)
			if row < 23 {
				v = before[0x5800+(row+1)*32+col]
			}
			want[0x5800+row*32+col] = v
		}
	}
	z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
	minSP := uint16(0xf000)
	for i := 0; i < 2000000; i++ {
		if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
			break
		}
		z.Step()
		if z.GetSP() < minSP {
			minSP = z.GetSP()
		}
	}
	if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 {
		t.Fatalf("PC/SP=%04x/%04x", z.GetPC(), z.GetSP())
	}
	for i := range want {
		if i >= int(minSP) && i < 0xf000 {
			continue
		}
		if z.Memory()[i] != want[i] {
			t.Fatalf("memory%04x got%02x want%02x", i, z.Memory()[i], want[i])
		}
	}
	t.Logf("module %d bytes; stack %d; %d T including CALL", len(a.Binary), 0xf000-int(minSP), z.GetCycles())
}

func TestZMachineConsoleScrollRegionNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.console\nfun main() { con_scroll_region(0) }", "scroll.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
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
	tops := make([]int, 26)
	for i := 0; i <= 24; i++ {
		tops[i] = i
	}
	tops[25] = 255
	for _, top := range tops {
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		for i := range z.Memory() {
			z.Memory()[i] = byte(i*41 + 29)
		}
		z.LoadMemory(0x8000, a.Binary)
		attr := a.Symbols["zx__console__con_attr"]
		z.Memory()[attr] = 0x47
		addr := func(row, line, col int) int {
			return 0x4000 | ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | col
		}
		for row := 0; row < 24; row++ {
			for line := 0; line < 8; line++ {
				for col := 0; col < 32; col++ {
					z.Memory()[addr(row, line, col)] = byte(row*11 + line*7 + col)
				}
			}
			for col := 0; col < 32; col++ {
				z.Memory()[0x5800+row*32+col] = byte(row*5 + col)
			}
		}
		target := a.Symbols["zx__console__con_scroll_region"]
		z.LoadMemory(0x7000, []byte{0x3e, byte(top), 0xcd, byte(target), byte(target >> 8), 0x76})
		before := append([]byte(nil), z.Memory()...)
		want := append([]byte(nil), before...)
		for row := top; row < 24; row++ {
			for line := 0; line < 8; line++ {
				for col := 0; col < 32; col++ {
					v := byte(0)
					if row < 23 {
						v = before[addr(row+1, line, col)]
					}
					want[addr(row, line, col)] = v
				}
			}
			for col := 0; col < 32; col++ {
				v := byte(0x47)
				if row < 23 {
					v = before[0x5800+(row+1)*32+col]
				}
				want[0x5800+row*32+col] = v
			}
		}
		z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
		minSP := uint16(0xf000)
		for i := 0; i < 2000000; i++ {
			if z.GetPC() == 0x7005 && z.GetSP() == 0xf000 {
				break
			}
			z.Step()
			if z.GetSP() < minSP {
				minSP = z.GetSP()
			}
		}
		if z.GetPC() != 0x7005 || z.GetSP() != 0xf000 {
			t.Fatalf("PC/SP=%04x/%04x", z.GetPC(), z.GetSP())
		}
		for i := range want {
			if i >= int(minSP) && i < 0xf000 {
				continue
			}
			if z.Memory()[i] != want[i] {
				t.Fatalf("top%d memory%04x got%02x want%02x", top, i, z.Memory()[i], want[i])
			}
		}
		if top == 1 {
			t.Logf("module %d bytes; stack %d; %d T for top=1", len(a.Binary), 0xf000-int(minSP), z.GetCycles())
		}
	}
}

func TestZMachineConsoleResidentBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.nanz")
	cmd := exec.Command("python3", "../../../scripts/zmachine/prepare_native_core.py", path, "--story-bytes", "86928", "--console")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate %v %s", err, out)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + s.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	t.Logf("console resident %d bytes ending %04x", len(a.Binary), 0x5b00+len(a.Binary))
	if 0x5b00+len(a.Binary) > 0xbe00 {
		t.Fatalf("console resident overlaps reserved native stack")
	}
}

// Execute the real generated verify_story against independently summed bytes.
func TestZMachineChecksumNative(t *testing.T) {
	path := filepath.Join(t.TempDir(), "core.nanz")
	cmd := exec.Command("python3", "../../../scripts/zmachine/prepare_native_core.py", path, "--story-bytes", "98304")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate %v %s", err, out)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x5b00\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	for _, length := range []int{0, 62, 64, 66, 16384, 16386, 32768, 65536, 65538, 98304, 98306} {
		for _, corrupt := range []bool{false, true} {
			m, err := spectrum.NewPentagon128(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i, b := range a.Binary {
				m.Memory.Write(0x5b00+uint16(i), b, false)
			}
			pages := []int{0, 1, 3, 4, 6, 7}
			var sum uint16
			for i := 0; i < 98304; i++ {
				b := byte(i*37 + (i>>14)*17)
				m.Memory.RAM[pages[i/16384]][i%16384] = b
				if i >= 64 && i < length {
					sum += uint16(b)
				}
			}
			binary.BigEndian.PutUint16(m.Memory.RAM[0][26:28], uint16(length/2))
			if corrupt {
				sum++
			}
			binary.BigEndian.PutUint16(m.Memory.RAM[0][28:30], sum)
			at := uint16(a.Symbols["story_size"])
			for i, b := range []byte{0, 0x80, 1, 0} {
				m.Memory.Write(at+uint16(i), b, false)
			}
			m.Memory.SetPagingForce(0x10)
			m.Memory.Write(0xbf00, 0, false)
			m.Memory.Write(0xbf01, 0x50, false)
			before := m.Memory.RAM
			m.CPU.SetPC(uint16(a.Symbols["verify_story"]))
			m.CPU.SetSP(0xbf00)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			for i := 0; i < 1500000; i++ {
				if m.CPU.PC() == 0x5000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			want := byte(0)
			if length >= 64 && length <= 98304 && !corrupt {
				want = 1
			}
			if m.CPU.PC() != 0x5000 || m.CPU.SP() != 0xbf02 || byte(m.CPU.AF()>>8) != want || m.Memory.PagingState() != 0x10 || m.CPU.IFF1() {
				t.Fatalf("length%d corrupt%v result%02x want%02x PC/SP%04x/%04x", length, corrupt, m.CPU.AF()>>8, want, m.CPU.PC(), m.CPU.SP())
			}
			for _, page := range pages {
				if before[page] != m.Memory.RAM[page] {
					t.Fatal("checksum wrote story", page)
				}
			}
		}
	}
}
