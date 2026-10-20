package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// A byte-typed value keeps a one-byte spill slot (z80SpillBytes sizes homes
// from the vreg type). Reading that slot with a 16-bit LD rr,(nn) takes the
// neighbouring slot's byte as the high byte: a u16 loop bound of 3 became
// 0xEA03 and the loop ran away (direct-call fuzz seed 985). A word-width read
// of a byte slot must extend the stored byte, like promote8toPair does for
// byte registers.
func TestByteSpillSlotWordReadExtends(t *testing.T) {
	const neighbour = 0xEA
	for _, cond := range []CmpCond{CmpEq, CmpNe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
		for _, bound := range []int64{0, 1, 3, 255} {
			t.Run(fmt.Sprintf("%v/%d", cond, bound), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("cmp")
				b := f.NewBlock("entry")
				x, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
				f.Contract.Params = []Param{{Reg: x, Ty: TyU16}}
				// y is a byte-width constant: collectRegInfo types it u8, so
				// its spill home is one byte and the word compare must extend.
				konst := &Inst{Op: OpConst, Dst: y, Ty: TyU8, Imm: bound}
				b.Append(konst)
				inst := &Inst{Op: OpCmp, Dst: out, Src: [2]Reg{x, y}, SrcTy: TyU16, Ty: TyBool, Cond: cond}
				b.Append(inst)
				b.Seal(&TermRet{Vals: []Reg{out}})
				slot := Z80SpillLabel("cmp", y)
				next := Z80SpillLabel("cmp", out)
				if z80SpillBytes(TyU8) != 1 {
					t.Fatalf("u8 spill home is %d bytes, test assumes 1", z80SpillBytes(TyU8))
				}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{
					x: {Kind: LocReg, Name: "BC"},
					y: {Kind: LocMem, Name: "rhs_mem"},
				}}
				sb := new(strings.Builder)
				g := &z80cg{mod: m, fn: f, ar: ar, sb: sb, curBlock: b, regInfo: collectRegInfo(f),
					constVals: map[Reg]int64{}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{},
					cmpNeedsTwo: map[Reg]bool{}, cmpSwapped: map[Reg]bool{}, cmpAndZero: map[Reg]bool{},
					callFlags: map[Reg]CmpCond{}, pendingFlagReg: NoReg, pendingAccReg: NoReg}
				// Writer: the production byte-width constant store.
				g.genInst(konst)
				if g.err != nil {
					t.Fatal(g.err)
				}
				g.genCmp(inst)
				if g.err != nil {
					t.Fatal(g.err)
				}
				g.emitFlagPredicateByte(g.condCode(f, out))
				g.emit("    LD (0x6200), A")
				g.emit("    DI")
				g.emit("    HALT")
				asm := "ORG 0x8000\nentry:\n" + sb.String() + "\n" +
					slot + ": DB 0\n" + next + ": DB " + fmt.Sprint(neighbour) + "\n"
				a, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil {
					t.Fatal(err, asm)
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors, asm)
				}
				for _, value := range []uint16{0, 1, 2, 3, 4, 254, 255, 256, 257, 511, 512, 32768, 65535} {
					z := emulator.NewRemogattoZ80()
					z.LoadMemory(0x8000, a.Binary)
					z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xf000, BC: value})
					for n := 0; n < 1000 && !z.IsHalted(); n++ {
						z.Step()
					}
					var want bool
					switch cond {
					case CmpEq:
						want = int64(value) == bound
					case CmpNe:
						want = int64(value) != bound
					case CmpUlt:
						want = int64(value) < bound
					case CmpUle:
						want = int64(value) <= bound
					case CmpUgt:
						want = int64(value) > bound
					case CmpUge:
						want = int64(value) >= bound
					}
					got := z.Memory()[0x6200] != 0
					if !z.IsHalted() || got != want {
						t.Fatalf("%d vs %d got %v want %v halted=%v\n%s", value, bound, got, want, z.IsHalted(), asm)
					}
				}
			})
		}
	}
}
