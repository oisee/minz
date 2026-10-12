package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestReturnPlacementIsExplicitAfterParameterReassignment(t *testing.T) {
	f := (&mir2.Module{}).AddFunc("identity")
	f.Contract.Params = []mir2.Param{{Reg: mir2.Reg(1), Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := f.NewBlock("entry")
	b.Params = []mir2.BlockParam{{Dst: mir2.Reg(1), Ty: mir2.TyU8}}
	b.Seal(&mir2.TermRet{Vals: []mir2.Reg{mir2.Reg(1)}})
	ops := appendReturnMove(nil, b, Z80, f)
	if len(ops) != 1 || ops[0].Op != OpMove || ops[0].Src[0] != 1 || ops[0].DstHint != Z80.LocSetByNames("A") || !ops[0].SrcHint[0].IsEmpty() {
		t.Fatalf("missing explicit solver return placement: %+v", ops)
	}
}
