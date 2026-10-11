package pipeline

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"

	"github.com/minz/minzc/pkg/hir"
)

type MemoryHostArenaSelectorOptions struct {
	Goal     string `json:"goal"`
	MaxSites int    `json:"max_sites"`
}
type MemoryHostArenaPrecompileProof struct {
	Schema                string                    `json:"schema"`
	Function              string                    `json:"function"`
	SourceSite            hir.LoopSite              `json:"source_site"`
	OriginalCompatibility *hir.ProfileCompatibility `json:"original_compatibility"`
	ActualCompatibility   *hir.ProfileCompatibility `json:"actual_compatibility"`
	OriginalFunctions     *hir.FunctionSiteMap      `json:"original_functions"`
	ActualFunctions       *hir.FunctionSiteMap      `json:"actual_functions"`
	ActualSites           *hir.LoopSiteMap          `json:"actual_sites"`
}

type MemoryHostArenaCandidate struct {
	Peeled          bool                            `json:"peeled"`
	PrecompileProof *MemoryHostArenaPrecompileProof `json:"precompile_proof"`
	Evidence        *MemoryHostBaselineEvidence     `json:"evidence"`
}
type MemoryHostArenaSelection struct {
	Schema      string                         `json:"schema"`
	Options     MemoryHostArenaSelectorOptions `json:"options"`
	Profile     *hir.MemoryHostLoopProfileRun  `json:"profile"`
	Association *MemoryHostLoopProfileEvidence `json:"association"`
	SiteID      string                         `json:"site_id"`
	Candidates  []MemoryHostArenaCandidate     `json:"candidates"`
	Selected    int                            `json:"selected"`
}

// MeasureMemoryHostArenaPeel is bounded to one exact source helper/two variants.
// Goal compares measured costs; it does not change compiler options. Candidate
// construction is private and its proof applies before compiler preparation.
func MeasureMemoryHostArenaPeel(factory func() (*hir.Module, error), w hir.MemoryHostWorkload, serialized []byte, o MemoryHostArenaSelectorOptions) (*MemoryHostArenaSelection, error) {
	if factory == nil || o.MaxSites != 1 || (o.Goal != "speed" && o.Goal != "size") {
		return nil, fmt.Errorf("arena selector: invalid bounded options/factory")
	}
	if len(serialized) > hir.MaxMemoryHostLoopProfileBytes {
		return nil, fmt.Errorf("arena selector: profile size limit")
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
			return nil, fmt.Errorf("arena selector: nil source")
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
		_, ctx, e := hir.PrepareMemoryHostWorkload(m, owned)
		if e != nil {
			return e
		}
		ctx.SemanticsID += ":" + hir.MemoryHostLoopProfileSemantics
		k, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(k, loaded.Compatibility) {
			return fmt.Errorf("arena selector: retained source identity changed")
		}
		fs, e := hir.BuildFunctionSiteMap(m)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(fs, expectedFunctions) {
			return fmt.Errorf("arena selector: function/origin identity changed")
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
		return nil, fmt.Errorf("arena selector: association factory lifecycle changed")
	}
	excludedCompiled[retained[2]] = true
	if e = check(); e != nil {
		return nil, e
	}
	if !reflect.DeepEqual(loaded, association.Profile) {
		return nil, fmt.Errorf("arena selector: serialized counts/observations differ from fresh closed run")
	}
	site := ""
	for i, s := range loaded.Sites.Sites {
		if s.Function == "region_match" {
			if site != "" || loaded.Counts[i].Entries == 0 {
				return nil, fmt.Errorf("arena selector: unique observed helper required")
			}
			site = s.ID
		}
	}
	if site == "" {
		return nil, fmt.Errorf("arena selector: helper site absent")
	}
	// Candidate runner remains private: caller factory only returns originals.
	// Each original is checked and retained separately from the mutated candidate.
	var precompileProof *MemoryHostArenaPrecompileProof
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
			return nil, fmt.Errorf("arena selector: nil candidate source")
		}
		if e = jointCheckFreshHIR(actual, seen); e != nil {
			return nil, e
		}
		if e = identity(actual); e != nil {
			return nil, e
		}
		retainArenaSourceGraph(actual, &privateRoots)
		if e = peelMemoryHostArenaFirstByte(actual, owned); e != nil {
			return nil, e
		}
		if e = hir.ValidateMemoryHostArenaPeel(original, actual, owned); e != nil {
			return nil, e
		}
		proof, e := buildMemoryHostArenaPrecompileProof(original, actual, owned, site)
		if e != nil {
			return nil, e
		}
		if precompileProof == nil {
			precompileProof = proof
		} else if !reflect.DeepEqual(precompileProof, proof) {
			return nil, fmt.Errorf("arena selector: precompile proof drift")
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
	if e = arenaSameObservations(association.Baseline.Raw.Invocations, variant.Raw.Invocations); e != nil {
		return nil, e
	}
	if e = arenaSameObservations(association.Baseline.Target.Invocations, variant.Target.Invocations); e != nil {
		return nil, e
	}
	out := &MemoryHostArenaSelection{Schema: "minz.memory-host-arena-selection/v1", Options: o, Profile: loaded, Association: association, SiteID: site, Candidates: []MemoryHostArenaCandidate{{Evidence: association.Baseline}, {Peeled: true, PrecompileProof: precompileProof, Evidence: variant}}}
	a, b := association.Baseline.Target, variant.Target
	out.Selected = selectMemoryHostArenaCandidate(o.Goal, a, b)
	runtime.KeepAlive(privateRoots)
	runtime.KeepAlive(retained)
	return out, nil
}
func arenaSameObservations(a, b []hir.MemoryHostObservation) error {
	if len(a) != len(b) {
		return fmt.Errorf("arena selector: invocation mismatch")
	}
	for i := range a {
		x, y := a[i], b[i]
		x.Steps, y.Steps = 0, 0
		if (x.Error == "") != (y.Error == "") {
			return fmt.Errorf("arena selector: failure mismatch")
		}
		x.Error, y.Error = "", ""
		if !reflect.DeepEqual(x, y) {
			return fmt.Errorf("arena selector: observation mismatch %d", i)
		}
	}
	return nil
}

// Keep every formerly owned node/backing allocation alive even if normal
// compiler preparation overwrites the pointers in its original root object.
func retainArenaSourceGraph(m *hir.Module, roots *[]any) {
	visited := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		switch v.Kind() {
		case reflect.Pointer, reflect.Map:
			if v.IsNil() || visited[v.Pointer()] {
				return
			}
			visited[v.Pointer()] = true
			if v.CanInterface() {
				*roots = append(*roots, v.Interface())
			}
			if v.Kind() == reflect.Pointer {
				walk(v.Elem())
			} else {
				it := v.MapRange()
				for it.Next() {
					walk(it.Key())
					walk(it.Value())
				}
			}
		case reflect.Slice:
			if v.CanInterface() {
				*roots = append(*roots, v.Interface())
			}
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Field(i).CanInterface() {
					walk(v.Field(i))
				}
			}
		}
	}
	walk(reflect.ValueOf(m))
}

// Equal primary measured costs retain original. This helper never constructs
// evidence; callers of the public API always provide two actual target runs.
func selectMemoryHostArenaCandidate(goal string, a, b *MemoryHostTargetEvidence) int {
	if goal == "speed" && b.TotalTStates < a.TotalTStates || goal == "size" && len(b.Binary) < len(a.Binary) {
		return 1
	}
	return 0
}

// Captured independently validated precompile identities. This record contains
// no claim that the same HIR shape survives compiler splitting or lowering.
func buildMemoryHostArenaPrecompileProof(source, actual *hir.Module, w hir.MemoryHostWorkload, siteID string) (*MemoryHostArenaPrecompileProof, error) {
	if e := hir.ValidateMemoryHostArenaPeel(source, actual, w); e != nil {
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
	if !found || sourceSite.Function != "region_match" || sourceSite.HIRPath != "body/s1" {
		return nil, fmt.Errorf("arena selector: unexpected source proof site")
	}
	return &MemoryHostArenaPrecompileProof{Schema: "minz.exact-memory-host-arena-peel-precompile/v1", Function: "region_match", SourceSite: sourceSite, OriginalCompatibility: sk, ActualCompatibility: ak, OriginalFunctions: sf, ActualFunctions: af, ActualSites: as}, nil
}
