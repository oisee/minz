package hir

import (
	"fmt"
	"reflect"
	"strings"
)

type MixedCopyRegionChoices struct {
	Copy   map[string]bool `json:"copy_sites"`
	Region map[string]bool `json:"region_compare_sites"`
}
type MixedCopyRegionProof struct {
	Copies  []ForwardCopyProof   `json:"copy_proofs"`
	Regions []RegionCompareProof `json:"region_compare_proofs"`
}

func MixedCopyRegionSites(m *Module) ([]string, []string, error) {
	regions, e := EligibleRegionCompareSites(m)
	if e != nil {
		return nil, nil, e
	}
	copies, e := EligibleForwardCopySites(m)
	if e != nil {
		return nil, nil, e
	}
	sites, e := BuildLoopSiteMap(m)
	if e != nil {
		return nil, nil, e
	}
	owner := map[string]string{}
	for _, s := range sites.Sites {
		owner[s.ID] = s.Function
	}
	seen := map[string]bool{}
	functions := map[string]bool{}
	for _, id := range copies {
		seen[id] = true
		functions[owner[id]] = true
	}
	for _, id := range regions {
		if seen[id] || functions[owner[id]] {
			return nil, nil, fmt.Errorf("copy region: overlapping source ownership")
		}
	}
	return copies, regions, nil
}
func mixedCopyRegionKeys(m *Module, choices MixedCopyRegionChoices) ([]string, []string, error) {
	copies, regions, e := MixedCopyRegionSites(m)
	if e != nil {
		return nil, nil, e
	}
	sets := []map[string]bool{{}, {}}
	for _, id := range copies {
		sets[0][id] = true
	}
	for _, id := range regions {
		sets[1][id] = true
	}
	for j, requested := range []map[string]bool{choices.Copy, choices.Region} {
		for id := range requested {
			if !sets[j][id] {
				return nil, nil, fmt.Errorf("copy region: unknown/wrong kind site")
			}
		}
	}
	return copies, regions, nil
}
func ApplyMixedCopyRegionChoices(m *Module, c *ProfileCompatibility, choices MixedCopyRegionChoices) error {
	if m == nil || c == nil || m.LoopLineage != nil || m.ForwardCopyPolicy != nil || m.RegionComparePolicy != nil || m.PointerWalkPolicy != nil || m.InvariantBasePolicy != nil || m.GraceRulePolicy != nil {
		return fmt.Errorf("copy region: fresh original required")
	}
	current, e := BuildProfileCompatibility(m, c.Context)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(current, c) {
		return fmt.Errorf("copy region: stale compatibility")
	}
	if _, _, e = mixedCopyRegionKeys(m, choices); e != nil {
		return e
	}
	saved := *m
	old := map[*Func]*Block{}
	for _, f := range m.Funcs {
		if f.Body != nil {
			old[f] = Blk(append([]Stmt(nil), f.Body.Body...)...)
		} else {
			old[f] = nil
		}
	}
	if e = SeedLoopLineage(m); e != nil {
		return e
	}
	own := func(in map[string]bool) map[string]bool {
		out := map[string]bool{}
		for id, v := range in {
			out[id] = v
		}
		return out
	}
	m.ForwardCopyPolicy = own(choices.Copy)
	m.RegionComparePolicy = own(choices.Region)
	m.PointerWalkPolicy = map[string]bool{}
	m.InvariantBasePolicy = map[string]bool{}
	m.GraceRulePolicy = map[string]bool{ForwardCopyRule: true, RegionCompareRule: true}
	if _, e = RunGraceWithOptions(m, GraceOptions{OptimizeForSize: false}); e != nil {
		*m = saved
		for f, b := range old {
			f.Body = b
		}
		return e
	}
	return nil
}

// ValidateMixedCopyRegionChoices is a distinct aggregate contract. Standalone
// proofs retain their own strict policy/module boundaries; this aggregate owns
// both kinds, checks every legal disabled site and every untouched source loop.
func ValidateMixedCopyRegionChoices(source, actual *Module, choices MixedCopyRegionChoices) (MixedCopyRegionProof, error) {
	out := MixedCopyRegionProof{Copies: []ForwardCopyProof{}, Regions: []RegionCompareProof{}}
	fail := func(s string) (MixedCopyRegionProof, error) { return out, fmt.Errorf("copy region: %s", s) }
	copies, regions, e := mixedCopyRegionKeys(source, choices)
	if e != nil {
		return out, e
	}
	if actual == nil {
		return fail("nil actual")
	}
	if e = Verify(actual); e != nil {
		return out, e
	}
	if actual.ForwardCopyPolicy == nil || actual.RegionComparePolicy == nil || actual.InvariantBasePolicy == nil || actual.PointerWalkPolicy == nil || actual.GraceRulePolicy == nil || len(actual.InvariantBasePolicy) != 0 || len(actual.PointerWalkPolicy) != 0 || len(actual.GraceRulePolicy) != 2 || !actual.GraceRulePolicy[ForwardCopyRule] || !actual.GraceRulePolicy[RegionCompareRule] {
		return fail("unrequested/missing policies")
	}
	checkMap := func(legal []string, got, wanted map[string]bool) bool {
		known := map[string]bool{}
		for _, id := range legal {
			known[id] = true
		}
		for id, v := range got {
			if !known[id] || wanted[id] != v {
				return false
			}
		}
		for id, v := range wanted {
			if got[id] != v {
				return false
			}
		}
		return true
	}
	if !checkMap(copies, actual.ForwardCopyPolicy, choices.Copy) || !checkMap(regions, actual.RegionComparePolicy, choices.Region) {
		return fail("policy differs from requested choice")
	}
	sites, e := BuildLoopSiteMap(source)
	if e != nil {
		return out, e
	}
	owned := map[string]bool{}
	kinds := map[string]string{}
	for _, id := range copies {
		kinds[id] = ForwardCopyRule
	}
	for _, id := range regions {
		kinds[id] = RegionCompareRule
	}
	for _, s := range sites.Sites {
		if kinds[s.ID] != "" {
			owned[s.Function] = true
		}
	}
	x, y := *source, *actual
	x.SourceLocations = nil
	y.SourceLocations = nil
	x.LoopLineage = nil
	y.LoopLineage = nil
	x.ForwardCopyPolicy = nil
	y.ForwardCopyPolicy = nil
	x.RegionComparePolicy = nil
	y.RegionComparePolicy = nil
	x.InvariantBasePolicy = nil
	y.InvariantBasePolicy = nil
	x.PointerWalkPolicy = nil
	y.PointerWalkPolicy = nil
	x.GraceRulePolicy = nil
	y.GraceRulePolicy = nil
	x.Funcs = make([]*Func, len(source.Funcs))
	y.Funcs = make([]*Func, len(actual.Funcs))
	for j, f := range source.Funcs {
		copy := *f
		if owned[f.Name] {
			copy.Body = nil
		}
		x.Funcs[j] = &copy
	}
	for j, f := range actual.Funcs {
		copy := *f
		if owned[f.Name] {
			copy.Body = nil
		}
		y.Funcs[j] = &copy
	}
	if !reflect.DeepEqual(x, y) {
		return fail("unrelated module content changed")
	}
	for _, id := range copies {
		proof, e := validateForwardCopyShape(source, actual, id, choices.Copy[id])
		if e != nil {
			return out, e
		}
		out.Copies = append(out.Copies, proof)
	}
	for _, id := range regions {
		proof, e := validateRegionCompareShape(source, actual, id, choices.Region[id], false)
		if e != nil {
			return out, e
		}
		out.Regions = append(out.Regions, proof)
	}
	known := map[string]LoopSite{}
	for _, s := range sites.Sites {
		known[s.ID] = s
	}
	counts := map[string]map[string]int{}
	reachable := map[Stmt]bool{}
	var bad error
	for _, f := range actual.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for j, s := range b.Body {
				reachable[s] = true
				l, marked := actual.LoopLineage[s]
				loop := false
				switch s.(type) {
				case *WhileStmt, *ForRangeStmt, *ForEachStmt:
					loop = true
				}
				if marked != loop {
					bad = fmt.Errorf("copy region: markers only on every actual loop")
					continue
				}
				if !loop {
					continue
				}
				origin, ok := known[l.SiteID]
				if !ok || origin.Function != f.Name || l.PreviousRule != "" {
					bad = fmt.Errorf("copy region: unknown/cross function lineage")
					continue
				}
				if counts[l.SiteID] == nil {
					counts[l.SiteID] = map[string]int{}
				}
				counts[l.SiteID][l.Role]++
				enabled := choices.Copy[l.SiteID] || choices.Region[l.SiteID]
				if enabled {
					if l.Rule != kinds[l.SiteID] || (l.Role != "fast" && l.Role != "fallback") {
						bad = fmt.Errorf("copy region: wrong selected role/rule")
					}
				} else if l.Role != "original" || l.Rule != "" || origin.HIRPath != fmt.Sprintf("%s/s%d", path, j) {
					bad = fmt.Errorf("copy region: original source path/rule changed")
				}
				if l.Role == "original" || l.Role == "fallback" {
					if loc, ok := actual.SourceLocations[s]; !ok || loc != origin.Origin {
						bad = fmt.Errorf("copy region: origin site mismatch")
					}
				} else if _, ok := actual.SourceLocations[s]; ok {
					bad = fmt.Errorf("copy region: invented fast origin")
				}
			}
		})
	}
	if bad != nil {
		return out, bad
	}
	for stmt := range actual.LoopLineage {
		if !reachable[stmt] {
			return fail("orphan lineage")
		}
	}
	for id := range known {
		got := counts[id]
		if choices.Copy[id] || choices.Region[id] {
			if len(got) != 2 || got["fast"] != 1 || got["fallback"] != 1 {
				return fail("selected lineage multiplicity")
			}
		} else if len(got) != 1 || got["original"] != 1 {
			return fail("original lineage multiplicity")
		}
	}
	if e := mixedCopyRegionOrigins(source, actual, owned, kinds, choices); e != nil {
		return out, e
	}

	return out, nil
}
func ValidateMixedCopyRegionCompiledChoices(source, actual *Module, choices MixedCopyRegionChoices) (MixedCopyRegionProof, error) {
	view, e := normalizeRegionCompareSplits(source, actual)
	if e != nil {
		return MixedCopyRegionProof{}, e
	}
	return ValidateMixedCopyRegionChoices(source, view, choices)
}

func mixedCopyRegionOrigins(source, actual *Module, owned map[string]bool, kinds map[string]string, choices MixedCopyRegionChoices) error {
	expected := map[string]struct {
		stmt   Stmt
		origin SourceLocation
	}{}
	sites, _ := BuildLoopSiteMap(source)
	siteByFunc := map[string]LoopSite{}
	sourceReachable := map[Stmt]bool{}
	duplicate := false
	for _, site := range sites.Sites {
		if kinds[site.ID] != "" {
			siteByFunc[site.Function] = site
		}
	}
	for _, f := range source.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for j, s := range b.Body {
				if sourceReachable[s] {
					duplicate = true
				}
				sourceReachable[s] = true
				origin, ok := source.SourceLocations[s]
				if !ok {
					continue
				}
				p := fmt.Sprintf("%s/s%d", path, j)
				site := siteByFunc[f.Name]
				if owned[f.Name] && (choices.Copy[site.ID] || choices.Region[site.ID]) && strings.HasPrefix(p, "body/s1") {
					prefix := "body/s1/else/s0"
					if kinds[site.ID] == RegionCompareRule {
						prefix = "body/s1/s6/else/s0"
					}
					p = prefix + strings.TrimPrefix(p, "body/s1")
				}
				expected[f.Name+"/"+p] = struct {
					stmt   Stmt
					origin SourceLocation
				}{s, origin}
			}
		})
	}
	if duplicate {
		return fmt.Errorf("copy region: ambiguous shared source statement")
	}
	for stmt := range source.SourceLocations {
		if !sourceReachable[stmt] {
			return fmt.Errorf("copy region: detached original origin")
		}
	}
	if len(expected) != len(source.SourceLocations) {
		return fmt.Errorf("copy region: ambiguous original origins")
	}
	seen := map[Stmt]bool{}
	matched := 0
	var bad error
	for _, f := range actual.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for j, s := range b.Body {
				if seen[s] {
					bad = fmt.Errorf("copy region: ambiguous shared actual statement")
				}
				seen[s] = true
				key := f.Name + "/" + fmt.Sprintf("%s/s%d", path, j)
				want, required := expected[key]
				got, present := actual.SourceLocations[s]
				if required {
					matched++
					if !present || got != want.origin || !reflect.DeepEqual(s, want.stmt) {
						bad = fmt.Errorf("copy region: source origin exact path changed")
					}
				} else if present {
					bad = fmt.Errorf("copy region: invented/moved source origin")
				}
			}
		})
	}
	if bad != nil {
		return bad
	}
	if matched != len(expected) || len(actual.SourceLocations) != len(expected) {
		return fmt.Errorf("copy region: missing source origin path")
	}
	for s := range actual.SourceLocations {
		if !seen[s] {
			return fmt.Errorf("copy region: detached source origin")
		}
	}
	return nil
}
