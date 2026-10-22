package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

const RegionCompareRule = "hir-guarded-region-compare"

type regionComparePlan struct {
	loop                                     *WhileStmt
	index, left, right, start, offset, count string
	names                                    [5]string
}

// Recognize one whole-function, ordered, read-only byte comparison. Parameter
// values may alias; their bindings and offsets cannot be changed by this body.
func findRegionCompare(f *Func, reserved map[string]bool) *regionComparePlan {
	if f == nil || f.IsExtern || f.IsIO || f.ExternAddr != 0 || len(f.ExternClobbers) != 0 || len(f.Captures) != 0 || f.RetTy != mir2.TyU8 || len(f.RetTys) != 0 || len(f.Params) != 5 || f.Body == nil || len(f.Body.Body) != 3 {
		return nil
	}
	tys := []mir2.Ty{mir2.TyPtr, mir2.TyPtr, mir2.TyU16, mir2.TyU16, mir2.TyU8}
	seen := map[string]bool{}
	for j, p := range f.Params {
		if p.Name == "" || p.Ty != tys[j] || p.SMC || p.RegClass != 0 || seen[p.Name] || reserved[p.Name] {
			return nil
		}
		seen[p.Name] = true
	}
	d, ok := f.Body.Body[0].(*VarDeclStmt)
	if !ok || !plainGraceU8Zero(d) || seen[d.Name] || reserved[d.Name] {
		return nil
	}
	loop, ok := f.Body.Body[1].(*WhileStmt)
	if !ok {
		return nil
	}
	p := &regionComparePlan{loop: loop, index: d.Name, left: f.Params[0].Name, right: f.Params[1].Name, start: f.Params[2].Name, offset: f.Params[3].Name, count: f.Params[4].Name}
	i, n := Var(p.index, mir2.TyU8), Var(p.count, mir2.TyU8)
	idx := func(base string, off Expr) Expr {
		return &IndexExpr{Base: Var(base, mir2.TyPtr), Idx: off, ElemTy: mir2.TyU8}
	}
	expected := Blk(Decl(p.index, mir2.TyU8, sourceU8(0)), While(Lt(i, n), Blk(If(Ne(idx(p.left, Add(Var(p.start, mir2.TyU16), i, mir2.TyU16)), idx(p.right, Add(Add(Var(p.offset, mir2.TyU16), sourceU8(1), mir2.TyU16), i, mir2.TyU16))), Blk(&ReturnStmt{Val: sourceU8(0)}), nil), Assign(i, Add(i, sourceU8(1), mir2.TyU8)))), &ReturnStmt{Val: sourceU8(1)})
	if !reflect.DeepEqual(f.Body, expected) {
		return nil
	}
	used := map[string]bool{}
	for n, v := range reserved {
		used[n] = v
	}
	for n := range seen {
		used[n] = true
	}
	used[p.index] = true
	for j := range p.names {
		p.names[j] = freshGracePointerName(f, used)
		used[p.names[j]] = true
	}
	return p
}

func (p *regionComparePlan) rewrite() (*Block, *ForRangeStmt) {
	i, n := Var(p.index, mir2.TyU8), Var(p.count, mir2.TyU8)
	start, off := Var(p.start, mir2.TyU16), Var(p.offset, mir2.TyU16)
	l, r := Var(p.left, mir2.TyPtr), Var(p.right, mir2.TyPtr)
	limit, rightLimit := Var(p.names[0], mir2.TyU16), Var(p.names[1], mir2.TyU16)
	lp, rp := Var(p.names[2], mir2.TyPtr), Var(p.names[3], mir2.TyPtr)
	and := func(a, b Expr) Expr { return &BinExpr{Op: "&&", L: a, R: b, Ty: mir2.TyBool} }
	guard := and(Le(start, limit), and(Le(Cast(l, mir2.TyU16), Sub(limit, start, mir2.TyU16)), and(Le(off, rightLimit), Le(Cast(r, mir2.TyU16), Sub(rightLimit, off, mir2.TyU16)))))
	tail := &ForRangeStmt{Var: p.names[4], Start: U8(1), End: n, Body: Blk(If(Ne(&LoadExpr{Ptr: lp, Ty: mir2.TyU8}, &LoadExpr{Ptr: rp, Ty: mir2.TyU8}), Blk(&ReturnStmt{Val: U8(0)}), nil), Assign(lp, Add(lp, U16(1), mir2.TyPtr)), Assign(rp, Add(rp, U16(1), mir2.TyPtr)))}
	fast := Blk(Decl(p.names[2], mir2.TyPtr, Add(l, Add(start, U16(1), mir2.TyU16), mir2.TyPtr)), Decl(p.names[3], mir2.TyPtr, Add(r, Add(off, U16(2), mir2.TyU16), mir2.TyPtr)), tail)
	firstL := &IndexExpr{Base: l, Idx: start, ElemTy: mir2.TyU8}
	firstR := &IndexExpr{Base: r, Idx: Add(off, U8(1), mir2.TyU16), ElemTy: mir2.TyU8}
	return Blk(If(Eq(n, U8(0)), Blk(&ReturnStmt{Val: U8(1)}), nil), If(Ne(firstL, firstR), Blk(&ReturnStmt{Val: U8(0)}), nil), Assign(i, U8(1)), If(Eq(n, U8(1)), Blk(&ReturnStmt{Val: U8(1)}), nil), Decl(p.names[0], mir2.TyU16, Sub(U16(65535), Cast(n, mir2.TyU16), mir2.TyU16)), Decl(p.names[1], mir2.TyU16, Sub(limit, U16(1), mir2.TyU16)), If(guard, fast, Blk(p.loop))), tail
}
