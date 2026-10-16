package c89

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// aggregateExpr/aggregateBlock deliberately reject unknown IR shapes. New HIR
// nodes must be audited before this pass may remove an addressable C object.
func aggregateExpr(e hir.Expr, f func(hir.Expr) (hir.Expr, bool)) (hir.Expr, bool) {
	if e == nil {
		return nil, true
	}
	e, ok := f(e)
	if !ok {
		return e, false
	}
	child := func(p *hir.Expr) {
		if ok {
			*p, ok = aggregateExpr(*p, f)
		}
	}
	switch x := e.(type) {
	case *hir.IntLitExpr, *hir.BoolLitExpr, *hir.VarRefExpr, *hir.ConstPtrExpr:
	case *hir.FieldExpr:
		child(&x.X)
	case *hir.AddrOfExpr:
		child(&x.X)
	case *hir.BinExpr:
		child(&x.L)
		child(&x.R)
	case *hir.UnaryExpr:
		child(&x.X)
	case *hir.CastExpr:
		child(&x.X)
	case *hir.LoadExpr:
		child(&x.Ptr)
	case *hir.DerefExpr:
		child(&x.Ptr)
	case *hir.IndexExpr:
		child(&x.Base)
		child(&x.Idx)
	case *hir.CallExpr:
		for i := range x.Args {
			child(&x.Args[i])
		}
	case *hir.CallIndirectExpr:
		child(&x.FnPtr)
		for i := range x.Args {
			child(&x.Args[i])
		}
	case *hir.StructLitExpr:
		for i := range x.Fields {
			child(&x.Fields[i].Val)
		}
	case *hir.CondExpr:
		child(&x.Cond)
		child(&x.Then)
		child(&x.Else)
	default:
		return e, false
	}
	return e, ok
}

func aggregateBlock(b *hir.Block, f func(hir.Expr) (hir.Expr, bool)) bool {
	if b == nil {
		return true
	}
	ok := true
	expr := func(p *hir.Expr) {
		if ok {
			*p, ok = aggregateExpr(*p, f)
		}
	}
	block := func(p *hir.Block) {
		if ok {
			ok = aggregateBlock(p, f)
		}
	}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *hir.Block:
			block(x)
		case *hir.VarDeclStmt:
			expr(&x.Init)
			for i := range x.Initial {
				expr(&x.Initial[i])
			}
		case *hir.AssignStmt:
			expr(&x.Target)
			expr(&x.Val)
		case *hir.ReturnStmt:
			expr(&x.Val)
			for i := range x.Vals {
				expr(&x.Vals[i])
			}
		case *hir.TupleLetStmt:
			var e hir.Expr = x.Call
			expr(&e)
			x.Call = e.(*hir.CallExpr)
		case *hir.ExprStmt:
			expr(&x.Expr)
		case *hir.IfStmt:
			expr(&x.Cond)
			block(x.Then)
			block(x.Else)
		case *hir.WhileStmt:
			expr(&x.Cond)
			block(x.Body)
		case *hir.ForRangeStmt:
			expr(&x.Start)
			expr(&x.End)
			block(x.Body)
		case *hir.SwitchStmt:
			expr(&x.Val)
			for _, c := range x.Cases {
				block(c.Body)
			}
			block(x.Default)
		case *hir.StoreStmt:
			expr(&x.Ptr)
			expr(&x.Val)
		case *hir.BreakStmt, *hir.ContinueStmt, *hir.LabelStmt, *hir.GotoStmt:
		default:
			return false
		}
		if !ok {
			return false
		}
	}
	return true
}

func scalarFields(st *mir2.StructTy, name string) map[string]*hir.VarRefExpr {
	result := make(map[string]*hir.VarRefExpr)
	for i, field := range st.Fields {
		result[field.Name] = hir.Var(fmt.Sprintf("__c89_scalar_%s_%d", name, i), field.Ty)
	}
	return result
}

// scalarObjectUses validates every use before removing storage. Internal C
// lvalue address temporaries are admitted only if all their uses dereference
// exactly that field. An escaping pointer, volatile access, or whole-object use
// keeps the original object and memory ABI.
func scalarObjectUses(b *hir.Block, decl *hir.VarDeclStmt, fields map[string]*hir.VarRefExpr, apply bool) bool {
	aliases := make(map[string]*hir.VarRefExpr)
	duplicate := false
	visitAggregateStmts(b, func(s hir.Stmt) {
		d, ok := s.(*hir.VarDeclStmt)
		if !ok {
			return
		}
		if d != decl && d.Name == decl.Name {
			duplicate = true
		}
		if d != decl {
			for _, v := range fields {
				if d.Name == v.Name {
					duplicate = true
				}
			}
		}
		if d != decl {
			if a, ok := d.Init.(*hir.AddrOfExpr); ok {
				if fe, ok := a.X.(*hir.FieldExpr); ok && !fe.Volatile {
					if r, ok := fe.X.(*hir.VarRefExpr); ok && r.Name == decl.Name && !r.Volatile {
						if v := fields[fe.Field]; v != nil {
							aliases[d.Name] = v
						}
					}
				}
			}
		}
	})
	if duplicate {
		return false
	}
	rewrite := func(e hir.Expr) (hir.Expr, bool) {
		switch x := e.(type) {
		case *hir.FieldExpr:
			if r, ok := x.X.(*hir.VarRefExpr); ok && r.Name == decl.Name {
				v := fields[x.Field]
				if v == nil || x.Volatile || r.Volatile {
					return e, false
				}
				// Substitute during validation too, to avoid visiting the object reference.
				return hir.Var(v.Name, v.Ty), true
			}
		case *hir.DerefExpr:
			if r, ok := x.Ptr.(*hir.VarRefExpr); ok && aliases[r.Name] != nil && !x.Volatile {
				v := aliases[r.Name]
				return hir.Var(v.Name, v.Ty), true
			}
		case *hir.LoadExpr:
			if r, ok := x.Ptr.(*hir.VarRefExpr); ok && aliases[r.Name] != nil && !x.Volatile {
				v := aliases[r.Name]
				return hir.Var(v.Name, v.Ty), true
			}
		case *hir.VarRefExpr:
			if x.Name == decl.Name || aliases[x.Name] != nil {
				return e, false
			}
			for _, v := range fields {
				if x.Name == v.Name {
					return e, false
				}
			}
		case *hir.AddrOfExpr:
			if x.Sym == decl.Name || aliases[x.Sym] != nil {
				return e, false
			}
		}
		return e, true
	}
	// Validate without mutating original expressions: use a dry visitor that
	// replaces accepted fields with a sentinel, not a scalar binding.
	validate := func(e hir.Expr) (hir.Expr, bool) {
		r, ok := rewrite(e)
		if !ok {
			return e, false
		}
		if r != e {
			return hir.U8(0), true
		}
		return e, true
	}
	visit := func(block *hir.Block) bool {
		if block == nil {
			return true
		}
		for _, s := range block.Body {
			if d, ok := s.(*hir.VarDeclStmt); ok && (d == decl || aliases[d.Name] != nil) {
				continue
			}
			// A shallow copy of a statement is insufficient: validation must not rewrite
			// nested nodes. Instead the expression walker below only inspects children.
			if !inspectAggregateStmt(s, validate) {
				return false
			}
		}
		return true
	}
	if !visit(b) {
		return false
	}
	if !apply {
		return true
	}
	var transform func(*hir.Block)
	transform = func(block *hir.Block) {
		if block == nil {
			return
		}
		var body []hir.Stmt
		for _, s := range block.Body {
			if d, ok := s.(*hir.VarDeclStmt); ok && aliases[d.Name] != nil {
				continue
			}
			switch x := s.(type) {
			case *hir.Block:
				transform(x)
			case *hir.IfStmt:
				transform(x.Then)
				transform(x.Else)
			case *hir.WhileStmt:
				transform(x.Body)
			case *hir.ForRangeStmt:
				transform(x.Body)
			case *hir.SwitchStmt:
				for _, c := range x.Cases {
					transform(c.Body)
				}
				transform(x.Default)
			}
			if s != decl {
				aggregateBlock(&hir.Block{Body: []hir.Stmt{s}}, func(e hir.Expr) (hir.Expr, bool) { r, _ := rewrite(e); return r, true })
			}
			body = append(body, s)
		}
		block.Body = body
	}
	transform(b)
	return true
}

func inspectAggregateExpr(e hir.Expr, f func(hir.Expr) (hir.Expr, bool)) bool {
	if e == nil {
		return true
	}
	next, ok := f(e)
	if !ok {
		return false
	}
	if next != e {
		return true
	}
	child := func(e hir.Expr) bool { return inspectAggregateExpr(e, f) }
	switch x := e.(type) {
	case *hir.IntLitExpr, *hir.BoolLitExpr, *hir.VarRefExpr, *hir.ConstPtrExpr:
		return true
	case *hir.FieldExpr:
		return child(x.X)
	case *hir.AddrOfExpr:
		return child(x.X)
	case *hir.BinExpr:
		return child(x.L) && child(x.R)
	case *hir.UnaryExpr:
		return child(x.X)
	case *hir.CastExpr:
		return child(x.X)
	case *hir.LoadExpr:
		return child(x.Ptr)
	case *hir.DerefExpr:
		return child(x.Ptr)
	case *hir.IndexExpr:
		return child(x.Base) && child(x.Idx)
	case *hir.CallExpr:
		for _, a := range x.Args {
			if !child(a) {
				return false
			}
		}
		return true
	case *hir.CallIndirectExpr:
		if !child(x.FnPtr) {
			return false
		}
		for _, a := range x.Args {
			if !child(a) {
				return false
			}
		}
		return true
	case *hir.StructLitExpr:
		for _, a := range x.Fields {
			if !child(a.Val) {
				return false
			}
		}
		return true
	case *hir.CondExpr:
		return child(x.Cond) && child(x.Then) && child(x.Else)
	}
	return false
}
func inspectAggregateStmt(s hir.Stmt, f func(hir.Expr) (hir.Expr, bool)) bool {
	e := func(e hir.Expr) bool { return inspectAggregateExpr(e, f) }
	b := func(b *hir.Block) bool {
		if b == nil {
			return true
		}
		for _, s := range b.Body {
			if !inspectAggregateStmt(s, f) {
				return false
			}
		}
		return true
	}
	switch x := s.(type) {
	case *hir.Block:
		return b(x)
	case *hir.VarDeclStmt:
		if !e(x.Init) {
			return false
		}
		for _, v := range x.Initial {
			if !e(v) {
				return false
			}
		}
		return true
	case *hir.AssignStmt:
		return e(x.Target) && e(x.Val)
	case *hir.ReturnStmt:
		if !e(x.Val) {
			return false
		}
		for _, v := range x.Vals {
			if !e(v) {
				return false
			}
		}
		return true
	case *hir.TupleLetStmt:
		return e(x.Call)
	case *hir.ExprStmt:
		return e(x.Expr)
	case *hir.IfStmt:
		return e(x.Cond) && b(x.Then) && b(x.Else)
	case *hir.WhileStmt:
		return e(x.Cond) && b(x.Body)
	case *hir.ForRangeStmt:
		return e(x.Start) && e(x.End) && b(x.Body)
	case *hir.SwitchStmt:
		if !e(x.Val) || !b(x.Default) {
			return false
		}
		for _, c := range x.Cases {
			if !b(c.Body) {
				return false
			}
		}
		return true
	case *hir.StoreStmt:
		return e(x.Ptr) && e(x.Val)
	case *hir.BreakStmt, *hir.ContinueStmt, *hir.LabelStmt, *hir.GotoStmt:
		return true
	}
	return false
}

func scalarizeAggregateLocals(b *hir.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *hir.Block:
			scalarizeAggregateLocals(x)
		case *hir.IfStmt:
			scalarizeAggregateLocals(x.Then)
			scalarizeAggregateLocals(x.Else)
		case *hir.WhileStmt:
			scalarizeAggregateLocals(x.Body)
		case *hir.ForRangeStmt:
			scalarizeAggregateLocals(x.Body)
		case *hir.SwitchStmt:
			for _, c := range x.Cases {
				scalarizeAggregateLocals(c.Body)
			}
			scalarizeAggregateLocals(x.Default)
		}
	}
	for i := 0; i < len(b.Body); i++ {
		d, ok := b.Body[i].(*hir.VarDeclStmt)
		if !ok || d.Volatile || d.ArrayLen != 0 || d.At != nil {
			continue
		}
		lit, ok := d.Init.(*hir.StructLitExpr)
		if !ok || !isShallowScalarStruct(lit.St) {
			continue
		}
		fields := scalarFields(lit.St, d.Name)
		tupleFields := false
		if i > 0 {
			if tuple, ok := b.Body[i-1].(*hir.TupleLetStmt); ok && len(tuple.Names) == len(lit.Fields) {
				tupleFields = true
				for j, fi := range lit.Fields {
					r, ok := fi.Val.(*hir.VarRefExpr)
					if !ok || r.Name != tuple.Names[j] {
						tupleFields = false
						break
					}
				}
				if tupleFields {
					for _, fi := range lit.Fields {
						r := fi.Val.(*hir.VarRefExpr)
						fields[fi.Name] = hir.Var(r.Name, r.Ty)
					}
				}
			}
		}
		if !tupleFields {
			used := map[string]bool{}
			inspectAggregateStmt(b, func(e hir.Expr) (hir.Expr, bool) {
				if r, ok := e.(*hir.VarRefExpr); ok {
					used[r.Name] = true
				}
				return e, true
			})
			walkStmts(b, func(s hir.Stmt) bool {
				if d, ok := s.(*hir.VarDeclStmt); ok {
					used[d.Name] = true
				}
				return true
			})
			for _, field := range lit.St.Fields {
				v := fields[field.Name]
				for used[v.Name] {
					v.Name += "_"
				}
				used[v.Name] = true
			}
		}
		if !scalarObjectUses(b, d, fields, false) {
			continue
		}
		scalarObjectUses(b, d, fields, true)
		// Preserve initialization order and evaluate each initializer once.
		var init []hir.Stmt
		for _, fi := range lit.Fields {
			if tupleFields {
				continue
			}
			v := fields[fi.Name]
			if v == nil {
				continue
			}
			init = append(init, &hir.VarDeclStmt{Name: v.Name, Ty: v.Ty, Init: fi.Val})
		}
		var body []hir.Stmt
		for _, s := range b.Body {
			if s == d {
				body = append(body, init...)
			} else {
				body = append(body, s)
			}
		}
		b.Body = body
	}
}

// Scalarize only direct, module-local call sites with already materialized
// arguments. This avoids duplicating calls or changing indirect/public ABI.
func scalarizeAggregateParams(m *hir.Module) {
	for _, fn := range m.Funcs {
		if fn.IsExtern || fn.Body == nil {
			continue
		}
		for pi := len(fn.Params) - 1; pi >= 0; pi-- {
			p := fn.Params[pi]
			var decl *hir.VarDeclStmt
			for _, s := range fn.Body.Body {
				if d, ok := s.(*hir.VarDeclStmt); ok && d.Name == p.Name {
					if st, ok := d.Ty.(*mir2.StructTy); ok && isShallowScalarStruct(st) {
						if r, ok := d.Init.(*hir.VarRefExpr); ok && r.Name == p.Name {
							decl = d
						}
					}
				}
			}
			if decl == nil || decl.Volatile {
				continue
			}
			st := decl.Ty.(*mir2.StructTy)
			fields := scalarFields(st, p.Name)
			collision := false
			for _, existing := range fn.Params {
				for _, v := range fields {
					if existing.Name == v.Name {
						collision = true
					}
				}
			}
			if collision || !scalarObjectUses(fn.Body, decl, fields, false) {
				continue
			}
			safe := true
			calls := 0
			for _, caller := range m.Funcs {
				if caller.Body == nil {
					continue
				}
				if !inspectAggregateStmt(caller.Body, func(e hir.Expr) (hir.Expr, bool) {
					if a, ok := e.(*hir.AddrOfExpr); ok && a.Sym == fn.Name {
						safe = false
					}
					if r, ok := e.(*hir.VarRefExpr); ok && r.Name == fn.Name {
						safe = false
					}
					if c, ok := e.(*hir.CallExpr); ok && c.Fn == fn.Name {
						calls++
						if pi >= len(c.Args) {
							safe = false
						} else {
							if r, ok := c.Args[pi].(*hir.VarRefExpr); !ok || r.Volatile {
								safe = false
							}
						}
					}
					return e, true
				}) {
					safe = false
				}
			}
			for _, g := range m.Globals {
				for _, symbol := range g.VtableSyms {
					if symbol == fn.Name {
						safe = false
					}
				}
			}
			if !safe || calls == 0 {
				continue
			}
			scalarObjectUses(fn.Body, decl, fields, true)
			var body []hir.Stmt
			for _, s := range fn.Body.Body {
				if s != decl {
					body = append(body, s)
				}
			}
			fn.Body.Body = body
			var params []hir.Param
			for _, field := range st.Fields {
				v := fields[field.Name]
				params = append(params, hir.Param{Name: v.Name, Ty: v.Ty})
			}
			fn.Params = append(append(append([]hir.Param{}, fn.Params[:pi]...), params...), fn.Params[pi+1:]...)
			for _, caller := range m.Funcs {
				aggregateBlock(caller.Body, func(e hir.Expr) (hir.Expr, bool) {
					if c, ok := e.(*hir.CallExpr); ok && c.Fn == fn.Name {
						arg := c.Args[pi]
						var args []hir.Expr
						for j, field := range st.Fields {
							args = append(args, &hir.FieldExpr{X: arg, Field: field.Name, Offset: st.ByteOffset(j), Ty: field.Ty})
						}
						c.Args = append(append(append([]hir.Expr{}, c.Args[:pi]...), args...), c.Args[pi+1:]...)
					}
					return e, true
				})
			}
		}
	}
}

func visitAggregateStmts(b *hir.Block, f func(hir.Stmt)) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		f(s)
		switch x := s.(type) {
		case *hir.Block:
			visitAggregateStmts(x, f)
		case *hir.IfStmt:
			visitAggregateStmts(x.Then, f)
			visitAggregateStmts(x.Else, f)
		case *hir.WhileStmt:
			visitAggregateStmts(x.Body, f)
		case *hir.ForRangeStmt:
			visitAggregateStmts(x.Body, f)
		case *hir.SwitchStmt:
			for _, c := range x.Cases {
				visitAggregateStmts(c.Body, f)
			}
			visitAggregateStmts(x.Default, f)
		}
	}
}
