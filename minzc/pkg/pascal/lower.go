package pascal

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// CompileOpts configures the Pascal compilation.
type CompileOpts struct {
	StdlibDir string // path to stdlib/ root; empty = no auto-import
}

// Compile is the top-level entry point: Pascal source → HIR module.
func Compile(src, name string, opts ...CompileOpts) (*hir.Module, error) {
	prog, err := ParseProgram(src)
	if err != nil {
		return nil, fmt.Errorf("pascal parse: %w", err)
	}
	var o CompileOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	hm, err := LowerProgram(prog, name)
	if err != nil {
		return nil, fmt.Errorf("pascal lower: %w", err)
	}
	// Emit built-in runtime functions (ConOut, WriteStr, etc.) for any
	// referenced but undefined names.  Uses inline asm with correct CP/M
	// register conventions — no external Lizp import needed.
	_ = o // StdlibDir no longer used; runtime is generated directly.
	emitRuntimeFuncs(hm)
	return hm, nil
}

// LowerProgram converts a parsed Pascal AST to a HIR module.
func LowerProgram(prog *Program, name string) (*hir.Module, error) {
	l := &lowerer{
		prog:     prog,
		consts:   make(map[string]int64),
		types:    make(map[string]PasType),
		fnRetTy:  make(map[string]PasType),
		varTypes: make(map[string]PasType),
	}
	if name == "" {
		name = prog.Name
	}
	l.hm = &hir.Module{Name: name}
	return l.lower()
}

type lowerer struct {
	prog     *Program
	hm       *hir.Module
	consts   map[string]int64   // resolved compile-time constants
	types    map[string]PasType // named type aliases
	fnRetTy  map[string]PasType // function return types
	varTypes map[string]PasType // all declared variables (global + local scope)
}

func (l *lowerer) lower() (*hir.Module, error) {
	// Pass 1: collect constants, types, function signatures.
	for _, d := range l.prog.Decls {
		switch d := d.(type) {
		case *ConstDecl:
			if lit, ok := d.Val.(*IntLit); ok {
				l.consts[d.Name] = lit.Val
			}
			if ch, ok := d.Val.(*CharLit); ok {
				l.consts[d.Name] = int64(ch.Val)
			}
		case *TypeDecl:
			l.types[d.Name] = d.Ty
		case *ProcDecl:
			if d.RetTy != nil {
				l.fnRetTy[d.Name] = d.RetTy
			}
		}
	}

	// Pass 2: emit declarations.
	for _, d := range l.prog.Decls {
		if err := l.lowerDecl(d); err != nil {
			return nil, err
		}
	}

	// Main body → function main (always emitted, even if empty).
	mainFn, err := l.lowerMainBody(l.prog.Body)
	if err != nil {
		return nil, err
	}
	l.hm.Funcs = append(l.hm.Funcs, mainFn)

	return l.hm, nil
}

func (l *lowerer) lowerDecl(d Decl) error {
	switch d := d.(type) {
	case *ConstDecl:
		// Untyped constants are inlined at use site; nothing to emit.
		return nil

	case *TypedConstDecl:
		// Typed const → mutable global with init value.
		ty := l.resolveType(d.Ty)
		mty := PasTypeToMIR2(ty)
		g := mir2.Global{Name: d.Name, Ty: mty}
		l.hm.Globals = append(l.hm.Globals, g)
		l.varTypes[d.Name] = ty
		return nil

	case *TypeDecl:
		// Record types → HIR structs.
		if rt, ok := l.resolveType(d.Ty).(*RecordType); ok {
			st := l.lowerRecordType(d.Name, rt)
			l.hm.Structs = append(l.hm.Structs, st)
		}
		return nil

	case *VarDecl:
		ty := l.resolveType(d.Ty)
		mty := PasTypeToMIR2(ty)
		for _, name := range d.Names {
			l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: name, Ty: mty})
			l.varTypes[name] = ty
		}
		return nil

	case *ProcDecl:
		f, err := l.lowerProc(d)
		if err != nil {
			return fmt.Errorf("procedure %s: %w", d.Name, err)
		}
		l.hm.Funcs = append(l.hm.Funcs, f)
		return nil

	default:
		return nil
	}
}

func (l *lowerer) lowerRecordType(name string, rt *RecordType) *mir2.StructTy {
	st := &mir2.StructTy{Name: name}
	for _, f := range rt.Fields {
		fty := PasTypeToMIR2(l.resolveType(f.Ty))
		for _, fn := range f.Names {
			st.Fields = append(st.Fields, mir2.StructField{Name: fn, Ty: fty})
		}
	}
	return st
}

func (l *lowerer) lowerMainBody(stmts []Stmt) (*hir.Func, error) {
	pl := &procLow{low: l, localTypes: make(map[string]PasType)}
	var body []hir.Stmt
	for _, s := range stmts {
		hs, err := pl.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		body = append(body, hs...)
	}
	body = append(body, hir.RetVoid())
	return &hir.Func{
		Name:  "main",
		RetTy: mir2.TyVoid,
		Body:  hir.Blk(body...),
	}, nil
}

// ── Procedure/Function lowerer ───────────────────────────────────────────────

type procLow struct {
	low        *lowerer
	localTypes map[string]PasType
	funcName   string // for function result assignment
	funcRetTy  PasType
}

func (l *lowerer) lowerProc(pd *ProcDecl) (*hir.Func, error) {
	pl := &procLow{
		low:        l,
		localTypes: make(map[string]PasType),
		funcName:   pd.Name,
		funcRetTy:  pd.RetTy,
	}

	// Build params.
	var params []hir.Param
	for _, pg := range pd.Params {
		ty := l.resolveType(pg.Ty)
		mty := PasTypeToMIR2(ty)
		if pg.IsVar {
			mty = mir2.TyPtr // var params passed as pointer
		}
		for _, name := range pg.Names {
			params = append(params, hir.Param{Name: name, Ty: mty})
			pl.localTypes[name] = ty
		}
	}

	var retTy mir2.Ty = mir2.TyVoid
	if pd.RetTy != nil {
		retTy = PasTypeToMIR2(l.resolveType(pd.RetTy))
	}

	// Local declarations.
	var bodyStmts []hir.Stmt
	for _, ld := range pd.Locals {
		if vd, ok := ld.(*VarDecl); ok {
			ty := l.resolveType(vd.Ty)
			mty := PasTypeToMIR2(ty)
			for _, name := range vd.Names {
				bodyStmts = append(bodyStmts, &hir.VarDeclStmt{Name: name, Ty: mty})
				pl.localTypes[name] = ty
			}
		}
	}

	// Function result variable (Pascal convention: FuncName := result)
	if pd.RetTy != nil {
		bodyStmts = append(bodyStmts, &hir.VarDeclStmt{
			Name: pd.Name,
			Ty:   retTy,
		})
		pl.localTypes[pd.Name] = pd.RetTy
	}

	// Nested procedures — emit as separate top-level functions (flat namespace).
	for _, sub := range pd.SubProc {
		sf, err := l.lowerProc(sub)
		if err != nil {
			return nil, err
		}
		l.hm.Funcs = append(l.hm.Funcs, sf)
	}

	// Body statements.
	for _, s := range pd.Body {
		hs, err := pl.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		bodyStmts = append(bodyStmts, hs...)
	}

	// Implicit return of function result.
	if pd.RetTy != nil {
		bodyStmts = append(bodyStmts, hir.Ret(hir.Var(pd.Name, retTy)))
	} else {
		bodyStmts = append(bodyStmts, hir.RetVoid())
	}

	return &hir.Func{
		Name:   pd.Name,
		Params: params,
		RetTy:  retTy,
		Body:   hir.Blk(bodyStmts...),
	}, nil
}

// typeOf looks up a variable's Pascal type.
func (pl *procLow) typeOf(name string) PasType {
	if ty, ok := pl.localTypes[name]; ok {
		return ty
	}
	if ty, ok := pl.low.varTypes[name]; ok {
		return ty
	}
	return &BasicType{Name: "INTEGER"} // fallback
}

func (pl *procLow) mir2TypeOf(name string) mir2.Ty {
	return PasTypeToMIR2(pl.low.resolveType(pl.typeOf(name)))
}

// ── Statement lowering ───────────────────────────────────────────────────────

func (pl *procLow) lowerStmt(s Stmt) ([]hir.Stmt, error) {
	switch s := s.(type) {
	case *AssignStmt:
		return pl.lowerAssign(s)
	case *CallStmt:
		return pl.lowerCallStmt(s)
	case *IfStmt:
		return pl.lowerIf(s)
	case *WhileStmt:
		return pl.lowerWhile(s)
	case *RepeatStmt:
		return pl.lowerRepeat(s)
	case *ForStmt:
		return pl.lowerFor(s)
	case *CaseStmt:
		return pl.lowerCase(s)
	case *Block:
		return pl.lowerBlock(s)
	case *AssertStmt:
		return pl.lowerAssert(s)
	case *ExitStmt:
		if pl.funcRetTy != nil {
			retTy := PasTypeToMIR2(pl.low.resolveType(pl.funcRetTy))
			return []hir.Stmt{hir.Ret(hir.Var(pl.funcName, retTy))}, nil
		}
		return []hir.Stmt{hir.RetVoid()}, nil
	default:
		return nil, fmt.Errorf("unhandled Pascal statement type %T", s)
	}
}

func (pl *procLow) lowerAssign(s *AssignStmt) ([]hir.Stmt, error) {
	val, err := pl.lowerExpr(s.Val)
	if err != nil {
		return nil, err
	}

	// Simple variable assignment
	if vr, ok := s.Target.(*VarRef); ok {
		ty := pl.mir2TypeOf(vr.Name)
		return []hir.Stmt{hir.Assign(hir.Var(vr.Name, ty), val)}, nil
	}

	// Array index assignment: a[i] := val
	if ie, ok := s.Target.(*IndexExpr); ok {
		if vr, ok := ie.Base.(*VarRef); ok {
			idx, err := pl.lowerExpr(ie.Idx)
			if err != nil {
				return nil, err
			}
			elemTy := pl.elemTypeOf(vr.Name)
			base := &hir.AddrOfExpr{Sym: vr.Name}
			ptr := &hir.IndexExpr{Base: base, Idx: idx, ElemTy: elemTy}
			return []hir.Stmt{&hir.StoreStmt{Ptr: ptr, Val: val}}, nil
		}
	}

	// Field assignment: r.f := val
	if fe, ok := s.Target.(*FieldExpr); ok {
		base, err := pl.lowerExpr(fe.Base)
		if err != nil {
			return nil, err
		}
		// For now, emit as field set — we need the struct type info
		fieldExpr := &hir.FieldExpr{X: base, Field: fe.Field, Ty: val.ExprTy()}
		return []hir.Stmt{hir.Assign(fieldExpr, val)}, nil
	}

	return nil, fmt.Errorf("unsupported assignment target %T", s.Target)
}

func (pl *procLow) elemTypeOf(name string) mir2.Ty {
	ty := pl.low.resolveType(pl.typeOf(name))
	if at, ok := ty.(*ArrayType); ok {
		return PasTypeToMIR2(pl.low.resolveType(at.Elem))
	}
	return PasTypeToMIR2(ty)
}

func (pl *procLow) lowerCallStmt(s *CallStmt) ([]hir.Stmt, error) {
	// Built-in procedures
	switch s.Name {
	case "WRITELN":
		return pl.lowerWriteLn(s.Args)
	case "WRITE":
		return pl.lowerWrite(s.Args)
	case "INC":
		return pl.lowerIncDec(s.Args, "+")
	case "DEC":
		return pl.lowerIncDec(s.Args, "-")
	case "HALT":
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("PascalHalt", mir2.TyVoid)}}, nil
	}

	var retTy mir2.Ty = mir2.TyVoid
	if rt, ok := pl.low.fnRetTy[s.Name]; ok {
		retTy = PasTypeToMIR2(pl.low.resolveType(rt))
	}
	args, err := pl.lowerExprs(s.Args)
	if err != nil {
		return nil, err
	}
	return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call(s.Name, retTy, args...)}}, nil
}

func (pl *procLow) lowerWriteLn(args []Expr) ([]hir.Stmt, error) {
	stmts, err := pl.lowerWrite(args)
	if err != nil {
		return nil, err
	}
	stmts = append(stmts, &hir.ExprStmt{Expr: hir.Call("WriteCrLf", mir2.TyVoid)})
	return stmts, nil
}

func (pl *procLow) lowerWrite(args []Expr) ([]hir.Stmt, error) {
	var stmts []hir.Stmt
	for _, a := range args {
		switch a := a.(type) {
		case *StrLit:
			// Intern string as $-terminated, call WriteStr(addr)
			sym := pl.low.internString(a.Val)
			stmts = append(stmts, &hir.ExprStmt{
				Expr: hir.Call("WriteStr", mir2.TyVoid, &hir.AddrOfExpr{Sym: sym}),
			})
		case *CharLit:
			stmts = append(stmts, &hir.ExprStmt{
				Expr: hir.Call("ConOut", mir2.TyVoid, hir.U8(int64(a.Val))),
			})
		default:
			he, err := pl.lowerExpr(a)
			if err != nil {
				return nil, err
			}
			// Choose WriteU8 or WriteI16 based on type
			ty := he.ExprTy()
			if ty == mir2.TyU8 || ty == mir2.TyBool {
				stmts = append(stmts, &hir.ExprStmt{
					Expr: hir.Call("WriteU8", mir2.TyVoid, he),
				})
			} else {
				stmts = append(stmts, &hir.ExprStmt{
					Expr: hir.Call("WriteI16", mir2.TyVoid, he),
				})
			}
		}
	}
	return stmts, nil
}

func (pl *procLow) lowerIncDec(args []Expr, op string) ([]hir.Stmt, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("Inc/Dec requires at least 1 argument")
	}
	vr, ok := args[0].(*VarRef)
	if !ok {
		return nil, fmt.Errorf("Inc/Dec first arg must be variable")
	}
	ty := pl.mir2TypeOf(vr.Name)
	var step hir.Expr = hir.U8(1)
	if len(args) >= 2 {
		var err error
		step, err = pl.lowerExpr(args[1])
		if err != nil {
			return nil, err
		}
	}
	inc := &hir.BinExpr{Op: op, L: hir.Var(vr.Name, ty), R: step, Ty: ty}
	return []hir.Stmt{hir.Assign(hir.Var(vr.Name, ty), inc)}, nil
}

func (pl *procLow) lowerIf(s *IfStmt) ([]hir.Stmt, error) {
	cond, err := pl.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	then, err := pl.lowerAsBlock(s.Then)
	if err != nil {
		return nil, err
	}
	var els *hir.Block
	if s.Else != nil {
		els, err = pl.lowerAsBlock(s.Else)
		if err != nil {
			return nil, err
		}
	}
	return []hir.Stmt{hir.If(cond, then, els)}, nil
}

func (pl *procLow) lowerWhile(s *WhileStmt) ([]hir.Stmt, error) {
	cond, err := pl.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	body, err := pl.lowerAsBlock(s.Body)
	if err != nil {
		return nil, err
	}
	return []hir.Stmt{hir.While(cond, body)}, nil
}

func (pl *procLow) lowerRepeat(s *RepeatStmt) ([]hir.Stmt, error) {
	// repeat..until → while(true) { body; if(cond) break; }
	bodyStmts, err := pl.lowerStmtList(s.Body)
	if err != nil {
		return nil, err
	}
	cond, err := pl.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	// if (cond) break
	bodyStmts = append(bodyStmts, hir.If(cond, hir.Blk(&hir.BreakStmt{}), nil))
	loop := hir.While(&hir.BoolLitExpr{Val: true}, hir.Blk(bodyStmts...))
	return []hir.Stmt{loop}, nil
}

func (pl *procLow) lowerFor(s *ForStmt) ([]hir.Stmt, error) {
	ty := pl.mir2TypeOf(s.Var)
	start, err := pl.lowerExpr(s.Start)
	if err != nil {
		return nil, err
	}
	end, err := pl.lowerExpr(s.End)
	if err != nil {
		return nil, err
	}

	// init: var := start
	init := hir.Assign(hir.Var(s.Var, ty), start)

	var cond hir.Expr
	var inc hir.Stmt

	if s.Downto {
		// for I := N downto 0 → while I >= end { body; I := I - 1 }
		cond = &hir.BinExpr{Op: ">=", L: hir.Var(s.Var, ty), R: end, Ty: mir2.TyBool}
		inc = hir.Assign(hir.Var(s.Var, ty), &hir.BinExpr{
			Op: "-", L: hir.Var(s.Var, ty), R: hir.U16(1), Ty: ty,
		})
	} else {
		// for I := 0 to N → while I < (end + 1) { body; I := I + 1 }
		endPlus1 := &hir.BinExpr{Op: "+", L: end, R: hir.U16(1), Ty: ty}
		cond = &hir.BinExpr{Op: "<", L: hir.Var(s.Var, ty), R: endPlus1, Ty: mir2.TyBool}
		inc = hir.Assign(hir.Var(s.Var, ty), &hir.BinExpr{
			Op: "+", L: hir.Var(s.Var, ty), R: hir.U16(1), Ty: ty,
		})
	}

	body, err := pl.lowerAsBlock(s.Body)
	if err != nil {
		return nil, err
	}
	bodyStmts := append(body.Body, inc)

	return []hir.Stmt{init, hir.While(cond, hir.Blk(bodyStmts...))}, nil
}

func (pl *procLow) lowerCase(s *CaseStmt) ([]hir.Stmt, error) {
	sel, err := pl.lowerExpr(s.Sel)
	if err != nil {
		return nil, err
	}
	var cases []*hir.SwitchCase
	for _, arm := range s.Arms {
		armBody, err := pl.lowerAsBlock(arm.Body)
		if err != nil {
			return nil, err
		}
		for _, v := range arm.Vals {
			cases = append(cases, hir.Case(v, armBody))
		}
	}
	var def *hir.Block
	if s.Default != nil {
		defStmts, err := pl.lowerStmtList(s.Default)
		if err != nil {
			return nil, err
		}
		def = hir.Blk(defStmts...)
	}
	return []hir.Stmt{hir.Switch(sel, cases, def)}, nil
}

func (pl *procLow) lowerAssert(s *AssertStmt) ([]hir.Stmt, error) {
	// Convert assert to HIR module-level Assert.
	var args []int64
	for _, a := range s.Args {
		switch a := a.(type) {
		case *IntLit:
			args = append(args, a.Val)
		case *CharLit:
			args = append(args, int64(a.Val))
		case *BoolLit:
			if a.Val {
				args = append(args, 1)
			} else {
				args = append(args, 0)
			}
		case *VarRef:
			if v, ok := pl.low.consts[a.Name]; ok {
				args = append(args, v)
			} else {
				return nil, fmt.Errorf("assert args must be literal values, got variable %q", a.Name)
			}
		default:
			return nil, fmt.Errorf("assert args must be literal values")
		}
	}

	var expected int64
	switch e := s.Expected.(type) {
	case *IntLit:
		expected = e.Val
	case *CharLit:
		expected = int64(e.Val)
	case *BoolLit:
		if e.Val {
			expected = 1
		}
	default:
		return nil, fmt.Errorf("assert expected must be a literal value")
	}

	pl.low.hm.Asserts = append(pl.low.hm.Asserts, hir.Assert{
		FuncName: s.FuncName,
		Args:     args,
		Expected: expected,
		Source:   fmt.Sprintf("assert %s(...) %s %d", s.FuncName, s.Op, expected),
	})
	return nil, nil // asserts don't emit runtime code
}

func (pl *procLow) lowerBlock(s *Block) ([]hir.Stmt, error) {
	return pl.lowerStmtList(s.Stmts)
}

func (pl *procLow) lowerStmtList(stmts []Stmt) ([]hir.Stmt, error) {
	var out []hir.Stmt
	for _, s := range stmts {
		hs, err := pl.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		out = append(out, hs...)
	}
	return out, nil
}

func (pl *procLow) lowerAsBlock(s Stmt) (*hir.Block, error) {
	if b, ok := s.(*Block); ok {
		stmts, err := pl.lowerStmtList(b.Stmts)
		if err != nil {
			return nil, err
		}
		return hir.Blk(stmts...), nil
	}
	stmts, err := pl.lowerStmt(s)
	if err != nil {
		return nil, err
	}
	return hir.Blk(stmts...), nil
}

// ── Expression lowering ──────────────────────────────────────────────────────

func (pl *procLow) lowerExpr(e Expr) (hir.Expr, error) {
	switch e := e.(type) {
	case *IntLit:
		if e.Val >= 0 && e.Val <= 255 {
			return hir.U8(e.Val), nil
		}
		return hir.U16(e.Val), nil

	case *CharLit:
		return hir.U8(int64(e.Val)), nil

	case *StrLit:
		// String literal → not directly supported yet; return 0
		return hir.U8(0), nil

	case *BoolLit:
		if e.Val {
			return hir.U8(1), nil
		}
		return hir.U8(0), nil

	case *VarRef:
		// Check if it's a constant.
		if v, ok := pl.low.consts[e.Name]; ok {
			if v >= 0 && v <= 255 {
				return hir.U8(v), nil
			}
			return hir.U16(v), nil
		}
		ty := pl.mir2TypeOf(e.Name)
		return hir.Var(e.Name, ty), nil

	case *BinOp:
		l, err := pl.lowerExpr(e.L)
		if err != nil {
			return nil, err
		}
		r, err := pl.lowerExpr(e.R)
		if err != nil {
			return nil, err
		}
		op := pasOp(e.Op)
		ty := pasResultTy(l.ExprTy(), r.ExprTy(), op)
		return &hir.BinExpr{Op: op, L: l, R: r, Ty: ty}, nil

	case *UnaryOp:
		x, err := pl.lowerExpr(e.X)
		if err != nil {
			return nil, err
		}
		switch e.Op {
		case "-":
			return &hir.UnaryExpr{Op: "-", X: x, Ty: x.ExprTy()}, nil
		case "NOT":
			return &hir.UnaryExpr{Op: "~", X: x, Ty: x.ExprTy()}, nil
		default:
			return nil, fmt.Errorf("unknown unary op %q", e.Op)
		}

	case *CallExpr:
		return pl.lowerCallExpr(e)

	case *IndexExpr:
		if vr, ok := e.Base.(*VarRef); ok {
			idx, err := pl.lowerExpr(e.Idx)
			if err != nil {
				return nil, err
			}
			elemTy := pl.elemTypeOf(vr.Name)
			base := &hir.AddrOfExpr{Sym: vr.Name}
			return &hir.IndexExpr{Base: base, Idx: idx, ElemTy: elemTy}, nil
		}
		return nil, fmt.Errorf("unsupported index base type %T", e.Base)

	case *FieldExpr:
		base, err := pl.lowerExpr(e.Base)
		if err != nil {
			return nil, err
		}
		// Simplified: field access. The HIR FieldExpr needs offset resolution.
		return &hir.FieldExpr{X: base, Field: e.Field, Ty: mir2.TyU16}, nil

	case *DerefExpr:
		x, err := pl.lowerExpr(e.X)
		if err != nil {
			return nil, err
		}
		return &hir.LoadExpr{Ptr: x, Ty: mir2.TyU16}, nil

	case *AddrExpr:
		if vr, ok := e.X.(*VarRef); ok {
			return &hir.AddrOfExpr{Sym: vr.Name}, nil
		}
		return nil, fmt.Errorf("@expr requires a variable")

	default:
		return nil, fmt.Errorf("unhandled Pascal expression type %T", e)
	}
}

func (pl *procLow) lowerCallExpr(e *CallExpr) (hir.Expr, error) {
	// Built-in functions
	switch e.Name {
	case "ORD":
		if len(e.Args) == 1 {
			return pl.lowerExpr(e.Args[0])
		}
	case "CHR":
		if len(e.Args) == 1 {
			x, err := pl.lowerExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			return hir.Cast(x, mir2.TyU8), nil
		}
	case "LO", "LOW":
		if len(e.Args) == 1 {
			x, err := pl.lowerExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			return &hir.BinExpr{Op: "&", L: x, R: hir.U8(0xFF), Ty: mir2.TyU8}, nil
		}
	case "HI", "HIGH":
		if len(e.Args) == 1 {
			x, err := pl.lowerExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			shifted := &hir.BinExpr{Op: ">>", L: x, R: hir.U8(8), Ty: x.ExprTy()}
			return hir.Cast(shifted, mir2.TyU8), nil
		}
	case "SUCC":
		if len(e.Args) == 1 {
			x, err := pl.lowerExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			return &hir.BinExpr{Op: "+", L: x, R: hir.U8(1), Ty: x.ExprTy()}, nil
		}
	case "PRED":
		if len(e.Args) == 1 {
			x, err := pl.lowerExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			return &hir.BinExpr{Op: "-", L: x, R: hir.U8(1), Ty: x.ExprTy()}, nil
		}
	case "ODD":
		if len(e.Args) == 1 {
			x, err := pl.lowerExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			return &hir.BinExpr{Op: "&", L: x, R: hir.U8(1), Ty: mir2.TyBool}, nil
		}
	case "ABS":
		// Simplified: just pass through (correct for positive values)
		if len(e.Args) == 1 {
			return pl.lowerExpr(e.Args[0])
		}
	case "LENGTH":
		if len(e.Args) == 1 {
			// string[0] holds the length
			if vr, ok := e.Args[0].(*VarRef); ok {
				base := &hir.AddrOfExpr{Sym: vr.Name}
				return &hir.IndexExpr{Base: base, Idx: hir.U8(0), ElemTy: mir2.TyU8}, nil
			}
		}
	}

	// Check if this is an indirect call through a function pointer variable.
	// Check both local (params + local vars) and global var types.
	vt, ok := pl.localTypes[e.Name]
	if !ok {
		vt, ok = pl.low.varTypes[e.Name]
	}
	if ok {
		resolved := pl.low.resolveType(vt)
		if _, isProcPtr := resolved.(*ProcPtrType); isProcPtr {
			args, err := pl.lowerExprs(e.Args)
			if err != nil {
				return nil, err
			}
			retTy := mir2.Ty(mir2.TyVoid)
			if ppt := resolved.(*ProcPtrType); ppt.RetTy != nil {
				retTy = PasTypeToMIR2(ppt.RetTy)
			}
			return &hir.CallIndirectExpr{
				FnPtr: &hir.VarRefExpr{Name: e.Name, Ty: mir2.TyPtr},
				Args:  args,
				Ty:    retTy,
			}, nil
		}
	}

	var retTy mir2.Ty = mir2.TyVoid
	if rt, ok := pl.low.fnRetTy[e.Name]; ok {
		retTy = PasTypeToMIR2(pl.low.resolveType(rt))
	}
	args, err := pl.lowerExprs(e.Args)
	if err != nil {
		return nil, err
	}
	return hir.Call(e.Name, retTy, args...), nil
}

func (pl *procLow) lowerExprs(exprs []Expr) ([]hir.Expr, error) {
	out := make([]hir.Expr, 0, len(exprs))
	for _, e := range exprs {
		he, err := pl.lowerExpr(e)
		if err != nil {
			return nil, err
		}
		out = append(out, he)
	}
	return out, nil
}

// ── Type helpers ─────────────────────────────────────────────────────────────

// internString adds a $-terminated string to the module's string pool
// and returns the symbol name for its address.
func (l *lowerer) internString(s string) string {
	// Append '$' terminator for BDOS function 9
	terminated := s + "$"
	idx := len(l.hm.Strings)
	l.hm.Strings = append(l.hm.Strings, terminated)
	l.hm.StrKinds = append(l.hm.StrKinds, mir2.StrCString)
	return fmt.Sprintf("@mir2.str.%d", idx)
}

func (l *lowerer) resolveType(t PasType) PasType {
	if tr, ok := t.(*TypeRef); ok {
		if resolved, ok := l.types[tr.Name]; ok {
			return l.resolveType(resolved)
		}
	}
	return t
}

func pasOp(op string) string {
	switch op {
	case "=":
		return "=="
	case "<>":
		return "!="
	case "AND":
		return "&"
	case "OR":
		return "|"
	case "XOR":
		return "^"
	case "DIV":
		return "/"
	case "MOD":
		return "%"
	case "SHL":
		return "<<"
	case "SHR":
		return ">>"
	default:
		return op
	}
}

func pasResultTy(l, r mir2.Ty, op string) mir2.Ty {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return mir2.TyBool
	}
	if l == mir2.TyI16 || r == mir2.TyI16 {
		return mir2.TyI16
	}
	if l == mir2.TyU16 || r == mir2.TyU16 {
		return mir2.TyU16
	}
	if l == mir2.TyPtr || r == mir2.TyPtr {
		return mir2.TyPtr
	}
	return mir2.TyU8
}

// ── Auto-import system unit ──────────────────────────────────────────────────


// moduleReferences checks if a function name is called anywhere in the module.
func moduleReferences(hm *hir.Module, name string) bool {
	for _, f := range hm.Funcs {
		if f.Body != nil && blockReferences(f.Body, name) {
			return true
		}
	}
	return false
}

func blockReferences(blk *hir.Block, name string) bool {
	for _, s := range blk.Body {
		if stmtReferences(s, name) {
			return true
		}
	}
	return false
}

func stmtReferences(s hir.Stmt, name string) bool {
	switch s := s.(type) {
	case *hir.ExprStmt:
		return exprReferences(s.Expr, name)
	case *hir.AssignStmt:
		return exprReferences(s.Val, name)
	case *hir.IfStmt:
		if exprReferences(s.Cond, name) {
			return true
		}
		if s.Then != nil && blockReferences(s.Then, name) {
			return true
		}
		if s.Else != nil && blockReferences(s.Else, name) {
			return true
		}
	case *hir.WhileStmt:
		if exprReferences(s.Cond, name) {
			return true
		}
		if s.Body != nil && blockReferences(s.Body, name) {
			return true
		}
	case *hir.Block:
		return blockReferences(s, name)
	case *hir.ReturnStmt:
		if s.Val != nil && exprReferences(s.Val, name) {
			return true
		}
	case *hir.SwitchStmt:
		if exprReferences(s.Val, name) {
			return true
		}
		for _, c := range s.Cases {
			if c.Body != nil && blockReferences(c.Body, name) {
				return true
			}
		}
		if s.Default != nil && blockReferences(s.Default, name) {
			return true
		}
	case *hir.VarDeclStmt:
		if s.Init != nil && exprReferences(s.Init, name) {
			return true
		}
	}
	return false
}

func exprReferences(e hir.Expr, name string) bool {
	if e == nil {
		return false
	}
	switch e := e.(type) {
	case *hir.CallExpr:
		if e.Fn == name {
			return true
		}
		for _, a := range e.Args {
			if exprReferences(a, name) {
				return true
			}
		}
	case *hir.BinExpr:
		return exprReferences(e.L, name) || exprReferences(e.R, name)
	case *hir.UnaryExpr:
		return exprReferences(e.X, name)
	case *hir.CastExpr:
		return exprReferences(e.X, name)
	}
	return false
}

// emitRuntimeFuncs generates built-in CP/M runtime functions directly as HIR
// with inline asm, so BDOS register conventions (C=func, DE=param) are correct.
// Only functions actually referenced by the module are emitted.
func emitRuntimeFuncs(hm *hir.Module) {
	defined := map[string]bool{}
	for _, f := range hm.Funcs {
		defined[f.Name] = true
	}

	need := func(name string) bool {
		return !defined[name] && moduleReferences(hm, name)
	}

	// ConOut(ch: u8) -> void — BDOS 2, char in E
	if need("ConOut") {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "ConOut",
			Params: []hir.Param{{Name: "ch", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{Body: []hir.Stmt{
				&hir.AsmStmt{
					Target:     "z80",
					Code:       "LD E, A / LD C, 2 / CALL 0x0005",
					ClobberAll: true,
					Ins:        []hir.AsmOperand{{Name: "ch"}},
				},
				&hir.ReturnStmt{},
			}},
		})
		defined["ConOut"] = true
	}

	// WriteCrLf() -> void — output CR+LF via inline asm (BDOS 2)
	if need("WriteCrLf") {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "WriteCrLf",
			RetTy: mir2.TyVoid,
			Body: &hir.Block{Body: []hir.Stmt{
				&hir.AsmStmt{
					Target:     "z80",
					Code:       "LD E, 13 / LD C, 2 / CALL 5 / LD E, 10 / LD C, 2 / CALL 5",
					ClobberAll: true,
				},
				&hir.ReturnStmt{},
			}},
		})
		defined["WriteCrLf"] = true
	}

	// WriteStr(addr: u16) -> void — BDOS 9, string addr in DE
	if need("WriteStr") {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "WriteStr",
			Params: []hir.Param{{Name: "addr", Ty: mir2.TyU16}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{Body: []hir.Stmt{
				&hir.AsmStmt{
					Target:     "z80",
					Code:       "EX DE, HL / LD C, 9 / CALL 0x0005",
					ClobberAll: true,
					Ins:        []hir.AsmOperand{{Name: "addr"}},
				},
				&hir.ReturnStmt{},
			}},
		})
		defined["WriteStr"] = true
	}

	// WriteU8(val: u8) -> void — print decimal
	if need("WriteU8") {
		// Ensure ConOut is available
		if !defined["ConOut"] {
			hm.Funcs = append(hm.Funcs, &hir.Func{
				Name:   "ConOut",
				Params: []hir.Param{{Name: "ch", Ty: mir2.TyU8}},
				RetTy:  mir2.TyVoid,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.AsmStmt{
						Target:     "z80",
						Code:       "LD E, A / LD C, 2 / CALL 0x0005",
						ClobberAll: true,
						Ins:        []hir.AsmOperand{{Name: "ch"}},
					},
					&hir.ReturnStmt{},
				}},
			})
			defined["ConOut"] = true
		}
		// WriteU8: emit decimal digits via asm for compact output
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "WriteU8",
			Params: []hir.Param{{Name: "val", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{Body: []hir.Stmt{
				// Use inline asm: val is in A, decompose and print digits
				&hir.AsmStmt{
					Target: "z80",
					Code: "LD B, A / LD C, 0 / LD A, B" +
						" / CP 100 / JR C, .wu8_tens" +
						" / .wu8_h: SUB 100 / INC C / CP 100 / JR NC, .wu8_h" +
						" / LD B, A / LD A, C / ADD A, 48 / LD E, A / PUSH BC / LD C, 2 / CALL 0x0005 / POP BC / LD A, B" +
						" / LD C, 1" + // flag: had hundreds
						" / .wu8_tens: LD B, 0" +
						" / CP 10 / JR C, .wu8_ones" +
						" / .wu8_t: SUB 10 / INC B / CP 10 / JR NC, .wu8_t" +
						" / PUSH AF / LD A, B / ADD A, 48 / LD E, A / LD C, 2 / CALL 0x0005 / POP AF" +
						" / .wu8_ones: ADD A, 48 / LD E, A / LD C, 2 / CALL 0x0005",
					ClobberAll: true,
					Ins:        []hir.AsmOperand{{Name: "val"}},
				},
				&hir.ReturnStmt{},
			}},
		})
		defined["WriteU8"] = true
	}

	// WriteI16(val: u16) -> void — print i16 as decimal via BDOS 2
	if need("WriteI16") {
		if !defined["ConOut"] {
			hm.Funcs = append(hm.Funcs, &hir.Func{
				Name:   "ConOut",
				Params: []hir.Param{{Name: "ch", Ty: mir2.TyU8}},
				RetTy:  mir2.TyVoid,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.AsmStmt{
						Target:     "z80",
						Code:       "LD E, A / LD C, 2 / CALL 0x0005",
						ClobberAll: true,
						Ins:        []hir.AsmOperand{{Name: "ch"}},
					},
					&hir.ReturnStmt{},
				}},
			})
			defined["ConOut"] = true
		}
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "WriteI16",
			Params: []hir.Param{{Name: "val", Ty: mir2.TyU16}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{Body: []hir.Stmt{
				&hir.AsmStmt{
					Target: "z80",
					// HL = val. Print decimal via subtract-and-count.
					Code: "PUSH IX / PUSH HL / POP IX / LD D, 0" +
						" / PUSH IX / POP HL / LD BC, 10000 / CALL _wr16_dig / PUSH HL / POP IX" +
						" / PUSH IX / POP HL / LD BC, 1000 / CALL _wr16_dig / PUSH HL / POP IX" +
						" / PUSH IX / POP HL / LD BC, 100 / CALL _wr16_dig / PUSH HL / POP IX" +
						" / PUSH IX / POP HL / LD BC, 10 / CALL _wr16_dig / PUSH HL / POP IX" +
						" / LD A, IXL / ADD A, 48 / LD E, A / LD C, 2 / CALL 5" +
						" / POP IX",
					ClobberAll: true,
					Ins:        []hir.AsmOperand{{Name: "val"}},
				},
				&hir.ReturnStmt{},
			}},
		})
		// Helper: _wr16_dig — same as ABAP's _abap_wr_dig
		if !defined["_wr16_dig"] {
			hm.Funcs = append(hm.Funcs, &hir.Func{
				Name:  "_wr16_dig",
				RetTy: mir2.TyVoid,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "LD A, 48" +
							" / _w16_sub: OR A / SBC HL, BC / JR NC, _w16_cont" +
							" / ADD HL, BC / CP 48 / JR NZ, _w16_pr" +
							" / LD A, D / OR A / RET Z / LD A, 48" +
							" / _w16_pr: LD D, 1 / LD E, A / PUSH HL / PUSH DE / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP DE / POP HL / RET" +
							" / _w16_cont: INC A / JR _w16_sub",
						ClobberAll: true,
					},
					&hir.ReturnStmt{},
				}},
			})
			defined["_wr16_dig"] = true
		}
		defined["WriteI16"] = true
	}

	// PascalHalt() -> void — BDOS 0
	if need("PascalHalt") {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "PascalHalt",
			RetTy: mir2.TyVoid,
			Body: &hir.Block{Body: []hir.Stmt{
				&hir.AsmStmt{
					Target:     "z80",
					Code:       "LD C, 0 / CALL 0x0005",
					ClobberAll: true,
				},
				&hir.ReturnStmt{},
			}},
		})
		defined["PascalHalt"] = true
	}
}
