package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// Exercise physical pairs directly, including live A and both signed overflow
// directions. Value consumers must continue to receive canonical booleans.
func TestWordCompareBranchFlags(t *testing.T) {
	vals := []uint16{0, 1, 127, 128, 255, 256, 257, 32767, 32768, 32769, 65280, 65534, 65535}
	for _, pair := range []string{"HL", "DE", "BC", "IX", "IY"} {
		for _, cond := range []CmpCond{CmpEq, CmpNe, CmpLt, CmpGe, CmpGt, CmpLe} {
			for _, zero := range []bool{true, false} {
				for _, branch := range []bool{true, false} {
					t.Run(fmt.Sprintf("%s/%v/zero=%v/branch=%v", pair, cond, zero, branch), func(t *testing.T) {
						m := &Module{}
						f := m.AddFunc("cmp")
						entry := f.NewBlock("entry")
						yes := f.NewBlock("yes")
						no := f.NewBlock("no")
						x, y, out, acc := f.AllocReg(), f.AllocReg(), f.AllocReg(), f.AllocReg()
						f.Contract.Params = []Param{{Reg: x, Ty: TyI16}, {Reg: y, Ty: TyI16}, {Reg: acc, Ty: TyU8}}
						inst := &Inst{Op: OpCmp, Dst: out, Src: [2]Reg{x, y}, SrcTy: TyI16, Ty: TyBool, Cond: cond}
						entry.Append(inst)
						if branch {
							entry.Seal(&TermBrIf{Cond: out, Then: yes.Label, Else: no.Label})
						} else {
							entry.Seal(&TermRet{Vals: []Reg{out, x, y, acc}})
						}
						yes.Seal(&TermRet{Vals: []Reg{x, y, acc}})
						no.Seal(&TermRet{Vals: []Reg{x, y, acc}})
						rhs := "DE"
						if pair == rhs {
							rhs = "BC"
						}
						ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: pair}, y: {Kind: LocReg, Name: rhs}, out: {Kind: LocReg, Name: "F"}, acc: {Kind: LocReg, Name: "A"}}}
						// Occupy every pair so preserving live A must use the stack
						// fallback, without borrowing an operand's high byte.
						live := []Reg{x, y, acc}
						for _, p := range []string{"HL", "DE", "BC", "IX", "IY"} {
							if p == pair || p == rhs {
								continue
							}
							r := f.AllocReg()
							f.Contract.Params = append(f.Contract.Params, Param{Reg: r, Ty: TyU16})
							ar.Locs[r] = PhysLoc{Kind: LocReg, Name: p}
							live = append(live, r)
						}
						yes.Term = &TermRet{Vals: live}
						no.Term = &TermRet{Vals: live}
						if !branch {
							entry.Term = &TermRet{Vals: append([]Reg{out}, live...)}
						}
						sb := new(strings.Builder)
						g := &z80cg{mod: m, fn: f, ar: ar, sb: sb, curBlock: entry, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, cmpNeedsTwo: map[Reg]bool{}, cmpSwapped: map[Reg]bool{}, cmpAndZero: map[Reg]bool{}, callFlags: map[Reg]CmpCond{}, pendingFlagReg: NoReg, pendingAccReg: NoReg}
						if zero {
							g.constVals[y] = 0
						}
						g.genCmp(inst)
						shape := sb.String()
						if zero && (cond == CmpEq || cond == CmpNe || branch) {
							if strings.Contains(shape, "SBC HL") || strings.Contains(shape, "SCF") {
								t.Fatalf("zero comparison uses subtraction/carry:\n%s", shape)
							}
							if branch && (cond == CmpLt || cond == CmpGe) && !strings.Contains(shape, "BIT 7,") {
								t.Fatalf("missing sign test:\n%s", shape)
							}
							if (cond == CmpEq || cond == CmpNe) && !strings.Contains(shape, "OR "+lowByte(pair)) {
								t.Fatalf("missing word zero test:\n%s", shape)
							}
						}
						if branch {
							g.genTerm(f, entry.Term)
							g.emitf("%s:", blockLabel(f.Name, yes.Label))
							g.emit("LD (0x6201),A")
							g.emit("LD A,1")
							g.emit("JP done")
							g.emitf("%s:", blockLabel(f.Name, no.Label))
							g.emit("LD (0x6201),A")
							g.emit("LD A,0")
						} else {
							g.emitLD8("A", g.loc(acc))
							g.emit("LD (0x6201),A")
							g.emitFlagPredicateByte(g.condCode(f, out))
						}
						g.emit("done:")
						g.emit("LD (0x6200),A")
						g.emit("DI")
						g.emit("HALT")
						asm := "ORG 0x8000\nentry:\n" + sb.String()
						if branch && strings.Contains(asm, ".scmp_") {
							t.Fatalf("branch materializes signed carry:\n%s", asm)
						}
						bin, err := z80asm.NewAssembler().AssembleString(asm)
						if err != nil {
							t.Fatal(err, asm)
						}
						if len(bin.Errors) > 0 {
							t.Fatal(bin.Errors, asm)
						}
						for _, a := range vals {
							for _, b := range vals {
								if zero && b != 0 {
									continue
								}
								z := emulator.NewRemogattoZ80()
								z.LoadMemory(0x8000, bin.Binary)
								regs := emulator.Registers{PC: 0x8000, SP: 0xf000, A: 0x5a, HL: 0x1234, DE: 0x5678, BC: 0x1357, IX: 0x2468, IY: 0x369a}
								set := func(p string, v uint16) {
									switch p {
									case "HL":
										regs.HL = v
									case "DE":
										regs.DE = v
									case "BC":
										regs.BC = v
									case "IX":
										regs.IX = v
									case "IY":
										regs.IY = v
									}
								}
								set(pair, a)
								set(rhs, b)
								z.SetRegisters(regs)
								for n := 0; n < 1000 && !z.IsHalted(); n++ {
									z.Step()
								}
								want := false
								switch cond {
								case CmpEq:
									want = a == b
								case CmpNe:
									want = a != b
								case CmpLt:
									want = int16(a) < int16(b)
								case CmpGe:
									want = int16(a) >= int16(b)
								case CmpGt:
									want = int16(a) > int16(b)
								case CmpLe:
									want = int16(a) <= int16(b)
								}
								got := z.GetRegisters()
								if !z.IsHalted() || (z.Memory()[0x6200] != 0) != want || z.Memory()[0x6201] != 0x5a || got.HL != regs.HL || got.DE != regs.DE || got.BC != regs.BC || got.IX != regs.IX || got.IY != regs.IY {
									t.Fatalf("%d vs %d: bool=%d want=%v savedA=%x regs=%+v want=%+v\n%s", a, b, z.Memory()[0x6200], want, z.Memory()[0x6201], got, regs, asm)
								}
							}
						}
					})
				}
			}
		}
	}
}
