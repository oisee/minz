package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestLiteralStorePointerReuseBoundaries(t *testing.T) {
	for _, distinct := range []bool{false, true} {
		literal := func(address int64) hir.Expr { return &hir.IntLitExpr{Val: address, Ty: mir2.TyPtr} }
		second := int64(0x6000)
		if distinct {
			second++
		}
		value := &hir.BinExpr{Op: "^", Ty: mir2.TyU8,
			L: &hir.LoadExpr{Ptr: literal(0x6000), Ty: mir2.TyU8},
			R: &hir.LoadExpr{Ptr: literal(second), Ty: mir2.TyU8}}
		statements := []hir.Stmt{}
		for i := 0; i < 2; i++ {
			statements = append(statements, &hir.AssignStmt{Target: &hir.DerefExpr{Ptr: literal(0x6000), Ty: mir2.TyU8}, Val: value})
		}
		h := &hir.Module{Funcs: []*hir.Func{{Name: "modify", RetTy: mir2.TyVoid, Body: &hir.Block{Body: statements}}}}
		m := hir.LowerModule(h)
		constants, loads, stores := 0, 0, 0
		for _, b := range m.Funcs[0].Blocks {
			for _, instruction := range b.Insts {
				switch instruction.Op {
				case mir2.OpConst:
					constants++
				case mir2.OpLoad:
					loads++
				case mir2.OpStore:
					stores++
				}
			}
		}
		want := 2
		if distinct {
			want = 4
		}
		if constants != want || loads != 4 || stores != 2 {
			t.Fatalf("distinct=%v constants=%d want=%d loads=%d stores=%d", distinct, constants, want, loads, stores)
		}
	}
}

func TestLiteralStorePointerDoesNotCrossCalls(t *testing.T) {
	ptr := func() hir.Expr { return &hir.IntLitExpr{Val: 0x6000, Ty: mir2.TyPtr} }
	value := &hir.BinExpr{Op: "+", Ty: mir2.TyU8, L: &hir.LoadExpr{Ptr: ptr(), Ty: mir2.TyU8}, R: &hir.CallExpr{Fn: "effect", Ty: mir2.TyU8}}
	h := &hir.Module{Funcs: []*hir.Func{
		{Name: "modify", RetTy: mir2.TyVoid, Body: &hir.Block{Body: []hir.Stmt{&hir.AssignStmt{Target: &hir.DerefExpr{Ptr: ptr(), Ty: mir2.TyU8}, Val: value}}}},
		{Name: "effect", RetTy: mir2.TyU8, IsExtern: true, IsIO: true},
	}}
	m := hir.LowerModule(h)
	count := 0
	for _, b := range m.Funcs[0].Blocks {
		for _, inst := range b.Insts {
			if inst.Op == mir2.OpConst && inst.Ty == mir2.TyPtr {
				count++
			}
		}
	}
	if count != 2 {
		t.Fatalf("pointer reuse crossed an effectful call: constants=%d", count)
	}
}
