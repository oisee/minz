package pipeline

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"

	"github.com/minz/minzc/pkg/hir"
)

type MemoryHostArenaAppendSelectorOptions struct {
	Goal     string `json:"goal"`
	MaxSites int    `json:"max_sites"`
}
type MemoryHostArenaAppendSourceSites struct {
	Arena  hir.LoopSite `json:"arena"`
	Append hir.LoopSite `json:"append"`
}

type MemoryHostArenaAppendPrecompileProof struct {
	Choices               hir.MemoryHostArenaAppendChoices `json:"choices"`
	Schema                string                           `json:"schema"`
	Function              string                           `json:"function"`
	SourceSites           MemoryHostArenaAppendSourceSites `json:"source_sites"`
	OriginalCompatibility *hir.ProfileCompatibility        `json:"original_compatibility"`
	ActualCompatibility   *hir.ProfileCompatibility        `json:"actual_compatibility"`
	OriginalFunctions     *hir.FunctionSiteMap             `json:"original_functions"`
	ActualFunctions       *hir.FunctionSiteMap             `json:"actual_functions"`
	ActualSites           *hir.LoopSiteMap                 `json:"actual_sites"`
}

type MemoryHostArenaAppendCandidate struct {
	Choices         hir.MemoryHostArenaAppendChoices      `json:"choices"`
	PrecompileProof *MemoryHostArenaAppendPrecompileProof `json:"precompile_proof"`
	Evidence        *MemoryHostBaselineEvidence           `json:"evidence"`
}
type MemoryHostArenaAppendSelection struct {
	Schema      string                               `json:"schema"`
	Options     MemoryHostArenaAppendSelectorOptions `json:"options"`
	Profile     *hir.MemoryHostLoopProfileRun        `json:"profile"`
	Association *MemoryHostLoopProfileEvidence       `json:"association"`
	SourceSites MemoryHostArenaAppendSourceSites     `json:"source_sites"`
	Candidates  []MemoryHostArenaAppendCandidate     `json:"candidates"`
	Selected    int                                  `json:"selected"`
}

// MeasureMemoryHostArenaAppend is bounded to two exact source loops/four variants.
// Goal compares measured costs; it does not change compiler options. Candidate
// construction is private and its proof applies before compiler preparation.
func MeasureMemoryHostArenaAppend(factory func() (*hir.Module, error), w hir.MemoryHostWorkload, serialized []byte, o MemoryHostArenaAppendSelectorOptions) (*MemoryHostArenaAppendSelection, error) {
	if factory == nil || o.MaxSites != 2 || (o.Goal != "speed" && o.Goal != "size") {
		return nil, fmt.Errorf("arena append selector: invalid bounded options/factory")
	}
	if len(serialized) > hir.MaxMemoryHostLoopProfileBytes {
		return nil, fmt.Errorf("arena append selector: profile size limit")
	}
	owned, e := ownMemoryHostWorkload(w)
	if e != nil {
		return nil, e
	}
	payload := append([]byte{}, serialized...)
	seen := map[uintptr]any{}
	retained := []*hir.Module{}
	privateRoots := []any{}
	defer func() { runtime.KeepAlive(privateRoots); runtime.KeepAlive(retained) }()
	fresh := func() (*hir.Module, error) {
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if m == nil {
			return nil, fmt.Errorf("arena append selector: nil source")
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		retainArenaSourceGraph(m, &privateRoots)
		retained = append(retained, m)
		return m, nil
	}
	source, e := fresh()
	if e != nil {
		return nil, e
	}
	loaded, e := hir.LoadMemoryHostLoopProfile(bytes.NewReader(payload), source, owned)
	if e != nil {
		return nil, e
	}
	expectedFunctions, e := hir.BuildFunctionSiteMap(source)
	if e != nil {
		return nil, e
	}
	excludedCompiled := map[*hir.Module]bool{}
	identity := func(m *hir.Module) error {
		_, _, k, e := hir.PrepareMemoryHostWorkloadCompatibility(m, owned)
		if e != nil {
			return e
		}
		k.Context.SemanticsID += ":" + hir.MemoryHostLoopProfileSemantics
		if !reflect.DeepEqual(k, loaded.Compatibility) {
			return fmt.Errorf("arena append selector: retained source identity changed")
		}
		fs, e := hir.BuildFunctionSiteMap(m)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(fs, expectedFunctions) {
			return fmt.Errorf("arena append selector: function/origin identity changed")
		}
		return nil
	}
	check := func() error {
		for _, m := range retained {
			if excludedCompiled[m] {
				continue
			}
			if e := identity(m); e != nil {
				return e
			}
		}
		return nil
	}
	association, e := RunZ80MemoryHostLoopProfile(fresh, owned)
	if e != nil {
		return nil, e
	}
	if len(retained) != 6 {
		return nil, fmt.Errorf("arena append selector: association factory lifecycle changed")
	}
	excludedCompiled[retained[2]] = true
	if e = check(); e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(loaded, association.Profile) {
		return nil, fmt.Errorf("arena append selector: serialized counts/observations differ from fresh closed run")
	}
	pair, e := memoryHostArenaAppendSourceSites(loaded.Sites)
	if e != nil {
		return nil, e
	}
	for _, wanted := range []hir.LoopSite{pair.Arena, pair.Append} {
		observed := false
		for i, site := range loaded.Sites.Sites {
			if site.ID == wanted.ID {
				observed = loaded.Counts[i].Entries > 0
			}
		}
		if !observed {
			return nil, fmt.Errorf("arena append selector: both exact sites must be observed")
		}
	}
	originalProof, e := buildMemoryHostArenaAppendPrecompileProof(source, retained[1], owned, hir.MemoryHostArenaAppendChoices{})
	if e != nil {
		return nil, e
	}
	out := &MemoryHostArenaAppendSelection{Schema: "minz.memory-host-arena-append-selection/v1", Options: o, Profile: loaded, Association: association, SourceSites: pair, Candidates: []MemoryHostArenaAppendCandidate{{Choices: hir.MemoryHostArenaAppendChoices{}, PrecompileProof: originalProof, Evidence: association.Baseline}}}
	if e = bindMemoryHostArenaAppendProof(originalProof, pair, hir.MemoryHostArenaAppendChoices{}, association.Baseline, association.Baseline); e != nil {
		return nil, e
	}
	for mask := 1; mask < 4; mask++ {
		choices := hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}
		countBefore := len(retained)
		calls := 0
		// Candidate runner remains private: caller factory only returns originals.
		// Each original is checked and retained separately from the mutated candidate.
		var precompileProof *MemoryHostArenaAppendPrecompileProof
		variantFactory := func() (*hir.Module, error) {
			calls++
			original, e := fresh()
			if e != nil {
				return nil, e
			}
			if e = identity(original); e != nil {
				return nil, e
			}
			calls++
			actual, e := factory()
			if e != nil {
				return nil, e
			}
			if actual == nil {
				return nil, fmt.Errorf("arena append selector: nil candidate source")
			}
			if e = jointCheckFreshHIR(actual, seen); e != nil {
				return nil, e
			}
			if e = identity(actual); e != nil {
				return nil, e
			}
			retainArenaSourceGraph(actual, &privateRoots)
			if e = constructMemoryHostArenaAppend(actual, owned, choices.ArenaPeel, choices.AppendHoist); e != nil {
				return nil, e
			}
			if e = hir.ValidateMemoryHostArenaAppendChoices(original, actual, owned, choices); e != nil {
				return nil, e
			}
			proof, e := buildMemoryHostArenaAppendPrecompileProof(original, actual, owned, choices)
			if e != nil {
				return nil, e
			}
			if precompileProof == nil {
				precompileProof = proof
			} else if !reflect.DeepEqual(precompileProof, proof) {
				return nil, fmt.Errorf("arena append selector: precompile proof drift")
			}
			if e = check(); e != nil {
				return nil, e
			}
			return actual, nil
		}
		variant, e := RunZ80MemoryHostBaseline(variantFactory, owned)
		if e != nil {
			return nil, e
		}
		if e = check(); e != nil {
			return nil, e
		}
		if calls != 6 || len(retained) != countBefore+3 {
			return nil, fmt.Errorf("arena append selector: candidate lifecycle changed")
		}
		if e = bindMemoryHostArenaAppendProof(precompileProof, pair, choices, association.Baseline, variant); e != nil {
			return nil, e
		}
		if e = arenaSameObservations(association.Baseline.Raw.Invocations, variant.Raw.Invocations); e != nil {
			return nil, e
		}
		if e = arenaSameObservations(association.Baseline.Target.Invocations, variant.Target.Invocations); e != nil {
			return nil, e
		}
		out.Candidates = append(out.Candidates, MemoryHostArenaAppendCandidate{Choices: choices, PrecompileProof: precompileProof, Evidence: variant})
		if selectMemoryHostArenaCandidate(o.Goal, out.Candidates[out.Selected].Evidence.Target, variant.Target) == 1 {
			out.Selected = mask
		}
	}
	if e = check(); e != nil {
		return nil, e
	}
	return out, nil
}

// Captured independently validated precompile identities. This record contains
// no claim that the same HIR shape survives compiler splitting or lowering.
func buildMemoryHostArenaAppendPrecompileProof(source, actual *hir.Module, w hir.MemoryHostWorkload, choices hir.MemoryHostArenaAppendChoices) (*MemoryHostArenaAppendPrecompileProof, error) {
	if e := hir.ValidateMemoryHostArenaAppendChoices(source, actual, w, choices); e != nil {
		return nil, e
	}
	_, sourceContext, e := hir.PrepareMemoryHostWorkload(source, w)
	if e != nil {
		return nil, e
	}
	_, actualContext, e := hir.PrepareMemoryHostWorkload(actual, w)
	if e != nil {
		return nil, e
	}
	sk, e := hir.BuildProfileCompatibility(source, sourceContext)
	if e != nil {
		return nil, e
	}
	ak, e := hir.BuildProfileCompatibility(actual, actualContext)
	if e != nil {
		return nil, e
	}
	sf, e := hir.BuildFunctionSiteMap(source)
	if e != nil {
		return nil, e
	}
	af, e := hir.BuildFunctionSiteMap(actual)
	if e != nil {
		return nil, e
	}
	ss, e := hir.BuildLoopSiteMap(source)
	if e != nil {
		return nil, e
	}
	as, e := hir.BuildLoopSiteMap(actual)
	if e != nil {
		return nil, e
	}
	pair, e := memoryHostArenaAppendSourceSites(ss)
	if e != nil {
		return nil, e
	}
	for _, wanted := range []struct {
		site hir.LoopSite
		path string
	}{{pair.Arena, "body/s1"}, {pair.Append, "body/s5"}} {
		path := wanted.path
		if wanted.site.ID == pair.Arena.ID && choices.ArenaPeel {
			path = "body/s4"
		}
		if wanted.site.ID == pair.Append.ID && choices.AppendHoist {
			path = "body/s7"
		}
		found := false
		for _, site := range as.Sites {
			if site.ID == wanted.site.ID {
				found = true
				if site.Function != wanted.site.Function || site.Kind != wanted.site.Kind || site.HIRPath != path || !reflect.DeepEqual(site.Origin, wanted.site.Origin) {
					return nil, fmt.Errorf("arena append selector: actual site differs")
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("arena append selector: actual site absent")
		}
	}
	return &MemoryHostArenaAppendPrecompileProof{Schema: "minz.exact-memory-host-arena-append-precompile/v1", Choices: choices, Function: "region_match+intern_region", SourceSites: pair, OriginalCompatibility: sk, ActualCompatibility: ak, OriginalFunctions: sf, ActualFunctions: af, ActualSites: as}, nil
}
func memoryHostArenaAppendSourceSites(sites *hir.LoopSiteMap) (MemoryHostArenaAppendSourceSites, error) {
	var out MemoryHostArenaAppendSourceSites
	for _, s := range sites.Sites {
		if s.Function == "region_match" && s.HIRPath == "body/s1" {
			if out.Arena.ID != "" {
				return out, fmt.Errorf("duplicatearena site")
			}
			out.Arena = s
		}
		if s.Function == "intern_region" && s.HIRPath == "body/s5" {
			if out.Append.ID != "" {
				return out, fmt.Errorf("duplicateappend site")
			}
			out.Append = s
		}
	}
	if out.Arena.ID == "" || out.Append.ID == "" {
		return out, fmt.Errorf("missingexactsource sites")
	}
	return out, nil
}
func bindMemoryHostArenaAppendProof(p *MemoryHostArenaAppendPrecompileProof, sites MemoryHostArenaAppendSourceSites, choices hir.MemoryHostArenaAppendChoices, original, actual *MemoryHostBaselineEvidence) error {
	if p == nil || p.Schema != "minz.exact-memory-host-arena-append-precompile/v1" || p.Function != "region_match+intern_region" || p.ActualSites == nil || p.Choices != choices || !reflect.DeepEqual(p.SourceSites, sites) || !reflect.DeepEqual(p.OriginalCompatibility, original.Compatibility) || !reflect.DeepEqual(p.ActualCompatibility, actual.Compatibility) || !reflect.DeepEqual(p.OriginalFunctions, original.FunctionSites) || !reflect.DeepEqual(p.ActualFunctions, actual.FunctionSites) {
		return fmt.Errorf("arena append selector: measuredbaseline differs from proof")
	}
	for _, expected := range []struct {
		site hir.LoopSite
		path string
	}{{sites.Arena, "body/s1"}, {sites.Append, "body/s5"}} {
		path := expected.path
		if expected.site.ID == sites.Arena.ID && choices.ArenaPeel {
			path = "body/s4"
		}
		if expected.site.ID == sites.Append.ID && choices.AppendHoist {
			path = "body/s7"
		}
		found := false
		for _, actualSite := range p.ActualSites.Sites {
			if actualSite.ID == expected.site.ID {
				if found || actualSite.Function != expected.site.Function || actualSite.Kind != expected.site.Kind || actualSite.HIRPath != path || !reflect.DeepEqual(actualSite.Origin, expected.site.Origin) {
					return fmt.Errorf("arena append selector: proof actual site differs")
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("arena append selector: proof actual site missing")
		}
	}
	return nil
}
