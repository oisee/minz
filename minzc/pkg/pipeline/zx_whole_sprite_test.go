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
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
)

// Pixel oracle uses raster coordinates, not the generator's masks or DOWN_HL.
func wholeSpriteOracle(memory []byte, pixels []string, x, y int) {
	for row, line := range pixels {
		for col, ch := range line {
			if ch == '.' {
				continue
			}
			scan, px := y+row, x+col
			at := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + px/8
			bit := byte(128 >> (px % 8))
			if ch == '#' {
				memory[at] |= bit
			} else {
				memory[at] &= ^bit
			}
		}
	}
}

func TestZXWholeSpriteNative(t *testing.T) {
	sourceBytes, err := os.ReadFile("../../../reports/advanced-demos/whole-sprite/demo.nanz")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
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
			for _, pos := range positions {
				valid := pos.x%8 == 0 && pos.x+16 <= 256 && pos.y+8 <= 192
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
						wholeSpriteOracle(wantBitmap, pixels, pos.x, pos.y)
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
						for row := 0; row < 8; row++ {
							scan := pos.y + row
							address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + pos.x/8
							slot, ok := a.Symbols[fmt.Sprintf("Ship_row_%d$dst$imm", row)]
							if !ok {
								t.Fatal("missing patch slot", row)
							}
							want[slot], want[slot+1] = byte(address), byte(address>>8)
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
			t.Logf("%d positions x 2 backgrounds: native + raw/optimized VM; full bitmap/masks, native 64K RAM except 64-byte bounded stack and exact SMC operands, PC/SP; full module %d bytes, valid draw %d..%d T", len(positions), len(standalone.Binary), minT, maxT)
			if root := os.Getenv("MINZ_WHOLE_SPRITE_EXPORT"); root != "" {
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

func TestZXWholeSpriteAdmission(t *testing.T) {

	// Fully transparent and fully opaque rows also exercise empty SMC kernels.
	source := `import zx.sprite
@zx_sprite("Probe") {
 size 16 3
 pixels "................"
 pixels "########________"
 pixels "........########"
}
fun run()->u8 {
 let actor:Probe=Probe{x:32,y:63}
 if actor.draw()==false {return 0}
 let a:^u8=ptr(20452)
 let b:^u8=ptr(18436)
 let c:^u8=ptr(18692)
 return a^+b^+c^
}`
	// y=63 -> 0x4fe4, y=64 -> 0x4804, y=65 -> 0x4904.
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(source, "transparent-sprite.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	mem := map[uint16]byte{0x4fe4: 90, 0x4fe5: 90, 0x4804: 90, 0x4805: 90, 0x4904: 90, 0x4905: 90}
	want := map[int64]byte{0x4fe4: 90, 0x4fe5: 90, 0x4804: 255, 0x4805: 0, 0x4904: 90, 0x4905: 255}
	vm := mir2.NewVM(hir.LowerModule(parse()))
	vm.EnsureHeap(0x6000)
	for at, v := range mem {
		vm.WriteHeap(int64(at), v)
	}
	got, e := vm.Call("run", nil)
	if e != nil || len(got) != 1 || got[0].I != 179 {
		t.Fatalf("transparent rows: %v %v", got, e)
	}
	for at, v := range want {
		if vm.ReadHeap(at, 1)[0] != v {
			t.Fatalf("transparent row %04x", at)
		}
	}
	for _, opt := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, mem, 179)
	}
	for _, tc := range []struct{ body, diagnostic string }{
		{`size 16 1 pixels "........"`, "width mismatch"},
		{`size 8 2 pixels "........"`, "row count"},
		{`size 8 1 pixels "...x...."`, "expected #"},
		{`size 7 1 pixels "......."`, "multiple of eight"},
		{`size 72 1 pixels "........"`, "maximum width"},
		{`size 8 65 pixels "........"`, "maximum height"},
		{`size 8 1 size 8 1 pixels "........"`, "duplicate size"},
		{`pixels "........"`, "missing size"},
		{`size 8 1 row "........"`, "unknown field"},
	} {
		_, err := nanz.ParseWithOpts("import zx.sprite\n@zx_sprite(\"Bad\") {\n"+strings.NewReplacer(" pixels", "\npixels", " size", "\nsize", " row", "\nrow").Replace(tc.body)+"\n}", "bad-sprite.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
			t.Fatalf("%s: %v want%q", tc.body, err, tc.diagnostic)
		}
	}
}

// Observe the actual draw activation and ROM interrupts while the ULA fetches
// the screen. This is a static whole-sprite showcase, not yet Tetris animation.
type wholeSpriteMachineObserver struct {
	spectrum.CPUCore
	machine                  *spectrum.Machine
	entry, returnPC, entrySP uint16
	active, completed        bool
	drawT, start             int64
	irq                      int
	t                        *testing.T
}

func (c *wholeSpriteMachineObserver) Interrupt() {
	if c.IFF1() {
		c.irq++
	}
	c.CPUCore.Interrupt()
}
func (c *wholeSpriteMachineObserver) DoOpcode() {
	if c.PC() == c.entry && !c.active && !c.completed {
		c.active = true
		c.entrySP = c.SP()
		c.returnPC = uint16(c.machine.Memory.ReadByteInternal(c.SP())) | uint16(c.machine.Memory.ReadByteInternal(c.SP()+1))<<8
		c.start = c.machine.AbsoluteTStates()
	}
	if c.active && c.PC() == c.returnPC {
		if c.SP() != c.entrySP+2 {
			c.t.Fatal("draw stack", c.SP(), c.entrySP)
		}
		c.drawT = c.machine.AbsoluteTStates() - c.start
		c.active = false
		c.completed = true
	}
	c.CPUCore.DoOpcode()
}

func TestZXWholeSpriteMachine(t *testing.T) {
	src, e := os.ReadFile("../../../reports/advanced-demos/whole-sprite/demo.nanz")
	if e != nil {
		t.Fatal(e)
	}
	rom, e := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if e != nil {
		t.Fatal(e)
	}
	var pixels []string
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pixels ") {
			pixels = append(pixels, strings.Trim(line[len("pixels "):], "\""))
		}
	}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		for _, y := range []int{0, 1, 2, 3, 4, 5, 6, 7, 63, 127, 184} {
			t.Run(fmt.Sprintf("%d/y%d", mode, y), func(t *testing.T) {
				source := string(src) + fmt.Sprintf(`
fun main(){
 asm z80 {EI
 HALT
 DI}
 draw_ship(112,%d)
 asm z80 {EI}
 while true {asm z80 {HALT}}
}
`, y)
				h, e := nanz.ParseWithOpts(source, "whole-sprite-machine.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
				if e != nil {
					t.Fatal(e)
				}
				steps, e := CompileHIRSteps(h, opt)
				if e != nil {
					t.Fatal(e)
				}
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors)
				}
				m, e := spectrum.New48K(rom)
				if e != nil {
					t.Fatal(e)
				}
				expected := make([]byte, 65536)
				for at := 0x4000; at < 0x5800; at++ {
					expected[at] = byte(at*37 + 173)
					m.Memory.Write(uint16(at), expected[at], false)
				}
				for at := 0x5800; at < 0x5b00; at++ {
					expected[at] = 0x47
					m.Memory.Write(uint16(at), 0x47, false)
				}
				wholeSpriteOracle(expected, pixels, 112, y)
				for i, b := range a.Binary {
					m.Memory.Write(0x8000+uint16(i), b, false)
				}
				m.CPU.SetPC(uint16(a.Symbols["main"]))
				m.CPU.SetSP(0xeff0)
				m.CPU.SetIY(0x5c3a)
				m.CPU.SetIM(1)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				observer := &wholeSpriteMachineObserver{CPUCore: m.CPU, machine: m, entry: uint16(a.Symbols["draw_ship"]), t: t}
				m.CPU = observer
				checked := 0
				for frame := 0; frame < 4; frame++ {
					m.RunFrame()
					if !observer.completed {
						continue
					}
					fb := m.ULA.Framebuffer()
					for scan := 0; scan < 192; scan++ {
						for px := 0; px < 256; px++ {
							at := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + px/8
							v := byte(0)
							if expected[at]&(128>>uint(px%8)) != 0 {
								v = 255
							}
							offset := ((scan+m.Mode.BorderTop)*m.Mode.TotalPixelWidth + px + m.Mode.BorderLeft) * 4
							if fb[offset] != v || fb[offset+1] != v || fb[offset+2] != v || fb[offset+3] != 255 {
								t.Fatalf("fetched frame%d pixel(%d,%d): %v want%d", frame, px, scan, fb[offset:offset+4], v)
							}
						}
					}
					for at := 0x4000; at < 0x5b00; at++ {
						if m.Memory.ReadByteInternal(uint16(at)) != expected[at] {
							t.Fatalf("screen %04x", at)
						}
					}
					checked++
				}
				if checked < 2 || observer.irq == 0 || observer.drawT >= int64(m.Mode.TStatesPerFrame()) {
					t.Fatalf("frames%d IRQ%d drawT%d", checked, observer.irq, observer.drawT)
				}
				t.Logf("%d fetched complete frames; ROM IRQ%d; contended draw %dT; PC/SP", checked, observer.irq, observer.drawT)
				if root := os.Getenv("MINZ_WHOLE_SPRITE_EXPORT"); root != "" {
					dir := filepath.Join(root, "machine", fmt.Sprint(mode), fmt.Sprint(y))
					if e = os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					receipt := fmt.Sprintf("fetched_frames=%d\nROM_IRQ=%d\ncontended_draw_T=%d\n", checked, observer.irq, observer.drawT)
					for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte(steps.Assembly), "program.bin": a.Binary, "receipt.txt": []byte(receipt)} {
						if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
					if e = formats.SaveSNA(filepath.Join(dir, "running.sna"), m); e != nil {
						t.Fatal(e)
					}
				}
			})
		}
	}
}

func TestZXStagedSpriteNative(t *testing.T) {

	directSource, err := os.ReadFile("../../../reports/advanced-demos/whole-sprite/staged.nanz")
	if err != nil {
		t.Fatal(err)
	}
	cancelSource := string(directSource) + `
fun cancel_direct()->bool {
 let block:Block=Block{x:112,y:0}
 if block.stage()==false {return false}
 if block.draw()==false {return false}
 return block.publish()==false
}`
	parseCancel := func() *hir.Module {
		h, e := nanz.ParseWithOpts(cancelSource, "cancel-direct-sprite.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	vmCancel := mir2.NewVM(hir.LowerModule(parseCancel()))
	vmCancel.EnsureHeap(0x6000)
	got, err := vmCancel.Call("cancel_direct", nil)
	if err != nil || len(got) != 1 || got[0].I != 1 {
		t.Fatal("direct draw did not cancel staged pose", got, err)
	}
	for _, opt := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parseCancel(), "cancel_direct", opt, nil, nil, 1)
	}
	src, e := os.ReadFile("../../../reports/advanced-demos/whole-sprite/staged.nanz")
	if e != nil {
		t.Fatal(e)
	}
	pixels := []string{"________", "_######_", "_######_", "_######_", "_######_", "_######_", "_######_", "________"}
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(string(src), "staged-sprite.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			steps, e := CompileHIRSteps(parse(), opt)
			if e != nil {
				t.Fatal(e)
			}
			minPub, maxPub := int(^uint(0)>>1), 0
			for _, y := range []int{0, 1, 2, 3, 4, 5, 6, 7, 63, 127, 184, 185} {
				valid := y <= 184
				expected := int64(0)
				if valid {
					expected = 1
				}
				assertion := hir.Assert{FuncName: "stage_block", Args: []int64{112, int64(y)}, Expected: expected, Via: "z80"}
				f := steps.MIR2Module.FuncByName("stage_block")
				boot := buildAssertBootstrap(assertLoadAddr, assertion, f, steps.Allocation)
				a, e := z80asm.NewAssembler().AssembleString(boot + "\n" + steps.Assembly)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors)
				}
				for _, seed := range []int{0, 173} {
					platform := emulator.NewRemogattoZ80WithScreen()
					platform.SetROMEnd(0)
					z := platform.RemogattoZ80
					z.SetSMCTracker(nil)
					memory := z.Memory()
					for at := range memory {
						memory[at] = byte(at*37 + seed)
					}
					if err := z.LoadMemory(assertLoadAddr, a.Binary); err != nil {
						t.Fatal(err)
					}
					z.SetPC(assertLoadAddr)
					for i := 0; z.GetPC() != uint16(a.Symbols["stage_block"]); i++ {
						if i > 1000 {
							t.Fatal("bootstrap")
						}
						z.Step()
					}
					entrySP := z.GetSP()
					returnPC := uint16(memory[entrySP]) | uint16(memory[entrySP+1])<<8
					checkMemory := func(want []byte) {
						for at, v := range want {
							if at >= int(entrySP)-64 && at < int(entrySP) {
								continue
							}
							if memory[at] != v {
								t.Fatalf("y%d seed%d RAM%04x=%02x want%02x", y, seed, at, memory[at], v)
							}
						}
					}
					execute := func(name string, expected int64) int {
						begin := z.GetCycles()
						minimum := entrySP
						for i := 0; z.GetPC() != returnPC || z.GetSP() != entrySP+2; i++ {
							if i > 20000 {
								t.Fatal("return", name, z.GetRegisters())
							}
							z.Step()
							if z.GetSP() < minimum {
								minimum = z.GetSP()
							}
						}
						if minimum < entrySP-64 {
							t.Fatal("stack bound", minimum)
						}
						assertion := hir.Assert{FuncName: name, Expected: expected, Via: "z80"}
						if e = checkAssertZ80Result(z, assertion, steps.MIR2Module.FuncByName(name), map[string]*hir.Func{name: parse().FuncByName(name)}); e != nil {
							t.Fatal(e)
						}
						return z.GetCycles() - begin
					}
					want := append([]byte(nil), memory...)
					initialScreen := append([]byte(nil), memory[0x4000:0x5b00]...)
					ready, ok := a.Symbols["Block_ready"]
					if !ok {
						t.Fatal("ready symbol")
					}
					want[ready] = byte(expected)
					if valid {
						for row := 0; row < 8; row++ {
							scan := y + row
							address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + 112/8
							slot := a.Symbols[fmt.Sprintf("Block_row_%d$dst$imm", row)]
							want[slot], want[slot+1] = byte(address), byte(address>>8)
						}
					}
					execute("stage_block", expected)
					checkMemory(want) // staging never writes bitmap.
					if valid {
						wholeSpriteOracle(want, pixels, 112, y)
					}
					for repeat := 0; repeat < 2; repeat++ {
						memory[entrySP], memory[entrySP+1] = byte(returnPC), byte(returnPC>>8)
						z.SetPC(uint16(a.Symbols["publish_block"]))
						z.SetSP(entrySP)
						elapsed := execute("publish_block", expected)
						checkMemory(want)
						if valid {
							if elapsed < minPub {
								minPub = elapsed
							}
							if elapsed > maxPub {
								maxPub = elapsed
							}
						}
					}
					for _, module := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
						vm := mir2.NewVM(module)
						vm.EnsureHeap(0x6000)
						vm.WriteHeapSlice(0x4000, initialScreen)
						untouched := vm.ReadHeap(0x4000, 0x1b00)
						got, e := vm.Call("publish_block", nil)
						if e != nil || got[0].I != 0 {
							t.Fatal("unprepared VM", got, e)
						}
						got, e = vm.Call("stage_block", []mir2.Value{{I: 112}, {I: int64(y)}})
						if e != nil || got[0].I != expected {
							t.Fatal("stage VM", got, e)
						}
						if string(vm.ReadHeap(0x4000, 0x1b00)) != string(untouched) {
							t.Fatal("stage VM wrote screen")
						}
						got, e = vm.Call("publish_block", nil)
						if e != nil || got[0].I != expected {
							t.Fatal("publish VM", got, e)
						}
						if string(vm.ReadHeap(0x4000, 0x1b00)) != string(want[0x4000:0x5b00]) {
							t.Fatal("publish VM screen")
						}
						if valid {
							got, e = vm.Call("stage_block", []mir2.Value{{I: 112}, {I: 185}})
							if e != nil || got[0].I != 0 {
								t.Fatal("cancel VM", got, e)
							}
							got, e = vm.Call("publish_block", nil)
							if e != nil || got[0].I != 0 {
								t.Fatal("cancel did not inhibit publication", got, e)
							}
						}
					}
				}
			}
			t.Logf("24 staged poses and 48 repeated publications; full 64K except bounded stack and exact SMC/state; publish %d..%d T", minPub, maxPub)
		})
	}
}
