package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

const appendSourceBase = "__append_src_base"
const appendDestinationBase = "__append_dst_base"

// hoistMemoryHostAppendBases is a private experiment with closed byte hosts.
// It preserves resource effects, but intentionally removes repeated global reads.
func hoistMemoryHostAppendBases(m *hir.Module, w hir.MemoryHostWorkload) error {
	if _, _, e := hir.PrepareMemoryHostWorkload(m, w); e != nil {
		return e
	}
	if w.Hosts["peek"] != hir.MemoryHostReadU8 || w.Hosts["poke"] != hir.MemoryHostWriteU8 {
		return fmt.Errorf("closed peek/poke descriptors required")
	}
	// Reserve names across the full typed module, including nested declarations.
	var names func(reflect.Value) bool
	names = func(x reflect.Value) bool {
		if !x.IsValid() {
			return false
		}
		if x.Kind() == reflect.Interface || x.Kind() == reflect.Pointer {
			if x.IsNil() {
				return false
			}
			return names(x.Elem())
		}
		switch x.Kind() {
		case reflect.Struct:
			for i := 0; i < x.NumField(); i++ {
				if x.Type().Field(i).Name == "Name" && x.Field(i).Kind() == reflect.String {
					n := x.Field(i).String()
					if n == appendSourceBase || n == appendDestinationBase {
						return true
					}
				}
				if names(x.Field(i)) {
					return true
				}
			}
		case reflect.Slice:
			for i := 0; i < x.Len(); i++ {
				if names(x.Index(i)) {
					return true
				}
			}
		}
		return false
	}
	if names(reflect.ValueOf(m.Funcs)) || names(reflect.ValueOf(m.Globals)) {
		return fmt.Errorf("reserved append base name collision")
	}
	f := m.FuncByName("intern_region")
	if f == nil {
		return fmt.Errorf("missing intern_region")
	}
	// Source templates retain the parser's exact literal metadata.
	literal := func(n int64) hir.Expr { return &hir.IntLitExpr{Val: n, Ty: mir2.TyU8, Untyped: true} }
	v := hir.Var
	add := func(a, b hir.Expr) hir.Expr { return hir.Add(a, b, mir2.TyU16) }
	p := v("p", mir2.TyU16)
	n := v("len", mir2.TyU8)
	start := v("start", mir2.TyU16)
	j := v("j", mir2.TyU8)
	slen := v("slen", mir2.TyU8)
	bump := v("str_bump", mir2.TyU16)
	peek := func(a hir.Expr) hir.Expr { return hir.Call("peek", mir2.TyU8, a) }
	poke := func(a, b hir.Expr) hir.Stmt { return &hir.ExprStmt{Expr: hir.Call("poke", mir2.TyVoid, a, b)} }
	expected := &hir.Func{Name: "intern_region", Params: []hir.Param{{Name: "start", Ty: mir2.TyU16}, {Name: "len", Ty: mir2.TyU8}}, RetTy: mir2.TyU16}
	expected.Body = hir.Blk(hir.Decl("p", mir2.TyU16, v("str_arena", mir2.TyU16)), hir.While(hir.Lt(p, bump), hir.Blk(hir.Decl("slen", mir2.TyU8, peek(p)), hir.If(hir.Eq(slen, n), hir.Blk(hir.If(hir.Eq(hir.Call("region_match", mir2.TyU8, p, start, n), literal(1)), hir.Blk(&hir.ReturnStmt{Val: p}), nil)), nil), hir.Assign(p, add(add(p, literal(1)), slen)))), hir.Decl("new_p", mir2.TyU16, bump), poke(bump, n), hir.Decl("j", mir2.TyU8, literal(0)), hir.While(hir.Lt(j, n), hir.Blk(poke(add(add(bump, literal(1)), j), peek(add(add(v("src_buf", mir2.TyU16), start), j))), hir.Assign(j, hir.Add(j, literal(1), mir2.TyU8)))), hir.Assign(bump, add(add(bump, literal(1)), n)), &hir.ReturnStmt{Val: v("new_p", mir2.TyU16)})
	if !reflect.DeepEqual(f, expected) {
		return fmt.Errorf("intern_region exact source shape refused")
	}
	loop := f.Body.Body[5].(*hir.WhileStmt)
	// Retain the original loop and statement nodes and their source coordinates.
	call := loop.Body.Body[0].(*hir.ExprStmt).Expr.(*hir.CallExpr)
	read := call.Args[1].(*hir.CallExpr)
	call.Args[0] = add(v(appendDestinationBase, mir2.TyU16), j)
	read.Args[0] = add(v(appendSourceBase, mir2.TyU16), j)
	body := f.Body.Body
	next := append([]hir.Stmt{}, body[:4]...)
	next = append(next, hir.Decl(appendSourceBase, mir2.TyU16, add(v("src_buf", mir2.TyU16), start)), hir.Decl(appendDestinationBase, mir2.TyU16, add(bump, hir.U8(1))))
	f.Body.Body = append(next, body[4:]...)
	return nil
}
