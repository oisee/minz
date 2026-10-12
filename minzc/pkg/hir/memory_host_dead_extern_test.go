package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"testing"
)

func TestMemoryHostDeadExternSourceAndCandidateAdmission(t *testing.T) {
	const suffix = `@extern fun unused(x:u8)->u16
fun dead()->u16 {return unused(3)}
`
	fixture := func(extra string) (*hir.Module, hir.MemoryHostWorkload) {
		_, w := memoryFixture(t)
		h, e := nanz.Parse(memorySrc+suffix+extra, "dead-extern.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h, w
	}
	h, w := fixture("")
	run, e := hir.RunMemoryHostBaseline(h, w)
	if e != nil || len(run.Invocations) != 2 || run.Invocations[1].Results[0].I != 255 {
		t.Fatal(run, e)
	}
	_, ctx, e := hir.PrepareMemoryHostWorkload(h, w)
	if e != nil {
		t.Fatal(e)
	}
	// A dead declaration remains part of full-source compatibility even though
	// its workload never invokes it. Do not hash a pruned source.
	changed, _ := fixture("")
	changed.FuncByName("unused").RetTy = mir2.TyU8
	_, ctx2, e := hir.PrepareMemoryHostWorkload(changed, w)
	compat, e1 := hir.BuildProfileCompatibility(h, ctx)
	compat2, e2 := hir.BuildProfileCompatibility(changed, ctx2)
	if e != nil || e1 != nil || e2 != nil || reflect.DeepEqual(compat, compat2) {
		t.Fatal("dead signature missing from source identity", e)
	}
	for name, entry := range map[string]string{"direct": "dead", "transitive": "via", "false_branch": "false_call"} {
		t.Run(name, func(t *testing.T) {
			h, w := fixture("\nfun via()->u16 {return dead()}\nfun false_call()->u16 {if false {return unused(4)}\n return 1}\n")
			w.Calls = []hir.MemoryHostInvocation{{Function: entry}}
			if _, _, e := hir.PrepareMemoryHostWorkload(h, w); e == nil {
				t.Fatal("reachable unbound extern admitted")
			}
		})
	}
	for name, mutate := range map[string]func(*mir2.Module){
		"extern_body": func(m *mir2.Module) {
			m.FuncByName("unused").Blocks = []*mir2.Block{{Label: "entry", Term: &mir2.TermRet{}}}
		},
		"extern_to_definition": func(m *mir2.Module) { m.FuncByName("unused").Attrs.IsExtern = false },
		"parameter":            func(m *mir2.Module) { m.FuncByName("unused").Contract.Params[0].Ty = mir2.TyU16 },
		"return":               func(m *mir2.Module) { m.FuncByName("unused").Contract.Returns[0].Ty = mir2.TyU8 },
		"address":              func(m *mir2.Module) { m.FuncByName("unused").Attrs.ExternAddr = 0x10 },
		"clobber":              func(m *mir2.Module) { m.FuncByName("unused").Contract.ExternClobbers = []string{"A"} },
		"call": func(m *mir2.Module) {
			m.FuncByName("step").Blocks[0].Insts = append(m.FuncByName("step").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCall, Sym: "unused", Ty: mir2.TyU16})
		},
		"indirect": func(m *mir2.Module) {
			m.FuncByName("dead").Blocks[0].Insts = append(m.FuncByName("dead").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCallIndirect, Ty: mir2.TyVoid})
		},
		"address_escape": func(m *mir2.Module) {
			m.FuncByName("dead").Blocks[0].Insts = append(m.FuncByName("dead").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpAddrOf, Sym: "unused", Ty: mir2.TyPtr})
		},
		"unknown_dead_call": func(m *mir2.Module) {
			m.FuncByName("dead").Blocks[0].Insts = append(m.FuncByName("dead").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCall, Sym: "unknown", Ty: mir2.TyVoid})
		},
	} {
		t.Run(name, func(t *testing.T) {
			h, w := fixture("")
			m := hir.LowerModule(h)
			mutate(m)
			if e := hir.ValidateMemoryHostMIR(h, m, w); e == nil {
				t.Fatal("changed candidate admitted")
			}
		})
	}
	// Retained source obligations are not removed by a candidate deleting a call.
	h, w = fixture("")
	w.Calls = []hir.MemoryHostInvocation{{Function: "dead"}}
	m := hir.LowerModule(h)
	m.FuncByName("dead").Blocks[0].Insts = nil
	if e := hir.ValidateMemoryHostMIR(h, m, w); e == nil {
		t.Fatal("candidate erased source host obligation")
	}
	// Conversely, dropping a genuinely dead declaration is legal DCE.
	h, w = fixture("")
	m = hir.LowerModule(h)
	fs := m.Funcs[:0]
	for _, f := range m.Funcs {
		if f.Name != "unused" && f.Name != "dead" {
			fs = append(fs, f)
		}
	}
	m.Funcs = fs
	if e := hir.ValidateMemoryHostMIR(h, m, w); e != nil {
		t.Fatal("dead declaration DCE", e)
	}
}
