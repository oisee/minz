package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"strings"
)

// ValidateMemoryHostAppendCopyHoist validates independent immutable input and
// precompile snapshots. It preserves closed host byte events and final state,
// not compiler-global load counts, instrumentation gas, or a postcompile proof.
// Closed peek/poke regions cannot overlap VM/native globals: neither host can
// change src_buf or str_bump while the append loop runs. Both u16 base sums and
// their later additions are modulo 65536; no nowrap assumption is required.
func ValidateMemoryHostAppendCopyHoist(source, actual *Module, w MemoryHostWorkload) error {
	fail := func(s string) error { return fmt.Errorf("append copy proof: %s", s) }
	if source == nil || actual == nil {
		return fail("nil module")
	}
	if w.Hosts["peek"] != MemoryHostReadU8 || w.Hosts["poke"] != MemoryHostWriteU8 {
		return fail("closed byte hosts required")
	}
	for _, m := range []*Module{source, actual} {
		if _, _, e := PrepareMemoryHostWorkload(m, w); e != nil {
			return e
		}
		if m.LoopLineage != nil || m.PointerWalkPolicy != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil || m.ForwardCopyPolicy != nil || m.RegionComparePolicy != nil || m.OrderedBytePolicy != nil || m.OrderedByteRWPolicy != nil {
			return fail("prior policy/lineage")
		}
		if _, e := BuildFunctionSiteMap(m); e != nil {
			return e
		}
		// Prepare validates source hosts, but resource/module-storage exclusion
		// is a separate runner precondition. Establish it here without calls
		// or executing the workload; this is necessary for base invariance.
		raw := LowerModule(m)
		if e := mir2.Verify(raw); e != nil {
			return e
		}
		used := mir2.NewVM(raw).AllocHeap(nil).I
		for _, r := range w.Regions {
			if int64(r.Address) < used {
				return fail("resource overlaps VM module storage")
			}
		}
	}
	f, g := source.FuncByName("intern_region"), actual.FuncByName("intern_region")
	if f == nil || g == nil || !reflect.DeepEqual(f, appendCopyOriginalShape()) {
		return fail("original complete function")
	}
	sf, af := *f, *g
	sf.Body = nil
	af.Body = nil
	if !reflect.DeepEqual(sf, af) {
		return fail("function metadata")
	}
	// Global assignment lowering can prefer globals over locals. Refuse original
	// local/global collisions as well as any reserved generated binding.
	reserved := map[string]bool{"__append_src_base": true, "__append_dst_base": true}
	for _, h := range source.Funcs {
		if reserved[h.Name] {
			return fail("generated function collision")
		}
		for _, p := range append(append([]Param{}, h.Params...), h.Captures...) {
			if reserved[p.Name] {
				return fail("generated parameter collision")
			}
		}
	}
	paths, e := orderedByteStatements(source)
	if e != nil {
		return e
	}
	for _, s := range paths {
		if d, ok := s.(*VarDeclStmt); ok && reserved[d.Name] {
			return fail("generated declaration collision")
		}
	}
	for _, global := range source.Globals {
		if reserved[global.Name] || global.Name == "p" || global.Name == "j" {
			return fail("global binding collision")
		}
	}
	if g.Body == nil || len(g.Body.Body) != 10 {
		return fail("post cardinality")
	}
	for j := 0; j < 4; j++ {
		if !reflect.DeepEqual(f.Body.Body[j], g.Body.Body[j]) {
			return fail("search/header prefix")
		}
	}
	src := Add(Var("src_buf", mir2.TyU16), Var("start", mir2.TyU16), mir2.TyU16)
	dst := Add(Var("str_bump", mir2.TyU16), U8(1), mir2.TyU16)
	if !reflect.DeepEqual(g.Body.Body[4], Decl("__append_src_base", mir2.TyU16, src)) || !reflect.DeepEqual(g.Body.Body[5], Decl("__append_dst_base", mir2.TyU16, dst)) {
		return fail("ordered typed base declarations")
	}
	if !reflect.DeepEqual(f.Body.Body[4], g.Body.Body[6]) || !reflect.DeepEqual(f.Body.Body[6], g.Body.Body[8]) || !reflect.DeepEqual(f.Body.Body[7], g.Body.Body[9]) {
		return fail("counter/bump/return")
	}
	before := f.Body.Body[5].(*WhileStmt)
	after, ok := g.Body.Body[7].(*WhileStmt)
	if !ok || after.Body == nil || len(after.Body.Body) != 2 {
		return fail("append loop")
	}
	x, y := *before, *after
	x.Body = nil
	y.Body = nil
	if !reflect.DeepEqual(x, y) || !reflect.DeepEqual(before.Body.Body[1], after.Body.Body[1]) {
		return fail("loop control")
	}
	j := Var("j", mir2.TyU8)
	event := &ExprStmt{Expr: Call("poke", mir2.TyVoid, Add(Var("__append_dst_base", mir2.TyU16), j, mir2.TyU16), Call("peek", mir2.TyU8, Add(Var("__append_src_base", mir2.TyU16), j, mir2.TyU16)))}
	if !reflect.DeepEqual(after.Body.Body[0], event) {
		return fail("ordered byte operation")
	}
	// Mask only the independently validated function body, never other semantics.
	sm, am := *source, *actual
	sm.SourceLocations = nil
	am.SourceLocations = nil
	sm.Funcs = make([]*Func, len(source.Funcs))
	am.Funcs = make([]*Func, len(actual.Funcs))
	for k, h := range source.Funcs {
		c := *h
		if h.Name == f.Name {
			c.Body = nil
		}
		sm.Funcs[k] = &c
	}
	for k, h := range actual.Funcs {
		c := *h
		if h.Name == f.Name {
			c.Body = nil
		}
		am.Funcs[k] = &c
	}
	if !reflect.DeepEqual(sm, am) {
		return fail("unrelated whole module content")
	}
	actualPaths, e := orderedByteStatements(actual)
	if e != nil {
		return e
	}
	expected := map[string]SourceLocation{}
	for p, s := range paths {
		loc, has := source.SourceLocations[s]
		if !has {
			continue
		}
		q := p
		for k := 4; k < 8; k++ {
			root := fmt.Sprintf("intern_region/body/s%d", k)
			if p == root || strings.HasPrefix(p, root+"/") {
				q = fmt.Sprintf("intern_region/body/s%d", k+2) + strings.TrimPrefix(p, root)
				break
			}
		}
		expected[q] = loc
	}
	if len(expected) != len(source.SourceLocations) || len(actual.SourceLocations) != len(expected) {
		return fail("origin cardinality")
	}
	for p, s := range actualPaths {
		got, has := actual.SourceLocations[s]
		want, need := expected[p]
		if has != need || has && got != want {
			return fail("exact origin path")
		}
	}
	for p := range expected {
		if actualPaths[p] == nil {
			return fail("missing mapped origin")
		}
	}
	return nil
}

func appendCopyOriginalShape() *Func {
	v := Var
	add := func(a, b Expr) Expr { return Add(a, b, mir2.TyU16) }
	p := v("p", mir2.TyU16)
	n := v("len", mir2.TyU8)
	j := v("j", mir2.TyU8)
	start := v("start", mir2.TyU16)
	bump := v("str_bump", mir2.TyU16)
	peek := func(a Expr) Expr { return Call("peek", mir2.TyU8, a) }
	poke := func(a, b Expr) Stmt { return &ExprStmt{Expr: Call("poke", mir2.TyVoid, a, b)} }
	return &Func{Name: "intern_region", Params: []Param{{Name: "start", Ty: mir2.TyU16}, {Name: "len", Ty: mir2.TyU8}}, RetTy: mir2.TyU16, Body: Blk(
		Decl("p", mir2.TyU16, v("str_arena", mir2.TyU16)),
		While(Lt(p, bump), Blk(Decl("slen", mir2.TyU8, peek(p)), If(Eq(v("slen", mir2.TyU8), n), Blk(If(Eq(Call("region_match", mir2.TyU8, p, start, n), sourceU8(1)), Blk(Ret(p)), nil)), nil), Assign(p, add(add(p, sourceU8(1)), v("slen", mir2.TyU8))))),
		Decl("new_p", mir2.TyU16, bump), poke(bump, n), Decl("j", mir2.TyU8, sourceU8(0)),
		While(Lt(j, n), Blk(poke(add(add(bump, sourceU8(1)), j), peek(add(add(v("src_buf", mir2.TyU16), start), j))), Assign(j, Add(j, sourceU8(1), mir2.TyU8)))),
		Assign(bump, add(add(bump, sourceU8(1)), n)), Ret(v("new_p", mir2.TyU16)))}
}
