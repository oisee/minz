package pipeline

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"

	"github.com/minz/minzc/pkg/hir"
)

type MemoryHostAppendSelectorOptions struct {
	Goal     string `json:"goal"`
	MaxSites int    `json:"max_sites"`
}
type MemoryHostAppendPrecompileProof struct {
	Schema                string                    `json:"schema"`
	Function              string                    `json:"function"`
	SourceSite            hir.LoopSite              `json:"source_site"`
	OriginalCompatibility *hir.ProfileCompatibility `json:"original_compatibility"`
	ActualCompatibility   *hir.ProfileCompatibility `json:"actual_compatibility"`
	OriginalFunctions     *hir.FunctionSiteMap      `json:"original_functions"`
	ActualFunctions       *hir.FunctionSiteMap      `json:"actual_functions"`
	ActualSites           *hir.LoopSiteMap          `json:"actual_sites"`
}

type MemoryHostAppendCandidate struct {
	Hoisted         bool                             `json:"hoisted"`
	PrecompileProof *MemoryHostAppendPrecompileProof `json:"precompile_proof"`
	Evidence        *MemoryHostBaselineEvidence      `json:"evidence"`
}
type MemoryHostAppendSelection struct {
	Schema      string                          `json:"schema"`
	Options     MemoryHostAppendSelectorOptions `json:"options"`
	Profile     *hir.MemoryHostLoopProfileRun   `json:"profile"`
	Association *MemoryHostLoopProfileEvidence  `json:"association"`
	SiteID      string                          `json:"site_id"`
	Candidates  []MemoryHostAppendCandidate     `json:"candidates"`
	Selected    int                             `json:"selected"`
}

// MeasureMemoryHostAppendCopyHoist is bounded to one exact source helper/two variants.
// Goal compares measured costs; it does not change compiler options. Candidate
// construction is private and its proof applies before compiler preparation.
func MeasureMemoryHostAppendCopyHoist(factory func() (*hir.Module, error), w hir.MemoryHostWorkload, serialized []byte, o MemoryHostAppendSelectorOptions) (*MemoryHostAppendSelection, error) {
	if factory == nil || o.MaxSites != 1 || (o.Goal != "speed" && o.Goal != "size") {
		return nil, fmt.Errorf("append selector: invalid bounded options/factory")
	}
	if len(serialized) > hir.MaxMemoryHostLoopProfileBytes {
		return nil, fmt.Errorf("append selector: profile size limit")
	}
	owned, e := ownMemoryHostWorkload(w)
	if e != nil {
		return nil, e
	}
	payload := append([]byte{}, serialized...)
	seen := map[uintptr]bool{}
	retained := []*hir.Module{}
	privateRoots := []any{}
	defer func() { runtime.KeepAlive(privateRoots); runtime.KeepAlive(retained) }()
	fresh := func() (*hir.Module, error) {
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if m == nil {
			return nil, fmt.Errorf("append selector: nil source")
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
			return fmt.Errorf("append selector: retained source identity changed")
		}
		fs, e := hir.BuildFunctionSiteMap(m)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(fs, expectedFunctions) {
			return fmt.Errorf("append selector: function/origin identity changed")
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
		return nil, fmt.Errorf("append selector: association factory lifecycle changed")
	}
	excludedCompiled[retained[2]] = true
	if e = check(); e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(loaded, association.Profile) {
		return nil, fmt.Errorf("append selector: serialized counts/observations differ from fresh closed run")
	}
	site := ""
	for i, s := range loaded.Sites.Sites {
		if s.Function == "intern_region" && s.HIRPath == "body/s5" {
			if site != "" || loaded.Counts[i].Entries == 0 {
				return nil, fmt.Errorf("append selector: unique observed helper required")
			}
			site = s.ID
		}
	}
	if site == "" {
		return nil, fmt.Errorf("append selector: helper site absent")
	}
	// Candidate runner remains private: caller factory only returns originals.
	// Each original is checked and retained separately from the mutated candidate.
	var precompileProof *MemoryHostAppendPrecompileProof
	variantFactory := func() (*hir.Module, error) {
		original, e := fresh()
		if e != nil {
			return nil, e
		}
		if e = identity(original); e != nil {
			return nil, e
		}
		actual, e := factory()
		if e != nil {
			return nil, e
		}
		if actual == nil {
			return nil, fmt.Errorf("append selector: nil candidate source")
		}
		if e = jointCheckFreshHIR(actual, seen); e != nil {
			return nil, e
		}
		if e = identity(actual); e != nil {
			return nil, e
		}
		retainArenaSourceGraph(actual, &privateRoots)
		if e = hoistMemoryHostAppendBases(actual, owned); e != nil {
			return nil, e
		}
		if e = hir.ValidateMemoryHostAppendCopyHoist(original, actual, owned); e != nil {
			return nil, e
		}
		proof, e := buildMemoryHostAppendPrecompileProof(original, actual, owned, site)
		if e != nil {
			return nil, e
		}
		if precompileProof == nil {
			precompileProof = proof
		} else if !reflect.DeepEqual(precompileProof, proof) {
			return nil, fmt.Errorf("append selector: precompile proof drift")
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
	if precompileProof == nil || !reflect.DeepEqual(precompileProof.OriginalCompatibility, association.Baseline.Compatibility) || !reflect.DeepEqual(precompileProof.ActualCompatibility, variant.Compatibility) || !reflect.DeepEqual(precompileProof.OriginalFunctions, association.Baseline.FunctionSites) || !reflect.DeepEqual(precompileProof.ActualFunctions, variant.FunctionSites) {
		return nil, fmt.Errorf("append selector: measured baseline differs from precompile proof")
	}
	var selectedSource hir.LoopSite
	for _, s := range loaded.Sites.Sites {
		if s.ID == site {
			selectedSource = s
		}
	}
	if !reflect.DeepEqual(precompileProof.SourceSite, selectedSource) {
		return nil, fmt.Errorf("append selector: loaded source site differs from proof")
	}
	if e = arenaSameObservations(association.Baseline.Raw.Invocations, variant.Raw.Invocations); e != nil {
		return nil, e
	}
	if e = arenaSameObservations(association.Baseline.Target.Invocations, variant.Target.Invocations); e != nil {
		return nil, e
	}
	out := &MemoryHostAppendSelection{Schema: "minz.memory-host-append-selection/v1", Options: o, Profile: loaded, Association: association, SiteID: site, Candidates: []MemoryHostAppendCandidate{{Evidence: association.Baseline}, {Hoisted: true, PrecompileProof: precompileProof, Evidence: variant}}}
	a, b := association.Baseline.Target, variant.Target
	out.Selected = selectMemoryHostArenaCandidate(o.Goal, a, b)
	runtime.KeepAlive(privateRoots)
	runtime.KeepAlive(retained)
	return out, nil
}

// Captured independently validated precompile identities. This record contains
// no claim that the same HIR shape survives compiler splitting or lowering.
func buildMemoryHostAppendPrecompileProof(source, actual *hir.Module, w hir.MemoryHostWorkload, siteID string) (*MemoryHostAppendPrecompileProof, error) {
	if e := hir.ValidateMemoryHostAppendCopyHoist(source, actual, w); e != nil {
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
	var sourceSite hir.LoopSite
	found := false
	for _, s := range ss.Sites {
		if s.ID == siteID {
			sourceSite = s
			found = true
		}
	}
	if !found || sourceSite.Function != "intern_region" || sourceSite.HIRPath != "body/s5" {
		return nil, fmt.Errorf("append selector: unexpected source proof site")
	}
	appendFound := false
	for _, s := range as.Sites {
		if s.ID == siteID {
			if s.Function != "intern_region" || s.HIRPath != "body/s7" || !reflect.DeepEqual(s.Origin, sourceSite.Origin) {
				return nil, fmt.Errorf("append selector: actual append site differs")
			}
			appendFound = true
		}
	}
	if !appendFound {
		return nil, fmt.Errorf("append selector: missing actual append site")
	}
	return &MemoryHostAppendPrecompileProof{Schema: "minz.exact-memory-host-append-hoist-precompile/v1", Function: "intern_region", SourceSite: sourceSite, OriginalCompatibility: sk, ActualCompatibility: ak, OriginalFunctions: sf, ActualFunctions: af, ActualSites: as}, nil
}
