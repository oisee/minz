package mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// A load through DE stages the byte in A before copying it to C. Reusing C
// for a constant must break that copy alias before the next accumulator op.
func TestZ80CodegenConstantReplacesAccumulatorAlias(t *testing.T) {
	for _, tc := range []struct {
		op   mir2.Op
		want byte
	}{
		{mir2.OpAdd, 11},
		{mir2.OpSub, 247},
		{mir2.OpAnd, 0},
		{mir2.OpOr, 11},
		{mir2.OpXor, 11},
	} {
		t.Run(tc.op.String(), func(t *testing.T) {
			m := &mir2.Module{Name: "const_alias"}
			f := m.AddFunc("arith")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			ptr := b.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
			rhs := b.Param("rhs", mir2.TyU8, mir2.ClassGeneral)
			old := b.Load(ptr, mir2.TyU8, mir2.ClassGeneral)
			b.Store(ptr, old, mir2.TyU8)
			one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
			result := b.BinOp(tc.op, one, rhs, mir2.TyU8, mir2.ClassAcc)
			b.Ret(result)
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{
				ptr:    {Kind: mir2.LocReg, Name: "DE"},
				rhs:    {Kind: mir2.LocReg, Name: "B"},
				old:    {Kind: mir2.LocReg, Name: "C"},
				one:    {Kind: mir2.LocReg, Name: "C"},
				result: {Kind: mir2.LocReg, Name: "A"},
			}}
			asm := "ORG 0x8000\nLD SP,0xFF00\nLD DE,input\nLD B,10\nCALL arith\nDI\nHALT\n" +
				mustZ80Asm(mir2.Z80Codegen(m, ar)) + "\ninput: DB 15\n"
			got := fix1Execute(t, asm).A
			if got != tc.want {
				t.Fatalf("constant lhs %s: got %d want %d\n%s", tc.op, got, tc.want, asm)
			}
		})
	}
}
