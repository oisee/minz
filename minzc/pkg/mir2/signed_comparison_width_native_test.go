package mir2

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"strings"
	"testing"
)

func TestNativeMixedComparisonSrcTyAndLiveHomes(t *testing.T) {
	for _, cw := range []int{8, 16, 24, 32} {
		for _, cond := range []CmpCond{CmpLt, CmpLe, CmpGt, CmpGe, CmpEq, CmpNe, CmpUlt, CmpUge} {
			for _, sw := range []int{8, 16, 24, 32} {
				for _, reverse := range []bool{false, true} {
					source := "DE"
					if sw == 8 {
						source = "A"
					}
					t.Run(fmt.Sprintf("cw%d_sw%d_%v_reverse%t", cw, sw, cond, reverse), func(t *testing.T) {
						ty := func(w int) Ty {
							switch w {
							case 8:
								return TyI8
							case 16:
								return TyI16
							case 24:
								return TyI24
							default:
								return TyI32
							}
						}
						m := &Module{}
						f := m.AddFunc("cmp")
						b := f.NewBlock("entry")
						x, y, out := Reg(1), Reg(2), Reg(3)
						f.Contract.Params = []Param{{Reg: x, Ty: ty(sw)}, {Reg: y, Ty: TyI32}}
						inst := &Inst{Op: OpCmp, Dst: out, Src: [2]Reg{x, y}, SrcTy: ty(cw), Ty: TyBool, Cond: cond}
						if reverse {
							inst.Src = [2]Reg{y, x}
						}
						b.Insts = []*Inst{inst}
						b.Term = &TermRet{Vals: []Reg{out}}
						sourceKind := LocDWord
						if sw <= 16 {
							sourceKind = LocReg
						}
						var body strings.Builder
						g := &z80cg{mod: m, fn: f, curBlock: b, sb: &body, ar: &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: sourceKind, Name: source}, y: {Kind: LocDWord, Name: "HL"}}}, regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, cmpNeedsTwo: map[Reg]bool{}, cmpSwapped: map[Reg]bool{}, cmpAndZero: map[Reg]bool{}, callFlags: map[Reg]CmpCond{}, pendingFlagReg: NoReg}
						g.genCmp32(inst)
						if g.err != nil {
							t.Fatal(g.err)
						}
						// Capture result without losing live A or comparison flags. The following
						// predicate materialization happens after a balanced PUSH/POP pair.
						g.emit("PUSH AF")
						g.emitFlagPredicateByte(g.condCode(f, out))
						g.emit("LD (0x6200),A")
						g.emit("POP AF")
						g.emit("RET")
						asm := "ORG 0x8000\nentry:\n" + body.String()
						a, e := z80asm.NewAssembler().AssembleString(asm)
						if e != nil || a == nil || len(a.Errors) > 0 {
							t.Fatal(e, asm)
						}
						sx := func(v uint32, w int) int64 { return int64(int32(v<<uint(32-w)) >> uint(32-w)) }
						mask := uint32(0xffffffff)
						if sw < 32 {
							mask = uint32(1)<<uint(sw) - 1
						}
						for _, xv := range []uint32{0, 1, 127, 128, 255, 32767, 32768, 65535, 0xffffff, 0x80000000, 0xffffffff} {
							for _, yv := range []uint32{0, 1, 128, 65535, 65536, 0x80000000, 0xffffffff} {
								xv &= mask
								mem := &flagLifetimeMemory{}
								for i := range mem.data {
									mem.data[i] = 0xa5
								}
								copy(mem.data[0x8000:], a.Binary)
								mem.data[0xeff0], mem.data[0xeff1] = 0, 3
								before := mem.data
								cpu := z80.NewZ80(mem, flagLifetimePorts{})
								mem.cpu = cpu
								cpu.A = 23
								cpu.F = 0x41
								cpu.A_, cpu.F_ = 0x57, 0x95
								cpu.H, cpu.L = byte(yv>>8), byte(yv)
								cpu.H_, cpu.L_ = byte(yv>>24), byte(yv>>16)
								cpu.D, cpu.E = byte(xv>>8), byte(xv)
								cpu.D_, cpu.E_ = byte(xv>>24), byte(xv>>16)
								cpu.B, cpu.C = 0x12, 0x34
								cpu.B_, cpu.C_ = 0x56, 0x78
								cpu.IXH, cpu.IXL = 0x9a, 0xbc
								cpu.IYH, cpu.IYL = 0xde, 0xf0
								if sw == 8 {
									cpu.A = byte(xv)
								}
								priorA := cpu.A
								cpu.SetSP(0xeff0)
								cpu.SetPC(0x8000)
								for n := 0; n < 1000 && cpu.PC() != 0x300; n++ {
									cpu.DoOpcode()
								}
								want := false
								av, bv := xv, yv
								if reverse {
									av, bv = bv, av
								}
								eq := av == bv
								lt := sx(av, cw) < sx(bv, cw)
								switch cond {
								case CmpEq:
									want = eq
								case CmpNe:
									want = !eq
								case CmpUlt:
									want = av < bv
								case CmpUge:
									want = av >= bv
								case CmpLt:
									want = lt
								case CmpLe:
									want = sx(av, cw) <= sx(bv, cw)
								case CmpGt:
									want = sx(av, cw) > sx(bv, cw)
								case CmpGe:
									want = !lt
								}
								if (mem.data[0x6200] == 1) != want || mem.data[0x6200] > 1 || cpu.PC() != 0x300 || cpu.SP() != 0xeff2 || cpu.A != priorA {
									t.Fatalf("x%x y%x got%d want%t A%d/%d\n%s", xv, yv, mem.data[0x6200], want, cpu.A, priorA, asm)
								}
								gotX := uint32(cpu.DE()) | uint32(uint16(cpu.D_)<<8|uint16(cpu.E_))<<16
								gotY := uint32(cpu.HL()) | uint32(uint16(cpu.H_)<<8|uint16(cpu.L_))<<16
								if gotX != xv || gotY != yv || cpu.BC() != 0x1234 || cpu.B_ != 0x56 || cpu.C_ != 0x78 || cpu.IXH != 0x9a || cpu.IXL != 0xbc || cpu.IYH != 0xde || cpu.IYL != 0xf0 || cpu.A_ != 0x57 || cpu.F_ != 0x95 {
									t.Fatal("live home clobber")
								}
								for p, v := range before {
									if p == 0x6200 || p >= 0xef80 && p < 0xeff2 {
										continue
									}
									if mem.data[p] != v {
										t.Fatal("unrelated RAM", p)
									}
								}
								if !bytes.Equal(mem.data[0x8000:0x8000+len(a.Binary)], a.Binary) {
									t.Fatal("code mutation")
								}
							}
						}
					})
				}
			}
		}
	}
}
