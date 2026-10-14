package hir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// smcAccessors describes compiler-owned setters without changing the input HIR.
// The first admission supports one pointer/legacy u16 slot, direct calls and
// scalar/void returns. More slots require loads near their uses, not entry spills.
func smcAccessors(m *Module) ([]*Func, error) {
	names := map[string]bool{}
	for _, f := range m.Funcs {
		if f != nil {
			names[f.Name] = true
		}
	}
	var setters []*Func
	for _, f := range m.Funcs {
		if f == nil {
			continue
		}
		count := 0
		for _, p := range f.Params {
			if !p.SMC {
				continue
			}
			count++
			if f.IsExtern || len(f.RetTys) != 0 || len(f.Captures) != 0 {
				return nil, fmt.Errorf("HIR %s: SMC requires a local scalar/void function", f.Name)
			}
			switch mir2.BaseOf(f.RetTy).(type) {
			case *mir2.StructTy, *mir2.ArrayTy:
				return nil, fmt.Errorf("HIR %s: SMC aggregate returns are unsupported", f.Name)
			}
			if p.Ty == nil || (p.Ty != mir2.TyU16 && !mir2.IsPointer(p.Ty)) {
				return nil, fmt.Errorf("HIR %s: SMC parameter %s requires a pointer or u16", f.Name, p.Name)
			}
			name := mir2.SMCSetterName(f.Name + "$" + p.Name)
			if names[name] {
				return nil, fmt.Errorf("HIR %s: reserved SMC setter %s", f.Name, name)
			}
			names[name] = true
			setters = append(setters, &Func{Name: name, Params: []Param{{Name: "value", Ty: p.Ty, RegClass: mir2.ClassPointer}}, RetTy: mir2.TyVoid, IsExtern: true, IsIO: true})
		}
		if count > 1 {
			return nil, fmt.Errorf("HIR %s: first SMC admission supports one address slot per function", f.Name)
		}
	}
	return setters, nil
}

func hasSMCParams(f *Func) bool {
	for _, p := range f.Params {
		if p.SMC {
			return true
		}
	}
	return false
}

// Full source calls bind slots before invocation. Reduced calls reuse slots.
func smcCallParams(f *Func, argc int) []Param {
	if argc == len(f.Params) || !hasSMCParams(f) {
		return f.Params
	}
	var runtime []Param
	for _, p := range f.Params {
		if !p.SMC {
			runtime = append(runtime, p)
		}
	}
	if argc == len(runtime) {
		return runtime
	}
	return f.Params
}

// Run after structural verification, so walking HIR cannot encounter cycles.
func verifySMCRecursion(m *Module) error {
	anySMC := false
	for _, f := range m.Funcs {
		if hasSMCParams(f) {
			anySMC = true
			break
		}
	}
	if !anySMC {
		return nil
	}
	edges := map[string][]string{}
	for _, f := range m.Funcs {
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for _, s := range b.Body {
				visitStmtExprs(s, func(e Expr) {
					if c, ok := e.(*CallExpr); ok {
						edges[f.Name] = append(edges[f.Name], c.Fn)
					}
				})
			}
		})
	}
	for _, f := range m.Funcs {
		if !hasSMCParams(f) {
			continue
		}
		seen := map[string]bool{}
		var reaches func(string) bool
		reaches = func(name string) bool {
			if seen[name] {
				return false
			}
			seen[name] = true
			for _, next := range edges[name] {
				if next == f.Name || reaches(next) {
					return true
				}
			}
			return false
		}
		if reaches(f.Name) {
			return fmt.Errorf("HIR %s: recursive SMC activation is unsupported", f.Name)
		}
	}
	return nil
}
