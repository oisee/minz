package pipeline

import (
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

// Test exact clearing and staged erase-old/draw-new, including overlapping masks.
// The independent raster oracle clears opaque pixels to zero, never restores
// background data. Native RAM is checked outside only the bounded call stack.
func TestZXPairedSpriteNative(t *testing.T) {
	sourceBytes, err := os.ReadFile("../../../reports/advanced-demos/whole-sprite/demo.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for _, staged := range []bool{false, true} {
		t.Run(fmt.Sprintf("staged=%v", staged), func(t *testing.T) {
			source := strings.Replace(string(sourceBytes), "size 16 8", "size 16 8\n    erase", 1)
			source = strings.Replace(source, "return ship.draw()", "return ship.erase()", 1)
			if staged {
				source = source[:strings.Index(source, "fun draw_ship")] + `fun draw_ship(x:u8,y:u8)->bool {
 if (x & 7)!=0 {return false}
 if x>232 {return false}
 if y>183 {return false}
 let old:Ship=Ship{x:x,y:y}
 if old.draw()==false {return false}
 if old.stage_erase()==false {return false}
 let next:Ship=Ship{x:x+8,y:y+1}
 if next.stage()==false {return false}
 if old.erase_staged()==false {return false}
 return next.publish()
}`
			}
			var pixels []string
			for _, line := range strings.Split(source, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "pixels ") {
					pixels = append(pixels, strings.Trim(line[len("pixels "):], "\""))
				}
			}
			if len(pixels) != 8 {
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
				for _, x := range []int{0, 112, 240} {
					positions = append(positions, position{x, y})
				}
			}
			positions = append(positions, position{1, 0}, position{7, 10}, position{241, 0}, position{248, 0}, position{255, 0}, position{0, 185}, position{0, 192}, position{0, 255})
			for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
				t.Run(fmt.Sprint(mode), func(t *testing.T) {
					steps, e := CompileHIRSteps(parse(), opt)
					if e != nil {
						t.Fatal(e)
					}
					f := steps.MIR2Module.FuncByName("draw_ship")
					raw := hir.LowerModule(parse())
					descriptor := parse().FuncByName("draw_ship")
					minT, maxT := int(^uint(0)>>1), 0
					maxErase, maxPublish := 0, 0
					for _, pos := range positions {
						valid := pos.x%8 == 0 && pos.x+16 <= 256 && pos.y+8 <= 192
						if staged {
							valid = valid && pos.x <= 232 && pos.y <= 183
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
								clearPixels := make([]string, len(pixels))
								for row, line := range pixels {
									clearPixels[row] = strings.ReplaceAll(line, "#", "_")
								}
								wholeSpriteOracle(wantBitmap, clearPixels, pos.x, pos.y)
								if staged {
									wholeSpriteOracle(wantBitmap, pixels, pos.x+8, pos.y+1)
								}
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
								patch := func(prefix string, x, y int) {
									for row := 0; row < 8; row++ {
										scan := y + row
										address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + x/8
										slot, ok := a.Symbols[fmt.Sprintf("%s%d$dst$imm", prefix, row)]
										if !ok {
											t.Fatal("missing patch slot", prefix, row)
										}
										want[slot], want[slot+1] = byte(address), byte(address>>8)
									}
								}
								patch("Ship_clear_row_", pos.x, pos.y)
								if staged {
									patch("Ship_row_", pos.x+8, pos.y+1)
									want[a.Symbols["Ship_ready"]] = 1
									want[a.Symbols["Ship_erase_ready"]] = 1
								}
							}

							type watchedCall struct {
								entry, ret, sp uint16
								start, elapsed int
								screen         []byte
								active         bool
							}
							watchers := []watchedCall{{entry: uint16(a.Symbols["Ship_erase_staged"])}, {entry: uint16(a.Symbols["Ship_publish"])}, {entry: uint16(a.Symbols["Ship_stage_erase"])}, {entry: uint16(a.Symbols["Ship_stage"])}}
							start := z.GetCycles()
							minSP := entrySP
							for i := 0; z.GetPC() != returnPC || z.GetSP() != entrySP+2; i++ {
								if i > 20000 {
									t.Fatalf("return (%d,%d): %+v", pos.x, pos.y, z.GetRegisters())
								}
								if staged {
									for wi := range watchers {
										w := &watchers[wi]
										if !w.active && z.GetPC() == w.entry {
											w.active = true
											w.sp = z.GetSP()
											w.ret = uint16(memory[w.sp]) | uint16(memory[w.sp+1])<<8
											w.start = z.GetCycles()
											if wi >= 2 {
												w.screen = append([]byte(nil), memory[0x4000:0x5b00]...)
											}
										}
									}
								}
								z.Step()
								if staged {
									for wi := range watchers {
										w := &watchers[wi]
										if w.active && z.GetPC() == w.ret && z.GetSP() == w.sp+2 {
											w.active = false
											w.elapsed = z.GetCycles() - w.start
											if wi >= 2 && string(w.screen) != string(memory[0x4000:0x5b00]) {
												t.Fatal("staging changed bitmap or attributes", wi, pos)
											}
										}
									}
								}
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
								if staged {
									if watchers[0].elapsed == 0 || watchers[1].elapsed == 0 || watchers[2].elapsed == 0 || watchers[3].elapsed == 0 {
										t.Fatal("missing staged kernel return", watchers)
									}
									maxErase = max(maxErase, watchers[0].elapsed)
									maxPublish = max(maxPublish, watchers[1].elapsed)
								}
							}
						}
					}
					standalone, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
					if e != nil || len(standalone.Errors) != 0 {
						t.Fatal(e, standalone.Errors)
					}
					t.Logf("%d positions x 2 backgrounds: native + raw/optimized VM; full bitmap/masks, native 64K RAM except 64-byte bounded stack and exact SMC operands, PC/SP; full module %d bytes, valid clear/transition %d..%d T", len(positions), len(standalone.Binary), minT, maxT)
					if staged {
						t.Logf("staged clear kernel <=%d T; staged draw kernel <=%d T; excludes placement and CALL", maxErase, maxPublish)
					}
					if root := os.Getenv("MINZ_PAIRED_SPRITE_EXPORT"); root != "" {
						dir := filepath.Join(root, fmt.Sprintf("staged-%v", staged), fmt.Sprint(mode))
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
		})
	}
}

func TestZXPairedSpriteAdmission(t *testing.T) {
	for _, tc := range []struct{ body, diagnostic string }{
		{"erase 1", "erase takes no arguments"},
		{"erase\nerase", "duplicate erase"},
	} {
		source := "import zx.sprite\n@zx_sprite(\"Bad\") {\nsize 8 1\npixels \"########\"\n" + tc.body + "\n}"
		_, err := nanz.ParseWithOpts(source, "bad-erase.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
			t.Fatalf("%s: %v", tc.body, err)
		}
	}
	source := `import zx.sprite
@zx_sprite_instances("Pair") {
 instances 2
 size 8 1
 erase
 pixels ".#_#_#_."
}
fun run()->u8 {
 let a:Pair_0=Pair_0{x:0,y:0}
 let b:Pair_1=Pair_1{x:8,y:0}
 if a.stage_erase()==false {return 0}
 if b.stage()==false {return 0}
 if a.erase_staged()==false {return 0}
 if b.publish()==false {return 0}
 let first:^u8=ptr(0x4000)
 let second:^u8=ptr(0x4001)
 if first^!=0x81 {return 0}
 if second^!=0xd5 {return 0}
 return 1
}`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "paired-instances.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for _, opt := range []Options{{AssertMode: "none", ContractOpt: true}, {AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, map[uint16]byte{0x4000: 255, 0x4001: 255}, 1)
	}
	steps, err := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
		vm := mir2.NewVM(module)
		vm.EnsureHeap(0x6000)
		vm.WriteHeap(0x4000, 255)
		vm.WriteHeap(0x4001, 255)
		got, err := vm.Call("run", nil)
		if err != nil || len(got) != 1 || got[0].I != 1 {
			t.Fatal(got, err)
		}
		if b := vm.ReadHeap(0x4000, 2); b[0] != 0x81 || b[1] != 0xd5 {
			t.Fatalf("independent instances %x", b)
		}
	}
}
