package hir_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// A call's SSA operands must have the declared parameter width before inlining
// substitutes them into word operations or argument placement selects an ABI.
func TestCallUnsignedArgumentWidening(t *testing.T) {
	for _, composite := range []bool{false, true} {
		t.Run(map[bool]string{false: "variable", true: "expression"}[composite], func(t *testing.T) {
			arg := hir.Expr(hir.Var("a", mir2.TyU8))
			if composite {
				arg = &hir.BinExpr{Op: "^", L: arg, R: &hir.IntLitExpr{Val: 2, Ty: mir2.TyU8}, Ty: mir2.TyU8}
			}
			h := &hir.Module{Funcs: []*hir.Func{
				{Name: "callee", Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, RetTy: mir2.TyU16, Body: hir.Blk(hir.Ret(hir.Var("x", mir2.TyU16)))},
				{Name: "caller", Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}}, RetTy: mir2.TyU16, Body: hir.Blk(hir.Ret(&hir.CallExpr{Fn: "callee", Args: []hir.Expr{arg}, Ty: mir2.TyU16}))},
			}}
			m := hir.LowerModule(h)
			types := map[mir2.Reg]mir2.Ty{}
			for _, b := range m.Funcs[1].Blocks {
				for _, inst := range b.Insts {
					if inst.Dst != mir2.NoReg {
						types[inst.Dst] = inst.Ty
					}
					if inst.Op == mir2.OpCall && types[inst.Args[0]] != mir2.TyU16 {
						t.Fatalf("composite=%v: call argument has type %v, want u16", composite, types[inst.Args[0]])
					}
				}
			}
		})
	}
}
