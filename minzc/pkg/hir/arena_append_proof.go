package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"strings"
)

// MemoryHostArenaAppendChoices is a finite exact two-function contract, not a
// policy map or authorization for arbitrary rewritten factories.
type MemoryHostArenaAppendChoices struct {
	ArenaPeel   bool `json:"arena_peel"`
	AppendHoist bool `json:"append_hoist"`
}

// ValidateMemoryHostArenaAppendChoices checks independent immutable snapshots
// before compiler preparation. It certifies only two exact structural deltas,
// closed resource effects and source origins; no inherited counts, gas equality,
// native authority or post-lowering lineage. Standalone proofs remain strict.
func ValidateMemoryHostArenaAppendChoices(source, actual *Module, w MemoryHostWorkload, choices MemoryHostArenaAppendChoices) error {
	fail := func(s string) error { return fmt.Errorf("arena append proof: %s", s) }
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
		sites, e := BuildLoopSiteMap(m)
		if e != nil {
			return e
		}
		if sites.UnmappedLoops != 0 {
			return fail("unmapped loop")
		}
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
	sf, af := source.FuncByName("region_match"), actual.FuncByName("region_match")
	sg, ag := source.FuncByName("intern_region"), actual.FuncByName("intern_region")
	if sf == nil || af == nil || sg == nil || ag == nil || !reflect.DeepEqual(sf, arenaPeelOriginalShape()) || !reflect.DeepEqual(sg, appendCopyOriginalShape()) {
		return fail("original exact functions")
	}
	sp, e := orderedByteStatements(source)
	if e != nil {
		return e
	}
	ap, e := orderedByteStatements(actual)
	if e != nil {
		return e
	}
	// A projection is not an expected transform. It is the original module with
	// ONLY the actual named function and its actually attached origin entries.
	// Unchanged strict validators independently inspect that actual delta. The
	// original source and actual module are never mutated; the aggregate below
	// still checks actual unrelated content and the entire union-origin mapping.
	if choices.ArenaPeel {
		projected := arenaAppendFunctionProjection(source, actual, "region_match", sp, ap)
		if e := ValidateMemoryHostArenaPeel(source, projected, w); e != nil {
			return e
		}
	} else if !reflect.DeepEqual(sf, af) {
		return fail("unrequested arena change")
	}
	if choices.AppendHoist {
		projected := arenaAppendFunctionProjection(source, actual, "intern_region", sp, ap)
		if e := ValidateMemoryHostAppendCopyHoist(source, projected, w); e != nil {
			return e
		}
	} else if !reflect.DeepEqual(sg, ag) {
		return fail("unrequested append change")
	}
	// Only a function whose delta has just passed the unchanged strict validator
	// can be masked. False-choice functions remain wholebody equality checked.
	x, y := *source, *actual
	x.SourceLocations = nil
	y.SourceLocations = nil
	x.Funcs = make([]*Func, len(source.Funcs))
	y.Funcs = make([]*Func, len(actual.Funcs))
	enabled := func(name string) bool {
		return name == "region_match" && choices.ArenaPeel || name == "intern_region" && choices.AppendHoist
	}
	for i, f := range source.Funcs {
		q := *f
		if enabled(f.Name) {
			q.Body = nil
		}
		x.Funcs[i] = &q
	}
	for i, f := range actual.Funcs {
		q := *f
		if enabled(f.Name) {
			q.Body = nil
		}
		y.Funcs[i] = &q
	}
	if !reflect.DeepEqual(x, y) {
		return fail("unrelated whole module content")
	}
	expected := map[string]SourceLocation{}
	for path, stmt := range sp {
		loc, has := source.SourceLocations[stmt]
		if !has {
			continue
		}
		q := path
		if choices.ArenaPeel {
			for from, to := range []int{3, 4, 5} {
				root := fmt.Sprintf("region_match/body/s%d", from)
				if path == root || strings.HasPrefix(path, root+"/") {
					q = fmt.Sprintf("region_match/body/s%d", to) + strings.TrimPrefix(path, root)
					break
				}
			}
		}
		if choices.AppendHoist {
			for from := 4; from < 8; from++ {
				root := fmt.Sprintf("intern_region/body/s%d", from)
				if path == root || strings.HasPrefix(path, root+"/") {
					q = fmt.Sprintf("intern_region/body/s%d", from+2) + strings.TrimPrefix(path, root)
					break
				}
			}
		}
		if _, exists := expected[q]; exists {
			return fail("ambiguous origin mapping")
		}
		expected[q] = loc
	}
	if len(expected) != len(source.SourceLocations) || len(actual.SourceLocations) != len(expected) {
		return fail("origin cardinality")
	}
	for path, stmt := range ap {
		got, has := actual.SourceLocations[stmt]
		want, need := expected[path]
		if has != need || has && got != want {
			return fail("exact union origin path")
		}
	}
	for path := range expected {
		if ap[path] == nil {
			return fail("missing mapped origin")
		}
	}
	return nil
}

// Deliberately private and finite: no callback, body generation or arbitrary
// target names. Called only with the two exact original functions above.
func arenaAppendFunctionProjection(source, actual *Module, name string, sp, ap map[string]Stmt) *Module {
	p := *source
	p.Funcs = append([]*Func{}, source.Funcs...)
	for i, f := range p.Funcs {
		if f.Name == name {
			p.Funcs[i] = actual.FuncByName(name)
		}
	}
	p.SourceLocations = map[Stmt]SourceLocation{}
	prefix := name + "/"
	for path, stmt := range sp {
		if !strings.HasPrefix(path, prefix) {
			if loc, has := source.SourceLocations[stmt]; has {
				p.SourceLocations[stmt] = loc
			}
		}
	}
	for path, stmt := range ap {
		if strings.HasPrefix(path, prefix) {
			if loc, has := actual.SourceLocations[stmt]; has {
				p.SourceLocations[stmt] = loc
			}
		}
	}
	return &p
}
