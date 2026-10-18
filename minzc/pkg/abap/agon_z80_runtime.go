package abap

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// BindAgonZ80Runtime selects compiler-owned platform leaves before lowering.
// It compares complete freshly generated HIR bodies before pruning any opaque
// built-in ASM. Names or Internal markers alone grant no authority.
func BindAgonZ80Runtime(m *hir.Module) error {
	if m == nil || m.Target != hir.TargetAgon {
		return fmt.Errorf("agon-z80 ABAP runtime requires platform Agon")
	}
	canonical := &hir.Module{Target: hir.TargetAgon}
	emitRuntimeFuncs(canonical, hir.TargetAgon)
	if err := emitInteger32Runtime(canonical, hir.TargetAgon); err != nil {
		return err
	}
	known := map[string]bool{}
	for _, f := range m.Funcs {
		if want := canonical.FuncByName(f.Name); want != nil {
			if !reflect.DeepEqual(f, want) {
				return fmt.Errorf("agon-z80 rejects modified ABAP runtime body %s", f.Name)
			}
			known[f.Name] = true
		}
	}
	// Validate every noncanonical function, including unreachable opaque ASM.
	for _, f := range m.Funcs {
		if known[f.Name] {
			continue
		}
		var opaque bool
		walkAgonHIR(reflect.ValueOf(f.Body), func(v any) {
			switch v.(type) {
			case *hir.AsmStmt, *hir.CallIndirectExpr:
				opaque = true
			}
		})
		if opaque {
			return fmt.Errorf("agon-z80 rejects opaque ASM/indirect calls in %s", f.Name)
		}
	}
	live := map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if live[name] || strings.HasPrefix(name, "@") {
			return nil
		}
		f := m.FuncByName(name)
		if f == nil {
			return fmt.Errorf("agon-z80 unresolved ABAP call %s", name)
		}
		live[name] = true
		if name == "_abap_num_putchar" && known[name] {
			f.Body = &hir.Block{Body: []hir.Stmt{&hir.ExprStmt{Expr: &hir.CallExpr{
				Fn: "@mir.io.print.char", Args: []hir.Expr{&hir.VarRefExpr{Name: "ch", Ty: mir2.TyU8}}, Ty: mir2.TyVoid,
			}}}}
			return nil
		}
		var calls []string
		var opaque bool
		walkAgonHIR(reflect.ValueOf(f.Body), func(v any) {
			switch n := v.(type) {
			case *hir.AsmStmt, *hir.CallIndirectExpr:
				opaque = true
			case *hir.CallExpr:
				calls = append(calls, n.Fn)
			}
		})
		if opaque {
			return fmt.Errorf("agon-z80 ABAP leaf %s is not supported (console only)", name)
		}
		for _, call := range calls {
			if err := visit(call); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit("main"); err != nil {
		return err
	}
	funcs := m.Funcs[:0]
	for _, f := range m.Funcs {
		if live[f.Name] {
			funcs = append(funcs, f)
		}
	}
	m.Funcs = funcs
	return nil
}

// Only descend into HIR syntax, not type metadata or module/source maps.
func walkAgonHIR(v reflect.Value, visit func(any)) {
	if !v.IsValid() {
		return
	}
	if v.CanInterface() {
		visit(v.Interface())
	}
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			walkAgonHIR(v.Elem(), visit)
		}
	case reflect.Pointer:
		if !v.IsNil() && v.Elem().Kind() == reflect.Struct && v.Elem().Type().PkgPath() == "github.com/minz/minzc/pkg/hir" {
			walkAgonHIR(v.Elem(), visit)
		}
	case reflect.Struct:
		if v.Type().PkgPath() == "github.com/minz/minzc/pkg/hir" {
			for i := 0; i < v.NumField(); i++ {
				walkAgonHIR(v.Field(i), visit)
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			walkAgonHIR(v.Index(i), visit)
		}
	}
}
