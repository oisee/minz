package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// A runtime byte in H/L must survive loading the word operand into HL.
// This allocation occurred in the native TRD directory open loop (<128).
func TestNativeWordCompareByteInHL(t *testing.T) {
	for _, byteReg := range []string{"H", "L"} {
		for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUle, mir2.CmpUgt, mir2.CmpUge} {
			for _, reverse := range []bool{false, true} {
				m := &mir2.Module{Name: "mixed"}
				f := m.AddFunc("arith")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				w := b.Param("word", mir2.TyU16, mir2.ClassPair)
				v := b.Param("byte", mir2.TyU8, mir2.ClassGeneral)
				lhs, rhs := w, v
				if reverse {
					lhs, rhs = v, w
				}
				c := b.Cmp(cond, lhs, rhs, mir2.ClassFlag, false)
				b.BrIf(c, "yes", nil, "no", nil)
				b.SwitchToNewBlock("yes")
				one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
				b.Ret(one)
				b.SwitchToNewBlock("no")
				zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)
				b.Ret(zero)
				alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{w: {Kind: mir2.LocReg, Name: "DE"}, v: {Kind: mir2.LocReg, Name: byteReg}, c: {Kind: mir2.LocFlag, Name: "F"}, one: {Kind: mir2.LocReg, Name: "A"}, zero: {Kind: mir2.LocReg, Name: "A"}}}
				text := mustZ80Asm(mir2.Z80Codegen(m, alloc))
				for _, word := range []int{0, 1, 127, 128, 129, 255, 256, 32768, 65535} {
					for _, value := range []int{0, 1, 127, 128, 255} {
						a, z := word, value
						if reverse {
							a, z = z, a
						}
						want := false
						switch cond {
						case mir2.CmpEq:
							want = a == z
						case mir2.CmpNe:
							want = a != z
						case mir2.CmpUlt:
							want = a < z
						case mir2.CmpUle:
							want = a <= z
						case mir2.CmpUgt:
							want = a > z
						case mir2.CmpUge:
							want = a >= z
						}
						asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nLD BC,0x35a7\nLD HL,0xc56d\nLD DE,%d\nLD %s,%d\nCALL arith\nDI\nHALT\n", word, byteReg, value) + text
						got := fix1Execute(t, asm)
						if (got.A != 0) != want || got.SP != 0xff00 {
							t.Fatalf("%s %v reverse=%v word=%d byte=%d gotA=%d SP=%04x\n%s", byteReg, cond, reverse, word, value, got.A, got.SP, asm)
						}
					}
				}
			}
		}
	}
}
