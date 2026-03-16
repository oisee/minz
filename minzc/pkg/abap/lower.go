package abap

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// Compile is the top-level entry point: ABAP source → HIR module.
// It shells out to the abaplint bridge (Node.js) for parsing, then lowers
// the JSON AST to HIR.
func Compile(src, name string) (*hir.Module, error) {
	prog, err := Parse(src, name)
	if err != nil {
		return nil, fmt.Errorf("abap parse: %w", err)
	}
	hm, err := LowerProgram(prog)
	if err != nil {
		return nil, fmt.Errorf("abap lower: %w", err)
	}
	emitRuntimeFuncs(hm)
	return hm, nil
}

// LowerProgram converts a semantic ABAP Program to a HIR module.
func LowerProgram(prog *Program) (*hir.Module, error) {
	l := &lowerer{
		prog:     prog,
		varTypes: make(map[string]mir2.Ty),
	}
	l.hm = &hir.Module{Name: prog.Name}
	return l.lower()
}

type lowerer struct {
	prog     *Program
	hm       *hir.Module
	varTypes map[string]mir2.Ty
}

func (l *lowerer) lower() (*hir.Module, error) {
	// Collect global DATA declarations and FORM signatures.
	var mainBody []Stmt_
	for _, d := range l.prog.Decls {
		switch d := d.(type) {
		case *DataDecl:
			ty := l.abapTypeToHIR(d.AbapTy, d.Length)
			l.varTypes[d.Name] = ty
			l.lowerGlobal(d)
		case *FormDecl:
			l.lowerForm(d)
		case *ClassDecl:
			l.lowerClass(d)
		case *InterfaceDecl:
			l.lowerInterface(d)
		case *formBodyDecl:
			// Top-level statements (outside FORM) → collect into implicit main
			mainBody = append(mainBody, d.stmt)
		}
	}

	// If there are top-level statements, wrap them in a main function.
	if len(mainBody) > 0 {
		body, err := l.lowerStmts(mainBody)
		if err != nil {
			l.hm.Warnings = append(l.hm.Warnings, fmt.Sprintf("main: %v", err))
		} else {
			l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
				Name:  "main",
				RetTy: mir2.TyVoid,
				Body:  body,
			})
		}
	}

	return l.hm, nil
}

// ── Type mapping ─────────────────────────────────────────────────────────────

func (l *lowerer) abapTypeToHIR(abapTy string, length int) mir2.Ty {
	switch abapTy {
	case "i": // ABAP integer (4 bytes) → u16 for Z80
		return mir2.TyU16
	case "int8", "b":
		return mir2.TyU8
	case "c", "n":
		if length <= 1 {
			return mir2.TyU8
		}
		return mir2.TyPtr // pointer to character data
	case "string", "csequence", "clike":
		return mir2.TyPtr
	case "x":
		return mir2.TyU8
	case "f", "p":
		return mir2.TyU16 // approximate
	case "d", "t":
		return mir2.TyPtr
	default:
		return mir2.TyU16
	}
}

// ── Globals ──────────────────────────────────────────────────────────────────

func (l *lowerer) lowerGlobal(d *DataDecl) {
	ty := l.abapTypeToHIR(d.AbapTy, d.Length)
	g := mir2.Global{
		Name: d.Name,
		Ty:   ty,
	}
	if d.Value != nil {
		if lit, ok := (*d.Value).(*IntLit); ok {
			w := mir2.ByteWidth(ty)
			g.Init = make([]byte, w)
			if w >= 1 {
				g.Init[0] = byte(lit.Val)
			}
			if w >= 2 {
				g.Init[1] = byte(lit.Val >> 8)
			}
		}
	}
	l.hm.Globals = append(l.hm.Globals, g)
}

// ── FORM → hir.Func ─────────────────────────────────────────────────────────

func (l *lowerer) lowerForm(d *FormDecl) {
	params := make([]hir.Param, 0, len(d.Using)+len(d.Changing))
	for _, p := range d.Using {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}
	for _, p := range d.Changing {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}

	body, err := l.lowerStmts(d.Body)
	if err != nil {
		l.hm.Warnings = append(l.hm.Warnings, fmt.Sprintf("FORM %s: %v", d.Name, err))
		return
	}

	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
		Name:   d.Name,
		Params: params,
		RetTy:  mir2.TyVoid,
		Body:   body,
	})
}

// ── CLASS → struct + methods ─────────────────────────────────────────────────

func (l *lowerer) lowerClass(d *ClassDecl) {
	st := &mir2.StructTy{Name: d.Name}
	for _, attr := range d.Attrs {
		ty := l.abapTypeToHIR(attr.AbapTy, attr.Length)
		st.Fields = append(st.Fields, mir2.StructField{Name: attr.Name, Ty: ty})
	}
	l.hm.Structs = append(l.hm.Structs, st)

	for _, m := range d.Methods {
		l.lowerMethod(d.Name, m)
	}
}

func (l *lowerer) lowerMethod(className string, m *MethodDecl) {
	selfTy := mir2.TyPtr
	params := []hir.Param{{Name: "self", Ty: selfTy}}

	for _, p := range m.Importing {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}

	var retTy mir2.Ty = mir2.TyVoid
	if m.Returning != nil {
		retTy = l.abapTypeToHIR(m.Returning.AbapTy, 0)
	}

	body, err := l.lowerStmts(m.Body)
	if err != nil {
		l.hm.Warnings = append(l.hm.Warnings, fmt.Sprintf("METHOD %s~%s: %v", className, m.Name, err))
		return
	}

	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
		Name:   fmt.Sprintf("%s_%s", className, m.Name),
		Params: params,
		RetTy:  retTy,
		Body:   body,
	})
}

// ── INTERFACE → hir.InterfaceDecl ────────────────────────────────────────────

func (l *lowerer) lowerInterface(d *InterfaceDecl) {
	l.hm.Interfaces = append(l.hm.Interfaces, &hir.InterfaceDecl{
		Name:    d.Name,
		Methods: d.Methods,
	})
}

// ── Statement lowering ───────────────────────────────────────────────────────

func (l *lowerer) lowerStmts(stmts []Stmt_) (*hir.Block, error) {
	block := &hir.Block{}
	for _, s := range stmts {
		hs, err := l.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		if hs != nil {
			block.Body = append(block.Body, hs)
		}
	}
	return block, nil
}

func (l *lowerer) lowerStmt(s Stmt_) (hir.Stmt, error) {
	switch s := s.(type) {
	case *WriteStmt:
		return l.lowerWrite(s)
	case *AssignStmt:
		return l.lowerAssign(s)
	case *IfStmt:
		return l.lowerIf(s)
	case *DoStmt:
		return l.lowerDo(s)
	case *WhileStmt:
		return l.lowerWhile(s)
	case *PerformStmt:
		return l.lowerPerform(s)
	case *ExitStmt:
		return &hir.BreakStmt{}, nil
	case *ContinueStmt:
		return &hir.ContinueStmt{}, nil
	case *ReturnStmt:
		return &hir.ReturnStmt{}, nil
	case *CaseStmt:
		return l.lowerCase(s)
	default:
		return nil, fmt.Errorf("unsupported ABAP statement: %T", s)
	}
}

func (l *lowerer) lowerWrite(s *WriteStmt) (hir.Stmt, error) {
	// WRITE each expression via abap_write / abap_write_str
	if len(s.Exprs) == 0 {
		return nil, nil
	}
	// For simplicity: emit call for last expression (TODO: emit block for multiple)
	e := s.Exprs[len(s.Exprs)-1]
	he, err := l.lowerExpr(e)
	if err != nil {
		return nil, err
	}

	fnName := "abap_write"
	if _, ok := e.(*StringLit); ok {
		fnName = "abap_write_str"
	}

	return &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   fnName,
			Args: []hir.Expr{he},
			Ty:   mir2.TyVoid,
		},
	}, nil
}

func (l *lowerer) lowerAssign(s *AssignStmt) (hir.Stmt, error) {
	val, err := l.lowerExpr(s.Val)
	if err != nil {
		return nil, err
	}
	if _, exists := l.varTypes[s.Target]; !exists {
		ty := val.ExprTy()
		l.varTypes[s.Target] = ty
		return &hir.VarDeclStmt{
			Name: s.Target,
			Ty:   ty,
			Init: val,
		}, nil
	}
	return &hir.AssignStmt{
		Target: &hir.VarRefExpr{Name: s.Target, Ty: l.varTypes[s.Target]},
		Val:    val,
	}, nil
}

func (l *lowerer) lowerIf(s *IfStmt) (hir.Stmt, error) {
	cond, err := l.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	then, err := l.lowerStmts(s.Then)
	if err != nil {
		return nil, err
	}

	var elseBranch *hir.Block
	// Handle ELSEIF chains by nesting: elseif C { B } → else { if C { B } }
	if len(s.ElseIf) > 0 {
		// Build nested if chain from the end
		var nested hir.Stmt
		if len(s.Else) > 0 {
			nested2, err := l.lowerStmts(s.Else)
			if err != nil {
				return nil, err
			}
			nested = nested2.Body[0] // wrap
			_ = nested
		}
		// Simplified: just handle first elseif + else for now
		ei := s.ElseIf[0]
		eiCond, err := l.lowerExpr(ei.Cond)
		if err != nil {
			return nil, err
		}
		eiThen, err := l.lowerStmts(ei.Body)
		if err != nil {
			return nil, err
		}
		var eiElse *hir.Block
		if len(s.Else) > 0 {
			eiElse, err = l.lowerStmts(s.Else)
			if err != nil {
				return nil, err
			}
		}
		elseBranch = &hir.Block{
			Body: []hir.Stmt{
				&hir.IfStmt{Cond: eiCond, Then: eiThen, Else: eiElse},
			},
		}
	} else if len(s.Else) > 0 {
		elseBranch, err = l.lowerStmts(s.Else)
		if err != nil {
			return nil, err
		}
	}

	return &hir.IfStmt{Cond: cond, Then: then, Else: elseBranch}, nil
}

func (l *lowerer) lowerDo(s *DoStmt) (hir.Stmt, error) {
	body, err := l.lowerStmts(s.Body)
	if err != nil {
		return nil, err
	}
	if s.Times != nil {
		timesExpr, err := l.lowerExpr(*s.Times)
		if err != nil {
			return nil, err
		}
		return &hir.ForRangeStmt{
			Var:   "_abap_sy_index",
			Start: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU16},
			End:   timesExpr,
			Body:  body,
		}, nil
	}
	return &hir.WhileStmt{
		Cond: &hir.BoolLitExpr{Val: true},
		Body: body,
	}, nil
}

func (l *lowerer) lowerWhile(s *WhileStmt) (hir.Stmt, error) {
	cond, err := l.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	body, err := l.lowerStmts(s.Body)
	if err != nil {
		return nil, err
	}
	return &hir.WhileStmt{Cond: cond, Body: body}, nil
}

func (l *lowerer) lowerPerform(s *PerformStmt) (hir.Stmt, error) {
	args := make([]hir.Expr, 0, len(s.Using)+len(s.Changing))
	for _, u := range s.Using {
		e, err := l.lowerExpr(u)
		if err != nil {
			return nil, err
		}
		args = append(args, e)
	}
	for _, c := range s.Changing {
		e, err := l.lowerExpr(c)
		if err != nil {
			return nil, err
		}
		args = append(args, e)
	}
	return &hir.ExprStmt{
		Expr: &hir.CallExpr{Fn: s.FormName, Args: args, Ty: mir2.TyVoid},
	}, nil
}

func (l *lowerer) lowerCase(s *CaseStmt) (hir.Stmt, error) {
	val, err := l.lowerExpr(s.Val)
	if err != nil {
		return nil, err
	}
	var cases []*hir.SwitchCase
	for _, w := range s.Whens {
		if len(w.Vals) > 0 {
			if lit, ok := w.Vals[0].(*IntLit); ok {
				body, err := l.lowerStmts(w.Body)
				if err != nil {
					return nil, err
				}
				cases = append(cases, &hir.SwitchCase{Val: lit.Val, Body: body})
			}
		}
	}
	var defBlock *hir.Block
	if len(s.Others) > 0 {
		defBlock, err = l.lowerStmts(s.Others)
		if err != nil {
			return nil, err
		}
	}
	return &hir.SwitchStmt{Val: val, Cases: cases, Default: defBlock}, nil
}

// ── Expression lowering ──────────────────────────────────────────────────────

func (l *lowerer) lowerExpr(e Expr_) (hir.Expr, error) {
	switch e := e.(type) {
	case *IntLit:
		ty := mir2.TyU16
		if e.Val >= 0 && e.Val <= 255 {
			ty = mir2.TyU8
		}
		return &hir.IntLitExpr{Val: e.Val, Ty: ty}, nil

	case *StringLit:
		// Intern string into module pool, return AddrOfExpr
		idx := len(l.hm.Strings)
		l.hm.Strings = append(l.hm.Strings, e.Val)
		l.hm.StrKinds = append(l.hm.StrKinds, mir2.StrCString) // null-terminated
		return &hir.AddrOfExpr{Sym: fmt.Sprintf("@mir2.str.%d", idx)}, nil

	case *VarRef:
		ty, ok := l.varTypes[e.Name]
		if !ok {
			ty = mir2.TyU16
		}
		return &hir.VarRefExpr{Name: e.Name, Ty: ty}, nil

	case *BinOp:
		return l.lowerBinOp(e)

	case *UnaryOp:
		val, err := l.lowerExpr(e.Val)
		if err != nil {
			return nil, err
		}
		op := "-"
		if e.Op == "NOT" {
			op = "!"
		}
		return &hir.UnaryExpr{Op: op, X: val, Ty: val.ExprTy()}, nil

	case *FuncCall:
		args := make([]hir.Expr, len(e.Args))
		for i, a := range e.Args {
			ha, err := l.lowerExpr(a)
			if err != nil {
				return nil, err
			}
			args[i] = ha
		}
		return &hir.CallExpr{Fn: e.Name, Args: args, Ty: mir2.TyU16}, nil

	default:
		return &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}, nil
	}
}

func (l *lowerer) lowerBinOp(e *BinOp) (hir.Expr, error) {
	lhs, err := l.lowerExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	rhs, err := l.lowerExpr(e.RHS)
	if err != nil {
		return nil, err
	}

	// Map ABAP operators to HIR string operators
	var op string
	switch e.Op {
	case "+":
		op = "+"
	case "-":
		op = "-"
	case "*":
		op = "*"
	case "/":
		op = "/"
	case "MOD", "mod":
		op = "%"
	case "=", "EQ", "eq":
		op = "=="
	case "<>", "NE", "ne":
		op = "!="
	case "<", "LT", "lt":
		op = "<"
	case ">", "GT", "gt":
		op = ">"
	case "<=", "LE", "le":
		op = "<="
	case ">=", "GE", "ge":
		op = ">="
	case "AND", "and":
		op = "&"
	case "OR", "or":
		op = "|"
	default:
		op = "+"
	}

	// Result type: comparisons → bool, arithmetic → promoted type
	ty := lhs.ExprTy()
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		ty = mir2.TyBool
	default:
		if lhs.ExprTy() == mir2.TyU16 || rhs.ExprTy() == mir2.TyU16 {
			ty = mir2.TyU16
		}
	}

	return &hir.BinExpr{Op: op, L: lhs, R: rhs, Ty: ty}, nil
}

// ── Runtime functions ────────────────────────────────────────────────────────

// emitRuntimeFuncs adds built-in ABAP runtime (WRITE output via CP/M BDOS).
func emitRuntimeFuncs(hm *hir.Module) {
	names := map[string]bool{}
	for _, f := range hm.Funcs {
		names[f.Name] = true
	}

	if !names["abap_write"] {
		// Print a u8 value as decimal via CP/M console
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_write",
			Params: []hir.Param{{Name: "val", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target:      "z80",
						Code:        "LD E, A / LD C, 2 / CALL 5",
						Ins:         []hir.AsmOperand{{Name: "val"}},
						ClobberRegs: []string{"A", "C", "D", "E"},
					},
				},
			},
		})
	}

	if !names["abap_write_str"] {
		// Print a null-terminated string via CP/M BDOS 9
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_write_str",
			Params: []hir.Param{{Name: "str", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target:      "z80",
						Code:        "EX DE, HL / LD C, 9 / CALL 5",
						Ins:         []hir.AsmOperand{{Name: "str"}},
						ClobberRegs: []string{"A", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
	}
}
