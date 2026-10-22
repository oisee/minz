package pipeline

import (
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
	"testing"
)

func TestSingleCallSiteInlineVMZ80(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		var h *hir.Module
		var err error
		if language == "c" {
			h, err = c89.CompileWithOpts(`static unsigned short helper(unsigned char x) { if(x<128) return ((unsigned short)x<<8)+x; return 65535-x; } unsigned short wrapper(unsigned char x){return helper(x)+1;}`, "single.c", c89.CompileOpts{NoLibc: true})
		} else {
			h = graceMeasureParse(t, `fun helper(x:u8)->u16 { if x<128 { return ((x as u16)<<8)+(x as u16) } return 65535-(x as u16) } fun wrapper(x:u8)->u16 { return helper(x)+1 }`)
			h.FuncByName("helper").Internal = true
		}
		if err != nil {
			t.Fatal(err)
		}
		raw := hir.LowerModule(h)
		s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(s.MIR2Module.Dump(), "call @helper") || s.MIR2Module.FuncByName("helper") != nil {
			t.Fatalf("private single call was not inlined:\n%s", s.MIR2Module.Dump())
		}
		vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
		for x := int64(0); x < 256; x++ {
			want := int64(65536-x) & 65535
			if x < 128 {
				want = (x << 8) + x + 1
			}
			for _, vm := range vms {
				got, err := vm.Call("wrapper", []mir2.Value{{I: x}})
				if err != nil || len(got) != 1 || got[0].I != want {
					t.Fatalf("%s x%d VM%v %v want%d", language, x, got, err, want)
				}
			}
			measureGraceZ80Compiled(t, h, s, "wrapper", []int64{x}, nil, want, 1000)
		}
	}
}

func TestSingleCallExportCopiesVMZ80(t *testing.T) {
	h := graceMeasureParse(t, `global a:u8 global b:u8
 fun left()->u8 {return (a+1)&(b+1)}
 fun right()->u8 {return (a+1)|(b+1)}
 fun combined(x:u8,y:u8)->u16 {a=x b=y let lo:u8=left() let hi:u8=right() return (lo as u16)|((hi as u16)<<8)}`)
	raw := hir.LowerModule(h)
	candidate := hir.LowerModule(h)
	mir2.InlineSingleCallSites(candidate, functionPruneRoots(h))
	// Compile the transformed MIR directly to exercise retained copies even
	// when the production byte-cost gate chooses the original call shape.
	s, err := compileMIRSteps(h, Options{ContractOpt: true, AssertMode: "none", disableExportCopies: true}, Steps{}, nil, candidate)
	if err != nil {
		t.Fatal(err)
	}
	loads := 0
	for _, b := range s.MIR2Module.FuncByName("combined").Blocks {
		for _, i := range b.Insts {
			if i.Op == mir2.OpLoad {
				loads++
			}
			if i.Op == mir2.OpCall {
				t.Fatalf("call survived: %s", s.MIR2Module.Dump())
			}
		}
	}
	if loads != 2 || s.MIR2Module.FuncByName("left") == nil || s.MIR2Module.FuncByName("right") == nil {
		t.Fatalf("exports or shared reads lost: loads=%d\n%s", loads, s.MIR2Module.Dump())
	}
	for _, x := range []int64{0, 1, 127, 128, 254, 255} {
		for _, y := range []int64{0, 1, 127, 128, 254, 255} {
			a, b := int64(byte(x+1)), int64(byte(y+1))
			want := (a & b) | (a|b)<<8
			for _, m := range []*mir2.Module{raw, s.MIR2Module} {
				v, e := mir2.NewVM(m).Call("combined", []mir2.Value{{I: x}, {I: y}})
				if e != nil || len(v) != 1 || v[0].I != want {
					t.Fatal(v, e, want)
				}
			}
			measureGraceZ80Compiled(t, h, s, "combined", []int64{x, y}, nil, want, 1000)
		}
	}
	for _, name := range []string{"left", "right"} {
		measureGraceZ80Compiled(t, h, s, name, nil, nil, 1, 1000)
	}
}
