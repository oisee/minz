package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80spec"
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

func TestStructReturnIndirectTargetRejectsDifferentContract(t *testing.T) {
	h, e := nanz.Parse(`struct S {v:u8}
fun build(x:u8)->S {var r:S=S{v:x}
if x==0 {r.v=0}
return r}
fun invoke(f:fun(u8)->S,x:u8)->u8 {let r:S=f(x)
return r.v}`, "target-contract.nanz")
	if e != nil {
		t.Fatal(e)
	}
	m := hir.LowerModule(h)
	target := m.FuncByName("build")
	var call *mir2.Inst
	for _, b := range m.FuncByName("invoke").Blocks {
		for _, i := range b.Insts {
			if i.Op == mir2.OpCallIndirect {
				call = i
			}
		}
	}
	if e := mir2.ValidateIndirectTargetABI(call, target); e != nil {
		t.Fatal(e)
	}
	target.Attrs.FallibleCY = true
	if e := mir2.ValidateIndirectTargetABI(call, target); e == nil {
		t.Fatal("same-width fallible target accepted")
	}
	target.Attrs.FallibleCY = false
	target.Contract.Returns[0].Ty = &mir2.StructTy{Name: "Other", Fields: []mir2.StructField{{Name: "q", Ty: mir2.TyU8}}}
	if e := mir2.ValidateIndirectTargetABI(call, target); e == nil {
		t.Fatal("same-size different-layout target accepted")
	}
}

func TestStructReturnCanonicalIndirectSlotsDoNotOverlap(t *testing.T) {
	for _, tys := range [][]mir2.Ty{{mir2.TyU16, mir2.TyPtr}, {mir2.TyPtr, mir2.TyPtr}, {mir2.TyU8, mir2.TyPtr, mir2.TyU16}, {mir2.TyPtr, mir2.TyPtr, mir2.TyPtr}, {mir2.TyPtr, mir2.TyPtr, mir2.TyPtr, mir2.TyPtr}} {
		classes, e := mir2.CanonicalIndirectParamClasses(tys)
		if e != nil {
			t.Fatal(e)
		}
		var occupied []string
		for i, cls := range classes {
			loc := mir2.ReturnLocation(cls, tys[i])
			for _, previous := range occupied {
				if z80spec.Overlaps(previous, loc) {
					t.Fatal("overlapping ABI", tys, classes)
				}
			}
			occupied = append(occupied, loc)
		}
	}
	if _, e := mir2.CanonicalIndirectParamClasses([]mir2.Ty{mir2.TyPtr, mir2.TyPtr, mir2.TyPtr, mir2.TyPtr, mir2.TyPtr}); e == nil {
		t.Fatal("exhausted ABI accepted")
	}
}

func TestStructReturnCanonicalReturnSlotsDoNotOverlap(t *testing.T) {
	for _, tys := range [][]mir2.Ty{{mir2.TyPtr, mir2.TyPtr}, {mir2.TyU8, mir2.TyU8, mir2.TyU8, mir2.TyU8}, {mir2.TyU16, mir2.TyU16, mir2.TyU16}} {
		classes, e := mir2.CanonicalIndirectReturnClasses(tys)
		if e != nil {
			t.Fatal(e)
		}
		occupied := map[byte]bool{}
		for i, cls := range classes {
			loc := mir2.ReturnLocation(cls, tys[i])
			for j := range loc {
				if occupied[loc[j]] {
					t.Fatal("overlapping return ABI", tys, classes)
				}
				occupied[loc[j]] = true
			}
		}
	}
}
