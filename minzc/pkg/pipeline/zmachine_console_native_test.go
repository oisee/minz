package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
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
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
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
	t.Logf("module %d bytes; stack %d", len(a.Binary), 0xf000-int(minSP))
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
