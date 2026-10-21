package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestDemandedBytePromotionNative(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(byte, byte) byte
	}{
		{"shift-xor", "(x << 1) ^ 7", func(x, y byte) byte { return (x << 1) ^ 7 }},
		{"add-and", "(x+1) & (y+1)", func(x, y byte) byte { return (x + 1) & (y + 1) }},
		{"right-shift", "x >> 1", func(x, y byte) byte { return x >> 1 }},
	}
	for _, tc := range cases {
		for _, mode := range []string{"default", "size", "grace"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				h, err := c89.CompileWithOpts("unsigned char a,b,result; unsigned char calculate(unsigned char x,unsigned char y){return "+tc.expr+";} void execute(void){result=calculate(a,b);}", "promotion.c", c89.CompileOpts{NoLibc: true})
				if err != nil {
					t.Fatal(err)
				}
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if err != nil {
					t.Fatal(err)
				}
				byteOps := 0
				for _, f := range s.MIR2Module.Funcs {
					if f.Name == "calculate" || f.Name == "execute" {
						for _, b := range f.Blocks {
							for _, i := range b.Insts {
								switch i.Op {
								case mir2.OpAdd, mir2.OpAnd, mir2.OpXor, mir2.OpShl, mir2.OpShr, mir2.OpSar:
									byteOps++
									if i.Ty.Width() != 8 {
										t.Fatal("word arithmetic remains", i, s.Assembly)
									}
								}
							}
						}
					}
				}
				if byteOps == 0 {
					t.Fatal("byte arithmetic disappeared from the inspected functions")
				}
				image, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil {
					t.Fatal(err)
				}
				if len(image.Errors) > 0 {
					t.Fatal(image.Errors)
				}
				addr := func(name string) int {
					p := image.Symbols[name]
					if p == 0 {
						p = image.Symbols["v_"+name]
					}
					if p == 0 {
						t.Fatal(name)
					}
					return p
				}
				a, b, out, entry := addr("a"), addr("b"), addr("result"), addr("execute")
				vm := mir2.NewVM(s.MIR2Module)
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], image.Binary)
				for x := 0; x < 256; x++ {
					for _, y := range []byte{0, 1, 127, 128, 254, 255} {
						want := tc.want(byte(x), y)
						vm.WriteHeapSlice(vm.GlobalAddr("a"), []byte{byte(x)})
						vm.WriteHeapSlice(vm.GlobalAddr("b"), []byte{y})
						if _, err := vm.Call("execute", nil); err != nil {
							t.Fatal(err)
						}
						if got := vm.ReadHeap(vm.GlobalAddr("result"), 1)[0]; got != want {
							t.Fatal("VM", x, y, got, want)
						}
						mem[a], mem[b], mem[out] = byte(x), y, 0xa5
						mem[0xeffe], mem[0xefff] = 0, 0xf1
						z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe, F: byte(x) ^ y})
						for step := 0; step < 1000 && z.GetPC() != 0xf100; step++ {
							z.Step()
						}
						r := z.GetRegisters()
						if r.PC != 0xf100 || r.SP != 0xf000 || mem[out] != want || mem[a] != byte(x) || mem[b] != y {
							t.Fatal("Z80", x, y, mem[out], want, r, s.Assembly)
						}
					}
				}
			})
		}
	}
}
