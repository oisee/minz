package c89_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

// C99 6.5.6 and 6.5.2.4: pointer steps are in elements, subtraction is
// in elements, and postfix returns the old value. All pointers here stay
// within a single array, and all differences fit the target's signed int.
func TestCPointerArithmeticNativeVM(t *testing.T) {
	types := []struct {
		name, decl, typ string
		stride, base    int
	}{
		{"byte", "unsigned char data[9];", "unsigned char", 1, 4},
		{"word", "unsigned int data[9];", "unsigned int", 2, 4},
		{"long", "unsigned long data[9];", "unsigned long", 4, 4},
		{"struct", "struct Item {unsigned char tag; unsigned int value;}; struct Item data[9];", "struct Item", 3, 4},
		{"word_wide", "unsigned int data[20000];", "unsigned int", 2, 19000},
		{"long_wide", "unsigned long data[10000];", "unsigned long", 4, 9000},
		{"struct_wide", "struct Item {unsigned char tag; unsigned int value;}; struct Item data[12000];", "struct Item", 3, 11000},
	}
	operations := []struct {
		name, body string
		want       func(int, int) int
	}{
		{"add", "p=p+input; result=(char*)p-(char*)data;", func(n, s int) int { return (4 + n) * s }},
		{"commuted_add", "p=input+p; result=(char*)p-(char*)data;", func(n, s int) int { return (4 + n) * s }},
		{"subtract", "p=p-input; result=(char*)p-(char*)data;", func(n, s int) int { return (4 - n) * s }},
		{"compound_add", "p+=input; result=(char*)p-(char*)data;", func(n, s int) int { return (4 + n) * s }},
		{"compound_sub", "p-=input; result=(char*)p-(char*)data;", func(n, s int) int { return (4 - n) * s }},
		{"post_inc", "q=p++; result=((char*)q-(char*)data)*16+((char*)p-(char*)data);", func(n, s int) int { return (4*16 + 5) * s }},
		{"post_dec", "q=p--; result=((char*)q-(char*)data)*16+((char*)p-(char*)data);", func(n, s int) int { return (4*16 + 3) * s }},
		{"pre_inc", "q=++p; result=((char*)q-(char*)data)*16+((char*)p-(char*)data);", func(n, s int) int { return (5*16 + 5) * s }},
		{"pre_dec", "q=--p; result=((char*)q-(char*)data)*16+((char*)p-(char*)data);", func(n, s int) int { return (3*16 + 3) * s }},
		{"difference", "q=&data[4+input]; result=q-p;", func(n, s int) int { return n }},
		{"wide_difference", "q=&data[4+input]; result=q-p;", nil},
		{"wide_reverse_difference", "q=&data[4+input]; result=p-q;", nil},
		{"roundtrip", "q=&*p; result=(char*)q-(char*)data;", func(n, s int) int { return 4 * s }},
		{"field_address", "result=(char*)&p->value-(char*)p;", func(n, s int) int { return 1 }},
	}
	for _, ty := range types {
		for _, op := range operations {
			if (ty.base != 4) != (op.want == nil) || op.name == "field_address" && ty.name != "struct" {
				continue
			}
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(ty.name+"/"+op.name+"/"+mode, func(t *testing.T) {
					source := fmt.Sprintf("%s int input,result; void execute(void){%s *p=&data[%d], *q; %s}", ty.decl, ty.typ, ty.base, op.body)
					parse := func() *hir.Module {
						h, e := c89.Compile(source, "pointers.c")
						if e != nil {
							t.Fatal(e)
						}
						return h
					}
					raw := mir2.NewVM(hir.LowerModule(parse()))
					steps, e := pipeline.CompileHIRSteps(parse(), pipeline.Options{ContractOpt: true, UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size", AssertMode: "none"})
					if e != nil {
						t.Fatal(e)
					}
					final := mir2.NewVM(steps.MIR2Module)
					bin, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + steps.Assembly)
					if e != nil || len(bin.Errors) > 0 {
						t.Fatal(e, bin.Errors)
					}
					symbol := func(n string) int {
						v := bin.Symbols[n]
						if v == 0 {
							v = bin.Symbols["v_"+n]
						}
						if v == 0 {
							t.Fatalf("missing %s", n)
						}
						return v
					}
					z := emulator.NewRemogattoZ80()
					if err := z.LoadMemory(0x4000, bin.Binary); err != nil {
						t.Fatal(err)
					}
					if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
						t.Fatal(err)
					}
					for n := -4; n <= 4; n++ {
						var want uint16
						if op.want != nil {
							want = uint16(op.want(n, ty.stride))
						} else if op.name == "wide_difference" {
							want = uint16(4 + n - ty.base)
						} else {
							want = uint16(ty.base - 4 - n)
						}
						var data [2]byte
						binary.LittleEndian.PutUint16(data[:], uint16(n))
						for _, vm := range []*mir2.VM{raw, final} {
							vm.WriteHeapSlice(vm.GlobalAddr("input"), data[:])
							if _, e := vm.Call("execute", nil); e != nil {
								t.Fatal(e)
							}
							if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != want {
								t.Fatalf("VM input=%d got=%04x want=%04x", n, got, want)
							}
						}
						copy(z.Memory()[symbol("input"):], data[:])
						z.SetRegisters(emulator.Registers{PC: uint16(symbol("execute")), SP: 0xeffe})
						for i := 0; i < 20000 && z.GetRegisters().PC != 0xf100; i++ {
							z.Step()
						}
						r := z.GetRegisters()
						got := binary.LittleEndian.Uint16(z.Memory()[symbol("result") : symbol("result")+2])
						if got != want || r.PC != 0xf100 || r.SP != 0xf000 {
							t.Fatalf("Z80 input=%d got=%04x want=%04x regs=%v\n%s", n, got, want, r, steps.Assembly)
						}
					}
				})
			}
		}
	}
}
