package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// PointerWalkFact describes a guarded opportunity, not an assertion that the
// caller's pointer is in bounds. IndexMaximum is the type-derived bound;
// actual executed indices are 0 <= i < n. The runtime guard is necessary.
type PointerWalkFact struct {
	Function, Site, Index, Bound, Base string
	IndexMinimum, IndexMaximum         int64
	Stride                             int
	AddressMaximum                     int64
}

type gracePointerWalk struct {
	loop                             *WhileStmt
	sum, index, base, bound, element string
}

// Start with the exact scalar sum idiom emitted by mechanical translations.
// Only function-root loops qualify: the original loop in the generated fallback
// is nested and therefore cannot be versioned again on the next Grace round.
func findGracePointerWalk(f *Func, b *Block, reserved map[string]bool) *gracePointerWalk {
	if f.IsExtern || b == nil || b != f.Body || len(b.Body) != 4 {
		return nil
	}
	sum, ok := b.Body[0].(*VarDeclStmt)
	if !ok || !plainGraceU8Zero(sum) {
		return nil
	}
	index, ok := b.Body[1].(*VarDeclStmt)
	if !ok || !plainGraceU8Zero(index) || index.Name == sum.Name {
		return nil
	}
	loop, ok := b.Body[2].(*WhileStmt)
	if !ok || loop.Body == nil || len(loop.Body.Body) != 2 {
		return nil
	}
	if _, ok := b.Body[3].(*ReturnStmt); !ok {
		return nil
	}
	cond, ok := loop.Cond.(*BinExpr)
	if !ok || cond.Op != "<" || cond.Ty != mir2.TyBool || !graceRef(cond.L, index.Name, mir2.TyU8) {
		return nil
	}
	bound, ok := cond.R.(*VarRefExpr)
	if !ok || bound.Ty != mir2.TyU8 {
		return nil
	}
	accum, ok := loop.Body.Body[0].(*AssignStmt)
	if !ok || !graceRef(accum.Target, sum.Name, mir2.TyU8) {
		return nil
	}
	add, ok := accum.Val.(*BinExpr)
	if !ok || add.Op != "+" || add.Ty != mir2.TyU8 || !graceRef(add.L, sum.Name, mir2.TyU8) {
		return nil
	}
	load, ok := add.R.(*IndexExpr)
	if !ok || load.ElemTy != mir2.TyU8 || (load.ElemStride != 0 && load.ElemStride != 1) || !graceRef(load.Idx, index.Name, mir2.TyU8) {
		return nil
	}
	base, ok := load.Base.(*VarRefExpr)
	if !ok || base.Ty != mir2.TyPtr {
		return nil
	}
	inc, ok := loop.Body.Body[1].(*AssignStmt)
	if !ok || !graceRef(inc.Target, index.Name, mir2.TyU8) {
		return nil
	}
	next, ok := inc.Val.(*BinExpr)
	if !ok || next.Op != "+" || next.Ty != mir2.TyU8 || !graceRef(next.L, index.Name, mir2.TyU8) {
		return nil
	}
	one, ok := next.R.(*IntLitExpr)
	if !ok || one.Ty != mir2.TyU8 || one.Val != 1 {
		return nil
	}
	params := map[string]mir2.Ty{}
	for _, p := range f.Params {
		if _, exists := params[p.Name]; exists {
			return nil
		}
		params[p.Name] = p.Ty
	}
	if params[base.Name] != mir2.TyPtr || params[bound.Name] != mir2.TyU8 || params[sum.Name] != nil || params[index.Name] != nil {
		return nil
	}
	for _, name := range []string{sum.Name, index.Name, base.Name, bound.Name} {
		if reserved[name] {
			return nil
		}
	}
	// These inputs have no rebinding before the loop (the two zero declarations
	// are the whole prefix); its exact body cannot mutate parameters or capture.
	element := freshGracePointerName(f, reserved)
	return &gracePointerWalk{loop, sum.Name, index.Name, base.Name, bound.Name, element}
}
func plainGraceU8Zero(d *VarDeclStmt) bool {
	v, ok := d.Init.(*IntLitExpr)
	return ok && v.Val == 0 && v.Ty == mir2.TyU8 && d.Ty == mir2.TyU8 && d.ArrayLen == 0 && d.At == nil && len(d.Initial) == 0
}
func graceRef(e Expr, name string, ty mir2.Ty) bool {
	v, ok := e.(*VarRefExpr)
	return ok && v.Name == name && v.Ty == ty
}
func freshGracePointerName(f *Func, reserved map[string]bool) string {
	used := map[string]bool{}
	for name := range reserved {
		used[name] = true
	}
	for _, p := range f.Params {
		used[p.Name] = true
	}
	walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
		for _, s := range b.Body {
			switch x := s.(type) {
			case *VarDeclStmt:
				used[x.Name] = true
			case *TupleLetStmt:
				for _, n := range x.Names {
					used[n] = true
				}
			case *ForRangeStmt:
				used[x.Var] = true
			case *ForEachStmt:
				used[x.Var] = true
			}
			visitStmtExprs(s, func(e Expr) {
				switch x := e.(type) {
				case *VarRefExpr:
					used[x.Name] = true
				case *LetInExpr:
					used[x.Name] = true
				case *AddrOfExpr:
					used[x.Sym] = true
				}
			})
		}
	})
	for n := 0; ; n++ {
		name := fmt.Sprintf("__grace_element_%d", n)
		if !used[name] {
			return name
		}
	}
}

func (p *gracePointerWalk) versionedLoop() *IfStmt {
	// Subtraction is evaluated in u16. n is u8, so it cannot underflow; a u16
	// ptr+n test would wrap and incorrectly admit overflowing input addresses.
	limit := Sub(U16(65535), Cast(Var(p.bound, mir2.TyU8), mir2.TyU16), mir2.TyU16)
	guard := Le(Cast(Var(p.base, mir2.TyPtr), mir2.TyU16), limit)
	each := &ForEachStmt{Var: p.element, ElemTy: mir2.TyU8, ElemStride: 1,
		Ptr: Var(p.base, mir2.TyPtr), Start: U8(0), Len: Var(p.bound, mir2.TyU8),
		Body: Blk(Assign(Var(p.sum, mir2.TyU8), Add(Var(p.sum, mir2.TyU8), Var(p.element, mir2.TyU8), mir2.TyU8))),
	}
	fast := Blk(each, Assign(Var(p.index, mir2.TyU8), Var(p.bound, mir2.TyU8)))
	return If(guard, fast, Blk(p.loop))
}
