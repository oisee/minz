package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

// A constant's allocation is not evidence that A contains its value: folded
// bitwise constants have no LD, and a preceding ALU result may still be in A.
func TestZ80ByteConstantAccumulatorAllocation(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpAdd, mir2.OpSub, mir2.OpAnd, mir2.OpOr, mir2.OpXor} {
		for _, loc := range []string{"A", "C"} {
			for _, x := range []byte{0, 1, 14, 15, 16, 127, 128, 254, 255} {
				for _, k := range []byte{0, 1, 15, 16, 255} {
					t.Run(fmt.Sprintf("%s_%s_%d_%d", op, loc, x, k), func(t *testing.T) {
						m := &mir2.Module{Name: "constant"}
						f := m.AddFunc("constant")
						b := mir2.NewBuilder(f)
						b.SwitchToNewBlock("entry")
						input := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
						one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
						next := b.Add(input, one, mir2.TyU8, mir2.ClassGeneral)
						c := b.Const(int64(k), mir2.TyU8, mir2.ClassGeneral)
						out := b.BinOp(op, next, c, mir2.TyU8, mir2.ClassAcc)
						b.Ret(out)
						f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
						// C is shared only when the constant is suppressed; ADD/SUB must
						// materialize it, so keep their operand allocation separate.
						cLoc := loc
						if loc == "C" && (op == mir2.OpAdd || op == mir2.OpSub) {
							cLoc = "E"
						}
						ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{input: {Name: "D"}, one: {Name: "E"}, next: {Name: "C"}, c: {Name: cLoc}, out: {Name: "A"}}}
						asm := "ORG 0x8000\nLD SP,0xFF00\n" + fmt.Sprintf("LD D,%d\n", x) + "CALL constant\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
						n := x + 1
						var want byte
						switch op {
						case mir2.OpAdd:
							want = n + k
						case mir2.OpSub:
							want = n - k
						case mir2.OpAnd:
							want = n & k
						case mir2.OpOr:
							want = n | k
						case mir2.OpXor:
							want = n ^ k
						}
						if got := fix1Execute(t, asm).A; got != want {
							t.Fatalf("got%d want%d\n%s", got, want, asm)
						}
					})
				}
			}
		}
	}
}
