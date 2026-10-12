package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestZ80BoolFlagTruncationPreservesLiveAccumulator(t *testing.T) {
	for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUle, mir2.CmpUgt, mir2.CmpUge} {
		for _, pair := range [][2]byte{{0, 0}, {0, 1}, {1, 0}, {127, 128}, {255, 255}} {
			for _, flagLoc := range []string{"F", "A"} {
				for _, dst := range []string{"C", "E", "H", "IYH", "spill8"} {
					t.Run(fmt.Sprintf("%v_%d_%d_%s_%s", cond, pair[0], pair[1], dst, flagLoc), func(t *testing.T) {
						m := &mir2.Module{}
						f := m.AddFunc("extend")
						b := mir2.NewBuilder(f)
						b.SwitchToNewBlock("entry")
						x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
						y := b.Param("y", mir2.TyU8, mir2.ClassGeneral)
						cmp := b.Cmp(cond, x, y, mir2.ClassFlag, false)
						// Extra pending predicate is consumed through the exact
						// identity-sensitive conversion rather than raw F bits.
						live := b.Const(103, mir2.TyU8, mir2.ClassAcc)
						ty := mir2.TyBool
						if dst == "C" || dst == "spill8" {
							ty = mir2.TyU8
						}
						out := b.Trunc(cmp, mir2.TyBool, ty, mir2.ClassGeneral)
						b.Ret(out, live)
						f.Contract.Returns = []mir2.Return{{Ty: ty, Class: mir2.ClassPointer}, {Ty: mir2.TyU8, Class: mir2.ClassAcc}}
						f.Contract.Returns[0].Class = mir2.ClassRegC
						ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Name: "D"}, y: {Name: "E"}, cmp: {Name: flagLoc}, live: {Name: "A"}, out: {Name: dst}}}
						if dst == "spill8" || dst == "spill16" {
							ar.Locs[out] = mir2.PhysLoc{Kind: mir2.LocMem}
							ar.Spilled = []mir2.Reg{out}
						}
						asm := "ORG 0x8000\nLD SP,0xFF00\n" + fmt.Sprintf("LD D,%d\nLD E,%d\nCALL extend\nDI\nHALT\n", pair[0], pair[1]) + mustZ80Asm(mir2.Z80Codegen(m, ar))
						want := false
						switch cond {
						case mir2.CmpEq:
							want = pair[0] == pair[1]
						case mir2.CmpNe:
							want = pair[0] != pair[1]
						case mir2.CmpUlt:
							want = pair[0] < pair[1]
						case mir2.CmpUle:
							want = pair[0] <= pair[1]
						case mir2.CmpUgt:
							want = pair[0] > pair[1]
						case mir2.CmpUge:
							want = pair[0] >= pair[1]
						}
						w := uint16(0)
						if want {
							w = 1
						}
						r := fix1Execute(t, asm)
						got := r.BC & 255
						if got != w || r.A != 103 {
							t.Fatalf("result%d want%d A%d want103\n%s", got, w, r.A, asm)
						}
					})
				}
			}
		}
	}
}
