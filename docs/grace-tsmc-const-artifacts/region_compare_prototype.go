package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// ApplyRegionComparePrototype is a manual opt-in, exact-source prototype.
// It is not registered in Grace or any production profile API.
func ApplyRegionComparePrototype(m *Module, name string) error {
	if err := Verify(m); err != nil {
		return err
	}
	f := m.FuncByName(name)
	if f == nil {
		return fmt.Errorf("missing kernel")
	}
	src := Var("src", mir2.TyPtr)
	arena := Var("arena", mir2.TyPtr)
	start := Var("src_start", mir2.TyU16)
	offset := Var("arena_ptr", mir2.TyU16)
	n := Var("len", mir2.TyU8)
	i := Var("i", mir2.TyU8)
	idx := func(base, off Expr) Expr { return &IndexExpr{Base: base, Idx: off, ElemTy: mir2.TyU8} }
	expected := Blk(Decl("i", mir2.TyU8, U8(0)), While(Lt(i, n), Blk(If(Ne(idx(src, Add(start, i, mir2.TyU16)), idx(arena, Add(Add(offset, U8(1), mir2.TyU16), i, mir2.TyU16))), Blk(&ReturnStmt{Val: U8(0)}), nil), Assign(i, Add(i, U8(1), mir2.TyU8)))), &ReturnStmt{Val: U8(1)})
	// Exact structural source check; no inferred call effects.
	if !reflect.DeepEqual(f.Body, expected) {
		return fmt.Errorf("not exact source body")
	}
	if f.IsExtern || f.RetTy != mir2.TyU8 || len(f.RetTys) != 0 || len(f.Captures) != 0 || len(f.Params) != 5 || f.Body == nil || len(f.Body.Body) != 3 {
		return fmt.Errorf("not exact region shape")
	}
	names := []string{"src", "arena", "src_start", "arena_ptr", "len"}
	tys := []mir2.Ty{mir2.TyPtr, mir2.TyPtr, mir2.TyU16, mir2.TyU16, mir2.TyU8}
	for j, p := range f.Params {
		if p.Name != names[j] || p.Ty != tys[j] || p.SMC {
			return fmt.Errorf("metadata")
		}
	}

	loop, ok := f.Body.Body[1].(*WhileStmt)
	if !ok || loop.Body == nil || len(loop.Body.Body) != 2 {
		return fmt.Errorf("loop")
	}
	// Refuse generated binding collisions with all source module names.
	for _, g := range m.Globals {
		if g.Name == "__region_left" || g.Name == "__region_right" || g.Name == "__region_elem" || g.Name == "__region_limit" || g.Name == "__region_right_limit" {
			return fmt.Errorf("reserved collision")
		}
	}

	width := Cast(n, mir2.TyU16)
	limit := Var("__region_limit", mir2.TyU16)
	rightLimit := Var("__region_right_limit", mir2.TyU16)
	and := func(a, b Expr) Expr { return &BinExpr{Op: "&&", L: a, R: b, Ty: mir2.TyBool} }
	guard := and(Le(start, limit), and(Le(Cast(src, mir2.TyU16), Sub(limit, start, mir2.TyU16)), and(Le(offset, rightLimit), Le(Cast(arena, mir2.TyU16), Sub(rightLimit, offset, mir2.TyU16)))))
	lp, rp := Var("__region_left", mir2.TyPtr), Var("__region_right", mir2.TyPtr)
	body := Blk(If(Ne(&LoadExpr{Ptr: lp, Ty: mir2.TyU8}, &LoadExpr{Ptr: rp, Ty: mir2.TyU8}), Blk(&ReturnStmt{Val: U8(0)}), nil), Assign(lp, Add(lp, U16(1), mir2.TyPtr)), Assign(rp, Add(rp, U16(1), mir2.TyPtr)))
	loopFast := &ForRangeStmt{Var: "__region_elem", Start: U8(0), End: n, Body: body}
	fast := Blk(Decl("__region_left", mir2.TyPtr, Add(src, start, mir2.TyPtr)), Decl("__region_right", mir2.TyPtr, Add(arena, Add(offset, U16(1), mir2.TyU16), mir2.TyPtr)), loopFast)
	f.Body.Body[1] = Blk(Decl("__region_limit", mir2.TyU16, Sub(U16(65535), width, mir2.TyU16)), Decl("__region_right_limit", mir2.TyU16, Sub(Var("__region_limit", mir2.TyU16), U16(1), mir2.TyU16)), If(guard, fast, Blk(loop)))
	return Verify(m)
}
