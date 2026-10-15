package pipeline

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// ZX layout is computed from scanline divisions, not from generated addresses.
// The gauge owns one byte column (9), rows 16..175. Eight cleared lines fill
// one character row; its preset INK changes from white through yellow to blue.
func gaugeScreenAddress(y int) int { return 0x4000 + (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + 9 }
func gaugeAttribute(row int) byte {
	colors := []byte{71, 70, 69, 68, 67, 66, 65, 1}
	if row >= 14 {
		return colors[21-row]
	}
	return 1
}

const lineGaugeTestSource = `import zx.line_gauge
 global gauge:LineGauge=LineGauge{height:0,shown:0,count:0}
 global gauge_lines:u8=0
 global gauge_ready:bool=false
 fun gauge_init_probe(){gauge.init()}
 fun gauge_prepare_probe(){gauge.add_lines(gauge_lines) gauge_ready=gauge.stage()}
 fun gauge_commit_probe(){gauge.publish()}
`

func TestLineGaugeOwnedMemoryNativeVM(t *testing.T) {
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(lineGaugeTestSource, "line-gauge.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
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
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(a.Errors) != 0 {
				t.Fatal(e, a.Errors)
			}
			if 0x8000+len(a.Binary) >= 0xfe00 {
				t.Fatal("fixture overlaps stack")
			}
			sym := func(name string) int {
				v, ok := a.Symbols[name]
				if !ok {
					t.Fatal(name)
				}
				return v
			}
			addresses := "zx__line_gauge__LINE_GAUGE_ADDRESSES"
			allowed := map[int]bool{}
			// Compiler spill bytes are scratch, never blanket permission to modify code.
			for _, line := range strings.Split(steps.Assembly, "\n") {
				f := strings.Fields(line)
				if len(f) < 3 || !strings.HasPrefix(f[0], "_spill_") {
					continue
				}
				width := 1
				if f[1] == "DW" {
					width = 2
				} else if f[1] != "DB" {
					t.Fatal(line)
				}
				count := width * len(strings.Split(strings.Join(f[2:], ""), ","))
				base := sym(strings.TrimSuffix(f[0], ":"))
				for i := 0; i < count; i++ {
					allowed[base+i] = true
				}
			}
			raw := hir.LowerModule(parse())
			cases := 0
			for _, initial := range []int{0, 1, 7, 8, 15, 16, 47, 48, 49, 63, 64, 65, 127, 128, 159, 160} {
				for _, lines := range []int{0, 1, 4, 5, 255} {
					height := initial + lines
					if height > 160 {
						height = 160
					}
					count := height - initial
					admitted := count <= 4
					state := []byte{byte(height), byte(initial), 77}
					ready := byte(0)
					prepared := bytes.Repeat([]byte{0xa5}, 8)
					if admitted {
						ready = 1
						state[2] = byte(count)
						for i := 0; i < count; i++ {
							at := gaugeScreenAddress(175 - initial - i)
							prepared[2*i] = byte(at)
							prepared[2*i+1] = byte(at >> 8)
						}
					}
					z := emulator.NewRemogattoZ80()
					ram := z.Memory()
					for at := range ram {
						ram[at] = byte(at*37 + initial*11 + lines)
					}
					copy(ram[0x8000:], a.Binary)
					copy(ram[sym("gauge"):], []byte{byte(initial), byte(initial), 77})
					ram[sym("gauge_lines")] = byte(lines)
					ram[sym("gauge_ready")] = 0
					copy(ram[sym(addresses):], bytes.Repeat([]byte{0xa5}, 8))
					expected := append([]byte(nil), ram...)
					call := func(name string) {
						ram[0xff00], ram[0xff01] = 0x00, 0x70
						z.SetRegisters(emulator.Registers{PC: uint16(sym(name)), SP: 0xff00, A: 0x5a, F: 0x45})
						for n := 0; (z.GetPC() != 0x7000 || z.GetSP() != 0xff02) && n < 100000; n++ {
							z.Step()
							if z.GetSP() < 0xfe00 || z.GetSP() > 0xff02 {
								t.Fatalf("%s stack %04x", name, z.GetSP())
							}
						}
						if z.GetPC() != 0x7000 || z.GetSP() != 0xff02 {
							t.Fatalf("%s PC/SP %04x/%04x", name, z.GetPC(), z.GetSP())
						}
						for at, v := range ram {
							if at >= 0xfe00 && at < 0xff02 || allowed[at] {
								continue
							}
							if v != expected[at] {
								t.Fatalf("initial%d lines%d %s memory %04x got%02x want%02x gauge=%v ready=%d destinations=%v\n%s", initial, lines, name, at, v, expected[at], ram[sym("gauge"):sym("gauge")+3], ram[sym("gauge_ready")], ram[sym(addresses):sym(addresses)+8], steps.Assembly+"\n"+steps.MIR2Opt)
							}
						}
					}
					for row := 2; row < 22; row++ {
						expected[0x5800+row*32+9] = gaugeAttribute(row)
					}
					call("gauge_init_probe")
					copy(expected[sym("gauge"):], state)
					expected[sym("gauge_ready")] = ready
					copy(expected[sym(addresses):], prepared)
					call("gauge_prepare_probe") // All 6912 display bytes must stay unchanged.
					if admitted {
						expected[sym("gauge")+1] = byte(height)
						for i := 0; i < count; i++ {
							expected[gaugeScreenAddress(175-initial-i)] = 255
						}
						call("gauge_commit_probe")
					}
					for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
						vm := mir2.NewVM(module)
						vm.EnsureHeap(65536)
						vm.WriteHeapSlice(0x4000, expected[0x4000:0x5b00])
						// Restore the independent pre-publication bitmap input, keeping attrs.
						for y := 16; y < 176; y++ {
							at := gaugeScreenAddress(y)
							vm.WriteHeap(int64(at), byte(at*37+initial*11+lines))
						}
						vm.WriteHeapSlice(vm.GlobalAddr("gauge"), []byte{byte(initial), byte(initial), 77})
						vm.WriteHeap(vm.GlobalAddr("gauge_lines"), byte(lines))
						vm.WriteHeap(vm.GlobalAddr("gauge_ready"), 0)
						vm.WriteHeapSlice(vm.GlobalAddr(addresses), bytes.Repeat([]byte{0xa5}, 8))
						for _, name := range []string{"gauge_init_probe", "gauge_prepare_probe"} {
							if _, e := vm.Call(name, nil); e != nil {
								t.Fatal(e)
							}
						}
						if got := vm.ReadHeap(vm.GlobalAddr("gauge"), 3); !bytes.Equal(got, state) {
							t.Fatalf("VM state %v want%v", got, state)
						}
						if got := vm.ReadHeap(vm.GlobalAddr("gauge_ready"), 1)[0]; got != ready {
							t.Fatal("VM admission", got, ready)
						}
						if got := vm.ReadHeap(vm.GlobalAddr(addresses), 8); !bytes.Equal(got, prepared) {
							t.Fatalf("VM destinations %v want%v", got, prepared)
						}
						before := vm.ReadHeap(0x4000, 6912)
						for i := 0; i < count && admitted; i++ {
							at := gaugeScreenAddress(175 - initial - i)
							if before[at-0x4000] != byte(at*37+initial*11+lines) {
								t.Fatal("VM stage wrote screen")
							}
						}
						if admitted {
							if _, e := vm.Call("gauge_commit_probe", nil); e != nil {
								t.Fatal(e)
							}
						}
						if got := vm.ReadHeap(0x4000, 6912); !bytes.Equal(got, expected[0x4000:0x5b00]) {
							t.Fatal("VM full screen mismatch")
						}
					}
					cases++
				}
			}
			t.Logf("%d cases: full 64K native memory, raw/final VM screen, admission and PC/SP", cases)
		})
	}
}
