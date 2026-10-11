package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"reflect"
	"runtime"
)

// Each candidate retains its mandatory policy envelope. The precompile proof
// describes source HIR identity, not a backend policy or postallocation shape.
type MemoryHostArenaAppendCandidateV2 struct {
	Choices         hir.MemoryHostArenaAppendChoices      `json:"choices"`
	PrecompileProof *MemoryHostArenaAppendPrecompileProof `json:"precompile_proof"`
	Evidence        *MemoryHostBaselineEvidenceV2         `json:"evidence"`
}

type MemoryHostArenaAppendSelectionV2 struct {
	Schema         string                               `json:"schema"`
	Options        MemoryHostArenaAppendSelectorOptions `json:"options"`
	CompileOptions MemoryHostBaselineOptions            `json:"compile_options"`
	Profile        *hir.MemoryHostLoopProfileRun        `json:"profile"`
	Association    *MemoryHostLoopProfileEvidenceV2     `json:"association"`
	SourceSites    MemoryHostArenaAppendSourceSites     `json:"source_sites"`
	Candidates     []MemoryHostArenaAppendCandidateV2   `json:"candidates"`
	Selected       int                                  `json:"selected"`
}

// MeasureMemoryHostArenaAppendWithOptions fixes one explicit codegen policy for
// all four masks. GoalSize compares measured bytes; it never enables OptSize.
// Serialized input is the whole association v2 envelope, not a bare profile.
func MeasureMemoryHostArenaAppendWithOptions(factory func() (*hir.Module, error), w hir.MemoryHostWorkload, serialized []byte, o MemoryHostArenaAppendSelectorOptions, compileOptions MemoryHostBaselineOptions) (*MemoryHostArenaAppendSelectionV2, error) {
	if _, e := resolveUnsignedPairComparePolicy(Options{UnsignedPairComparePolicy: compileOptions.UnsignedPairComparePolicy}); e != nil {
		return nil, e
	}
	ownedOptions := compileOptions
	if factory == nil || o.MaxSites != 2 || (o.Goal != "speed" && o.Goal != "size") {
		return nil, fmt.Errorf("arena append selector: invalid bounded options/factory")
	}
	if len(serialized) > MaxMemoryHostPolicyAssociationBytes {
		return nil, fmt.Errorf("arena append selector: profile size limit")
	}
	owned, e := ownMemoryHostWorkload(w)
	if e != nil {
		return nil, e
	}
	payload := append([]byte{}, serialized...)
	preflight, e := decodePolicyAssociation(payload)
	if e != nil {
		return nil, e
	}
	if e = validatePolicyAssociationEnvelope(preflight, ownedOptions); e != nil {
		return nil, e
	}
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
	loaded, e := LoadMemoryHostLoopProfileEvidenceV2(bytes.NewReader(payload), source, owned, ownedOptions)
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
		if !reflect.DeepEqual(k, loaded.Profile.Compatibility) {
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
	association, e := RunZ80MemoryHostLoopProfileWithOptions(fresh, owned, ownedOptions)
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
	if !reflect.DeepEqual(loaded, association) {
		return nil, fmt.Errorf("arena append selector: serialized counts/observations differ from fresh closed run")
	}
	pair, e := memoryHostArenaAppendSourceSites(loaded.Profile.Sites)
	if e != nil {
		return nil, e
	}
	for _, wanted := range []hir.LoopSite{pair.Arena, pair.Append} {
		observed := false
		for i, site := range loaded.Profile.Sites.Sites {
			if site.ID == wanted.ID {
				observed = loaded.Profile.Counts[i].Entries > 0
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
	out := &MemoryHostArenaAppendSelectionV2{Schema: "minz.memory-host-arena-append-selection/v2", Options: o, CompileOptions: ownedOptions, Profile: loaded.Profile, Association: association, SourceSites: pair, Candidates: []MemoryHostArenaAppendCandidateV2{{Choices: hir.MemoryHostArenaAppendChoices{}, PrecompileProof: originalProof, Evidence: association.Baseline}}}
	if e = bindMemoryHostArenaAppendPolicyProof(originalProof, pair, hir.MemoryHostArenaAppendChoices{}, ownedOptions, association.Baseline, association.Baseline); e != nil {
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
		variant, e := RunZ80MemoryHostBaselineWithOptions(variantFactory, owned, ownedOptions)
		if e != nil {
			return nil, e
		}
		if e = check(); e != nil {
			return nil, e
		}
		if calls != 6 || len(retained) != countBefore+3 {
			return nil, fmt.Errorf("arena append selector: candidate lifecycle changed")
		}
		if e = bindMemoryHostArenaAppendPolicyProof(precompileProof, pair, choices, ownedOptions, association.Baseline, variant); e != nil {
			return nil, e
		}
		if e = arenaSameObservations(association.Baseline.Baseline.Raw.Invocations, variant.Baseline.Raw.Invocations); e != nil {
			return nil, e
		}
		if e = arenaSameObservations(association.Baseline.Baseline.Target.Invocations, variant.Baseline.Target.Invocations); e != nil {
			return nil, e
		}
		out.Candidates = append(out.Candidates, MemoryHostArenaAppendCandidateV2{Choices: choices, PrecompileProof: precompileProof, Evidence: variant})
		if selectMemoryHostArenaCandidate(o.Goal, out.Candidates[out.Selected].Evidence.Baseline.Target, variant.Baseline.Target) == 1 {
			out.Selected = mask
		}
	}
	if e = check(); e != nil {
		return nil, e
	}
	return out, nil
}

func bindMemoryHostArenaAppendPolicyProof(p *MemoryHostArenaAppendPrecompileProof, sites MemoryHostArenaAppendSourceSites, choices hir.MemoryHostArenaAppendChoices, options MemoryHostBaselineOptions, original, actual *MemoryHostBaselineEvidenceV2) error {
	for _, e := range []*MemoryHostBaselineEvidenceV2{original, actual} {
		if e == nil || e.Schema != "minz.memory-host-baseline/v2" || e.CompileOptions != options || e.Baseline == nil || e.Baseline.Schema != "minz.memory-host-baseline/v1" || e.Baseline.Raw == nil || e.Baseline.Target == nil {
			return fmt.Errorf("arena append policy selector: candidate policy envelope differs")
		}
	}
	return bindMemoryHostArenaAppendProof(p, sites, choices, original.Baseline, actual.Baseline)
}
