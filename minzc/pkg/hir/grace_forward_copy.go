package hir

import "github.com/minz/minzc/pkg/mir2"

const ForwardCopyRule = "hir-guarded-forward-copy"

type graceForwardCopy struct {
	loop                                     *WhileStmt
	index, dst, src, bound, dstNext, element string
}

// Exact sequential byte copy, with immutable pointer/count parameters and no
// other effects. Aliasing is allowed: every iteration reads before its store.
func findGraceForwardCopy(f *Func, b *Block, reserved map[string]bool) *graceForwardCopy {
	if f.IsExtern || f.RetTy != mir2.TyU8 || len(f.RetTys) != 0 || len(f.Captures) != 0 || b == nil || b != f.Body || len(b.Body) != 3 {
		return nil
	}
	index, ok := b.Body[0].(*VarDeclStmt)
	if !ok || !plainGraceU8Zero(index) {
		return nil
	}
	loop, ok := b.Body[1].(*WhileStmt)
	if !ok || loop.Body == nil || len(loop.Body.Body) != 2 {
		return nil
	}
	ret, ok := b.Body[2].(*ReturnStmt)
	if !ok || !graceRef(ret.Val, index.Name, mir2.TyU8) || len(ret.Vals) != 0 {
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
	store, ok := loop.Body.Body[0].(*AssignStmt)
	if !ok {
		return nil
	}
	dst, ok := store.Target.(*IndexExpr)
	if !ok || dst.ElemTy != mir2.TyU8 || (dst.ElemStride != 0 && dst.ElemStride != 1) || !graceRef(dst.Idx, index.Name, mir2.TyU8) {
		return nil
	}
	src, ok := store.Val.(*IndexExpr)
	if !ok || src.ElemTy != mir2.TyU8 || (src.ElemStride != 0 && src.ElemStride != 1) || !graceRef(src.Idx, index.Name, mir2.TyU8) {
		return nil
	}
	dstBase, ok := dst.Base.(*VarRefExpr)
	if !ok || dstBase.Ty != mir2.TyPtr {
		return nil
	}
	srcBase, ok := src.Base.(*VarRefExpr)
	if !ok || srcBase.Ty != mir2.TyPtr {
		return nil
	}
	inc, ok := loop.Body.Body[1].(*AssignStmt)
	if !ok || !graceRef(inc.Target, index.Name, mir2.TyU8) {
		return nil
	}
	add, ok := inc.Val.(*BinExpr)
	if !ok || add.Op != "+" || add.Ty != mir2.TyU8 || !graceRef(add.L, index.Name, mir2.TyU8) || !sameGraceU8One(add.R) {
		return nil
	}
	params := map[string]mir2.Ty{}
	for _, p := range f.Params {
		if p.SMC {
			return nil
		}
		if params[p.Name] != nil {
			return nil
		}
		params[p.Name] = p.Ty
	}
	if len(f.Params) != 3 || params[dstBase.Name] != mir2.TyPtr || params[srcBase.Name] != mir2.TyPtr || params[bound.Name] != mir2.TyU8 || params[index.Name] != nil || dstBase.Name == srcBase.Name {
		return nil
	}
	for _, name := range []string{index.Name, dstBase.Name, srcBase.Name, bound.Name} {
		if reserved[name] {
			return nil
		}
	}
	fresh := freshGracePointerName(f, reserved)
	used := map[string]bool{}
	for n, v := range reserved {
		used[n] = v
	}
	used[fresh] = true
	return &graceForwardCopy{loop: loop, index: index.Name, dst: dstBase.Name, src: srcBase.Name, bound: bound.Name, element: fresh, dstNext: freshGracePointerName(f, used)}
}
func sameGraceU8One(e Expr) bool {
	v, ok := e.(*IntLitExpr)
	return ok && v.Ty == mir2.TyU8 && v.Val == 1
}

func (p *graceForwardCopy) versionedLoop() *IfStmt {
	limit := Sub(U16(65535), Cast(Var(p.bound, mir2.TyU8), mir2.TyU16), mir2.TyU16)
	srcGuard := Le(Cast(Var(p.src, mir2.TyPtr), mir2.TyU16), limit)
	dstGuard := Le(Cast(Var(p.dst, mir2.TyPtr), mir2.TyU16), limit)
	body := Blk(&StoreStmt{Ptr: Var(p.dstNext, mir2.TyPtr), Val: &LoadExpr{Ptr: Var(p.element, mir2.TyPtr), Ty: mir2.TyU8}}, Assign(Var(p.dstNext, mir2.TyPtr), Add(Var(p.dstNext, mir2.TyPtr), U16(1), mir2.TyPtr)))
	each := &ForEachStmt{Var: p.element, ElemTy: mir2.TyU8, ElemStride: 1, PtrIter: true, Ptr: Var(p.src, mir2.TyPtr), Start: U8(0), Len: Var(p.bound, mir2.TyU8), Body: body}
	fast := Blk(Decl(p.dstNext, mir2.TyPtr, Var(p.dst, mir2.TyPtr)), each, Assign(Var(p.index, mir2.TyU8), Var(p.bound, mir2.TyU8)))
	return If(&BinExpr{Op: "&&", L: srcGuard, R: dstGuard, Ty: mir2.TyBool}, fast, Blk(p.loop))
}
