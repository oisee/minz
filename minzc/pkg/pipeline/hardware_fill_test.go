package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// This oracle models the physical address layout, independently of the emitted
// Nanz source. In particular, adjacent logical ZX rows are usually not adjacent
// in RAM (Antique Toy, chapter 2, "Screen Layout and Addressing").
type hardwareFillCase struct {
	name, layout, traversal  string
	base, cols, rows, stride int
	x, y, width, height      int
	value                    byte
}

func (c hardwareFillCase) body() string {
	layout := fmt.Sprintf("layout %q %d %d %d %d;", c.layout, c.base, c.cols, c.rows, c.stride)
	if c.layout == "zx_bitmap_bytes" {
		layout = fmt.Sprintf("layout %q %d;", c.layout, c.base)
	}
	return fmt.Sprintf("%s region %d %d %d %d; traversal %q; value %d;", layout, c.x, c.y, c.width, c.height, c.traversal, c.value)
}

func (c hardwareFillCase) addresses() map[int]bool {
	writes := make(map[int]bool)
	for y := c.y; y < c.y+c.height; y++ {
		for x := c.x; x < c.x+c.width; x++ {
			address := c.base + y*c.stride + x
			if c.layout == "zx_bitmap_bytes" {
				address = c.base + ((y & 0xc0) << 5) + ((y & 7) << 8) + ((y & 0x38) << 2) + x
			}
			writes[address] = true
		}
	}
	return writes
}

func hardwareFillParse(t *testing.T, body string) *hir.Module {
	t.Helper()
	source := hardwareFillSource(body)
	h, err := nanz.ParseWithOpts(source, "hardware_fill.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func hardwareFillSource(body string) string {
	return "import hw.fill\nfun fill_test() { @fill() {\n" + strings.ReplaceAll(body, ";", "\n") + "\n} }\n"
}

func hardwareFillExport(t *testing.T, name string, data []byte) {
	t.Helper()
	if directory := os.Getenv("MINZ_HW_FILL_EXPORT"); directory != "" {
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHardwareFillMemoryNativeAndVM(t *testing.T) {
	cases := []hardwareFillCase{
		{"linear-one", "linear", "rows", 0x6800, 1, 1, 1, 0, 0, 1, 1, 0xad},
		{"linear-strided", "linear", "rows", 0x5ffe, 7, 4, 11, 1, 1, 5, 3, 0x53},
		{"linear-255", "linear", "rows", 0x6000, 255, 1, 255, 0, 0, 255, 1, 0x80},
		{"linear-256", "linear", "rows", 0x6000, 256, 1, 256, 0, 0, 256, 1, 0xff},
		{"linear-last-byte", "linear", "rows", 0xfffc, 2, 2, 2, 0, 0, 2, 2, 0x39},
		{"linear-zero-width", "linear", "rows", 0x6800, 8, 4, 10, 8, 0, 0, 4, 0xa5},
		{"linear-zero-height", "linear", "rows", 0x6800, 8, 4, 10, 0, 4, 8, 0, 0xa5},
		{"zx-row-7-8", "zx_bitmap_bytes", "rows", 0x4000, 32, 192, 32, 2, 7, 5, 2, 0x47},
		{"zx-row-63-64", "zx_bitmap_bytes", "rows", 0x4000, 32, 192, 32, 29, 63, 3, 2, 0x93},
		{"zx-last-byte", "zx_bitmap_bytes", "rows", 0x4000, 32, 192, 32, 31, 191, 1, 1, 0xc2},
		{"zx-full-rows", "zx_bitmap_bytes", "rows", 0x4000, 32, 192, 32, 0, 0, 32, 192, 0x5a},
		{"zx-full-storage", "zx_bitmap_bytes", "storage", 0x4000, 32, 192, 32, 0, 0, 32, 192, 0x5a},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hardwareFillExport(t, tc.name+"/source.nanz", []byte(hardwareFillSource(tc.body())))
			writes := tc.addresses()
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(mode, func(t *testing.T) {
					raw := hir.LowerModule(hardwareFillParse(t, tc.body()))
					steps, err := CompileHIRSteps(hardwareFillParse(t, tc.body()), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
					if err != nil {
						t.Fatal(err)
					}
					seed := make([]byte, 65536)
					for address := range seed {
						seed[address] = byte(address*37 + address/256 + 19)
					}
					check := func(label string, before, after []byte, allowed map[int]bool) {
						t.Helper()
						for address, old := range before {
							want := old
							if writes[address] {
								want = tc.value
							} else if allowed[address] {
								continue
							}
							if after[address] != want {
								t.Fatalf("%s RAM[%04x] = %02x, want %02x (target=%v)", label, address, after[address], want, writes[address])
							}
						}
					}
					for index, module := range []*mir2.Module{raw, steps.MIR2Module} {
						vm := mir2.NewVM(module)
						vm.EnsureHeap(65536)
						vm.WriteHeapSlice(0, seed)
						if _, err := vm.Call("fill_test", nil); err != nil {
							t.Fatal(err)
						}
						check(fmt.Sprintf("VM%d", index), seed, vm.ReadHeap(0, 65536), nil)
					}
					assembly := "ORG 0x8000\n" + steps.Assembly
					assembled, err := z80asm.NewAssembler().AssembleString(assembly)
					if err != nil {
						t.Fatal(err)
					}
					if len(assembled.Errors) != 0 {
						t.Fatal(assembled.Errors)
					}
					hardwareFillExport(t, tc.name+"/"+mode+"/program.a80", []byte(assembly))
					hardwareFillExport(t, tc.name+"/"+mode+"/program.bin", assembled.Binary)
					z := emulator.NewRemogattoZ80()
					memory := z.Memory()
					copy(memory, seed)
					z.LoadMemory(0x8000, assembled.Binary)
					memory[0xeff0], memory[0xeff1] = 0, 0x70
					before := append([]byte(nil), memory...)
					allowed := make(map[int]bool)
					// The native call may use a bounded stack and exact declared
					// spill slots. Instruction bytes remain protected by the oracle.
					for address := 0xef00; address < 0xeff2; address++ {
						allowed[address] = true
					}
					for _, line := range strings.Split(assembly, "\n") {
						fields := strings.Fields(line)
						if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
							continue
						}
						address, ok := assembled.Symbols[strings.TrimSuffix(fields[0], ":")]
						if !ok {
							t.Fatal("missing spill symbol", fields[0])
						}
						width := 1
						if fields[1] == "DW" {
							width = 2
						} else if fields[1] != "DB" {
							t.Fatal("unexpected spill declaration", line)
						}
						count := len(strings.Split(strings.Join(fields[2:], ""), ","))
						for offset := 0; offset < width*count; offset++ {
							allowed[address+offset] = true
						}
					}
					entry, ok := assembled.Symbols["fill_test"]
					if !ok {
						t.Fatal("missing fill_test symbol")
					}
					z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeff0})
					returned := false
					for count := 0; count < 2000000; count++ {
						if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
							returned = true
							break
						}
						z.Step()
					}
					if !returned {
						t.Fatal("native did not restore return PC/SP", z.GetRegisters())
					}
					check("Z80", before, memory, allowed)
					result, err := json.MarshalIndent(map[string]any{"PASS": true, "mode": mode, "bytes": len(assembled.Binary), "T_states": z.GetCycles(), "written_bytes": len(writes), "scope": "raw/final VM and native Z80; independent layout oracle; all RAM except exact declared native spill slots and bounded stack; return PC/SP"}, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					hardwareFillExport(t, tc.name+"/"+mode+"/result.json", append(result, '\n'))
					t.Logf("%d bytes, %d T-states; %d target bytes", len(assembled.Binary), z.GetCycles(), len(writes))
				})
			}
		})
	}
}

func TestHardwareFillRejectsInvalidDescriptions(t *testing.T) {
	valid := `layout "linear" 8192 8 4 10; region 0 0 8 4; traversal "rows"; value 71;`
	cases := map[string]string{
		"unknown-layout":     strings.Replace(valid, `"linear"`, `"mystery"`, 1),
		"unknown-traversal":  strings.Replace(valid, `"rows"`, `"diagonal"`, 1),
		"negative-base":      strings.Replace(valid, "8192", `"-1"`, 1),
		"base-overflow":      strings.Replace(valid, "8192", "65536", 1),
		"address-overflow":   strings.Replace(valid, "8192", "65530", 1),
		"bad-stride":         strings.Replace(valid, "8 4 10", "8 4 7", 1),
		"region-overflow":    strings.Replace(valid, "region 0 0 8 4", "region 1 0 8 4", 1),
		"negative-region":    strings.Replace(valid, "region 0 0 8 4", `region "-1" 0 8 4`, 1),
		"value-overflow":     strings.Replace(valid, "value 71", "value 256", 1),
		"duplicate-region":   valid + " region 0 0 1 1;",
		"duplicate-value":    valid + " value 12;",
		"duplicate-layout":   valid + ` layout "linear" 8192 8 4 10;`,
		"duplicate-order":    valid + ` traversal "rows";`,
		"missing-layout":     strings.Replace(valid, `layout "linear" 8192 8 4 10;`, "", 1),
		"missing-value":      strings.Replace(valid, "value 71;", "", 1),
		"layout-bad-arity":   strings.Replace(valid, "8192 8 4 10", "8192 8 4", 1),
		"region-bad-arity":   strings.Replace(valid, "region 0 0 8 4", "region 0 0 8", 1),
		"int32-overflow":     strings.Replace(valid, "8192", "2147483648", 1),
		"unknown-field":      valid + " mystery 12;",
		"zx-partial-storage": `layout "zx_bitmap_bytes" 16384; region 1 0 31 192; traversal "storage"; value 71;`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			source := "import hw.fill\nfun fill_test() { @fill() {\n" + strings.ReplaceAll(body, ";", "\n") + "\n} }\n"
			if _, err := nanz.ParseWithOpts(source, "hardware_fill_invalid.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"}); err == nil {
				t.Fatal("invalid description was accepted", body)
			} else if name == "int32-overflow" {
				if !strings.Contains(err.Error(), "node_arg_int") {
					t.Fatalf("expected integer host diagnostic, got %v", err)
				}
			} else if !strings.Contains(err.Error(), "fill:") {
				t.Fatalf("expected fill contract diagnostic, got %v", err)
			}
		})
	}
}
