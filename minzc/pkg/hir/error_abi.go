package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
)

// ResolveErrorABI derives language-domain CY ABI, including non-? intrinsics.
// This initial ABI admits defined u8/void functions and immediate statement
// checks. Nested fallible expressions require explicit outcome threading.
func ResolveErrorABI(m *Module) error {
	funcs := map[string]*Func{}
	for _, f := range m.Funcs {
		funcs[f.Name] = f
		f.FallibleCY = strings.HasSuffix(f.Name, "?") || f.FallibleCY
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for _, st := range b.Body {
				visitStmtExprs(st, func(e Expr) {
					if c, ok := e.(*CallExpr); ok && (c.Fn == "@error" || c.Fn == "@propagate" || c.Fn == "@check") {
						f.FallibleCY = true
					}
				})
			}
		})
	}
	for changed := true; changed; {
		changed = false
		for _, f := range m.Funcs {
			walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
				for _, st := range b.Body {
					visitStmtExprs(st, func(e Expr) {
						if c, ok := e.(*CallExpr); ok {
							if callee := funcs[c.Fn]; callee != nil && callee.FallibleCY && !f.FallibleCY {
								f.FallibleCY = true
								changed = true
							}
						}
					})
				}
			})
		}
	}
	for _, f := range m.Funcs {
		if f.FallibleCY && (f.IsExtern || len(f.RetTys) > 0 || f.RetTy != mir2.TyVoid && f.RetTy != mir2.TyU8) {
			return fmt.Errorf("CY error ABI currently requires defined u8 or void function: %s", f.Name)
		}
		var validation error
		walkGraceBlocks(f.Body, "", func(_ string, b *Block) {
			for i, st := range b.Body {
				var root Expr
				switch x := st.(type) {
				case *ExprStmt:
					root = x.Expr
				case *VarDeclStmt:
					root = x.Init
				case *AssignStmt:
					root = x.Val
				}
				preceding := false
				if c, ok := root.(*CallExpr); ok {
					if c.Fn == "@propagate" || c.Fn == "@check" {
						if i > 0 {
							var prev Expr
							switch x := b.Body[i-1].(type) {
							case *ExprStmt:
								prev = x.Expr
							case *VarDeclStmt:
								prev = x.Init
							case *AssignStmt:
								prev = x.Val
							}
							if pc, ok := prev.(*CallExpr); ok {
								preceding = funcs[pc.Fn] != nil && funcs[pc.Fn].FallibleCY
							}
						}
						if !preceding {
							validation = fmt.Errorf("%s: %s requires immediately preceding fallible direct call", f.Name, c.Fn)
						}
					}
				}
				visitStmtExprs(st, func(e Expr) {
					c, ok := e.(*CallExpr)
					if !ok {
						return
					}
					if c.Fn == "@error" || c.Fn == "@check" || c.Fn == "@propagate" {
						if c != root {
							validation = fmt.Errorf("%s: error intrinsic requires standalone statement", f.Name)
						}
						if c.Fn == "@error" && len(c.Args) != 1 {
							validation = fmt.Errorf("%s: @error requires one code", f.Name)
						}
						return
					}
					callee := funcs[c.Fn]
					if callee == nil || !callee.FallibleCY {
						return
					}
					checked := false
					if i+1 < len(b.Body) {
						if next, ok := b.Body[i+1].(*ExprStmt); ok {
							if nc, ok := next.Expr.(*CallExpr); ok {
								checked = nc.Fn == "@check" || nc.Fn == "@propagate"
							}
						}
					}
					if c != root || !checked {
						validation = fmt.Errorf("%s: fallible call %s requires immediate statement @check/@propagate", f.Name, c.Fn)
					}
				})
			}
		})
		if validation != nil {
			return validation
		}
	}
	return nil
}

// RejectValueOnlyErrorProfiles prevents existing value-only profile schemas
// from treating a language failure as a successful measured return.
func RejectValueOnlyErrorProfiles(m *Module) error {
	if m == nil {
		return fmt.Errorf("missing source")
	}
	if err := ResolveErrorABI(m); err != nil {
		return err
	}
	for _, f := range m.Funcs {
		if f.FallibleCY {
			return fmt.Errorf("value-only profile workload does not support CY language outcomes: %s", f.Name)
		}
	}
	return nil
}
