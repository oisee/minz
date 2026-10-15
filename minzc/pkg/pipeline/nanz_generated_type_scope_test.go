package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestNanzGeneratedCallerTypeNativeVM(t *testing.T) {
	source := `struct Scoped {value:u8}
fun Scoped.read(self:^Scoped)->u8 {return self.value}
fun @borrow()->void {emit(c"fun generated(value:^Scoped)->u8 {return value.read()}")}
@borrow()
fun run()->u8 {let value:Scoped=Scoped{value:7} return generated(&value)}`
	parse := func() *hir.Module {
		h, e := nanz.Parse(source, "generated-types.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		measureGraceZ80HIR(t, parse(), "run", opt, nil, nil, 7)
		steps, e := CompileHIRSteps(parse(), opt)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range []*mir2.Module{hir.LowerModule(parse()), steps.MIR2Module} {
			vm := mir2.NewVM(m)
			result, e := vm.Call("run", nil)
			if e != nil || len(result) != 1 || result[0].I != 7 {
				t.Fatal(result, e)
			}
		}
	}
}
