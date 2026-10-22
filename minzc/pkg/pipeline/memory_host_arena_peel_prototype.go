package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// peelMemoryHostArenaFirstByte is an isolated experiment, not a registered rule
// or optimization certificate. The caller must retain original input identity.
func peelMemoryHostArenaFirstByte(m *hir.Module, w hir.MemoryHostWorkload) error {
	if _, _, e := hir.PrepareMemoryHostWorkload(m, w); e != nil {
		return e
	}
	if w.Hosts["peek"] != hir.MemoryHostReadU8 {
		return fmt.Errorf("closed peek descriptor required")
	}
	f := m.FuncByName("region_match")
	if f == nil {
		return fmt.Errorf("missing region_match")
	}
	for _, g := range m.Globals {
		if g.Name == "i" {
			return fmt.Errorf("local/global collision")
		}
	}
	// Parse this independent exact-shape template rather than accepting merely a
	// matching function name. No purity or nowrap assumption is introduced.
	expected := &hir.Func{Name: "region_match", Params: []hir.Param{{Name: "arena_p", Ty: mir2.TyU16}, {Name: "src_start", Ty: mir2.TyU16}, {Name: "len", Ty: mir2.TyU8}}, RetTy: mir2.TyU8}
	// Source templates retain the parser's exact literal metadata.
	literal := func(n int64) hir.Expr { return &hir.IntLitExpr{Val: n, Ty: mir2.TyU8, Untyped: true} }
	v := hir.Var
	peek := func(x hir.Expr) hir.Expr { return &hir.CallExpr{Fn: "peek", Args: []hir.Expr{x}, Ty: mir2.TyU8} }
	left := hir.Add(v("arena_p", mir2.TyU16), literal(1), mir2.TyU16)
	right := hir.Add(v("src_buf", mir2.TyU16), v("src_start", mir2.TyU16), mir2.TyU16)
	i, n := v("i", mir2.TyU8), v("len", mir2.TyU8)
	expected.Body = hir.Blk(hir.Decl("i", mir2.TyU8, literal(0)), hir.While(hir.Lt(i, n), hir.Blk(hir.If(hir.Ne(peek(hir.Add(left, i, mir2.TyU16)), peek(hir.Add(right, i, mir2.TyU16))), hir.Blk(&hir.ReturnStmt{Val: literal(0)}), nil), hir.Assign(i, hir.Add(i, literal(1), mir2.TyU8)))), &hir.ReturnStmt{Val: literal(1)})
	if !reflect.DeepEqual(f, expected) {
		return fmt.Errorf("region_match exact source shape refused")
	}
	// Generated prefix constants remain typed, as required by the proof.
	left = hir.Add(v("arena_p", mir2.TyU16), hir.U8(1), mir2.TyU16)
	loop := f.Body.Body[1].(*hir.WhileStmt)
	// Preserve the original tail node and its attached source origin. New prefix
	// nodes have no manufactured source coordinates or loop observations.
	declaration := f.Body.Body[0].(*hir.VarDeclStmt)
	declaration.Init = hir.U8(1)
	f.Body = hir.Blk(hir.If(hir.Eq(n, hir.U8(0)), hir.Blk(&hir.ReturnStmt{Val: hir.U8(1)}), nil), hir.If(hir.Ne(peek(left), peek(right)), hir.Blk(&hir.ReturnStmt{Val: hir.U8(0)}), nil), hir.If(hir.Eq(n, hir.U8(1)), hir.Blk(&hir.ReturnStmt{Val: hir.U8(1)}), nil), declaration, loop, f.Body.Body[2])
	return nil
}
