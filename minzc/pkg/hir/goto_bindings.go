package hir

import "fmt"

// verifyGotoBindings checks bindings at a label against every explicit incoming
// jump. Labels remain function-wide: entering an empty/nondemanding block is
// legal, but a jump cannot manufacture locals whose declaration was skipped.
func verifyGotoBindings(f *Func) error {
	labels := map[string]map[string]Stmt{}
	demands := gotoLabelDemands(f.Body)
	jumps := map[*GotoStmt]map[string]Stmt{}
	paths := map[*GotoStmt][]map[string]Stmt{}
	jumpPath := map[*GotoStmt][]*Block{}
	labelPath := map[string][]*Block{}
	labelTail := map[string][]Stmt{}
	loopBodies := map[*Block]bool{}
	whileControls := map[*Block]map[string]bool{}
	var blockStack []*Block
	var envStack []map[string]Stmt
	var walk func(*Block, map[string]Stmt, []Stmt)
	clone := func(in map[string]Stmt) map[string]Stmt {
		out := map[string]Stmt{}
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	walk = func(b *Block, outer map[string]Stmt, continuation []Stmt) {
		if b == nil {
			return
		}
		env := clone(outer)
		blockStack = append(blockStack, b)
		envStack = append(envStack, env)
		defer func() { blockStack = blockStack[:len(blockStack)-1]; envStack = envStack[:len(envStack)-1] }()
		for i, s := range b.Body {
			switch x := s.(type) {
			case *VarDeclStmt:
				env[x.Name] = s
			case *TupleLetStmt:
				for _, n := range x.Names {
					if n != "_" {
						env[n] = s
					}
				}
			case *LabelStmt:
				labelTail[x.Name] = append(append([]Stmt(nil), b.Body[i+1:]...), continuation...)
				labels[x.Name] = clone(env)
				labelPath[x.Name] = append([]*Block(nil), blockStack...)
			case *GotoStmt:
				jumps[x] = clone(env)
				jumpPath[x] = append([]*Block(nil), blockStack...)
				for _, scope := range envStack {
					paths[x] = append(paths[x], clone(scope))
				}
			case *Block:
				walk(x, env, append(append([]Stmt(nil), b.Body[i+1:]...), continuation...))
			case *IfStmt:
				walk(x.Then, env, append(append([]Stmt(nil), b.Body[i+1:]...), continuation...))
				walk(x.Else, env, append(append([]Stmt(nil), b.Body[i+1:]...), continuation...))
			case *WhileStmt:
				refs := map[string]bool{}
				collectExprRefs(x.Cond, refs)
				whileControls[x.Body] = refs
				walk(x.Body, env, nil)
			case *ForRangeStmt:
				loopBodies[x.Body] = true
				inner := clone(env)
				inner[x.Var] = s
				walk(x.Body, inner, nil)
			case *ForEachStmt:
				loopBodies[x.Body] = true
				inner := clone(env)
				inner[x.Var] = s
				walk(x.Body, inner, nil)
			case *SwitchStmt:
				for _, c := range x.Cases {
					walk(c.Body, env, append(append([]Stmt(nil), b.Body[i+1:]...), continuation...))
				}
				walk(x.Default, env, append(append([]Stmt(nil), b.Body[i+1:]...), continuation...))
			}
		}
	}
	env := map[string]Stmt{}
	for _, p := range f.Params {
		env[p.Name] = nil
	}
	for _, p := range f.Captures {
		env[p.Name] = nil
	}
	walk(f.Body, env, nil)
	for jump, source := range jumps {
		common := 0
		for i, b := range labelPath[jump.Label] {
			if i >= len(jumpPath[jump]) || jumpPath[jump][i] != b {
				break
			}
			common = i + 1
		}
		if common > 0 && common < len(jumpPath[jump]) {
			source = paths[jump][common-1]
		}

		for _, body := range labelPath[jump.Label] {
			needsState := loopBodies[body]
			for name := range whileControls[body] {
				binding, local := labels[jump.Label][name]
				other, available := source[name]
				if local && (!available || binding != other) {
					needsState = true
				}
			}
			if !needsState {
				continue
			}
			inside := false
			for _, sourceBlock := range jumpPath[jump] {
				if sourceBlock == body {
					inside = true
				}
			}
			if inside {
				continue
			}
			// Range/iterator bodies have compiler-generated induction state.
			// Entering directly is safe only when every continuation exits
			// without reading that skipped state.
			visiting := map[string]bool{}
			var escapes func([]Stmt) bool
			escapes = func(stmts []Stmt) bool {
				for _, stmt := range stmts {
					switch x := stmt.(type) {
					case *ReturnStmt:
						return true
					case *BreakStmt, *ContinueStmt:
						return false
					case *GotoStmt:
						local := false
						for _, targetBlock := range labelPath[x.Label] {
							if targetBlock == body {
								local = true
							}
						}
						if !local {
							return true
						}
						if visiting[x.Label] {
							return false
						}
						visiting[x.Label] = true
						result := escapes(labelTail[x.Label])
						delete(visiting, x.Label)
						return result
					case *Block:
						if escapes(x.Body) {
							return true
						}
					case *IfStmt:
						if x.Else != nil && escapes(x.Then.Body) && escapes(x.Else.Body) {
							return true
						}
					}
				}
				return false
			}
			if !escapes(labelTail[jump.Label]) {
				return fmt.Errorf("goto %s enters loop with uninitialized induction state", jump.Label)
			}
		}
		for name, binding := range labels[jump.Label] {
			if !demands[jump.Label][name] {
				continue
			}
			other, present := source[name]
			if present && other == binding {
				continue
			}
			return fmt.Errorf("goto %s skips lexical binding %s", jump.Label, name)
		}
	}
	return nil
}

func gotoLabelDemands(root *Block) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	targets := map[string]map[string]map[string]bool{}
	var walk func(*Block, []Stmt)
	walk = func(b *Block, continuation []Stmt) {
		if b == nil {
			return
		}
		for i, s := range b.Body {
			tail := append(append([]Stmt(nil), b.Body[i+1:]...), continuation...)
			switch x := s.(type) {
			case *LabelStmt:
				refs := map[string]bool{}
				bound := map[string]bool{}
				edges := map[string]map[string]bool{}
				for _, stmt := range tail {
					before := map[string]bool{}
					for n, v := range bound {
						before[n] = v
					}
					if a, ok := stmt.(*AssignStmt); ok {
						if vr, ok := a.Target.(*VarRefExpr); ok {
							uses := map[string]bool{}
							collectExprRefs(a.Val, uses)
							for n := range uses {
								if !bound[n] {
									refs[n] = true
								}
							}
							bound[vr.Name] = true
							continue
						}
					}
					if conditional, ok := stmt.(*IfStmt); ok && conditional.Else != nil {
						uses := map[string]bool{}
						collectExprRefs(conditional.Cond, uses)
						for n := range uses {
							if !bound[n] {
								refs[n] = true
							}
						}
						var branch func(*Block, map[string]bool)
						branch = func(block *Block, kills map[string]bool) {
							original := map[string]bool{}
							locals := map[string]bool{}
							defer func() {
								for n := range locals {
									if original[n] {
										kills[n] = true
									} else {
										delete(kills, n)
									}
								}
							}()
							for _, child := range block.Body {
								if assignment, ok := child.(*AssignStmt); ok {
									if variable, ok := assignment.Target.(*VarRefExpr); ok {
										uses := map[string]bool{}
										collectExprRefs(assignment.Val, uses)
										for n := range uses {
											if !kills[n] {
												refs[n] = true
											}
										}
										kills[variable.Name] = true
										continue
									}
								}
								if nested, ok := child.(*Block); ok {
									branch(nested, kills)
									if gotoStmtTerminates(nested) {
										return
									}
									continue
								}
								switch declaration := child.(type) {
								case *VarDeclStmt:
									if !locals[declaration.Name] {
										original[declaration.Name] = kills[declaration.Name]
									}
									locals[declaration.Name] = true
								case *TupleLetStmt:
									for _, n := range declaration.Names {
										if n != "_" {
											if !locals[n] {
												original[n] = kills[n]
											}
											locals[n] = true
										}
									}
								}
								splitFreeRefs(child, kills, refs)
								if gotoStmtTerminates(child) {
									return
								}
							}
						}
						left, right := map[string]bool{}, map[string]bool{}
						for n, v := range bound {
							left[n] = v
							right[n] = v
						}
						branch(conditional.Then, left)
						branch(conditional.Else, right)
						for n := range left {
							if right[n] || gotoStmtTerminates(conditional.Else) {
								bound[n] = true
							}
						}
						if gotoStmtTerminates(conditional.Then) {
							for n := range right {
								bound[n] = true
							}
						}
					} else {
						splitFreeRefs(stmt, bound, refs)
					}
					gotoCollectDemandEdges(stmt, before, edges)
					// Only the explicit edge can continue past a terminator.
					// Later labels contribute through the fixed point, rather
					// than demanding values from unreachable textual suffixes.
					switch stmt.(type) {
					case *GotoStmt, *ReturnStmt, *BreakStmt, *ContinueStmt:
						goto finishedTail
					}
				}
			finishedTail:
				out[x.Name] = refs
				targets[x.Name] = edges
			case *Block:
				walk(x, tail)
			case *IfStmt:
				walk(x.Then, tail)
				walk(x.Else, tail)
			case *WhileStmt:
				walk(x.Body, append([]Stmt{&ExprStmt{Expr: x.Cond}}, tail...))
			case *ForRangeStmt:
				walk(x.Body, tail)
			case *ForEachStmt:
				walk(x.Body, tail)
			case *SwitchStmt:
				for _, c := range x.Cases {
					walk(c.Body, tail)
				}
				walk(x.Default, tail)
			}
		}
	}
	walk(root, nil)
	changed := true
	for changed {
		changed = false
		for name, edges := range targets {
			for target, killed := range edges {
				for variable := range out[target] {
					if killed[variable] {
						continue
					}
					if !out[name][variable] {
						out[name][variable] = true
						changed = true
					}
				}
			}
		}
	}
	return out
}

// gotoAssignedNames identifies bindings fully assigned on every branch before
// fallthrough. It is used when a goto skipped the original initializer.
func gotoAssignedNames(b *Block) map[string]bool {
	out := map[string]bool{}
	if b == nil {
		return out
	}
	locals := map[string]bool{}
	outer := map[string]bool{}
	defer func() {
		for n := range locals {
			if outer[n] {
				out[n] = true
			} else {
				delete(out, n)
			}
		}
	}()
	for _, s := range b.Body {
		switch x := s.(type) {
		case *VarDeclStmt:
			if !locals[x.Name] {
				outer[x.Name] = out[x.Name]
			}
			locals[x.Name] = true
		case *TupleLetStmt:
			for _, n := range x.Names {
				if n != "_" {
					if !locals[n] {
						outer[n] = out[n]
					}
					locals[n] = true
				}
			}
		case *AssignStmt:
			if target, ok := x.Target.(*VarRefExpr); ok {
				out[target.Name] = true
			}
		case *Block:
			for n := range gotoAssignedNames(x) {
				out[n] = true
			}
			if gotoStmtTerminates(x) {
				return out
			}
		case *IfStmt:
			if x.Else != nil {
				left, right := gotoAssignedNames(x.Then), gotoAssignedNames(x.Else)
				for n := range left {
					if right[n] || gotoStmtTerminates(x.Else) {
						out[n] = true
					}
				}
				if gotoStmtTerminates(x.Then) {
					for n := range right {
						out[n] = true
					}
				}
			}
		case *ReturnStmt, *GotoStmt, *BreakStmt, *ContinueStmt:
			return out
		}
	}
	return out
}

func gotoStmtTerminates(s Stmt) bool {
	switch x := s.(type) {
	case *ReturnStmt, *GotoStmt, *BreakStmt, *ContinueStmt:
		return true
	case *Block:
		for _, child := range x.Body {
			if gotoStmtTerminates(child) {
				return true
			}
		}
	case *IfStmt:
		return x.Else != nil && gotoStmtTerminates(x.Then) && gotoStmtTerminates(x.Else)
	}
	return false
}

func gotoCollectDemandEdges(s Stmt, bound map[string]bool, edges map[string]map[string]bool) {
	clone := func(in map[string]bool) map[string]bool {
		out := map[string]bool{}
		for n, v := range in {
			out[n] = v
		}
		return out
	}
	shadows := map[string]int{}
	shadowOuter := map[string][]bool{}
	var walk func(Stmt, map[string]bool)
	walk = func(stmt Stmt, killed map[string]bool) {
		switch x := stmt.(type) {
		case *GotoStmt:
			for n := range shadows {
				killed[n] = shadowOuter[n][0]
			}
			if previous, ok := edges[x.Label]; ok {
				for n := range previous {
					if !killed[n] {
						delete(previous, n)
					}
				}
			} else {
				edges[x.Label] = clone(killed)
			}
		case *AssignStmt:
			if target, ok := x.Target.(*VarRefExpr); ok {
				killed[target.Name] = true
			}
		case *VarDeclStmt:
			killed[x.Name] = true
		case *TupleLetStmt:
			for _, n := range x.Names {
				if n != "_" {
					killed[n] = true
				}
			}
		case *Block:
			original := map[string]bool{}
			locals := map[string]bool{}
			defer func() {
				for n := range locals {
					shadowOuter[n] = shadowOuter[n][:len(shadowOuter[n])-1]
					shadows[n]--
					if shadows[n] == 0 {
						delete(shadows, n)
					}
					if original[n] {
						killed[n] = true
					} else {
						delete(killed, n)
					}
				}
			}()
			for _, child := range x.Body {
				switch declaration := child.(type) {
				case *VarDeclStmt:
					if !locals[declaration.Name] {
						original[declaration.Name] = killed[declaration.Name]
						shadowOuter[declaration.Name] = append(shadowOuter[declaration.Name], killed[declaration.Name])
						shadows[declaration.Name]++
					}
					locals[declaration.Name] = true
				case *TupleLetStmt:
					for _, n := range declaration.Names {
						if n != "_" {
							if !locals[n] {
								original[n] = killed[n]
								shadowOuter[n] = append(shadowOuter[n], killed[n])
								shadows[n]++
							}
							locals[n] = true
						}
					}
				}
				walk(child, killed)
				if gotoStmtTerminates(child) {
					break
				}
			}
		case *IfStmt:
			walk(x.Then, clone(killed))
			if x.Else != nil {
				walk(x.Else, clone(killed))
			}
		case *WhileStmt:
			walk(x.Body, clone(killed))
		case *ForRangeStmt:
			walk(x.Body, clone(killed))
		case *ForEachStmt:
			walk(x.Body, clone(killed))
		case *SwitchStmt:
			for _, c := range x.Cases {
				walk(c.Body, clone(killed))
			}
			if x.Default != nil {
				walk(x.Default, clone(killed))
			}
		}
	}
	walk(s, clone(bound))
}
