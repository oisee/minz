// Package plm implements a PL/M-80 frontend that compiles PL/M-80 source to
// HIR (pkg/hir), which is then lowered to MIR2 and finally Z80 assembly.
//
// Pipeline:
//
//	PL/M-80 source
//	    → Lexer   (lex.go)
//	    → Parser  (parser.go) → pkg/plm AST
//	    → Lowerer (lower.go)  → pkg/hir Module
//	    → hir.LowerModule     → pkg/mir2 Module
//	    → Z80Codegen          → .a80 assembly
package plm

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Compile is the top-level entry point: PL/M source → HIR module.
func Compile(src string) (*hir.Module, error) {
	m, err := ParseModule(src)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	hm, err := LowerModule(m)
	if err != nil {
		return nil, fmt.Errorf("lower: %w", err)
	}
	return hm, nil
}

// LowerModule converts a parsed PL/M-80 Module to a HIR module.
func LowerModule(pm *Module) (*hir.Module, error) {
	l := &modLowerer{
		pm:      pm,
		fnRetTy: make(map[string]PLMType),
	}
	return l.lower()
}

// ── Module lowerer ────────────────────────────────────────────────────────────

type modLowerer struct {
	pm      *Module
	fnRetTy map[string]PLMType // return type per procedure name (first-pass)
}

func (l *modLowerer) lower() (*hir.Module, error) {
	hm := &hir.Module{Name: l.pm.Name}

	// First pass: collect procedure return types so call expressions can be typed.
	for _, d := range l.pm.Decls {
		if pd, ok := d.(*ProcDecl); ok {
			l.fnRetTy[pd.Name] = pd.RetTy
		}
	}

	// Second pass: lower every declaration.
	for _, d := range l.pm.Decls {
		switch d := d.(type) {
		case *VarDecl:
			for _, name := range d.Names {
				ty := plmToMIR2(d.Ty)
				// mir2.Global has no Addr field; AT() address is recorded as a
				// comment for future linker support.
				g := mir2.Global{Name: name, Ty: ty}
				hm.Globals = append(hm.Globals, g)
			}
		case *ProcDecl:
			f, err := l.lowerProc(d)
			if err != nil {
				return nil, fmt.Errorf("procedure %s: %w", d.Name, err)
			}
			hm.Funcs = append(hm.Funcs, f)
		}
	}
	return hm, nil
}

// ── Procedure lowerer ─────────────────────────────────────────────────────────

type procLowerer struct {
	ml         *modLowerer
	paramTypes map[string]PLMType // name → PLM type (from DECLARE inside proc)
	localTypes map[string]PLMType // non-param locals
}

func (l *modLowerer) lowerProc(pd *ProcDecl) (*hir.Func, error) {
	// Build a name→type map from all DECLARE statements in the procedure.
	declared := make(map[string]PLMType)
	for _, vd := range pd.Decls {
		for _, n := range vd.Names {
			declared[n] = vd.Ty
		}
	}

	// Separate params from locals.
	paramSet := make(map[string]bool)
	for _, n := range pd.Params {
		paramSet[n] = true
	}

	// Build HIR param list.
	var params []hir.Param
	for _, name := range pd.Params {
		ty, ok := declared[name]
		if !ok {
			ty = PLMWord // default: WORD if not explicitly declared
		}
		params = append(params, hir.Param{Name: name, Ty: plmToMIR2(ty)})
	}

	// Collect non-param locals (for VarDeclStmt at top of body).
	localTypes := make(map[string]PLMType)
	for n, ty := range declared {
		if !paramSet[n] {
			localTypes[n] = ty
		}
	}

	hf := &hir.Func{
		Name:   pd.Name,
		Params: params,
		RetTy:  plmToMIR2(pd.RetTy),
	}

	pl := &procLowerer{
		ml:         l,
		paramTypes: declared,
		localTypes: localTypes,
	}

	// Emit local variable declarations at the top of the body.
	var stmts []hir.Stmt
	for _, vd := range pd.Decls {
		for _, name := range vd.Names {
			if !paramSet[name] {
				stmts = append(stmts, hir.Decl(name, plmToMIR2(vd.Ty), nil))
			}
		}
	}

	// Lower body statements.
	for _, s := range pd.Body {
		hs, err := pl.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, hs...)
	}

	hf.Body = hir.Blk(stmts...)
	return hf, nil
}

// typeOf returns the PLM type of a named variable (param or local).
func (pl *procLowerer) typeOf(name string) PLMType {
	if ty, ok := pl.paramTypes[name]; ok {
		return ty
	}
	if ty, ok := pl.localTypes[name]; ok {
		return ty
	}
	return PLMWord // default
}

// ── Statement lowering ────────────────────────────────────────────────────────

func (pl *procLowerer) lowerStmt(s Stmt) ([]hir.Stmt, error) {
	switch s := s.(type) {
	case *AssignStmt:
		val, err := pl.lowerExpr(s.Val)
		if err != nil {
			return nil, err
		}
		ty := plmToMIR2(pl.typeOf(s.Name))
		return []hir.Stmt{hir.Assign(hir.Var(s.Name, ty), val)}, nil

	case *CallStmt:
		var retTy mir2.Ty = mir2.TyVoid
		if rt, ok := pl.ml.fnRetTy[s.Fn]; ok {
			retTy = plmToMIR2(rt)
		}
		args, err := pl.lowerExprs(s.Args)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call(s.Fn, retTy, args...)}}, nil

	case *ReturnStmt:
		if s.Val == nil {
			return []hir.Stmt{hir.RetVoid()}, nil
		}
		val, err := pl.lowerExpr(s.Val)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{hir.Ret(val)}, nil

	case *IfStmt:
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

	case *DoBlock:
		return pl.lowerStmtList(s.Body)

	case *DoWhileStmt:
		cond, err := pl.lowerExpr(s.Cond)
		if err != nil {
			return nil, err
		}
		body, err := pl.lowerStmtList(s.Body)
		if err != nil {
			return nil, err
		}
		return []hir.Stmt{hir.While(cond, hir.Blk(body...))}, nil

	case *DoCaseStmt:
		sel, err := pl.lowerExpr(s.Sel)
		if err != nil {
			return nil, err
		}
		var cases []*hir.SwitchCase
		for i, arm := range s.Arms {
			armBlk, err := pl.lowerAsBlock(arm)
			if err != nil {
				return nil, err
			}
			cases = append(cases, hir.Case(int64(i), armBlk))
		}
		return []hir.Stmt{hir.Switch(sel, cases, nil)}, nil

	case *DoToStmt:
		// Desugar: var = start; WHILE var <= end { body; var = var + step }
		ty := plmToMIR2(pl.typeOf(s.Var))
		start, err := pl.lowerExpr(s.Start)
		if err != nil {
			return nil, err
		}
		end, err := pl.lowerExpr(s.End)
		if err != nil {
			return nil, err
		}
		var step hir.Expr
		if s.Step != nil {
			step, err = pl.lowerExpr(s.Step)
			if err != nil {
				return nil, err
			}
		} else {
			step = hir.U8(1)
		}
		lhs := hir.Var(s.Var, ty)
		init := hir.Assign(lhs, start)
		// Use I < (end + step) instead of I <= end to avoid the broken CmpLe path.
		endPlusStep := &hir.BinExpr{Op: "+", L: end, R: step, Ty: ty}
		cond := hir.Lt(hir.Var(s.Var, ty), endPlusStep)
		bodyStmts, err := pl.lowerStmtList(s.Body)
		if err != nil {
			return nil, err
		}
		// Append: var = var + step
		inc := hir.Assign(hir.Var(s.Var, ty),
			&hir.BinExpr{Op: "+", L: hir.Var(s.Var, ty), R: step, Ty: ty})
		bodyStmts = append(bodyStmts, inc)
		return []hir.Stmt{init, hir.While(cond, hir.Blk(bodyStmts...))}, nil

	case *EnableStmt:
		// Lower as an ExprStmt calling a built-in intrinsic @ei
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("@ei", mir2.TyVoid)}}, nil

	case *DisableStmt:
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("@di", mir2.TyVoid)}}, nil

	case *HaltStmt:
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("@halt", mir2.TyVoid)}}, nil

	case *GoToStmt:
		// GO TO is not yet lowered; emit a call to a placeholder intrinsic.
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("@goto_"+s.Label, mir2.TyVoid)}}, nil

	default:
		return nil, fmt.Errorf("unhandled PL/M statement type %T", s)
	}
}

func (pl *procLowerer) lowerStmtList(stmts []Stmt) ([]hir.Stmt, error) {
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

// lowerAsBlock lowers a statement as a *hir.Block.
// DoBlock maps to a multi-statement block; any other statement becomes a
// single-statement block.
func (pl *procLowerer) lowerAsBlock(s Stmt) (*hir.Block, error) {
	if db, ok := s.(*DoBlock); ok {
		stmts, err := pl.lowerStmtList(db.Body)
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

// ── Expression lowering ───────────────────────────────────────────────────────

func (pl *procLowerer) lowerExpr(e Expr) (hir.Expr, error) {
	switch e := e.(type) {
	case *NumberLit:
		if e.Val <= 255 {
			return hir.U8(int64(e.Val)), nil
		}
		return hir.U16(int64(e.Val)), nil

	case *VarRef:
		ty := plmToMIR2(pl.typeOf(e.Name))
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
		op := plmOp(e.Op)
		ty := binaryResultTy(l.ExprTy(), r.ExprTy(), op)
		return &hir.BinExpr{Op: op, L: l, R: r, Ty: ty}, nil

	case *UnOp:
		x, err := pl.lowerExpr(e.X)
		if err != nil {
			return nil, err
		}
		switch e.Op {
		case "NOT":
			return &hir.UnaryExpr{Op: "~", X: x, Ty: x.ExprTy()}, nil
		case "-":
			return &hir.UnaryExpr{Op: "-", X: x, Ty: x.ExprTy()}, nil
		case ".HIGH.":
			// (x >> 8) truncated to u8
			shifted := &hir.BinExpr{Op: ">>", L: x, R: hir.U8(8), Ty: x.ExprTy()}
			return hir.Cast(shifted, mir2.TyU8), nil
		case ".LOW.":
			// x & 0xFF as u8
			return &hir.BinExpr{Op: "&", L: x, R: hir.U8(0xFF), Ty: mir2.TyU8}, nil
		default:
			return nil, fmt.Errorf("unknown unary op %q", e.Op)
		}

	case *CallExpr:
		var retTy mir2.Ty = mir2.TyVoid
		if rt, ok := pl.ml.fnRetTy[e.Fn]; ok {
			retTy = plmToMIR2(rt)
		}
		args, err := pl.lowerExprs(e.Args)
		if err != nil {
			return nil, err
		}
		return hir.Call(e.Fn, retTy, args...), nil

	case *CastExpr:
		x, err := pl.lowerExpr(e.X)
		if err != nil {
			return nil, err
		}
		return hir.Cast(x, plmToMIR2(e.Ty)), nil

	default:
		return nil, fmt.Errorf("unhandled PL/M expression type %T", e)
	}
}

func (pl *procLowerer) lowerExprs(exprs []Expr) ([]hir.Expr, error) {
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

// ── Type helpers ──────────────────────────────────────────────────────────────

func plmToMIR2(t PLMType) mir2.Ty {
	switch t {
	case PLMByte:
		return mir2.TyU8
	case PLMWord:
		return mir2.TyU16
	case PLMAddress:
		return mir2.TyPtr
	default:
		return mir2.TyVoid
	}
}

// plmOp maps a PL/M-80 operator string to the HIR BinExpr op string.
func plmOp(op string) string {
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
	default:
		return op
	}
}

// binaryResultTy returns the result type of a binary operation.
// Comparison operators always produce TyBool.
// Arithmetic/bitwise: promote to wider of the two operands.
func binaryResultTy(l, r mir2.Ty, op string) mir2.Ty {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return mir2.TyBool
	}
	if l == mir2.TyU16 || r == mir2.TyU16 {
		return mir2.TyU16
	}
	if l == mir2.TyPtr || r == mir2.TyPtr {
		return mir2.TyPtr
	}
	return l
}
