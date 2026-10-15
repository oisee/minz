package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestZXSpritePreviousEraseNativeVM(t *testing.T) {
	source := `import zx.sprite
@zx_sprite_instances("Mark") {
 instances 2
 size 8 1
 erase
 pixels ".#_#_#_."
}
fun run()->u8 {
 let a:Mark_0=Mark_0{x:0,y:0}
 let b:Mark_1=Mark_1{x:8,y:0}
 if a.stage_previous_erase()!=false {return 0}
 if a.stage()==false {return 0}
 let invalid:Mark_0=Mark_0{x:1,y:0}
 if invalid.stage()!=false {return 0}
 if invalid.stage_previous_erase()!=false {return 0}
 if a.erase_staged()!=false {return 0}
 if a.stage()==false {return 0}
 if b.stage()==false {return 0}
 if a.stage_previous_erase()==false {return 0}
 let moved:Mark_0=Mark_0{x:16,y:0}
 if moved.stage()==false {return 0}
 if a.erase_staged()==false {return 0}
 if moved.publish()==false {return 0}
 if b.publish()==false {return 0}
 if b.stage_previous_erase()==false {return 0}
 if b.erase_staged()==false {return 0}
 let first:^u8=ptr(0x4000)
 let second:^u8=ptr(0x4001)
 let third:^u8=ptr(0x4002)
 if first^!=0x81 {return 0}
 if second^!=0x81 {return 0}
 if third^!=0xd5 {return 0}
 return 1
}`
	parse := func() *hir.Module {
		h, err := nanz.ParseWithOpts(source, "cached-erase.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for _, opt := range []Options{
		{AssertMode: "none", ContractOpt: true},
		{AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true},
		{AssertMode: "none", ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true},
	} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, map[uint16]byte{0x4000: 255, 0x4001: 255, 0x4002: 255}, 1)
		steps, err := CompileHIRSteps(parse(), opt)
		if err != nil {
			t.Fatal(err)
		}
		for _, module := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			vm := mir2.NewVM(module)
			vm.EnsureHeap(0x6000)
			vm.WriteHeapSlice(0x4000, []byte{255, 255, 255})
			result, err := vm.Call("run", nil)
			if err != nil || len(result) != 1 || result[0].I != 1 {
				t.Fatal("cached-erase VM", result, err)
			}
			if got := vm.ReadHeap(0x4000, 3); got[0] != 0x81 || got[1] != 0x81 || got[2] != 0xd5 {
				t.Fatalf("cached-erase masks %x", got)
			}
		}
	}
}
