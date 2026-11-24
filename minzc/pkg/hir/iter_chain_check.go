package hir

import "fmt"

// IsIterChainMethod reports whether name is an iterator chain method
// (map, filter, skip, take, zip, forEach, mapInPlace, fold, reduce, count, sum).
// It is a chain method in a module only when no function of that name exists.
func IsIterChainMethod(name string) bool { return isIterChainMethod(name) }

// CheckIterChains rejects iterator chains the lowering cannot fuse, before
// lowering would emit a CALL to an undefined chain method.
//
// A chain is lowered only in statement position: an expression statement
// ending in forEach/mapInPlace/fold/reduce/count/sum, or a value terminal
// (fold/reduce/count/sum) that is the whole initialiser of a let/var, the
// whole value of a return, or the whole right-hand side of an assignment. A
// chain method call anywhere else, or a chain the recogniser refuses, is a
// diagnostic. Names that the module declares as functions are ordinary calls
// and are never checked.
//
// It also rejects the frontend's unresolved method calls (Module.
// MethodCallLines) that name neither a module function nor a chain
// method — `src.min(n)` with no `fun min` (audit finding G0-5).
func CheckIterChains(m *Module) error {
	names := make(map[string]bool, len(m.Funcs))
	funcs := make(map[string]*Func, len(m.Funcs))
	for _, f := range m.Funcs {
		names[f.Name] = true
		funcs[f.Name] = f
	}
	if len(m.MethodCallLines) > 0 {
		var bad *CallExpr
		for call := range m.MethodCallLines {
			if names[call.Fn] || isIterChainMethod(call.Fn) {
				continue
			}
			if bad == nil || m.MethodCallLines[call] < m.MethodCallLines[bad] ||
				m.MethodCallLines[call] == m.MethodCallLines[bad] && call.Fn < bad.Fn {
				bad = call
			}
		}
		if bad != nil {
			return fmt.Errorf("line %d: unknown method .%s(...): no function %q is declared, and it is not an iterator chain method (map, filter, skip, take, zip, forEach, mapInPlace, fold, reduce, count, sum)",
				m.MethodCallLines[bad], bad.Fn, bad.Fn)
		}
	}
	callbacks := map[string]bool{}         // functions inlined as chain callbacks
	uses := map[*Func][]chainCallbackUse{} // callbacks of the chains inside each function
	for _, f := range m.Funcs {
		if f.Body == nil {
			continue
		}
		ck := &chainChecker{
			m: m, fn: f,
			l:        &lowerer{hirFuncNames: names, hirFuncs: funcs, fnAliases: map[string]string{}},
			consumed: map[*CallExpr]bool{},
		}
		ck.collectAliases(f.Body)
		if err := ck.block(f.Body); err != nil {
			return err
		}
		if err := ck.leftovers(f.Body); err != nil {
			return err
		}
		for _, u := range ck.uses {
			callbacks[u.fn] = true
		}
		uses[f] = ck.uses
	}
	// A callback inlined into an outer callback's body may not capture that
	// outer callback's parameter: inlining renames the parameter, and the
	// inner callback's own body would then name an undefined variable.
	for _, f := range m.Funcs {
		if !callbacks[f.Name] {
			continue
		}
		params := map[string]bool{}
		for _, p := range f.Params {
			params[p.Name] = true
		}
		for _, u := range uses[f] {
			inner := funcs[u.fn]
			if inner == nil {
				continue
			}
			for _, c := range inner.Captures {
				if params[c.Name] {
					ck := &chainChecker{m: m, fn: f}
					return ck.errorf(u.stmt, "a callback inside another callback may not use the outer callback's parameter %q; bind it to a local first (let v = %s)", c.Name, c.Name)
				}
			}
		}
	}
	return nil
}

type chainCallbackUse struct {
	fn   string
	stmt Stmt
}

type chainChecker struct {
	m        *Module
	fn       *Func
	l        *lowerer
	consumed map[*CallExpr]bool
	uses     []chainCallbackUse
}

func (ck *chainChecker) errorf(s Stmt, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if loc, ok := ck.m.SourceLocations[s]; ok && loc.Line > 0 {
		return fmt.Errorf("line %d: %s", loc.Line, msg)
	}
	return fmt.Errorf("%s: %s", ck.fn.Name, msg)
}

// collectAliases records local names bound to functions (let f = |x| ...),
// flow-insensitively, as the lowerer's fnAliases would at the chain.
func (ck *chainChecker) collectAliases(b *Block) {
	walkChainBlocks(b, func(s Stmt) {
		d, ok := s.(*VarDeclStmt)
		if !ok {
			return
		}
		if vr, ok := d.Init.(*VarRefExpr); ok {
			if ck.l.hirFuncNames[vr.Name] {
				ck.l.fnAliases[d.Name] = vr.Name
			} else if a, ok := ck.l.fnAliases[vr.Name]; ok {
				ck.l.fnAliases[d.Name] = a
			}
		}
	})
}

func walkChainBlocks(b *Block, visit func(Stmt)) {
	if b == nil {
		return
	}
	for _, s := range b.Body {
		visit(s)
		switch st := s.(type) {
		case *Block:
			walkChainBlocks(st, visit)
		case *IfStmt:
			walkChainBlocks(st.Then, visit)
			walkChainBlocks(st.Else, visit)
		case *WhileStmt:
			walkChainBlocks(st.Body, visit)
		case *ForRangeStmt:
			walkChainBlocks(st.Body, visit)
		case *ForEachStmt:
			walkChainBlocks(st.Body, visit)
		case *GenForStmt:
			walkChainBlocks(st.Body, visit)
		case *SwitchStmt:
			for _, c := range st.Cases {
				walkChainBlocks(c.Body, visit)
			}
			walkChainBlocks(st.Default, visit)
		}
	}
}

func (ck *chainChecker) isChainCall(e Expr) (*CallExpr, bool) {
	c, ok := e.(*CallExpr)
	if !ok || ck.l.hirFuncNames[c.Fn] || !isIterChainMethod(c.Fn) {
		return nil, false
	}
	return c, true
}

func (ck *chainChecker) block(b *Block) error {
	var err error
	walkChainBlocks(b, func(s Stmt) {
		if err != nil {
			return
		}
		switch st := s.(type) {
		case *ExprStmt:
			if c, ok := ck.isChainCall(st.Expr); ok && isIterTerminal(c.Fn) {
				err = ck.root(s, c)
			}
		case *VarDeclStmt:
			if c, ok := ck.isChainCall(st.Init); ok && isValueIterTerminal(c.Fn) {
				err = ck.root(s, c)
			}
		case *ReturnStmt:
			if c, ok := ck.isChainCall(st.Val); ok && isValueIterTerminal(c.Fn) && len(st.Vals) == 0 {
				err = ck.root(s, c)
			}
		case *AssignStmt:
			if c, ok := ck.isChainCall(st.Val); ok && isValueIterTerminal(c.Fn) {
				err = ck.root(s, c)
			}
		case *StoreStmt:
			if c, ok := ck.isChainCall(st.Val); ok && isValueIterTerminal(c.Fn) {
				err = ck.root(s, c)
			}
		}
	})
	return err
}

// root validates one chain in a lowered position and marks its calls consumed.
func (ck *chainChecker) root(s Stmt, call *CallExpr) error {
	use := func(fn string) {
		if fn != "" {
			ck.uses = append(ck.uses, chainCallbackUse{fn: fn, stmt: s})
		}
	}
	if ck.l.usesNewChainMethods(call) {
		c, err := ck.l.recognizeChain(call)
		if err != nil {
			return ck.errorf(s, "unsupported iterator chain: %v", err)
		}
		use(c.cb)
		for _, st := range c.stages {
			use(st.fn)
		}
	} else {
		chain, ok := ck.l.recognizeIterChain(call)
		if !ok {
			return ck.errorf(s, "unsupported iterator chain .%s(...): %s", call.Fn, oldChainShape(call.Fn))
		}
		use(chain.cbName)
		for _, st := range chain.stages {
			use(st.fn)
		}
		if !chain.isRange {
			if chain.len == nil {
				return ck.errorf(s, ".%s on a memory source needs the element count as its last argument", call.Fn)
			}
			if c, ok := ck.isChainCall(chain.ptr); ok {
				if c.Fn == "map" || c.Fn == "filter" {
					return ck.errorf(s, ".%s: the callback must be a function name or a lambda taking one element", c.Fn)
				}
				return ck.errorf(s, ".%s cannot be the source of .%s", c.Fn, call.Fn)
			}
		}
	}
	// Mark the terminal and its stage nest consumed.
	ck.consumed[call] = true
	e := call.Args[0]
	for {
		c, ok := ck.isChainCall(e)
		if !ok || len(c.Args) == 0 {
			break
		}
		ck.consumed[c] = true
		e = c.Args[0]
	}
	return nil
}

func oldChainShape(fn string) string {
	switch fn {
	case "forEach":
		return "expected src.forEach(callback, n) on memory or range(lo..hi).forEach(callback), with a function or lambda callback"
	case "mapInPlace":
		return "expected src.mapInPlace(callback, n) on memory, with a function or lambda callback"
	case "fold", "reduce":
		return "expected src." + fn + "(init, callback, n) on memory or range(lo..hi)." + fn + "(init, callback), with a function or lambda callback"
	}
	return "not a supported chain shape"
}

// leftovers rejects chain method calls that no lowered chain consumes.
func (ck *chainChecker) leftovers(b *Block) error {
	var err error
	walkChainBlocks(b, func(s Stmt) {
		if err != nil {
			return
		}
		visitStmtExprs(s, func(e Expr) {
			if err != nil {
				return
			}
			c, ok := ck.isChainCall(e)
			if !ok || ck.consumed[c] {
				return
			}
			if isIterTerminal(c.Fn) {
				if isValueIterTerminal(c.Fn) {
					err = ck.errorf(s, ".%s(...) must be the whole value of a let, an assignment or a return (or a statement on its own); bind it with let first", c.Fn)
				} else {
					err = ck.errorf(s, ".%s(...) must be a statement on its own", c.Fn)
				}
				return
			}
			err = ck.errorf(s, ".%s(...) is an iterator chain stage; end the chain with .forEach, .mapInPlace, .fold, .reduce, .count or .sum", c.Fn)
		})
	})
	return err
}
