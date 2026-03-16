package abap

import (
	"fmt"
	"strings"

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

type stringInit struct {
	varName string
	strIdx  int
	ty      mir2.Ty
}

type lowerer struct {
	prog               *Program
	hm                 *hir.Module
	varTypes           map[string]mir2.Ty
	paramRegistrations []*ParamDecl // collected during lowering
	stringInits        []stringInit // DATA x TYPE string VALUE '...' → deferred init
}

func (l *lowerer) lower() (*hir.Module, error) {
	// ── SY structure (global system variable) ────────────────────────────
	l.emitSYStruct()

	// ── Collect declarations ─────────────────────────────────────────────
	var mainBody []Stmt_
	for _, d := range l.prog.Decls {
		switch d := d.(type) {
		case *DataDecl:
			ty := l.abapTypeToHIR(d.AbapTy, d.Length)
			l.varTypes[d.Name] = ty
			l.lowerGlobal(d)
		case *ParamDecl:
			// PARAMETERS → global variable + register in screen table
			ty := l.abapTypeToHIR(d.AbapTy, d.Length)
			l.varTypes[d.Name] = ty
			dd := &DataDecl{Name: d.Name, AbapTy: d.AbapTy, Length: d.Length, Value: d.Default}
			l.lowerGlobal(dd)
			l.lowerParamRegister(d)
		case *FormDecl:
			l.lowerForm(d)
		case *ClassDecl:
			l.lowerClass(d)
		case *InterfaceDecl:
			l.lowerInterface(d)
		case *EventDecl:
			// Events handled below
		case *formBodyDecl:
			mainBody = append(mainBody, d.stmt)
		}
	}

	// ── Lower events into the main() execution flow ──────────────────────
	//
	// ABAP report execution order:
	//   1. Globals + PARAMETERS defaults (already done above)
	//   2. INITIALIZATION event
	//   3. Selection screen display (sel_show host function)
	//   4. AT-SELECTION-SCREEN event (with SY-UCOMM)
	//   5. START-OF-SELECTION event
	//   6. END-OF-SELECTION event
	//   7. Top-level statements (non-event code)

	var mainStmts []hir.Stmt

	// String DATA globals — assign interned string addresses
	for _, si := range l.stringInits {
		mainStmts = append(mainStmts, &hir.AssignStmt{
			Target: &hir.VarRefExpr{Name: si.varName, Ty: si.ty},
			Val:    &hir.AddrOfExpr{Sym: fmt.Sprintf("@mir2.str.%d", si.strIdx)},
		})
	}

	// ── PARAMETERS: allocate per-param buffers, set defaults, prompt ────
	//
	// Each string param gets a static buffer: _abap_buf_<name>
	//   [0]: max length (for BDOS 0x0A)
	//   [1]: actual length (filled by BDOS)
	//   [2..]: text data (null-terminated)
	//
	// p_name always points to _abap_buf_<name> + 2 (text area).
	// Default value pre-filled. Empty input = keep default.
	for _, p := range l.prog.Params {
		ty := l.varTypes[p.Name]
		if ty == mir2.TyPtr {
			// String param: allocate TWO adjacent globals:
			//   _abap_hdr_<name>: [max_len, actual_len] (2 bytes, BDOS 0x0A header)
			//   _abap_txt_<name>: [text..., NUL]         (text data area)
			//
			// BDOS 0x0A writes starting at the header, text lands in txt.
			// p_name points to _abap_txt_<name> (the text part).
			// If user enters empty line, txt keeps the pre-filled default.

			bufLen := p.Length
			if bufLen < 20 {
				bufLen = 20
			}
			defStr := ""
			if p.Default != nil {
				if sl, ok := (*p.Default).(*StringLit); ok {
					defStr = sl.Val
				}
			}

			hdrName := "_abap_hdr_" + p.Name
			txtName := "_abap_txt_" + p.Name

			// Header: [max_len, pre-filled actual_len]
			l.hm.Globals = append(l.hm.Globals, mir2.Global{
				Name: hdrName,
				Ty:   mir2.TyU8,
				Init: []byte{byte(bufLen), byte(len(defStr))},
			})
			// Text area: default value + padding + NUL
			txtData := make([]byte, bufLen+1) // +1 for NUL
			copy(txtData, []byte(defStr))
			l.hm.Globals = append(l.hm.Globals, mir2.Global{
				Name: txtName,
				Ty:   mir2.TyU8,
				Init: txtData,
			})

			// p_name = &_abap_txt_<name>
			mainStmts = append(mainStmts, &hir.AssignStmt{
				Target: &hir.VarRefExpr{Name: p.Name, Ty: ty},
				Val:    &hir.AddrOfExpr{Sym: txtName},
			})
		} else if p.Default != nil {
			// Integer param: just set the default value
			val, err := l.lowerExpr(*p.Default)
			if err == nil {
				mainStmts = append(mainStmts, &hir.AssignStmt{
					Target: &hir.VarRefExpr{Name: p.Name, Ty: ty},
					Val:    val,
				})
			}
		}
	}

	// INITIALIZATION
	if body, ok := l.prog.Events["INITIALIZATION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// Selection screen — prompt for each PARAMETER, read via BDOS 0x0A
	if len(l.prog.Params) > 0 {
		for _, p := range l.prog.Params {
			// Build prompt string: "P_NAME [default]: "
			promptStr := strings.ToUpper(p.Name)
			defStr := ""
			if p.Default != nil {
				if sl, ok := (*p.Default).(*StringLit); ok {
					defStr = sl.Val
				} else if il, ok := (*p.Default).(*IntLit); ok {
					defStr = fmt.Sprintf("%d", il.Val)
				}
			}
			if defStr != "" {
				promptStr += " [" + defStr + "]"
			}
			promptStr += ": "

			promptIdx := len(l.hm.Strings)
			l.hm.Strings = append(l.hm.Strings, promptStr)
			l.hm.StrKinds = append(l.hm.StrKinds, mir2.StrCString)

			// Print prompt
			mainStmts = append(mainStmts, &hir.ExprStmt{
				Expr: &hir.CallExpr{
					Fn:   "abap_write_str",
					Args: []hir.Expr{&hir.AddrOfExpr{Sym: fmt.Sprintf("@mir2.str.%d", promptIdx)}},
					Ty:   mir2.TyVoid,
				},
			})

			ty := l.varTypes[p.Name]
			if ty == mir2.TyPtr {
				// Read into the param's TEXT area via abap_sel_read(txt_ptr)
				txtName := "_abap_txt_" + p.Name
				mainStmts = append(mainStmts, &hir.ExprStmt{
					Expr: &hir.CallExpr{
						Fn:   "abap_sel_read",
						Args: []hir.Expr{&hir.AddrOfExpr{Sym: txtName}},
						Ty:   mir2.TyVoid,
					},
				})
			} else {
				// Integer: read line into shared buffer, parse, assign
				mainStmts = append(mainStmts, &hir.AssignStmt{
					Target: &hir.VarRefExpr{Name: p.Name, Ty: ty},
					Val:    &hir.CallExpr{Fn: "abap_read_int", Ty: mir2.TyU16},
				})
			}
		}

		// Newline after selection screen
		nlIdx := len(l.hm.Strings)
		l.hm.Strings = append(l.hm.Strings, "\r\n")
		l.hm.StrKinds = append(l.hm.StrKinds, mir2.StrCString)
		mainStmts = append(mainStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "abap_write_str",
				Args: []hir.Expr{&hir.AddrOfExpr{Sym: fmt.Sprintf("@mir2.str.%d", nlIdx)}},
				Ty:   mir2.TyVoid,
			},
		})
	}

	// AT-SELECTION-SCREEN — runs after screen, SY-UCOMM is set
	if body, ok := l.prog.Events["AT-SELECTION-SCREEN"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// START-OF-SELECTION
	if body, ok := l.prog.Events["START-OF-SELECTION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// Top-level statements (non-event code)
	if len(mainBody) > 0 {
		stmts, err := l.lowerStmts(mainBody)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// END-OF-SELECTION
	if body, ok := l.prog.Events["END-OF-SELECTION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// Emit main function
	if len(mainStmts) > 0 {
		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:  "main",
			RetTy: mir2.TyVoid,
			Body:  &hir.Block{Body: mainStmts},
		})
	}

	return l.hm, nil
}

// emitSYStruct creates the SY system structure and a global instance.
func (l *lowerer) emitSYStruct() {
	// SY struct: INDEX, SUBRC, TABIX, UCOMM (u16 each for simplicity)
	st := &mir2.StructTy{
		Name: "SY",
		Fields: []mir2.StructField{
			{Name: "SUBRC", Ty: mir2.TyU16},
			{Name: "INDEX", Ty: mir2.TyU16},
			{Name: "TABIX", Ty: mir2.TyU16},
			{Name: "UCOMM", Ty: mir2.TyU16}, // simplified: command code as u16
			{Name: "DATUM", Ty: mir2.TyU16},  // date as days-since-epoch (simplified)
			{Name: "UZEIT", Ty: mir2.TyU16},  // time as minutes-since-midnight
		},
	}
	l.hm.Structs = append(l.hm.Structs, st)

	// Global SY instance (zero-initialized)
	width := 0
	for _, f := range st.Fields {
		width += int(mir2.ByteWidth(f.Ty))
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{
		Name: "sy",
		Ty:   mir2.TyPtr, // pointer to struct
	})
	// Track field types for SY-FIELD access
	for _, f := range st.Fields {
		l.varTypes["SY-"+f.Name] = f.Ty
	}
}

// lowerParamRegister emits a sel_register() call for a PARAMETERS field.
func (l *lowerer) lowerParamRegister(p *ParamDecl) {
	// Register parameter with selection screen host function
	// sel_register(name_ptr, type_code, length, default_ptr)
	// This is called at startup to build the screen field list.
	// The actual registration happens via a generated init function.
	l.paramRegistrations = append(l.paramRegistrations, p)
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
		switch lit := (*d.Value).(type) {
		case *IntLit:
			w := mir2.ByteWidth(ty)
			g.Init = make([]byte, w)
			if w >= 1 {
				g.Init[0] = byte(lit.Val)
			}
			if w >= 2 {
				g.Init[1] = byte(lit.Val >> 8)
			}
		case *StringLit:
			// String DATA → intern the string and record an init assignment
			idx := len(l.hm.Strings)
			l.hm.Strings = append(l.hm.Strings, lit.Val)
			l.hm.StrKinds = append(l.hm.StrKinds, mir2.StrCString)
			l.stringInits = append(l.stringInits, stringInit{
				varName: d.Name,
				strIdx:  idx,
				ty:      ty,
			})
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
	} else if he.ExprTy() == mir2.TyPtr {
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

	case *SYField:
		// SY-INDEX etc. → host function call sy_get_FIELD()
		fnName := fmt.Sprintf("sy_get_%s", strings.ToLower(e.Field))
		return &hir.CallExpr{Fn: fnName, Ty: mir2.TyU16}, nil

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

// emitRuntimeFuncs adds built-in ABAP runtime functions.
func emitRuntimeFuncs(hm *hir.Module) {
	names := map[string]bool{}
	for _, f := range hm.Funcs {
		names[f.Name] = true
	}

	// SY-* getter stubs (return 0 on Z80, overridden by MZV host functions)
	syFields := []string{"subrc", "index", "tabix", "ucomm", "datum", "uzeit"}
	for _, field := range syFields {
		fnName := "sy_get_" + field
		if !names[fnName] {
			hm.Funcs = append(hm.Funcs, &hir.Func{
				Name:  fnName,
				RetTy: mir2.TyU16,
				Body: &hir.Block{
					Body: []hir.Stmt{
						&hir.ReturnStmt{Val: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}},
					},
				},
			})
			names[fnName] = true
		}
	}

	// sel_show — no-op on Z80 (overridden by MZV host function for TUI)
	if !names["sel_show"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "sel_show",
			RetTy: mir2.TyVoid,
			Body:  &hir.Block{},
		})
		names["sel_show"] = true
	}

	// sel_register — no-op on Z80
	if !names["sel_register"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "sel_register",
			Params: []hir.Param{
				{Name: "name", Ty: mir2.TyPtr},
				{Name: "ty", Ty: mir2.TyU8},
				{Name: "length", Ty: mir2.TyU8},
			},
			RetTy: mir2.TyVoid,
			Body:  &hir.Block{},
		})
		names["sel_register"] = true
	}

	// abap_sel_read(buf: ^u8) — read line into buffer via BDOS 0x0A.
	// Buffer format: [max_len, actual_len, text...].
	// If user enters empty line (len=0), keeps pre-filled default.
	// Null-terminates the text after actual_len bytes.
	if !names["abap_sel_read"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_sel_read",
			Params: []hir.Param{{Name: "buf", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// HL = buf pointer (ClassPointer)
						// Save old actual_len (for empty-input check)
						// HL = destination text pointer (param's text area).
						// We read into _abap_inbuf (shared), then copy to dest if non-empty.
						Code: "PUSH HL / " + // save dest ptr
							// Read into shared inbuf via BDOS 0x0A
							"LD DE, _abap_inbuf / LD A, 20 / LD (_abap_inbuf), A / LD C, 10 / CALL 5 / " +
							// Print CR/LF
							"LD E, 13 / LD C, 2 / CALL 5 / LD E, 10 / LD C, 2 / CALL 5 / " +
							// Check actual_len at inbuf+1
							"LD A, (_abap_inbuf+1) / OR A / POP HL / JR Z, .keep / " +
							// Non-empty: copy inbuf+2..inbuf+2+len to dest, then NUL
							"LD B, A / LD DE, _abap_inbuf+2 / " +
							".copy: LD A, (DE) / LD (HL), A / INC HL / INC DE / DJNZ .copy / " +
							"LD (HL), 0 / JR .done / " + // null-terminate
							".keep: / " + // empty input: dest unchanged (keeps default)
							".done:",
						Ins:         []hir.AsmOperand{{Name: "buf"}},
						ClobberRegs: []string{"A", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["abap_sel_read"] = true
	}

	// abap_read_int — read integer from console (read line, parse decimal)
	if !names["abap_read_int"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "abap_read_int",
			RetTy: mir2.TyU16,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// Read line via BDOS 0x0A, then parse decimal
						Code: "LD HL, _abap_inbuf / LD (HL), 20 / EX DE, HL / LD C, 10 / CALL 5 / " +
							// Print newline
							"LD E, 13 / LD C, 2 / CALL 5 / LD E, 10 / LD C, 2 / CALL 5 / " +
							// Parse decimal: HL = _abap_inbuf+2, count = (inbuf+1)
							"LD HL, _abap_inbuf+1 / LD B, (HL) / INC HL / " +
							// Accumulate in DE
							"LD DE, 0 / " +
							".parse_loop: LD A, B / OR A / JR Z, .parse_done / " +
							"LD A, (HL) / SUB 48 / JR C, .parse_done / CP 10 / JR NC, .parse_done / " +
							// DE = DE * 10 + A
							"PUSH HL / PUSH AF / " +
							"LD H, D / LD L, E / ADD HL, HL / ADD HL, HL / ADD HL, DE / ADD HL, HL / " +
							"POP AF / LD E, A / LD D, 0 / ADD HL, DE / EX DE, HL / POP HL / " +
							"INC HL / DEC B / JR .parse_loop / " +
							".parse_done: EX DE, HL",
						RetReg:      "HL",
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["abap_read_int"] = true
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

	// Input buffer for BDOS 0x0A (22 bytes: 1 max + 1 actual + 20 chars)
	hasInbuf := false
	for _, g := range hm.Globals {
		if g.Name == "_abap_inbuf" {
			hasInbuf = true
		}
	}
	if !hasInbuf && (names["abap_read_line"] || names["abap_read_int"]) {
		hm.Globals = append(hm.Globals, mir2.Global{
			Name: "_abap_inbuf",
			Ty:   mir2.TyU8,
			Init: make([]byte, 22), // 22 bytes zeroed
		})
	}

	if !names["abap_write_str"] {
		// Print a null-terminated string by outputting char-by-char via BDOS 2
		// (BDOS 9 requires $-terminated strings, but we use C-strings with NUL)
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_write_str",
			Params: []hir.Param{{Name: "str", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: ".loop: LD A, (HL) / OR A / RET Z / LD E, A / LD C, 2 / PUSH HL / CALL 5 / POP HL / INC HL / JR .loop",
						Ins:         []hir.AsmOperand{{Name: "str"}},
						ClobberRegs: []string{"A", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
	}
}
