package pipeline

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"reflect"
	"runtime"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

const memoryHostSearchSourceSHA = "0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526"
const MaxMemoryHostPolicySearchBytes = 2 * MaxMemoryHostPolicyAssociationBytes

// Source bytes are independently parsed to establish the exact expected HIR.
// Association slots are named and mandatory; neither policy can be omitted.
type MemoryHostPolicySearchInputs struct {
	Source        []byte
	SourceName    string
	Legacy        []byte
	PreserveSpeed []byte
}
type MemoryHostPolicySearchOptions struct {
	Goal     string `json:"goal"`
	MaxSites int    `json:"max_sites"`
}
type MemoryHostPolicySearchChoice struct {
	Policy mir2.UnsignedPairComparePolicy `json:"policy"`
	Mask   int                            `json:"mask"`
}
type MemoryHostPolicySearchCandidate struct {
	Choice  MemoryHostPolicySearchChoice     `json:"choice"`
	Choices hir.MemoryHostArenaAppendChoices `json:"choices"`
}
type MemoryHostPolicySearchEvidence struct {
	Schema                string                               `json:"schema"`
	Options               MemoryHostPolicySearchOptions        `json:"options"`
	SourceName            string                               `json:"source_name"`
	SourceSHA256          string                               `json:"source_sha256"`
	OriginalCompatibility *hir.ProfileCompatibility            `json:"original_compatibility"`
	OriginalFunctions     *hir.FunctionSiteMap                 `json:"original_functions"`
	AssociationSHA256     [2]string                            `json:"association_sha256"`
	Selections            [2]*MemoryHostArenaAppendSelectionV2 `json:"selections"`
	Candidates            []MemoryHostPolicySearchCandidate    `json:"candidates"`
	Selected              MemoryHostPolicySearchChoice         `json:"selected"`
}

// MeasureMemoryHostArenaAppendPolicySearch searches eight complete measured
// candidates for one exact source. Existing fixed-policy APIs remain unchanged.
func MeasureMemoryHostArenaAppendPolicySearch(factory func() (*hir.Module, error), w hir.MemoryHostWorkload, inputs MemoryHostPolicySearchInputs, o MemoryHostPolicySearchOptions) (*MemoryHostPolicySearchEvidence, error) {
	if factory == nil || o.MaxSites != 2 || (o.Goal != "speed" && o.Goal != "size") {
		return nil, fmt.Errorf("policy search: invalid bounded options/factory")
	}
	if len(inputs.Legacy) == 0 || len(inputs.PreserveSpeed) == 0 || len(inputs.Legacy) > MaxMemoryHostPolicyAssociationBytes || len(inputs.PreserveSpeed) > MaxMemoryHostPolicyAssociationBytes || len(inputs.Legacy) > MaxMemoryHostPolicySearchBytes-len(inputs.PreserveSpeed) {
		return nil, fmt.Errorf("policy search: two bounded association inputs required")
	}
	if len(inputs.Source) > 8<<20 || inputs.SourceName == "" || fmt.Sprintf("%x", sha256.Sum256(inputs.Source)) != memoryHostSearchSourceSHA {
		return nil, fmt.Errorf("policy search: exact original source bytes required")
	}
	sourceBytes := append([]byte{}, inputs.Source...)
	payloads := [2][]byte{append([]byte{}, inputs.Legacy...), append([]byte{}, inputs.PreserveSpeed...)}
	options := [2]MemoryHostBaselineOptions{{UnsignedPairComparePolicy: mir2.UnsignedPairCompareLegacy}, {UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed}}
	for i, payload := range payloads {
		p, e := decodePolicyAssociation(payload)
		if e != nil {
			return nil, e
		}
		if e = validatePolicyAssociationEnvelope(p, options[i]); e != nil {
			return nil, e
		}
	}
	owned, e := ownMemoryHostWorkload(w)
	if e != nil {
		return nil, e
	}
	expected, e := nanz.Parse(string(sourceBytes), inputs.SourceName)
	if e != nil {
		return nil, e
	}
	expected.Target = hir.TargetCPM
	if !expected.SourceUnitsComplete || len(expected.SourceUnits) != 1 || expected.SourceUnits[inputs.SourceName] != memoryHostSearchSourceSHA {
		return nil, fmt.Errorf("policy search: expected source units differ")
	}
	_, ctx, e := hir.PrepareMemoryHostWorkload(expected, owned)
	if e != nil {
		return nil, e
	}
	compatibility, e := hir.BuildProfileCompatibility(expected, ctx)
	if e != nil {
		return nil, e
	}
	functions, e := hir.BuildFunctionSiteMap(expected)
	if e != nil {
		return nil, e
	}
	// Bind both serialized envelopes to the independently parsed original before
	// invoking caller code. Loaded native payloads still require fresh replay.
	for i, payload := range payloads {
		if _, e = LoadMemoryHostLoopProfileEvidenceV2(bytes.NewReader(payload), expected, owned, options[i]); e != nil {
			return nil, e
		}
	}
	seen := map[uintptr]bool{}
	roots := []any{expected}
	originals := []*hir.Module{}
	defer func() { runtime.KeepAlive(roots); runtime.KeepAlive(originals) }()
	identity := func(m *hir.Module) error {
		_, c, e := hir.PrepareMemoryHostWorkload(m, owned)
		if e != nil {
			return e
		}
		k, e := hir.BuildProfileCompatibility(m, c)
		if e != nil {
			return e
		}
		f, e := hir.BuildFunctionSiteMap(m)
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(k, compatibility) || !reflect.DeepEqual(f, functions) || !reflect.DeepEqual(m.Asserts, expected.Asserts) || !reflect.DeepEqual(m.Sandboxes, expected.Sandboxes) || !reflect.DeepEqual(m.AssertStats, expected.AssertStats) || !reflect.DeepEqual(m.Warnings, expected.Warnings) {
			return fmt.Errorf("policy search: source differs from independently parsed original")
		}
		return nil
	}
	checkOriginals := func() error {
		for _, m := range originals {
			if e := identity(m); e != nil {
				return e
			}
		}
		return nil
	}
	calls := 0
	wrapped := func() (*hir.Module, error) {
		if e := checkOriginals(); e != nil {
			return nil, e
		}
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if m == nil {
			return nil, fmt.Errorf("policy search: nil source")
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		retainArenaSourceGraph(m, &roots)
		if e = identity(m); e != nil {
			return nil, e
		}
		index := calls % 24
		calls++
		if index != 2 && !(index >= 7 && index%2 == 1) {
			originals = append(originals, m)
		}
		if e = checkOriginals(); e != nil {
			return nil, e
		}
		return m, nil
	}
	out := &MemoryHostPolicySearchEvidence{Schema: "minz.memory-host-arena-append-policy-search/v1", Options: o, SourceName: inputs.SourceName, SourceSHA256: memoryHostSearchSourceSHA, OriginalCompatibility: compatibility, OriginalFunctions: functions, Candidates: []MemoryHostPolicySearchCandidate{}}
	for policy := 0; policy < 2; policy++ {
		before := calls
		selection, e := MeasureMemoryHostArenaAppendWithOptions(wrapped, owned, payloads[policy], MemoryHostArenaAppendSelectorOptions{Goal: o.Goal, MaxSites: 2}, options[policy])
		if e != nil {
			return nil, e
		}
		if calls != before+24 {
			return nil, fmt.Errorf("policy search: fixed selector lifecycle changed")
		}
		if e = checkOriginals(); e != nil {
			return nil, e
		}
		out.Selections[policy] = selection
		out.AssociationSHA256[policy] = fmt.Sprintf("%x", sha256.Sum256(payloads[policy]))
	}
	if calls != 48 {
		return nil, fmt.Errorf("policy search: incomplete lifecycle")
	}
	if e = validateMemoryHostPolicySearch(out); e != nil {
		return nil, e
	}
	if e = checkOriginals(); e != nil {
		return nil, e
	}
	return out, nil
}

// Validate all authoritative nested records before constructing the index and
// ranking it. References carry identity only; costs stay in baseline envelopes.
func validateMemoryHostPolicySearch(out *MemoryHostPolicySearchEvidence) error {
	if out == nil || out.Schema != "minz.memory-host-arena-append-policy-search/v1" || out.Options.MaxSites != 2 || (out.Options.Goal != "speed" && out.Options.Goal != "size") {
		return fmt.Errorf("policy search: invalid evidence")
	}
	for policy, selection := range out.Selections {
		options := MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairComparePolicy(policy)}
		if selection == nil || selection.Schema != "minz.memory-host-arena-append-selection/v2" || selection.CompileOptions != options || selection.Options != (MemoryHostArenaAppendSelectorOptions{Goal: out.Options.Goal, MaxSites: 2}) || len(selection.Candidates) != 4 || selection.Association == nil || selection.Profile == nil {
			return fmt.Errorf("policy search: incomplete fixed-policy selection")
		}
		if e := validatePolicyAssociationEnvelope(selection.Association, options); e != nil {
			return e
		}
		original := selection.Candidates[0].Evidence
		if original == nil || original.Baseline == nil || !reflect.DeepEqual(original.Baseline.Compatibility, out.OriginalCompatibility) || !reflect.DeepEqual(original.Baseline.FunctionSites, out.OriginalFunctions) {
			return fmt.Errorf("policy search: original source identity differs")
		}
		if !reflect.DeepEqual(selection.Association.Baseline, original) || !reflect.DeepEqual(selection.Association.Profile, selection.Profile) {
			return fmt.Errorf("policy search: original association differs")
		}
		if policy == 1 {
			first := out.Selections[0]
			if !reflect.DeepEqual(first.Profile, selection.Profile) || !reflect.DeepEqual(first.SourceSites, selection.SourceSites) {
				return fmt.Errorf("policy search: original profile differs across policies")
			}
		}
		for mask, candidate := range selection.Candidates {
			choices := hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}
			if candidate.Choices != choices {
				return fmt.Errorf("policy search: candidate choices differ")
			}
			if e := bindMemoryHostArenaAppendPolicyProof(candidate.PrecompileProof, selection.SourceSites, choices, options, original, candidate.Evidence); e != nil {
				return e
			}
			for _, observations := range [][]hir.MemoryHostObservation{candidate.Evidence.Baseline.Raw.Invocations, candidate.Evidence.Baseline.Target.Invocations} {
				for _, observation := range observations {
					if observation.Error != "" || observation.Failure != nil {
						return fmt.Errorf("policy search: failed candidate is inadmissible")
					}
				}
			}
			if policy == 1 {
				a, b := out.Selections[0].Candidates[mask].Evidence.Baseline, candidate.Evidence.Baseline
				if !reflect.DeepEqual(a.Raw, b.Raw) || !reflect.DeepEqual(a.Compatibility, b.Compatibility) || !reflect.DeepEqual(a.FunctionSites, b.FunctionSites) || !reflect.DeepEqual(a.Target.Context, b.Target.Context) {
					return fmt.Errorf("policy search: same-mask source/context differs")
				}
				if e := policySearchSameObservations(a.Target.Invocations, b.Target.Invocations); e != nil {
					return e
				}
			}
		}
	}
	out.Candidates = []MemoryHostPolicySearchCandidate{}
	out.Selected = MemoryHostPolicySearchChoice{}
	for policy, selection := range out.Selections {
		for mask, c := range selection.Candidates {
			choice := MemoryHostPolicySearchChoice{Policy: mir2.UnsignedPairComparePolicy(policy), Mask: mask}
			out.Candidates = append(out.Candidates, MemoryHostPolicySearchCandidate{Choice: choice, Choices: c.Choices})
			winner := out.Selections[int(out.Selected.Policy)].Candidates[out.Selected.Mask].Evidence.Baseline.Target
			if selectMemoryHostArenaCandidate(out.Options.Goal, winner, c.Evidence.Baseline.Target) == 1 {
				out.Selected = choice
			}
		}
	}
	return nil
}

// Compare every observable field, including error text and typed failure.
// Steps differ legitimately between backend policies.
func policySearchSameObservations(a, b []hir.MemoryHostObservation) error {
	x := append([]hir.MemoryHostObservation(nil), a...)
	y := append([]hir.MemoryHostObservation(nil), b...)
	for i := range x {
		x[i].Steps = 0
	}
	for i := range y {
		y[i].Steps = 0
	}
	if !reflect.DeepEqual(x, y) {
		return fmt.Errorf("policy search: cross-policy observations differ")
	}
	return nil
}
