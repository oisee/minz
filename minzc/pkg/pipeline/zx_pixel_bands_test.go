package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// The oracle derives each byte from raster coordinates, independently of the
// kernel's alternating-AF width stash and high-byte scanline progression.
func TestZXPixelBandsNative(t *testing.T) {
	source := `import zx.pixel_bands
import zx.sprite
@pixel_bands("bands") { rows 2 }
@pixel_bands("colored") {
 rows 2
 attributes 1
}
@zx_sprite("Tile") {
 size 8 8
 pixels "........"
 pixels ".######."
 pixels ".#____#."
 pixels ".#____#."
 pixels ".#____#."
 pixels ".#____#."
 pixels ".######."
 pixels "........"
}
fun copy_band(source:u16,destination:u16,width:u8)->u8 {
 pixel_band_copy(ptr(source),ptr(destination),width)
 return 1
}
fun copy_prepared(source:u16,destination:u16,width:u8)->u8 {
 bands_sources[0]=source bands_destinations[0]=destination
 bands_widths[0]=width bands_count=1
 bands_publish()
 return 1
}
fun copy_attributes(source:u16,destination:u16,width:u8,attrs:u16)->u8 {
 pixel_band_copy_attributes(ptr(source),ptr(destination),width,ptr(attrs))
 return 1
}
fun copy_colored(source:u16,destination:u16,width:u8,attrs:u16)->u8 {
 colored_sources[0]=source colored_destinations[0]=destination
 colored_widths[0]=width colored_attributes[0]=attrs colored_count=1
 colored_publish()
 return 1
}
fun draw_surface(x:u8,y:u8,base:u16)->u8 {
 let tile:Tile=Tile{x:x,y:y}
 if tile.stage_on(base)==false {return 0}
 if tile.publish()==false {return 0}
 return 1
}`
	pixels := []string{"........", ".######.", ".#____#.", ".#____#.", ".#____#.", ".#____#.", ".######.", "........"}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			h, err := nanz.ParseWithOpts(source, "pixel-bands.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(h, opt)
			if err != nil {
				t.Fatal(err)
			}
			run := func(name string, args []int64, result int64, oracle func([]byte), patch bool) {
				t.Helper()
				f := steps.MIR2Module.FuncByName(name)
				assertion := hir.Assert{FuncName: name, Args: args, Expected: result, Via: "z80"}
				boot := buildAssertBootstrap(assertLoadAddr, assertion, f, steps.Allocation)
				a, err := z80asm.NewAssembler().AssembleString(boot + "\n" + steps.Assembly)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				platform := emulator.NewRemogattoZ80WithScreen()
				platform.SetROMEnd(0)
				z := platform.RemogattoZ80
				z.SetSMCTracker(nil)
				memory := z.Memory()
				for at := range memory {
					memory[at] = byte(at*37 + at/256*13 + 19)
				}
				if err = z.LoadMemory(assertLoadAddr, a.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetPC(assertLoadAddr)
				for n := 0; z.GetPC() != uint16(a.Symbols[name]); n++ {
					if n > 1000 {
						t.Fatal("bootstrap")
					}
					z.Step()
				}
				sp := z.GetSP()
				pc := uint16(memory[sp]) | uint16(memory[sp+1])<<8
				want := append([]byte(nil), memory...)
				oracle(want)
				allowed := map[int]bool{}
				for _, line := range strings.Split(steps.Assembly, "\n") {
					fields := strings.Fields(line)
					if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
						continue
					}
					symbol := strings.TrimSuffix(fields[0], ":")
					size := 1
					if fields[1] == "DW" {
						size = 2
					}
					for i := 0; i < size; i++ {
						allowed[int(a.Symbols[symbol])+i] = true
					}
				}
				if name == "copy_prepared" || name == "copy_colored" {
					prefix := "bands_"
					names := []string{"sources", "destinations", "widths", "count"}
					if name == "copy_colored" {
						prefix = "colored_"
						names = append(names, "attributes")
					}
					for _, suffix := range names {
						symbol := prefix + suffix
						size := 1
						if suffix == "sources" || suffix == "destinations" || suffix == "attributes" {
							size = 2
						}
						for i := 0; i < size; i++ {
							allowed[int(a.Symbols[symbol])+i] = true
						}
					}
				}
				if patch {
					allowed[int(a.Symbols["Tile_ready"])] = true
					for row := 0; row < 8; row++ {
						slot := int(a.Symbols[fmt.Sprintf("Tile_row_%d$dst$imm", row)])
						allowed[slot] = true
						allowed[slot+1] = true
					}
				}
				minSP := sp
				for n := 0; z.GetPC() != pc || z.GetSP() != sp+2; n++ {
					if n > 20000 {
						t.Fatalf("%s %v: no balanced return", name, args)
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				if minSP < sp-64 {
					t.Fatalf("unbounded stack %04x/%04x", minSP, sp)
				}
				if err = checkAssertZ80Result(z, assertion, f, map[string]*hir.Func{name: h.FuncByName(name)}); err != nil {
					t.Fatal(err)
				}
				for at := range memory {
					if allowed[at] || (at >= int(sp)-64 && at < int(sp)) {
						continue
					}
					if memory[at] != want[at] {
						t.Fatalf("%s %v: memory[%04x]=%02x want%02x", name, args, at, memory[at], want[at])
					}
				}
			}
			for _, entry := range []string{"copy_band", "copy_prepared", "copy_attributes", "copy_colored"} {
				for row := 0; row < 24; row++ {
					for _, span := range [][2]int{{0, 1}, {0, 10}, {11, 10}, {30, 1}, {0, 0}, {0, 32}} {
						left, width := span[0], span[1]
						address := func(base, r, scan int) int {
							y := r*8 + scan
							return base + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + left
						}
						src, dst := address(0x6000, row, 0), address(0x4000, 23-row, 0)
						args := []int64{int64(src), int64(dst), int64(width)}
						attrs := 0x7800 + row*32 + left
						colored := entry == "copy_attributes" || entry == "copy_colored"
						if colored {
							args = append(args, int64(attrs))
						}
						run(entry, args, 1, func(want []byte) {
							if width == 0 || width > 31 {
								return
							}
							for scan := 0; scan < 8; scan++ {
								copy(want[address(0x4000, 23-row, scan):address(0x4000, 23-row, scan)+width], want[address(0x6000, row, scan):address(0x6000, row, scan)+width])
							}
							if colored {
								at := 0x5800 + (23-row)*32 + left
								copy(want[at:at+width], want[attrs:attrs+width])
							}
						}, false)
					}
				}
			}
			for _, base := range []int{0x4000, 0x6000, 0xe801} {
				for _, y := range []int{0, 1, 7, 55, 63, 64, 119, 127, 128, 184, 185} {
					valid := base <= 0xe800 && y <= 184
					result := int64(0)
					if valid {
						result = 1
					}
					run("draw_surface", []int64{112, int64(y), int64(base)}, result, func(want []byte) {
						if !valid {
							return
						}
						screen := make([]byte, 65536)
						copy(screen[0x4000:0x5800], want[base:base+6144])
						wholeSpriteOracle(screen, pixels, 112, y)
						copy(want[base:base+6144], screen[0x4000:0x5800])
					}, true)
				}
			}
		})
	}
}
