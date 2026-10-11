package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func policyCG(m *Module, ar *AllocResult) *z80cg {
	f := m.Funcs[0]
	return &z80cg{unsignedPairComparePolicy: UnsignedPairComparePreserveForSpeed, mod: m, fn: f, curBlock: f.Entry(), ar: ar, regInfo: collectRegInfo(f), liveness: ComputeLiveness(f), pendingAccReg: NoReg, pendingFlagReg: NoReg, constVals: map[Reg]int64{}, sb: &strings.Builder{}, cmpSwapped: map[Reg]bool{}, cmpNeedsTwo: map[Reg]bool{}}
}

// A successor-only third value in HL makes BC/DE comparisons eligible too.
func thirdPairFixture(lhs, rhs string, cond CmpCond, half bool) (*Module, *AllocResult) {
	m := &Module{}
	f := m.AddFunc("third_pair")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU16, ClassPair)
	y := b.Param("y", TyU16, ClassPair)
	ty := TyU16
	cls := ClassPair
	home := "HL"
	ret := ClassPointer
	if half {
		ty = TyU8
		cls = ClassGeneral
		home = "H"
		ret = ClassAcc
	}
	keep := b.Param("keep", ty, cls)
	c := b.Cmp(cond, x, y, ClassFlag, false)
	b.BrIf(c, "yes", nil, "no", nil)
	b.SwitchToNewBlock("yes")
	b.Ret(keep)
	b.SwitchToNewBlock("no")
	b.Ret(keep)
	f.Contract.Returns = []Return{{Ty: ty, Class: ret}}
	return m, &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: lhs}, y: {Kind: LocReg, Name: rhs}, keep: {Kind: LocReg, Name: home}, c: {Kind: LocFlag, Name: "F"}}}
}

func TestUnsignedPairThirdLivePairAndHalfNative(t *testing.T) {
	tests := 0
	for _, p := range [][2]string{{"BC", "DE"}, {"DE", "BC"}} {
		for _, half := range []bool{false, true} {
			for _, cond := range []CmpCond{CmpUlt, CmpUge} {
				m, ar := thirdPairFixture(p[0], p[1], cond, half)
				if e := Verify(m); e != nil {
					t.Fatal(e)
				}
				asm, e := Z80Codegen(m, ar, Z80CodegenOptions{UnsignedPairComparePolicy: UnsignedPairComparePreserveForSpeed})
				if e != nil {
					t.Fatal(e)
				}
				if !strings.Contains(asm, ".ucmp_pair_done_") {
					t.Fatal("third HL liveness not selected", asm)
				}
				a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
				if e != nil || len(a.Errors) > 0 {
					t.Fatal(e, a.Errors)
				}
				done := uint16(0)
				for n, pc := range a.Symbols {
					if strings.Contains(n, "ucmp_pair_done_") {
						done = uint16(pc)
					}
				}
				for _, v := range [][2]uint16{{0, 0}, {0, 65535}, {65535, 0}, {255, 256}, {256, 255}, {32767, 32768}, {0x1234, 0x1235}} {
					for _, flags := range []byte{0, 255} {
						z := emulator.NewRemogattoZ80()
						z.Reset()
						z.LoadMemory(0x8000, a.Binary)
						r := emulator.Registers{PC: uint16(a.Symbols["third_pair"]), SP: 0xf000, HL: 0xa7b9, DE: 0x5678, BC: 0x9012, A: 0x65, F: flags}
						setCmpPair(&r, p[0], v[0])
						setCmpPair(&r, p[1], v[1])
						z.SetRegisters(r)
						for step := 0; z.GetRegisters().PC != done; step++ {
							if step > 5 {
								t.Fatal("compare did not finish")
							}
							z.Step()
						}
						got := z.GetRegisters()
						if got.HL != r.HL || got.DE != r.DE || got.BC != r.BC || got.SP != r.SP || (got.F&1 != 0) != (v[0] < v[1]) || (got.F&0x40 != 0) != (v[0] == v[1]) {
							t.Fatal("third-live preservation", p, v, got, r)
						}
						z.SetMemory(0xf000, 0)
						z.SetMemory(0xf001, 0x70)
						for step := 0; z.GetRegisters().PC != 0x7000; step++ {
							if step > 30 {
								t.Fatal("return did not finish")
							}
							z.Step()
						}
						if half {
							if z.GetRegisters().A != byte(r.HL>>8) {
								t.Fatal("live half return")
							}
						} else if z.GetRegisters().HL != r.HL {
							t.Fatal("live pair return")
						}
						tests++
					}
				}
			}
		}
	}
	t.Logf("actual third pair/half preservation and typed returns: %d cases", tests)
}

func TestUnsignedPairEligibilityGuards(t *testing.T) {
	for _, name := range []string{"A", "AF", "overrideA", "overrideSamePair", "pendingAcc", "pendingFlag", "constantLhs", "constantRhs", "IX", "IY", "spill", "legacy", "unknownPolicy", "signed", "twoTest", "flagsOnly", "sourceI16", "byteSource", "nonFlagDst"} {
		t.Run(name, func(t *testing.T) {
			m, ar := pairCmpFixture("HL", "DE", CmpUlt, false, false)
			g := policyCG(m, ar)
			i := g.curBlock.Insts[0]
			// Prove fixture would be selected before changing its unsupported condition.
			if !g.tryUnsignedPreservePairCmp(i) {
				t.Fatal("baseline not eligible")
			}
			g = policyCG(m, ar)
			switch name {
			case "A", "AF", "IX", "IY":
				ar.Locs[i.Src[0]] = PhysLoc{Kind: LocReg, Name: name}
			case "overrideA":
				g.physOverride = map[Reg]string{i.Src[0]: "A"}
			case "overrideSamePair":
				g.physOverride = map[Reg]string{i.Src[1]: "HL"}
			case "pendingAcc":
				g.pendingAccReg = i.Src[0]
			case "pendingFlag":
				g.pendingFlagReg = i.Src[1]
			case "constantLhs":
				g.constVals[i.Src[0]] = 0
			case "constantRhs":
				g.constVals[i.Src[1]] = 65535
			case "spill":
				ar.Locs[i.Src[0]] = PhysLoc{Kind: LocMem}
			case "legacy":
				g.unsignedPairComparePolicy = UnsignedPairCompareLegacy
			case "unknownPolicy":
				g.unsignedPairComparePolicy = UnsignedPairComparePolicy(255)
			case "signed":
				i.Cond = CmpLt
			case "twoTest":
				i.Cond = CmpUle
			case "flagsOnly":
				i.FlagsOnly = true
			case "sourceI16":
				i.SrcTy = TyI16
			case "byteSource":
				ri := g.regInfo[i.Src[0]]
				ri.Ty = TyU8
				g.regInfo[i.Src[0]] = ri
			case "nonFlagDst":
				ar.Locs[i.Dst] = PhysLoc{Kind: LocReg, Name: "A"}
			}
			if g.tryUnsignedPreservePairCmp(i) {
				t.Fatal("unsupported condition accepted")
			}
		})
	}
}

func TestUnsignedPairPolicyLegacyAndUnknownRefusal(t *testing.T) {
	m, ar := pairCmpFixture("HL", "DE", CmpUlt, false, false)
	original, e := Z80Codegen(m, ar)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []UnsignedPairComparePolicy{UnsignedPairCompareLegacy} {
		actual, e := Z80Codegen(m, ar, Z80CodegenOptions{UnsignedPairComparePolicy: p})
		if e != nil || actual != original {
			t.Fatal("legacy/unknown policy changes output", p, e)
		}
	}
	actual, err := Z80Codegen(m, ar, Z80CodegenOptions{UnsignedPairComparePolicy: 255})
	if err == nil || actual != "" {
		t.Fatal("invalid public policy accepted")
	}
}
