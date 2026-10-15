package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Reuse the independent raster oracle, not the generated phase masks.
func TestZXPixelShiftedSpriteNativeVM(t *testing.T)      { runZXPixelSpriteNativeVM(t, false, false) }
func TestZXPixelShiftedSpriteEraseNativeVM(t *testing.T) { runZXPixelSpriteNativeVM(t, true, false) }

func TestZXPixelSpriteSmokeNativeVM(t *testing.T) {
	t.Run("draw", func(t *testing.T) { runZXPixelSpriteNativeVM(t, false, true) })
	t.Run("erase", func(t *testing.T) { runZXPixelSpriteNativeVM(t, true, true) })
}

func runZXPixelSpriteNativeVM(t *testing.T, clear bool, quick bool, sharedRows ...bool) {
	runZXPixelSpriteNativeVMConfigured(t, clear, quick, false, sharedRows...)
}

func TestZXPixelSpriteFinitePhasesNativeVM(t *testing.T) {
	t.Run("draw", func(t *testing.T) { runZXPixelSpriteNativeVMConfigured(t, false, true, true) })
	t.Run("cached-erase", func(t *testing.T) { runZXPixelSpriteNativeVMConfigured(t, true, true, true) })
}

func runZXPixelSpriteNativeVMConfigured(t *testing.T, clear bool, quick bool, finite bool, sharedRows ...bool) {
	source := `import zx.sprite
@zx_sprite("Ship") {
 size 16 8
 erase
 pixel_shifts
 pixels ".#_..##._#_.#..."
 pixels "#._#.._#..##._#."
 pixels "..###_____###..."
 pixels "################"
 pixels "................"
 pixels "_____######_____"
 pixels "#..............#"
 pixels "._#_#_#_#_#_#_.."
}
fun draw_ship(x:u8,y:u8)->bool {
 let actor:Ship=Ship{x:x,y:y}
 if actor.stage()==false {return false}
 return actor.publish()
}`
	if finite {
		source = strings.Replace(source, "pixel_shifts", "pixel_phases 0 1 7", 1)
	}
	height := 8
	if quick {
		height = 1
		if len(sharedRows) > 0 || finite {
			height = 2
		}
		source = strings.Replace(source, "size 16 8", fmt.Sprintf("size 16 %d", height), 1)
		lines := strings.Split(source, "\n")
		kept := lines[:0]
		rows := 0
		first := ""
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "pixels ") {
				rows++
				if rows > height {
					continue
				}
				if rows == 1 {
					first = line
				}
				if rows == 2 && len(sharedRows) > 0 {
					line = first
					if !sharedRows[0] {
						line = strings.ReplaceAll(line, "#", "_")
					}
				}
			}
			kept = append(kept, line)
		}
		source = strings.Join(kept, "\n")
	}
	if clear {
		source = strings.Replace(source, "return actor.publish()", "actor.x=actor.x^7\n actor.y=0\n if actor.stage_previous_erase()==false {return false}\n return actor.erase_staged()", 1)
	}
	if finite && clear {
		// Cache an admitted pose, then reject an unsupported new phase. The old
		// erase address/phase must remain intact while draw readiness is cancelled.
		source = strings.Replace(source, "return actor.erase_staged()", "actor.x=2\n if actor.stage()!=false {return false}\n if actor.publish()!=false {return false}\n return actor.erase_staged()", 1)
	}
	var pixels []string
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pixels ") {
			pixels = append(pixels, strings.Trim(line[len("pixels "):], "\""))
		}
	}
	if len(pixels) != height {
		t.Fatal("fixture height", len(pixels))
	}
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(source, "whole-sprite.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	type position struct{ x, y int }
	var positions []position
	for _, y := range []int{0, 1, 2, 3, 4, 5, 6, 7, 55, 56, 57, 62, 63, 64, 119, 120, 121, 126, 127, 128, 183, 184} {
		for _, x := range []int{0, 1, 2, 3, 4, 5, 6, 7, 112, 113, 233, 239, 240} {
			positions = append(positions, position{x, y})
		}
	}
	positions = append(positions, position{1, 0}, position{7, 10}, position{241, 0}, position{248, 0}, position{255, 0}, position{0, 185}, position{0, 192}, position{0, 255})
	if quick {
		positions = nil
		for x := 0; x < 8; x++ {
			positions = append(positions, position{x, 7})
		}
		positions = append(positions, position{233, 63}, position{240, 127}, position{241, 191}, position{0, 191}, position{0, 192}, position{255, 255})
	}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			steps, e := CompileHIRSteps(parse(), opt)
			if e != nil {
				t.Fatal(e)
			}
			if len(sharedRows) > 0 {
				if strings.Contains(steps.Assembly, "Ship_clear_row_1_phase_0:") {
					t.Fatal("duplicate immutable clear kernel retained")
				}
				if sharedRows[0] && strings.Contains(steps.Assembly, "Ship_row_1_phase_0:") {
					t.Fatal("duplicate immutable draw kernel retained")
				}
				if !sharedRows[0] && !strings.Contains(steps.Assembly, "Ship_row_1_phase_0:") {
					t.Fatal("different ink patterns merged")
				}
			}
			if finite {
				for phase := 2; phase <= 6; phase++ {
					for _, prefix := range []string{"Ship_row_0_phase_", "Ship_clear_row_0_phase_"} {
						if strings.Contains(steps.Assembly, fmt.Sprintf("%s%d:", prefix, phase)) {
							t.Fatal("unsupported kernel retained", phase)
						}
					}
				}
			}
			f := steps.MIR2Module.FuncByName("draw_ship")
			raw := hir.LowerModule(parse())
			descriptor := parse().FuncByName("draw_ship")
			minT, maxT := int(^uint(0)>>1), 0
			for _, pos := range positions {
				valid := pos.x+16 <= 256 && pos.y+height <= 192
				if finite {
					phase := pos.x & 7
					valid = valid && (phase == 0 || phase == 1 || phase == 7)
				}
				expected := int64(0)
				if valid {
					expected = 1
				}
				assertion := hir.Assert{FuncName: "draw_ship", Args: []int64{int64(pos.x), int64(pos.y)}, Expected: expected, Via: "z80"}
				boot := buildAssertBootstrap(assertLoadAddr, assertion, f, steps.Allocation)
				a, e := z80asm.NewAssembler().AssembleString(boot + "\n" + steps.Assembly)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors)
				}
				for _, seed := range []int{0, 173} {
					background := make([]byte, 65536)
					for at := range background {
						background[at] = byte(at*37 + seed)
					}
					wantBitmap := append([]byte(nil), background...)
					if valid {
						oraclePixels := pixels
						if clear {
							oraclePixels = make([]string, len(pixels))
							for r, line := range pixels {
								oraclePixels[r] = strings.ReplaceAll(line, "#", "_")
							}
						}
						wholeSpriteOracle(wantBitmap, oraclePixels, pos.x, pos.y)
					}
					for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
						vm := mir2.NewVM(module)
						vm.EnsureHeap(0x6000)
						vm.WriteHeapSlice(0x4000, background[0x4000:0x5b00])
						got, e := vm.Call("draw_ship", []mir2.Value{{I: int64(pos.x)}, {I: int64(pos.y)}})
						if e != nil || len(got) != 1 || got[0].I != expected {
							t.Fatalf("VM (%d,%d): %v %v", pos.x, pos.y, got, e)
						}
						if actual := vm.ReadHeap(0x4000, 0x1b00); string(actual) != string(wantBitmap[0x4000:0x5b00]) {
							t.Fatalf("VM pixels (%d,%d)", pos.x, pos.y)
						}
					}
					platform := emulator.NewRemogattoZ80WithScreen()
					platform.SetROMEnd(0)
					z := platform.RemogattoZ80
					z.SetSMCTracker(nil)
					memory := z.Memory()
					copy(memory, background)
					if e = z.LoadMemory(assertLoadAddr, a.Binary); e != nil {
						t.Fatal(e)
					}
					z.SetPC(assertLoadAddr)
					for i := 0; z.GetPC() != uint16(a.Symbols["draw_ship"]); i++ {
						if i > 1000 {
							t.Fatal("bootstrap")
						}
						z.Step()
					}
					entrySP := z.GetSP()
					returnPC := uint16(memory[entrySP]) | uint16(memory[entrySP+1])<<8
					want := append([]byte(nil), memory...)
					copy(want[0x4000:0x5b00], wantBitmap[0x4000:0x5b00])
					if valid {
						want[a.Symbols["Ship_ready"]] = 1
						if finite && clear {
							want[a.Symbols["Ship_ready"]] = 0
						}
						for row := 0; row < height; row++ {
							scan := pos.y + row
							address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + pos.x/8
							slot, ok := a.Symbols[fmt.Sprintf("Ship_row_%d$dst$imm", row)]
							if !ok {
								t.Fatal("missing patch slot", row)
							}
							want[slot], want[slot+1] = byte(address), byte(address>>8)
							cache := a.Symbols["Ship_draw_addresses"] + row*2
							want[cache], want[cache+1] = byte(address), byte(address>>8)
							want[a.Symbols["Ship_draw_phases"]+row] = byte(pos.x & 7)
							if clear {
								want[a.Symbols["Ship_erase_ready"]] = 1
								want[a.Symbols["Ship_erase_phases"]+row] = byte(pos.x & 7)
								clearSlot := a.Symbols[fmt.Sprintf("Ship_clear_row_%d$dst$imm", row)]
								want[clearSlot], want[clearSlot+1] = byte(address), byte(address>>8)
							}
						}
					}
					start := z.GetCycles()
					minSP := entrySP
					for i := 0; z.GetPC() != returnPC || z.GetSP() != entrySP+2; i++ {
						if i > 20000 {
							t.Fatalf("return (%d,%d): %+v", pos.x, pos.y, z.GetRegisters())
						}
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
					}
					if minSP < entrySP-64 {
						t.Fatalf("stack exceeded bound: %x", minSP)
					}
					if e = checkAssertZ80Result(z, assertion, f, map[string]*hir.Func{"draw_ship": descriptor}); e != nil {
						t.Fatalf("native (%d,%d): %v\n%s", pos.x, pos.y, e, steps.Assembly)
					}
					for at, v := range want {
						if at >= int(entrySP)-64 && at < int(entrySP) {
							continue
						}
						if memory[at] != v {
							t.Fatalf("native (%d,%d) seed%d RAM%04x=%02x want%02x\n%s", pos.x, pos.y, seed, at, memory[at], v, steps.Assembly)
						}
					}
					elapsed := z.GetCycles() - start
					if valid {
						if elapsed < minT {
							minT = elapsed
						}
						if elapsed > maxT {
							maxT = elapsed
						}
					}
				}
			}
			standalone, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(standalone.Errors) != 0 {
				t.Fatal(e, standalone.Errors)
			}
			t.Logf("%d positions x 2 backgrounds: native + raw/optimized VM; full bitmap/masks, native 64K RAM with exact staging state/SMC operands and a 64-byte bounded stack, PC/SP; full module %d bytes, valid staged operation %d..%d T", len(positions), len(standalone.Binary), minT, maxT)
			if root := os.Getenv("MINZ_PIXEL_SPRITE_EXPORT"); root != "" {
				if clear {
					root = filepath.Join(root, "erase")
				}
				dir := filepath.Join(root, fmt.Sprint(mode))
				if e = os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte(steps.Assembly), "program.bin": standalone.Binary} {
					if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
		})
	}
}

func TestZXPixelSpriteAdmission(t *testing.T) {
	for _, field := range []string{"pixel_shifts 1", "pixel_shifts\n pixel_shifts"} {
		source := "import zx.sprite\n@zx_sprite(\"Invalid\") {\n size 8 1\n pixels \"########\"\n" + field + "\n}"
		_, err := nanz.ParseWithOpts(source, "invalid-pixel-shifts.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), "pixel_shifts") {
			t.Fatalf("%s: %v", field, err)
		}
	}
}

func TestZXPixelSpriteSharedRowsNativeVM(t *testing.T) {
	for _, sameInk := range []bool{false, true} {
		for _, clear := range []bool{false, true} {
			t.Run(fmt.Sprintf("sameInk-%v/erase-%v", sameInk, clear), func(t *testing.T) { runZXPixelSpriteNativeVM(t, clear, true, sameInk) })
		}
	}
}

func TestZXPixelSpriteFinitePhaseAdmission(t *testing.T) {
	for _, field := range []string{"pixel_phases", "pixel_phases -1", "pixel_phases 8", "pixel_phases 0 0", "pixel_phases 0\n pixel_phases 1", "pixel_phases 0 1 2 3 4 5 6 7 0"} {
		for _, descriptor := range []string{"@zx_sprite(\"Invalid\") {", "@zx_sprite_instances(\"Invalid\") {\n instances 2"} {
			source := "import zx.sprite\n" + descriptor + "\n size 8 1\n pixels \"########\"\n" + field + "\n}"
			_, err := nanz.ParseWithOpts(source, "finite-phase-admission.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err == nil || (field != "pixel_phases -1" && !strings.Contains(err.Error(), "phase")) {
				t.Fatalf("%s / %s: %v", descriptor, field, err)
			}
		}
	}
}
