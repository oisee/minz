package c89_test

import (
	c89 "github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

// C sequence points: only the selected arm executes; postfix yields the old
// value, prefix yields the updated value, before the following statement.
func TestSwitchArmPostfixEffectsVMZ80(t *testing.T) {
	const source = `unsigned int probe(unsigned char sel, unsigned char start) {
 unsigned char n=start; unsigned char x=0;
 switch(sel) {
 case 0: x=n++; break;
 case 1: x=++n; break;
 default: x=n+2; break;
 }
 return (unsigned int)n*256+x;
 }`
	parse := func() *hir.Module {
		h, e := c89.Compile(source, "switch-effects.c")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	want := func(sel, start int64) int64 {
		n, x := start, (start+2)&255
		if sel == 0 {
			n = (start + 1) & 255
			x = start
		}
		if sel == 1 {
			n = (start + 1) & 255
			x = n
		}
		return n*256 + x
	}
	raw := mir2.NewVM(hir.LowerModule(parse()))
	for sel := int64(0); sel < 256; sel++ {
		for _, start := range []int64{0, 1, 127, 128, 254, 255} {
			v, e := raw.Call("probe", []mir2.Value{{I: sel}, {I: start}})
			if e != nil || len(v) != 1 || v[0].I != want(sel, start) {
				t.Fatalf("rawVM sel%d start%d got%v want%d err%v", sel, start, v, want(sel, start), e)
			}
		}
	}
	for _, o := range []pipeline.Options{{ContractOpt: true}, {ContractOpt: true, UseHIRGrace: true, UseGrace: true}, {ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true}} {
		h := parse()
		for _, sel := range []int64{0, 1, 2, 255} {
			for _, start := range []int64{0, 1, 127, 128, 254, 255} {
				h.Asserts = append(h.Asserts, hir.Assert{FuncName: "probe", Args: []int64{sel, start}, Expected: want(sel, start), Via: "z80"})
			}
		}
		steps, e := pipeline.CompileHIRSteps(h, o)
		if e != nil {
			t.Fatal(e)
		}
		final := mir2.NewVM(steps.MIR2Module)
		for sel := int64(0); sel < 256; sel++ {
			for _, start := range []int64{0, 1, 127, 128, 254, 255} {
				v, e := final.Call("probe", []mir2.Value{{I: sel}, {I: start}})
				if e != nil || len(v) != 1 || v[0].I != want(sel, start) {
					t.Fatal("final", sel, start, v, e)
				}
			}
		}
	}
}

func TestSwitchSelectorPostfixAndEmptyEffects(t *testing.T) {
	for _, source := range []string{
		`unsigned char probe(unsigned char n) { switch(n++) {case 0: break; default: break;} return n; }`,
		`unsigned char probe(unsigned char n) { switch(n++) {} return n; }`,
	} {
		parse := func() *hir.Module {
			h, e := c89.Compile(source, "selector.c")
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		raw := mir2.NewVM(hir.LowerModule(parse()))
		for n := int64(0); n < 256; n++ {
			v, e := raw.Call("probe", []mir2.Value{{I: n}})
			if e != nil || len(v) != 1 || v[0].I != (n+1)&255 {
				t.Fatal(n, v, e)
			}
		}
		h := parse()
		for _, n := range []int64{0, 1, 127, 128, 254, 255} {
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: "probe", Args: []int64{n}, Expected: (n + 1) & 255, Via: "z80"})
		}
		if _, e := pipeline.CompileHIRSteps(h, pipeline.Options{ContractOpt: true}); e != nil {
			t.Fatal(e)
		}
	}
}
