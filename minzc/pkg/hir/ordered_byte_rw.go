package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"strings"
)

const OrderedByteReadKind = "read-u16-le"
const OrderedByteWriteKind = "write-u16-le"

type FunctionChoice struct {
	Kind    string `json:"kind"`
	Enabled bool   `json:"enabled"`
}
type OrderedByteRWFunctionSite struct {
	FunctionSite
	Kind string `json:"kind"`
}
type OrderedByteRWProof struct {
	OrderedByteHelperProof
	Kind string `json:"kind"`
}

func findOrderedByteWriteHelper(f *Func, reserved map[string]bool) *orderedBytePlan {
	if f == nil || len(f.Params) != 3 || f.Body == nil || len(f.Body.Body) != 3 {
		return nil
	}
	d, ok := f.Body.Body[0].(*VarDeclStmt)
	if !ok {
		return nil
	}
	p := &orderedBytePlan{arr: f.Params[0].Name, idx: f.Params[1].Name, base: d.Name}
	valName := f.Params[2].Name
	seen := map[string]bool{}
	for _, n := range []string{p.arr, p.idx, p.base, valName} {
		if n == "" || seen[n] || reserved[n] {
			return nil
		}
		seen[n] = true
	}
	arr := Var(p.arr, mir2.TyPtr)
	base := Var(p.base, mir2.TyU16)
	val := Var(valName, mir2.TyU16)
	ix := func(x Expr) Expr { return &IndexExpr{Base: arr, Idx: x, ElemTy: mir2.TyU8} }
	want := Func{Name: f.Name, Params: []Param{{Name: p.arr, Ty: mir2.TyPtr}, {Name: p.idx, Ty: mir2.TyU16}, {Name: valName, Ty: mir2.TyU16}}, RetTy: mir2.TyVoid, Body: Blk(Decl(p.base, mir2.TyU16, &BinExpr{Op: "*", L: Var(p.idx, mir2.TyU16), R: sourceU8(2), Ty: mir2.TyU16}), Assign(ix(base), Cast(val, mir2.TyU8)), Assign(ix(Add(base, sourceU8(1), mir2.TyU16)), Cast(&BinExpr{Op: "/", L: val, R: sourceU16(256), Ty: mir2.TyU16}, mir2.TyU8)))}
	if !reflect.DeepEqual(*f, want) {
		return nil
	}
	p.pointer = freshGracePointerName(f, reserved)
	return p
}
func EligibleOrderedByteRWHelpers(m *Module) ([]OrderedByteRWFunctionSite, error) {
	if !orderedByteFresh(m) {
		return nil, fmt.Errorf("ordered byte RW: fresh original required")
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
		return nil, fmt.Errorf("ordered byte RW: unmapped loops")
	}
	out := []OrderedByteRWFunctionSite{}
	reserved := forwardCopyReserved(m)
	for _, s := range sites.Sites {
		kind := ""
		f := m.FuncByName(s.Function)
		if findOrderedByteHelper(f, reserved) != nil {
			kind = OrderedByteReadKind
		} else if findOrderedByteWriteHelper(f, reserved) != nil {
			kind = OrderedByteWriteKind
		}
		if kind != "" {
			out = append(out, OrderedByteRWFunctionSite{FunctionSite: s, Kind: kind})
		}
	}
	return out, nil
}
func orderedByteRWKeys(m *Module, choices map[string]FunctionChoice) ([]OrderedByteRWFunctionSite, error) {
	sites, e := EligibleOrderedByteRWHelpers(m)
	if e != nil {
		return nil, e
	}
	if len(sites) > 4 {
		return nil, fmt.Errorf("ordered byte RW: exceeds four total helper dimensions")
	}
	known := map[string]string{}
	for _, s := range sites {
		known[s.ID] = s.Kind
	}
	for id, c := range choices {
		if known[id] == "" || known[id] != c.Kind {
			return nil, fmt.Errorf("ordered byte RW: unknown helper or kind")
		}
	}
	return sites, nil
}
func ApplyOrderedByteRWChoices(m *Module, c *ProfileCompatibility, choices map[string]FunctionChoice) error {
	if !orderedByteFresh(m) || c == nil {
		return fmt.Errorf("ordered byte RW: fresh original required")
	}
	current, e := BuildProfileCompatibility(m, c.Context)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(current, c) {
		return fmt.Errorf("ordered byte RW: stale compatibility")
	}
	sites, e := orderedByteRWKeys(m, choices)
	if e != nil {
		return e
	}
	saved := *m
	bodies := map[*Func]*Block{}
	origins := map[Stmt]SourceLocation{}
	for s, o := range m.SourceLocations {
		origins[s] = o
	}
	if e = SeedLoopLineage(m); e != nil {
		*m = saved
		return e
	}
	m.OrderedByteRWPolicy = map[string]FunctionChoice{}
	for id, c := range choices {
		m.OrderedByteRWPolicy[id] = c
	}
	m.GraceRulePolicy = map[string]bool{}
	m.PointerWalkPolicy = map[string]bool{}
	m.InvariantBasePolicy = map[string]bool{}
	m.ForwardCopyPolicy = map[string]bool{}
	m.RegionComparePolicy = map[string]bool{}
	m.SourceLocations = origins
	for _, s := range sites {
		if !choices[s.ID].Enabled {
			continue
		}
		f := m.FuncByName(s.Function)
		old := f.Body
		bodies[f] = old
		p := findOrderedByteHelper(f, forwardCopyReserved(m))
		if s.Kind == OrderedByteWriteKind {
			p = findOrderedByteWriteHelper(f, forwardCopyReserved(m))
		}
		ptr := Var(p.pointer, mir2.TyPtr)
		prefix := Decl(p.pointer, mir2.TyPtr, Add(Var(p.arr, mir2.TyPtr), Var(p.base, mir2.TyU16), mir2.TyPtr))
		if s.Kind == OrderedByteReadKind {
			f.Body = Blk(old.Body[0], prefix, &ReturnStmt{Val: Add(Cast(&LoadExpr{Ptr: ptr, Ty: mir2.TyU8}, mir2.TyU16), &BinExpr{Op: "*", L: Cast(&LoadExpr{Ptr: Add(ptr, U16(1), mir2.TyPtr), Ty: mir2.TyU8}, mir2.TyU16), R: U16(256), Ty: mir2.TyU16}, mir2.TyU16)})
		} else {
			f.Body = Blk(old.Body[0], prefix, &StoreStmt{Ptr: ptr, Val: old.Body[1].(*AssignStmt).Val}, &StoreStmt{Ptr: Add(ptr, U16(1), mir2.TyPtr), Val: old.Body[2].(*AssignStmt).Val})
		}
		for j := 1; j < len(old.Body); j++ {
			if o, ok := m.SourceLocations[old.Body[j]]; ok {
				delete(m.SourceLocations, old.Body[j])
				m.SourceLocations[f.Body.Body[j+1]] = o
			}
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
func validateOrderedByteWriteShape(source, actual *Func, p *orderedBytePlan, enabled bool) error {
	if actual == nil {
		return fmt.Errorf("ordered byte RW: missing write helper")
	}
	x, y := *source, *actual
	x.Body = nil
	y.Body = nil
	if !reflect.DeepEqual(x, y) {
		return fmt.Errorf("ordered byte RW: write metadata changed")
	}
	if !enabled {
		if !reflect.DeepEqual(source.Body, actual.Body) {
			return fmt.Errorf("ordered byte RW: disabled write body changed")
		}
		return nil
	}
	if actual.Body == nil || len(actual.Body.Body) != 4 || !reflect.DeepEqual(source.Body.Body[0], actual.Body.Body[0]) {
		return fmt.Errorf("ordered byte RW: base prefix")
	}
	d, ok := actual.Body.Body[1].(*VarDeclStmt)
	if !ok {
		return fmt.Errorf("ordered byte RW: pointer declaration")
	}
	dc := *d
	dc.Init = nil
	if !reflect.DeepEqual(dc, VarDeclStmt{Name: p.pointer, Ty: mir2.TyPtr}) {
		return fmt.Errorf("ordered byte RW: pointer declaration metadata")
	}
	add, ok := d.Init.(*BinExpr)
	if !ok || add.Op != "+" || add.Ty != mir2.TyPtr || !graceRef(add.L, p.arr, mir2.TyPtr) || !graceRef(add.R, p.base, mir2.TyU16) {
		return fmt.Errorf("ordered byte RW: address reuse")
	}
	for j := 0; j < 2; j++ {
		store, ok := actual.Body.Body[j+2].(*StoreStmt)
		if !ok {
			return fmt.Errorf("ordered byte RW: ordered byte stores")
		}
		sc := *store
		sc.Ptr = nil
		sc.Val = nil
		if !reflect.DeepEqual(sc, StoreStmt{}) || !reflect.DeepEqual(store.Val, source.Body.Body[j+1].(*AssignStmt).Val) {
			return fmt.Errorf("ordered byte RW: original value expression order")
		}
		if j == 0 {
			if !graceRef(store.Ptr, p.pointer, mir2.TyPtr) {
				return fmt.Errorf("ordered byte RW: first store address")
			}
		} else {
			next, ok := store.Ptr.(*BinExpr)
			if !ok || next.Op != "+" || next.Ty != mir2.TyPtr || !graceRef(next.L, p.pointer, mir2.TyPtr) || !reflect.DeepEqual(next.R, U16(1)) {
				return fmt.Errorf("ordered byte RW: second store address")
			}
		}
	}
	return nil
}
func ValidateOrderedByteRWChoices(source, actual *Module, choices map[string]FunctionChoice) ([]OrderedByteRWProof, error) {
	sites, e := orderedByteRWKeys(source, choices)
	if e != nil {
		return nil, e
	}
	if actual == nil {
		return nil, fmt.Errorf("ordered byte RW: nil actual")
	}
	if e = Verify(actual); e != nil {
		return nil, e
	}
	if choices == nil {
		choices = map[string]FunctionChoice{}
	}
	if actual.OrderedBytePolicy != nil || actual.OrderedByteRWPolicy == nil || !reflect.DeepEqual(actual.OrderedByteRWPolicy, choices) {
		return nil, fmt.Errorf("ordered byte RW: exact typed policy required")
	}
	for _, p := range []map[string]bool{actual.GraceRulePolicy, actual.PointerWalkPolicy, actual.InvariantBasePolicy, actual.ForwardCopyPolicy, actual.RegionComparePolicy} {
		if p == nil || len(p) != 0 {
			return nil, fmt.Errorf("ordered byte RW: foreign policy")
		}
	}
	out := []OrderedByteRWProof{}
	owned := map[string]bool{}
	remap := map[string]string{}
	for _, s := range sites {
		f := source.FuncByName(s.Function)
		post := actual.FuncByName(s.Function)
		enabled := choices[s.ID].Enabled
		owned[s.Function] = true
		p := findOrderedByteHelper(f, forwardCopyReserved(source))
		if s.Kind == OrderedByteReadKind {
			e = validateOrderedByteShape(f, post, p, enabled)
		} else {
			p = findOrderedByteWriteHelper(f, forwardCopyReserved(source))
			e = validateOrderedByteWriteShape(f, post, p, enabled)
		}
		if e != nil {
			return nil, e
		}
		hash, e := orderedByteDigest(post.Body)
		if e != nil {
			return nil, e
		}
		out = append(out, OrderedByteRWProof{OrderedByteHelperProof: OrderedByteHelperProof{FunctionSiteID: s.ID, Function: s.Function, SourceSHA256: s.SourceUnitsSHA256, Enabled: enabled, BeforeBodySHA256: s.BodySHA256, AfterBodySHA256: hash}, Kind: s.Kind})
		if enabled {
			for j := 1; j < len(f.Body.Body); j++ {
				remap[fmt.Sprintf("%s/body/s%d", f.Name, j)] = fmt.Sprintf("%s/body/s%d", f.Name, j+1)
			}
		}
	}
	if e = validateOrderedByteModule(source, actual, owned, remap); e != nil {
		return nil, e
	}
	return out, nil
}
func ValidateOrderedByteRWCompiledChoices(source, actual *Module, choices map[string]FunctionChoice) ([]OrderedByteRWProof, error) {
	sites, e := orderedByteRWKeys(source, choices)
	if e != nil {
		return nil, e
	}
	if actual == nil {
		return nil, fmt.Errorf("ordered byte RW: nil actual")
	}
	for _, s := range sites {
		for _, f := range actual.Funcs {
			if f != nil && strings.HasPrefix(f.Name, s.Function+"$split_") {
				return nil, fmt.Errorf("ordered byte RW: split target helper unsupported")
			}
		}
	}
	view, e := normalizeRegionCompareSplits(source, actual)
	if e != nil {
		return nil, e
	}
	return ValidateOrderedByteRWChoices(source, view, choices)
}
