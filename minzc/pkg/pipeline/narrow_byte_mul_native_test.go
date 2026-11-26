package pipeline

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// (unsigned char)(x * k) is computed in bytes when k is a constant, and stays
// a word multiply for a variable factor. Both are checked against the C
// semantics on the VM and on the emulated Z80, exhaustively over the inputs.
func TestNarrowByteMultiplyNative(t *testing.T) {
	type tc struct {
		name, expr string
		variable   bool
		want       func(x, y int) byte
	}
	var cases []tc
	for _, k := range []int{2, 3, 10, 100, 300} {
		k := k
		cases = append(cases, tc{fmt.Sprintf("k%d", k), fmt.Sprintf("(unsigned char)(x * %d)", k), false, func(x, y int) byte { return byte(x * k) }})
	}
	cases = append(cases, tc{"xy", "(unsigned char)(x * y)", true, func(x, y int) byte { return byte(x * y) }})
	for _, c := range cases {
		for _, size := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/size=%v", c.name, size), func(t *testing.T) {
				src := "unsigned char a,b,result; unsigned char calculate(unsigned char x,unsigned char y){return " + c.expr + ";} void execute(void){result=calculate(a,b);}"
				h, err := c89.CompileWithOpts(src, "nmul.c", c89.CompileOpts{NoLibc: true})
				if err != nil {
					t.Fatal(err)
				}
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: true, OptSize: size})
				if err != nil {
					t.Fatal(err)
				}
				byteMul, wordMul := 0, 0
				for _, f := range s.MIR2Module.Funcs {
					if f.Name != "calculate" && f.Name != "execute" {
						continue
					}
					for _, b := range f.Blocks {
						for _, i := range b.Insts {
							if i.Op == mir2.OpMul {
								if i.Ty.Width() == 8 {
									byteMul++
								} else {
									wordMul++
								}
							}
						}
					}
				}
				if c.variable && (byteMul != 0 || wordMul == 0) {
					t.Fatal("variable product must keep its word form", byteMul, wordMul, s.Assembly)
				}
				if !c.variable && wordMul != 0 {
					t.Fatal("constant product was not narrowed", byteMul, wordMul, s.Assembly)
				}
				image, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil || len(image.Errors) > 0 {
					t.Fatal(err, image.Errors)
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
				n := 0
				defer func() { t.Log("iterations", n) }()
				ys := []int{0}
				if c.variable {
					ys = ys[:0]
					for y := 0; y < 256; y++ {
						ys = append(ys, y)
					}
				}
				for x := 0; x < 256; x++ {
					for _, y := range ys {
						want := c.want(x, y)
						n++
						vm.WriteHeapSlice(vm.GlobalAddr("a"), []byte{byte(x)})
						vm.WriteHeapSlice(vm.GlobalAddr("b"), []byte{byte(y)})
						if _, err := vm.Call("execute", nil); err != nil {
							t.Fatal(err)
						}
						if got := vm.ReadHeap(vm.GlobalAddr("result"), 1)[0]; got != want {
							t.Fatal("VM", x, y, got, want)
						}
						mem[a], mem[b], mem[out] = byte(x), byte(y), 0xa5
						mem[0xeffe], mem[0xefff] = 0, 0xf1
						z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe, F: byte(x) ^ byte(y)})
						for step := 0; step < 5000 && z.GetPC() != 0xf100; step++ {
							z.Step()
						}
						r := z.GetRegisters()
						if r.PC != 0xf100 || r.SP != 0xf000 || mem[out] != want || mem[a] != byte(x) || mem[b] != byte(y) {
							t.Fatal("Z80", x, y, mem[out], want, r, s.Assembly)
						}
					}
				}
			})
		}
	}
}
