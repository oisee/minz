package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"strings"
)

// Generated runtimes are parsed independently of the ABAP report, then linked
// into its HIR module. Give their mutable locals and parameters private names
// before linking/lowering: a user global with the same spelling must not capture
// assignments to a runtime local. Runtime global symbols and call ABIs stay put.
func namespaceABAPRuntimeBindings(hm *hir.Module) {
	for _, f := range hm.Funcs {
		if !f.Internal || !reservedABAPRuntimeName(f.Name) || strings.HasPrefix(f.Name, "_abap_event_") {
			continue
		}
		names := map[string]string{}
		add := func(name string) { names[name] = "_abap_private_" + f.Name + "_" + name }
		for _, p := range f.Params {
			add(p.Name)
		}
		walkRuntimeHIR(f.Body, func(n any) {
			switch n := n.(type) {
			case *hir.VarDeclStmt:
				add(n.Name)
			case *hir.ForRangeStmt:
				add(n.Var)
			case *hir.ForEachStmt:
				add(n.Var)
			case *hir.LetInExpr:
				add(n.Name)
			}
		})
		for i := range f.Params {
			f.Params[i].Name = names[f.Params[i].Name]
		}
		walkRuntimeHIR(f.Body, func(n any) {
			switch n := n.(type) {
			case *hir.AsmStmt:
				for i := range n.Ins {
					if name, ok := names[n.Ins[i].Name]; ok {
						n.Ins[i].Name = name
					}
				}
				for i := range n.Outs {
					if name, ok := names[n.Outs[i].Name]; ok {
						n.Outs[i].Name = name
					}
				}
			case *hir.VarRefExpr:
				if name, ok := names[n.Name]; ok {
					n.Name = name
				}
			case *hir.VarDeclStmt:
				n.Name = names[n.Name]
			case *hir.ForRangeStmt:
				n.Var = names[n.Var]
			case *hir.ForEachStmt:
				n.Var = names[n.Var]
			case *hir.LetInExpr:
				n.Name = names[n.Name]
			}
		})
	}
}

func walkRuntimeHIR(body *hir.Block, visit func(any)) {
	seen := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.CanInterface() {
			if _, ty := v.Interface().(mir2.Ty); ty {
				return
			}
		}
		if v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			if seen[v.Pointer()] {
				return
			}
			seen[v.Pointer()] = true
			if v.CanInterface() {
				visit(v.Interface())
			}
			walk(v.Elem())
			return
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i))
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(body))
}
