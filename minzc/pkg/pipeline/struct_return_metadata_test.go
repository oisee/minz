package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestStructReturnMetadataRejectsForgedIndirectABI(t *testing.T) {
	const source = `struct S {v:u8}
fun build(x:u8)->S {var r:S=S{v:x}
if x==0 {r.v=0}
return r}
fun invoke(f:fun(u8)->S,x:u8)->u8 {let r:S=f(x)
return r.v}`
	fresh := func() (*mir2.Module, *mir2.Inst) {
		h, e := nanz.Parse(source, "struct-metadata.nanz")
		if e != nil {
			t.Fatal(e)
		}
		m := hir.LowerModule(h)
		if e = mir2.Verify(m); e != nil {
			t.Fatal(e)
		}
		for _, b := range m.FuncByName("invoke").Blocks {
			for _, i := range b.Insts {
				if i.Op == mir2.OpCallIndirect {
					return m, i
				}
			}
		}
		t.Fatal("missing indirect call")
		return nil, nil
	}
	for name, mutate := range map[string]func(*mir2.Module, *mir2.Inst){
		"missing descriptor":         func(m *mir2.Module, i *mir2.Inst) { i.StructReturnBuffers = nil },
		"missing signature":          func(m *mir2.Module, i *mir2.Inst) { i.IndirectParamTys = nil },
		"word is not hidden pointer": func(m *mir2.Module, i *mir2.Inst) { i.IndirectParamTys[len(i.IndirectParamTys)-1] = mir2.TyU16 },
		"wrong return slot":          func(m *mir2.Module, i *mir2.Inst) { i.StructReturnBuffers[0].ReturnIndex = 1 },
		"wrong buffer index":         func(m *mir2.Module, i *mir2.Inst) { i.StructReturnBuffers[0].ParamIndex = 0 },
		"missing function ownership": func(m *mir2.Module, i *mir2.Inst) { m.FuncByName("build").Contract.StructReturnBuffers = nil },
	} {
		t.Run(name, func(t *testing.T) {
			m, i := fresh()
			mutate(m, i)
			if e := mir2.Verify(m); e == nil {
				t.Fatal("forged ABI accepted")
			}
		})
	}
}

func TestStructReturnPublicVMKeepsSourceArguments(t *testing.T) {
	h, e := nanz.Parse(`struct S {v:u16}
fun build(x:u16)->S {var r:S=S{v:x}
if x==0 {r.v=0}
return r}`, "struct-public.nanz")
	if e != nil {
		t.Fatal(e)
	}
	m := hir.LowerModule(h)
	vm := mir2.NewVM(m)
	first, e := vm.Call("build", []mir2.Value{{I: 0x1234}})
	if e != nil {
		t.Fatal(e)
	}
	second, e := vm.Call("build", []mir2.Value{{I: 0xabcd}})
	if e != nil {
		t.Fatal(e)
	}
	if len(first) != 1 || len(second) != 1 || first[0].I == second[0].I {
		t.Fatal("public calls did not obtain distinct storage", first, second)
	}
	if got := vm.ReadHeap(first[0].I, 2); len(got) != 2 || got[0] != 0x34 || got[1] != 0x12 {
		t.Fatal("first result overwritten", got)
	}
	if _, e = vm.Call("build", []mir2.Value{{I: 1}, {I: 2}}); e == nil {
		t.Fatal("physical hidden argument accepted as public source argument")
	}
}
