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
	sumTy                            mir2.Ty
	weight                           Expr
	loopIndex                        int
	sum, index, base, bound, element string
}

// Accept exact byte sums: u8 zero-initialized, or u16 with a literal/parameter
// seed and an explicit zero-extension of each byte. The weighted u16 form
// additionally accepts exactly sum += a+b for immutable u16 scalar operands.
// Only function-root loops qualify: the original loop in the generated fallback
// is nested and therefore cannot be versioned again on the next Grace round.
func findGracePointerWalk(f *Func, b *Block, reserved map[string]bool) *gracePointerWalk {
	if f.IsExtern || b == nil || b != f.Body || (len(b.Body) != 4 && len(b.Body) != 5) {
		return nil
	}
	sum, ok := b.Body[0].(*VarDeclStmt)
	if !ok || !plainGraceSumSeed(sum) {
		return nil
	}
	index, ok := b.Body[1].(*VarDeclStmt)
	if !ok || !plainGraceU8Zero(index) || index.Name == sum.Name {
		return nil
	}
	loopIndex := 2
	var hoisted *VarDeclStmt
	if len(b.Body) == 5 {
		loopIndex = 3
		hoisted, ok = b.Body[2].(*VarDeclStmt)
		if !ok || hoisted.Ty != mir2.TyU16 || hoisted.ArrayLen != 0 || hoisted.At != nil || len(hoisted.Initial) != 0 || hoisted.Name == sum.Name || hoisted.Name == index.Name {
			return nil
		}
	}
	loop, ok := b.Body[loopIndex].(*WhileStmt)
	if !ok || loop.Body == nil || (len(loop.Body.Body) != 2 && len(loop.Body.Body) != 3) {
		return nil
	}
	if _, ok := b.Body[loopIndex+1].(*ReturnStmt); !ok {
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
	// The weighted form has exactly one extra, pure u16 accumulation before
	// the byte load. It must remain in that order, including before a load trap.
	var weight Expr
	var weightProof *BinExpr
	accumIndex := 0
	if len(loop.Body.Body) == 3 {
		if sum.Ty != mir2.TyU16 {
			return nil
		}
		weighted, ok := loop.Body.Body[0].(*AssignStmt)
		if !ok || !graceRef(weighted.Target, sum.Name, mir2.TyU16) {
			return nil
		}
		outer, ok := weighted.Val.(*BinExpr)
		if !ok || outer.Op != "+" || outer.Ty != mir2.TyU16 || !graceRef(outer.L, sum.Name, mir2.TyU16) {
			return nil
		}
		weight = outer.R
		if hoisted != nil {
			if !graceRef(weight, hoisted.Name, mir2.TyU16) {
				return nil
			}
			weightProof, ok = hoisted.Init.(*BinExpr)
		} else {
			weightProof, ok = weight.(*BinExpr)
		}
		if !ok || weightProof.Op != "+" || weightProof.Ty != mir2.TyU16 {
			return nil
		}
		accumIndex = 1
	}
	if hoisted != nil && weight == nil {
		return nil
	}
	accum, ok := loop.Body.Body[accumIndex].(*AssignStmt)
	if !ok || !graceRef(accum.Target, sum.Name, sum.Ty) {
		return nil
	}
	add, ok := accum.Val.(*BinExpr)
	if !ok || add.Op != "+" || add.Ty != sum.Ty || !graceRef(add.L, sum.Name, sum.Ty) {
		return nil
	}
	elementExpr := add.R
	if sum.Ty == mir2.TyU16 {
		extension, ok := elementExpr.(*CastExpr)
		if !ok || extension.X == nil || extension.Ty != mir2.TyU16 || extension.X.ExprTy() != mir2.TyU8 {
			return nil
		}
		elementExpr = extension.X
	}
	load, ok := elementExpr.(*IndexExpr)
	if !ok || load.ElemTy != mir2.TyU8 || (load.ElemStride != 0 && load.ElemStride != 1) || !graceRef(load.Idx, index.Name, mir2.TyU8) {
		return nil
	}
	base, ok := load.Base.(*VarRefExpr)
	if !ok || base.Ty != mir2.TyPtr {
		return nil
	}
	inc, ok := loop.Body.Body[accumIndex+1].(*AssignStmt)
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
	if weight != nil {
		for _, operand := range []Expr{weightProof.L, weightProof.R} {
			switch x := operand.(type) {
			case *VarRefExpr:
				if x.Ty != mir2.TyU16 || params[x.Name] != mir2.TyU16 || reserved[x.Name] || x.Name == sum.Name || x.Name == index.Name {
					return nil
				}
			case *IntLitExpr:
				if x.Ty != mir2.TyU16 || x.Val < 0 || x.Val > 65535 {
					return nil
				}
			default:
				return nil
			}
		}
	}
	if hoisted != nil {
		if reserved[hoisted.Name] || params[hoisted.Name] != nil {
			return nil
		}
		// A structural single-use binding proof, independent of the temp's
		// spelling or lineage. Arbitrary source prefixes do not qualify.
		uses := 0
		invalid := false
		walkGraceBlocks(f.Body, "", func(_ string, block *Block) {
			for _, stmt := range block.Body {
				if decl, ok := stmt.(*VarDeclStmt); ok && decl.Name == hoisted.Name && decl != hoisted {
					invalid = true
				}
				visitStmtExprs(stmt, func(expr Expr) {
					if ref, ok := expr.(*VarRefExpr); ok && ref.Name == hoisted.Name {
						uses++
					}
					if addr, ok := expr.(*AddrOfExpr); ok && addr.Sym == hoisted.Name {
						invalid = true
					}
				})
			}
		})
		if invalid || uses != 1 {
			return nil
		}
	}
	if seed, ok := sum.Init.(*VarRefExpr); ok {
		if params[seed.Name] != mir2.TyU16 || reserved[seed.Name] || seed.Name == index.Name {
			return nil
		}
	}
	if params[base.Name] != mir2.TyPtr || params[bound.Name] != mir2.TyU8 || params[sum.Name] != nil || params[index.Name] != nil {
		return nil
	}
	for _, name := range []string{sum.Name, index.Name, base.Name, bound.Name} {
		if reserved[name] {
			return nil
		}
	}
	// The complete prefix is an immutable seed initialization and zero index.
	// Its exact accumulator-update/increment body cannot mutate parameters or capture.
	element := freshGracePointerName(f, reserved)
	return &gracePointerWalk{loop: loop, sumTy: sum.Ty, weight: weight, loopIndex: loopIndex, sum: sum.Name, index: index.Name, base: base.Name, bound: bound.Name, element: element}
}
func plainGraceSumSeed(d *VarDeclStmt) bool {
	if d.Ty == mir2.TyU8 {
		return plainGraceU8Zero(d)
	}
	if d.Ty != mir2.TyU16 || d.ArrayLen != 0 || d.At != nil || len(d.Initial) != 0 {
		return false
	}
	switch seed := d.Init.(type) {
	case *IntLitExpr:
		return seed.Val >= 0 && seed.Val <= 65535 && (seed.Ty == mir2.TyU8 || seed.Ty == mir2.TyU16)
	case *VarRefExpr:
		return seed.Ty == mir2.TyU16
	}
	return false
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
	element := Expr(Var(p.element, mir2.TyU8))
	// Weighted bodies must perform their arithmetic before the potentially
	// trapping read. Pointer iteration defers that read into its original place.
	if p.weight != nil {
		element = &LoadExpr{Ptr: Var(p.element, mir2.TyPtr), Ty: mir2.TyU8}
	}
	if p.sumTy == mir2.TyU16 {
		element = Cast(element, mir2.TyU16)
	}
	body := Blk(Assign(Var(p.sum, p.sumTy), Add(Var(p.sum, p.sumTy), element, p.sumTy)))
	if p.weight != nil {
		body.Body = append([]Stmt{Assign(Var(p.sum, p.sumTy), Add(Var(p.sum, p.sumTy), p.weight, p.sumTy))}, body.Body...)
	}
	each := &ForEachStmt{Var: p.element, ElemTy: mir2.TyU8, ElemStride: 1, PtrIter: p.weight != nil,
		Ptr: Var(p.base, mir2.TyPtr), Start: U8(0), Len: Var(p.bound, mir2.TyU8), Body: body}
	fast := Blk(each, Assign(Var(p.index, mir2.TyU8), Var(p.bound, mir2.TyU8)))
	return If(guard, fast, Blk(p.loop))
}
