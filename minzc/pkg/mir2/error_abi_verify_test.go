package mir2

import (
	"strings"
	"testing"
)

func TestErrorABIVerifyAndSimultaneousOutcomeDefs(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("failed")
	f.Attrs.FallibleCY = true
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	code := b.Const(7, TyU8, ClassGeneral)
	b.ErrorRet(code)
	g := m.AddFunc("caller")
	gb := NewBuilder(g)
	gb.SwitchToNewBlock("entry")
	value, status, errorCode := gb.CallFallible("failed", nil, TyU8, ClassAcc)
	sum := gb.Add(value, errorCode, TyU8, ClassGeneral)
	combined := gb.Add(sum, status, TyU8, ClassAcc)
	gb.Ret(combined)
	g.Contract.Returns = f.Contract.Returns
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	graph := BuildInterferenceGraph(g, ComputeLiveness(g))
	for _, pair := range [][2]Reg{{value, status}, {value, errorCode}, {status, errorCode}} {
		if !graph.Neighbors(pair[0]).Has(pair[1]) {
			t.Fatalf("simultaneous outputs do not interfere %v", pair)
		}
	}
	before := m.Dump()
	if !strings.Contains(before, "outcome") || !strings.Contains(before, "error_ret") {
		t.Fatal(before)
	}
	m.RenumberRegs()
	if err := Verify(m); err != nil {
		t.Fatal("renumber lost outcome", err)
	}
	f.Attrs.FallibleCY = false
	if err := Verify(m); err == nil {
		t.Fatal("untyped language error accepted")
	}
	f.Attrs.FallibleCY = true
	call := g.Entry().Insts[0]
	call.FallibleCY = false
	if err := Verify(m); err == nil {
		t.Fatal("missing error outputs accepted")
	}
	call.FallibleCY = true
	call.ExtraRetTys[0] = TyU16
	if err := Verify(m); err == nil {
		t.Fatal("word CY output accepted")
	}
}

func TestVMErrorOutcomeIsNotTrapAndFreshSuccess(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("failure")
	f.Attrs.FallibleCY = true
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	r := b.Const(9, TyU8, ClassGeneral)
	b.ErrorRet(r)
	g := m.AddFunc("success")
	g.Attrs.FallibleCY = true
	gb := NewBuilder(g)
	gb.SwitchToNewBlock("entry")
	gb.Ret()
	vm := NewVM(m)
	out, err := vm.CallWithOutcome("failure", nil)
	if err != nil || !out.Failed || out.ErrorCode != 9 {
		t.Fatal(out, err)
	}
	out, err = vm.CallWithOutcome("success", nil)
	if err != nil || out.Failed || out.ErrorCode != 0 {
		t.Fatal("stale language error", out, err)
	}
	vm.MaxSteps = 1
	h := m.AddFunc("budget")
	hb := NewBuilder(h)
	hb.SwitchToNewBlock("entry")
	hb.Const(1, TyU8, ClassGeneral)
	hb.Ret()
	_, err = vm.CallWithOutcome("budget", nil)
	if err == nil {
		t.Fatal("trap became language outcome")
	}
}
