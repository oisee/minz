package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"strings"
)

type OrderedByteHelperProof struct {
	FunctionSiteID   string `json:"function_site_id"`
	Function         string `json:"function"`
	SourceSHA256     string `json:"source_units_sha256"`
	Enabled          bool   `json:"enabled"`
	BeforeBodySHA256 string `json:"before_body_sha256"`
	AfterBodySHA256  string `json:"after_body_sha256"`
}

// validateOrderedByteShape checks actual invariants directly; it does not create
// an expected rewritten body or execute the rewrite to certify its own output.
func validateOrderedByteShape(source, actual *Func, p *orderedBytePlan, enabled bool) error {
	if actual == nil {
		return fmt.Errorf("ordered byte: missing actual function")
	}
	x, y := *source, *actual
	x.Body = nil
	y.Body = nil
	if !reflect.DeepEqual(x, y) {
		return fmt.Errorf("ordered byte: function metadata changed")
	}
	if !enabled {
		if !reflect.DeepEqual(source.Body, actual.Body) {
			return fmt.Errorf("ordered byte: disabled body changed")
		}
		return nil
	}
	if actual.Body == nil || len(actual.Body.Body) != 3 || !reflect.DeepEqual(source.Body.Body[0], actual.Body.Body[0]) {
		return fmt.Errorf("ordered byte: ordered base prefix changed")
	}
	d, ok := actual.Body.Body[1].(*VarDeclStmt)
	if !ok || d.Name != p.pointer || d.Ty != mir2.TyPtr || d.ArrayLen != 0 || d.At != nil {
		return fmt.Errorf("ordered byte: pointer binding")
	}
	dc := *d
	dc.Init = nil
	wantDecl := VarDeclStmt{Name: p.pointer, Ty: mir2.TyPtr}
	if !reflect.DeepEqual(dc, wantDecl) {
		return fmt.Errorf("ordered byte: pointer declaration metadata")
	}
	addition, ok := d.Init.(*BinExpr)
	if !ok || addition.Op != "+" || addition.Ty != mir2.TyPtr || !graceRef(addition.L, p.arr, mir2.TyPtr) || !graceRef(addition.R, p.base, mir2.TyU16) {
		return fmt.Errorf("ordered byte: address reuse arithmetic")
	}
	ret, ok := actual.Body.Body[2].(*ReturnStmt)
	if !ok || len(ret.Vals) != 0 {
		return fmt.Errorf("ordered byte: return shape")
	}
	rc := *ret
	rc.Val = nil
	if !reflect.DeepEqual(rc, ReturnStmt{}) {
		return fmt.Errorf("ordered byte: return metadata")
	}
	sum, ok := ret.Val.(*BinExpr)
	if !ok || sum.Op != "+" || sum.Ty != mir2.TyU16 {
		return fmt.Errorf("ordered byte: result sum")
	}
	low, ok := sum.L.(*CastExpr)
	if !ok || low.Ty != mir2.TyU16 {
		return fmt.Errorf("ordered byte: low widening")
	}
	load, ok := low.X.(*LoadExpr)
	if !ok || load.Ty != mir2.TyU8 || !graceRef(load.Ptr, p.pointer, mir2.TyPtr) {
		return fmt.Errorf("ordered byte: ordered low byte load")
	}
	product, ok := sum.R.(*BinExpr)
	if !ok || product.Op != "*" || product.Ty != mir2.TyU16 || !reflect.DeepEqual(product.R, U16(256)) {
		return fmt.Errorf("ordered byte: high shift")
	}
	high, ok := product.L.(*CastExpr)
	if !ok || high.Ty != mir2.TyU16 {
		return fmt.Errorf("ordered byte: high widening")
	}
	load, ok = high.X.(*LoadExpr)
	if !ok || load.Ty != mir2.TyU8 {
		return fmt.Errorf("ordered byte: high byte load")
	}
	next, ok := load.Ptr.(*BinExpr)
	if !ok || next.Op != "+" || next.Ty != mir2.TyPtr || !graceRef(next.L, p.pointer, mir2.TyPtr) || !reflect.DeepEqual(next.R, U16(1)) {
		return fmt.Errorf("ordered byte: high address")
	}
	return nil
}
func ValidateOrderedByteHelperChoices(source, actual *Module, choices map[string]bool) ([]OrderedByteHelperProof, error) {
	out := []OrderedByteHelperProof{}
	sites, e := orderedByteKeys(source, choices)
	if e != nil {
		return nil, e
	}
	if actual == nil {
		return nil, fmt.Errorf("ordered byte: nil actual")
	}
	if e = Verify(actual); e != nil {
		return nil, e
	}
	if actual.OrderedByteRWPolicy != nil || actual.OrderedBytePolicy == nil || len(actual.OrderedBytePolicy) != len(choices) || actual.GraceRulePolicy == nil || len(actual.GraceRulePolicy) != 0 || actual.PointerWalkPolicy == nil || len(actual.PointerWalkPolicy) != 0 || actual.InvariantBasePolicy == nil || len(actual.InvariantBasePolicy) != 0 || actual.ForwardCopyPolicy == nil || len(actual.ForwardCopyPolicy) != 0 || actual.RegionComparePolicy == nil || len(actual.RegionComparePolicy) != 0 {
		return nil, fmt.Errorf("ordered byte: unrequested policy")
	}
	known := map[string]bool{}
	owned := map[string]bool{}
	for _, s := range sites {
		known[s.ID] = true
		owned[s.Function] = true
	}
	for id, v := range actual.OrderedBytePolicy {
		if !known[id] || choices[id] != v {
			return nil, fmt.Errorf("ordered byte: changed function policy")
		}
	}
	for id, v := range choices {
		if got, present := actual.OrderedBytePolicy[id]; !present || got != v {
			return nil, fmt.Errorf("ordered byte: missing selected policy")
		}
	}
	originRemap := map[string]string{}
	for _, site := range sites {
		original := source.FuncByName(site.Function)
		post := actual.FuncByName(site.Function)
		p := findOrderedByteHelper(original, forwardCopyReserved(source))
		if e = validateOrderedByteShape(original, post, p, choices[site.ID]); e != nil {
			return nil, e
		}
		after, e := orderedByteDigest(post.Body)
		if e != nil {
			return nil, e
		}
		out = append(out, OrderedByteHelperProof{FunctionSiteID: site.ID, Function: site.Function, SourceSHA256: site.SourceUnitsSHA256, Enabled: choices[site.ID], BeforeBodySHA256: site.BodySHA256, AfterBodySHA256: after})
		if choices[site.ID] {
			originRemap[site.Function+"/body/s1"] = site.Function + "/body/s2"
		}
	}
	if e = validateOrderedByteModule(source, actual, owned, originRemap); e != nil {
		return nil, e
	}
	return out, nil
}
func ValidateOrderedByteHelperCompiledChoices(source, actual *Module, choices map[string]bool) ([]OrderedByteHelperProof, error) {
	sites, err := orderedByteKeys(source, choices)
	if err != nil {
		return nil, err
	}
	if actual == nil {
		return nil, fmt.Errorf("ordered byte: nil compiled actual")
	}
	for _, site := range sites {
		for _, f := range actual.Funcs {
			if f != nil && strings.HasPrefix(f.Name, site.Function+"$split_") {
				return nil, fmt.Errorf("ordered byte: split target helper unsupported")
			}
		}
	}
	view, e := normalizeRegionCompareSplits(source, actual)
	if e != nil {
		return nil, e
	}
	return ValidateOrderedByteHelperChoices(source, view, choices)
}

func validateOrderedByteModule(source, actual *Module, owned map[string]bool, originRemap map[string]string) error {
	x, y := *source, *actual
	x.SourceLocations = nil
	y.SourceLocations = nil
	x.LoopLineage = nil
	y.LoopLineage = nil
	x.OrderedByteRWPolicy = nil
	x.OrderedBytePolicy = nil
	y.OrderedByteRWPolicy = nil
	y.OrderedBytePolicy = nil
	x.GraceRulePolicy = nil
	y.GraceRulePolicy = nil
	x.PointerWalkPolicy = nil
	y.PointerWalkPolicy = nil
	x.InvariantBasePolicy = nil
	y.InvariantBasePolicy = nil
	x.ForwardCopyPolicy = nil
	y.ForwardCopyPolicy = nil
	x.RegionComparePolicy = nil
	y.RegionComparePolicy = nil
	x.Funcs = make([]*Func, len(source.Funcs))
	y.Funcs = make([]*Func, len(actual.Funcs))
	for j, f := range source.Funcs {
		c := *f
		if owned[f.Name] {
			c.Body = nil
		}
		x.Funcs[j] = &c
	}
	for j, f := range actual.Funcs {
		c := *f
		if owned[f.Name] {
			c.Body = nil
		}
		y.Funcs[j] = &c
	}
	if !reflect.DeepEqual(x, y) {
		return fmt.Errorf("ordered byte: unrelated semantic module changed")
	}
	paths, e := orderedByteStatements(source)
	if e != nil {
		return e
	}
	actualPaths, e := orderedByteStatements(actual)
	if e != nil {
		return e
	}
	expectedOrigins := map[string]SourceLocation{}
	for path, s := range paths {
		loc, ok := source.SourceLocations[s]
		if !ok {
			continue
		}
		newPath := path
		if mapped, ok := originRemap[path]; ok {
			newPath = mapped
		}
		expectedOrigins[newPath] = loc
	}
	matches := 0
	for path, s := range actualPaths {
		got, present := actual.SourceLocations[s]
		want, required := expectedOrigins[path]
		if required {
			matches++
			if !present || got != want {
				return fmt.Errorf("ordered byte: exact origin path changed")
			}
		} else if present {
			return fmt.Errorf("ordered byte: invented origin")
		}
	}
	if matches != len(expectedOrigins) || len(actual.SourceLocations) != len(expectedOrigins) {
		return fmt.Errorf("ordered byte: missing origin")
	}
	loops, e := BuildLoopSiteMap(source)
	if e != nil {
		return e
	}
	expectedLoops := map[string]LoopSite{}
	for _, s := range loops.Sites {
		expectedLoops[s.Function+"/"+s.HIRPath] = s
	}
	if actual.LoopLineage == nil || len(actual.LoopLineage) != len(expectedLoops) {
		return fmt.Errorf("ordered byte: lineage cardinality")
	}
	for path, s := range actualPaths {
		marker, present := actual.LoopLineage[s]
		want, required := expectedLoops[path]
		if required {
			if !present || marker.SiteID != want.ID || marker.Rule != "" || marker.Role != "original" || marker.PreviousRule != "" {
				return fmt.Errorf("ordered byte: original loop attribution")
			}
		} else if present {
			return fmt.Errorf("ordered byte: lineage on non-loop")
		}
	}
	return nil
}
