package pipeline

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestZXPixelSpriteShiftBoundsNativeVM(t *testing.T) {
	source := `import zx.sprite
@zx_sprite("Ripple") {
 size 16 2
 erase
 pixel_shifts
 pixels ".#_#_#_.##__..#_"
 pixels "##..__##.#_#_#_."
}
global shifts:[i8;2]
fun render(x:u8,y:u8,amount:u8)->u8 {
 shifts[0]=0
 shifts[1]=i8(amount)
 let sprite:Ripple=Ripple{x:x,y:y}
 if sprite.stage_shifted(&shifts[0])==false {
   if sprite.publish()!=false {return 2}
   return 0
 }
 if sprite.publish()==false {return 2}
 return 1
}`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "pixel-shift-bounds.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	type pose struct{ x, y, shift int }
	var cases []pose
	for _, y := range []int{0, 7, 63, 127, 190, 191} {
		for _, x := range []int{0, 1, 7, 8, 233, 239, 240, 248} {
			for _, shift := range []int{-128, -1, 0, 1, 127} {
				cases = append(cases, pose{x, y, shift})
			}
		}
	}
	cases = append(cases, pose{1, 0, 0}, pose{255, 0, 0}, pose{8, 255, 0})
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			steps, err := CompileHIRSteps(parse(), opt)
			if err != nil {
				t.Fatal(err)
			}
			f := steps.MIR2Module.FuncByName("render")
			raw := hir.LowerModule(parse())
			for _, p := range cases {
				valid := p.y+2 <= 192 && p.x+16 <= 256 && p.x+p.shift >= 0 && p.x+p.shift+16 <= 256
				expected := int64(0)
				if valid {
					expected = 1
				}
				background := make([]byte, 6912)
				for i := range background {
					background[i] = byte(i*37 + 173)
				}
				want := append([]byte(nil), background...)
				if valid {
					screen := make([]byte, 65536)
					copy(screen[0x4000:], background)
					wholeSpriteOracle(screen, []string{".#_#_#_.##__..#_"}, p.x, p.y)
					wholeSpriteOracle(screen, []string{"##..__##.#_#_#_."}, p.x+p.shift, p.y+1)
					copy(want, screen[0x4000:0x5b00])
				}
				args := []int64{int64(p.x), int64(p.y), int64(byte(p.shift))}
				for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
					vm := mir2.NewVM(module)
					vm.EnsureHeap(0x6000)
					vm.WriteHeapSlice(0x4000, background)
					got, e := vm.Call("render", []mir2.Value{{I: args[0]}, {I: args[1]}, {I: args[2]}})
					if e != nil || len(got) != 1 || got[0].I != expected {
						t.Fatalf("VM %v: %v %v", p, got, e)
					}
					if !bytes.Equal(vm.ReadHeap(0x4000, 6912), want) {
						t.Fatalf("VM screen %v", p)
					}
				}
				assertion := hir.Assert{FuncName: "render", Args: args, Expected: expected, Via: "z80"}
				boot := buildAssertBootstrap(assertLoadAddr, assertion, f, steps.Allocation)
				a, e := z80asm.NewAssembler().AssembleString(boot + "\n" + steps.Assembly)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a.Errors)
				}
				platform := emulator.NewRemogattoZ80WithScreen()
				platform.SetROMEnd(0)
				z := platform.RemogattoZ80
				z.SetSMCTracker(nil)
				memory := z.Memory()
				copy(memory[0x4000:0x5b00], background)
				if e = z.LoadMemory(assertLoadAddr, a.Binary); e != nil {
					t.Fatal(e)
				}
				z.SetPC(assertLoadAddr)
				for i := 0; z.GetPC() != uint16(a.Symbols["render"]); i++ {
					if i > 1000 {
						t.Fatal("bootstrap")
					}
					z.Step()
				}
				sp := z.GetSP()
				ret := uint16(memory[sp]) | uint16(memory[sp+1])<<8
				slots := make([]byte, 4)
				for row := 0; row < 2; row++ {
					at := a.Symbols[fmt.Sprintf("Ripple_row_%d$dst$imm", row)]
					copy(slots[row*2:], memory[at:at+2])
				}
				for i := 0; z.GetPC() != ret || z.GetSP() != sp+2; i++ {
					if i > 20000 {
						t.Fatalf("return %v", p)
					}
					z.Step()
				}
				if e = checkAssertZ80Result(z, assertion, f, map[string]*hir.Func{"render": parse().FuncByName("render")}); e != nil {
					t.Fatalf("native %v: %v", p, e)
				}
				if !bytes.Equal(memory[0x4000:0x5b00], want) {
					t.Fatalf("native screen %v", p)
				}
				if !valid {
					for row := 0; row < 2; row++ {
						at := a.Symbols[fmt.Sprintf("Ripple_row_%d$dst$imm", row)]
						if !bytes.Equal(slots[row*2:row*2+2], memory[at:at+2]) {
							t.Fatalf("rejected staging patched row %d: %v", row, p)
						}
					}
				}
			}
			t.Logf("%d positions: signed extremes, late-row rejection, scanline/third boundaries, masks, exact 6912-byte screen, raw/final VM, native PC/SP", len(cases))
		})
	}
}
