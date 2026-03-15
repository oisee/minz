package lanz

import (
	"fmt"
	"strconv"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Compile parses Lanz S-expressions and returns a HIR module.
func Compile(src, name string) (*hir.Module, error) {
	nodes, err := ParseSExpr(src)
	if err != nil {
		return nil, err
	}
	c := &compiler{name: name}
	return c.compileModule(nodes)
}

type compiler struct {
	name string
}

func (c *compiler) compileModule(nodes []Node) (*hir.Module, error) {
	m := &hir.Module{Name: c.name}
	for _, n := range nodes {
		if !n.IsList() || len(n.List) == 0 {
			return nil, fmt.Errorf("line %d: expected top-level form, got atom %q", n.Line, n.Atom)
		}
		head := n.List[0]
		if !head.IsAtom() {
			return nil, fmt.Errorf("line %d: expected keyword at start of form", n.Line)
		}
		switch head.Atom {
		case "fun":
			f, err := c.compileFunc(n)
			if err != nil {
				return nil, err
			}
			m.Funcs = append(m.Funcs, f)
		case "extern":
			f, err := c.compileExtern(n)
			if err != nil {
				return nil, err
			}
			m.Funcs = append(m.Funcs, f)
		case "global":
			g, err := c.compileGlobal(n)
			if err != nil {
				return nil, err
			}
			m.Globals = append(m.Globals, g)
		case "struct":
			st, err := c.compileStruct(n)
			if err != nil {
				return nil, err
			}
			m.Structs = append(m.Structs, st)
		default:
			return nil, fmt.Errorf("line %d: unknown top-level form %q", head.Line, head.Atom)
		}
	}
	return m, nil
}

// (fun name ((p1 ty1) (p2 ty2)) retty body...)
func (c *compiler) compileFunc(n Node) (*hir.Func, error) {
	if len(n.List) < 4 {
		return nil, fmt.Errorf("line %d: fun: expected (fun name params retty body...)", n.Line)
	}
	name := n.List[1]
	if !name.IsAtom() {
		return nil, fmt.Errorf("line %d: fun: name must be atom", n.Line)
	}
	params, err := c.compileParams(n.List[2])
	if err != nil {
		return nil, fmt.Errorf("line %d: fun %s: %w", n.Line, name.Atom, err)
	}
	retTy := resolveType(n.List[3].Atom)

	var body []hir.Stmt
	for _, s := range n.List[4:] {
		st, err := c.compileStmt(s)
		if err != nil {
			return nil, fmt.Errorf("line %d: fun %s: %w", n.Line, name.Atom, err)
		}
		body = append(body, st)
	}

	f := &hir.Func{
		Name:   name.Atom,
		Params: params,
		RetTy:  retTy,
	}
	if len(body) > 0 {
		f.Body = &hir.Block{Body: body}
	}
	return f, nil
}

// (extern name ((p1 ty1) ...) retty)
func (c *compiler) compileExtern(n Node) (*hir.Func, error) {
	if len(n.List) < 4 {
		return nil, fmt.Errorf("line %d: extern: expected (extern name params retty)", n.Line)
	}
	name := n.List[1].Atom
	params, err := c.compileParams(n.List[2])
	if err != nil {
		return nil, err
	}
	retTy := resolveType(n.List[3].Atom)
	f := &hir.Func{
		Name:     name,
		Params:   params,
		RetTy:    retTy,
		IsExtern: true,
	}
	// Optional address: (extern name params retty addr)
	if len(n.List) >= 5 && n.List[4].IsAtom() {
		addr, err := strconv.ParseUint(n.List[4].Atom, 0, 16)
		if err == nil {
			f.ExternAddr = uint16(addr)
		}
	}
	return f, nil
}

// ((p1 ty1) (p2 ty2) ...)
func (c *compiler) compileParams(n Node) ([]hir.Param, error) {
	if n.IsAtom() {
		if n.Atom == "()" || n.Atom == "nil" || n.Atom == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("params must be a list, got %q", n.Atom)
	}
	if len(n.List) == 0 {
		return nil, nil
	}
	var params []hir.Param
	for _, p := range n.List {
		if !p.IsList() || len(p.List) < 2 {
			return nil, fmt.Errorf("param must be (name type), got %s", p)
		}
		params = append(params, hir.Param{
			Name: p.List[0].Atom,
			Ty:   resolveType(p.List[1].Atom),
		})
	}
	return params, nil
}

// (global name ty) or (global name ty init)
func (c *compiler) compileGlobal(n Node) (mir2.Global, error) {
	if len(n.List) < 3 {
		return mir2.Global{}, fmt.Errorf("line %d: global: expected (global name ty)", n.Line)
	}
	name := n.List[1].Atom
	ty := resolveType(n.List[2].Atom)
	g := mir2.Global{Name: name, Ty: ty}
	if len(n.List) >= 4 {
		// Init value — for now just integer constants
		if n.List[3].IsAtom() && IsInt(n.List[3].Atom) {
			v, _ := ParseInt(n.List[3].Atom)
			w := ty.Width() / 8
			if w == 0 {
				w = 1
			}
			g.Init = make([]byte, w)
			g.Init[0] = byte(v)
			if w >= 2 {
				g.Init[1] = byte(v >> 8)
			}
		}
	}
	return g, nil
}

// (struct name ((f1 ty1) (f2 ty2) ...))
func (c *compiler) compileStruct(n Node) (*mir2.StructTy, error) {
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: struct: expected (struct name fields)", n.Line)
	}
	name := n.List[1].Atom
	fields := n.List[2]
	if !fields.IsList() {
		return nil, fmt.Errorf("line %d: struct %s: fields must be a list", n.Line, name)
	}
	st := &mir2.StructTy{Name: name}
	for _, f := range fields.List {
		if !f.IsList() || len(f.List) < 2 {
			return nil, fmt.Errorf("line %d: struct %s: field must be (name type)", n.Line, name)
		}
		ty := resolveType(f.List[1].Atom)
		st.Fields = append(st.Fields, mir2.StructField{
			Name: f.List[0].Atom,
			Ty:   ty,
		})
	}
	return st, nil
}

// ── Statements ───────────────────────────────────────────────────────────────

func (c *compiler) compileStmt(n Node) (hir.Stmt, error) {
	if n.IsAtom() {
		// Bare expression (e.g. function call as statement)
		expr, err := c.compileExpr(n)
		if err != nil {
			return nil, err
		}
		return &hir.ExprStmt{Expr: expr}, nil
	}
	if len(n.List) == 0 {
		return nil, fmt.Errorf("line %d: empty form", n.Line)
	}
	head := n.List[0]
	if !head.IsAtom() {
		// Nested expression
		expr, err := c.compileExpr(n)
		if err != nil {
			return nil, err
		}
		return &hir.ExprStmt{Expr: expr}, nil
	}

	switch head.Atom {
	case "var":
		return c.compileVar(n)
	case "set":
		return c.compileSet(n)
	case "return":
		return c.compileReturn(n)
	case "if":
		return c.compileIf(n)
	case "while":
		return c.compileWhile(n)
	case "for":
		return c.compileFor(n)
	case "store":
		return c.compileStore(n)
	case "block":
		return c.compileBlock(n)
	case "break":
		return &hir.BreakStmt{}, nil
	case "continue":
		return &hir.ContinueStmt{}, nil
	case "switch":
		return c.compileSwitch(n)
	default:
		// Try as expression statement
		expr, err := c.compileExpr(n)
		if err != nil {
			return nil, err
		}
		return &hir.ExprStmt{Expr: expr}, nil
	}
}

// (var name ty) or (var name ty init)
func (c *compiler) compileVar(n Node) (hir.Stmt, error) {
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: var: expected (var name ty [init])", n.Line)
	}
	name := n.List[1].Atom
	ty := resolveType(n.List[2].Atom)
	st := &hir.VarDeclStmt{Name: name, Ty: ty}
	if len(n.List) >= 4 {
		init, err := c.compileExpr(n.List[3])
		if err != nil {
			return nil, err
		}
		st.Init = init
	}
	return st, nil
}

// (set target val)
func (c *compiler) compileSet(n Node) (hir.Stmt, error) {
	if len(n.List) != 3 {
		return nil, fmt.Errorf("line %d: set: expected (set target val)", n.Line)
	}
	target, err := c.compileExpr(n.List[1])
	if err != nil {
		return nil, err
	}
	val, err := c.compileExpr(n.List[2])
	if err != nil {
		return nil, err
	}
	return &hir.AssignStmt{Target: target, Val: val}, nil
}

// (return) or (return expr)
func (c *compiler) compileReturn(n Node) (hir.Stmt, error) {
	if len(n.List) == 1 {
		return &hir.ReturnStmt{}, nil
	}
	val, err := c.compileExpr(n.List[1])
	if err != nil {
		return nil, err
	}
	return &hir.ReturnStmt{Val: val}, nil
}

// (if cond then) or (if cond then else)
func (c *compiler) compileIf(n Node) (hir.Stmt, error) {
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: if: expected (if cond then [else])", n.Line)
	}
	cond, err := c.compileExpr(n.List[1])
	if err != nil {
		return nil, err
	}
	thenStmt, err := c.compileStmt(n.List[2])
	if err != nil {
		return nil, err
	}
	thenBlock := stmtToBlock(thenStmt)
	var elseBlock *hir.Block
	if len(n.List) >= 4 {
		elseStmt, err := c.compileStmt(n.List[3])
		if err != nil {
			return nil, err
		}
		elseBlock = stmtToBlock(elseStmt)
	}
	return &hir.IfStmt{Cond: cond, Then: thenBlock, Else: elseBlock}, nil
}

// (while cond body...)
func (c *compiler) compileWhile(n Node) (hir.Stmt, error) {
	if len(n.List) < 3 {
		return nil, fmt.Errorf("line %d: while: expected (while cond body...)", n.Line)
	}
	cond, err := c.compileExpr(n.List[1])
	if err != nil {
		return nil, err
	}
	var body []hir.Stmt
	for _, s := range n.List[2:] {
		st, err := c.compileStmt(s)
		if err != nil {
			return nil, err
		}
		body = append(body, st)
	}
	return &hir.WhileStmt{Cond: cond, Body: &hir.Block{Body: body}}, nil
}

// (for var lo hi body...)
func (c *compiler) compileFor(n Node) (hir.Stmt, error) {
	if len(n.List) < 4 {
		return nil, fmt.Errorf("line %d: for: expected (for var lo hi body...)", n.Line)
	}
	varName := n.List[1].Atom
	lo, err := c.compileExpr(n.List[2])
	if err != nil {
		return nil, err
	}
	hi, err := c.compileExpr(n.List[3])
	if err != nil {
		return nil, err
	}
	var body []hir.Stmt
	for _, s := range n.List[4:] {
		st, err := c.compileStmt(s)
		if err != nil {
			return nil, err
		}
		body = append(body, st)
	}
	return &hir.ForRangeStmt{Var: varName, Start: lo, End: hi, Body: &hir.Block{Body: body}}, nil
}

// (store ptr val)
func (c *compiler) compileStore(n Node) (hir.Stmt, error) {
	if len(n.List) != 3 {
		return nil, fmt.Errorf("line %d: store: expected (store ptr val)", n.Line)
	}
	ptr, err := c.compileExpr(n.List[1])
	if err != nil {
		return nil, err
	}
	val, err := c.compileExpr(n.List[2])
	if err != nil {
		return nil, err
	}
	return &hir.StoreStmt{Ptr: ptr, Val: val}, nil
}

// (block stmts...)
func (c *compiler) compileBlock(n Node) (hir.Stmt, error) {
	var body []hir.Stmt
	for _, s := range n.List[1:] {
		st, err := c.compileStmt(s)
		if err != nil {
			return nil, err
		}
		body = append(body, st)
	}
	return &hir.Block{Body: body}, nil
}

// (switch val (case v body...) ... (default body...))
func (c *compiler) compileSwitch(n Node) (hir.Stmt, error) {
	if len(n.List) < 2 {
		return nil, fmt.Errorf("line %d: switch: expected (switch val cases...)", n.Line)
	}
	val, err := c.compileExpr(n.List[1])
	if err != nil {
		return nil, err
	}
	sw := &hir.SwitchStmt{Val: val}
	for _, arm := range n.List[2:] {
		if !arm.IsList() || len(arm.List) < 2 {
			continue
		}
		head := arm.List[0].Atom
		if head == "case" {
			v, _ := ParseInt(arm.List[1].Atom)
			var body []hir.Stmt
			for _, s := range arm.List[2:] {
				st, err := c.compileStmt(s)
				if err != nil {
					return nil, err
				}
				body = append(body, st)
			}
			sw.Cases = append(sw.Cases, &hir.SwitchCase{Val: v, Body: &hir.Block{Body: body}})
		} else if head == "default" {
			var body []hir.Stmt
			for _, s := range arm.List[1:] {
				st, err := c.compileStmt(s)
				if err != nil {
					return nil, err
				}
				body = append(body, st)
			}
			sw.Default = &hir.Block{Body: body}
		}
	}
	return sw, nil
}

// ── Expressions ──────────────────────────────────────────────────────────────

func (c *compiler) compileExpr(n Node) (hir.Expr, error) {
	if n.IsAtom() {
		return c.compileAtomExpr(n)
	}
	if len(n.List) == 0 {
		return nil, fmt.Errorf("line %d: empty expression", n.Line)
	}
	head := n.List[0]
	if !head.IsAtom() {
		return nil, fmt.Errorf("line %d: expected operator/function at start of expression", n.Line)
	}

	switch head.Atom {
	case "+", "-", "*", "/", "%", "&", "|", "^", "<<", ">>",
		"==", "!=", "<", "<=", ">", ">=":
		return c.compileBinExpr(n)
	case "neg":
		if len(n.List) != 2 {
			return nil, fmt.Errorf("line %d: neg: expected (neg x)", n.Line)
		}
		x, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "-", X: x, Ty: x.ExprTy()}, nil
	case "not":
		if len(n.List) != 2 {
			return nil, fmt.Errorf("line %d: not: expected (not x)", n.Line)
		}
		x, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "!", X: x, Ty: mir2.TyBool}, nil
	case "bitnot":
		if len(n.List) != 2 {
			return nil, fmt.Errorf("line %d: bitnot: expected (bitnot x)", n.Line)
		}
		x, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		return &hir.UnaryExpr{Op: "~", X: x, Ty: x.ExprTy()}, nil
	case "call":
		return c.compileCall(n)
	case "load":
		if len(n.List) < 2 {
			return nil, fmt.Errorf("line %d: load: expected (load ptr [ty])", n.Line)
		}
		ptr, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		ty := mir2.Ty(mir2.TyU8)
		if len(n.List) >= 3 && n.List[2].IsAtom() {
			ty = resolveType(n.List[2].Atom)
		}
		return &hir.LoadExpr{Ptr: ptr, Ty: ty}, nil
	case "addr":
		if len(n.List) != 2 {
			return nil, fmt.Errorf("line %d: addr: expected (addr sym)", n.Line)
		}
		return &hir.AddrOfExpr{Sym: n.List[1].Atom}, nil
	case "cast":
		if len(n.List) != 3 {
			return nil, fmt.Errorf("line %d: cast: expected (cast expr ty)", n.Line)
		}
		x, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		return &hir.CastExpr{X: x, Ty: resolveType(n.List[2].Atom)}, nil
	case "index":
		if len(n.List) < 3 {
			return nil, fmt.Errorf("line %d: index: expected (index base idx [elemty])", n.Line)
		}
		base, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		idx, err := c.compileExpr(n.List[2])
		if err != nil {
			return nil, err
		}
		elemTy := mir2.Ty(mir2.TyU8)
		if len(n.List) >= 4 && n.List[3].IsAtom() {
			elemTy = resolveType(n.List[3].Atom)
		}
		return &hir.IndexExpr{Base: base, Idx: idx, ElemTy: elemTy}, nil
	case "field":
		if len(n.List) != 4 {
			return nil, fmt.Errorf("line %d: field: expected (field base name offset)", n.Line)
		}
		base, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		fieldName := n.List[2].Atom
		off, _ := strconv.Atoi(n.List[3].Atom)
		return &hir.FieldExpr{X: base, Field: fieldName, Offset: off, Ty: mir2.TyU8}, nil
	case "if":
		// if-as-expression: (if cond then else)
		if len(n.List) != 4 {
			return nil, fmt.Errorf("line %d: if-expr: expected (if cond then else)", n.Line)
		}
		cond, err := c.compileExpr(n.List[1])
		if err != nil {
			return nil, err
		}
		then, err := c.compileExpr(n.List[2])
		if err != nil {
			return nil, err
		}
		els, err := c.compileExpr(n.List[3])
		if err != nil {
			return nil, err
		}
		return &hir.CondExpr{Cond: cond, Then: then, Else: els, Ty: then.ExprTy()}, nil
	default:
		// Treat as function call: (fname args...)
		return c.compileCall(n)
	}
}

func (c *compiler) compileAtomExpr(n Node) (hir.Expr, error) {
	s := n.Atom
	if s == "true" {
		return &hir.BoolLitExpr{Val: true}, nil
	}
	if s == "false" {
		return &hir.BoolLitExpr{Val: false}, nil
	}
	if IsInt(s) {
		v, err := ParseInt(s)
		if err != nil {
			return nil, fmt.Errorf("line %d: bad int %q: %w", n.Line, s, err)
		}
		ty := mir2.Ty(mir2.TyU8)
		if v > 255 || v < -128 {
			ty = mir2.TyU16
		}
		return &hir.IntLitExpr{Val: v, Ty: ty}, nil
	}
	// Variable reference
	return &hir.VarRefExpr{Name: s, Ty: mir2.TyU8}, nil
}

func (c *compiler) compileBinExpr(n Node) (hir.Expr, error) {
	if len(n.List) != 3 {
		return nil, fmt.Errorf("line %d: %s: expected binary op with 2 args", n.Line, n.List[0].Atom)
	}
	l, err := c.compileExpr(n.List[1])
	if err != nil {
		return nil, err
	}
	r, err := c.compileExpr(n.List[2])
	if err != nil {
		return nil, err
	}
	op := n.List[0].Atom
	ty := l.ExprTy()
	if op == "==" || op == "!=" || op == "<" || op == "<=" || op == ">" || op == ">=" {
		ty = mir2.TyBool
	}
	return &hir.BinExpr{Op: op, L: l, R: r, Ty: ty}, nil
}

// (call fn args...) or (fn args...) — bare call
func (c *compiler) compileCall(n Node) (hir.Expr, error) {
	start := 0
	if n.List[0].Atom == "call" {
		start = 1
	}
	if start >= len(n.List) {
		return nil, fmt.Errorf("line %d: call: missing function name", n.Line)
	}
	fn := n.List[start].Atom
	var args []hir.Expr
	for _, a := range n.List[start+1:] {
		expr, err := c.compileExpr(a)
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
	}
	return &hir.CallExpr{Fn: fn, Args: args, Ty: mir2.TyVoid}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func resolveType(s string) mir2.Ty {
	switch s {
	case "u8":
		return mir2.TyU8
	case "u16":
		return mir2.TyU16
	case "i8":
		return mir2.TyI8
	case "i16":
		return mir2.TyI16
	case "bool":
		return mir2.TyBool
	case "void":
		return mir2.TyVoid
	case "ptr", "^":
		return mir2.TyPtr
	default:
		return mir2.TyU8
	}
}

func stmtToBlock(s hir.Stmt) *hir.Block {
	if b, ok := s.(*hir.Block); ok {
		return b
	}
	return &hir.Block{Body: []hir.Stmt{s}}
}
