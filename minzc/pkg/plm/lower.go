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
		pm:           pm,
		fnRetTy:      make(map[string]PLMType),
		globalArrays: make(map[string]int),
		globalBased:  make(map[string]string),
		globalAt:     make(map[string]*uint16),
	}
	return l.lower()
}

// ── Module lowerer ────────────────────────────────────────────────────────────

type modLowerer struct {
	pm           *Module
	fnRetTy      map[string]PLMType  // return type per procedure name
	globalArrays map[string]int      // name → array size (0 = scalar)
	globalBased  map[string]string   // name → base variable (BASED)
	globalAt     map[string]*uint16  // name → AT address
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
				elemTy := plmToMIR2(d.Ty)

				// Track array metadata.
				if d.Size != nil {
					l.globalArrays[name] = *d.Size
				}
				// Track BASED variables — don't emit globals for them.
				if d.Based != "" {
					l.globalBased[name] = d.Based
					continue
				}
				// Track AT addresses.
				if d.AtAddr != nil {
					addr := *d.AtAddr
					l.globalAt[name] = &addr
				}

				// Build global type (scalar or array).
				var gTy mir2.Ty = elemTy
				if d.Size != nil {
					gTy = &mir2.ArrayTy{Elem: elemTy, Len: *d.Size}
				}

				g := mir2.Global{Name: name, Ty: gTy}
				if d.AtAddr != nil {
					addr := *d.AtAddr
					g.At = &addr
				}
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
	ml          *modLowerer
	paramTypes  map[string]PLMType // name → PLM type (from DECLARE inside proc)
	localTypes  map[string]PLMType // non-param locals
	localArrays map[string]int     // name → array size for local arrays
	localBased  map[string]string  // name → base variable for BASED locals
	localAt     map[string]*uint16 // name → AT address for local vars
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

	// Collect non-param locals.
	localTypes := make(map[string]PLMType)
	localArrays := make(map[string]int)
	localBased := make(map[string]string)
	localAt := make(map[string]*uint16)
	for _, vd := range pd.Decls {
		for _, n := range vd.Names {
			if !paramSet[n] {
				localTypes[n] = vd.Ty
				if vd.Size != nil {
					localArrays[n] = *vd.Size
				}
				if vd.Based != "" {
					localBased[n] = vd.Based
				}
				if vd.AtAddr != nil {
					addr := *vd.AtAddr
					localAt[n] = &addr
				}
			}
		}
	}

	hf := &hir.Func{
		Name:   pd.Name,
		Params: params,
		RetTy:  plmToMIR2(pd.RetTy),
	}

	pl := &procLowerer{
		ml:          l,
		paramTypes:  declared,
		localTypes:  localTypes,
		localArrays: localArrays,
		localBased:  localBased,
		localAt:     localAt,
	}

	// Emit local variable declarations at the top of the body.
	var stmts []hir.Stmt
	for _, vd := range pd.Decls {
		for _, name := range vd.Names {
			if paramSet[name] {
				continue
			}
			// BASED variables have no storage — they alias through a pointer.
			if vd.Based != "" {
				continue
			}
			elemTy := plmToMIR2(vd.Ty)
			decl := &hir.VarDeclStmt{Name: name, Ty: elemTy}
			if vd.Size != nil {
				decl.ArrayLen = *vd.Size
			}
			if vd.AtAddr != nil {
				addr := *vd.AtAddr
				decl.At = &addr
			}
			stmts = append(stmts, decl)
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

// arraySize returns (size, true) if name is a known array in this scope.
func (pl *procLowerer) arraySize(name string) (int, bool) {
	if n, ok := pl.localArrays[name]; ok {
		return n, true
	}
	if n, ok := pl.ml.globalArrays[name]; ok {
		return n, true
	}
	return 0, false
}

// basedVar returns (baseVarName, true) if name is BASED on another variable.
func (pl *procLowerer) basedVar(name string) (string, bool) {
	if b, ok := pl.localBased[name]; ok {
		return b, true
	}
	if b, ok := pl.ml.globalBased[name]; ok {
		return b, true
	}
	return "", false
}

// ── Statement lowering ────────────────────────────────────────────────────────

func (pl *procLowerer) lowerStmt(s Stmt) ([]hir.Stmt, error) {
	switch s := s.(type) {
	case *AssignStmt:
		val, err := pl.lowerExpr(s.Val)
		if err != nil {
			return nil, err
		}

		// BASED variable write: X BASED Y → *Y = val
		if base, ok := pl.basedVar(s.Name); ok {
			baseTy := plmToMIR2(pl.typeOf(base))
			ptr := hir.Var(base, baseTy)
			return []hir.Stmt{&hir.StoreStmt{Ptr: ptr, Val: val}}, nil
		}

		// Array element write: arr(idx) = val → *(&arr + idx) = val
		if s.Idx != nil {
			idx, err := pl.lowerExpr(s.Idx)
			if err != nil {
				return nil, err
			}
			elemTy := plmToMIR2(pl.typeOf(s.Name))
			base := &hir.AddrOfExpr{Sym: s.Name}
			indexedPtr := &hir.IndexExpr{Base: base, Idx: idx, ElemTy: elemTy}
			return []hir.Stmt{&hir.StoreStmt{Ptr: indexedPtr, Val: val}}, nil
		}

		// PL/M scalar assignment to an array variable (no explicit index) means
		// element 0: arr = val; → arr[0] = val
		if _, ok := pl.arraySize(s.Name); ok {
			elemTy := plmToMIR2(pl.typeOf(s.Name))
			base := &hir.AddrOfExpr{Sym: s.Name}
			indexedPtr := &hir.IndexExpr{Base: base, Idx: hir.U8(0), ElemTy: elemTy}
			return []hir.Stmt{&hir.StoreStmt{Ptr: indexedPtr, Val: val}}, nil
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
			armStmts, err := pl.lowerStmtList([]Stmt{arm})
			if err != nil {
				return nil, err
			}
			cases = append(cases, hir.Case(int64(i), hir.Blk(armStmts...)))
		}
		return []hir.Stmt{hir.Switch(sel, cases, nil)}, nil

	case *DoToStmt:
		ty := plmToMIR2(pl.typeOf(s.Var))
		start, err := pl.lowerExpr(s.Start)
		if err != nil {
			return nil, err
		}
		end, err := pl.lowerExpr(s.End)
		if err != nil {
			return nil, err
		}
		var step hir.Expr = hir.U16(1)
		if s.Step != nil {
			step, err = pl.lowerExpr(s.Step)
			if err != nil {
				return nil, err
			}
		}
		// init: var = start
		init := hir.Assign(hir.Var(s.Var, ty), start)
		// cond: var < (end + 1)  (avoids broken CmpLe on unsigned)
		endPlus1 := &hir.BinExpr{Op: "+", L: end, R: hir.U16(1), Ty: ty}
		cond := &hir.BinExpr{Op: "<", L: hir.Var(s.Var, ty), R: endPlus1, Ty: mir2.TyBool}
		bodyStmts, err := pl.lowerStmtList(s.Body)
		if err != nil {
			return nil, err
		}
		inc := hir.Assign(hir.Var(s.Var, ty), &hir.BinExpr{
			Op: "+", L: hir.Var(s.Var, ty), R: step, Ty: ty,
		})
		bodyStmts = append(bodyStmts, inc)
		return []hir.Stmt{init, hir.While(cond, hir.Blk(bodyStmts...))}, nil

	case *EnableStmt:
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("@ei", mir2.TyVoid)}}, nil

	case *DisableStmt:
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("@di", mir2.TyVoid)}}, nil

	case *HaltStmt:
		return []hir.Stmt{&hir.ExprStmt{Expr: hir.Call("@halt", mir2.TyVoid)}}, nil

	case *GoToStmt:
		// Lower as a call to a synthesized label intrinsic.
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
		// BASED variable read: X BASED Y → *Y (load through pointer)
		if base, ok := pl.basedVar(e.Name); ok {
			elemTy := plmToMIR2(pl.typeOf(e.Name))
			baseTy := plmToMIR2(pl.typeOf(base))
			return &hir.LoadExpr{Ptr: hir.Var(base, baseTy), Ty: elemTy}, nil
		}
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
		// Check: is this actually an array subscript read?  arr(i) in expr context.
		if _, ok := pl.arraySize(e.Fn); ok && len(e.Args) == 1 {
			idx, err := pl.lowerExpr(e.Args[0])
			if err != nil {
				return nil, err
			}
			elemTy := plmToMIR2(pl.typeOf(e.Fn))
			base := &hir.AddrOfExpr{Sym: e.Fn}
			return &hir.IndexExpr{Base: base, Idx: idx, ElemTy: elemTy}, nil
		}
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
		return op // +, -, *, /, <, >, <=, >= pass through
	}
}

// binaryResultTy returns the result type of a binary operation.
// Comparison operators always produce TyBool.
func binaryResultTy(l, r mir2.Ty, op string) mir2.Ty {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return mir2.TyBool
	}
	// Promote: if either side is wider, use the wider type.
	if l == mir2.TyU16 || r == mir2.TyU16 {
		return mir2.TyU16
	}
	if l == mir2.TyPtr || r == mir2.TyPtr {
		return mir2.TyPtr
	}
	return mir2.TyU8
}
