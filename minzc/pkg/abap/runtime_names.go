package abap

import (
	"fmt"
	"strings"
)

// Stable runtime ABI names are reserved: otherwise a user object can replace
// a generated helper or be intercepted by a VM host with the same name.
func reservedABAPRuntimeName(name string) bool {
	name = strings.ToLower(name)
	for _, prefix := range []string{"_abap_", "_itab_", "_zx_", "abap_", "sy_get_", "sy_set_", "sel_", "sqlite_"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
func validateABAPRuntimeNames(p *Program) error {
	check := func(name string) error {
		if reservedABAPRuntimeName(name) {
			return fmt.Errorf("ABAP object %q uses the reserved compiler runtime namespace", name)
		}
		return nil
	}
	visit := func(d *DataDecl) error { return check(d.Name) }
	for _, decl := range p.Decls {
		var body []Stmt_
		switch d := decl.(type) {
		case *DataDecl:
			if err := check(d.Name); err != nil {
				return err
			}
		case *ParamDecl:
			if err := check(d.Name); err != nil {
				return err
			}
		case *FormDecl:
			if err := check(d.Name); err != nil {
				return err
			}
			body = d.Body
		case *FunctionDecl:
			if err := check(d.Name); err != nil {
				return err
			}
			body = d.Body
		case *ClassDecl:
			for _, a := range d.Attrs {
				if err := check(a.Name); err != nil {
					return err
				}
			}
			for _, m := range d.Methods {
				if err := check(d.Name + "_" + m.Name); err != nil {
					return err
				}
				if err := walkDataDeclarations(m.Body, visit); err != nil {
					return err
				}
			}
		case *EventDecl:
			body = d.Body
		case *formBodyDecl:
			body = []Stmt_{d.stmt}
		}
		if err := walkDataDeclarations(body, visit); err != nil {
			return err
		}
	}
	for _, param := range p.Params {
		if err := check(param.Name); err != nil {
			return err
		}
	}
	for _, body := range p.Events {
		if err := walkDataDeclarations(body, visit); err != nil {
			return err
		}
	}
	return nil
}
