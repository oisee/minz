package c89_test

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// C99 6.2.4 and 6.5.3.2: automatic objects retain identity while their
// address is in use. Callee stores and direct assignments observe the same
// object, including parameters, nested scopes and recursive calls.
func TestCLocalAddressesNativeVM(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32) uint32
	}{
		{"callee_write", "callee_write()", func(v uint32) uint32 { return uint32(uint16(v + 3)) }},
		{"direct_write", "direct_write()", func(v uint32) uint32 { return uint32(uint16(v + 3)) }},
		{"parameter", "parameter_cell((unsigned int)input)", func(v uint32) uint32 { return uint32(uint16(v + 3)) }},
		{"long_cell", "long_cell()", func(v uint32) uint32 { return v + 1 }},
		{"pointer_cell", "pointer_cell()", func(v uint32) uint32 { return 1 }},
		{"shadow", "shadow_cell()", func(v uint32) uint32 { return 10 }},
		{"loop", "loop_cell()", func(v uint32) uint32 { return 12 }},
		{"recursive", "recursive_cell((unsigned int)(input%5))", func(v uint32) uint32 { n := v % 5; return n * (n + 1) / 2 }},
	}
	const prefix = `unsigned long input,result;
void write16(unsigned int *p,unsigned int n){*p=n+3;}
void write32(unsigned long *p,unsigned long n){*p=n+1;}
unsigned int callee_write(void){unsigned int value=5;write16(&value,(unsigned int)input);return value;}
unsigned int direct_write(void){unsigned int value=(unsigned int)input;unsigned int *p=&value;value+=3;return *p;}
unsigned int parameter_cell(unsigned int n){write16(&n,n);return n;}
unsigned long long_cell(void){unsigned long value=0;write32(&value,input);return value;}
unsigned int pointer_cell(void){unsigned int value=7;unsigned int *p=&value;unsigned int **pp=&p;*pp=0;return p==0;}
unsigned int shadow_cell(void){unsigned int value=10;unsigned int *p=&value;{unsigned int value=7;write16(&value,1);}return *p;}
unsigned int loop_cell(void){unsigned int i=0,sum=0;while(i<3){unsigned int value=0;write16(&value,i);sum+=value;i++;}return sum;}
unsigned int recursive_cell(unsigned int n){unsigned int value=n;unsigned int *p=&value;if(n){unsigned int child=recursive_cell(n-1);return child+*p;}return *p;}
`
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x7fffffff, 0x80000000, 0xffffffff}
	for _, c := range cases {
		for _, mode := range []string{"default", "grace", "size"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				parse := func() *hir.Module {
					h, e := c89.Compile(prefix+fmt.Sprintf("void execute(void){result=%s;}", c.expr), "scalar.c")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				raw := mir2.NewVM(hir.LowerModule(parse()))
				steps, e := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
				if e != nil {
					t.Fatal(e)
				}
				final := mir2.NewVM(steps.MIR2Module)
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil {
					t.Fatal(e)
				}
				if len(bin.Errors) > 0 {
					t.Fatal(bin.Errors)
				}
				symbol := func(n string) int {
					p := bin.Symbols[n]
					if p == 0 {
						p = bin.Symbols["v_"+n]
					}
					if p == 0 {
						t.Fatalf("missing %s", n)
					}
					return p
				}
				z := emulator.NewRemogattoZ80()
				if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
					t.Fatal(err)
				}
				if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
					t.Fatal(err)
				}
				for _, v := range values {
					want := c.want(v)
					var data [4]byte
					binary.LittleEndian.PutUint32(data[:], v)
					for _, vm := range []*mir2.VM{raw, final} {
						vm.WriteHeapSlice(vm.GlobalAddr("input"), data[:])
						if _, e := vm.Call("execute", nil); e != nil {
							t.Fatal(e)
						}
						if got := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("result"), 4)); got != want {
							t.Fatalf("VM input=%08x got=%08x want=%08x", v, got, want)
						}
					}
					copy(z.Memory()[symbol("input"):], data[:])
					z.SetRegisters(emulator.Registers{PC: uint16(symbol("execute")), SP: 0xeffe})
					for n := 0; n < 20000 && z.GetRegisters().PC != 0xf100; n++ {
						z.Step()
					}
					r := z.GetRegisters()
					got := binary.LittleEndian.Uint32(z.Memory()[symbol("result") : symbol("result")+4])
					if got != want || r.PC != 0xf100 || r.SP != 0xf000 {
						t.Fatalf("Z80 input=%08x got=%08x want=%08x regs=%v\n%s", v, got, want, r, steps.Assembly)
					}
				}
			})
		}
	}
}
