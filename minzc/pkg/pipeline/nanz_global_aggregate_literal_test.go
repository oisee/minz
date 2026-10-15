package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestNanzGlobalAggregateLiteralNativeVM(t *testing.T) {
	source := `struct Inner {a:u8,b:u16}
struct Outer {head:u8,inner:Inner,tail:u8}
global layout:Outer=Outer{head:0x11,inner:Inner{a:0x22,b:0xbeef},tail:0x33}
global enabled:bool=true
global source:^u8=ptr(0x4000)
global narrow:u16=u8(300)
global signed:i16=i8(255)
fun run()->u8 {
 let p:^u8=ptr(u16(&layout))
 if p[0]!=0x11 {return 0}
 if p[1]!=0x22 {return 0}
 if p[2]!=0xef {return 0}
 if p[3]!=0xbe {return 0}
 if p[4]!=0x33 {return 0}
 if narrow!=44 {return 0}
 if signed!=i16(-1) {return 0}
 if enabled==false {return 0}
 if source^!=0x93 {return 0}
 return 1
}`
	parse := func() *hir.Module {
		h, e := nanz.Parse(source, "aggregate-literals.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, map[uint16]byte{0x4000: 0x93}, 1)
		steps, e := CompileHIRSteps(parse(), opt)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			vm := mir2.NewVM(m)
			vm.EnsureHeap(0x6000)
			vm.WriteHeapSlice(0x4000, []byte{0x93})
			result, e := vm.Call("run", nil)
			if e != nil || len(result) != 1 || result[0].I != 1 {
				t.Fatal(result, e)
			}
		}
	}
}
