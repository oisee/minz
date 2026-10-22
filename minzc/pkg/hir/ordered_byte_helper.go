package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

type orderedBytePlan struct{ arr, idx, base, pointer string }

func findOrderedByteHelper(f *Func, reserved map[string]bool) *orderedBytePlan {
	if f == nil || len(f.Params) != 2 || f.Body == nil || len(f.Body.Body) != 2 {
		return nil
	}
	d, ok := f.Body.Body[0].(*VarDeclStmt)
	if !ok {
		return nil
	}
	p := &orderedBytePlan{arr: f.Params[0].Name, idx: f.Params[1].Name, base: d.Name}
	if p.arr == p.idx || p.base == p.arr || p.base == p.idx {
		return nil
	}
	for _, n := range []string{p.arr, p.idx, p.base} {
		if n == "" || reserved[n] {
			return nil
		}
	}
	arr := Var(p.arr, mir2.TyPtr)
	base := Var(p.base, mir2.TyU16)
	idx := func(offset Expr) Expr { return &IndexExpr{Base: arr, Idx: offset, ElemTy: mir2.TyU8} }
	want := Func{Name: f.Name, Params: []Param{{Name: p.arr, Ty: mir2.TyPtr}, {Name: p.idx, Ty: mir2.TyU16}}, RetTy: mir2.TyU16, Body: Blk(Decl(p.base, mir2.TyU16, &BinExpr{Op: "*", L: Var(p.idx, mir2.TyU16), R: sourceU8(2), Ty: mir2.TyU16}), &ReturnStmt{Val: Add(Cast(idx(base), mir2.TyU16), &BinExpr{Op: "*", L: Cast(idx(Add(base, sourceU8(1), mir2.TyU16)), mir2.TyU16), R: sourceU16(256), Ty: mir2.TyU16}, mir2.TyU16)})}
	if !reflect.DeepEqual(*f, want) {
		return nil
	}
	p.pointer = freshGracePointerName(f, reserved)
	return p
}
func orderedByteFresh(m *Module) bool {
	return m != nil && m.LoopLineage == nil && m.GraceRulePolicy == nil && m.PointerWalkPolicy == nil && m.InvariantBasePolicy == nil && m.ForwardCopyPolicy == nil && m.RegionComparePolicy == nil && m.OrderedBytePolicy == nil && m.OrderedByteRWPolicy == nil
}
func EligibleOrderedByteHelpers(m *Module) ([]string, error) {
	if !orderedByteFresh(m) {
		return nil, fmt.Errorf("ordered byte: fresh original required")
	}
	sites, e := BuildFunctionSiteMap(m)
	if e != nil {
		return nil, e
	}
	loops, e := BuildLoopSiteMap(m)
	if e != nil {
		return nil, e
	}
	if loops.UnmappedLoops != 0 {
		return nil, fmt.Errorf("ordered byte: unmapped original loop")
	}
	out := []string{}
	for _, s := range sites.Sites {
		if findOrderedByteHelper(m.FuncByName(s.Function), forwardCopyReserved(m)) != nil {
			out = append(out, s.ID)
		}
	}
	return out, nil
}
func orderedByteKeys(m *Module, choices map[string]bool) ([]FunctionSite, error) {
	ids, e := EligibleOrderedByteHelpers(m)
	if e != nil {
		return nil, e
	}
	if len(ids) > 4 {
		return nil, fmt.Errorf("ordered byte: exceeds four legal function dimensions")
	}
	known := map[string]bool{}
	for _, id := range ids {
		known[id] = true
	}
	for id := range choices {
		if !known[id] {
			return nil, fmt.Errorf("ordered byte: unknown function choice")
		}
	}
	sites, e := BuildFunctionSiteMap(m)
	if e != nil {
		return nil, e
	}
	out := []FunctionSite{}
	for _, s := range sites.Sites {
		if known[s.ID] {
			out = append(out, s)
		}
	}
	return out, nil
}

// ApplyOrderedByteHelperChoices is currently a direct, explicit opt-in contract;
// it does not register a default Grace rule or invent loop rewrite attribution.
func ApplyOrderedByteHelperChoices(m *Module, c *ProfileCompatibility, choices map[string]bool) error {
	if !orderedByteFresh(m) || c == nil {
		return fmt.Errorf("ordered byte: fresh original/compatibility required")
	}
	current, e := BuildProfileCompatibility(m, c.Context)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(current, c) {
		return fmt.Errorf("ordered byte: stale compatibility")
	}
	sites, e := orderedByteKeys(m, choices)
	if e != nil {
		return e
	}
	bodies := map[*Func]*Block{}
	origins := map[Stmt]SourceLocation{}
	for s, o := range m.SourceLocations {
		origins[s] = o
	}
	saved := *m
	if e = SeedLoopLineage(m); e != nil {
		*m = saved
		return e
	}
	m.OrderedBytePolicy = map[string]bool{}
	for id, v := range choices {
		m.OrderedBytePolicy[id] = v
	}
	m.GraceRulePolicy = map[string]bool{}
	m.PointerWalkPolicy = map[string]bool{}
	m.InvariantBasePolicy = map[string]bool{}
	m.ForwardCopyPolicy = map[string]bool{}
	m.RegionComparePolicy = map[string]bool{}
	m.SourceLocations = origins
	for _, site := range sites {
		if !choices[site.ID] {
			continue
		}
		f := m.FuncByName(site.Function)
		p := findOrderedByteHelper(f, forwardCopyReserved(m))
		old := f.Body
		bodies[f] = old
		ptr := Var(p.pointer, mir2.TyPtr)
		f.Body = Blk(old.Body[0], Decl(p.pointer, mir2.TyPtr, Add(Var(p.arr, mir2.TyPtr), Var(p.base, mir2.TyU16), mir2.TyPtr)), &ReturnStmt{Val: Add(Cast(&LoadExpr{Ptr: ptr, Ty: mir2.TyU8}, mir2.TyU16), &BinExpr{Op: "*", L: Cast(&LoadExpr{Ptr: Add(ptr, U16(1), mir2.TyPtr), Ty: mir2.TyU8}, mir2.TyU16), R: U16(256), Ty: mir2.TyU16}, mir2.TyU16)})
		if loc, ok := m.SourceLocations[old.Body[1]]; ok {
			delete(m.SourceLocations, old.Body[1])
			m.SourceLocations[f.Body.Body[2]] = loc
		}
	}
	if e = Verify(m); e != nil {
		*m = saved
		for f, b := range bodies {
			f.Body = b
		}
		return e
	}
	return nil
}
