package pipeline

import (
	"encoding/hex"
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

type hardwareSpriteCase struct {
	name, layout                                  string
	base, cols, rows, stride, x, y, width, height int
	masks, graphics                               []string
}

func (c hardwareSpriteCase) address(x, y int) int {
	// Antique Toy chapter 2: independent physical ZX row permutation.
	if c.layout == "zx_bitmap_bytes" {
		return c.base + (y&0xc0)*32 + (y&7)*256 + (y&0x38)*4 + x
	}
	return c.base + y*c.stride + x
}

func (c hardwareSpriteCase) body(primitive string) string {
	var b strings.Builder
	if c.layout == "zx_bitmap_bytes" {
		fmt.Fprintf(&b, "destination %q %d;", c.layout, c.base)
	} else {
		fmt.Fprintf(&b, "destination %q %d %d %d %d;", c.layout, c.base, c.cols, c.rows, c.stride)
	}
	fmt.Fprintf(&b, "region %d %d %d %d;", c.x, c.y, c.width, c.height)
	for row := 0; row < c.height; row++ {
		fmt.Fprintf(&b, "row %q %q;", c.masks[row], c.graphics[row])
	}
	if primitive != "" {
		fmt.Fprintf(&b, "via %q;", primitive)
	}
	return b.String()
}
func hardwareSpriteSource(body string) string {
	return "import hw.sprite\nfun sprite_test() { @sprite() {\n" + strings.ReplaceAll(body, ";", "\n") + "\n} }\n"
}
func hardwareSpriteParse(t *testing.T, source string) *hir.Module {
	t.Helper()
	h, err := nanz.ParseWithOpts(source, "hardware_sprite.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func hardwareSpriteExport(t *testing.T, name string, data []byte) {
	t.Helper()
	if directory := os.Getenv("MINZ_HW_SPRITE_EXPORT"); directory != "" {
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHardwareSpritePointerMemoryNativeAndVM(t *testing.T) { hardwareSpriteMatrix(t, "pointer") }
func TestHardwareSpriteCompiledNative(t *testing.T)           { hardwareSpriteMatrix(t, "compiled") }

func hardwareSpriteMatrix(t *testing.T, primitive string) {
	cases := []hardwareSpriteCase{
		{"transparent", "linear", 0x6000, 4, 1, 4, 0, 0, 4, 1, []string{"ffffffff"}, []string{"00000000"}},
		{"opaque", "linear", 0x6000, 4, 1, 4, 0, 0, 4, 1, []string{"00000000"}, []string{"0055aaff"}},
		{"mixed", "linear", 0x6000, 6, 1, 6, 0, 0, 6, 1, []string{"fff00faa5500"}, []string{"00050340aaff"}},
		{"overlapping-mask-graphics", "linear", 0x6000, 4, 1, 4, 0, 0, 4, 1, []string{"fff055aa"}, []string{"ffff55aa"}},
		{"strided", "linear", 0x60fd, 8, 4, 11, 2, 1, 4, 3, []string{"00ff55aa", "f00f0000", "ffff0000"}, []string{"12340011", "0521abcd", "00004567"}},
		{"page-crossing", "linear", 0x60ff, 3, 1, 3, 0, 0, 3, 1, []string{"ff00aa"}, []string{"008811"}},
		{"last-byte", "linear", 0xfffc, 2, 2, 2, 0, 0, 2, 2, []string{"00ff", "aa00"}, []string{"7700", "1188"}},
		{"zero-width", "linear", 0x6000, 8, 3, 10, 8, 0, 0, 3, []string{"", "", ""}, []string{"", "", ""}},
		{"zero-height", "linear", 0x6000, 8, 3, 10, 0, 3, 8, 0, nil, nil},
		{"zx-row-7-8", "zx_bitmap_bytes", 0x4000, 32, 192, 32, 2, 7, 4, 2, []string{"00ff55aa", "f00f0000"}, []string{"12340011", "0521abcd"}},
		{"zx-row-63-64", "zx_bitmap_bytes", 0x4000, 32, 192, 32, 28, 63, 4, 2, []string{"00ff55aa", "f00f0000"}, []string{"12340011", "0521abcd"}},
		{"zx-last-row", "zx_bitmap_bytes", 0x4000, 32, 192, 32, 30, 191, 2, 1, []string{"aa00"}, []string{"1188"}},
		{"default-via", "linear", 0x6000, 2, 1, 2, 0, 0, 2, 1, []string{"00ff"}, []string{"ab00"}},
	}
	// A realistic 16-byte × 16-row sprite combines transparent, opaque, and
	// partially masked pixels, with an oracle independent of generated code.
	large := hardwareSpriteCase{name: "patterned-128x16", layout: "zx_bitmap_bytes", base: 0x4000, cols: 32, rows: 192, stride: 32, x: 8, y: 60, width: 16, height: 16}
	for row := 0; row < large.height; row++ {
		mask := make([]byte, large.width)
		graphic := make([]byte, large.width)
		for column := range mask {
			switch (row + column) % 4 {
			case 0:
				mask[column], graphic[column] = 0xff, 0
			case 1:
				mask[column], graphic[column] = 0, byte(row*17+column*11)
			case 2:
				mask[column], graphic[column] = 0xf0, byte((row+column)&15)
			case 3:
				mask[column], graphic[column] = 0x55, 0xaa
			}
		}
		large.masks = append(large.masks, hex.EncodeToString(mask))
		large.graphics = append(large.graphics, hex.EncodeToString(graphic))
	}
	cases = append(cases, large)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chosen := primitive
			if tc.name == "default-via" {
				if primitive == "compiled" {
					t.Skip("default selects pointer; covered by pointer matrix")
				}
				chosen = ""
			}
			source := hardwareSpriteSource(tc.body(chosen))
			hardwareSpriteExport(t, primitive+"/"+tc.name+"/source.nanz", []byte(source))
			type operation struct{ mask, graphics byte }
			writes := make(map[int]operation)
			for row := 0; row < tc.height; row++ {
				masks, err := hex.DecodeString(tc.masks[row])
				if err != nil {
					t.Fatal(err)
				}
				graphics, err := hex.DecodeString(tc.graphics[row])
				if err != nil {
					t.Fatal(err)
				}
				for column := 0; column < tc.width; column++ {
					writes[tc.address(tc.x+column, tc.y+row)] = operation{masks[column], graphics[column]}
				}
			}
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(mode, func(t *testing.T) {
					steps, err := CompileHIRSteps(hardwareSpriteParse(t, source), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
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
					hardwareSpriteExport(t, prefix+"program.a80", []byte(assembly))
					hardwareSpriteExport(t, prefix+"program.bin", a.Binary)
					allowed := make(map[int]bool)
					if primitive == "compiled" {
						for address := 0xefec; address < 0xeff0; address++ {
							allowed[address] = true
						}
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
					for _, salt := range []int{19, 173} {
						seed := make([]byte, 65536)
						for address := range seed {
							seed[address] = byte(address*37 + address/256*11 + salt)
						}
						check := func(label string, before, after []byte, exemptions map[int]bool) {
							t.Helper()
							for address, want := range before {
								if op, ok := writes[address]; ok {
									want = (want & op.mask) | op.graphics
								} else if exemptions[address] {
									continue
								}
								if after[address] != want {
									t.Fatalf("%s RAM[%04x]=%02x want %02x", label, address, after[address], want)
								}
							}
						}
						if primitive == "pointer" {
							for index, module := range []*mir2.Module{hir.LowerModule(hardwareSpriteParse(t, source)), steps.MIR2Module} {
								vm := mir2.NewVM(module)
								vm.EnsureHeap(65536)
								vm.WriteHeapSlice(0, seed)
								if _, err := vm.Call("sprite_test", nil); err != nil {
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
						entry, ok := a.Symbols["sprite_test"]
						if !ok {
							t.Fatal("missing sprite_test symbol")
						}
						initial := emulator.Registers{PC: uint16(entry), SP: 0xeff0, A: 0xa5, F: 0xd7, HL: 0x1234, BC: 0x5678, DE: 0x9abc, IX: 0x1357, IY: 0x2468}
						z.SetRegisters(initial)
						for count := 0; count < 100000; count++ {
							if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
								break
							}
							z.Step()
						}
						if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
							t.Fatal("native did not restore return PC/SP", z.GetRegisters())
						}
						check("Z80", before, memory, allowed)
						if primitive == "compiled" {
							got := z.GetRegisters()
							if got.A != initial.A || got.F != initial.F || got.HL != initial.HL || got.BC != initial.BC || got.DE != initial.DE || got.IX != initial.IX || got.IY != initial.IY {
								t.Fatalf("preservation: got %+v initial %+v", got, initial)
							}
						}
						measurements = append(measurements, map[string]any{"seed_salt": salt, "T_states": z.GetCycles()})
						t.Logf("seed %d: %d bytes, %d T-states; %d sprite bytes", salt, len(a.Binary), z.GetCycles(), len(writes))
					}
					result, err := json.MarshalIndent(map[string]any{"PASS": true, "mode": mode, "bytes": len(a.Binary), "sprite_bytes": len(writes), "primitive": primitive, "measurements": measurements, "scope": "independent mask and ZX address oracle; two runtime backgrounds; all native RAM except exact spill declarations and bounded four-byte compiled stack; exact PC/SP; raw/final VM for pointer only; preserved AF/BC/DE/HL/IX/IY for compiled"}, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					hardwareSpriteExport(t, prefix+"result.json", append(result, '\n'))
				})
			}
		})
	}
}

func TestHardwareSpriteRejectsInvalidDescriptions(t *testing.T) {
	valid := `destination "linear" 24576 8 4 10; region 1 1 2 2; row "00ff" "1234"; row "aa55" "1188"; via "pointer";`
	cases := map[string]string{
		"empty":                 "",
		"unknown-layout":        strings.Replace(valid, `"linear"`, `"mystery"`, 1),
		"unknown-via":           strings.Replace(valid, `"pointer"`, `"magic"`, 1),
		"missing-destination":   strings.Replace(valid, `destination "linear" 24576 8 4 10;`, "", 1),
		"missing-region":        strings.Replace(valid, `region 1 1 2 2;`, "", 1),
		"duplicate-destination": valid + ` destination "linear" 24576 8 4 10;`,
		"duplicate-region":      valid + ` region 0 0 2 2;`,
		"duplicate-via":         valid + ` via "compiled";`,
		"destination-arity":     strings.Replace(valid, "24576 8 4 10", "24576 8 4", 1),
		"region-arity":          strings.Replace(valid, "region 1 1 2 2", "region 1 1 2", 1),
		"row-arity":             strings.Replace(valid, `row "00ff" "1234"`, `row "00ff"`, 1),
		"via-arity":             strings.Replace(valid, `via "pointer"`, `via "pointer" 1`, 1),
		"unknown-field":         valid + ` traversal "rows";`,
		"negative-base":         strings.Replace(valid, "24576", `"-1"`, 1),
		"address-overflow":      strings.Replace(valid, "24576", "65530", 1),
		"invalid-stride":        strings.Replace(valid, "24576 8 4 10", "24576 8 4 7", 1),
		"region-overflow":       strings.Replace(valid, "region 1 1 2 2", "region 7 1 2 2", 1),
		"negative-region":       strings.Replace(valid, "region 1 1 2 2", `region "-1" 1 2 2`, 1),
		"bad-mask-hex":          strings.Replace(valid, `"00ff"`, `"00gg"`, 1),
		"bad-graphics-hex":      strings.Replace(valid, `"1234"`, `"12gg"`, 1),
		"odd-mask-hex":          strings.Replace(valid, `"00ff"`, `"0ff"`, 1),
		"wrong-mask-width":      strings.Replace(valid, `"00ff"`, `"ff"`, 1),
		"wrong-graphics-width":  strings.Replace(valid, `"1234"`, `"123456"`, 1),
		"missing-row":           strings.Replace(valid, `row "aa55" "1188";`, "", 1),
		"extra-row":             valid + ` row "aa55" "1188";`,
		"zx-arity":              `destination "zx_bitmap_bytes" 16384 32; region 0 0 1 1; row "00" "ff";`,
		"zx-overflow":           `destination "zx_bitmap_bytes" 61440; region 0 0 1 1; row "00" "ff";`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := nanz.ParseWithOpts(hardwareSpriteSource(body), "hardware_sprite_invalid.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err == nil {
				t.Fatal("invalid description was accepted", body)
			}
			if !strings.Contains(err.Error(), "sprite:") {
				t.Fatalf("expected sprite contract diagnostic, got %v", err)
			}
		})
	}
}

// Values live across preserving generated ASM are a distinct ABI requirement
// from checking register contents at the entry/return of a bare sprite call.
func TestHardwareSpriteCompiledLiveValues(t *testing.T) {
	source := `import hw.sprite
global input: u16 = 0
global answer: u16 = 0
fun sprite_test() {
 let keep: u16 = input * 3 + 7
 @sprite() {
  destination "linear" 0x6000 4 1 4
  region 0 0 4 1
  row "ff0055aa" "00880011"
  via "compiled"
 }
 @sprite() {
  destination "linear" 0x6100 1 1 1
  region 0 0 1 1
  row "f0" "05"
  via "compiled"
 }
 answer = keep ^ input
}`
	hardwareSpriteExport(t, "live-values/source.nanz", []byte(source))
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			steps, err := CompileHIRSteps(hardwareSpriteParse(t, source), Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
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
			hardwareSpriteExport(t, "live-values/"+mode+"/program.a80", []byte(assembly))
			hardwareSpriteExport(t, "live-values/"+mode+"/program.bin", a.Binary)
			inputAddress, inputOK := a.Symbols["input"]
			answerAddress, answerOK := a.Symbols["answer"]
			if !inputOK || !answerOK {
				t.Fatal("missing globals", a.Symbols)
			}
			for _, value := range []uint16{0, 1, 2, 255, 256, 0x5555, 0x7fff, 0x8000, 0xfffe, 0xffff} {
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0x8000, a.Binary)
				memory := z.Memory()
				memory[inputAddress], memory[inputAddress+1] = byte(value), byte(value>>8)
				for column := 0; column < 4; column++ {
					memory[0x6000+column] = byte(int(value) + column*37)
				}
				memory[0x6100] = byte(value) ^ 0xa5
				before := append([]byte(nil), memory...)
				memory[0xeff0], memory[0xeff1] = 0, 0x70
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["sprite_test"]), SP: 0xeff0, HL: 0xdead, BC: 0xbeef, DE: 0x1234, A: 0xa5})
				for count := 0; count < 100000; count++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						break
					}
					z.Step()
				}
				if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
					t.Fatal("return", z.GetRegisters())
				}
				got := uint16(memory[answerAddress]) | uint16(memory[answerAddress+1])<<8
				want := (value*3 + 7) ^ value
				if got != want {
					t.Fatalf("input %04x: answer %04x want %04x", value, got, want)
				}
				for index, mask := range []byte{0xff, 0, 0x55, 0xaa} {
					graphics := []byte{0, 0x88, 0, 0x11}[index]
					want := (before[0x6000+index] & mask) | graphics
					if memory[0x6000+index] != want {
						t.Fatalf("sprite[%d] got %02x want %02x", index, memory[0x6000+index], want)
					}
				}
				if want := (before[0x6100] & 0xf0) | 5; memory[0x6100] != want {
					t.Fatalf("second sprite got %02x want %02x", memory[0x6100], want)
				}
			}
			hardwareSpriteExport(t, "live-values/"+mode+"/result.json", []byte("{\"PASS\":true,\"inputs\":10,\"scope\":\"live word across two compiled sprites; both sprite outputs; exact PC/SP\"}\n"))
		})
	}
}
