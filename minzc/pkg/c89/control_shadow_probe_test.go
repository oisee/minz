package c89_test

import (
	c89 "github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"testing"
)

func TestControlContinueShadowProbe(t *testing.T) {
	for _, source := range []string{
		`unsigned int probe(void) { unsigned char i=0,total=0; for(;i<3;i++) { unsigned char i=99; total+=i; continue; } return (unsigned int)i*256+total; }`,
		`unsigned int probe(void) { unsigned char i=0,total=0; do { { unsigned char i=99; total+=i; continue; } } while(i++<2); return (unsigned int)i*256+total; }`,
		`unsigned int probe(void) { unsigned char i=0,total=0; while(i<3) { { unsigned char i=99; total+=i; } i++; } return (unsigned int)i*256+total; }`,
	} {
		h, e := c89.CompileWithOpts(source, "shadow.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		for _, o := range []pipeline.Options{{}, {UseHIRGrace: true, UseGrace: true}, {UseHIRGrace: true, UseGrace: true, OptSize: true}} {
			compiled, err := c89.CompileWithOpts(source, "shadow.c", c89.CompileOpts{NoLibc: true})
			if err != nil {
				t.Fatal(err)
			}
			compiled.Asserts = append(compiled.Asserts, hir.Assert{FuncName: "probe", Expected: 809, Via: "z80"})
			steps, err := pipeline.CompileHIRSteps(compiled, o)
			if err != nil {
				t.Fatal(err)
			}
			final := mir2.NewVM(steps.MIR2Module)
			final.MaxSteps = 10000
			values, err := final.Call("probe", nil)
			if err != nil || len(values) != 1 || values[0].I != 809 {
				t.Fatal("final", values, err)
			}
		}
		vm := mir2.NewVM(hir.LowerModule(h))
		vm.MaxSteps = 10000
		v, e := vm.Call("probe", nil)
		t.Log(source, v, e)
		if e != nil || len(v) != 1 || v[0].I != 809 {
			t.Errorf("got%v err%v want809", v, e)
		}
	}
}
