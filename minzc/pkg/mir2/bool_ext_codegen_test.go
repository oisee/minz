package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestZ80BoolByteAndWordExtension(t *testing.T) {
	for _, src := range []string{"A", "C", "D", "H", "IYH"} {
		for _, dst := range []string{"C", "E", "HL", "DE", "BC", "IX", "IY", "spill8", "spill16"} {
			for _, v := range []byte{0, 1} {
				t.Run(fmt.Sprintf("%s_%s_%d", src, dst, v), func(t *testing.T) {
					m := &mir2.Module{}
					f := m.AddFunc("extend")
					b := mir2.NewBuilder(f)
					b.SwitchToNewBlock("entry")
					x := b.Param("x", mir2.TyBool, mir2.ClassGeneral)
					ty := mir2.TyU16
					if len(dst) == 1 || dst == "spill8" {
						ty = mir2.TyU8
					}
					out := b.Ext(x, mir2.TyBool, ty, mir2.ClassGeneral)
					b.Ret(out)
					rc := mir2.ClassPointer
					if ty == mir2.TyU8 {
						rc = mir2.ClassAcc
					}
					f.Contract.Returns = []mir2.Return{{Ty: ty, Class: rc}}
					ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Name: src}, out: {Name: dst}}}
					if dst == "spill8" || dst == "spill16" {
						ar.Locs[out] = mir2.PhysLoc{Kind: mir2.LocMem}
						ar.Spilled = []mir2.Reg{out}
					}
					asm := "ORG 0x8000\nLD SP,0xFF00\n" + fmt.Sprintf("LD %s,%d\nCALL extend\nDI\nHALT\n", src, v) + mustZ80Asm(mir2.Z80Codegen(m, ar))
					r := fix1Execute(t, asm)
					got := r.HL
					if ty == mir2.TyU8 {
						got = uint16(r.A)
					}
					if got != uint16(v) {
						t.Fatalf("got%d want%d\n%s", got, v, asm)
					}
				})
			}
		}
	}
}
func TestZ80BoolFlagExtensionPreservesLiveAccumulator(t *testing.T) {
	for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUle, mir2.CmpUgt, mir2.CmpUge} {
		for _, pair := range [][2]byte{{0, 0}, {0, 1}, {1, 0}, {127, 128}, {255, 255}} {
			for _, flagLoc := range []string{"F", "A"} {
				for _, dst := range []string{"C", "HL", "DE", "BC", "IX", "IY", "spill8", "spill16"} {
					t.Run(fmt.Sprintf("%v_%d_%d_%s_%s", cond, pair[0], pair[1], dst, flagLoc), func(t *testing.T) {
						m := &mir2.Module{}
						f := m.AddFunc("extend")
						b := mir2.NewBuilder(f)
						b.SwitchToNewBlock("entry")
						x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
						y := b.Param("y", mir2.TyU8, mir2.ClassGeneral)
						cmp := b.Cmp(cond, x, y, mir2.ClassFlag, false)
						live := b.Const(103, mir2.TyU8, mir2.ClassAcc)
						ty := mir2.TyU16
						if dst == "C" || dst == "spill8" {
							ty = mir2.TyU8
						}
						out := b.Ext(cmp, mir2.TyBool, ty, mir2.ClassGeneral)
						b.Ret(out, live)
						f.Contract.Returns = []mir2.Return{{Ty: ty, Class: mir2.ClassPointer}, {Ty: mir2.TyU8, Class: mir2.ClassAcc}}
						if ty == mir2.TyU8 {
							f.Contract.Returns[0].Class = mir2.ClassRegC
						}
						ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Name: "D"}, y: {Name: "E"}, cmp: {Name: flagLoc}, live: {Name: "A"}, out: {Name: dst}}}
						if dst == "spill8" || dst == "spill16" {
							ar.Locs[out] = mir2.PhysLoc{Kind: mir2.LocMem}
							ar.Spilled = []mir2.Reg{out}
						}
						asm := "ORG 0x8000\nLD SP,0xFF00\n" + fmt.Sprintf("LD D,%d\nLD E,%d\nCALL extend\nDI\nHALT\n", pair[0], pair[1]) + mustZ80Asm(mir2.Z80Codegen(m, ar))
						want := false
						switch cond {
						case mir2.CmpEq:
							want = pair[0] == pair[1]
						case mir2.CmpNe:
							want = pair[0] != pair[1]
						case mir2.CmpUlt:
							want = pair[0] < pair[1]
						case mir2.CmpUle:
							want = pair[0] <= pair[1]
						case mir2.CmpUgt:
							want = pair[0] > pair[1]
						case mir2.CmpUge:
							want = pair[0] >= pair[1]
						}
						w := uint16(0)
						if want {
							w = 1
						}
						r := fix1Execute(t, asm)
						got := r.HL
						if ty == mir2.TyU8 {
							got = r.BC & 255
						}
						if got != w || r.A != 103 {
							t.Fatalf("result%d want%d A%d want103\n%s", got, w, r.A, asm)
						}
					})
				}
			}
		}
	}
}

func TestZ80BoolExtensionKeepsByteSourceAndLiveAccumulator(t *testing.T) {
	for _, where := range [][2]string{{"C", "HL"}, {"H", "DE"}, {"IYH", "IX"}, {"D", "BC"}} {
		for _, v := range []byte{0, 1} {
			t.Run(fmt.Sprintf("%s_%s_%d", where[0], where[1], v), func(t *testing.T) {
				m := &mir2.Module{}
				f := m.AddFunc("extend")
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				x := b.Param("x", mir2.TyBool, mir2.ClassGeneral)
				live := b.Param("live", mir2.TyU8, mir2.ClassAcc)
				out := b.Ext(x, mir2.TyBool, mir2.TyU16, mir2.ClassPointer)
				b.Ret(out, x, live)
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}, {Ty: mir2.TyBool, Class: mir2.ClassRegC}, {Ty: mir2.TyU8, Class: mir2.ClassAcc}}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Name: where[0]}, live: {Name: "A"}, out: {Name: where[1]}}}
				asm := "ORG 0x8000\nLD SP,0xFF00\n" + fmt.Sprintf("LD %s,%d\nLD A,103\nCALL extend\nDI\nHALT\n", where[0], v) + mustZ80Asm(mir2.Z80Codegen(m, ar))
				r := fix1Execute(t, asm)
				if r.HL != uint16(v) || byte(r.BC) != v || r.A != 103 {
					t.Fatalf("HL%d C%d A%d\n%s", r.HL, byte(r.BC), r.A, asm)
				}
			})
		}
	}
}

// A flag-resident predicate remains live after its first widening. Subsequent
// arithmetic and a second comparison must consume the same canonical value.
func TestZ80BoolFlagRepeatedExtensionAcrossArithmetic(t *testing.T) {
	for _, cond := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUle, mir2.CmpUgt, mir2.CmpUge} {
		for _, xy := range [][2]byte{{0, 0}, {0, 1}, {1, 0}, {127, 128}, {255, 255}} {
			t.Run(fmt.Sprintf("%v_%d_%d", cond, xy[0], xy[1]), func(t *testing.T) {
				m := &mir2.Module{}
				f := m.AddFunc("extend")
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
				y := b.Param("y", mir2.TyU8, mir2.ClassGeneral)
				predicate := b.Cmp(cond, x, y, mir2.ClassFlag, false)
				first := b.Ext(predicate, mir2.TyBool, mir2.TyU16, mir2.ClassPointer)
				other := b.Cmp(mir2.CmpNe, x, y, mir2.ClassFlag, false)
				byteOther := b.Ext(other, mir2.TyBool, mir2.TyU8, mir2.ClassGeneral)
				second := b.Ext(predicate, mir2.TyBool, mir2.TyU16, mir2.ClassPointer)
				sum := b.Add(first, second, mir2.TyU16, mir2.ClassPointer)
				b.Ret(sum, byteOther)
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}, {Ty: mir2.TyU8, Class: mir2.ClassAcc}}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Name: "D"}, y: {Name: "E"}, predicate: {Name: "F"}, first: {Name: "IX"}, other: {Name: "F"}, byteOther: {Name: "C"}, second: {Name: "HL"}, sum: {Name: "HL"}}}
				asm := "ORG 0x8000\nLD SP,0xFF00\n" + fmt.Sprintf("LD D,%d\nLD E,%d\nCALL extend\nDI\nHALT\n", xy[0], xy[1]) + mustZ80Asm(mir2.Z80Codegen(m, ar))
				r := fix1Execute(t, asm)
				truth := false
				switch cond {
				case mir2.CmpEq:
					truth = xy[0] == xy[1]
				case mir2.CmpNe:
					truth = xy[0] != xy[1]
				case mir2.CmpUlt:
					truth = xy[0] < xy[1]
				case mir2.CmpUle:
					truth = xy[0] <= xy[1]
				case mir2.CmpUgt:
					truth = xy[0] > xy[1]
				case mir2.CmpUge:
					truth = xy[0] >= xy[1]
				}
				want := uint16(0)
				if truth {
					want = 2
				}
				extra := byte(0)
				if xy[0] != xy[1] {
					extra = 1
				}
				if r.HL != want || r.A != extra {
					t.Fatalf("HL%d want%d A%d want%d\n%s", r.HL, want, r.A, extra, asm)
				}
			})
		}
	}
}
