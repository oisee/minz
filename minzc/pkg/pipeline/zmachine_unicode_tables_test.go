package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

func TestZMachineUnicodeTablesNative(t *testing.T) {
	original, err := os.ReadFile("../../../examples/zmachine/spectrum128/zvm_core.nanz")
	if err != nil {
		t.Fatal(err)
	}
	unicode := map[int]int{}
	reverse := map[int]int{}
	lower := map[int]int{}
	for _, m := range regexp.MustCompile(`(?m)^    glyph (\d+) (\d+)$`).FindAllStringSubmatch(string(original), -1) {
		c, _ := strconv.Atoi(m[1])
		u, _ := strconv.Atoi(m[2])
		unicode[c] = u
		reverse[u] = c
	}
	for _, m := range regexp.MustCompile(`(?m)^    lower (\d+) (\d+)$`).FindAllStringSubmatch(string(original), -1) {
		u, _ := strconv.Atoi(m[1])
		l, _ := strconv.Atoi(m[2])
		lower[u] = l
	}
	if len(unicode) != 69 {
		t.Fatal("Unicode DSL mapping count", len(unicode))
	}
	path := filepath.Join(t.TempDir(), "core.nanz")
	cmd := exec.Command("python3", "../../../scripts/zmachine/prepare_native_core.py", path, "--story-bytes", "86928")
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
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	m, err := spectrum.NewPentagon128(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range a.Binary {
		m.Memory.Write(0x4000+uint16(i), b, false)
	}
	for _, name := range []string{"z3_unicode", "z3_from_unicode", "z3_lower"} {
		f := steps.MIR2Module.FuncByName(name)
		home := steps.Allocation.Locs[f.Contract.Params[0].Reg].Name
		limit := 65536
		if name == "z3_lower" {
			limit = 256
		}
		for value := 0; value < limit; value++ {
			want := 63
			switch name {
			case "z3_unicode":
				if u, ok := unicode[value]; ok {
					want = u
				}
			case "z3_from_unicode":
				if c, ok := reverse[value]; ok {
					want = c
				}
			case "z3_lower":
				want = value
				if l, ok := lower[value]; ok {
					want = l
				}
			}
			switch home {
			case "HL":
				m.CPU.SetHL(uint16(value))
			case "DE":
				m.CPU.SetDE(uint16(value))
			case "BC":
				m.CPU.SetBC(uint16(value))
			case "A":
				m.CPU.SetAF(uint16(value) << 8)
			default:
				t.Fatal("unsupported argument home", home)
			}
			m.Memory.Write(0xbf00, 0, false)
			m.Memory.Write(0xbf01, 0x30, false)
			m.CPU.SetPC(uint16(a.Symbols[name]))
			m.CPU.SetSP(0xbf00)
			for i := 0; i < 400; i++ {
				if m.CPU.PC() == 0x3000 && m.CPU.SP() == 0xbf02 {
					break
				}
				m.CPU.DoOpcode()
			}
			got := int(m.CPU.AF() >> 8)
			if name == "z3_unicode" {
				got = int(m.CPU.HL())
			}
			if got != want || m.CPU.PC() != 0x3000 || m.CPU.SP() != 0xbf02 {
				t.Fatalf("%s(%d) got%d want%d PC/SP%04x/%04x", name, value, got, want, m.CPU.PC(), m.CPU.SP())
			}
		}
	}
}
