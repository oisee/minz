package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func pairCmpFixture(lhs, rhs string, cond CmpCond, liveA, dead bool) (*Module, *AllocResult) {
	m := &Module{}
	f := m.AddFunc("pair_compare")
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	if liveA {
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	}
	if dead {
		f.Contract.Returns = []Return{{Ty: TyBool, Class: ClassAcc}}
	}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU16, ClassPair)
	y := b.Param("y", TyU16, ClassPair)
	a := NoReg
	if liveA {
		a = b.Param("a", TyU8, ClassAcc)
	}
	c := b.Cmp(cond, x, y, ClassFlag, false)
	if dead {
		b.Ret(c)
	} else {
		b.BrIf(c, "yes", nil, "no", nil)
		b.SwitchToNewBlock("yes")
		if liveA {
			b.Ret(a)
		} else {
			b.Ret(x)
		}
		b.SwitchToNewBlock("no")
		if liveA {
			b.Ret(a)
		} else {
			b.Ret(y)
		}
	}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: lhs}, y: {Kind: LocReg, Name: rhs}, c: {Kind: LocFlag, Name: "F"}}}
	if liveA {
		ar.Locs[a] = PhysLoc{Kind: LocReg, Name: "A"}
	}
	return m, ar
}
func setCmpPair(r *emulator.Registers, p string, v uint16) {
	switch p {
	case "HL":
		r.HL = v
	case "DE":
		r.DE = v
	case "BC":
		r.BC = v
	}
}
func TestExperimentalUnsignedPairCompareNative(t *testing.T) {
	values := [][2]uint16{{0, 0}, {0, 65535}, {65535, 0}, {255, 256}, {256, 255}, {32767, 32768}, {32768, 32767}, {0x1234, 0x1235}, {0x1235, 0x1234}, {65535, 65535}}
	for _, p := range [][2]string{{"HL", "DE"}, {"DE", "HL"}, {"HL", "BC"}, {"BC", "HL"}, {"DE", "BC"}, {"BC", "DE"}} {
		for _, cond := range []CmpCond{CmpUlt, CmpUge} {
			m, ar := pairCmpFixture(p[0], p[1], cond, false, false)
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			text, e := Z80Codegen(m, ar, Z80CodegenOptions{ExperimentalUnsignedPreservePair: true})
			if e != nil {
				t.Fatal(e)
			}
			if !strings.Contains(text, ".ucmp_pair_done_") {
				if p[0] != "HL" && p[1] != "HL" {
					continue
				}
				t.Fatalf("not selected%v\n%s", p, text)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			var done uint16
			for n, pc := range a.Symbols {
				if strings.Contains(n, "ucmp_pair_done_") {
					done = uint16(pc)
				}
			}
			if done == 0 {
				t.Fatal("no done")
			}
			for _, v := range values {
				for _, flags := range []byte{0, 255} {
					z := emulator.NewRemogattoZ80()
					z.Reset()
					z.LoadMemory(0x8000, a.Binary)
					r := emulator.Registers{PC: uint16(a.Symbols["pair_compare"]), SP: 0xf000, HL: 0xabcd, DE: 0x5678, BC: 0x9012, A: 0x65, F: flags}
					setCmpPair(&r, p[0], v[0])
					setCmpPair(&r, p[1], v[1])
					z.SetRegisters(r)
					cycles := 0
					for steps := 0; z.GetRegisters().PC != done; steps++ {
						if steps > 5 {
							t.Fatal("no finish")
						}
						cycles += z.Step()
					}
					got := z.GetRegisters()
					wantT := 23
					if byte(v[0]>>8) != byte(v[1]>>8) {
						wantT = 20
					}
					if (got.F&1 != 0) != (v[0] < v[1]) || (got.F&0x40 != 0) != (v[0] == v[1]) || cycles != wantT || got.HL != r.HL || got.DE != r.DE || got.BC != r.BC || got.SP != r.SP {
						t.Fatalf("p%v v%v got%+v T%d", p, v, got, cycles)
					}
					// Consume the predicate through generated branch and return.
					z.SetMemory(0xf000, 0)
					z.SetMemory(0xf001, 0x70)
					for steps := 0; z.GetRegisters().PC != 0x7000; steps++ {
						if steps > 30 {
							t.Fatal("return did not finish")
						}
						z.Step()
					}
					choose := v[0] < v[1]
					if cond == CmpUge {
						choose = !choose
					}
					want := v[1]
					if choose {
						want = v[0]
					}
					if z.GetRegisters().HL != want {
						t.Fatalf("predicate orientation cond%d values%v got%04x want%04x", cond, v, z.GetRegisters().HL, want)
					}
				}
			}
		}
	}
}
func TestExperimentalUnsignedPairCompareFallback(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cond    CmpCond
		a, dead bool
	}{{"liveA", CmpUlt, true, false}, {"deadPairs", CmpUlt, false, true}, {"signed", CmpLt, false, false}, {"twoTest", CmpUle, false, false}} {
		m, ar := pairCmpFixture("HL", "DE", tc.cond, tc.a, tc.dead)
		old, e := Z80Codegen(m, ar)
		if e != nil {
			t.Fatal(e)
		}
		new, e := Z80Codegen(m, ar, Z80CodegenOptions{ExperimentalUnsignedPreservePair: true})
		if e != nil || old != new {
			t.Fatal("fallback", tc.name, e)
		}
	}
}

func TestExperimentalUnsignedPairPendingAndConstantRefusal(t *testing.T) {
	m, ar := pairCmpFixture("HL", "DE", CmpUlt, false, false)
	f := m.Funcs[0]
	inst := f.Blocks[0].Insts[0]
	g := &z80cg{experimentalUnsignedPreservePair: true, mod: m, fn: f, curBlock: f.Blocks[0], ar: ar, regInfo: collectRegInfo(f), constVals: map[Reg]int64{inst.Src[0]: 7}}
	if g.tryExperimentalUnsignedPairCmp(inst) {
		t.Fatal("constant operand admitted")
	}
	g.constVals = map[Reg]int64{}
	g.pendingAccReg = inst.Src[0]
	if g.tryExperimentalUnsignedPairCmp(inst) {
		t.Fatal("live pending accumulator admitted")
	}
	g.pendingAccReg = NoReg
	g.pendingFlagReg = inst.Src[1]
	if g.tryExperimentalUnsignedPairCmp(inst) {
		t.Fatal("live pending condition admitted")
	}
}
