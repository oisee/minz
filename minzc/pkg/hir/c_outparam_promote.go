package hir

import "github.com/minz/minzc/pkg/mir2"

// cOutCall is a closed direct call writing a nonescaping local C value object.
// Delaying its field stores until return is unobservable: the object has no
// address use except these calls, and the callee never reads or escapes out.
type cOutCall struct {
	call *CallExpr
	decl *VarDeclStmt
}

// promoteCOutParameters handles the C frontend's captured-lvalue form. It
// preserves the evaluation time of each field RHS in a local scalar slot,
// rather than gathering expressions and replaying them at the final return.
func promoteCOutParameters(m *Module) bool {
	changed := false
	for _, fn := range m.Funcs {
		if !fn.CValueABI || fn.IsExtern || fn.FallibleCY || fn.Body == nil || fn.RetTy != mir2.TyVoid || len(fn.RetTys) != 0 || len(fn.Params) == 0 {
			continue
		}
		pi := len(fn.Params) - 1
		if fn.Params[pi].Ty != mir2.TyPtr || fn.Params[pi].SMC {
			continue
		}
		calls := map[*CallExpr]*cOutCall{}
		var st *mir2.StructTy
		safe := true
		for _, caller := range m.Funcs {
			if caller.Body == nil {
				continue
			}
			declarations := map[string]*VarDeclStmt{}
			duplicates := map[string]bool{}
			visitAggregateStmts(caller.Body, func(s Stmt) {
				if d, ok := s.(*VarDeclStmt); ok {
					if declarations[d.Name] != nil {
						duplicates[d.Name] = true
					}
					declarations[d.Name] = d
				}
			})
			visitAggregateStmts(caller.Body, func(s Stmt) {
				e, ok := s.(*ExprStmt)
				if !ok {
					return
				}
				c, ok := e.Expr.(*CallExpr)
				if !ok || c.Fn != fn.Name {
					return
				}
				if len(c.Args) != len(fn.Params) {
					safe = false
					return
				}
				a, ok := c.Args[pi].(*AddrOfExpr)
				if !ok || a.X != nil {
					safe = false
					return
				}
				d := declarations[a.Sym]
				if d == nil || duplicates[a.Sym] || d.Volatile || d.ArrayLen != 0 || d.At != nil {
					safe = false
					return
				}
				typ := aggregateDeclarationType(d)
				if !isShallowScalarStruct(typ) || (st != nil && st != typ) {
					safe = false
					return
				}
				st = typ
				calls[c] = &cOutCall{c, d}
			})
		}
		if !safe || len(calls) == 0 || st == nil {
			continue
		}
		// No expression-level calls or escaped function addresses may retain the old ABI.
		for _, caller := range m.Funcs {
			if caller.Body == nil {
				continue
			}
			if !inspectAggregateStmt(caller.Body, func(e Expr) (Expr, bool) {
				if c, ok := e.(*CallExpr); ok && c.Fn == fn.Name && calls[c] == nil {
					safe = false
				}
				if a, ok := e.(*AddrOfExpr); ok && a.Sym == fn.Name {
					safe = false
				}
				if r, ok := e.(*VarRefExpr); ok && r.Name == fn.Name {
					safe = false
				}
				return e, true
			}) {
				safe = false
			}
			// The dry view removes only the admitted out arguments, without changing
			// original nodes. Existing scalar-object analysis rejects all other address
			// escapes, aliases, volatile uses and unknown/ASM statement shapes.
			view := cOutDryView(caller.Body, calls, pi)
			for _, site := range calls {
				found := false
				visitAggregateStmts(caller.Body, func(s Stmt) {
					if s == site.decl {
						found = true
					}
				})
				if found && !scalarObjectUses(view, site.decl, scalarFields(st, site.decl.Name), false) {
					safe = false
				}
			}
		}
		for _, g := range m.Globals {
			for _, r := range g.Relocs {
				if r.Symbol == fn.Name {
					safe = false
				}
			}
			for _, s := range g.VtableSyms {
				if s == fn.Name {
					safe = false
				}
			}
		}
		if !safe {
			continue
		}
		out := fn.Params[pi].Name
		captures := map[*VarDeclStmt]*FieldExpr{}
		writes := map[*AssignStmt]*FieldExpr{}
		allowed := map[Expr]bool{}
		refs := map[string]int{}
		inspectAggregateStmt(fn.Body, func(e Expr) (Expr, bool) {
			if r, ok := e.(*VarRefExpr); ok {
				refs[r.Name]++
			}
			return e, true
		})
		var scan func(*Block)
		scan = func(b *Block) {
			if b == nil {
				return
			}
			for i, s := range b.Body {
				if d, ok := s.(*VarDeclStmt); ok && i+1 < len(b.Body) {
					a, ok := d.Init.(*AddrOfExpr)
					if ok {
						if f, ok := a.X.(*FieldExpr); ok {
							if r, ok := f.X.(*VarRefExpr); ok && r.Name == out {
								w, ok := b.Body[i+1].(*AssignStmt)
								if !ok {
									safe = false
									continue
								}
								target, ok := w.Target.(*DerefExpr)
								if !ok {
									safe = false
									continue
								}
								ref, ok := target.Ptr.(*VarRefExpr)
								if !ok || ref.Name != d.Name || refs[d.Name] != 1 || d.Volatile || f.Volatile || r.Volatile || target.Volatile {
									safe = false
									continue
								}
								valid := false
								for j, field := range st.Fields {
									if field.Name == f.Field && st.ByteOffset(j) == f.Offset && field.Ty == f.Ty && field.Ty == target.Ty {
										valid = true
									}
								}
								if !valid {
									safe = false
									continue
								}
								captures[d] = f
								writes[w] = f
								allowed[a] = true
								allowed[target] = true
							}
						}
					}
				}
				switch x := s.(type) {
				case *Block:
					scan(x)
				case *IfStmt:
					scan(x.Then)
					scan(x.Else)
				case *WhileStmt:
					scan(x.Body)
				case *ForRangeStmt:
					scan(x.Body)
				}
			}
		}
		scan(fn.Body)
		if len(writes) == 0 {
			continue
		}
		if !inspectAggregateStmt(fn.Body, func(e Expr) (Expr, bool) {
			if allowed[e] {
				return U8(0), true
			}
			if r, ok := e.(*VarRefExpr); ok && r.Name == out {
				return e, false
			}
			if a, ok := e.(*AddrOfExpr); ok && a.Sym == out {
				return e, false
			}
			// Calls may indirectly observe captured stack storage or retain addresses;
			// their effect closure needs a separate proof before this pass admits them.
			switch e.(type) {
			case *CallExpr, *CallIndirectExpr:
				return e, false
			// An unrelated pointer can still designate out via an integer-derived
			// address. Nonescape alone does not prove such memory disjoint. Only
			// the matched output stores above have a memory-effect proof here.
			case *LoadExpr, *DerefExpr, *IndexExpr, *FieldExpr:
				return e, false
			}
			return e, true
		}) {
			safe = false
		}
		// Definite assignment holds on every returning path, including early returns.
		assigned, falls, ok := cOutAssigned(fn.Body, map[string]bool{}, writes, st)
		if falls {
			for _, field := range st.Fields {
				if !assigned[field.Name] {
					ok = false
				}
			}
		}
		if !ok || !safe {
			continue
		}
		slots := map[string]*VarRefExpr{}
		names := cOutNames(fn)
		for _, field := range st.Fields {
			slots[field.Name] = Var(names("__out_"+field.Name), field.Ty)
		}
		var rewrite func(*Block)
		values := func() []Expr {
			v := []Expr{}
			for _, field := range st.Fields {
				r := slots[field.Name]
				v = append(v, Var(r.Name, r.Ty))
			}
			return v
		}
		rewrite = func(b *Block) {
			if b == nil {
				return
			}
			body := []Stmt{}
			for _, s := range b.Body {
				if d, ok := s.(*VarDeclStmt); ok && captures[d] != nil {
					continue
				}
				if a, ok := s.(*AssignStmt); ok && writes[a] != nil {
					field := writes[a].Field
					r := slots[field]
					a.Target = Var(r.Name, r.Ty)
				}
				switch x := s.(type) {
				case *ReturnStmt:
					x.Val = nil
					x.Vals = values()
				case *Block:
					rewrite(x)
				case *IfStmt:
					rewrite(x.Then)
					rewrite(x.Else)
				case *WhileStmt:
					rewrite(x.Body)
				case *ForRangeStmt:
					rewrite(x.Body)
				}
				body = append(body, s)
			}
			b.Body = body
		}
		rewrite(fn.Body)
		declarations := []Stmt{}
		for _, field := range st.Fields {
			r := slots[field.Name]
			declarations = append(declarations, &VarDeclStmt{Name: r.Name, Ty: r.Ty})
			fn.RetTys = append(fn.RetTys, field.Ty)
		}
		fn.Body.Body = append(declarations, fn.Body.Body...)
		if falls {
			fn.Body.Body = append(fn.Body.Body, &ReturnStmt{Vals: values()})
		}
		fn.Params = fn.Params[:pi]
		for _, caller := range m.Funcs {
			if caller.Body == nil {
				continue
			}
			cOutRewriteCalls(caller.Body, calls, pi, st, cOutNames(caller))
		}
		changed = true
	}
	return changed
}

func cOutNames(fn *Func) func(string) string {
	used := map[string]bool{}
	for _, p := range fn.Params {
		used[p.Name] = true
	}
	visitAggregateStmts(fn.Body, func(s Stmt) {
		if d, ok := s.(*VarDeclStmt); ok {
			used[d.Name] = true
		}
		if t, ok := s.(*TupleLetStmt); ok {
			for _, n := range t.Names {
				used[n] = true
			}
		}
	})
	inspectAggregateStmt(fn.Body, func(e Expr) (Expr, bool) {
		if r, ok := e.(*VarRefExpr); ok {
			used[r.Name] = true
		}
		return e, true
	})
	return func(name string) string {
		for used[name] {
			name += "_"
		}
		used[name] = true
		return name
	}
}

func cOutDryView(b *Block, calls map[*CallExpr]*cOutCall, pi int) *Block {
	if b == nil {
		return nil
	}
	out := &Block{}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *ExprStmt:
			if c, ok := x.Expr.(*CallExpr); ok && calls[c] != nil {
				copy := *c
				copy.Args = append([]Expr{}, c.Args[:pi]...)
				s = &ExprStmt{Expr: &copy}
			}
		case *Block:
			s = cOutDryView(x, calls, pi)
		case *IfStmt:
			copy := *x
			copy.Then = cOutDryView(x.Then, calls, pi)
			copy.Else = cOutDryView(x.Else, calls, pi)
			s = &copy
		case *WhileStmt:
			copy := *x
			copy.Body = cOutDryView(x.Body, calls, pi)
			s = &copy
		case *ForRangeStmt:
			copy := *x
			copy.Body = cOutDryView(x.Body, calls, pi)
			s = &copy
		}
		out.Body = append(out.Body, s)
	}
	return out
}

func cOutAssigned(b *Block, in map[string]bool, writes map[*AssignStmt]*FieldExpr, st *mir2.StructTy) (map[string]bool, bool, bool) {
	set := map[string]bool{}
	for k, v := range in {
		set[k] = v
	}
	all := func() bool {
		for _, f := range st.Fields {
			if !set[f.Name] {
				return false
			}
		}
		return true
	}
	if b == nil {
		return set, true, true
	}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *AssignStmt:
			if f := writes[x]; f != nil {
				set[f.Field] = true
			}
		case *ReturnStmt:
			if x.Val != nil || len(x.Vals) != 0 || !all() {
				return set, false, false
			}
			return set, false, true
		case *IfStmt:
			a, af, ok := cOutAssigned(x.Then, set, writes, st)
			if !ok {
				return set, false, false
			}
			z, zf, ok := cOutAssigned(x.Else, set, writes, st)
			if !ok {
				return set, false, false
			}
			if !af && !zf {
				return set, false, true
			}
			if !af {
				set = z
			} else if !zf {
				set = a
			} else {
				for k := range set {
					delete(set, k)
				}
				for k, v := range a {
					if v && z[k] {
						set[k] = true
					}
				}
			}
		case *Block:
			next, falls, ok := cOutAssigned(x, set, writes, st)
			if !ok || !falls {
				return next, falls, ok
			}
			set = next
		case *WhileStmt:
			_, _, ok := cOutAssigned(x.Body, set, writes, st)
			if !ok {
				return set, false, false
			}
		case *ForRangeStmt:
			_, _, ok := cOutAssigned(x.Body, set, writes, st)
			if !ok {
				return set, false, false
			}
		case *VarDeclStmt, *ExprStmt:
		default:
			return set, false, false
		}
	}
	return set, true, true
}

func cOutRewriteCalls(b *Block, calls map[*CallExpr]*cOutCall, pi int, st *mir2.StructTy, name func(string) string) {
	if b == nil {
		return
	}
	body := []Stmt{}
	for _, s := range b.Body {
		switch x := s.(type) {
		case *Block:
			cOutRewriteCalls(x, calls, pi, st, name)
		case *IfStmt:
			cOutRewriteCalls(x.Then, calls, pi, st, name)
			cOutRewriteCalls(x.Else, calls, pi, st, name)
		case *WhileStmt:
			cOutRewriteCalls(x.Body, calls, pi, st, name)
		case *ForRangeStmt:
			cOutRewriteCalls(x.Body, calls, pi, st, name)
		}
		if e, ok := s.(*ExprStmt); ok {
			if c, ok := e.Expr.(*CallExpr); ok && calls[c] != nil {
				site := calls[c]
				c.Args = c.Args[:pi]
				names := []string{}
				tys := []mir2.Ty{}
				for _, f := range st.Fields {
					names = append(names, name("__out_result_"+f.Name))
					tys = append(tys, f.Ty)
				}
				body = append(body, &TupleLetStmt{Names: names, Tys: tys, Call: c})
				for i, f := range st.Fields {
					body = append(body, &AssignStmt{Target: &FieldExpr{X: Var(site.decl.Name, mir2.TyPtr), Field: f.Name, Offset: st.ByteOffset(i), Ty: f.Ty}, Val: Var(names[i], f.Ty)})
				}
				continue
			}
		}
		body = append(body, s)
	}
	b.Body = body
}
