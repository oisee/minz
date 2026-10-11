package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestGraceCondRetRejectsFlagClobberingHoistNative(t *testing.T) {
	for _, predicate := range []string{"bit", "zero", "carry"} {
		m := &mir2.Module{}
		f := m.AddFunc("flag_return")
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
		raw := b.Param("raw", mir2.TyU16, mir2.ClassPointer)
		locs := map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "C"}, raw: {Kind: mir2.LocReg, Name: "HL"}}
		lhs := x
		condType := mir2.CmpNe
		rhsValue := int64(0)
		if predicate == "bit" {
			one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
			lhs = b.And(x, one, mir2.TyU8, mir2.ClassGeneral)
			locs[one] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"}
			locs[lhs] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "E"}
		}
		if predicate == "zero" {
			condType = mir2.CmpEq
		}
		if predicate == "carry" {
			condType = mir2.CmpUlt
			rhsValue = 128
		}
		rhs := b.Const(rhsValue, mir2.TyU8, mir2.ClassGeneral)
		locs[rhs] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "B"}
		cond := b.Cmp(condType, lhs, rhs, mir2.ClassFlag, false)
		locs[cond] = mir2.PhysLoc{Kind: mir2.LocFlag, Name: "F"}
		b.BrIf(cond, "yes", nil, "no", nil)
		b.SwitchToNewBlock("yes")
		yes := b.Const(65535, mir2.TyU16, mir2.ClassPointer)
		locs[yes] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
		b.Ret(yes)
		b.SwitchToNewBlock("no")
		mask := b.Const(4095, mir2.TyU16, mir2.ClassPair)
		out := b.And(raw, mask, mir2.TyU16, mir2.ClassPointer)
		locs[mask] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "DE"}
		locs[out] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
		b.Ret(out)
		// The Go and declarative paths must refuse exactly the same unsafe hoist.
		if mir2.CondRetSink(f) {
			t.Fatal("Go pass unexpectedly admitted flag-clobbering hoist")
		}
		if mir2.RunGraceCondRetSink(f) {
			t.Fatalf("Grace admitted %s hoist", predicate)
		}
		asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
		vm := mir2.NewVM(m)
		for value := 0; value < 256; value++ {
			take := value&1 != 0
			if predicate == "zero" {
				take = value == 0
			}
			if predicate == "carry" {
				take = value < 128
			}
			want := uint16(0xabc)
			if take {
				want = 65535
			}
			got, e := vm.CallFunc(f, []mir2.Value{{I: int64(value)}, {I: 0xdabc}})
			if e != nil || got[0].I != int64(want) {
				t.Fatalf("VM %s %d: %v %v", predicate, value, got, e)
			}
			regs := fix1Execute(t, fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD C,%d\nLD HL,0xdabc\nCALL flag_return\nDI\nHALT\n", value)+asm)
			if regs.HL != want || regs.SP != 0xff00 {
				t.Fatalf("native %s %d got%x want%x SP%x\n%s", predicate, value, regs.HL, want, regs.SP, asm)
			}
		}
	}
}
