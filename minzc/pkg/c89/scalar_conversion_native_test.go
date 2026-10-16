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
	"strings"
	"testing"
)

// C99 6.3.1.2, 6.5.16 and 6.8.6.4: conversion to _Bool tests the whole
// scalar, rather than its low byte. Static objects preserve their full ABI
// storage width and initializer, including arrays and struct fields.
func TestCScalarConversionsNativeVM(t *testing.T) {
	cases := []struct {
		name, expr string
		want       func(uint32) uint32
	}{
		{"cast", "(_Bool)input", func(v uint32) uint32 {
			if v != 0 {
				return 1
			}
			return 0
		}},
		{"assign", "assign_bool()", func(v uint32) uint32 {
			if v != 0 {
				return 1
			}
			return 0
		}},
		{"argument", "bool_arg(input)", func(v uint32) uint32 {
			if v != 0 {
				return 1
			}
			return 0
		}},
		{"return", "bool_return(input)", func(v uint32) uint32 {
			if v != 0 {
				return 1
			}
			return 0
		}},
		{"compound", "compound_bool()", func(v uint32) uint32 {
			if v+1 != 0 {
				return 1
			}
			return 0
		}},
		{"increment", "increment_bool()", func(v uint32) uint32 { return 17 }},
		{"struct_init", "struct_bool()", func(v uint32) uint32 {
			if v != 0 {
				return 23
			}
			return 7
		}},
		{"array_init", "array_bool()", func(v uint32) uint32 {
			if v != 0 {
				return 16
			}
			return 0
		}},
		{"static_long", "static_long()", func(v uint32) uint32 { return 0x12345678 }},
		{"static_struct", "static_struct()", func(v uint32) uint32 { return 515 }},
		{"static_array", "static_array()", func(v uint32) uint32 { return 0xabcd }},
		{"static_bool", "static_bool()", func(v uint32) uint32 { return 1 }},
	}
	const prefix = `unsigned long input,result;
unsigned int bool_arg(_Bool b){return b;}
_Bool bool_return(unsigned long n){return n;}
unsigned int assign_bool(void){_Bool b=0;b=input;return b;}
unsigned int compound_bool(void){_Bool b=1;b+=input;return b;}
unsigned int increment_bool(void){_Bool b=1;unsigned int old=b++;return old*16+b;}
struct Flag{_Bool value;unsigned char marker;};
unsigned int struct_bool(void){struct Flag f={input,7};return f.value*16+f.marker;}
unsigned int array_bool(void){_Bool v[2]={input,0};return v[0]*16+v[1];}
unsigned long static_long(void){static unsigned long n=0x12345678UL;return n;}
struct Pair{unsigned char tag;unsigned int value;};
unsigned int static_struct(void){static struct Pair p={2,513};return p.tag+p.value;}
unsigned int static_array(void){static unsigned int values[3]={0x1234,0xabcd,7};return values[1];}
unsigned int static_bool(void){static _Bool b=256;return b;}
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
				z.LoadMemory(0x8000, bin.Binary)
				z.LoadMemory(0xeffe, []byte{0, 0xf1})
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

func TestCRejectsUnsupportedFloatingTypes(t *testing.T) {
	for _, src := range []string{"float data;", "double f(double x){return x;}", "long double data;", "int f(void){return (int)1.0f;}"} {
		if _, e := c89.Compile(src, "float.c"); e == nil || (!strings.Contains(e.Error(), "unsupported Z80 type") && !strings.Contains(e.Error(), "floating-point")) {
			t.Fatalf("expected explicit unsupported float diagnostic, got %v", e)
		}
	}
}
