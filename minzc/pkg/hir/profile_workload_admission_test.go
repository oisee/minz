package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func profileDeadFixture(t *testing.T, text string) (*hir.Module, hir.ProfileWorkload) {
	t.Helper()
	m, e := nanz.Parse(text, "profile-dead.nanz")
	if e != nil {
		t.Fatal(e)
	}
	return m, hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Resources: map[string][]byte{}, Calls: []hir.ProfileInvocation{{Function: "entry"}}}
}

const profileDeadSource = `@extern fun disk_read(p:^u8,n:u16)->u8
fun dead()->u8 { return disk_read(0,1) }
fun entry()->u8 { return 173 }
`

func TestProfileUncalledOutputIntrinsicHasNoExecutionAuthority(t *testing.T) {
	m, w := profileDeadFixture(t, `fun entry()->u8 { return 173 }
fun main()->void { @print_u8(7) }`)
	if _, err := hir.RunLoopProfile(m, w); err != nil {
		t.Fatal(err)
	}
	w.Calls = []hir.ProfileInvocation{{Function: "main"}}
	if run, err := hir.RunLoopProfile(m, w); err == nil || run != nil {
		t.Fatal("reachable output host admitted")
	}
	w.Calls = []hir.ProfileInvocation{{Function: "entry"}}
	candidate := hir.LowerModule(m)
	candidate.FuncByName("main").Blocks[0].Insts = append(candidate.FuncByName("main").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCall, Sym: "@mir.io.unknown", Ty: mir2.TyVoid})
	if err := hir.ValidateProfileWorkloadMIR(m, candidate, w); err == nil {
		t.Fatal("unknown uncalled intrinsic admitted")
	}
}

func TestProfileWorkloadDeadExternAdmission(t *testing.T) {
	m, w := profileDeadFixture(t, profileDeadSource)
	raw := hir.LowerModule(m)
	if e := hir.ValidateProfileWorkloadMIR(m, raw, w); e != nil {
		t.Fatal(e)
	}
	r, e := hir.RunLoopProfile(m, w)
	if e != nil || len(r.Results) != 1 || r.Results[0][0].I != 173 {
		t.Fatal(r, e)
	}
	key, e := hir.BuildWorkloadProfileContext(m, w)
	if e != nil {
		t.Fatal(e)
	}
	m.FuncByName("disk_read").RetTy = mir2.TyU16
	next, e := hir.BuildWorkloadProfileContext(m, w)
	if e != nil {
		t.Fatal(e)
	}
	a, e := hir.BuildProfileCompatibility(m, key)
	if e != nil {
		t.Fatal(e)
	}
	b, e := hir.BuildProfileCompatibility(m, next)
	if e != nil {
		t.Fatal(e)
	}
	// Context itself depends on executable/resources; full key retains dead ABI.
	orig, _ := profileDeadFixture(t, profileDeadSource)
	old, e := hir.BuildProfileCompatibility(orig, key)
	if e != nil {
		t.Fatal(e)
	}
	if a.HIRSHA256 == old.HIRSHA256 || b.HIRSHA256 == old.HIRSHA256 {
		t.Fatal("dead callable identity disappeared")
	}
}
func TestProfileWorkloadAdmissionSourceAndCandidateDenials(t *testing.T) {
	for name, body := range map[string]string{"direct": "return disk_read(0,1)", "transitive": "return dead()", "false-block": "if false { return disk_read(0,1) } return 173"} {
		t.Run(name, func(t *testing.T) {
			m, w := profileDeadFixture(t, strings.Replace(profileDeadSource, "return 173", body, 1))
			if r, e := hir.RunLoopProfile(m, w); e == nil || r != nil {
				t.Fatal("admitted source host", r, e)
			}
			// Removing the optimized call does not erase a source obligation.
			plain, _ := profileDeadFixture(t, profileDeadSource)
			if e := hir.ValidateProfileWorkloadMIR(m, hir.LowerModule(plain), w); e == nil {
				t.Fatal("erased source obligation")
			}
		})
	}
	mutations := map[string]func(*mir2.Module){
		"return":    func(m *mir2.Module) { m.FuncByName("disk_read").Contract.Returns[0].Ty = mir2.TyU16 },
		"parameter": func(m *mir2.Module) { m.FuncByName("disk_read").Contract.Params[1].Ty = mir2.TyU8 },
		"class":     func(m *mir2.Module) { m.FuncByName("disk_read").Contract.Params[0].Class = mir2.ClassAcc },
		"clobbers":  func(m *mir2.Module) { m.FuncByName("disk_read").Contract.ExternClobbers = []string{"A"} },
		"address":   func(m *mir2.Module) { m.FuncByName("disk_read").Attrs.ExternAddr = 1234 },
		"body": func(m *mir2.Module) {
			f := m.FuncByName("disk_read")
			f.Attrs.IsExtern = false
			f.Blocks = m.FuncByName("entry").Blocks
		},
		"injected-call": func(m *mir2.Module) {
			m.FuncByName("entry").Blocks[0].Insts = append(m.FuncByName("entry").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCall, Sym: "dead", Ty: mir2.TyU8})
		},
		"undefined": func(m *mir2.Module) {
			m.FuncByName("dead").Blocks[0].Insts = append(m.FuncByName("dead").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCall, Sym: "unknown", Ty: mir2.TyVoid})
		},
		"function-address": func(m *mir2.Module) {
			m.FuncByName("entry").Blocks[0].Insts = append(m.FuncByName("entry").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpAddrOf, Sym: "dead", Ty: mir2.TyPtr})
		},
		"indirect": func(m *mir2.Module) {
			m.FuncByName("dead").Blocks[0].Insts = append(m.FuncByName("dead").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCallIndirect, Ty: mir2.TyVoid})
		},
		"ports": func(m *mir2.Module) {
			m.FuncByName("dead").Blocks[0].Insts = append(m.FuncByName("dead").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpOut8, Ty: mir2.TyVoid})
		},
		"asm": func(m *mir2.Module) { m.FuncByName("dead").Attrs.HasAsm = true },
		"smc": func(m *mir2.Module) {
			m.FuncByName("dead").Blocks[0].Insts = append(m.FuncByName("dead").Blocks[0].Insts, &mir2.Inst{Op: mir2.OpPatch, Ty: mir2.TyVoid})
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			m, w := profileDeadFixture(t, profileDeadSource)
			raw := hir.LowerModule(m)
			mutate(raw)
			if e := hir.ValidateProfileWorkloadMIR(m, raw, w); e == nil {
				t.Fatal("accepted mutation")
			}
		})
	}
	t.Run("genuine-dead-DCE", func(t *testing.T) {
		m, w := profileDeadFixture(t, profileDeadSource)
		raw := hir.LowerModule(m)
		raw.Funcs = []*mir2.Func{raw.FuncByName("entry")}
		if e := hir.ValidateProfileWorkloadMIR(m, raw, w); e != nil {
			t.Fatal(e)
		}
	})
}
