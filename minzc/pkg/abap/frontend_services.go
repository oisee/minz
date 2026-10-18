package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"strconv"
	"strings"
)

// FrontendServiceStmt is the admitted, named client-side subset. It has no
// DATASET/server-filesystem semantics and no generic unknown-method fallback.
type FrontendServiceStmt struct {
	Method              string
	Exporting, Changing map[string]Expr_
	Exceptions          map[string]int64
}

func (*FrontendServiceStmt) abapStmt() {}

func parseFrontendService(tokens []string) (Decl, error) {
	t := append([]string(nil), tokens...)
	if len(t) > 2 && strings.EqualFold(t[0], "CALL") && strings.EqualFold(t[1], "METHOD") {
		t = t[2:]
	}
	if len(t) < 7 || !strings.EqualFold(t[0], "cl_gui_frontend_services") || t[1] != "=>" || t[3] != "(" || t[len(t)-2] != ")" || t[len(t)-1] != "." {
		return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES: unsupported call form %s", strings.Join(t, " "))
	}
	method := strings.ToLower(t[2])
	allowed := map[string]map[string]bool{}
	switch method {
	case "file_open_dialog":
		allowed["EXPORTING"] = map[string]bool{"window_title": true, "file_filter": true, "default_extension": true, "initial_directory": true}
		allowed["CHANGING"] = map[string]bool{"file_table": true, "rc": true, "user_action": true}
	case "directory_list_files":
		allowed["EXPORTING"] = map[string]bool{"directory": true, "filter": true}
		allowed["CHANGING"] = map[string]bool{"file_table": true, "count": true}
	case "gui_upload":
		allowed["EXPORTING"] = map[string]bool{"filename": true, "filetype": true}
		allowed["CHANGING"] = map[string]bool{"data_tab": true}
		allowed["EXCEPTIONS"] = map[string]bool{"file_open_error": true, "others": true}
	default:
		return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES method %s is not implemented", method)
	}
	s := &FrontendServiceStmt{Method: method, Exporting: map[string]Expr_{}, Changing: map[string]Expr_{}, Exceptions: map[string]int64{}}
	section := ""
	seen := map[string]bool{}
	for i := 4; i < len(t)-2; {
		keyword := strings.ToUpper(t[i])
		if keyword == "EXPORTING" || keyword == "CHANGING" || keyword == "IMPORTING" || keyword == "EXCEPTIONS" || keyword == "RECEIVING" {
			if allowed[keyword] == nil {
				return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES=>%s: section %s is not admitted", method, keyword)
			}
			section = keyword
			i++
			continue
		}
		name := strings.ToLower(t[i])
		if !allowed[section][name] {
			return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES=>%s: parameter/exception %s is not admitted in %s", method, name, section)
		}
		if seen[name] {
			return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES=>%s: duplicate parameter %s", method, name)
		}
		seen[name] = true
		if i+2 >= len(t)-2 || t[i+1] != "=" {
			return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES=>%s parameter %s requires a single supported operand", method, name)
		}
		if section == "EXCEPTIONS" {
			n, err := strconv.ParseInt(t[i+2], 10, 32)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("GUI_UPLOAD exception %s requires a positive integer mapping", name)
			}
			s.Exceptions[name] = n
		} else {
			e, err := expressionAtom(t[i+2])
			if err != nil {
				return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES=>%s parameter %s: %w", method, name, err)
			}
			if v, ok := e.(*VarRef); ok {
				v.Name = strings.ToLower(v.Name)
			}
			if section == "CHANGING" {
				switch e.(type) {
				case *VarRef, *FieldAccess:
				default:
					return nil, fmt.Errorf("CL_GUI_FRONTEND_SERVICES=>%s CHANGING %s requires an owned variable", method, name)
				}
				s.Changing[name] = e
			} else {
				s.Exporting[name] = e
			}
		}
		i += 3
	}
	return &formBodyDecl{stmt: s}, nil
}

func (l *lowerer) frontendText(s *FrontendServiceStmt, name, defaultText string) (characterBinding, error) {
	e := s.Exporting[name]
	if e == nil {
		e = &StringLit{Val: defaultText}
	}
	b, ok, err := l.characterExpr(e)
	if err != nil {
		return b, err
	}
	if !ok {
		return b, fmt.Errorf("%s parameter %s requires c/string", s.Method, name)
	}
	return b, nil
}
func frontendTargetName(e Expr_) (string, bool) {
	switch v := e.(type) {
	case *VarRef:
		return v.Name, true
	case *FieldAccess:
		if obj, ok := v.Obj.(*VarRef); ok {
			return obj.Name + "-" + v.Field, true
		}
	}
	return "", false
}
func (l *lowerer) frontendInteger(s *FrontendServiceStmt, name string) (hir.Expr, error) {
	target, ok := frontendTargetName(s.Changing[name])
	if !ok {
		return nil, fmt.Errorf("%s: missing CHANGING %s", s.Method, name)
	}
	return l.integerTarget(target)
}
func (l *lowerer) frontendTable(s *FrontendServiceStmt, name string) (*tableObject, byte, error) {
	v, ok := s.Changing[name].(*VarRef)
	if !ok {
		return nil, 0, fmt.Errorf("%s: CHANGING %s requires a declared table", s.Method, name)
	}
	t := l.tables[v.Name]
	if t == nil {
		return nil, 0, fmt.Errorf("%s parameter %s is not an internal table", s.Method, name)
	}
	if name == "data_tab" {
		if t.line.elementary == "c" {
			return t, 0, nil
		}
		if t.line.elementary == "string" {
			return t, 1, nil
		}
		return nil, 0, fmt.Errorf("GUI_UPLOAD data_tab requires elementary c/string lines")
	}
	if t.line.elementary != "" || len(t.line.comps) != 1 || t.line.comps[0].name != "filename" {
		return nil, 0, fmt.Errorf("%s file_table requires exactly one filename component", s.Method)
	}
	c := t.line.comps[0]
	if c.abapTy == "c" && c.length >= 12 {
		return t, 0, nil
	}
	if c.abapTy == "string" && c.length >= 16 {
		return t, 1, nil
	}
	return nil, 0, fmt.Errorf("%s filename must own c LENGTH >= 12 or a bounded string", s.Method)
}
func tableFrontendArgs(t *tableObject, kind byte) []hir.Expr {
	return []hir.Expr{t.base, &hir.AddrOfExpr{Sym: t.count}, charLen(t.capacity), charLen(t.line.size), &hir.IntLitExpr{Val: int64(kind), Ty: mir2.TyU8}}
}
func (l *lowerer) lowerFrontendService(s *FrontendServiceStmt) (hir.Stmt, error) {
	if l.hm.Target != hir.TargetCPM && l.hm.Target != hir.TargetAgon && l.hm.Target != hir.TargetZXSpectrum {
		return nil, fmt.Errorf("%s: frontend filesystem boundary requires CP/M BDOS, explicit Agon native MOS runtime or ZX sector adapter", s.Method)
	}
	l.requireFrontendRuntime()
	if s.Method == "gui_upload" {
		if s.Exporting["filename"] == nil {
			return nil, fmt.Errorf("GUI_UPLOAD: missing EXPORTING filename")
		}
		if e := s.Exporting["filetype"]; e != nil {
			lit, ok := e.(*StringLit)
			if !ok || lit.Val != "ASC" {
				return nil, fmt.Errorf("GUI_UPLOAD filetype: only literal 'ASC' is admitted")
			}
		}
		file, err := l.frontendText(s, "filename", "")
		if err != nil {
			return nil, err
		}
		t, kind, err := l.frontendTable(s, "data_tab")
		if err != nil {
			return nil, err
		}
		args := []hir.Expr{file.ptr, textValueLength(file)}
		args = append(args, tableFrontendArgs(t, kind)...)
		openError := s.Exceptions["file_open_error"]
		if openError == 0 {
			// OTHERS handles any supported exception without an explicit map.
			openError = s.Exceptions["others"]
		}
		args = append(args, i32Lit(openError), i32Lit(s.Exceptions["others"]))
		return charCall("_abap_fs_upload", args...), nil
	}
	t, kind, err := l.frontendTable(s, "file_table")
	if err != nil {
		return nil, err
	}
	filterName, directoryName := "filter", "directory"
	defaultFilter := "*.*"
	if s.Method == "file_open_dialog" {
		filterName, directoryName = "file_filter", "initial_directory"
		if ext := s.Exporting["default_extension"]; ext != nil {
			lit, ok := ext.(*StringLit)
			if !ok || len(lit.Val) < 1 || len(lit.Val) > 3 || strings.ContainsAny(lit.Val, ".*? /\\:|") {
				return nil, fmt.Errorf("FILE_OPEN_DIALOG default_extension requires an ASCII extension literal of 1..3 characters")
			}
			if s.Exporting[filterName] == nil {
				defaultFilter = "*." + lit.Val
			}
		}
	}
	filter, err := l.frontendText(s, filterName, defaultFilter)
	if err != nil {
		return nil, err
	}
	directory, err := l.frontendText(s, directoryName, "")
	if err != nil {
		return nil, err
	}
	args := tableFrontendArgs(t, kind)
	args = append(args, filter.ptr, textValueLength(filter), directory.ptr, textValueLength(directory))
	if s.Method == "directory_list_files" {
		count, err := l.frontendInteger(s, "count")
		if err != nil {
			return nil, err
		}
		return &hir.AssignStmt{Target: count, Val: &hir.CallExpr{Fn: "_abap_fs_directory", Args: args, Ty: mir2.TyI32}}, nil
	}
	title, err := l.frontendText(s, "window_title", "")
	if err != nil {
		return nil, err
	}
	args = append(args, title.ptr, textValueLength(title))
	rc, err := l.frontendInteger(s, "rc")
	if err != nil {
		return nil, err
	}
	body := []hir.Stmt{&hir.AssignStmt{Target: rc, Val: &hir.CallExpr{Fn: "_abap_fs_dialog", Args: args, Ty: mir2.TyI32}}}
	if s.Changing["user_action"] != nil {
		action, err := l.frontendInteger(s, "user_action")
		if err != nil {
			return nil, err
		}
		body = append(body, &hir.IfStmt{Cond: &hir.BinExpr{Op: ">", L: rc, R: i32Lit(0), Ty: mir2.TyBool}, Then: &hir.Block{Body: []hir.Stmt{&hir.AssignStmt{Target: action, Val: i32Lit(0)}}}, Else: &hir.Block{Body: []hir.Stmt{&hir.AssignStmt{Target: action, Val: i32Lit(1)}}}})
	}
	return &hir.Block{Body: body}, nil
}

func (l *lowerer) requireFrontendRuntime() {
	capacity := l.stringCapacity
	if capacity < 1024 {
		capacity = 1024
	}
	for _, b := range l.characters {
		if n, ok := b.length.(*hir.IntLitExpr); ok && n.Val > int64(capacity) {
			capacity = int(n.Val)
		}
	}
	// GUI_UPLOAD can target a table without a declared line work area. Its
	// maximum row width still bounds writes into the shared line buffer.
	for _, table := range l.tables {
		if table.line.size > capacity {
			capacity = table.line.size
		}
	}
	for i, g := range l.hm.Globals {
		if g.Name == "_abap_fs_work_buffer" {
			if a := g.Ty.(*mir2.ArrayTy); a.Len < capacity {
				l.hm.Globals[i].Ty = mir2.NewArray(mir2.TyU8, capacity)
			}
			return
		}
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{Name: "_abap_fs_work_buffer", Ty: mir2.NewArray(mir2.TyU8, capacity)})
}
