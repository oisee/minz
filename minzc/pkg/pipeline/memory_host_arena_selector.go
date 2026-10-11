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
type MemoryHostArenaCandidate struct {
	Peeled          bool                        `json:"peeled"`
	PrecompileProof string                      `json:"precompile_proof"`
	Evidence        *MemoryHostBaselineEvidence `json:"evidence"`
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
	owned, e := ownMemoryHostWorkload(w)
	if e != nil {
		return nil, e
	}
	payload := append([]byte{}, serialized...)
	seen := map[uintptr]bool{}
	retained := []*hir.Module{}
	privateRoots := []any{}
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
		// Preserve the original pointer graph before transformation/compilation
		// drops old bodies, expressions or backing slices. Address-only freshness
		// sets must never encounter GC reuse of formerly owned source nodes.
		root := *actual
		root.Funcs = make([]*hir.Func, len(actual.Funcs))
		for i, f := range actual.Funcs {
			c := *f
			root.Funcs[i] = &c
		}
		privateRoots = append(privateRoots, &root, actual)
		var retainPointers func(reflect.Value)
		visited := map[uintptr]bool{}
		retainPointers = func(v reflect.Value) {
			if !v.IsValid() {
				return
			}
			if v.Kind() == reflect.Interface {
				if !v.IsNil() {
					retainPointers(v.Elem())
				}
				return
			}
			switch v.Kind() {
			case reflect.Pointer:
				if v.IsNil() || visited[v.Pointer()] {
					return
				}
				visited[v.Pointer()] = true
				if v.CanInterface() {
					privateRoots = append(privateRoots, v.Interface())
				}
				retainPointers(v.Elem())
			case reflect.Map:
				if v.IsNil() || visited[v.Pointer()] {
					return
				}
				visited[v.Pointer()] = true
				if v.CanInterface() {
					privateRoots = append(privateRoots, v.Interface())
				}
				it := v.MapRange()
				for it.Next() {
					retainPointers(it.Key())
					retainPointers(it.Value())
				}
			case reflect.Slice:
				if v.CanInterface() {
					privateRoots = append(privateRoots, v.Interface())
				}
				for i := 0; i < v.Len(); i++ {
					retainPointers(v.Index(i))
				}
			case reflect.Array:
				for i := 0; i < v.Len(); i++ {
					retainPointers(v.Index(i))
				}
			case reflect.Struct:
				for i := 0; i < v.NumField(); i++ {
					if v.Field(i).CanInterface() {
						retainPointers(v.Field(i))
					}
				}
			}
		}
		retainPointers(reflect.ValueOf(actual))
		if e = peelMemoryHostArenaFirstByte(actual, owned); e != nil {
			return nil, e
		}
		if e = hir.ValidateMemoryHostArenaPeel(original, actual, owned); e != nil {
			return nil, e
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
	out := &MemoryHostArenaSelection{Schema: "minz.memory-host-arena-selection/v1", Options: o, Profile: loaded, Association: association, SiteID: site, Candidates: []MemoryHostArenaCandidate{{Evidence: association.Baseline}, {Peeled: true, PrecompileProof: "minz.exact-memory-host-arena-peel/v1", Evidence: variant}}}
	a, b := association.Baseline.Target, variant.Target
	if o.Goal == "speed" && b.TotalTStates < a.TotalTStates || o.Goal == "size" && len(b.Binary) < len(a.Binary) {
		out.Selected = 1
	}
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
