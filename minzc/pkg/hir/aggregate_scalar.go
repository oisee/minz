package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

// aggregateExpr/aggregateBlock deliberately reject unknown IR shapes. New HIR
// nodes must be audited before this pass may remove an addressable C object.
func aggregateExpr(e Expr, f func(Expr) (Expr, bool)) (Expr, bool) {
	if e == nil {
		return nil, true
	}
	e, ok := f(e)
	if !ok {
		return e, false
	}
	child := func(p *Expr) {
		if ok {
			*p, ok = aggregateExpr(*p, f)
		}
	}
	switch x := e.(type) {
	case *IntLitExpr, *BoolLitExpr, *VarRefExpr, *ConstPtrExpr:
	case *FieldExpr:
		child(&x.X)
	case *AddrOfExpr:
		child(&x.X)
	case *BinExpr:
		child(&x.L)
		child(&x.R)
	case *UnaryExpr:
		child(&x.X)
	case *CastExpr:
		child(&x.X)
	case *LoadExpr:
		child(&x.Ptr)
	case *DerefExpr:
		child(&x.Ptr)
	case *IndexExpr:
		child(&x.Base)
		child(&x.Idx)
	case *CallExpr:
		for i := range x.Args {
			child(&x.Args[i])
		}
	case *CallIndirectExpr:
		child(&x.FnPtr)
		for i := range x.Args {
			child(&x.Args[i])
		}
	case *StructLitExpr:
		for i := range x.Fields {
			child(&x.Fields[i].Val)
		}
	case *CondExpr:
		child(&x.Cond)
		child(&x.Then)
		child(&x.Else)
	default:
		return e, false
	}
	return e, ok
}

func aggregateBlock(b *Block, f func(Expr) (Expr, bool)) bool {
	if b == nil {
		return true
	}
	ok := true
	expr := func(p *Expr) {
		if ok {
			*p, ok = aggregateExpr(*p, f)
		}
	}
	block := func(p *Block) {
		if ok {
			ok = aggregateBlock(p, f)
		}
	}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *Block:
			block(x)
		case *VarDeclStmt:
			expr(&x.Init)
			for i := range x.Initial {
				expr(&x.Initial[i])
			}
		case *AssignStmt:
			expr(&x.Target)
			expr(&x.Val)
		case *ReturnStmt:
			expr(&x.Val)
			for i := range x.Vals {
				expr(&x.Vals[i])
			}
		case *TupleLetStmt:
			var e Expr = x.Call
			expr(&e)
			x.Call = e.(*CallExpr)
		case *ExprStmt:
			expr(&x.Expr)
		case *IfStmt:
			expr(&x.Cond)
			block(x.Then)
			block(x.Else)
		case *WhileStmt:
			expr(&x.Cond)
			block(x.Body)
		case *ForRangeStmt:
			expr(&x.Start)
			expr(&x.End)
			block(x.Body)
		case *SwitchStmt:
			expr(&x.Val)
			for _, c := range x.Cases {
				block(c.Body)
			}
			block(x.Default)
		case *StoreStmt:
			expr(&x.Ptr)
			expr(&x.Val)
		case *BreakStmt, *ContinueStmt, *LabelStmt, *GotoStmt:
		default:
			return false
		}
		if !ok {
			return false
		}
	}
	return true
}

func scalarFields(st *mir2.StructTy, name string) map[string]*VarRefExpr {
	result := make(map[string]*VarRefExpr)
	for i, field := range st.Fields {
		result[field.Name] = Var(fmt.Sprintf("__c89_scalar_%s_%d", name, i), field.Ty)
	}
	return result
}

// scalarObjectUses validates every use before removing storage. Internal C
// lvalue address temporaries are admitted only if all their uses dereference
// exactly that field. An escaping pointer, volatile access, or whole-object use
// keeps the original object and memory ABI.
func scalarObjectUses(b *Block, decl *VarDeclStmt, fields map[string]*VarRefExpr, apply bool) bool {
	return scalarObjectUsesWithAliases(b, decl, fields, apply, true)
}

// Only C lowering supplies the scoped lvalue temporaries admitted here.
// Typed frontend-neutral values require that no field address is taken.
func scalarObjectUsesWithAliases(b *Block, decl *VarDeclStmt, fields map[string]*VarRefExpr, apply, allowAliases bool) bool {
	aliases := make(map[string]*VarRefExpr)
	duplicate := false
	visitAggregateStmts(b, func(s Stmt) {
		d, ok := s.(*VarDeclStmt)
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
		if d != decl && allowAliases {
			if a, ok := d.Init.(*AddrOfExpr); ok {
				if fe, ok := a.X.(*FieldExpr); ok && !fe.Volatile {
					if r, ok := fe.X.(*VarRefExpr); ok && r.Name == decl.Name && !r.Volatile {
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
	rewrite := func(e Expr) (Expr, bool) {
		switch x := e.(type) {
		case *FieldExpr:
			if r, ok := x.X.(*VarRefExpr); ok && r.Name == decl.Name {
				v := fields[x.Field]
				if v == nil || x.Volatile || r.Volatile {
					return e, false
				}
				// Substitute during validation too, to avoid visiting the object reference.
				return Var(v.Name, v.Ty), true
			}
		case *DerefExpr:
			if r, ok := x.Ptr.(*VarRefExpr); ok && aliases[r.Name] != nil && !x.Volatile {
				v := aliases[r.Name]
				return Var(v.Name, v.Ty), true
			}
		case *LoadExpr:
			if r, ok := x.Ptr.(*VarRefExpr); ok && aliases[r.Name] != nil && !x.Volatile {
				v := aliases[r.Name]
				return Var(v.Name, v.Ty), true
			}
		case *VarRefExpr:
			if x.Name == decl.Name || aliases[x.Name] != nil {
				return e, false
			}
			for _, v := range fields {
				if x.Name == v.Name {
					return e, false
				}
			}
		case *AddrOfExpr:
			if x.Sym == decl.Name || aliases[x.Sym] != nil {
				return e, false
			}
			if !inspectAggregateExpr(x.X, func(child Expr) (Expr, bool) {
				if r, ok := child.(*VarRefExpr); ok && (r.Name == decl.Name || aliases[r.Name] != nil) {
					return child, false
				}
				return child, true
			}) {
				return e, false
			}
		case *UnaryExpr:
			if x.Op == "&" && !inspectAggregateExpr(x.X, func(child Expr) (Expr, bool) {
				if r, ok := child.(*VarRefExpr); ok && (r.Name == decl.Name || aliases[r.Name] != nil) {
					return child, false
				}
				return child, true
			}) {
				return e, false
			}
		}
		return e, true
	}
	// Validate without mutating original expressions: use a dry visitor that
	// replaces accepted fields with a sentinel, not a scalar binding.
	validate := func(e Expr) (Expr, bool) {
		r, ok := rewrite(e)
		if !ok {
			return e, false
		}
		if r != e {
			return U8(0), true
		}
		return e, true
	}
	visit := func(block *Block) bool {
		if block == nil {
			return true
		}
		for _, s := range block.Body {
			if d, ok := s.(*VarDeclStmt); ok && (d == decl || aliases[d.Name] != nil) {
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
	var transform func(*Block)
	transform = func(block *Block) {
		if block == nil {
			return
		}
		var body []Stmt
		for _, s := range block.Body {
			if d, ok := s.(*VarDeclStmt); ok && aliases[d.Name] != nil {
				continue
			}
			switch x := s.(type) {
			case *Block:
				transform(x)
			case *IfStmt:
				transform(x.Then)
				transform(x.Else)
			case *WhileStmt:
				transform(x.Body)
			case *ForRangeStmt:
				transform(x.Body)
			case *SwitchStmt:
				for _, c := range x.Cases {
					transform(c.Body)
				}
				transform(x.Default)
			}
			if s != decl {
				aggregateBlock(&Block{Body: []Stmt{s}}, func(e Expr) (Expr, bool) { r, _ := rewrite(e); return r, true })
			}
			body = append(body, s)
		}
		block.Body = body
	}
	transform(b)
	return true
}

func inspectAggregateExpr(e Expr, f func(Expr) (Expr, bool)) bool {
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
	child := func(e Expr) bool { return inspectAggregateExpr(e, f) }
	switch x := e.(type) {
	case *IntLitExpr, *BoolLitExpr, *VarRefExpr, *ConstPtrExpr:
		return true
	case *FieldExpr:
		return child(x.X)
	case *AddrOfExpr:
		return child(x.X)
	case *BinExpr:
		return child(x.L) && child(x.R)
	case *UnaryExpr:
		return child(x.X)
	case *CastExpr:
		return child(x.X)
	case *LoadExpr:
		return child(x.Ptr)
	case *DerefExpr:
		return child(x.Ptr)
	case *IndexExpr:
		return child(x.Base) && child(x.Idx)
	case *CallExpr:
		for _, a := range x.Args {
			if !child(a) {
				return false
			}
		}
		return true
	case *CallIndirectExpr:
		if !child(x.FnPtr) {
			return false
		}
		for _, a := range x.Args {
			if !child(a) {
				return false
			}
		}
		return true
	case *StructLitExpr:
		for _, a := range x.Fields {
			if !child(a.Val) {
				return false
			}
		}
		return true
	case *CondExpr:
		return child(x.Cond) && child(x.Then) && child(x.Else)
	}
	return false
}
func inspectAggregateStmt(s Stmt, f func(Expr) (Expr, bool)) bool {
	e := func(e Expr) bool { return inspectAggregateExpr(e, f) }
	b := func(b *Block) bool {
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
	case *Block:
		return b(x)
	case *VarDeclStmt:
		if !e(x.Init) {
			return false
		}
		for _, v := range x.Initial {
			if !e(v) {
				return false
			}
		}
		return true
	case *AssignStmt:
		return e(x.Target) && e(x.Val)
	case *ReturnStmt:
		if !e(x.Val) {
			return false
		}
		for _, v := range x.Vals {
			if !e(v) {
				return false
			}
		}
		return true
	case *TupleLetStmt:
		return e(x.Call)
	case *ExprStmt:
		return e(x.Expr)
	case *IfStmt:
		return e(x.Cond) && b(x.Then) && b(x.Else)
	case *WhileStmt:
		return e(x.Cond) && b(x.Body)
	case *ForRangeStmt:
		return e(x.Start) && e(x.End) && b(x.Body)
	case *SwitchStmt:
		if !e(x.Val) || !b(x.Default) {
			return false
		}
		for _, c := range x.Cases {
			if !b(c.Body) {
				return false
			}
		}
		return true
	case *StoreStmt:
		return e(x.Ptr) && e(x.Val)
	case *BreakStmt, *ContinueStmt, *LabelStmt, *GotoStmt:
		return true
	}
	return false
}

func scalarizeAggregateLocals(b *Block, allowAliases bool) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *Block:
			scalarizeAggregateLocals(x, allowAliases)
		case *IfStmt:
			scalarizeAggregateLocals(x.Then, allowAliases)
			scalarizeAggregateLocals(x.Else, allowAliases)
		case *WhileStmt:
			scalarizeAggregateLocals(x.Body, allowAliases)
		case *ForRangeStmt:
			scalarizeAggregateLocals(x.Body, allowAliases)
		case *SwitchStmt:
			for _, c := range x.Cases {
				scalarizeAggregateLocals(c.Body, allowAliases)
			}
			scalarizeAggregateLocals(x.Default, allowAliases)
		}
	}
	for i := 0; i < len(b.Body); i++ {
		d, ok := b.Body[i].(*VarDeclStmt)
		if !ok || d.Volatile || d.ArrayLen != 0 || d.At != nil {
			continue
		}
		lit, ok := d.Init.(*StructLitExpr)
		if !ok && d.Init == nil {
			if st := aggregateDeclarationType(d); isShallowScalarStruct(st) {
				// Keep uninitialized scalar declarations. Actual subsequent field
				// assignments provide the values; no initializer is replayed.
				lit = &StructLitExpr{St: st}
				for _, field := range st.Fields {
					lit.Fields = append(lit.Fields, FieldInit{Name: field.Name})
				}
				ok = true
			}
		}
		if !ok || !isShallowScalarStruct(lit.St) || !completeScalarLiteral(lit) {
			continue
		}
		fields := scalarFields(lit.St, d.Name)
		tupleFields := false
		if i > 0 {
			if tuple, ok := b.Body[i-1].(*TupleLetStmt); ok && len(tuple.Names) == len(lit.Fields) {
				tupleFields = true
				for j, fi := range lit.Fields {
					r, ok := fi.Val.(*VarRefExpr)
					if !ok || r.Name != tuple.Names[j] {
						tupleFields = false
						break
					}
				}
				if tupleFields {
					for _, fi := range lit.Fields {
						r := fi.Val.(*VarRefExpr)
						fields[fi.Name] = Var(r.Name, r.Ty)
					}
				}
			}
		}
		if !tupleFields {
			used := map[string]bool{}
			inspectAggregateStmt(b, func(e Expr) (Expr, bool) {
				if r, ok := e.(*VarRefExpr); ok {
					used[r.Name] = true
				}
				return e, true
			})
			walkStmts(b, func(s Stmt) bool {
				if d, ok := s.(*VarDeclStmt); ok {
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
		if !scalarObjectUsesWithAliases(b, d, fields, false, allowAliases) {
			continue
		}
		scalarObjectUsesWithAliases(b, d, fields, true, allowAliases)
		// Preserve initialization order and evaluate each initializer once.
		var init []Stmt
		for _, fi := range lit.Fields {
			if tupleFields {
				continue
			}
			v := fields[fi.Name]
			if v == nil {
				continue
			}
			init = append(init, &VarDeclStmt{Name: v.Name, Ty: v.Ty, Init: fi.Val})
		}
		var body []Stmt
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
func scalarizeAggregateParams(m *Module, onlyC, typedOnly bool) bool {
	changed := false
	for _, fn := range m.Funcs {
		if onlyC && !fn.CValueABI || typedOnly && fn.CValueABI {
			continue
		}
		if fn.IsExtern || fn.Body == nil {
			continue
		}
		for pi := len(fn.Params) - 1; pi >= 0; pi-- {
			p := fn.Params[pi]
			var decl *VarDeclStmt
			for _, s := range fn.Body.Body {
				if d, ok := s.(*VarDeclStmt); ok && d.Name == p.Name {
					if st, ok := d.Ty.(*mir2.StructTy); ok && isShallowScalarStruct(st) {
						if r, ok := d.Init.(*VarRefExpr); ok && r.Name == p.Name {
							decl = d
						}
					}
				}
			}
			if decl == nil && typedOnly {
				if st, ok := mir2.BaseOf(p.Ty).(*mir2.StructTy); ok && isShallowScalarStruct(st) {
					// Typed Nanz parameters alias their input storage. Only
					// read-only bodies may replace that address by field values.
					readOnly := true
					visitAggregateStmts(fn.Body, func(s Stmt) {
						switch s.(type) {
						case *AssignStmt, *StoreStmt:
							readOnly = false
						}
					})
					inspectAggregateStmt(fn.Body, func(e Expr) (Expr, bool) {
						switch e.(type) {
						case *CallExpr, *CallIndirectExpr:
							readOnly = false
						}
						return e, true
					})
					if readOnly {
						decl = &VarDeclStmt{Name: p.Name, Ty: st}
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
			if collision || !scalarObjectUsesWithAliases(fn.Body, decl, fields, false, !typedOnly) {
				continue
			}
			safe := true
			calls := 0
			for _, caller := range m.Funcs {
				if caller.Body == nil {
					continue
				}
				if !inspectAggregateStmt(caller.Body, func(e Expr) (Expr, bool) {
					if a, ok := e.(*AddrOfExpr); ok && a.Sym == fn.Name {
						safe = false
					}
					if r, ok := e.(*VarRefExpr); ok && r.Name == fn.Name {
						safe = false
					}
					if c, ok := e.(*CallExpr); ok && c.Fn == fn.Name {
						calls++
						if typedOnly {
							// Snapshotting fields must not cross evaluation of
							// another argument that could mutate their storage.
							for _, arg := range c.Args {
								inspectAggregateExpr(arg, func(e Expr) (Expr, bool) {
									switch e.(type) {
									case *CallExpr, *CallIndirectExpr:
										safe = false
									}
									return e, true
								})
							}
						}
						if pi >= len(c.Args) {
							safe = false
						} else {
							if r, ok := c.Args[pi].(*VarRefExpr); !ok || r.Volatile {
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
				for _, r := range g.Relocs {
					if r.Symbol == fn.Name {
						safe = false
					}
				}
				for _, symbol := range g.VtableSyms {
					if symbol == fn.Name {
						safe = false
					}
				}
			}
			if !safe || calls == 0 {
				continue
			}
			scalarObjectUsesWithAliases(fn.Body, decl, fields, true, !typedOnly)
			var body []Stmt
			for _, s := range fn.Body.Body {
				if s != decl {
					body = append(body, s)
				}
			}
			fn.Body.Body = body
			var params []Param
			for _, field := range st.Fields {
				v := fields[field.Name]
				params = append(params, Param{Name: v.Name, Ty: v.Ty})
			}
			changed = true
			fn.Params = append(append(append([]Param{}, fn.Params[:pi]...), params...), fn.Params[pi+1:]...)
			for _, caller := range m.Funcs {
				aggregateBlock(caller.Body, func(e Expr) (Expr, bool) {
					if c, ok := e.(*CallExpr); ok && c.Fn == fn.Name {
						arg := c.Args[pi]
						var args []Expr
						for j, field := range st.Fields {
							args = append(args, &FieldExpr{X: arg, Field: field.Name, Offset: st.ByteOffset(j), Ty: field.Ty})
						}
						c.Args = append(append(append([]Expr{}, c.Args[:pi]...), args...), c.Args[pi+1:]...)
					}
					return e, true
				})
			}
		}
	}

	return changed
}

func visitAggregateStmts(b *Block, f func(Stmt)) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		f(s)
		switch x := s.(type) {
		case *Block:
			visitAggregateStmts(x, f)
		case *IfStmt:
			visitAggregateStmts(x.Then, f)
			visitAggregateStmts(x.Else, f)
		case *WhileStmt:
			visitAggregateStmts(x.Body, f)
		case *ForRangeStmt:
			visitAggregateStmts(x.Body, f)
		case *SwitchStmt:
			for _, c := range x.Cases {
				visitAggregateStmts(c.Body, f)
			}
			visitAggregateStmts(x.Default, f)
		}
	}
}

// Omitted or repeated fields retain storage. In particular a frontend may
// permit partial literals whose remaining fields are initialized later.
func completeScalarLiteral(lit *StructLitExpr) bool {
	if lit.St == nil || len(lit.Fields) != len(lit.St.Fields) {
		return false
	}
	seen := map[string]bool{}
	for _, f := range lit.Fields {
		if seen[f.Name] {
			return false
		}
		seen[f.Name] = true
	}
	for _, f := range lit.St.Fields {
		if !seen[f.Name] {
			return false
		}
	}
	return true
}
