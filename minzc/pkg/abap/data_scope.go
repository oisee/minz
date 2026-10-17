package abap

import (
	"fmt"
	"sort"
)

func validateDataStartValue(d *DataDecl) error {
	if d.Value == nil {
		return nil
	}
	var literal func(Expr_) bool
	literal = func(e Expr_) bool {
		switch v := e.(type) {
		case *IntLit, *StringLit:
			return true
		case *UnaryOp:
			return (v.Op == "+" || v.Op == "-") && literal(v.Val)
		}
		return false
	}
	if !literal(*d.Value) {
		return fmt.Errorf("DATA %s: VALUE requires a supported literal start value; named constants are not implemented", d.Name)
	}
	return nil
}

func walkDataDeclarations(stmts []Stmt_, visit func(*DataDecl) error) error {
	for _, s := range stmts {
		var groups [][]Stmt_
		switch v := s.(type) {
		case *DataDecl:
			if err := visit(v); err != nil {
				return err
			}
		case *IfStmt:
			groups = append(groups, v.Then, v.Else)
			for _, e := range v.ElseIf {
				groups = append(groups, e.Body)
			}
		case *DoStmt:
			groups = append(groups, v.Body)
		case *WhileStmt:
			groups = append(groups, v.Body)
		case *SelectStmt:
			groups = append(groups, v.Body)
		case *LoopStmt:
			groups = append(groups, v.Body)
		case *CaseStmt:
			groups = append(groups, v.Others)
			for _, w := range v.Whens {
				groups = append(groups, w.Body)
			}
		}
		for _, g := range groups {
			if err := walkDataDeclarations(g, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// DATA in regular report event/control-flow blocks belongs to report storage.
// AT-SELECTION-SCREEN is a procedure and keeps its own local DATA.
// FORM/method DATA is collected separately, once per invocation.
func (l *lowerer) collectReportData() error {
	names := map[string]bool{}
	for _, g := range l.hm.Globals {
		names[g.Name] = true
	}
	visit := func(d *DataDecl) error {
		if l.reportData[d] {
			return nil
		}
		if names[d.Name] {
			return fmt.Errorf("DATA %s: duplicate report object", d.Name)
		}
		if err := validateDataStartValue(d); err != nil {
			return err
		}
		names[d.Name], l.reportData[d] = true, true
		if !l.aggregateDecl(d) {
			l.varTypes[d.Name] = l.abapTypeToHIR(d.AbapTy, d.Length)
		}
		return l.lowerGlobal(d)
	}
	for _, d := range l.prog.Decls {
		var ss []Stmt_
		switch v := d.(type) {
		case *EventDecl:
			if v.Event != "AT-SELECTION-SCREEN" {
				ss = v.Body
			}
		case *formBodyDecl:
			ss = []Stmt_{v.stmt}
		}
		if err := walkDataDeclarations(ss, visit); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(l.prog.Events))
	for k := range l.prog.Events {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == "AT-SELECTION-SCREEN" {
			continue
		}
		if err := walkDataDeclarations(l.prog.Events[k], visit); err != nil {
			return err
		}
	}
	return nil
}

func (l *lowerer) localObjectName(name string) string {
	if object, ok := l.localObjects[name]; ok {
		return object
	}
	return name
}
