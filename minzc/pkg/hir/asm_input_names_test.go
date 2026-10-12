package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestASMInputNamesBeforePhysicalAliases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params []hir.Param
		inputs []string
		want   []int
	}{
		{"ordinary", []hir.Param{{Name: "x", Ty: mir2.TyU8}, {Name: "y", Ty: mir2.TyU8}}, []string{"x", "y"}, []int{0, 1}},
		{"named_register", []hir.Param{{Name: "x", Ty: mir2.TyU8, RegClass: mir2.ClassAcc}, {Name: "A", Ty: mir2.TyU8}}, []string{"A", "x"}, []int{1, 0}},
		{"physical_alias", []hir.Param{{Name: "x", Ty: mir2.TyU8, RegClass: mir2.ClassAcc}, {Name: "y", Ty: mir2.TyU8, RegClass: mir2.ClassRegC}}, []string{"A", "C"}, []int{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := []hir.AsmOperand{}
			for _, n := range tc.inputs {
				ops = append(ops, hir.AsmOperand{Name: n})
			}
			m := hir.LowerModule(&hir.Module{Funcs: []*hir.Func{{Name: "probe", Params: tc.params, RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Code: "NOP", Ins: ops, ClobberAll: true}, &hir.ReturnStmt{}}}}}})
			f := m.Funcs[0]
			i := f.Entry().Insts[0]
			if len(i.Asm.Ins) != len(tc.want) {
				t.Fatal(i.Asm.Ins)
			}
			for n, p := range tc.want {
				if i.Asm.Ins[n] != f.Contract.Params[p].Reg {
					t.Fatal("binding", i.Asm.Ins, f.Contract.Params)
				}
			}
		})
	}
}
