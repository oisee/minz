package hir_test

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestLexicalBlocksVMZ80(t *testing.T) {
	v := func(n string) hir.Expr { return hir.Var(n, mir2.TyU16) }
	assign := func(n string, e hir.Expr) hir.Stmt { return &hir.AssignStmt{Target: v(n), Val: e} }
	add := func(a hir.Expr, n int64) hir.Expr { return &hir.BinExpr{Op: "+", L: a, R: hir.U16(n), Ty: mir2.TyU16} }
	cases := []struct {
		name string
		body *hir.Block
		want int64
	}{
		{"shadow", hir.Blk(hir.Blk(hir.Decl("x", mir2.TyU8, hir.U8(99))), &hir.ReturnStmt{Val: v("x")}), 7},
		{"outer_assignment", hir.Blk(hir.Blk(assign("x", add(v("x"), 2))), &hir.ReturnStmt{Val: v("x")}), 9},
		{"assignment_before_shadow", hir.Blk(hir.Blk(assign("x", add(v("x"), 2)), hir.Decl("x", mir2.TyU16, hir.U16(99)), assign("x", hir.U16(88))), &hir.ReturnStmt{Val: v("x")}), 9},
		{"nested", hir.Blk(hir.Blk(hir.Decl("x", mir2.TyU16, hir.U16(99)), hir.Blk(hir.Decl("x", mir2.TyU8, hir.U8(3))), assign("x", hir.U16(88))), &hir.ReturnStmt{Val: v("x")}), 7},
		{"if_shadow", hir.Blk(&hir.IfStmt{Cond: &hir.BoolLitExpr{Val: true}, Then: hir.Blk(hir.Decl("x", mir2.TyU8, hir.U8(99)), assign("x", hir.U16(88)))}, &hir.ReturnStmt{Val: v("x")}), 7},
		{"if_assignment", hir.Blk(&hir.IfStmt{Cond: &hir.BoolLitExpr{Val: true}, Then: hir.Blk(assign("x", add(v("x"), 2)))}, &hir.ReturnStmt{Val: v("x")}), 9},
		{"loop_shadow", hir.Blk(hir.Decl("i", mir2.TyU16, hir.U16(0)), &hir.WhileStmt{Cond: &hir.BinExpr{Op: "<", L: v("i"), R: hir.U16(3), Ty: mir2.TyBool}, Body: hir.Blk(hir.Decl("x", mir2.TyU8, hir.U8(99)), assign("i", add(v("i"), 1)))}, &hir.ReturnStmt{Val: v("x")}), 7},
		{"loop_assignment", hir.Blk(hir.Decl("i", mir2.TyU16, hir.U16(0)), &hir.WhileStmt{Cond: &hir.BinExpr{Op: "<", L: v("i"), R: hir.U16(3), Ty: mir2.TyBool}, Body: hir.Blk(assign("x", add(v("x"), 2)), assign("i", add(v("i"), 1)))}, &hir.ReturnStmt{Val: v("x")}), 13},
	}
	alias := func(fn string) hir.Stmt { return hir.Decl("cb", mir2.TyU16, hir.Var(fn, mir2.TyU16)) }
	callAlias := func() hir.Expr { return &hir.CallExpr{Fn: "cb", Args: []hir.Expr{v("x")}, Ty: mir2.TyU16} }
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"alias_shadow", hir.Blk(alias("plus_one"), hir.Blk(alias("plus_two"), &hir.ExprStmt{Expr: callAlias()}), &hir.ReturnStmt{Val: callAlias()}), 8})
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"alias_initializer", hir.Blk(alias("plus_one"), hir.Blk(hir.Decl("cb", mir2.TyU16, hir.Var("cb", mir2.TyU16)), &hir.ExprStmt{Expr: callAlias()}), &hir.ReturnStmt{Val: callAlias()}), 8})
	cases = append(cases,
		struct {
			name string
			body *hir.Block
			want int64
		}{"break_shadow", hir.Blk(&hir.WhileStmt{Cond: &hir.BoolLitExpr{Val: true}, Body: hir.Blk(assign("x", add(v("x"), 2)), hir.Decl("x", mir2.TyU16, hir.U16(99)), &hir.BreakStmt{})}, &hir.ReturnStmt{Val: v("x")}), 9},
		struct {
			name string
			body *hir.Block
			want int64
		}{"range_shadow", hir.Blk(&hir.ForRangeStmt{Var: "x", Start: hir.U16(0), End: hir.U16(3), Body: hir.Blk(hir.Decl("x", mir2.TyU8, hir.U8(99)))}, &hir.ReturnStmt{Val: v("x")}), 7},
		struct {
			name string
			body *hir.Block
			want int64
		}{"range_continue_shadow", hir.Blk(&hir.ForRangeStmt{Var: "x", Start: hir.U16(0), End: hir.U16(3), Body: hir.Blk(hir.Decl("x", mir2.TyU8, hir.U8(99)), &hir.ContinueStmt{})}, &hir.ReturnStmt{Val: v("x")}), 7},
	)
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"range_outer_bound", hir.Blk(hir.Decl("sum", mir2.TyU16, hir.U16(0)), &hir.ForRangeStmt{Var: "x", Start: hir.U16(0), End: v("x"), Body: hir.Blk(assign("sum", add(v("sum"), 1)))}, &hir.ReturnStmt{Val: &hir.BinExpr{Op: "+", L: v("x"), R: v("sum"), Ty: mir2.TyU16}}), 14})
	for _, wide := range []bool{false, true} {
		var length hir.Expr = hir.U8(3)
		suffix := "byte"
		if wide {
			length = hir.U16(3)
			suffix = "word"
		}
		cases = append(cases, struct {
			name string
			body *hir.Block
			want int64
		}{"foreach_shadow_" + suffix, hir.Blk(&hir.VarDeclStmt{Name: "buf", Ty: mir2.TyU8, ArrayLen: 3, Initial: []hir.Expr{hir.U8(1), hir.U8(2), hir.U8(3)}}, &hir.ForEachStmt{Var: "x", Ptr: hir.Var("buf", mir2.TyPtr), Len: length, Start: hir.U16(0), ElemTy: mir2.TyU8, Body: hir.Blk()}, &hir.ReturnStmt{Val: v("x")}), 7})
	}
	cases = append(cases,
		struct {
			name string
			body *hir.Block
			want int64
		}{"return_shadow", hir.Blk(hir.Blk(hir.Decl("x", mir2.TyU16, hir.U16(99)), &hir.ReturnStmt{Val: v("x")}), &hir.ReturnStmt{Val: v("x")}), 99},
		struct {
			name string
			body *hir.Block
			want int64
		}{"scalar_hides_alias", hir.Blk(alias("plus_one"), hir.Blk(hir.Decl("cb", mir2.TyU16, callAlias()), &hir.ExprStmt{Expr: hir.Var("cb", mir2.TyU16)}), &hir.ReturnStmt{Val: callAlias()}), 8},
		struct {
			name string
			body *hir.Block
			want int64
		}{"tuple_shadow", hir.Blk(hir.Blk(&hir.TupleLetStmt{Names: []string{"x", "z"}, Tys: []mir2.Ty{mir2.TyU16, mir2.TyU16}, Call: &hir.CallExpr{Fn: "pair", Ty: mir2.TyU16}}), &hir.ReturnStmt{Val: v("x")}), 7},
	)
	arrayIndex := func() hir.Expr {
		return &hir.IndexExpr{Base: hir.Var("buf", mir2.TyPtr), Idx: hir.U16(0), ElemTy: mir2.TyU8}
	}
	cases = append(cases,
		struct {
			name string
			body *hir.Block
			want int64
		}{"array_shadow_array", hir.Blk(&hir.VarDeclStmt{Name: "buf", Ty: mir2.TyU8, ArrayLen: 1, Initial: []hir.Expr{hir.U8(7)}}, hir.Blk(&hir.VarDeclStmt{Name: "buf", Ty: mir2.TyU8, ArrayLen: 1, Initial: []hir.Expr{hir.U8(99)}}), &hir.ReturnStmt{Val: &hir.CastExpr{X: arrayIndex(), Ty: mir2.TyU16}}), 7},
		struct {
			name string
			body *hir.Block
			want int64
		}{"array_shadow_scalar", hir.Blk(&hir.VarDeclStmt{Name: "buf", Ty: mir2.TyU8, ArrayLen: 1, Initial: []hir.Expr{hir.U8(7)}}, hir.Blk(hir.Decl("buf", mir2.TyU16, hir.U16(99))), &hir.ReturnStmt{Val: &hir.CastExpr{X: arrayIndex(), Ty: mir2.TyU16}}), 7},
	)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &hir.Func{Name: "scope", RetTy: mir2.TyU16, Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, Body: c.body}
			m := &hir.Module{Funcs: []*hir.Func{f,
				{Name: "plus_one", RetTy: mir2.TyU16, Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, Body: hir.Blk(&hir.ReturnStmt{Val: add(v("x"), 1)})},
				{Name: "pair", RetTy: mir2.TyU16, RetTys: []mir2.Ty{mir2.TyU16, mir2.TyU16}, Body: hir.Blk(&hir.ReturnStmt{Vals: []hir.Expr{hir.U16(99), hir.U16(3)}})},
				{Name: "plus_two", RetTy: mir2.TyU16, Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, Body: hir.Blk(&hir.ReturnStmt{Val: add(v("x"), 2)})},
			}}
			if e := hir.Verify(m); e != nil {
				t.Fatal(e)
			}
			mm := hir.LowerModule(m)
			got, e := mir2.NewVM(mm).Call("scope", []mir2.Value{{I: 7}})
			if e != nil || len(got) != 1 || got[0].I != c.want {
				t.Fatalf("VM%v want%d err%v", got, c.want, e)
			}
			mm.RenumberRegs()
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
			for _, mf := range mm.Funcs {
				a := mir2.PBQPAllocate(mf, mir2.ComputeLiveness(mf), mir2.Z80CostTable{})
				for r, l := range a.Locs {
					ar.Locs[r] = l
				}
				ar.Spilled = append(ar.Spilled, a.Spilled...)
			}
			param := mm.FuncByName("scope").Contract.Params[0].Reg
			paramHome := ar.Loc(param).Name
			code, e := mir2.Z80Codegen(mm, ar)
			if e != nil {
				t.Fatal(e)
			}
			src := "ORG 0x8000\nLD SP,0xFF00\nLD " + paramHome + ",7\nCALL scope\nDI\nHALT\n" + code
			a, e := z80asm.NewAssembler().AssembleString(src)
			if e != nil || len(a.Errors) > 0 {
				t.Fatalf("asm%v%v\n%s", e, a.Errors, src)
			}
			z := emulator.NewRemogattoZ80()
			z.Reset()
			z.LoadMemory(0x8000, a.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00})
			for n := 0; !z.IsHalted(); n++ {
				if n > 100000 {
					t.Fatal("nohalt")
				}
				z.Step()
			}
			if x := z.GetRegisters().HL; x != uint16(c.want) {
				t.Fatalf("machine got%d want%d\n%s", x, c.want, src)
			}
		})
	}
}
