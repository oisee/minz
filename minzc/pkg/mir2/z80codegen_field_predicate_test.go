package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// Generator frames store a comparison after computing its destination field.
// Field ADD/INC must not destroy the predicate's still-live CPU flags.
func TestNativePredicateSurvivesFieldAddress(t *testing.T) {
	for _, offset := range []int64{0, 1, 3, 4, 11, 256} {
		for _, cond := range []CmpCond{CmpEq, CmpNe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
			t.Run(fmt.Sprintf("%d/%v", offset, cond), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("predicate_field")
				b := f.NewBlock("entry")
				lhs, rhs, base, predicate, address := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
				f.Contract.Params = []Param{{Reg: lhs, Ty: TyU16}, {Reg: rhs, Ty: TyU16}, {Reg: base, Ty: TyPtr}}
				cmp := &Inst{Op: OpCmp, Dst: predicate, Src: [2]Reg{lhs, rhs}, SrcTy: TyU16, Ty: TyBool, Cond: cond}
				field := &Inst{Op: OpField, Dst: address, Src: [2]Reg{base}, Imm: offset, Ty: TyPtr}
				store := &Inst{Op: OpStore, Src: [2]Reg{address, predicate}, Ty: TyBool}
				b.Append(cmp)
				b.Append(field)
				b.Append(store)
				b.Seal(&TermRet{})
				ar := &AllocResult{Locs: map[Reg]PhysLoc{
					lhs: {Kind: LocReg, Name: "BC"}, rhs: {Kind: LocReg, Name: "DE"}, base: {Kind: LocReg, Name: "IX"},
					predicate: {Kind: LocReg, Name: "F"}, address: {Kind: LocReg, Name: "HL"},
				}}
				sb := new(strings.Builder)
				g := &z80cg{mod: m, fn: f, ar: ar, sb: sb, curBlock: b, regInfo: collectRegInfo(f), constVals: map[Reg]int64{},
					physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, cmpNeedsTwo: map[Reg]bool{}, cmpSwapped: map[Reg]bool{}, cmpAndZero: map[Reg]bool{},
					callFlags: map[Reg]CmpCond{}, pendingFlagReg: NoReg, pendingAccReg: NoReg}
				for _, inst := range b.Insts {
					g.genInst(inst)
				}
				if g.err != nil {
					t.Fatal(g.err)
				}
				g.emit("DI")
				g.emit("HALT")
				asm := "ORG 0x8000\nentry:\n" + sb.String()
				a, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil {
					t.Fatal(err)
				}
				if len(a.Errors) > 0 {
					t.Fatalf("assemble: %v\n%s", a.Errors, asm)
				}
				for _, values := range [][2]uint16{{0, 0}, {1, 2}, {2, 1}, {255, 256}, {256, 255}, {65535, 0}} {
					x, y := values[0], values[1]
					z := emulator.NewRemogattoZ80()
					if err := z.LoadMemory(0x8000, a.Binary); err != nil {
						t.Fatal(err)
					}
					z.Memory()[0x6200+offset] = 197
					z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000, BC: x, DE: y, IX: 0x6200})
					for n := 0; n < 1000 && !z.IsHalted(); n++ {
						z.Step()
					}
					want := false
					switch cond {
					case CmpEq:
						want = x == y
					case CmpNe:
						want = x != y
					case CmpUlt:
						want = x < y
					case CmpUle:
						want = x <= y
					case CmpUgt:
						want = x > y
					case CmpUge:
						want = x >= y
					}
					expected := byte(0)
					if want {
						expected = 1
					}
					if !z.IsHalted() || z.Memory()[0x6200+offset] != expected {
						t.Fatalf("%d vs %d: byte=%d want=%d\n%s", x, y, z.Memory()[0x6200+offset], expected, asm)
					}
				}
			})
		}
	}
}
