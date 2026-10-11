package pipeline

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func policySelectorInput(t *testing.T, w hir.MemoryHostWorkload, options MemoryHostBaselineOptions) []byte {
	t.Helper()
	e, err := RunZ80MemoryHostLoopProfileWithOptions(func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }, w, options)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMemoryHostPolicySelectorAdmission(t *testing.T) {
	w := arenaAppendSmallWorkload(t)
	calls := 0
	factory := func() (*hir.Module, error) { calls++; m, _ := memoryHostParserFixture(t); return m, nil }
	valid := MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}
	for _, o := range []MemoryHostArenaAppendSelectorOptions{{Goal: "other", MaxSites: 2}, {Goal: "size", MaxSites: 1}} {
		if e, err := MeasureMemoryHostArenaAppendWithOptions(factory, w, nil, o, MemoryHostBaselineOptions{}); e != nil || err == nil || calls != 0 {
			t.Fatal("invalid bounded options reached factory", err, calls)
		}
	}
	if e, err := MeasureMemoryHostArenaAppendWithOptions(factory, w, nil, valid, MemoryHostBaselineOptions{UnsignedPairComparePolicy: 255}); e != nil || err == nil || calls != 0 {
		t.Fatal("unknown policy reached factory", err, calls)
	}
	if e, err := MeasureMemoryHostArenaAppendWithOptions(factory, w, make([]byte, MaxMemoryHostPolicyAssociationBytes+1), valid, MemoryHostBaselineOptions{}); e != nil || err == nil || calls != 0 {
		t.Fatal("oversized input reached factory", err, calls)
	}
}

func TestMemoryHostPolicySelectorFixedPolicy(t *testing.T) {
	w := arenaAppendSmallWorkload(t)
	for _, policy := range []mir2.UnsignedPairComparePolicy{mir2.UnsignedPairCompareLegacy, mir2.UnsignedPairComparePreserveForSpeed} {
		options := MemoryHostBaselineOptions{UnsignedPairComparePolicy: policy}
		b := policySelectorInput(t, w, options)
		for _, goal := range []string{"speed", "size"} {
			t.Run(fmt.Sprintf("%d/%s", policy, goal), func(t *testing.T) {
				calls := 0
				factory := func() (*hir.Module, error) { calls++; runtime.GC(); m, _ := memoryHostParserFixture(t); return m, nil }
				e, err := MeasureMemoryHostArenaAppendWithOptions(factory, w, b, MemoryHostArenaAppendSelectorOptions{Goal: goal, MaxSites: 2}, options)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 24 || len(e.Candidates) != 4 || e.Schema != "minz.memory-host-arena-append-selection/v2" || e.CompileOptions != options {
					t.Fatal("bounded lifecycle/envelope", calls)
				}
				want := 0
				for mask, c := range e.Candidates {
					choices := hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}
					if c.Choices != choices || bindMemoryHostArenaAppendPolicyProof(c.PrecompileProof, e.SourceSites, choices, options, e.Candidates[0].Evidence, c.Evidence) != nil {
						t.Fatal("candidate proof or policy")
					}
					if selectMemoryHostArenaCandidate(goal, e.Candidates[want].Evidence.Baseline.Target, c.Evidence.Baseline.Target) == 1 {
						want = mask
					}
				}
				if e.Selected != want {
					t.Fatal("objective uses actual fixed-policy candidates")
				}
				// GoalSize is valid with policy1; it selects by bytes without OptSize.
				encoded, _ := json.Marshal(e)
				if !strings.Contains(string(encoded), `"unsigned_pair_compare_policy":`+fmt.Sprint(policy)) {
					t.Fatal("explicit policy missing")
				}
			})
		}
	}
}

func TestMemoryHostPolicySelectorReplayAndPolicyRefusals(t *testing.T) {
	w := arenaAppendSmallWorkload(t)
	options := MemoryHostBaselineOptions{}
	b := policySelectorInput(t, w, options)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	for _, kind := range []string{"requested-policy", "association-policy", "nested-policy", "bare-profile", "forged-counts", "workload"} {
		t.Run(kind, func(t *testing.T) {
			var a MemoryHostLoopProfileEvidenceV2
			if err := json.Unmarshal(b, &a); err != nil {
				t.Fatal(err)
			}
			requested := options
			workload := w
			payload := b
			switch kind {
			case "requested-policy":
				requested.UnsignedPairComparePolicy = mir2.UnsignedPairComparePreserveForSpeed
			case "association-policy":
				a.CompileOptions.UnsignedPairComparePolicy = mir2.UnsignedPairComparePreserveForSpeed
			case "nested-policy":
				a.Baseline.CompileOptions.UnsignedPairComparePolicy = mir2.UnsignedPairComparePreserveForSpeed
			case "bare-profile":
				payload, _ = json.Marshal(a.Profile)
			case "forged-counts":
				for i, s := range a.Profile.Sites.Sites {
					if s.Function == "intern_region" && s.HIRPath == "body/s5" {
						a.Profile.Counts[i].Entries++
						a.Profile.Invocations[1].Counts[i].Entries++
					}
				}
			case "workload":
				workload.Calls = append([]hir.MemoryHostInvocation{}, w.Calls...)
				workload.Calls[0].Arguments = []int64{0x8000, 0, 1}
			}
			if kind != "bare-profile" {
				payload, _ = json.Marshal(a)
			}
			e, err := MeasureMemoryHostArenaAppendWithOptions(factory, workload, payload, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}, requested)
			if e != nil || err == nil {
				t.Fatal("accepted", kind)
			}
			if kind == "forged-counts" && !strings.Contains(err.Error(), "serialized counts") {
				t.Fatal("forgery did not reach replay gate", err)
			}
		})
	}
}

func TestMemoryHostPolicySelectorLateMutationAndSharing(t *testing.T) {
	w := arenaAppendSmallWorkload(t)
	options := MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed}
	b := policySelectorInput(t, w, options)
	for _, phase := range []int{6, 12, 18, 24} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			calls := 0
			var first *hir.Module
			factory := func() (*hir.Module, error) {
				m, _ := memoryHostParserFixture(t)
				calls++
				if calls == 1 {
					first = m
				}
				if calls == phase {
					stmt := first.FuncByName("intern_region").Body.Body[5]
					loc := first.SourceLocations[stmt]
					loc.ByteOffset++
					first.SourceLocations[stmt] = loc
				}
				return m, nil
			}
			e, err := MeasureMemoryHostArenaAppendWithOptions(factory, w, b, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}, options)
			if e != nil || err == nil || calls < phase {
				t.Fatal("late retained-origin mutation", phase, calls, err)
			}
		})
	}
	calls := 0
	var first *hir.Module
	factory := func() (*hir.Module, error) {
		m, _ := memoryHostParserFixture(t)
		calls++
		if calls == 1 {
			first = m
		}
		if calls == 14 {
			runtime.GC()
			m.FuncByName("region_match").Body = first.FuncByName("region_match").Body
		}
		return m, nil
	}
	if e, err := MeasureMemoryHostArenaAppendWithOptions(factory, w, b, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}, options); e != nil || err == nil || calls != 14 {
		t.Fatal("cross-mask shared graph", calls, err)
	}
}

func TestMemoryHostPolicySelectorProofEnvelope(t *testing.T) {
	source, w := memoryHostParserFixture(t)
	actual, _ := memoryHostParserFixture(t)
	choices := hir.MemoryHostArenaAppendChoices{ArenaPeel: true, AppendHoist: true}
	if err := constructMemoryHostArenaAppend(actual, w, true, true); err != nil {
		t.Fatal(err)
	}
	p, err := buildMemoryHostArenaAppendPrecompileProof(source, actual, w, choices)
	if err != nil {
		t.Fatal(err)
	}
	options := MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed}
	envelope := func(k *hir.ProfileCompatibility, f *hir.FunctionSiteMap) *MemoryHostBaselineEvidenceV2 {
		return &MemoryHostBaselineEvidenceV2{Schema: "minz.memory-host-baseline/v2", CompileOptions: options, Baseline: &MemoryHostBaselineEvidence{Schema: "minz.memory-host-baseline/v1", Compatibility: k, FunctionSites: f, Raw: &hir.MemoryHostRun{}, Target: &MemoryHostTargetEvidence{}}}
	}
	original := envelope(p.OriginalCompatibility, p.OriginalFunctions)
	variant := envelope(p.ActualCompatibility, p.ActualFunctions)
	if err := bindMemoryHostArenaAppendPolicyProof(p, p.SourceSites, choices, options, original, variant); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"policy", "schema", "missing-baseline", "actual-identity", "actual-site"} {
		t.Run(kind, func(t *testing.T) {
			data, _ := json.Marshal(variant)
			var changed MemoryHostBaselineEvidenceV2
			json.Unmarshal(data, &changed)
			proofData, _ := json.Marshal(p)
			var proof MemoryHostArenaAppendPrecompileProof
			json.Unmarshal(proofData, &proof)
			switch kind {
			case "policy":
				changed.CompileOptions.UnsignedPairComparePolicy = 0
			case "schema":
				changed.Schema = "minz.memory-host-baseline/v1"
			case "missing-baseline":
				changed.Baseline = nil
			case "actual-identity":
				changed.Baseline.FunctionSites = original.Baseline.FunctionSites
			case "actual-site":
				for i, s := range proof.ActualSites.Sites {
					if s.ID == proof.SourceSites.Append.ID {
						proof.ActualSites.Sites[i].HIRPath = "body/s1"
					}
				}
			}
			if kind == "actual-identity" && reflect.DeepEqual(changed.Baseline.FunctionSites, p.ActualFunctions) {
				t.Fatal("fixture needs distinct function identity")
			}
			if bindMemoryHostArenaAppendPolicyProof(&proof, p.SourceSites, choices, options, original, &changed) == nil {
				t.Fatal("forged envelope/proof accepted")
			}
		})
	}
}
