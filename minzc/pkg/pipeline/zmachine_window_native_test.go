package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Standard 1.1 8.6.1-8.6.3: row0 is preserved; upper is cleared and
// never scrolls, selection resets upper cursor but preserves lower cursor.
func TestZMachineV3WindowsNative(t *testing.T) {
	h, e := nanz.ParseWithOpts(`import zx.console
fun main() { z3_windows_reset() z3_window_split(2) z3_window_select(1) z3_window_put(65) }`, "windows.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if e != nil {
		t.Fatal(e)
	}
	s, e := CompileHIRSteps(h, Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + calledMainTestAssembly(t, s))
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	z.LoadMemory(0x8000, a.Binary)
	for i := 0x4000; i < 0x5b00; i++ {
		z.Memory()[i] = 0x55
	}
	call := func(name string, value uint16) {
		target, ok := a.Symbols["zx__console__"+name]
		if !ok {
			t.Fatal("missing", name)
		}
		z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
		z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: byte(value), HL: value})
		for i := 0; i < 2000000; i++ {
			if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
				return
			}
			z.Step()
		}
		t.Fatal("return boundary", name, z.GetPC(), z.GetSP())
	}
	target := a.Symbols["main"]
	z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
	z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
	for i := 0; i < 2000000; i++ {
		if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
			break
		}
		z.Step()
	}
	if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 {
		t.Fatal("return", z.GetPC(), z.GetSP())
	}
	for line := 0; line < 8; line++ {
		for col := 0; col < 32; col++ {
			if z.Memory()[0x4000+line*256+col] != 0x55 {
				t.Fatal("status changed")
			}
		}
	}

	for line := 0; line < 8; line++ {
		for x := 0; x < 32; x++ {
			adr := 0x4000 | (line << 8) | (2 << 5) | x
			if z.Memory()[adr] != 0 {
				t.Fatal("split did not clear upper", line, x)
			}
		}
	}
	if z.Memory()[a.Symbols["zx__console__con_row"]] != 1 || z.Memory()[a.Symbols["zx__console__con_col"]] != 1 {
		t.Fatal("upper cursor", z.Memory()[a.Symbols["zx__console__con_row"]], z.Memory()[a.Symbols["zx__console__con_col"]], z.Memory()[a.Symbols["zx__console__z3_upper_height"]], z.Memory()[a.Symbols["zx__console__z3_selected_window"]])
	}
	row := a.Symbols["zx__console__con_row"]
	col := a.Symbols["zx__console__con_col"]
	call("z3_window_select", 0)
	if z.Memory()[row] != 23 || z.Memory()[col] != 0 {
		t.Fatal("lower cursor not restored")
	}
	z.Memory()[col] = 7
	call("z3_window_select", 1)
	if z.Memory()[row] != 1 || z.Memory()[col] != 0 {
		t.Fatal("upper selection not reset")
	}
	z.Memory()[row] = 2
	z.Memory()[col] = 31
	before := append([]byte(nil), z.Memory()[0x4000:0x5b00]...)
	call("z3_window_put", 13)
	for i, b := range before {
		if z.Memory()[0x4000+i] != b {
			t.Fatal("upper newline scrolled", i)
		}
	}
	call("z3_window_select", 0)
	if z.Memory()[row] != 23 || z.Memory()[col] != 7 {
		t.Fatal("independent lower cursor")
	}
	top := append([]byte(nil), z.Memory()[0x4000:0x5b00]...)
	call("z3_window_put", 13)
	for r := 0; r < 3; r++ {
		for line := 0; line < 8; line++ {
			for x := 0; x < 32; x++ {
				adr := 0x4000 | ((r & 0x18) << 8) | (line << 8) | ((r & 7) << 5) | x
				if z.Memory()[adr] != top[adr-0x4000] {
					t.Fatal("lower scroll changed upper", r, line, x)
				}
			}
		}
	}
	call("z3_windows_reset", 0)
	if z.Memory()[a.Symbols["zx__console__z3_upper_height"]] != 0 || z.Memory()[row] != 23 {
		t.Fatal("reset")
	}

	for _, height := range []uint16{0, 1, 2, 22, 23, 24, 255, 65535} {
		call("z3_windows_reset", 0)
		call("z3_window_split", height)
		expected := byte(23)
		if height < 23 {
			expected = byte(height)
		}
		if z.Memory()[a.Symbols["zx__console__z3_upper_height"]] != expected {
			t.Fatal("height clamp", height)
		}
		call("z3_window_select", 1)
		z.Memory()[row] = expected
		z.Memory()[col] = 31
		before := append([]byte(nil), z.Memory()[0x4000:0x5b00]...)
		call("z3_window_put", 13)
		for i, b := range before {
			if z.Memory()[0x4000+i] != b {
				t.Fatal("upper overflow scroll", height, i)
			}
		}
		if expected == 23 {
			call("z3_window_select", 0)
			call("z3_window_put", 65)
			for i, b := range before {
				if z.Memory()[0x4000+i] != b {
					t.Fatal("empty lower writes", i)
				}
			}
		}
	}

}
