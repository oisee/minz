package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// Encodability is decided before skipping AND/zero, so ordinary fallback has
// real values. A live word source and hostile stale mask destination remain.
func TestBitCmpFallbackNativeIndexedAndSpill(t *testing.T) {
	for _, home := range []string{"IX", "IY", "spill"} {
		for _, shift := range []bool{false, true} {
			for value := 0; value < 256; value++ {
				m := &mir2.Module{}
				f := m.AddFunc("testbit")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				x := b.Param("x", mir2.TyU16, mir2.ClassPair)
				r := x
				locations := map[mir2.Reg]mir2.PhysLoc{}
				if shift {
					k := b.Const(8, mir2.TyU16, mir2.ClassPair)
					r = b.Shr(x, k, mir2.TyU16, mir2.ClassPair)
					locations[k] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "BC"}
					locations[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "DE"}
				}
				one := b.Const(1, mir2.TyU16, mir2.ClassPair)
				masked := b.And(r, one, mir2.TyU16, mir2.ClassPair)
				zero := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
				cond := b.Cmp(mir2.CmpNe, masked, zero, mir2.ClassFlag, false)
				b.BrIf(cond, "yes", nil, "no", nil)
				b.SwitchToNewBlock("yes")
				y := b.Const(1, mir2.TyU8, mir2.ClassAcc)
				b.Ret(y)
				b.SwitchToNewBlock("no")
				n := b.Const(0, mir2.TyU8, mir2.ClassAcc)
				b.Ret(n)
				xLoc := mir2.PhysLoc{Kind: mir2.LocIXY, Name: home}
				init := fmt.Sprintf("LD %s,%d\n", home, value)
				arg := value
				if shift {
					arg = value << 8
					init = fmt.Sprintf("LD %s,%d\n", home, arg)
				}
				if home == "spill" {
					xLoc = mir2.PhysLoc{Kind: mir2.LocMem, Name: "$F000", Offset: 0xf000}
					init = fmt.Sprintf("LD HL,%d\nLD (_spill_testbit_r%d),HL\n", arg, x)
				}
				locations[x] = xLoc
				locations[one] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "BC"}
				locations[masked] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "HL"}
				locations[zero] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				locations[cond] = mir2.PhysLoc{Kind: mir2.LocFlag, Name: "F"}
				locations[y] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				locations[n] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
				asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locations}))
				regs := fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\n"+init+"LD HL,1\nLD A,1\nCALL testbit\nDI\nHALT\n"+asm)
				if regs.A != byte(value&1) {
					t.Fatalf("home%s shifted%v input%d got%d want%d\n%s", home, shift, value, regs.A, value&1, asm)
				}
			}
		}
	}
}
