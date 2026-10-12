package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// Explicit allocations stress the returned AF snapshot, caller AF/BC restores,
// and both callee/caller local frame cleanup without trusting PBQP choices.
func TestFallibleCYNativeCallerFramesAndSavedRegisters(t *testing.T) {
	for _, failed := range []bool{false, true} {
		m := &mir2.Module{}
		f := m.AddFunc("fallible")
		f.Attrs.FallibleCY = true
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		locs := map[mir2.Reg]mir2.PhysLoc{}
		loc := func(r mir2.Reg, n string) { locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: n} }
		p := b.Alloca(3)
		loc(p, "HL")
		v := b.Const(37, mir2.TyU8, mir2.ClassGeneral)
		loc(v, "C")
		if failed {
			b.ErrorRet(v)
		} else {
			b.Ret(v)
		}
		g := m.AddFunc("caller")
		g.Attrs.FallibleCY = true
		g.Contract.Returns = f.Contract.Returns
		gb := mir2.NewBuilder(g)
		gb.SwitchToNewBlock("entry")
		x := gb.Param("x", mir2.TyU8, mir2.ClassAcc)
		loc(x, "A")
		gp := gb.Alloca(5)
		loc(gp, "HL")
		value, status, code := gb.CallFallible("fallible", nil, mir2.TyU8, mir2.ClassGeneral)
		loc(value, "C")
		loc(status, "B")
		loc(code, "D")
		zero := gb.Const(0, mir2.TyBool, mir2.ClassGeneral)
		loc(zero, "E")
		condition := gb.Cmp(mir2.CmpNe, status, zero, mir2.ClassFlag, false)
		locs[condition] = mir2.PhysLoc{Kind: mir2.LocFlag, Name: "F"}
		gb.BrIf(condition, "error", nil, "ok", nil)
		gb.SwitchToNewBlock("error")
		gb.ErrorRet(code)
		gb.SwitchToNewBlock("ok")
		sum := gb.Add(value, x, mir2.TyU8, mir2.ClassAcc)
		loc(sum, "A")
		gb.Ret(sum)
		if err := mir2.Verify(m); err != nil {
			t.Fatal(err)
		}
		asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
		for x := 0; x < 256; x++ {
			want := byte(37 + x)
			carry := byte(0)
			if failed {
				want = 37
				carry = 1
			}
			r := fix1Execute(t, fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nLD A,%d\nSCF\nCALL caller\nDI\nHALT\n", x)+asm)
			if r.A != want || r.F&1 != carry || r.SP != 0xff00 {
				t.Fatalf("failed=%v x=%d actual=%+v want A=%d CY=%d\n%s", failed, x, r, want, carry, asm)
			}
			vm := mir2.NewVM(m)
			out, err := vm.CallWithOutcome("caller", []mir2.Value{{I: int64(x)}})
			if err != nil || out.Failed != failed || failed && out.ErrorCode != 37 || !failed && (len(out.Values) != 1 || out.Values[0].I != int64(want)) {
				t.Fatal("VM outcome", out, err)
			}
		}
	}
}
