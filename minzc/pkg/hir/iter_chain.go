package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// ── Iterator chains: skip, take, zip, count, sum ─────────────────────────────
//
// The original chain fusion (recognizeIterChain, lowerFusedForEach,
// lowerFusedFold, lowerRange*) handles map/filter stages ending in
// forEach/mapInPlace/fold/reduce. A chain that uses any of the methods below
// is recognised and lowered here instead; every other chain keeps the original
// path, so its output is unchanged.
//
//	src.skip(k)          stage: drop the first k elements that reach it
//	src.take(k)          stage: pass the first k elements that reach it, then stop
//	src.zip(other)       pair source element i with other[i] (by source position)
//	src.count(n)         terminal: u16 number of elements reaching it
//	src.sum(n)           terminal: u16 wrapping sum of the elements reaching it
//
// k is evaluated once, before the loop. skip/take directly on the source (or
// on src.zip(other)), before any map or filter, cost nothing per element: they
// adjust the start pointer(s) and the element count before the loop. Elsewhere
// a counter is kept; take stops the loop with a break before any stage runs
// for the element after the k-th accepted one (ADR 0050: take counts accepted
// outputs, not inspected source items).
//
// A name is a chain method only when the module declares no function of that
// name: `x.sum(n)` is ordinary UFCS for a user `fun sum(...)`.

// isIterChainMethod reports whether name is an iterator chain stage or terminal.
func isIterChainMethod(name string) bool {
	switch name {
	case "map", "filter", "skip", "take", "zip",
		"forEach", "mapInPlace", "fold", "reduce", "count", "sum":
		return true
	}
	return false
}

// isNewIterChainMethod reports the methods lowered by this file.
func isNewIterChainMethod(name string) bool {
	switch name {
	case "skip", "take", "zip", "count", "sum":
		return true
	}
	return false
}

// isChainCall reports whether call is an iterator chain method here: a chain
// method name that is no module function, and for the methods added with
// this file (skip, take, zip, count, sum) written with method syntax — a
// plain sum(buf, 4) may call an assembly label of that name.
func (l *lowerer) isChainCall(call *CallExpr) bool {
	if call == nil || l.hirFuncNames[call.Fn] || !isIterChainMethod(call.Fn) {
		return false
	}
	return call.Method || !isNewIterChainMethod(call.Fn)
}

func isIterTerminal(name string) bool {
	switch name {
	case "forEach", "mapInPlace", "fold", "reduce", "count", "sum":
		return true
	}
	return false
}

// isValueIterTerminal: terminals that produce a value (usable as let/assign/return RHS).
func isValueIterTerminal(name string) bool {
	switch name {
	case "fold", "reduce", "count", "sum":
		return true
	}
	return false
}

type chainStage struct {
	kind string  // "map", "filter", "skip", "take"
	fn   string  // resolved callback (map/filter)
	k    Expr    // skip/take count as written
	kTy  mir2.Ty // type of k
	head bool    // skip/take folded into the start pointer / element count
	kRef Expr    // k after single evaluation (literal or temp)
	cnt  string  // counter variable (non-head skip/take)
}

type chainSpec struct {
	term     string // forEach, mapInPlace, fold, reduce, count, sum
	src      Expr   // memory source (nil for range)
	isRange  bool
	rangeLo  Expr
	rangeHi  Expr
	rangeRev bool
	n        Expr // element count (memory only)
	zip      Expr // second memory source (nil when no zip)
	stages   []chainStage
	cb       string // forEach/mapInPlace/fold callback
	init     Expr   // fold init
	elemTy   mir2.Ty
	zipTy    mir2.Ty
	accTy    mir2.Ty
}

// usesNewChainMethods reports whether the chain rooted at call needs this
// file's lowering: a count/sum terminal, or skip/take/zip anywhere in the
// source nest. Names declared as module functions are ordinary calls.
func (l *lowerer) usesNewChainMethods(call *CallExpr) bool {
	if call == nil || l.hirFuncNames[call.Fn] || !isIterTerminal(call.Fn) || len(call.Args) == 0 {
		return false
	}
	if call.Fn == "count" || call.Fn == "sum" {
		return call.Method
	}
	// A memory terminal without its count over a sized array takes the
	// declared array length (forEach/mapInPlace(cb), fold/reduce(init, cb)) —
	// method syntax only; the functional spellings keep their meaning.
	short := call.Method && (len(call.Args) == 2 && (call.Fn == "forEach" || call.Fn == "mapInPlace") ||
		len(call.Args) == 3 && (call.Fn == "fold" || call.Fn == "reduce"))
	e := call.Args[0]
	for {
		inner, ok := e.(*CallExpr)
		if !ok || l.hirFuncNames[inner.Fn] || len(inner.Args) == 0 {
			return short && sizedArrayLen(l.retypeArraySource(e)) > 0
		}
		switch inner.Fn {
		case "skip", "take", "zip":
			if inner.Method {
				return true
			}
			return short && sizedArrayLen(l.retypeArraySource(e)) > 0
		case "map", "filter":
			e = inner.Args[0]
		default:
			return short && sizedArrayLen(l.retypeArraySource(e)) > 0
		}
	}
}

// sizedArrayLen returns the declared length of a sized array source (a
// global `[T; N]`), 0 otherwise.
func sizedArrayLen(e Expr) int {
	if e == nil {
		return 0
	}
	if at, ok := mir2.BaseOf(e.ExprTy()).(*mir2.ArrayTy); ok && at.Len > 0 {
		return at.Len
	}
	return 0
}

// chainLitTy returns the type of a skip/take count or an element count: u8
// or u16 only (an untyped literal takes the narrowest of the two), nil for a
// signed, bool, wider or negative value — those would count differently at
// the head (bounds) and mid-chain (counters).
func chainLitTy(k Expr) mir2.Ty {
	if lit, ok := k.(*IntLitExpr); ok {
		if lit.Val < 0 || lit.Val > 0xFFFF {
			return nil
		}
		if lit.Untyped || lit.Ty == nil {
			if lit.Val > 255 {
				return mir2.TyU16
			}
			return mir2.TyU8
		}
	}
	switch ty := mir2.BaseOf(k.ExprTy()); ty {
	case mir2.TyU8, mir2.TyU16:
		return ty
	}
	return nil
}

// isMemorySource: an array or pointer value (the legacy untyped ptr
// included). Scalars and bools are not sources.
func isMemorySource(e Expr) bool {
	ty := mir2.BaseOf(e.ExprTy())
	if _, isArray := ty.(*mir2.ArrayTy); isArray {
		return true
	}
	return mir2.IsPointer(ty)
}

// loadableElem reports whether ty is an element type the fused loop can load
// and pass to a callback: u8, i8, bool, u16 or i16.
func loadableElem(ty mir2.Ty) bool {
	switch mir2.BaseOf(ty) {
	case mir2.TyU8, mir2.TyI8, mir2.TyBool, mir2.TyU16, mir2.TyI16:
		return true
	}
	return false
}

// typedElemTy is declaredElemTy for sources whose type really names the
// element (arrays and typed pointers); nil for the legacy untyped pointer.
func typedElemTy(e Expr) mir2.Ty {
	if e == nil || e.ExprTy() == mir2.TyPtr {
		return nil
	}
	return declaredElemTy(e)
}

// declaredElemTy returns the element type of a memory source when its static
// type names one (array or typed pointer), nil otherwise.
func declaredElemTy(e Expr) mir2.Ty {
	if e == nil {
		return nil
	}
	switch ty := mir2.BaseOf(e.ExprTy()).(type) {
	case *mir2.ArrayTy:
		return ty.Elem
	}
	if mir2.IsPointer(e.ExprTy()) {
		return mir2.PointerElem(e.ExprTy())
	}
	return nil
}

// recognizeChain parses a chain that usesNewChainMethods. The error names the
// first unsupported construct.
func (l *lowerer) recognizeChain(call *CallExpr) (*chainSpec, error) {
	c := &chainSpec{term: call.Fn}
	args := call.Args
	var lenArg Expr
	hasLen := false
	switch call.Fn {
	case "forEach", "mapInPlace":
		if len(args) != 2 && len(args) != 3 {
			return nil, fmt.Errorf(".%s(callback, n) takes a callback and, for a memory source, an element count", call.Fn)
		}
		c.cb = l.resolveFunc(args[1])
		if c.cb == "" {
			return nil, fmt.Errorf(".%s: the callback must be a function name or a lambda", call.Fn)
		}
		if len(args) == 3 {
			lenArg, hasLen = args[2], true
		}
	case "fold", "reduce":
		if len(args) != 3 && len(args) != 4 {
			return nil, fmt.Errorf(".%s(init, callback, n) takes an initial value, a callback and, for a memory source, an element count", call.Fn)
		}
		c.init = args[1]
		c.cb = l.resolveFunc(args[2])
		if c.cb == "" {
			return nil, fmt.Errorf(".%s: the callback must be a function name or a lambda", call.Fn)
		}
		if len(args) == 4 {
			lenArg, hasLen = args[3], true
		}
	case "count", "sum":
		if len(args) != 1 && len(args) != 2 {
			return nil, fmt.Errorf(".%s(n) takes one argument, the element count (none for a range source)", call.Fn)
		}
		if len(args) == 2 {
			lenArg, hasLen = args[1], true
		}
		c.accTy = mir2.TyU16
	default:
		return nil, fmt.Errorf(".%s is not an iterator chain terminal", call.Fn)
	}

	// Unwrap the source nest (outermost first).
	var rev []chainStage
	src := args[0]
	for {
		inner, ok := src.(*CallExpr)
		if !ok || !l.isChainCall(inner) {
			break
		}
		if c.zip != nil {
			return nil, fmt.Errorf("a chain takes one .zip(other), applied directly to the source (src.zip(other)...); found .%s before it", inner.Fn)
		}
		switch inner.Fn {
		case "map", "filter":
			if len(inner.Args) != 2 {
				return nil, fmt.Errorf(".%s takes exactly one callback", inner.Fn)
			}
			fn := l.resolveFunc(inner.Args[1])
			if fn == "" {
				return nil, fmt.Errorf(".%s: the callback must be a function name or a lambda", inner.Fn)
			}
			rev = append(rev, chainStage{kind: inner.Fn, fn: fn})
		case "skip", "take":
			if len(inner.Args) != 2 {
				return nil, fmt.Errorf(".%s(k) takes exactly one argument", inner.Fn)
			}
			kTy := chainLitTy(inner.Args[1])
			if kTy == nil {
				return nil, fmt.Errorf(".%s(k): k must be a u8 or u16 value or a non-negative integer literal, got %s", inner.Fn, inner.Args[1].ExprTy())
			}
			rev = append(rev, chainStage{kind: inner.Fn, k: inner.Args[1], kTy: kTy})
		case "zip":
			if len(inner.Args) != 2 {
				return nil, fmt.Errorf(".zip(other) takes exactly one memory source")
			}
			c.zip = inner.Args[1]
			if _, isRange := c.zip.(*RangeSourceExpr); isRange {
				return nil, fmt.Errorf(".zip(other): other must be a memory source (array or pointer), not a range")
			}
			if oc, ok := c.zip.(*CallExpr); ok && l.isChainCall(oc) {
				return nil, fmt.Errorf(".zip(other): other must be a plain memory source, not an iterator chain")
			}
		default:
			return nil, fmt.Errorf(".%s is a terminal; it cannot be the source of another chain method", inner.Fn)
		}
		src = inner.Args[0]
	}
	for i := len(rev) - 1; i >= 0; i-- {
		c.stages = append(c.stages, rev[i])
	}

	if rs, ok := src.(*RangeSourceExpr); ok {
		if c.zip != nil {
			return nil, fmt.Errorf(".zip is supported on memory sources only, not on a range")
		}
		if c.term == "mapInPlace" {
			return nil, fmt.Errorf(".mapInPlace needs memory to store into; a range has none")
		}
		if hasLen {
			return nil, fmt.Errorf(".%s on a range source takes no element count", c.term)
		}
		c.isRange = true
		c.rangeLo, c.rangeHi, c.rangeRev = rs.Lo, rs.Hi, rs.Rev
	} else {
		src = l.retypeArraySource(src)
		c.src = src
		if c.zip != nil {
			c.zip = l.retypeArraySource(c.zip)
		}
		if !isMemorySource(src) {
			return nil, fmt.Errorf(".%s: the source must be an array or a pointer, got %s", c.term, src.ExprTy())
		}
		if c.zip != nil && !isMemorySource(c.zip) {
			return nil, fmt.Errorf(".zip(other): other must be an array or a pointer, got %s", c.zip.ExprTy())
		}
		if hasLen {
			c.n = lenArg
		} else if n := sizedArrayLen(src); n > 0 {
			// Implicit count: the declared length, and with zip the shorter
			// of the two declared lengths (other may be a plain pointer).
			if c.zip != nil {
				if m := sizedArrayLen(c.zip); m > 0 && m < n {
					n = m
				}
			}
			ty := mir2.Ty(mir2.TyU8)
			if n > 255 {
				ty = mir2.TyU16
			}
			c.n = &IntLitExpr{Val: int64(n), Ty: ty}
		} else {
			return nil, fmt.Errorf(".%s on a memory source needs the element count as its last argument", c.term)
		}
		if chainLitTy(c.n) == nil {
			return nil, fmt.Errorf(".%s: the element count must be a u8 or u16 value or a non-negative integer literal, got %s", c.term, c.n.ExprTy())
		}
	}

	// Head skip/take: directly on the (zipped) memory source.
	if !c.isRange {
		for i := range c.stages {
			if c.stages[i].kind != "skip" && c.stages[i].kind != "take" {
				break
			}
			c.stages[i].head = true
		}
	}
	if c.term == "mapInPlace" {
		if c.zip != nil {
			return nil, fmt.Errorf(".mapInPlace does not support .zip")
		}
		for _, s := range c.stages {
			if (s.kind == "skip" || s.kind == "take") && !s.head {
				return nil, fmt.Errorf(".mapInPlace supports .%s only directly on the source, before any map or filter", s.kind)
			}
		}
	}

	if err := l.typeChain(c); err != nil {
		return nil, err
	}
	return c, nil
}

// typeChain checks callback arities against the paired/single element state
// and fixes the element types.
func (l *lowerer) typeChain(c *chainSpec) error {
	// Element (load) type: the first callback's element parameter, else the
	// source's declared element type, else u8 (as the original chain path).
	first := func() (*Func, int) {
		for _, s := range c.stages {
			if s.kind == "map" || s.kind == "filter" {
				return l.hirFuncs[s.fn], 0
			}
		}
		switch c.term {
		case "forEach", "mapInPlace":
			return l.hirFuncs[c.cb], 0
		case "fold", "reduce":
			return l.hirFuncs[c.cb], 1
		}
		return nil, 0
	}
	if fn, idx := first(); fn != nil && len(fn.Params) > idx {
		c.elemTy = fn.Params[idx].Ty
	} else if c.isRange {
		c.elemTy = RangeType(c.rangeLo, c.rangeHi)
	} else if ty := declaredElemTy(c.src); ty != nil {
		c.elemTy = ty
	} else {
		c.elemTy = mir2.TyU8
	}
	if c.zip != nil {
		c.zipTy = nil
		paired := true
		check := func(fn *Func, off int) {
			if paired && c.zipTy == nil && fn != nil && len(fn.Params) > off+1 {
				c.zipTy = fn.Params[off+1].Ty
			}
		}
		for _, s := range c.stages {
			if s.kind == "map" || s.kind == "filter" {
				check(l.hirFuncs[s.fn], 0)
				if s.kind == "map" {
					paired = false
				}
			}
		}
		switch c.term {
		case "forEach":
			check(l.hirFuncs[c.cb], 0)
		case "fold", "reduce":
			check(l.hirFuncs[c.cb], 1)
		}
		if c.zipTy == nil {
			c.zipTy = declaredElemTy(c.zip)
		}
		if c.zipTy == nil {
			c.zipTy = mir2.TyU8
		}
	}

	// Element types must be scalars the loop can load, and agree in size with
	// what the source's own type declares (a u16 array walked with a u8
	// callback parameter would step bytes while the count is in elements).
	if !loadableElem(c.elemTy) {
		return fmt.Errorf("element type %s is not supported in a fused chain (u8, i8, bool, u16, i16)", c.elemTy)
	}
	if d := typedElemTy(c.src); d != nil && !loadableElem(d) {
		return fmt.Errorf("the source's elements are %s; a fused chain walks u8, i8, bool, u16 or i16 elements", d)
	}
	if c.zip != nil {
		if d := typedElemTy(c.zip); d != nil && !loadableElem(d) {
			return fmt.Errorf(".zip: other's elements are %s; a fused chain walks u8, i8, bool, u16 or i16 elements", d)
		}
	}
	if d := typedElemTy(c.src); d != nil && mir2.ByteWidth(d) != mir2.ByteWidth(c.elemTy) {
		return fmt.Errorf("the source's elements are %s but the first callback takes %s", d, c.elemTy)
	}
	if c.zip != nil {
		if !loadableElem(c.zipTy) {
			return fmt.Errorf(".zip: element type %s is not supported in a fused chain (u8, i8, bool, u16, i16)", c.zipTy)
		}
		if d := typedElemTy(c.zip); d != nil && mir2.ByteWidth(d) != mir2.ByteWidth(c.zipTy) {
			return fmt.Errorf(".zip: other's elements are %s but the callback's second element parameter is %s", d, c.zipTy)
		}
	}

	paired := c.zip != nil
	cur := c.elemTy
	want := func(what, fn string, extra int) error {
		f := l.hirFuncs[fn]
		if f == nil {
			return nil
		}
		n := 1
		shape := "one element parameter"
		if paired {
			n = 2
			shape = "two element parameters (after .zip)"
		}
		if len(f.Params) != n+extra {
			if extra > 0 {
				return fmt.Errorf("%s callback %s takes %d parameters; it needs the accumulator and %s", what, fn, len(f.Params), shape)
			}
			return fmt.Errorf("%s callback %s takes %d parameters; it needs %s", what, fn, len(f.Params), shape)
		}
		return nil
	}
	for i := range c.stages {
		s := &c.stages[i]
		switch s.kind {
		case "map":
			if err := want(".map", s.fn, 0); err != nil {
				return err
			}
			if f := l.hirFuncs[s.fn]; f != nil {
				if f.RetTy == nil || f.RetTy == mir2.TyVoid {
					return fmt.Errorf(".map callback %s must return a value", s.fn)
				}
				if !loadableElem(f.RetTy) {
					return fmt.Errorf(".map callback %s returns %s; fused chain elements are u8, i8, bool, u16 or i16", s.fn, f.RetTy)
				}
				cur = f.RetTy
			}
			paired = false
		case "filter":
			if err := want(".filter", s.fn, 0); err != nil {
				return err
			}
		}
	}
	switch c.term {
	case "forEach", "mapInPlace":
		if err := want("."+c.term, c.cb, 0); err != nil {
			return err
		}
	case "fold", "reduce":
		if err := want("."+c.term, c.cb, 1); err != nil {
			return err
		}
		c.accTy = mir2.TyU8
		if f := l.hirFuncs[c.cb]; f != nil && f.RetTy != nil && f.RetTy != mir2.TyVoid {
			c.accTy = f.RetTy
		}
	case "sum":
		if paired {
			return fmt.Errorf(".sum after .zip needs a .map(|x, y| ...) to one element first")
		}
		if !mir2.IsInt(mir2.BaseOf(cur)) || cur.Width() > 16 {
			return fmt.Errorf(".sum needs u8 or u16 elements, got %s", cur)
		}
	}
	if c.term == "mapInPlace" {
		// It stores the callback's result where the element was loaded: every
		// stage and the callback must keep the loaded element type.
		f := l.hirFuncs[c.cb]
		if !l.keepsElemTy(c) || f != nil && f.RetTy != c.elemTy {
			return fmt.Errorf(".mapInPlace stores back the element type it loads (%s); every map, filter and the mapInPlace callback must take and return %s", c.elemTy, c.elemTy)
		}
	}
	return nil
}

// chainElem is the element state flowing through the stages.
type chainElem struct {
	v, zv  string // element and (when paired) zip element variables
	ty, zt mir2.Ty
	paired bool
}

func (e chainElem) args() []Expr {
	a := []Expr{&VarRefExpr{Name: e.v, Ty: e.ty}}
	if e.paired {
		a = append(a, &VarRefExpr{Name: e.zv, Ty: e.zt})
	}
	return a
}

// renameParams renames the callback's element parameters (after skip leading
// non-element parameters) onto the element variables.
func renameParamsExpr(x Expr, f *Func, skip int, e chainElem) Expr {
	x = renameExpr(x, f.Params[skip].Name, e.v)
	if e.paired && len(f.Params) > skip+1 {
		x = renameExpr(x, f.Params[skip+1].Name, e.zv)
	}
	return x
}

// chainValue is the value of a map/filter callback on the current element.
func (l *lowerer) chainValue(fn string, e chainElem, ty mir2.Ty) Expr {
	if f := l.hirFuncs[fn]; f != nil && len(f.Params) >= 1 {
		if ret := extractReturnExpr(f.Body); ret != nil {
			return renameParamsExpr(ret, f, 0, e)
		}
	}
	return &CallExpr{Fn: fn, Args: e.args(), Ty: ty}
}

func chainLit(v int64, ty mir2.Ty) *IntLitExpr { return &IntLitExpr{Val: v, Ty: ty} }

func chainCast(x Expr, ty mir2.Ty) Expr {
	if x.ExprTy() == ty {
		return x
	}
	if lit, ok := x.(*IntLitExpr); ok && lit.Val >= 0 && lit.Val < int64(1)<<uint(ty.Width()) {
		return &IntLitExpr{Val: lit.Val, Ty: ty}
	}
	return Cast(x, ty)
}

func wider(a, b mir2.Ty) mir2.Ty {
	if b.Width() > a.Width() {
		return b
	}
	return a
}

// chainOnce evaluates k once before the loop: literals stay literals.
func (l *lowerer) chainOnce(k Expr, ty mir2.Ty) Expr {
	if lit, ok := k.(*IntLitExpr); ok {
		return chainLit(lit.Val, ty)
	}
	name := l.fresh("__iter_k")
	r := l.lowerExprAs(k, ty)
	l.bind(name, r, ty)
	return &VarRefExpr{Name: name, Ty: ty}
}

// chainBody builds the per-element statements from stage i on, nested so a
// failing filter or an active skip simply skips the rest (no continue).
func (l *lowerer) chainBody(c *chainSpec, i int, e chainElem, acc string) []Stmt {
	if i == len(c.stages) {
		return l.chainTerminal(c, e, acc)
	}
	s := c.stages[i]
	if s.head {
		return l.chainBody(c, i+1, e, acc)
	}
	switch s.kind {
	case "map":
		f := l.hirFuncs[s.fn]
		ty := e.ty
		if f != nil && f.RetTy != nil && f.RetTy != mir2.TyVoid {
			ty = f.RetTy
		}
		val := l.chainValue(s.fn, e, ty)
		next := chainElem{v: e.v, ty: e.ty}
		var st Stmt
		if ty == e.ty {
			st = &AssignStmt{Target: &VarRefExpr{Name: e.v, Ty: e.ty}, Val: val}
		} else {
			next.v, next.ty = l.fresh("__iter_map"), ty
			st = &VarDeclStmt{Name: next.v, Ty: ty, Init: val}
		}
		return append([]Stmt{st}, l.chainBody(c, i+1, next, acc)...)
	case "filter":
		cond := l.chainValue(s.fn, e, mir2.TyBool)
		return []Stmt{&IfStmt{Cond: cond, Then: &Block{Body: l.chainBody(c, i+1, e, acc)}}}
	case "skip":
		cnt := &VarRefExpr{Name: s.cnt, Ty: s.kTy}
		bump := &AssignStmt{Target: cnt, Val: &BinExpr{Op: "+", L: cnt, R: chainLit(1, s.kTy), Ty: s.kTy}}
		return []Stmt{&IfStmt{
			Cond: &BinExpr{Op: "<", L: cnt, R: s.kRef, Ty: mir2.TyBool},
			Then: &Block{Body: []Stmt{bump}},
			Else: &Block{Body: l.chainBody(c, i+1, e, acc)},
		}}
	case "take":
		cnt := &VarRefExpr{Name: s.cnt, Ty: s.kTy}
		bump := &AssignStmt{Target: cnt, Val: &BinExpr{Op: "+", L: cnt, R: chainLit(1, s.kTy), Ty: s.kTy}}
		return append([]Stmt{bump}, l.chainBody(c, i+1, e, acc)...)
	}
	panic("hir/lower: unknown chain stage " + s.kind)
}

func (l *lowerer) chainTerminal(c *chainSpec, e chainElem, acc string) []Stmt {
	accRef := &VarRefExpr{Name: acc, Ty: c.accTy}
	switch c.term {
	case "count":
		return []Stmt{&AssignStmt{Target: accRef, Val: &BinExpr{Op: "+", L: accRef, R: chainLit(1, mir2.TyU16), Ty: mir2.TyU16}}}
	case "sum":
		var x Expr = &VarRefExpr{Name: e.v, Ty: e.ty}
		if mir2.IsSigned(e.ty) { // zero-extend: reinterpret as unsigned first
			x = Cast(x, mir2.TyU8)
			if e.ty.Width() > 8 {
				x = Cast(&VarRefExpr{Name: e.v, Ty: e.ty}, mir2.TyU16)
			}
		}
		x = chainCast(x, mir2.TyU16)
		return []Stmt{&AssignStmt{Target: accRef, Val: &BinExpr{Op: "+", L: accRef, R: x, Ty: mir2.TyU16}}}
	case "fold", "reduce":
		f := l.hirFuncs[c.cb]
		var val Expr
		if f != nil && len(f.Params) >= 2 {
			if ret := extractReturnExpr(f.Body); ret != nil {
				val = renameExpr(ret, f.Params[0].Name, acc)
				val = renameParamsExpr(val, f, 1, e)
			}
		}
		if val == nil {
			val = &CallExpr{Fn: c.cb, Args: append([]Expr{accRef}, e.args()...), Ty: c.accTy}
		}
		return []Stmt{&AssignStmt{Target: accRef, Val: val}}
	case "forEach":
		f := l.hirFuncs[c.cb]
		if f == nil || f.Body == nil || iterCallbackNeedsCall(f.Body) || len(f.Params) < 1 {
			return []Stmt{&ExprStmt{Expr: &CallExpr{Fn: c.cb, Args: e.args(), Ty: mir2.TyVoid}}}
		}
		var out []Stmt
		if e.paired && len(f.Params) > 1 {
			l.zipInStmt = true
		}
		for _, s := range f.Body.Body {
			r := renameStmt(s, f.Params[0].Name, e.v, l.lineage)
			if e.paired && len(f.Params) > 1 {
				r = renameStmt(r, f.Params[1].Name, e.zv, l.lineage)
			}
			if _, isRet := r.(*ReturnStmt); isRet {
				continue
			}
			out = append(out, r)
		}
		return out
	}
	panic("hir/lower: unknown chain terminal " + c.term)
}

// tryLowerChain lowers a chain that uses skip/take/zip/count/sum. ok is false
// when expr is not such a chain (the original path then applies).
func (l *lowerer) tryLowerChain(expr Expr) (mir2.Reg, mir2.Ty, bool) {
	call, isCall := expr.(*CallExpr)
	if !isCall || !l.usesNewChainMethods(call) {
		return mir2.NoReg, nil, false
	}
	c, err := l.recognizeChain(call)
	if err != nil {
		panic(fmt.Sprintf("hir/lower: unsupported iterator chain: %v", err))
	}
	return l.lowerChain(c), c.accTy, true
}

// isOrderFreeArg: evaluating e has no effect and cannot observe one, so
// where it is evaluated relative to the other chain arguments is invisible.
func isOrderFreeArg(e Expr) bool {
	switch x := e.(type) {
	case nil, *IntLitExpr, *BoolLitExpr, *VarRefExpr:
		return true
	case *AddrOfExpr:
		return x.X == nil
	case *CastExpr:
		return isOrderFreeArg(x.X)
	}
	return false
}

// isConstArg: an order-free argument that reads no variable.
func isConstArg(e Expr) bool {
	switch x := e.(type) {
	case nil, *IntLitExpr, *BoolLitExpr:
		return true
	case *AddrOfExpr:
		return x.X == nil
	case *CastExpr:
		return isConstArg(x.X)
	case *VarRefExpr:
		_, isArray := mir2.BaseOf(x.Ty).(*mir2.ArrayTy)
		return isArray // an array's address is a constant
	}
	return false
}

// chainArgsInSourceOrder evaluates the chain's arguments once, in source
// order — the source (or range bounds), zip's argument, the stage arguments
// left to right, then the terminal's init and count — into temporaries, when
// any of them could observe the order. Otherwise the order is unobservable
// and nothing is emitted here.
func (l *lowerer) chainArgsInSourceOrder(c *chainSpec) {
	slots := []*Expr{}
	if c.isRange {
		slots = append(slots, &c.rangeLo, &c.rangeHi)
	} else {
		slots = append(slots, &c.src)
	}
	if c.zip != nil {
		slots = append(slots, &c.zip)
	}
	for i := range c.stages {
		if c.stages[i].kind == "skip" || c.stages[i].kind == "take" {
			slots = append(slots, &c.stages[i].k)
		}
	}
	if c.init != nil {
		slots = append(slots, &c.init)
	}
	if c.n != nil {
		slots = append(slots, &c.n)
	}
	free := true
	for _, p := range slots {
		free = free && isOrderFreeArg(*p)
	}
	if free {
		return
	}
	for _, p := range slots {
		if isConstArg(*p) {
			continue // literals, global and array addresses are constants
		}
		ty := (*p).ExprTy()
		name := l.fresh("__iter_arg")
		l.bind(name, l.lowerExpr(*p), ty)
		*p = &VarRefExpr{Name: name, Ty: ty}
	}
}

// keepsElemTy reports whether every callback sees and every map produces the
// source element type — the condition for the original single-variable
// builders.
func (l *lowerer) keepsElemTy(c *chainSpec) bool {
	same := func(fn string, idx int) bool {
		f := l.hirFuncs[fn]
		return f == nil || len(f.Params) <= idx || f.Params[idx].Ty == c.elemTy
	}
	for _, s := range c.stages {
		switch s.kind {
		case "map":
			if f := l.hirFuncs[s.fn]; !same(s.fn, 0) || f != nil && f.RetTy != c.elemTy {
				return false
			}
		case "filter":
			if !same(s.fn, 0) {
				return false
			}
		}
	}
	switch c.term {
	case "forEach", "mapInPlace":
		return same(c.cb, 0)
	case "fold", "reduce":
		return same(c.cb, 1)
	}
	return true
}

func (l *lowerer) lowerChain(c *chainSpec) mir2.Reg {
	l.chainArgsInSourceOrder(c)
	// Every skip/take count is evaluated exactly once, before the loop.
	for i := range c.stages {
		s := &c.stages[i]
		if s.kind == "skip" || s.kind == "take" {
			s.kRef = l.chainOnce(s.k, s.kTy)
		}
	}

	// Head skip/take: adjust the start index and element count.
	var start Expr = chainLit(0, mir2.TyU8)
	length := c.n
	if !c.isRange {
		nTy := mir2.BaseOf(c.n.ExprTy())
		var lenName, startName string
		for i := range c.stages {
			s := &c.stages[i]
			if !s.head {
				break
			}
			if lenName == "" {
				lenName = l.fresh("__iter_len")
				l.lowerStmt(&VarDeclStmt{Name: lenName, Ty: nTy, Init: c.n})
			}
			lr := &VarRefExpr{Name: lenName, Ty: nTy}
			w := wider(nTy, s.kTy)
			lw, kw := chainCast(lr, w), chainCast(s.kRef, w)
			switch s.kind {
			case "skip":
				// len = len > k ? len - k : 0
				l.lowerStmt(&IfStmt{
					Cond: &BinExpr{Op: ">", L: lw, R: kw, Ty: mir2.TyBool},
					Then: &Block{Body: []Stmt{&AssignStmt{Target: lr, Val: chainCast(&BinExpr{Op: "-", L: lw, R: kw, Ty: w}, nTy)}}},
					Else: &Block{Body: []Stmt{&AssignStmt{Target: lr, Val: chainLit(0, nTy)}}},
				})
				if startName == "" {
					startName = l.fresh("__iter_start")
					l.lowerStmt(&VarDeclStmt{Name: startName, Ty: mir2.TyU16, Init: chainCast(s.kRef, mir2.TyU16)})
				} else {
					sr := &VarRefExpr{Name: startName, Ty: mir2.TyU16}
					l.lowerStmt(&AssignStmt{Target: sr, Val: &BinExpr{Op: "+", L: sr, R: chainCast(s.kRef, mir2.TyU16), Ty: mir2.TyU16}})
				}
			case "take":
				// len = min(len, k); a literal k beyond the count's range never binds.
				if lit, ok := s.kRef.(*IntLitExpr); ok && lit.Val >= int64(1)<<uint(nTy.Width()) {
					continue
				}
				l.lowerStmt(&IfStmt{
					Cond: &BinExpr{Op: ">", L: lw, R: kw, Ty: mir2.TyBool},
					Then: &Block{Body: []Stmt{&AssignStmt{Target: lr, Val: chainCast(s.kRef, nTy)}}},
				})
			}
		}
		// take(0) runs nothing: with the stop test at the end of each element,
		// a zero count must empty the loop before it starts.
		for i := range c.stages {
			s := &c.stages[i]
			if s.kind != "take" || s.head {
				continue
			}
			if lit, isLit := s.kRef.(*IntLitExpr); isLit && lit.Val != 0 {
				continue
			}
			if lenName == "" {
				lenName = l.fresh("__iter_len")
				l.lowerStmt(&VarDeclStmt{Name: lenName, Ty: nTy, Init: c.n})
			}
			lr := &VarRefExpr{Name: lenName, Ty: nTy}
			zero := &AssignStmt{Target: lr, Val: chainLit(0, nTy)}
			if _, isLit := s.kRef.(*IntLitExpr); isLit {
				l.lowerStmt(zero)
			} else {
				l.lowerStmt(&IfStmt{Cond: &BinExpr{Op: "==", L: s.kRef, R: chainLit(0, s.kTy), Ty: mir2.TyBool}, Then: &Block{Body: []Stmt{zero}}})
			}
		}
		if lenName != "" {
			length = &VarRefExpr{Name: lenName, Ty: nTy}
		}
		if startName != "" {
			start = &VarRefExpr{Name: startName, Ty: mir2.TyU16}
		}
	}

	// Only head skip/take and no zip: the original builders produce exactly
	// the loop of the hand-hoisted spelling (ptr + k, adjusted count).
	headOnly := !c.isRange && c.zip == nil && l.keepsElemTy(c)
	for _, s := range c.stages {
		if (s.kind == "skip" || s.kind == "take") && !s.head {
			headOnly = false
		}
	}
	if headOnly && c.term != "count" && c.term != "sum" {
		old := iterChain{ptr: c.src, len: length, mutate: c.term == "mapInPlace", cbName: c.cb, elemTy: c.elemTy}
		if _, isLit := start.(*IntLitExpr); !isLit {
			old.ptr = l.chainOffsetPtr(c.src, start, c.elemTy)
		}
		for _, s := range c.stages {
			if s.kind == "map" || s.kind == "filter" {
				old.stages = append(old.stages, iterStage{kind: s.kind, fn: s.fn})
			}
		}
		if c.term == "fold" || c.term == "reduce" {
			old.isFold, old.foldInit, old.foldAccTy = true, c.init, c.accTy
			return l.lowerFusedFold(old)
		}
		l.lowerFusedForEach(old)
		return mir2.NoReg
	}

	// Counters for skip/take that are not folded into the bounds.
	for i := range c.stages {
		s := &c.stages[i]
		if (s.kind == "skip" || s.kind == "take") && !s.head {
			s.cnt = l.fresh("__iter_cnt")
			l.bind(s.cnt, l.bld.Const(0, s.kTy, classForExpr(s.kTy)), s.kTy)
		}
	}

	acc := ""
	if isValueIterTerminal(c.term) {
		acc = l.fresh("__fold_acc")
		var init Expr = chainLit(0, mir2.TyU16)
		if c.init != nil {
			init = c.init
		}
		l.bind(acc, l.lowerExprAs(init, c.accTy), c.accTy)
	}

	elemVar := "__iter_elem__"
	e := chainElem{v: elemVar, ty: c.elemTy}
	var prologue, epilogue []Stmt
	var zload Expr
	// take: stop as soon as the k-th accepted element has been processed, so
	// the next source element is never requested (ADR 0050). A memory loop
	// tests at the end of each element (take(0) emptied the count above); a
	// range loop has nothing to request and tests before each element.
	var stops []Stmt
	for _, s := range c.stages {
		if s.kind == "take" && !s.head {
			stops = append(stops, &IfStmt{
				Cond: &BinExpr{Op: "==", L: &VarRefExpr{Name: s.cnt, Ty: s.kTy}, R: s.kRef, Ty: mir2.TyBool},
				Then: &Block{Body: []Stmt{&BreakStmt{}}},
			})
		}
	}
	if c.isRange {
		prologue = append(prologue, stops...)
	}
	if c.zip != nil {
		ptrTy := l.m.PointerType()
		zp := l.fresh("__zip_ptr")
		zstride := int64(mir2.ByteWidth(c.zipTy))
		if zstride < 1 {
			zstride = 1
		}
		var zr mir2.Reg
		if _, isLit := start.(*IntLitExpr); isLit {
			zr = l.lowerExpr(c.zip)
		} else {
			zr = l.lowerExpr(l.chainOffsetPtr(c.zip, start, c.zipTy))
		}
		l.bind(zp, zr, ptrTy)
		e.paired, e.zv, e.zt = true, l.fresh("__zip_elem"), c.zipTy
		zref := &VarRefExpr{Name: zp, Ty: ptrTy}
		zload = &LoadExpr{Ptr: zref, Ty: c.zipTy}
		// The body has no continue, so every element reaches the bump at its end.
		epilogue = append(epilogue, &AssignStmt{Target: zref, Val: &BinExpr{Op: "+", L: zref, R: chainLit(zstride, mir2.TyU8), Ty: ptrTy}})
	}
	l.zipInStmt = false
	stages := l.chainBody(c, 0, e, acc)
	if zload != nil {
		// A zip element read once inside a callback expression is loaded at
		// that use, as the hand spelling `u16(rhs^)` does; otherwise once, up
		// front, into a variable.
		if l.zipInStmt || countVarRefs(stages, e.zv) != 1 {
			prologue = append(prologue, &VarDeclStmt{Name: e.zv, Ty: c.zipTy, Init: zload})
		} else {
			if l.lazyLoads == nil {
				l.lazyLoads = map[string]Expr{}
			}
			l.lazyLoads[e.zv] = zload
			defer delete(l.lazyLoads, e.zv)
		}
	}
	body := append(append(prologue, stages...), epilogue...)
	if !c.isRange {
		body = append(body, stops...)
	}

	if c.isRange {
		rc := iterChain{isRange: true, rangeLo: c.rangeLo, rangeHi: c.rangeHi, rangeRev: c.rangeRev, isFold: acc != ""}
		l.lowerRangeLoop(rc, elemVar, c.elemTy, body)
	} else {
		l.lowerForEach(&ForEachStmt{
			Var:    elemVar,
			ElemTy: c.elemTy,
			Ptr:    c.src,
			Start:  start,
			Len:    length,
			Body:   &Block{Body: body},
		})
	}
	if acc == "" {
		return mir2.NoReg
	}
	return l.env[acc]
}

// chainOffsetPtr is base + start*stride as a HIR expression already lowered
// into a temp pointer variable (so the base is evaluated once).
func (l *lowerer) chainOffsetPtr(base, start Expr, elemTy mir2.Ty) Expr {
	stride := int64(mir2.ByteWidth(elemTy))
	if stride < 1 {
		stride = 1
	}
	ptrTy := l.m.PointerType()
	p := l.lowerExpr(base)
	off := l.lowerExpr(start)
	if stride > 1 {
		factor := l.bld.Const(stride, mir2.TyU16, mir2.ClassPointer)
		off = l.bld.Mul(off, factor, mir2.TyU16, mir2.ClassIndex)
	}
	p = l.bld.PtrAdd(p, off, mir2.ClassPointer)
	name := l.fresh("__iter_base")
	l.bind(name, p, ptrTy)
	return &VarRefExpr{Name: name, Ty: ptrTy}
}

// countVarRefs counts references to name in stmts, nested blocks included.
func countVarRefs(stmts []Stmt, name string) int {
	n := 0
	walkChainBlocks(&Block{Body: stmts}, func(s Stmt) {
		visitStmtExprs(s, func(e Expr) {
			if vr, ok := e.(*VarRefExpr); ok && vr.Name == name {
				n++
			}
		})
	})
	return n
}

// lowerChainAssign lowers `target = chain` (AssignStmt) for a value chain
// whose result cannot simply be bound to a register local: any chain using
// the new methods, and an original fold chain assigned to a global, a cell
// (address-taken) local or a memory target. The chain is lowered into a
// temporary which the ordinary assignment path then stores. ok is false when
// the statement is left to the caller (not a value chain, or an original
// fold into a plain register local, whose lowering stays as it was).
func (l *lowerer) lowerChainAssign(target, val Expr) (terminated, ok bool) {
	call, isCall := val.(*CallExpr)
	if !isCall || !isValueIterTerminal(call.Fn) {
		return false, false
	}
	if !l.usesNewChainMethods(call) {
		if tgt, isVar := target.(*VarRefExpr); isVar && !l.localCells[tgt.Name] && !l.localArrays[tgt.Name] {
			if _, local := l.env[tgt.Name]; local {
				return false, false
			}
		}
	}
	reg, accTy, lowered := l.tryLowerFold(call)
	if !lowered {
		return false, false
	}
	tmp := l.fresh("__chain_val")
	l.bind(tmp, reg, accTy)
	return l.lowerStmt(&AssignStmt{Target: target, Val: &VarRefExpr{Name: tmp, Ty: accTy}}), true
}

// lowerChainStore lowers `p^ = chain` (StoreStmt) through a temporary.
func (l *lowerer) lowerChainStore(st *StoreStmt) (terminated, ok bool) {
	call, isCall := st.Val.(*CallExpr)
	if !isCall || !isValueIterTerminal(call.Fn) {
		return false, false
	}
	reg, accTy, lowered := l.tryLowerFold(call)
	if !lowered {
		return false, false
	}
	tmp := l.fresh("__chain_val")
	l.bind(tmp, reg, accTy)
	cp := *st
	cp.Val = &VarRefExpr{Name: tmp, Ty: accTy}
	return l.lowerStmt(&cp), true
}

// MethodMarked reports whether a text form of HIR must spell call's method
// mark: the mark changes the call's meaning only for the methods added with
// this file (skip, take, zip, count, sum) and for an original memory
// terminal written without its count (forEach/mapInPlace(cb),
// fold/reduce(init, cb)), which takes a sized array's length. Neither form
// compiled on origin/main, so text dumps of every program valid there are
// unchanged. A range terminal is never marked.
func MethodMarked(call *CallExpr) bool { return dumpMethodMark(call) }

func dumpMethodMark(call *CallExpr) bool {
	if call == nil || !call.Method {
		return false
	}
	if isNewIterChainMethod(call.Fn) {
		return true
	}
	short := len(call.Args) == 2 && (call.Fn == "forEach" || call.Fn == "mapInPlace") ||
		len(call.Args) == 3 && (call.Fn == "fold" || call.Fn == "reduce")
	if !short {
		return false
	}
	e := call.Args[0]
	for {
		inner, ok := e.(*CallExpr)
		if !ok || (inner.Fn != "map" && inner.Fn != "filter") || len(inner.Args) == 0 {
			break
		}
		e = inner.Args[0]
	}
	_, isRange := e.(*RangeSourceExpr)
	return !isRange
}

// retypeArraySource gives a variable naming an array its array type when the
// reference itself does not carry it: Nanz types a local array's reference
// as its element, Lanz every global's. The declaration (a local
// `let arr: [T; N]` in the function being lowered, or an array global)
// supplies the type, so such sources walk, size and check like Nanz globals.
func (l *lowerer) retypeArraySource(e Expr) Expr {
	vr, ok := e.(*VarRefExpr)
	if !ok {
		return e
	}
	if _, isArray := mir2.BaseOf(vr.Ty).(*mir2.ArrayTy); isArray || vr.Ty != nil && mir2.IsPointer(vr.Ty) {
		return e
	}
	if l.chainArrays == nil {
		l.chainArrays = map[string]mir2.Ty{}
		if l.m != nil {
			for _, g := range l.m.Globals {
				if at, isArray := mir2.BaseOf(g.Ty).(*mir2.ArrayTy); isArray {
					l.chainArrays[g.Name] = at
				}
			}
		}
		if l.hf != nil {
			collectLocalArrays(l.hf.Body, l.chainArrays)
		}
	}
	if ty, found := l.chainArrays[vr.Name]; found {
		return &VarRefExpr{Name: vr.Name, Ty: ty}
	}
	return e
}

// collectLocalArrays records `let name: [T; N]` declarations (flow-insensitive).
func collectLocalArrays(b *Block, into map[string]mir2.Ty) {
	walkChainBlocks(b, func(s Stmt) {
		if d, ok := s.(*VarDeclStmt); ok && d.ArrayLen > 0 && d.Ty != nil {
			into[d.Name] = &mir2.ArrayTy{Elem: d.Ty, Len: d.ArrayLen}
		}
	})
}
