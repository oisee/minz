package mir2

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestNativeWideComparisonAllHomes(t *testing.T) {
	vals := []uint32{0, 1, 255, 256, 65535, 65536, 0xffffff, 0x1000000, 0x7fffffff, 0x80000000, 0xffffffff, 0x12345678}
	for _, cond := range []CmpCond{CmpEq, CmpNe, CmpLt, CmpLe, CmpGt, CmpGe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
		for _, homes := range [][2]string{{"HL", "DE"}, {"DE", "HL"}, {"BC", "DE"}, {"HL", "HL"}, {"lhs_mem", "DE"}, {"BC", "rhs_mem"}, {"lhs_mem", "rhs_mem"}, {"HL", "constant"}} {
			t.Run(fmt.Sprintf("%v/%s/%s", cond, homes[0], homes[1]), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("cmp")
				block := f.NewBlock("entry")
				x, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
				f.Contract.Params = []Param{{Reg: x, Ty: TyI32}, {Reg: y, Ty: TyI32}}
				inst := &Inst{Op: OpCmp, Dst: out, Src: [2]Reg{x, y}, SrcTy: TyI32, Ty: TyBool, Cond: cond}
				block.Append(inst)
				block.Seal(&TermRet{Vals: []Reg{out}})
				ar := &AllocResult{Locs: map[Reg]PhysLoc{}}
				for i, r := range []Reg{x, y} {
					loc := homes[i]
					kind := LocDWord
					if strings.Contains(loc, "mem") {
						kind = LocMem
					}
					ar.Locs[r] = PhysLoc{Kind: kind, Name: loc}
				}
				sb := new(strings.Builder)
				g := &z80cg{mod: m, fn: f, ar: ar, sb: sb, curBlock: block, regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, cmpNeedsTwo: map[Reg]bool{}, cmpSwapped: map[Reg]bool{}, cmpAndZero: map[Reg]bool{}, callFlags: map[Reg]CmpCond{}, pendingFlagReg: NoReg, pendingAccReg: NoReg}
				// Directly exercise comparison emission: no allocator repair can hide clobbers.
				if homes[1] == "constant" {
					g.constVals[y] = 0x80000100
				}
				g.genCmp32(inst)
				if g.err != nil {
					t.Fatal(g.err)
				}
				g.emit("PUSH AF")
				g.emit("POP IY")
				g.emitFlagPredicateByte(g.condCode(f, out))
				g.emit("LD (0x6200),A")
				g.emit("LD A,0x5a")
				for i, p := range []string{"HL", "DE", "BC"} {
					g.emitf("LD (0x%x),%s", 0x6210+i*4, p)
					g.emit("EXX")
					g.emitf("LD (0x%x),%s", 0x6212+i*4, p)
					g.emit("EXX")
				}
				g.emit("DI")
				g.emit("HALT")
				asm := "ORG 0x8000\nentry:\n" + sb.String() + "\n_spill_cmp_r1: DB 0,0,0,0\n_spill_cmp_r2: DB 0,0,0,0\n"
				bin, e := z80asm.NewAssembler().AssembleString(asm)
				if e != nil {
					t.Fatal(e, asm)
				}
				if len(bin.Errors) > 0 {
					t.Fatal(bin.Errors, asm)
				}
				for _, a := range vals {
					for _, b := range vals {
						if homes[0] == homes[1] {
							b = a
						}
						if homes[1] == "constant" {
							b = 0x80000100
						}
						setup := map[string]uint32{"HL": 0x12345678, "DE": 0x9abcdef0, "BC": 0x13572468}
						for i, v := range []uint32{a, b} {
							if homes[i] == "HL" || homes[i] == "DE" || homes[i] == "BC" {
								setup[homes[i]] = v
							}
						}
						var boot strings.Builder
						for _, p := range []string{"HL", "DE", "BC"} {
							v := setup[p]
							fmt.Fprintf(&boot, "LD %s,%d\nEXX\nLD %s,%d\nEXX\n", p, v&65535, p, v>>16)
						}
						fmt.Fprintf(&boot, "LD IX,0x2468\nLD IY,0x3579\nLD A,0x5a\nSCF\nJP 0x8000\n")
						bboot, e := z80asm.NewAssembler().AssembleString("ORG 0x7000\n" + boot.String())
						if e != nil {
							t.Fatal(e)
						}
						z := emulator.NewRemogattoZ80()
						z.LoadMemory(0x8000, bin.Binary)
						z.LoadMemory(0x7000, bboot.Binary)
						binary.LittleEndian.PutUint32(z.Memory()[bin.Symbols["_spill_cmp_r1"]:bin.Symbols["_spill_cmp_r1"]+4], a)
						binary.LittleEndian.PutUint32(z.Memory()[bin.Symbols["_spill_cmp_r2"]:bin.Symbols["_spill_cmp_r2"]+4], b)
						z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
						for n := 0; n < 1000 && !z.IsHalted(); n++ {
							z.Step()
						}
						lt := a < b
						if cond == CmpLt || cond == CmpLe || cond == CmpGt || cond == CmpGe {
							lt = int32(a) < int32(b)
						}
						eq := a == b
						want := false
						switch cond {
						case CmpEq:
							want = eq
						case CmpNe:
							want = !eq
						case CmpLt, CmpUlt:
							want = lt
						case CmpLe, CmpUle:
							want = lt || eq
						case CmpGt, CmpUgt:
							want = !lt && !eq
						case CmpGe, CmpUge:
							want = !lt
						}
						got := z.Memory()[0x6200]
						if (got == 1) != want || got > 1 {
							t.Fatalf("a%x b%x got%d want%v\n%s", a, b, got, want, asm)
						}
						rr := z.GetRegisters()
						if rr.IX != 0x2468 || rr.SP != 0xf000 || !z.IsHalted() {
							t.Fatal("scratch/bank/SP", rr)
						}
						for i, p := range []string{"HL", "DE", "BC"} {
							if got := binary.LittleEndian.Uint32(z.Memory()[0x6210+i*4 : 0x6214+i*4]); got != setup[p] {
								t.Fatalf("live %s got%x want%x", p, got, setup[p])
							}
						}
						// AF snapshot captures original A, plus final full-value C/Z.
						if byte(rr.IY>>8) != 0x5a || ((rr.IY&0x40) != 0) != eq || ((rr.IY&1) != 0) != lt {
							t.Fatalf("full flags/live A a%x b%x IY%x", a, b, rr.IY)
						}
					}
				}
			})
		}
	}
}
