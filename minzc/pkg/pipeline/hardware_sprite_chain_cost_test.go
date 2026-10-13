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
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestHardwareSpriteChainMeasuredCost(t *testing.T) {
	var measurements []map[string]any
	for _, width := range []int{4, 8, 16} {
		for _, via := range []string{"pointer", "compiled", "push"} {
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(fmt.Sprintf("%d/%s/%s", width, via, mode), func(t *testing.T) {
					const height = 8
					rows := make([][]byte, height)
					for row := range rows {
						rows[row] = make([]byte, width)
						for x := range rows[row] {
							rows[row][x] = byte(row*17 + x*13 + 3)
						}
					}
					var b strings.Builder
					b.WriteString("global desired_x: u16 = 4\nglobal desired_y: u16 = 56\n")
					if via != "pointer" {
						fmt.Fprintf(&b, "import hw.sprite_chain\n@sprite_chain(\"actor\") {\ndestination \"zx_bitmap_bytes\" 0x4000\nregion 4 56 %d %d\n", width, height)
						for _, row := range rows {
							fmt.Fprintf(&b, "row %q %q\n", strings.Repeat("00", width), hex.EncodeToString(row))
						}
						fmt.Fprintf(&b, "via %q\nirq \"disabled\"\n}\n", via)
					} else {
						for row := range rows {
							fmt.Fprintf(&b, "global row_%d: u16 = 0\n", row)
						}
					}
					b.WriteString("fun position() -> void {\n")
					for row := range rows {
						fmt.Fprintf(&b, "let y_%d: u16 = desired_y + %d\nlet address_%d: u16 = 0x4000 + ((y_%d & 192) << 5) + ((y_%d & 7) << 8) + ((y_%d & 56) << 2) + desired_x\n", row, row, row, row, row, row)
						if via == "pointer" {
							fmt.Fprintf(&b, "row_%d = address_%d\n", row, row)
						} else {
							fmt.Fprintf(&b, "actor_set_row_%d(address_%d", row, row)
							if via == "push" {
								fmt.Fprintf(&b, " + %d", width)
							}
							b.WriteString(")\n")
						}
					}
					b.WriteString("}\n")
					if via == "pointer" {
						b.WriteString("fun actor() -> void {\n")
						for row, data := range rows {
							fmt.Fprintf(&b, "{ var p: ^u8 = ptr(row_%d)\n", row)
							for x, v := range data {
								fmt.Fprintf(&b, "p^ = %d\n", v)
								if x+1 < width {
									b.WriteString("p = p + 1\n")
								}
							}
							b.WriteString("}\n")
						}
						b.WriteString("}\n")
					}
					source := b.String()
					h, err := nanz.ParseWithOpts(source, "cost-chain.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
					if err != nil {
						t.Fatal(err)
					}
					steps, err := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
					if err != nil {
						t.Fatal(err)
					}
					assembly := "ORG 0x8000\n" + steps.Assembly
					a, err := z80asm.NewAssembler().AssembleString(assembly)
					if err != nil || len(a.Errors) > 0 {
						t.Fatal(err, a.Errors)
					}
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					for i := range mem {
						mem[i] = byte(i*31 + 7)
					}
					z.LoadMemory(0x8000, a.Binary)
					call := func(name string) int64 {
						mem[0xeff0], mem[0xeff1] = 0, 0x70
						before := z.GetCycles()
						z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols[name]), SP: 0xeff0})
						for n := 0; n < 100000; n++ {
							if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
								break
							}
							z.Step()
						}
						if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
							t.Fatalf("%s PC/SP %04x %04x", name, z.GetPC(), z.GetSP())
						}
						return int64(z.GetCycles() - before)
					}
					for _, xy := range [][2]int{{4, 56}, {9, 63}, {16, 128}} {
						x, y := xy[0], xy[1]
						mem[a.Symbols["desired_x"]], mem[a.Symbols["desired_x"]+1] = byte(x), 0
						mem[a.Symbols["desired_y"]], mem[a.Symbols["desired_y"]+1] = byte(y), 0
						moveT := call("position")
						drawT := call("actor")
						for row, data := range rows {
							scan := y + row
							addr := 0x4000 + (scan&0xc0)*32 + (scan&7)*256 + (scan&0x38)*4 + x
							for col, v := range data {
								if mem[addr+col] != v {
									t.Fatalf("%s row %d byte %d incorrect", via, row, col)
								}
							}
						}
						measurements = append(measurements, map[string]any{"width_bytes": width, "height": height, "via": via, "mode": mode, "x": x, "y": y, "move_T": moveT, "draw_T": drawT, "total_T": moveT + drawT, "module_bytes": len(a.Binary)})
						t.Logf("width%d %s %s position(%d,%d): move=%d draw=%d total=%d module=%d", width, via, mode, x, y, moveT, drawT, moveT+drawT, len(a.Binary))
					}
					if d := os.Getenv("MINZ_HW_CHAIN_COST_EXPORT"); d != "" {
						dir := filepath.Join(d, fmt.Sprint(width), via, mode)
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte(assembly), "program.bin": a.Binary} {
							if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
								t.Fatal(err)
							}
						}
					}
				})
			}
		}
	}
	if d := os.Getenv("MINZ_HW_CHAIN_COST_EXPORT"); d != "" {
		data, _ := json.MarshalIndent(map[string]any{"scope": "same opaque bytes and runtime positions; move computes 8 ZX row addresses, stores pointers or patches instructions; natural row links initialized once; focused Z80 CALL body including RET, excluding external CALL, DI/EI and ULA/IRQ; pointer baseline uses specialized ordinary Nanz stores; whole memory proof is separate native suite", "measurements": measurements}, "", "  ")
		if err := os.WriteFile(filepath.Join(d, "measurements.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
