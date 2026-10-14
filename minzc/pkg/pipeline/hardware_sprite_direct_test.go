package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func hardwareSpriteDirectSource() string {
	var source strings.Builder
	source.WriteString("import hw.sprite_chain\n")
	for sprite, name := range []string{"first", "second"} {
		fmt.Fprintf(&source, "@sprite_chain(\"%s\") {\ndestination \"zx_bitmap_bytes\" 0x4000\nregion 2 0 1 8\nvia \"direct\"\n", name)
		for row := 0; row < 8; row++ {
			fmt.Fprintf(&source, "row \"00\" \"%02x\"\n", 0x11+sprite*0x40+row)
		}
		source.WriteString("}\n")
	}
	for order, sequence := range [][]string{{"first", "second"}, {"second", "first"}} {
		fmt.Fprintf(&source, "fun pair%d() { asm z80 {\nLD HL, pair%d_continue\nPUSH HL\nLD HL, %s\nPUSH HL\nLD HL, %s\nPUSH HL\nRET\npair%d_continue:\n}\n}\n", order, order, sequence[1], sequence[0], order)
	}
	source.WriteString(`
fun screen_addr(x:u8,y:u8)->u16 {
 let h:u8=0x40+(y>>6)*8+(y&7)
 let l:u8=((y>>3)&7)*32+x
 return u16(h)*256+u16(l)
}
`)
	for _, recipe := range []string{"individual", "walk", "inline"} {
		fmt.Fprintf(&source, "fun place_%s(y:u8) {\n", recipe)
		for sprite, name := range []string{"first", "second"} {
			if recipe == "individual" {
				for row := 0; row < 8; row++ {
					fmt.Fprintf(&source, "%s_set_row_%d(screen_addr(%d,y+%d))\n", name, row, 2+sprite, row)
				}
			} else {
				suffix := "_position"
				if recipe == "inline" {
					suffix += "_inline"
				}
				fmt.Fprintf(&source, "%s%s(screen_addr(%d,y))\n", name, suffix, 2+sprite)
			}
		}
		source.WriteString("}\n")
	}
	return source.String()
}

func TestHardwareSpriteDirectRETNative(t *testing.T) {
	source := hardwareSpriteDirectSource()

	for mode, options := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, OptSize: true}, {AssertMode: "none", UseGrace: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			h, err := nanz.ParseWithOpts(source, "direct-sprite.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(h, options)
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil || len(a.Errors) != 0 {
				t.Fatal(err, a.Errors, steps.Assembly)
			}
			if directory := os.Getenv("MINZ_HW_DIRECT_EXPORT"); directory != "" {
				directory = filepath.Join(directory, fmt.Sprint(mode))
				if err = os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte(steps.Assembly), "program.bin": a.Binary} {
					if err = os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
				data, err := json.MarshalIndent(a.Symbols, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(directory, "symbols.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			var rows []map[string]any
			for _, y := range []int{0, 1, 2, 3, 4, 5, 6, 7, 56, 57, 63, 64, 120, 127, 128, 183, 184} {
				for order, sequence := range [][]string{{"first", "second"}, {"second", "first"}} {
					for _, recipe := range []string{"individual", "walk", "inline"} {
						z := emulator.NewRemogattoZ80()
						memory := z.Memory()
						for i := range memory {
							memory[i] = byte(i*37 + y)
						}
						if err = z.LoadMemory(0x8000, a.Binary); err != nil {
							t.Fatal(err)
						}
						writeWord := func(at int, value uint16) { memory[at], memory[at+1] = byte(value), byte(value>>8) }
						writeWord(0xeff0, 0x7000)
						patchExpected := append([]byte(nil), memory...)
						addresses := map[string][]uint16{}
						patchStart := z.GetCycles()
						// The oracle derives addresses from raster coordinates, never from
						// the metafunction's generated initial addresses or patch helpers.
						for sprite, name := range []string{"first", "second"} {
							for row := 0; row < 8; row++ {
								scan := y + row
								address := uint16(0x4000 + ((scan & 192) << 5) + ((scan & 7) << 8) + ((scan & 56) << 2) + 2 + sprite)
								addresses[name] = append(addresses[name], address)
								if recipe == "individual" || row == 0 {
									helper := fmt.Sprintf("%s_set_row_%d", name, row)
									if recipe == "walk" {
										helper = name + "_position"
									}
									if recipe == "inline" {
										helper = name + "_position_inline"
									}
									writeWord(0xeff0, 0x7000)
									z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols[helper]), SP: 0xeff0, HL: address})
									for i := 0; i < 300 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
										z.Step()
									}
									if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
										t.Fatal("patch return", helper, z.GetRegisters())
									}
								}
								operand := a.Symbols[fmt.Sprintf("%s_chain_row_%d", name, row)] + 1
								patchExpected[operand], patchExpected[operand+1] = byte(address), byte(address>>8)
								if uint16(memory[operand])|uint16(memory[operand+1])<<8 != address {
									t.Fatal("patch did not target the final immediate")
								}
							}
						}
						patchT := z.GetCycles() - patchStart
						for at, expected := range patchExpected {
							if recipe == "walk" && at >= 0xeff0-2 && at < 0xeff0 {
								continue
							}
							if memory[at] != expected {
								t.Fatal("patch changed RAM outside immediates/internal call frame", recipe, at)
							}
						}
						writeWord(0xeff0, 0x7000)
						before := append([]byte(nil), memory...)
						z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols[fmt.Sprintf("pair%d", order)]), SP: 0xeff0, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, IX: 0x9abc, IY: 0xcdef})
						start := z.GetCycles()
						var visits []string
						minSP := z.GetSP()
						for i := 0; i < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
							for _, name := range []string{"first", "second"} {
								if z.GetPC() == uint16(a.Symbols[name]) {
									visits = append(visits, name)
								}
							}
							z.Step()
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						if !slices.Equal(visits, sequence) || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 || minSP != 0xeff0-6 {
							t.Fatal("RET sequence/continuation", visits, sequence, z.GetRegisters())
						}
						registers := z.GetRegisters()
						if registers.A != 0x5a || registers.F != 0xa5 || registers.BC != 0x1234 || registers.DE != 0x5678 || registers.IX != 0x9abc || registers.IY != 0xcdef {
							t.Fatal("leaf wrote unrelated registers", registers)
						}
						for sprite, name := range []string{"first", "second"} {
							for row, address := range addresses[name] {
								before[address] = byte(0x11 + sprite*0x40 + row)
							}
						}
						for at, want := range before {
							if at >= 0xeff0-6 && at < 0xeff0 {
								continue // exactly three charged continuation words
							}
							if memory[at] != want {
								t.Fatalf("RAM %04x got%02x want%02x", at, memory[at], want)
							}
						}
						if y == 7 && order == 0 {
							t.Logf("%s: patch %d T; two leaves + RET %d T; module %d B", recipe, patchT, z.GetCycles()-start, len(a.Binary))
						}
						rows = append(rows, map[string]any{"recipe": recipe, "y": y, "order": sequence, "patch_T": patchT, "draw_and_ret_T": z.GetCycles() - start, "min_SP": minSP, "PASS": true})
					}
				}
			}
			if directory := os.Getenv("MINZ_HW_DIRECT_EXPORT"); directory != "" {
				data, err := json.MarshalIndent(map[string]any{"scope": "pure all-RAM Z80: two opaque byte-wide 8-line leaves; no ULA/IRQ injection; patch addresses provided by host oracle", "mode": mode, "binary_bytes": len(a.Binary), "binary_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary)), "cases": rows, "PASS": true}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(directory, fmt.Sprint(mode), "result.json"), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestHardwareSpriteDirectRejectedContracts(t *testing.T) {
	for _, body := range []string{
		"region 2 0 0 0",
		"region 2 0 1 1\nrow \"ff\" \"00\"",
	} {
		_, err := nanz.ParseWithOpts("import hw.sprite_chain\n@sprite_chain(\"bad\") {\ndestination \"zx_bitmap_bytes\" 0x4000\nvia \"direct\"\n"+body+"\n}", "bad-direct.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), "sprite_chain: direct requires") {
			t.Fatal("direct rejected contract", body, err)
		}
	}
	_, err := nanz.ParseWithOpts("import hw.sprite_chain\n@sprite_chain(\"bad\") {\ndestination \"zx_bitmap_bytes\" 0x4001\nvia \"direct\"\nregion 2 0 1 1\nrow \"00\" \"ff\"\n}", "unaligned-direct.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err == nil || !strings.Contains(err.Error(), "direct requires aligned ZX base") {
		t.Fatal("unaligned DOWN_HL base", err)
	}
}

// Include ordinary Nanz screen-address computation, ABI argument setup and
// actual CALLs, so pointer walk is compared with the complete former placement.
func TestHardwareSpritePointerWalkPlacementNative(t *testing.T) {
	for mode, options := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, OptSize: true}, {AssertMode: "none", UseGrace: true}} {
		h, err := nanz.ParseWithOpts(hardwareSpriteDirectSource(), "sprite-position.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		steps, err := CompileHIRSteps(h, options)
		if err != nil {
			t.Fatal(err)
		}
		a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil || len(a.Errors) != 0 {
			t.Fatal(err, a.Errors)
		}
		var cases []map[string]any
		for _, y := range []int{0, 1, 2, 3, 4, 5, 6, 7, 56, 57, 63, 64, 120, 127, 128, 183, 184} {
			for _, recipe := range []string{"individual", "walk", "inline"} {
				z := emulator.NewRemogattoZ80()
				if err = z.LoadMemory(0x8000, a.Binary); err != nil {
					t.Fatal(err)
				}
				z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
				name := "place_" + recipe
				f := steps.MIR2Module.FuncByName(name)
				if len(f.Contract.Params) != 1 {
					t.Fatal("placement ABI", f.Contract)
				}
				registers := emulator.Registers{PC: uint16(a.Symbols[name]), SP: 0xeff0}
				location := assertParamLocation(f.Contract.Params[0], steps.Allocation)
				if location.Kind == mir2.LocMem {
					at, found := a.Symbols[mir2.Z80SpillLabel(name, f.Contract.Params[0].Reg)]
					if !found {
						t.Fatal("placement spill ABI", location)
					}
					z.Memory()[at] = byte(y)
				} else {
					switch location.Name {
					case "A":
						registers.A = byte(y)
					case "B":
						registers.BC = uint16(y) << 8
					case "C":
						registers.BC = uint16(y)
					case "D":
						registers.DE = uint16(y) << 8
					case "E":
						registers.DE = uint16(y)
					case "H":
						registers.HL = uint16(y) << 8
					case "L":
						registers.HL = uint16(y)
					default:
						t.Fatal("placement ABI location", location)
					}
				}
				z.SetRegisters(registers)
				start := z.GetCycles()
				for n := 0; n < 10000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); n++ {
					z.Step()
				}
				if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
					t.Fatal("complete placement return", name, z.GetRegisters())
				}
				for sprite, symbol := range []string{"first", "second"} {
					for row := 0; row < 8; row++ {
						scan := y + row
						want := uint16(0x4000 + ((scan & 192) << 5) + ((scan & 7) << 8) + ((scan & 56) << 2) + 2 + sprite)
						at := a.Symbols[fmt.Sprintf("%s_chain_row_%d", symbol, row)] + 1
						got := uint16(z.Memory()[at]) | uint16(z.Memory()[at+1])<<8
						if got != want {
							t.Fatal("complete placement address", mode, y, recipe, symbol, row, got, want)
						}
					}
				}
				elapsed := z.GetCycles() - start
				cases = append(cases, map[string]any{"recipe": recipe, "y": y, "position_T": elapsed, "PASS": true})
				if y == 7 {
					t.Logf("mode%d %s complete placement %d T", mode, recipe, elapsed)
				}
			}
		}
		if root := os.Getenv("MINZ_HW_DIRECT_EXPORT"); root != "" {
			data, err := json.MarshalIndent(map[string]any{"scope": "all-RAM Z80, complete Nanz placement including address calculation/internal CALLs; no ULA", "mode": mode, "binary_bytes": len(a.Binary), "cases": cases, "PASS": true}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(root, fmt.Sprint(mode))
			if err = os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(directory, "placement.json"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
