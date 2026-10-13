package mir2

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

// Spilling a word must not turn equality into a comparison of its low byte.
// In the Z-machine a branch offset of 256 was mistaken for the return offset 0.
func TestNativeWordSpillComparisonConstants(t *testing.T) {
	for _, cond := range []CmpCond{CmpEq, CmpNe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
		for _, constant := range []int64{0, 1, 255, 256, 257, 32768, 65535} {
			t.Run(fmt.Sprintf("%v/%d", cond, constant), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("cmp")
				b := f.NewBlock("entry")
				x, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
				f.Contract.Params = []Param{{Reg: x, Ty: TyU16}, {Reg: y, Ty: TyU16}}
				inst := &Inst{Op: OpCmp, Dst: out, Src: [2]Reg{x, y}, SrcTy: TyU16, Ty: TyBool, Cond: cond}
				b.Append(inst)
				b.Seal(&TermRet{Vals: []Reg{out}})
				ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocMem, Name: "lhs_mem"}, y: {Kind: LocReg, Name: "BC"}}}
				if constant <= 255 {
					ar.Locs[y] = PhysLoc{Kind: LocReg, Name: "C"}
				}
				sb := new(strings.Builder)
				g := &z80cg{mod: m, fn: f, ar: ar, sb: sb, curBlock: b, regInfo: collectRegInfo(f), constVals: map[Reg]int64{y: constant}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, cmpNeedsTwo: map[Reg]bool{}, cmpSwapped: map[Reg]bool{}, cmpAndZero: map[Reg]bool{}, callFlags: map[Reg]CmpCond{}, pendingFlagReg: NoReg, pendingAccReg: NoReg}
				g.emitf("LD BC,%d", constant)
				g.genCmp(inst)
				g.emitFlagPredicateByte(g.condCode(f, out))
				g.emit("LD (0x6200),A")
				g.emit("DI")
				g.emit("HALT")
				asm := "ORG 0x8000\nentry:\n" + sb.String() + "\n_spill_cmp_r1: DB 0,0\n"
				a, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil {
					t.Fatal(err, asm)
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors, asm)
				}
				for _, value := range []uint16{0, 1, 2, 254, 255, 256, 257, 511, 512, 32767, 32768, 32769, 65280, 65534, 65535} {
					z := emulator.NewRemogattoZ80()
					z.LoadMemory(0x8000, a.Binary)
					binary.LittleEndian.PutUint16(z.Memory()[a.Symbols["_spill_cmp_r1"]:a.Symbols["_spill_cmp_r1"]+2], value)
					z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000})
					for n := 0; n < 1000 && !z.IsHalted(); n++ {
						z.Step()
					}
					want := false
					switch cond {
					case CmpEq:
						want = int64(value) == constant
					case CmpNe:
						want = int64(value) != constant
					case CmpUlt:
						want = int64(value) < constant
					case CmpUle:
						want = int64(value) <= constant
					case CmpUgt:
						want = int64(value) > constant
					case CmpUge:
						want = int64(value) >= constant
					}
					got := z.Memory()[0x6200] != 0
					if !z.IsHalted() || got != want {
						t.Fatalf("%d vs %d got %v want %v\n%s", value, constant, got, want, asm)
					}
				}
			})
		}
	}
}
