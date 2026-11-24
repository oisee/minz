package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

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
//
// It runs before Verify (an unknown method's call has no type, and Verify
// would only report that), so it must not trip over HIR that Verify exists to
// reject: a nil module, function, body, statement or expression is skipped,
// and any panic on such malformed input is swallowed — the error is left to
// Verify, which runs next.
func CheckIterChains(m *Module) (err error) {
	if m == nil {
		return nil
	}
	defer func() {
		if recover() != nil {
			err = nil // malformed HIR: Verify reports it
		}
	}()
	names := make(map[string]bool, len(m.Funcs))
	funcs := make(map[string]*Func, len(m.Funcs))
	for _, f := range m.Funcs {
		if f == nil {
			continue
		}
		names[f.Name] = true
		funcs[f.Name] = f
	}
	if len(m.MethodCallLines) > 0 {
		var bad *CallExpr
		for call := range m.MethodCallLines {
			if call == nil || names[call.Fn] || isIterChainMethod(call.Fn) {
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
		if f == nil || f.Body == nil {
			continue
		}
		arrays := map[string]mir2.Ty{}
		for _, g := range m.Globals {
			if at, isArray := mir2.BaseOf(g.Ty).(*mir2.ArrayTy); isArray {
				arrays[g.Name] = at
			}
		}
		collectLocalArrays(f.Body, arrays)
		ck := &chainChecker{
			m: m, fn: f,
			l:        &lowerer{hirFuncNames: names, hirFuncs: funcs, fnAliases: map[string]string{}, chainArrays: arrays},
			consumed: map[*CallExpr]bool{},
		}
		ck.collectAliases(f.Body)
		if err := ck.block(f.Body); err != nil {
			return err
		}
		if err := ck.leftovers(f.Body); err != nil {
			return err
		}
		for _, fn := range ck.inlined {
			callbacks[fn] = true
		}
		uses[f] = ck.uses
	}
	// A callback inlined into an outer callback's body may not capture that
	// outer callback's parameter: inlining renames the parameter, and the
	// inner callback's own body would then name an undefined variable. Only
	// forEach callbacks inlined as statements count; a callback that must be
	// called (early return, asm) keeps its parameter and is fine.
	for _, f := range m.Funcs {
		if f == nil || !callbacks[f.Name] {
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
	inlined  []string // forEach callbacks inlined as statement bodies
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
		if nilHIR(s) {
			continue
		}
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
	// Only method-syntax calls are checked: the functional spellings
	// (forEach(src, cb, n), fold(...), a plain sum(...)) keep their meaning.
	if !ok || !ck.methodSyntax(c) || !ck.l.isChainCall(c) {
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
		var e Expr
		value := true // only fold/reduce/count/sum may be a value
		switch st := s.(type) {
		case *ExprStmt:
			e, value = st.Expr, false
		case *VarDeclStmt:
			e = st.Init
		case *ReturnStmt:
			if len(st.Vals) == 0 {
				e = st.Val
			}
		case *AssignStmt:
			e = st.Val
		case *StoreStmt:
			e = st.Val
		}
		c, isCall := e.(*CallExpr)
		if !isCall || !ck.l.isChainCall(c) || !isIterTerminal(c.Fn) || value && !isValueIterTerminal(c.Fn) {
			return
		}
		// A functional terminal (fold(src, ...) with no method syntax) keeps
		// origin/main's meaning: lowered when recognised, never diagnosed for
		// itself; its method-syntax stages count as consumed when it is.
		err = ck.root(s, c, !ck.methodSyntax(c))
	})
	return err
}

// root validates one chain in a lowered position and marks its calls consumed.
func (ck *chainChecker) root(s Stmt, call *CallExpr, functional bool) error {
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
		if c.term == "forEach" {
			ck.inlinedForEach(c.cb)
		}
	} else {
		chain, ok := ck.l.recognizeIterChain(call)
		if !ok {
			if functional {
				return nil
			}
			return ck.errorf(s, "unsupported iterator chain .%s(...): %s", call.Fn, oldChainShape(call.Fn))
		}
		use(chain.cbName)
		for _, st := range chain.stages {
			use(st.fn)
		}
		if !chain.mutate && !chain.isFold {
			ck.inlinedForEach(chain.cbName)
		}
		if err := oldChainElemErr(chain); err != "" && !functional {
			return ck.errorf(s, "unsupported iterator chain .%s(...): %s", call.Fn, err)
		}
		if !chain.isRange && !functional {
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
		c, ok := e.(*CallExpr)
		if !ok || !ck.l.isChainCall(c) || len(c.Args) == 0 {
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

// inlinedForEach records a forEach callback that the lowering inlines as
// statements (the same test lowerFusedForEach and lowerRangeForEach use).
func (ck *chainChecker) inlinedForEach(fn string) {
	if f := ck.l.hirFuncs[fn]; f != nil && f.Body != nil && !iterCallbackNeedsCall(f.Body) {
		ck.inlined = append(ck.inlined, fn)
	}
}

// oldChainElemErr rejects struct elements in an original chain: a struct
// element parameter (typed or from the hint), and a non-scalar parameter over
// a struct array. On origin/main every such shape loaded the wrong bytes or
// emitted invalid assembly. A scalar parameter walking a struct or nested
// array as bytes or words, and an unread array parameter, worked there and
// are kept.
func oldChainElemErr(chain iterChain) string {
	if _, isStruct := mir2.BaseOf(chain.elemTy).(*mir2.StructTy); isStruct {
		return fmt.Sprintf("struct elements (%s) are not supported; take a pointer and index it in a for loop", chain.elemTy)
	}
	if mir2.IsPointer(chain.elemTy) {
		if _, isStruct := mir2.BaseOf(mir2.PointerElem(chain.elemTy)).(*mir2.StructTy); isStruct {
			return fmt.Sprintf("struct pointer elements (%s) are not supported", chain.elemTy)
		}
	}
	if d := typedElemTy(chain.ptr); d != nil && !chain.isRange {
		if _, isStruct := mir2.BaseOf(d).(*mir2.StructTy); isStruct && !mir2.IsInt(mir2.BaseOf(chain.elemTy)) && chain.elemTy != mir2.TyBool {
			return fmt.Sprintf("the source's elements are %s; walk them with a u8/u16 parameter or index them in a for loop", d)
		}
	}
	return ""
}

// methodSyntax: the frontend saw receiver.name(...) for this call — the node's
// semantic mark, or the module's record of method-syntax calls (the mark is
// set only where it changes the meaning; see MethodMarked).
func (ck *chainChecker) methodSyntax(c *CallExpr) bool {
	return c.Method || ck.m.MethodSyntax[c]
}
