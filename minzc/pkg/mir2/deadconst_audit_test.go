package mir2

import "testing"

// Suppression must agree with the consuming path even for forced spill and
// wide allocations which a small HIR fixture rarely reaches.
func TestDeadConstsConsumerAudit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		op       Op
		ty       Ty
		lhs, dst string
		fold     bool
	}{
		{"add16_inc", OpAdd, TyU16, "HL", "HL", true},
		{"sub16_dec", OpSub, TyU16, "DE", "DE", true},
		{"add16_other", OpAdd, TyU16, "DE", "HL", false},
		{"sub16_spill", OpSub, TyU16, "$F000", "$F000", false},
		{"add32", OpAdd, TyU32, "HL", "HL", false},
		{"sub32", OpSub, TyU32, "HL", "HL", false},
		{"and32", OpAnd, TyU32, "HL", "HL", false},
		{"or32", OpOr, TyU32, "HL", "HL", false},
		{"xor32", OpXor, TyU32, "HL", "HL", false},
		{"div16", OpDiv, TyU16, "HL", "HL", false},
		{"mod16", OpMod, TyU16, "HL", "HL", false},
		{"sdiv16", OpSDiv, TyI16, "HL", "HL", false},
		{"smod16", OpSMod, TyI16, "HL", "HL", false},
		{"div16_pow2", OpDiv, TyU16, "HL", "HL", true},
		{"mod16_pow2", OpMod, TyU16, "HL", "HL", true},
		{"sdiv16_pow2", OpSDiv, TyI16, "HL", "HL", true},
		{"smod16_pow2", OpSMod, TyI16, "HL", "HL", true},
		{"cmp16", OpCmp, TyBool, "HL", "F", false},
		{"cmp8", OpCmp, TyBool, "A", "F", true},
		{"shl16", OpShl, TyU16, "HL", "HL", true},
		{"shr16", OpShr, TyU16, "HL", "HL", true},
		{"sar16", OpSar, TyI16, "HL", "HL", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			imm := int64(1)
			if tc.op == OpDiv || tc.op == OpMod || tc.op == OpSDiv || tc.op == OpSMod {
				imm = 3 // fallback consumes a register; powers of two consume immediates
				if tc.fold {
					imm = 2
				}
			}
			f := &Func{Blocks: []*Block{{Insts: []*Inst{{Op: OpConst, Dst: 2, Imm: imm, Ty: tc.ty}, {Op: tc.op, Dst: 3, Src: [2]Reg{1, 2}, Ty: tc.ty}}}}}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Name: tc.lhs}, 2: {Name: "BC"}, 3: {Name: tc.dst}}}
			if got := computeDeadConsts(f, ar)[2]; got != tc.fold {
				t.Fatalf("constant suppressed=%v want %v", got, tc.fold)
			}
		})
	}
}
