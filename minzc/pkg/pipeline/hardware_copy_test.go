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

type hardwareCopyLayout struct {
	name                     string
	base, cols, rows, stride int
}

func (l hardwareCopyLayout) declaration(key string) string {
	if l.name == "zx_bitmap_bytes" {
		return fmt.Sprintf("%s %q %d;", key, l.name, l.base)
	}
	return fmt.Sprintf("%s %q %d %d %d %d;", key, l.name, l.base, l.cols, l.rows, l.stride)
}

// The oracle uses the documented physical ZX row permutation, rather than
// evaluating generated address expressions (Antique Toy, chapter 2).
func (l hardwareCopyLayout) address(x, y int) int {
	if l.name == "zx_bitmap_bytes" {
		return l.base + (y&0xc0)*32 + (y&7)*256 + (y&0x38)*4 + x
	}
	return l.base + y*l.stride + x
}

type hardwareCopyCase struct {
	name                          string
	source, destination           hardwareCopyLayout
	sx, sy, dx, dy, width, height int
}

func (c hardwareCopyCase) body(primitive string) string {
	return fmt.Sprintf("%s %s region %d %d %d %d %d %d; via %q;", c.source.declaration("source"), c.destination.declaration("destination"), c.sx, c.sy, c.dx, c.dy, c.width, c.height, primitive)
}

func hardwareCopySource(body string) string {
	return "import hw.copy\nfun copy_test() { @copy() {\n" + strings.ReplaceAll(body, ";", "\n") + "\n} }\n"
}

func hardwareCopyParse(t *testing.T, source string) *hir.Module {
	t.Helper()
	h, err := nanz.ParseWithOpts(source, "hardware_copy.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func hardwareCopyExport(t *testing.T, name string, data []byte) {
	t.Helper()
	if directory := os.Getenv("MINZ_HW_COPY_EXPORT"); directory != "" {
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHardwareCopyPointerMemoryNativeAndVM(t *testing.T) { hardwareCopyMatrix(t, "pointer") }
func TestHardwareCopyLDIRNative(t *testing.T)               { hardwareCopyMatrix(t, "ldir") }

func hardwareCopyMatrix(t *testing.T, primitive string) {
	linear := func(base, cols, rows, stride int) hardwareCopyLayout {
		return hardwareCopyLayout{"linear", base, cols, rows, stride}
	}
	zx := hardwareCopyLayout{"zx_bitmap_bytes", 0x4000, 32, 192, 32}
	cases := []hardwareCopyCase{
		{"linear-one", linear(0x6000, 1, 1, 1), linear(0x6800, 1, 1, 1), 0, 0, 0, 0, 1, 1},
		{"linear-strided", linear(0x60fd, 9, 5, 13), linear(0x68fe, 8, 4, 11), 2, 1, 1, 0, 6, 4},
		{"linear-page-crossing", linear(0x60ff, 2, 1, 2), linear(0x68ff, 2, 1, 2), 0, 0, 0, 0, 2, 1},
		{"linear-255", linear(0x6000, 255, 1, 255), linear(0x6800, 255, 1, 255), 0, 0, 0, 0, 255, 1},
		{"linear-256", linear(0x6000, 256, 1, 256), linear(0x6800, 256, 1, 256), 0, 0, 0, 0, 256, 1},
		{"destination-last-byte", linear(0x6000, 2, 2, 3), linear(0xfffc, 2, 2, 2), 0, 0, 0, 0, 2, 2},
		{"source-last-byte", linear(0xfffc, 2, 2, 2), linear(0x6800, 2, 2, 3), 0, 0, 0, 0, 2, 2},
		{"zero-width", linear(0x6000, 8, 4, 10), linear(0x6800, 8, 4, 11), 8, 0, 8, 0, 0, 4},
		{"zero-height", linear(0x6000, 8, 4, 10), linear(0x6800, 8, 4, 11), 0, 4, 0, 4, 8, 0},
		{"raster-to-zx-row-7-8", linear(0x6000, 8, 3, 10), zx, 1, 1, 2, 7, 5, 2},
		{"raster-to-zx-row-63-64", linear(0x6000, 5, 2, 7), zx, 1, 0, 29, 63, 3, 2},
		{"zx-to-raster", zx, linear(0x6000, 9, 4, 12), 3, 62, 1, 1, 7, 3},
		{"raster-to-zx-full", linear(0x6000, 32, 192, 32), zx, 0, 0, 0, 0, 32, 192},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := hardwareCopySource(tc.body(primitive))
			hardwareCopyExport(t, primitive+"/"+tc.name+"/source.nanz", []byte(source))
			writes := make(map[int]int)
			for y := 0; y < tc.height; y++ {
				for x := 0; x < tc.width; x++ {
					writes[tc.destination.address(tc.dx+x, tc.dy+y)] = tc.source.address(tc.sx+x, tc.sy+y)
				}
			}
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(mode, func(t *testing.T) {
					steps, err := CompileHIRSteps(hardwareCopyParse(t, source), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
					if err != nil {
						t.Fatal(err)
					}
					assembly := "ORG 0x8000\n" + steps.Assembly
					a, err := z80asm.NewAssembler().AssembleString(assembly)
					if err != nil {
						t.Fatal(err)
					}
					if len(a.Errors) != 0 {
						t.Fatal(a.Errors)
					}
					prefix := primitive + "/" + tc.name + "/" + mode + "/"
					hardwareCopyExport(t, prefix+"program.a80", []byte(assembly))
					hardwareCopyExport(t, prefix+"program.bin", a.Binary)
					allowed := make(map[int]bool)
					stackStart := 0xef00
					if primitive == "ldir" {
						stackStart = 0xefe8
					}
					for address := stackStart; address < 0xeff0; address++ {
						allowed[address] = true
					}
					for _, line := range strings.Split(assembly, "\n") {
						fields := strings.Fields(line)
						if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
							continue
						}
						address, ok := a.Symbols[strings.TrimSuffix(fields[0], ":")]
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
					var measurements []map[string]any
					// Two independently supplied runtime memories prove that the
					// generated copy reads data, rather than embedding a snapshot.
					for _, salt := range []int{19, 173} {
						seed := make([]byte, 65536)
						for address := range seed {
							seed[address] = byte(address*37 + address/256*11 + salt)
						}
						check := func(label string, before, after []byte, exemptions map[int]bool) {
							t.Helper()
							for address, want := range before {
								if from, target := writes[address]; target {
									want = before[from]
								} else if exemptions[address] {
									continue
								}
								if after[address] != want {
									t.Fatalf("%s RAM[%04x]=%02x want %02x", label, address, after[address], want)
								}
							}
						}
						if primitive == "pointer" {
							for index, module := range []*mir2.Module{hir.LowerModule(hardwareCopyParse(t, source)), steps.MIR2Module} {
								vm := mir2.NewVM(module)
								vm.EnsureHeap(65536)
								vm.WriteHeapSlice(0, seed)
								if _, err := vm.Call("copy_test", nil); err != nil {
									t.Fatal(err)
								}
								check(fmt.Sprintf("VM%d", index), seed, vm.ReadHeap(0, 65536), nil)
							}
						}
						z := emulator.NewRemogattoZ80()
						memory := z.Memory()
						copy(memory, seed)
						z.LoadMemory(0x8000, a.Binary)
						memory[0xeff0], memory[0xeff1] = 0, 0x70
						before := append([]byte(nil), memory...)
						entry, ok := a.Symbols["copy_test"]
						if !ok {
							t.Fatal("missing copy_test symbol")
						}
						initial := emulator.Registers{PC: uint16(entry), SP: 0xeff0, A: 0xa5, F: 0xd7, HL: 0x1234, BC: 0x5678, DE: 0x9abc, IX: 0x1357, IY: 0x2468}
						z.SetRegisters(initial)
						for count := 0; count < 2000000; count++ {
							if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
								break
							}
							z.Step()
						}
						if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
							t.Fatal("native did not restore return PC/SP", z.GetRegisters())
						}
						check("Z80", before, memory, allowed)
						if primitive == "ldir" {
							got := z.GetRegisters()
							if got.A != initial.A || got.F != initial.F || got.HL != initial.HL || got.BC != initial.BC || got.DE != initial.DE || got.IX != initial.IX || got.IY != initial.IY {
								t.Fatalf("preservation: got %+v initial %+v", got, initial)
							}
						}
						measurements = append(measurements, map[string]any{"seed_salt": salt, "T_states": z.GetCycles()})
						t.Logf("seed %d: %d bytes, %d T-states; %d copied bytes", salt, len(a.Binary), z.GetCycles(), len(writes))
					}
					result, err := json.MarshalIndent(map[string]any{"PASS": true, "mode": mode, "bytes": len(a.Binary), "copied_bytes": len(writes), "primitive": primitive, "measurements": measurements, "scope": "independent source/destination address oracle; all native RAM except exact spill declarations and bounded stack; source unchanged; exact PC/SP; raw/final VM for pointer only; preserved AF/BC/DE/HL/IX/IY for LDIR"}, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					hardwareCopyExport(t, prefix+"result.json", append(result, '\n'))
				})
			}
		})
	}
}

func TestHardwareCopyRejectsInvalidDescriptions(t *testing.T) {
	valid := `source "linear" 24576 8 4 10; destination "linear" 26624 8 4 11; region 0 0 0 0 8 4; via "pointer";`
	cases := map[string]string{
		"unknown-source-layout":        strings.Replace(valid, `source "linear"`, `source "mystery"`, 1),
		"unknown-destination-layout":   strings.Replace(valid, `destination "linear"`, `destination "mystery"`, 1),
		"unknown-via":                  strings.Replace(valid, `"pointer"`, `"magic"`, 1),
		"missing-source":               strings.Replace(valid, `source "linear" 24576 8 4 10;`, "", 1),
		"missing-destination":          strings.Replace(valid, `destination "linear" 26624 8 4 11;`, "", 1),
		"missing-region":               strings.Replace(valid, `region 0 0 0 0 8 4;`, "", 1),
		"duplicate-source":             valid + ` source "linear" 24576 8 4 10;`,
		"duplicate-destination":        valid + ` destination "linear" 26624 8 4 11;`,
		"duplicate-region":             valid + ` region 0 0 0 0 1 1;`,
		"duplicate-via":                valid + ` via "ldir";`,
		"source-arity":                 strings.Replace(valid, "24576 8 4 10", "24576 8 4", 1),
		"destination-arity":            strings.Replace(valid, "26624 8 4 11", "26624 8 4", 1),
		"region-arity":                 strings.Replace(valid, "region 0 0 0 0 8 4", "region 0 0 0 0 8", 1),
		"via-arity":                    strings.Replace(valid, `via "pointer"`, `via "pointer" 1`, 1),
		"unknown-field":                valid + ` traversal "rows";`,
		"negative-source-base":         strings.Replace(valid, "24576", `"-1"`, 1),
		"source-address-overflow":      strings.Replace(valid, "24576", "65530", 1),
		"destination-address-overflow": strings.Replace(valid, "26624", "65530", 1),
		"invalid-source-stride":        strings.Replace(valid, "24576 8 4 10", "24576 8 4 7", 1),
		"invalid-destination-stride":   strings.Replace(valid, "26624 8 4 11", "26624 8 4 7", 1),
		"source-region-overflow":       strings.Replace(valid, "region 0 0 0 0 8 4", "region 1 0 0 0 8 4", 1),
		"destination-region-overflow":  strings.Replace(valid, "region 0 0 0 0 8 4", "region 0 0 1 0 8 4", 1),
		"negative-region":              strings.Replace(valid, "region 0 0 0 0 8 4", `region "-1" 0 0 0 8 4`, 1),
		"overlapping-layouts":          strings.Replace(valid, "26624", "24580", 1),
		// Even disjoint selected rectangles are rejected when the declared
		// whole-layout bounding extents intersect: this is memcpy, not memmove.
		"disjoint-regions-overlapping-layouts": `source "linear" 24576 16 1 16; destination "linear" 24584 16 1 16; region 0 0 12 0 4 1;`,
		"zx-arity":                             `source "zx_bitmap_bytes" 16384 32; destination "linear" 24576 32 192 32; region 0 0 0 0 32 192;`,
		"zx-overflow":                          `source "zx_bitmap_bytes" 61440; destination "linear" 24576 32 192 32; region 0 0 0 0 32 192;`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := nanz.ParseWithOpts(hardwareCopySource(body), "hardware_copy_invalid.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err == nil {
				t.Fatal("invalid description was accepted", body)
			}
			if !strings.Contains(err.Error(), "copy:") {
				t.Fatalf("expected copy contract diagnostic, got %v", err)
			}
		})
	}
}

// Preserving inline ASM must not erase values live in the surrounding Nanz.
// The pointer oracle cannot cover inline ASM, so this is an independent native
// ABI check with a changing global input and two adjacent generated blocks.
func TestHardwareCopyLDIRLiveValues(t *testing.T) { hardwareCopyLiveValues(t, "ldir") }

func TestStructuredCopyLiveValues(t *testing.T) { hardwareCopyLiveValues(t, "pointer") }

func hardwareCopyLiveValues(t *testing.T, primitive string) {
	source := `import hw.copy
global input: u16 = 0
global answer: u16 = 0
fun copy_test() {
    let keep: u16 = input * 3 + 7
    @copy() {
        source "linear" 0x6000 256 1 256
        destination "linear" 0x6200 256 1 256
        region 0 0 0 0 256 1
        via "ldir"
    }
    @copy() {
        source "linear" 0x6100 1 1 1
        destination "linear" 0x6300 1 1 1
        region 0 0 0 0 1 1
        via "ldir"
    }
    answer = keep ^ input
}`
	source = strings.ReplaceAll(source, `via "ldir"`, `via "`+primitive+`"`)
	hardwareCopyExport(t, "live-values/source.nanz", []byte(source))
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			steps, err := CompileHIRSteps(hardwareCopyParse(t, source), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if err != nil {
				t.Fatal(err)
			}
			assembly := "ORG 0x8000\n" + steps.Assembly
			a, err := z80asm.NewAssembler().AssembleString(assembly)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Errors) != 0 {
				t.Fatal(a.Errors)
			}
			hardwareCopyExport(t, "live-values/"+mode+"/program.a80", []byte(assembly))
			hardwareCopyExport(t, "live-values/"+mode+"/program.bin", a.Binary)
			inputAddr, inputOK := a.Symbols["input"]
			answerAddr, answerOK := a.Symbols["answer"]
			if !inputOK || !answerOK {
				t.Fatal("missing globals", a.Symbols)
			}
			for _, value := range []uint16{0, 1, 2, 255, 256, 0x5555, 0x7fff, 0x8000, 0xfffe, 0xffff} {
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0x8000, a.Binary)
				memory := z.Memory()
				memory[inputAddr], memory[inputAddr+1] = byte(value), byte(value>>8)
				for offset := 0; offset <= 256; offset++ {
					memory[0x6000+offset] = byte(offset*37 + int(value))
				}
				memory[0xeff0], memory[0xeff1] = 0, 0x70
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["copy_test"]), SP: 0xeff0, HL: 0xdead, BC: 0xbeef, DE: 0x1234, A: 0xa5})
				for count := 0; count < 100000; count++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						break
					}
					z.Step()
				}
				if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
					t.Fatal("return", z.GetRegisters())
				}
				got := uint16(memory[answerAddr]) | uint16(memory[answerAddr+1])<<8
				want := (value*3 + 7) ^ value
				if got != want {
					t.Fatalf("input %04x: got %04x want %04x", value, got, want)
				}
				for offset := 0; offset <= 256; offset++ {
					wantByte := byte(offset*37 + int(value))
					if memory[0x6000+offset] != wantByte || memory[0x6200+offset] != wantByte {
						t.Fatalf("copy/source offset %d: source %02x destination %02x want %02x", offset, memory[0x6000+offset], memory[0x6200+offset], wantByte)
					}
				}
			}
		})
	}
}
