package mir2

import (
	"strings"
	"testing"
)

func TestSignedOperandWidthInvariant(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpSDiv, OpSMod, OpAnd, OpOr, OpXor, OpCmp} {
		t.Run(op.String(), func(t *testing.T) {
			m := &Module{Name: "bad_width"}
			f := m.AddFunc("bad")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			a := b.Const(-2, TyI8, ClassGeneral)
			c := b.Const(0, TyI16, ClassPointer)
			inst := &Inst{Dst: f.AllocReg(), Op: op, Ty: TyI16, Src: [2]Reg{c, a}, SrcTy: TyI16}
			b.Cur.Insts = append(b.Cur.Insts, inst)
			b.Ret(c)
			if err := Verify(m); err == nil || !strings.Contains(err.Error(), "explicit extension") {
				t.Fatalf("Verify got %v, want explicit extension diagnostic", err)
			}
			if _, err := Z80Codegen(m, &AllocResult{}); err == nil || !strings.Contains(err.Error(), "explicit extension") {
				t.Fatalf("Z80Codegen got %v, want explicit extension diagnostic", err)
			}
		})
	}
}
