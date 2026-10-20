package mir2_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestZ80NegPreservesLiveAccumulator(t *testing.T) {
	for _, dst := range []string{"A", "B"} {
		t.Run(dst, func(t *testing.T) {
			m := &mir2.Module{Name: "neg_live"}
			f := m.AddFunc("neg_live")
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
			y := b.Param("y", mir2.TyU8, mir2.ClassGeneral)
			old := b.BinOp(mir2.OpXor, x, y, mir2.TyU8, mir2.ClassAcc)
			neg := b.Neg(old, mir2.TyU8, mir2.ClassAcc)
			// Consume both values together to check that the original byte
			// and the negated result survive in their distinct homes.
			result := b.BinOp(mir2.OpXor, old, neg, mir2.TyU8, mir2.ClassAcc)
			b.Ret(result)
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
			// Simultaneously live values need distinct allocated homes.
			oldHome := "A"
			if dst == "A" {
				oldHome = "C"
			}
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
				x:      {Kind: mir2.LocReg, Name: "D"},
				y:      {Kind: mir2.LocReg, Name: "E"},
				old:    {Kind: mir2.LocReg, Name: oldHome},
				neg:    {Kind: mir2.LocReg, Name: dst},
				result: {Kind: mir2.LocReg, Name: "A"},
			}}
			code := mustZ80Asm(mir2.Z80Codegen(m, ar))
			for input := 0; input < 256; input++ {
				asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD D,%d\nLD E,39\nCALL neg_live\nDI\nHALT\n", input) + code
				value := byte(input) ^ 39
				if got, want := fix1Execute(t, asm).A, value^-value; got != want {
					t.Fatalf("input %d: got %d want %d\n%s", input, got, want, asm)
				}
			}
		})
	}
}
