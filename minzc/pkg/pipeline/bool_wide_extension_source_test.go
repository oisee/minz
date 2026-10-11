package pipeline

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Book v7 sections 2.9/3: explicit bool widening retains canonical 0 or 1.
// Full output bytes catch a stale high byte that a low-byte return could hide.
func TestBoolWideExtensionSourceNative(t *testing.T) {
	for _, width := range []int{24, 32} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			source := fmt.Sprintf("global input:u8\nglobal result:u%d\nglobal again:u%d\nglobal other:u8\nfun embed(){let b:bool=input==0\nresult=u%d(b)\nother=input+1\nagain=u%d(b)}", width, width, width, width)
			parse := func() *hir.Module {
				h, e := nanz.Parse(source, "bool-wide.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			for _, mode := range []Options{{AssertMode: "none"}, {AssertMode: "none", UseGrace: true}, {AssertMode: "none", UseGrace: true, OptSize: true}} {
				raw := mir2.NewVM(hir.LowerModule(parse()))
				steps, e := CompileHIRSteps(parse(), mode)
				if e != nil {
					t.Fatal(e)
				}
				final := mir2.NewVM(steps.MIR2Module)
				program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil {
					t.Fatal(e)
				}
				if len(program.Errors) > 0 {
					t.Fatal(program.Errors, steps.Assembly)
				}
				if dir := os.Getenv("BOOL_WIDE_EXPORT"); dir != "" {
					stem := fmt.Sprintf("u%d-grace%v-size%v", width, mode.UseGrace, mode.OptSize)
					if e := os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					for ext, data := range map[string][]byte{"nanz": []byte(source), "a80": []byte(steps.Assembly), "bin": program.Binary, "mir": []byte(steps.MIR2Module.Dump())} {
						if e := os.WriteFile(filepath.Join(dir, stem+"."+ext), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
				address := func(name string) int {
					a := program.Symbols[name]
					if a == 0 {
						a = program.Symbols["v_"+name]
					}
					if a == 0 {
						t.Fatal(name)
					}
					return a
				}
				for value := 0; value < 256; value++ {
					input := byte(value)
					want := uint32(0)
					if input == 0 {
						want = 1
					}
					for _, vm := range []*mir2.VM{raw, final} {
						vm.WriteHeapSlice(vm.GlobalAddr("input"), []byte{input})
						if _, e := vm.Call("embed", nil); e != nil {
							t.Fatal(e)
						}
						var b [4]byte
						copy(b[:], vm.ReadHeap(vm.GlobalAddr("result"), width/8))
						if got := binary.LittleEndian.Uint32(b[:]); got != want {
							t.Fatalf("VM input%d got%x want%x", input, got, want)
						}
						b = [4]byte{}
						copy(b[:], vm.ReadHeap(vm.GlobalAddr("again"), width/8))
						if binary.LittleEndian.Uint32(b[:]) != want || vm.ReadHeap(vm.GlobalAddr("other"), 1)[0] != input+1 {
							t.Fatal("VM live predicate", input, b)
						}
					}
					z := emulator.NewRemogattoZ80()
					z.LoadMemory(0x8000, program.Binary)
					z.LoadMemory(uint16(address("input")), []byte{input})
					z.LoadMemory(0xeffe, []byte{0, 0xf1})
					before := append([]byte(nil), z.Memory()...)
					z.SetRegisters(emulator.Registers{A: 0x5a, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: uint16(address("embed")), SP: 0xeffe})
					for n := 0; n < 10000 && z.GetPC() != 0xf100; n++ {
						z.Step()
					}
					var b [4]byte
					copy(b[:], z.Memory()[address("result"):address("result")+width/8])
					if got := binary.LittleEndian.Uint32(b[:]); got != want || z.GetPC() != 0xf100 || z.GetSP() != 0xf000 {
						t.Fatalf("native input%d got%x want%x registers%v\n%s", input, got, want, z.GetRegisters(), steps.Assembly)
					}
					b = [4]byte{}
					copy(b[:], z.Memory()[address("again"):address("again")+width/8])
					if binary.LittleEndian.Uint32(b[:]) != want || z.Memory()[address("other")] != input+1 {
						t.Fatalf("native live predicate input%d again%x\n%s", input, b, steps.Assembly)
					}
					mutable := map[int]bool{}
					for _, name := range []string{"result", "again", "other"} {
						size := width / 8
						if name == "other" {
							size = 1
						}
						for i := 0; i < size; i++ {
							mutable[address(name)+i] = true
						}
					}
					for _, f := range steps.MIR2Module.Funcs {
						for _, inst := range f.Blocks {
							for _, op := range inst.Insts {
								if op.Dst == mir2.NoReg {
									continue
								}
								label := mir2.Z80SpillLabel(f.Name, op.Dst)
								if a, ok := program.Symbols[label]; ok {
									size := mir2.ByteWidth(op.Ty)
									for i := 0; i < size; i++ {
										mutable[a+i] = true
									}
								}
							}
						}
					}
					for a, v := range before {
						if z.Memory()[a] != v && !mutable[a] && !(a >= 0xefc0 && a < 0xeffe) {
							t.Fatalf("unexpected RAM write %04x input%d", a, input)
						}
					}

				}
			}
		})
	}
}
