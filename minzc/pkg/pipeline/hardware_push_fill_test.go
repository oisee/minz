package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

type pushFillObserver struct {
	spectrum.CPUCore
	t       *testing.T
	machine *spectrum.Machine
	done    bool
	elapsed int64
	before  rotoMachineRegisters
}

func (c *pushFillObserver) DoOpcode() {
	if c.IFF1() || c.IFF2() {
		c.t.Fatal("fill enabled IRQ while caller requires DI")
	}
	if !c.done && c.PC() == 0x7000 {
		if c.SP() != 0xeff2 {
			c.t.Fatalf("fill returned with SP=%04x", c.SP())
		}
		if rotoMachineRegs(c.CPUCore) != c.before {
			c.t.Fatal("fill changed saved registers/flags")
		}
		c.elapsed = c.machine.AbsoluteTStates()
		c.done = true
	}
	c.CPUCore.DoOpcode()
}
func TestHardwarePushFillScreenMachine(t *testing.T) {
	rom, err := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if err != nil {
		t.Fatal(err)
	}
	var results []map[string]any
	for _, value := range []int{0, 90} {
		for _, unroll := range []int{0, 16, 32, 64, 128, 256} {
			t.Run(fmt.Sprintf("value-%d/unroll-%d", value, unroll), func(t *testing.T) {
				source := "import hw.fill\nfun clear() { @fill() { layout \"zx_bitmap_bytes\" 0x4000; value 0; via \"ldir\" } }\n"
				if unroll != 0 {
					source = fmt.Sprintf("import hw.push_fill\n@push_fill(\"clear\") { region 0x4000 6144; value 0; unroll %d; irq \"disabled\" }\n", unroll)
				}
				source = strings.Replace(source, "value 0", fmt.Sprintf("value %d", value), 1)
				source = strings.ReplaceAll(source, ";", "\n")
				h, e := nanz.ParseWithOpts(source, "push-fill-machine.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
				if e != nil {
					t.Fatal(e)
				}
				steps, e := CompileHIRSteps(h, Options{AssertMode: "none"})
				if e != nil {
					t.Fatal(e)
				}
				asm := "ORG 0x8000\n" + steps.Assembly
				a, e := z80asm.NewAssembler().AssembleString(asm)
				if e != nil {
					t.Fatal(e)
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors)
				}
				m, e := spectrum.New48K(rom)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0x4000; i < 65536; i++ {
					m.Memory.Write(uint16(i), byte(i*37+i/256+19), false)
				}
				for i, b := range a.Binary {
					m.Memory.Write(0x8000+uint16(i), b, false)
				}
				m.Memory.Write(0x7000, 0x76, false)
				m.Memory.Write(0xeff0, 0, false)
				m.Memory.Write(0xeff1, 0x70, false)
				m.CPU.SetPC(uint16(a.Symbols["clear"]))
				m.CPU.SetSP(0xeff0)
				m.CPU.SetAF(0x12d7)
				m.CPU.SetBC(0x2345)
				m.CPU.SetDE(0x6789)
				m.CPU.SetHL(0xabcd)
				m.CPU.SetIX(0x3456)
				m.CPU.SetIY(0x789a)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				c := &pushFillObserver{CPUCore: m.CPU, t: t, machine: m, before: rotoMachineRegs(m.CPU)}
				m.CPU = c
				before := make([]byte, 65536)
				for i := range before {
					before[i] = m.Memory.ReadByteInternal(uint16(i))
				}
				for i := 0; i < 4 && !c.done; i++ {
					m.RunFrame()
				}
				if !c.done {
					t.Fatal("clear did not return")
				}
				// All RAM, except the owned saved-register stack and exactly two SMC bytes.
				stackStart := 0xefe8
				if unroll != 0 {
					stackStart = 0xefec
					if unroll > 126 {
						stackStart = 0xefea
					}
				}
				patch := -1
				if unroll != 0 {
					patch = a.Symbols["clear_push_fill_restore"] + 1
				}
				for i := 0x4000; i < 65536; i++ {
					if i >= stackStart && i < 0xeff0 || i == patch || i == patch+1 {
						continue
					}
					want := before[i]
					if i < 0x5800 {
						want = byte(value)
					}
					if got := m.Memory.ReadByteInternal(uint16(i)); got != want {
						t.Fatalf("RAM[%04x]=%02x want %02x", i, got, want)
					}
				}
				if unroll != 0 && c.elapsed >= int64(m.Mode.TStatesPerFrame()) {
					t.Fatalf("PUSH clear %d T exceeds frame", c.elapsed)
				}
				result := map[string]any{"unroll_PUSH": unroll, "value": value, "machine_T": c.elapsed, "module_bytes": len(a.Binary), "PASS": true}
				results = append(results, result)
				t.Logf("unroll=%d: %d machine T, %d module bytes; RAM, PC/SP, registers, flags, DI preserved", unroll, c.elapsed, len(a.Binary))
				if d := os.Getenv("MINZ_PUSH_FILL_EXPORT"); d != "" {
					dir := filepath.Join(d, fmt.Sprint(value), fmt.Sprint(unroll))
					if e := os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte(asm), "program.bin": a.Binary} {
						if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
			})
		}
	}
	if d := os.Getenv("MINZ_PUSH_FILL_EXPORT"); d != "" {
		if e := os.MkdirAll(d, 0755); e != nil {
			t.Fatal(e)
		}
		data, e := json.MarshalIndent(map[string]any{"scope": "48K machine with ULA contention; complete clear entry through RET, excluding external CALL; caller DI, owned SMC RAM; no displayed-frame claim for full clear", "measurements": results}, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(d, "measurements.json"), data, 0644); e != nil {
			t.Fatal(e)
		}
	}
}
func TestHardwarePushFillRejectedContracts(t *testing.T) {
	valid := "region 0x4000 6144; value 0; unroll 32; irq \"disabled\""
	cases := []struct{ body, want string }{
		{strings.Replace(valid, "0x4000", "0x1000", 1), "destination must be RAM"},
		{strings.Replace(valid, "6144", "6143", 1), "length must be even"},
		{strings.Replace(valid, "0x4000 6144", "65534 4", 1), "region exceeds address space"},
		{strings.Replace(valid, "unroll 32", "unroll 31", 1), "unroll must divide word count"},
		{strings.Replace(valid, "unroll 32", "unroll 1", 1), "too many loop chunks"},
		{strings.Replace(valid, "disabled", "enabled", 1), "requires irq disabled"},
		{valid + "; value 1", "duplicate value"},
	}
	for _, tc := range cases {
		_, e := nanz.ParseWithOpts("import hw.push_fill\n@push_fill(\"clear\") {"+strings.ReplaceAll(tc.body, ";", "\n")+"}\n", "push-fill-reject.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e == nil || !strings.Contains(e.Error(), tc.want) {
			t.Fatalf("body %s: got %v want %s", tc.body, e, tc.want)
		}
	}
}
