package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// A word remaining in HL is consumed after an indexed shift. The oracle uses
// Go's modulo-65536 arithmetic rather than another code generator.
func TestIndexedWordShiftPreservesLiveHLNative(t *testing.T) {
	for _, home := range []string{"IX", "IY"} {
		for _, op := range []mir2.Op{mir2.OpShl, mir2.OpShr, mir2.OpSar} {
			for _, count := range []int{0, 1, 4, 8, 15} {
				for _, value := range []uint16{0, 1, 0x0d75, 0x7fff, 0x8000, 0xffff} {
					m := &mir2.Module{}
					f := m.AddFunc("shift_keep")
					f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
					b := mir2.NewBuilder(f)
					b.SwitchToNewBlock("entry")
					keep := b.Param("keep", mir2.TyU16, mir2.ClassPointer)
					x := b.Param("x", mir2.TyU16, mir2.ClassPair)
					k := b.Const(int64(count), mir2.TyU16, mir2.ClassPair)
					shifted := b.BinOp(op, x, k, mir2.TyU16, mir2.ClassPair)
					result := b.Or(keep, shifted, mir2.TyU16, mir2.ClassPointer)
					b.Ret(result)
					locs := map[mir2.Reg]mir2.PhysLoc{keep: {Kind: mir2.LocReg, Name: "HL"}, x: {Kind: mir2.LocIXY, Name: home}, k: {Kind: mir2.LocReg, Name: "BC"}, shifted: {Kind: mir2.LocIXY, Name: home}, result: {Kind: mir2.LocReg, Name: "HL"}}
					asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
					regs := fix1Execute(t, fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD HL,0x5a0a\nLD %s,%d\nCALL shift_keep\nDI\nHALT\n", home, value)+asm)
					var shiftedWant uint16
					switch op {
					case mir2.OpShl:
						shiftedWant = value << count
					case mir2.OpShr:
						shiftedWant = value >> count
					case mir2.OpSar:
						shiftedWant = uint16(int16(value) >> count)
					}
					want := uint16(0x5a0a) | shiftedWant
					got := regs.HL
					if got != want || regs.SP != 0xff00 {
						t.Fatalf("%s op%v count%d value%x got%x want%x SP%x\n%s", home, op, count, value, got, want, regs.SP, asm)
					}
				}
			}
		}
	}
}
