package pipeline

import (
	"bytes"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Independent Spectrum address formula, all byte values, cell-row/third
// boundaries and both screen/private RAM. Every call must change exactly eight
// bytes. This tests the reusable kernel beyond Tetris's two bitmap values.
func TestZXPixelFillCellNativeVM(t *testing.T) {
	const source = `import zx.pixel_bands
 global fill_address:u16=0
 global fill_value:u8=0
 fun fill_probe() {pixel_band_fill_cell(ptr(fill_address),fill_value)}
 `
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(source, "fill-cell.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	raw := hir.LowerModule(parse())
	for _, mode := range []string{"ordinary", "grace-speed", "grace-size"} {
		t.Run(mode, func(t *testing.T) {
			s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			platform := emulator.NewRemogattoZ80WithScreen()
			platform.SetROMEnd(0)
			z := platform.RemogattoZ80
			z.SetSMCTracker(nil)
			mem := z.Memory()
			for i := range mem {
				mem[i] = byte(i*37 + 13)
			}
			copy(mem[0x8000:], a.Binary)
			vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
			for _, vm := range vms {
				vm.EnsureHeap(65536)
			} // scalar-only kernel: no alloca
			const sp = 0xfce0
			const ret = 0x7000
			calls := 0
			for _, base := range []int{0x4000, 0xe000} {
				for _, cy := range []int{0, 2, 7, 8, 15, 16, 21, 23} {
					for _, cx := range []int{11, 20} {
						for value := 0; value < 256; value++ {
							scan := cy * 8
							address := base + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + cx
							at := a.Symbols["fill_address"]
							mem[at] = byte(address)
							mem[at+1] = byte(address >> 8)
							mem[a.Symbols["fill_value"]] = byte(value)
							mem[sp] = byte(ret & 255)
							mem[sp+1] = byte(ret >> 8)
							want := append([]byte(nil), mem...)
							for row := 0; row < 8; row++ {
								y := scan + row
								p := base + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + cx
								want[p] = byte(value)
							}
							z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["fill_probe"]), SP: sp})
							for n := 0; n < 2000 && !(z.GetPC() == ret && z.GetSP() == sp+2); n++ {
								z.Step()
							}
							if z.GetPC() != ret || z.GetSP() != sp+2 {
								t.Fatal("fill PC/SP")
							}
							for i, b := range want {
								if i >= sp-128 && i < sp {
									continue
								}
								if mem[i] != b {
									t.Fatalf("base%x cell%d,%d value%d native memory%x=%x want%x", base, cx, cy, value, i, mem[i], b)
								}
							}
							for _, vm := range vms {
								vm.WriteHeapSlice(vm.GlobalAddr("fill_address"), []byte{byte(address), byte(address >> 8)})
								vm.WriteHeap(vm.GlobalAddr("fill_value"), byte(value))
								expected := append([]byte(nil), vm.ReadHeap(0, 65536)...)
								for row := 0; row < 8; row++ {
									y := scan + row
									p := base + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + cx
									expected[p] = byte(value)
								}
								if _, e := vm.Call("fill_probe", nil); e != nil {
									t.Fatal(e)
								}
								if !bytes.Equal(vm.ReadHeap(0, 65536), expected) {
									t.Fatalf("VM changed unexpected memory: base%x cell%d,%d value%d", base, cx, cy, value)
								}
							}
							calls++
						}
					}
				}
			}
			t.Logf("%d calls × native/raw VM/final VM: all byte values, screen/shadow, full memory and PC/SP", calls)
		})
	}
}
