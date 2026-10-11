package pipeline

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strings"
	"testing"
)

func TestSignedWideExtensionSourceNative(t *testing.T) {
	// Book v7 §§2.9,3.1,3.2: widening retains signed mathematical value.
	for _, parameter := range []bool{false, true} {
		for _, width := range [][2]int{{8, 16}, {8, 24}, {8, 32}, {16, 24}, {16, 32}, {24, 32}} {
			sw, dw := width[0], width[1]
			t.Run(fmt.Sprintf("%d_%d/parameter%v", sw, dw, parameter), func(t *testing.T) {
				source := fmt.Sprintf("global input:i%d\nglobal result:i%d\nfun embed(){result=i%d(input)}", sw, dw, dw)
				if parameter {
					source = fmt.Sprintf("global input:i%d\nglobal result:i%d\nfun convert(x:i%d)->i%d {return i%d(x)}\nfun embed(){result=convert(input)}", sw, dw, sw, dw, dw)
				}
				parse := func() *hir.Module {
					h, e := nanz.Parse(source, "signed_embedding.nanz")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				raw := mir2.NewVM(hir.LowerModule(parse()))
				steps, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none"})
				if e != nil {
					t.Fatal(e)
				}
				final := mir2.NewVM(steps.MIR2Module)
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil {
					t.Fatal(e)
				}
				if len(bin.Errors) > 0 {
					t.Fatal(bin.Errors, steps.Assembly)
				}
				if dir := os.Getenv("SIGNED_EXT_EXPORT"); dir != "" {
					os.MkdirAll(dir, 0755)
					stem := fmt.Sprintf("%d_%d-parameter%v", sw, dw, parameter)
					for ext, b := range map[string][]byte{"nanz": []byte(source), "a80": []byte(steps.Assembly), "bin": bin.Binary, "mir": []byte(steps.MIR2Module.Dump())} {
						os.WriteFile(dir+"/"+stem+"."+ext, b, 0644)
					}
				}
				symbol := func(n string) int {
					a := bin.Symbols[n]
					if a == 0 {
						a = bin.Symbols["v_"+n]
					}
					if a == 0 {
						t.Fatal(n)
					}
					return a
				}
				vals := []uint32{0, 1, 127, 128, 255, 32767, 32768, 65535, 0x7fffff, 0x800000, 0xffffff}
				if sw == 8 {
					vals = nil
					for v := uint32(0); v < 256; v++ {
						vals = append(vals, v)
					}
				}
				for _, v := range vals {
					if uint64(v) >= uint64(1)<<uint(sw) {
						continue
					}
					var signed int32
					switch sw {
					case 8:
						signed = int32(int8(v))
					case 16:
						signed = int32(int16(v))
					case 24:
						signed = int32(v<<8) >> 8
					}
					want := uint32(signed)
					if dw < 32 {
						want &= (1 << uint(dw)) - 1
					}
					var buf [4]byte
					binary.LittleEndian.PutUint32(buf[:], v)
					for _, vm := range []*mir2.VM{raw, final} {
						vm.WriteHeapSlice(vm.GlobalAddr("input"), buf[:sw/8])
						if _, e := vm.Call("embed", nil); e != nil {
							t.Fatal(e)
						}
						var dst [4]byte
						copy(dst[:], vm.ReadHeap(vm.GlobalAddr("result"), dw/8))
						if got := binary.LittleEndian.Uint32(dst[:]); got != want {
							t.Fatalf("VM v%x got%x want%x", v, got, want)
						}
					}
					z := emulator.NewRemogattoZ80()
					z.LoadMemory(0x8000, bin.Binary)
					z.LoadMemory(uint16(symbol("input")), buf[:sw/8])
					z.LoadMemory(0xeffe, []byte{0, 0xf1})
					z.SetRegisters(emulator.Registers{A: 0x5a, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(symbol("embed")), SP: 0xeffe})
					for n := 0; n < 10000 && z.GetPC() != 0xf100; n++ {
						z.Step()
					}
					var out [4]byte
					copy(out[:], z.Memory()[symbol("result"):symbol("result")+dw/8])
					if got := binary.LittleEndian.Uint32(out[:]); got != want || z.GetPC() != 0xf100 || z.GetSP() != 0xf000 {
						t.Fatalf("native v%x got%x want%x regs%v\n%s", v, got, want, z.GetRegisters(), steps.Assembly)
					}
					for i, original := range bin.Binary {
						addr := bin.Origin + i
						mutable := false
						for name, start := range bin.Symbols {
							if strings.HasPrefix(name, "_spill_") && addr >= start && addr < start+4 {
								mutable = true
							}
						}
						for _, name := range []string{"input", "result"} {
							size := sw / 8
							if name == "result" {
								size = dw / 8
							}
							start := symbol(name)
							if addr >= start && addr < start+size {
								mutable = true
							}
						}
						if !mutable && z.Memory()[addr] != original {
							t.Fatalf("code/literal mutation at %x", addr)
						}
					}

				}
			})
		}
	}
}
