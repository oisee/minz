package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestZMachineKeyboardControlsNative(t *testing.T) {
	h, err := nanz.ParseWithOpts("import zx.console\nglobal buffer:[u8;80]\nfun main()->u8 { return con_input(&buffer,79) }", "keys.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	setup := func() *spectrum.Machine {
		m, err := spectrum.NewPentagon128(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range a.Binary {
			m.Memory.Write(0x8000+uint16(i), b, false)
		}
		m.Memory.SetPagingForce(0x10)
		m.CPU.SetIFF1(false)
		m.CPU.SetIFF2(false)
		return m
	}
	run := func(m *spectrum.Machine, entry string, events [][]spectrum.SpecKey) {
		m.Memory.Write(0xbf00, 0, false)
		m.Memory.Write(0xbf01, 0x70, false)
		m.CPU.SetPC(uint16(a.Symbols[entry]))
		m.CPU.SetSP(0xbf00)
		at := 0
		ready := false
		for i := 0; i < 2000000; i++ {
			if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xbf02 {
				break
			}
			if m.CPU.PC() == uint16(a.Symbols["zx__console__con_getchar"]) {
				m.Keyboard.Reset()
				ready = true
			}
			if ready && m.CPU.PC() == uint16(a.Symbols["zx__console__scan_row_key"]) {
				if at == len(events) {
					t.Fatal("unexpected key request")
				}
				for _, key := range events[at] {
					m.Keyboard.KeyPress(key.Row, key.Bit)
				}
				at++
				ready = false
			}
			m.CPU.DoOpcode()
		}
		if at != len(events) || m.CPU.PC() != 0x7000 || m.CPU.SP() != 0xbf02 || m.Memory.PagingState() != 0x10 {
			t.Fatalf("input boundary entry%s keys%d/%d PC/SP%04x/%04x", entry, at, len(events), m.CPU.PC(), m.CPU.SP())
		}
	}
	for _, c := range []struct {
		key   spectrum.SpecKey
		shift bool
		want  byte
	}{
		{spectrum.Key0, false, '0'}, {spectrum.Key0, true, 8},
		{spectrum.Key5, true, 9}, {spectrum.Key6, true, 10}, {spectrum.Key7, true, 11}, {spectrum.Key8, true, 12},
		{spectrum.KeyA, true, 'A'}, {spectrum.KeyZ, true, 'Z'},
	} {
		m := setup()
		keys := []spectrum.SpecKey{c.key}
		if c.shift {
			keys = append(keys, spectrum.KeyShift)
		}
		run(m, "zx__console__con_getchar", [][]spectrum.SpecKey{keys})
		if byte(m.CPU.AF()>>8) != c.want {
			t.Fatalf("key%v shift%v got%d want%d", c.key, c.shift, byte(m.CPU.AF()>>8), c.want)
		}
	}
	for _, c := range []struct {
		key  spectrum.SpecKey
		want byte
	}{{spectrum.KeyP, 34}, {spectrum.KeyN, 44}, {spectrum.KeyM, 46}, {spectrum.KeyC, 63}, {spectrum.Key7, 39}, {spectrum.Key1, 33}, {spectrum.KeyL, 61}} {
		m := setup()
		run(m, "zx__console__con_getchar", [][]spectrum.SpecKey{{spectrum.KeySym, c.key}})
		if byte(m.CPU.AF()>>8) != c.want {
			t.Fatalf("symbol key %v got%d want%d", c.key, byte(m.CPU.AF()>>8), c.want)
		}
	}
	for _, start := range []struct{ col, row byte }{{0, 2}, {30, 2}, {31, 23}} {
		m := setup()
		m.Memory.Write(uint16(a.Symbols["zx__console__con_col"]), start.col, false)
		m.Memory.Write(uint16(a.Symbols["zx__console__con_row"]), start.row, false)
		for ch := 32; ch < 128; ch++ {
			for line := 0; line < 8; line++ {
				m.Memory.ROM[1][0x3d00+(ch-32)*8+line] = byte(ch + line)
			}
		}
		run(m, "main", [][]spectrum.SpecKey{{spectrum.KeyA}, {spectrum.KeyB}, {spectrum.KeyShift, spectrum.Key0}, {spectrum.KeyC}, {spectrum.KeyEnter}})
		at := uint16(a.Symbols["buffer"])
		if m.Memory.Read(at) != 'a' || m.Memory.Read(at+1) != 'c' || m.Memory.Read(at+2) != 0 {
			t.Fatal("delete submitted wrong text")
		}
		col := int(start.col) + 2
		row := int(start.row) + col/32
		col %= 32
		if row >= 24 {
			row = 23
		}
		for line := 0; line < 8; line++ {
			addr := uint16(0x4000 | ((row & 0x18) << 8) | (line << 8) | ((row & 7) << 5) | col)
			if m.Memory.Read(addr) != byte(32+line) {
				t.Fatalf("stale underscore row%d col%d", row, col)
			}
		}
	}
	m := setup()
	check := func(events [][]spectrum.SpecKey, want string) {
		run(m, "main", events)
		at := uint16(a.Symbols["buffer"])
		for i, b := range []byte(want + "\x00") {
			if m.Memory.Read(at+uint16(i)) != b {
				t.Fatalf("edit got byte%d %d want%d (%q)", i, m.Memory.Read(at+uint16(i)), b, want)
			}
		}
	}
	left := []spectrum.SpecKey{spectrum.KeyShift, spectrum.Key5}
	right := []spectrum.SpecKey{spectrum.KeyShift, spectrum.Key8}
	up := []spectrum.SpecKey{spectrum.KeyShift, spectrum.Key7}
	down := []spectrum.SpecKey{spectrum.KeyShift, spectrum.Key6}
	del := []spectrum.SpecKey{spectrum.KeyShift, spectrum.Key0}
	check([][]spectrum.SpecKey{{spectrum.KeyA}, {spectrum.KeyC}, left, {spectrum.KeyB}, right, {spectrum.KeyEnter}}, "abc")
	check([][]spectrum.SpecKey{up, left, del, {spectrum.KeyEnter}}, "ac")
	check([][]spectrum.SpecKey{{spectrum.KeyZ}, up, up, down, down, {spectrum.KeyEnter}}, "z")
	for i, key := range []spectrum.SpecKey{spectrum.KeyD, spectrum.KeyE, spectrum.KeyF, spectrum.KeyG, spectrum.KeyH} {
		check([][]spectrum.SpecKey{{key}, {spectrum.KeyEnter}}, string(byte('d'+i)))
	}
	check([][]spectrum.SpecKey{up, up, up, up, up, {spectrum.KeyEnter}}, "e")
	full := make([][]spectrum.SpecKey, 0, 85)
	for i := 0; i < 84; i++ {
		full = append(full, []spectrum.SpecKey{spectrum.KeyA})
	}
	full = append(full, []spectrum.SpecKey{spectrum.KeyEnter})
	check(full, strings.Repeat("a", 79))
	check([][]spectrum.SpecKey{left, left, del, {spectrum.KeyEnter}}, "")
	t.Log("Caps Shift controls, uppercase, delete and wrapped/bottom-row input PASS")
}
