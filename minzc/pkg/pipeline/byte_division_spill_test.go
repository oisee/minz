package pipeline

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Force a legal byte result into memory independently of allocator heuristics.
// The quotient used to bypass emitLD8 and emit the invalid LD spill,B.
func TestByteDivisionSpillVMZ80(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpDiv, mir2.OpMod, mir2.OpSDiv, mir2.OpSMod} {
		for _, home := range []string{"spill", "IYH", "A", "B"} {
			t.Run(fmt.Sprintf("%v/%s", op, home), func(t *testing.T) {
				m := &mir2.Module{}
				f := m.AddFunc("probe")
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				ty := mir2.TyU8
				if op == mir2.OpSDiv || op == mir2.OpSMod {
					ty = mir2.TyI8
				}
				x := b.Param("x", ty, mir2.ClassCounter)
				y := b.Param("y", ty, mir2.ClassRegC)
				out := b.BinOp(op, x, y, ty, mir2.ClassGeneral)
				b.Ret(out)
				f.Contract.Returns = []mir2.Return{{Ty: ty, Class: mir2.ClassAcc}}
				loc := mir2.PhysLoc{Kind: mir2.LocReg, Name: home}
				if home == "spill" {
					loc = mir2.PhysLoc{Kind: mir2.LocMem, Name: "mem", Offset: 0xf000}
				}
				if home == "IYH" {
					loc.Kind = mir2.LocIXY8
				}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
					x: {Kind: mir2.LocReg, Name: "B"}, y: {Kind: mir2.LocReg, Name: "C"}, out: loc,
				}}
				asm, err := mir2.Z80Codegen(m, ar)
				if err != nil {
					t.Fatal(err)
				}
				if home == "spill" && !strings.Contains(asm, "LD (_spill_") {
					t.Fatalf("missing spill store:\n%s", asm)
				}
				// HIR supplies the public signed result domain to the emulator checker.
				typename := "u8"
				if ty == mir2.TyI8 {
					typename = "i8"
				}
				h, err := nanz.Parse(fmt.Sprintf("fun probe(x:%s,y:%s)->%s { return x }", typename, typename, typename), "division.nanz")
				if err != nil {
					t.Fatal(err)
				}
				s := Steps{MIR2Module: m, Allocation: ar, Assembly: asm}
				for _, xv := range []int64{0, 1, 7, 127, 128, 129, 255} {
					for _, yv := range []int64{1, 2, 3, 7, 127, 128, 253, 255} {
						lhs, rhs := xv, yv
						if ty == mir2.TyI8 {
							lhs, rhs = int64(int8(xv)), int64(int8(yv))
						}
						want := lhs / rhs
						if op == mir2.OpMod || op == mir2.OpSMod {
							want = lhs % rhs
						}
						if ty == mir2.TyI8 {
							want = int64(int8(want)) // byte result wraps after signed division
						}
						got, err := mir2.NewVM(m).Call("probe", []mir2.Value{{I: xv}, {I: yv}})
						if err != nil || len(got) != 1 || got[0].I&255 != want&255 {
							t.Fatalf("%d/%d VM=%v err=%v want=%d", lhs, rhs, got, err, want)
						}
						measureGraceZ80Compiled(t, h, s, "probe", []int64{xv, yv}, nil, want, 10000)
					}
				}
			})
		}
	}
}

// Full signed-fuzz programs retain the register pressure and CFG shape that
// diamond commoning exposed. Compare raw/final MIR and assembled native code.
func TestDiamondDivisionSpillSeedsVMZ80(t *testing.T) {
	for _, seed := range []int{53, 233, 261, 415} {
		for _, grace := range []bool{false, true} {
			t.Run(fmt.Sprintf("seed%d/grace%v", seed, grace), func(t *testing.T) {
				data, err := os.ReadFile(fmt.Sprintf("testdata/o8b/seed-%d.nanz", seed))
				if err != nil {
					t.Fatal(err)
				}
				parse := func() *hir.Module {
					h, err := nanz.Parse(string(data), "seed.nanz")
					if err != nil {
						t.Fatal(err)
					}
					return h
				}
				h := parse()
				if len(h.Asserts) != 1 {
					t.Fatalf("seed fixture must have one direct-call assert, got %d", len(h.Asserts))
				}
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: grace, AssertMode: "none"})
				if err != nil {
					t.Fatal(err)
				}
				for _, a := range h.Asserts {
					args := make([]mir2.Value, len(a.Args))
					for i, v := range a.Args {
						args[i] = mir2.Value{I: v}
					}
					for _, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
						got, err := mir2.NewVM(m).Call(a.FuncName, args)
						if err != nil || len(got) != 1 || got[0].I&65535 != a.Expected {
							t.Fatalf("VM=%v err=%v want=%d", got, err, a.Expected)
						}
					}
					measureGraceZ80Compiled(t, h, s, a.FuncName, a.Args, nil, a.Expected, 100000)
				}
			})
		}
	}
}
