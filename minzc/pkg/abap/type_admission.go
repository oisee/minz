package abap

import (
	"fmt"
	"strings"
)

// Validate before generating any runtime or ABI. Unknown and approximate
// numeric mappings must never become executable code. Character storage is
// hardened separately; admission here does not certify its copying semantics.
func validateProgramTypes(p *Program) error {
	// Report-level TYPES BEGIN OF structures are the only named types.
	structures := map[string]bool{}
	aliases := map[string]*TypesDecl{}
	for _, decl := range p.Decls {
		if td, ok := decl.(*TypesDecl); ok {
			if structures[td.Name] {
				return fmt.Errorf("TYPES %s: duplicate type", td.Name)
			}
			structures[td.Name] = true
			if td.Elementary != "" {
				aliases[td.Name] = td
			}
		}
	}
	check := func(context, ty string, length int) error {
		switch ty {
		case "i", "string":
			if length != 0 {
				return fmt.Errorf("%s: LENGTH is not admitted for TYPE %s", context, ty)
			}
		case "c":
			if length < 0 || length > 65535 {
				return fmt.Errorf("%s: character LENGTH %d is outside 1..65535", context, length)
			}
		default:
			return fmt.Errorf("%s: TYPE %s is not implemented precisely; approximate storage is not admitted", context, ty)
		}
		return nil
	}
	// DATA admits a flat report structure type and standard tables of one.
	data := func(d *DataDecl) error {
		context := "DATA " + d.Name
		if d.SalvRef {
			if d.AbapTy != "cl_salv_table" || d.Table != nil || d.Length != 0 || d.Value != nil {
				return fmt.Errorf("%s: unsupported SALV reference declaration", context)
			}
			return nil
		}
		if d.Table != nil {
			if !structures[d.AbapTy] && d.AbapTy != "c" && d.AbapTy != "string" {
				return fmt.Errorf("%s: table line type %s is not a flat structure declared with TYPES BEGIN OF; tables of elementary or unknown line types are not implemented", context, d.AbapTy)
			}
			return nil
		}
		if alias := aliases[d.AbapTy]; alias != nil {
			return check(context, alias.Elementary, alias.Length)
		}
		if structures[d.AbapTy] {
			if d.Length != 0 || d.Value != nil {
				return fmt.Errorf("%s: structure LENGTH/VALUE additions are not implemented", context)
			}
			return nil
		}
		return check(context, d.AbapTy, d.Length)
	}
	params := func(context string, ps []FormParam) error {
		for _, v := range ps {

			if !strings.HasPrefix(context, "FORM ") && (v.AbapTy == "c" || v.AbapTy == "string") {
				return fmt.Errorf("%s parameter %s: character ABI is not implemented", context, v.Name)
			}
			if err := check(context+" parameter "+v.Name, v.AbapTy, 0); err != nil {
				return err
			}
		}
		return nil
	}
	var body func([]Stmt_) error
	body = func(ss []Stmt_) error {
		for _, s := range ss {
			var groups [][]Stmt_
			switch v := s.(type) {
			case *DataDecl:
				if err := data(v); err != nil {
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
			case *LoopStmt:
				groups = append(groups, v.Body)
			case *SelectStmt:
				groups = append(groups, v.Body)
			case *CaseStmt:
				groups = append(groups, v.Others)
				for _, w := range v.Whens {
					groups = append(groups, w.Body)
				}
			}
			for _, g := range groups {
				if err := body(g); err != nil {
					return err
				}
			}
		}
		return nil
	}
	method := func(context string, m *MethodDecl) error {
		if len(m.Exporting) != 0 || len(m.Changing) != 0 {
			return fmt.Errorf("%s: EXPORTING/CHANGING method ABI is not implemented", context)
		}
		for _, ps := range [][]FormParam{m.Importing, m.Exporting, m.Changing} {
			if err := params(context, ps); err != nil {
				return err
			}
		}
		if m.Returning != nil {
			if err := params(context, []FormParam{*m.Returning}); err != nil {
				return err
			}
		}
		return body(m.Body)
	}
	for _, decl := range p.Decls {
		switch v := decl.(type) {
		case *DataDecl:
			if err := data(v); err != nil {
				return err
			}
		case *ParamDecl:
			if v.AbapTy == "string" {
				return fmt.Errorf("PARAMETERS %s: dynamic string selection fields are not implemented", v.Name)
			}
			if err := check("PARAMETERS "+v.Name, v.AbapTy, v.Length); err != nil {
				return err
			}
		case *FormDecl:
			if err := params("FORM "+v.Name, v.Using); err != nil {
				return err
			}
			if err := params("FORM "+v.Name, v.Changing); err != nil {
				return err
			}
			if err := body(v.Body); err != nil {
				return err
			}
		case *FunctionDecl:
			if len(v.Changing) != 0 {
				return fmt.Errorf("FUNCTION %s: CHANGING reference ABI is not implemented", v.Name)
			}
			if len(v.Exporting) > 1 {
				return fmt.Errorf("FUNCTION %s: multiple EXPORTING results are not implemented", v.Name)
			}
			for _, ps := range [][]FormParam{v.Importing, v.Exporting, v.Changing} {
				if err := params("FUNCTION "+v.Name, ps); err != nil {
					return err
				}
			}
			if err := body(v.Body); err != nil {
				return err
			}
		case *ClassDecl:
			if v.Super != "" || len(v.Interfaces) != 0 {
				return fmt.Errorf("CLASS %s: inheritance/interface dispatch is not implemented", v.Name)
			}
			for _, a := range v.Attrs {
				if a.Value != nil {
					return fmt.Errorf("CLASS %s attribute %s: VALUE initialization is not implemented", v.Name, a.Name)
				}
				if a.Table != nil || structures[a.AbapTy] {
					return fmt.Errorf("CLASS %s attribute %s: structure and table attributes are not implemented", v.Name, a.Name)
				}
				if a.AbapTy == "c" || a.AbapTy == "string" {
					return fmt.Errorf("CLASS %s attribute %s: owned character attributes are not implemented", v.Name, a.Name)
				}
				if err := check("CLASS "+v.Name+" attribute "+a.Name, a.AbapTy, a.Length); err != nil {
					return err
				}
			}
			for _, m := range v.Methods {
				if err := method("METHOD "+v.Name+"=>"+m.Name, m); err != nil {
					return err
				}
			}
		case *EventDecl:
			if err := body(v.Body); err != nil {
				return err
			}
		case *formBodyDecl:
			if err := body([]Stmt_{v.stmt}); err != nil {
				return err
			}
		}
	}
	for _, v := range p.Params {
		if v.AbapTy == "string" {
			return fmt.Errorf("PARAMETERS %s: dynamic string selection fields are not implemented", v.Name)
		}
		if err := check("PARAMETERS "+v.Name, v.AbapTy, v.Length); err != nil {
			return err
		}
	}
	for _, ss := range p.Events {
		if err := body(ss); err != nil {
			return err
		}
	}
	return nil
}
