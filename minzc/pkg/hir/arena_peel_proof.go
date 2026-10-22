package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"strings"
)

// ValidateMemoryHostArenaPeel proves the exact isolated first-byte peeling
// before compiler splitting/lowering. It grants no optimization/default policy
// and no inherited input-loop counts; actual HIR compatibility differs.
func ValidateMemoryHostArenaPeel(source, actual *Module, w MemoryHostWorkload) error {
	fail := func(s string) error { return fmt.Errorf("arena peel proof: %s", s) }
	if source == nil || actual == nil {
		return fail("nil module")
	}
	if _, _, e := PrepareMemoryHostWorkload(source, w); e != nil {
		return e
	}
	if _, _, e := PrepareMemoryHostWorkload(actual, w); e != nil {
		return e
	}
	if w.Hosts["peek"] != MemoryHostReadU8 {
		return fail("closed read host required")
	}
	for _, m := range []*Module{source, actual} {
		if m.LoopLineage != nil || m.PointerWalkPolicy != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil || m.ForwardCopyPolicy != nil || m.RegionComparePolicy != nil || m.OrderedBytePolicy != nil || m.OrderedByteRWPolicy != nil {
			return fail("prior policy/lineage")
		}
		if _, e := BuildFunctionSiteMap(m); e != nil {
			return e
		}
	}
	for _, g := range source.Globals {
		if g.Name == "i" {
			return fail("local/global collision")
		}
	}
	f, g := source.FuncByName("region_match"), actual.FuncByName("region_match")
	if f == nil || g == nil {
		return fail("missing function")
	}
	expected := arenaPeelOriginalShape()
	if !reflect.DeepEqual(f, expected) {
		return fail("original whole function shape")
	}
	sf, af := *f, *g
	sf.Body = nil
	af.Body = nil
	if !reflect.DeepEqual(sf, af) {
		return fail("function metadata")
	}
	if g.Body == nil || len(g.Body.Body) != 6 {
		return fail("post cardinality")
	}
	v := Var
	i, n := v("i", mir2.TyU8), v("len", mir2.TyU8)
	_ = i
	peek := func(x Expr) Expr { return &CallExpr{Fn: "peek", Args: []Expr{x}, Ty: mir2.TyU8} }
	left := Add(v("arena_p", mir2.TyU16), U8(1), mir2.TyU16)
	right := Add(v("src_buf", mir2.TyU16), v("src_start", mir2.TyU16), mir2.TyU16)
	prefix := []Stmt{If(Eq(n, U8(0)), Blk(&ReturnStmt{Val: U8(1)}), nil), If(Ne(peek(left), peek(right)), Blk(&ReturnStmt{Val: U8(0)}), nil), If(Eq(n, U8(1)), Blk(&ReturnStmt{Val: U8(1)}), nil)}
	for j, s := range prefix {
		if !reflect.DeepEqual(g.Body.Body[j], s) {
			return fail("ordered prefix")
		}
	}
	d, ok := g.Body.Body[3].(*VarDeclStmt)
	if !ok {
		return fail("post declaration")
	}
	decl := *f.Body.Body[0].(*VarDeclStmt)
	decl.Init = U8(1)
	if !reflect.DeepEqual(d, &decl) {
		return fail("declaration delta")
	}
	if !reflect.DeepEqual(g.Body.Body[4], f.Body.Body[1]) || !reflect.DeepEqual(g.Body.Body[5], f.Body.Body[2]) {
		return fail("tail/final return")
	}
	x, y := *source, *actual
	x.SourceLocations = nil
	y.SourceLocations = nil
	x.Funcs = make([]*Func, len(source.Funcs))
	y.Funcs = make([]*Func, len(actual.Funcs))
	for j, h := range source.Funcs {
		c := *h
		if h.Name == f.Name {
			c.Body = nil
		}
		x.Funcs[j] = &c
	}
	for j, h := range actual.Funcs {
		c := *h
		if h.Name == f.Name {
			c.Body = nil
		}
		y.Funcs[j] = &c
	}
	if !reflect.DeepEqual(x, y) {
		return fail("unrelated whole module content")
	}
	sp, e := orderedByteStatements(source)
	if e != nil {
		return e
	}
	ap, e := orderedByteStatements(actual)
	if e != nil {
		return e
	}
	wanted := map[string]SourceLocation{}
	for p, s := range sp {
		loc, ok := source.SourceLocations[s]
		if !ok {
			continue
		}
		q := p
		for j, k := range []int{3, 4, 5} {
			root := fmt.Sprintf("region_match/body/s%d", j)
			if p == root || strings.HasPrefix(p, root+"/") {
				q = fmt.Sprintf("region_match/body/s%d", k) + strings.TrimPrefix(p, root)
				break
			}
		}
		wanted[q] = loc
	}
	if len(wanted) != len(source.SourceLocations) || len(actual.SourceLocations) != len(wanted) {
		return fail("origin cardinality")
	}
	for p, s := range ap {
		loc, has := actual.SourceLocations[s]
		want, need := wanted[p]
		if has != need || has && loc != want {
			return fail("exact origin path")
		}
	}
	for p := range wanted {
		if ap[p] == nil {
			return fail("missing mapped origin")
		}
	}
	return nil
}
func arenaPeelOriginalShape() *Func {
	v := Var
	i, n := v("i", mir2.TyU8), v("len", mir2.TyU8)
	peek := func(x Expr) Expr { return &CallExpr{Fn: "peek", Args: []Expr{x}, Ty: mir2.TyU8} }
	left := Add(v("arena_p", mir2.TyU16), sourceU8(1), mir2.TyU16)
	right := Add(v("src_buf", mir2.TyU16), v("src_start", mir2.TyU16), mir2.TyU16)
	return &Func{Name: "region_match", Params: []Param{{Name: "arena_p", Ty: mir2.TyU16}, {Name: "src_start", Ty: mir2.TyU16}, {Name: "len", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: Blk(Decl("i", mir2.TyU8, sourceU8(0)), While(Lt(i, n), Blk(If(Ne(peek(Add(left, i, mir2.TyU16)), peek(Add(right, i, mir2.TyU16))), Blk(&ReturnStmt{Val: sourceU8(0)}), nil), Assign(i, Add(i, sourceU8(1), mir2.TyU8)))), &ReturnStmt{Val: sourceU8(1)})}
}
