package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
	"runtime"
	"sort"
)

type OrderedByteRWFunctionCandidate struct {
	RequestedSites map[string]hir.FunctionChoice `json:"requested_functions"`
	Proofs         []hir.OrderedByteRWProof      `json:"proofs"`
	FinalProofs    []hir.OrderedByteRWProof      `json:"postcompile_proofs"`
	HIR            string                        `json:"hir"`
	FinalHIR       string                        `json:"postcompile_hir"`
	RawRun         *hir.ProfileRun               `json:"raw_run"`
	FinalRawRun    *hir.ProfileRun               `json:"postcompile_raw_run"`
	Run            *Z80ProfileWorkloadEvidence   `json:"target_run"`
	Binary         []byte                        `json:"binary"`
	BinarySHA256   string                        `json:"binary_sha256"`
	ModuleBytes    int                           `json:"module_bytes"`
	TotalTStates   int64                         `json:"total_t_states"`
}
type OrderedByteRWFunctionEvidence struct {
	Schema        string                           `json:"schema"`
	Compatibility *hir.ProfileCompatibility        `json:"compatibility"`
	CompilationID string                           `json:"compilation_id"`
	Options       Options                          `json:"options"`
	MaxSites      int                              `json:"max_functions"`
	EligibleSites []hir.OrderedByteRWFunctionSite  `json:"eligible_functions"`
	FunctionSites *hir.FunctionSiteMap             `json:"function_sites"`
	Oracle        *hir.ProfileRun                  `json:"oracle"`
	Candidates    []OrderedByteRWFunctionCandidate `json:"candidates"`
	Selected      int                              `json:"selected"`
	SHA256        string                           `json:"sha256,omitempty"`
}
type OrderedByteRWFunctionResult struct {
	Steps    Steps
	Binary   []byte
	Evidence *OrderedByteRWFunctionEvidence
}

// MeasureOrderedByteRWFunctionVariantSets measures all combinations of at most
// four legal source-bound helpers on the complete controlled workload. Loop
// observations provide replay freshness only. The all-original candidate wins
// ties, including unobserved helper choices. Other HIR rules remain disabled.
func MeasureOrderedByteRWFunctionVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*OrderedByteRWFunctionResult, error) {
	fail := func(err error) (*OrderedByteRWFunctionResult, error) {
		return nil, fmt.Errorf("measure ordered byte read/write functions: %w", err)
	}
	if factory == nil || maxSites < 1 || maxSites > 4 || options.ProfileSelection != nil || options.SkipZ80Emission || (options.Backend != "" && options.Backend != "z80") {
		return fail(fmt.Errorf("unsupported factory/backend/configuration or cap"))
	}
	config, err := ProfileCompilationID(options, "default")
	if err != nil {
		return fail(err)
	}
	data, err := json.Marshal(workload)
	if err != nil {
		return fail(err)
	}
	var owned hir.ProfileWorkload
	if err := json.Unmarshal(data, &owned); err != nil {
		return fail(err)
	}
	workload = owned
	seen := map[uintptr]bool{}
	retained := []*hir.Module{}
	var key *hir.ProfileCompatibility
	var inputSites *hir.FunctionSiteMap
	clean := func(m *hir.Module) bool {
		return m != nil && m.OrderedByteRWPolicy == nil && m.OrderedBytePolicy == nil && m.RegionComparePolicy == nil && m.ForwardCopyPolicy == nil && m.PointerWalkPolicy == nil && m.GraceRulePolicy == nil && m.InvariantBasePolicy == nil && m.LoopLineage == nil
	}
	fresh := func() (*hir.Module, error) {
		m, err := factory()
		if err != nil {
			return nil, err
		}
		if !clean(m) {
			return nil, fmt.Errorf("factory must return fresh untransformed input")
		}
		if err := jointCheckFreshHIR(m, seen); err != nil {
			return nil, err
		}
		c, err := hir.BuildWorkloadProfileContext(m, workload)
		if err != nil {
			return nil, err
		}
		k, err := hir.BuildProfileCompatibility(m, c)
		if err != nil {
			return nil, err
		}
		if key == nil {
			key = k
		} else if !jointEqual(k, key) {
			return nil, fmt.Errorf("factory input identity changed")
		}
		sites, err := hir.BuildFunctionSiteMap(m)
		if err != nil {
			return nil, err
		}
		if inputSites == nil {
			inputSites = sites
		} else if !jointEqual(sites, inputSites) {
			return nil, fmt.Errorf("factory function/source-origin identity changed")
		}
		retained = append(retained, m)
		return m, nil
	}
	source, err := fresh()
	if err != nil {
		return fail(err)
	}
	profile, err := hir.LoadLoopProfile(bytes.NewReader(observations), source, key.Context)
	if err != nil {
		return fail(err)
	}
	sites, err := hir.BuildFunctionSiteMap(source)
	if err != nil {
		return fail(err)
	}
	legal, err := hir.EligibleOrderedByteRWHelpers(source)
	if err != nil {
		return fail(err)
	}
	// Function identities are separate from loop sites. Every legal helper is
	// measured, including helpers not called by this workload; no invocation
	// counts are inferred from the loop observation record.
	eligible := append([]hir.OrderedByteRWFunctionSite{}, legal...)
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	if len(eligible) > maxSites {
		return fail(fmt.Errorf("legal ordered byte helpers exceed cap"))
	}
	modules := make([]*hir.Module, 1<<len(eligible))
	for i := range modules {
		modules[i], err = fresh()
		if err != nil {
			return fail(err)
		}
	}
	for _, m := range retained {
		sites, err := hir.BuildFunctionSiteMap(m)
		if err != nil || !jointEqual(sites, inputSites) {
			return fail(fmt.Errorf("factory mutated previous function/source-origin identity: %v", err))
		}
		if !clean(m) {
			return fail(fmt.Errorf("factory mutated previous input policy"))
		}
		c, err := hir.BuildWorkloadProfileContext(m, workload)
		if err != nil {
			return fail(err)
		}
		k, err := hir.BuildProfileCompatibility(m, c)
		if err != nil || !jointEqual(k, key) {
			return fail(fmt.Errorf("factory mutated previous source/HIR: %v", err))
		}
	}
	oracle, err := hir.RunLoopProfile(source, workload)
	if err != nil {
		return fail(err)
	}
	if !jointEqual(profile.Counts, jointSortedCounts(oracle.Profile.Counts)) || !jointEqual(jointSortedGuards(profile.Guards), jointSortedGuards(oracle.Profile.Guards)) || profile.CompletedCalls != oracle.Profile.CompletedCalls {
		return fail(fmt.Errorf("observations differ from controlled replay"))
	}
	e := &OrderedByteRWFunctionEvidence{Schema: "minz.ordered-byte-rw-function-variant-sets/v1", Compatibility: key, CompilationID: config, Options: options, MaxSites: maxSites, EligibleSites: eligible, FunctionSites: sites, Oracle: oracle, Candidates: []OrderedByteRWFunctionCandidate{}}
	e.Options.GraceStats = nil
	check := func(m *hir.Module, choices map[string]hir.FunctionChoice) ([]hir.OrderedByteRWProof, error) {
		proofs, err := hir.ValidateOrderedByteRWChoices(source, m, choices)
		if err != nil {
			return nil, err
		}
		return proofs, nil
	}
	compiled := make([]Steps, len(modules))
	for mask, m := range modules {
		choices := map[string]hir.FunctionChoice{}
		for j, site := range eligible {
			choices[site.ID] = hir.FunctionChoice{Kind: site.Kind, Enabled: mask&(1<<(len(eligible)-1-j)) != 0}
		}
		if err := hir.ApplyOrderedByteRWChoices(m, key, choices); err != nil {
			return fail(err)
		}
		proofs, err := check(m, choices)
		if err != nil {
			return fail(err)
		}
		raw, err := jointRunRaw(m, workload)
		if err != nil || !jointSameState(raw, oracle) {
			return fail(fmt.Errorf("candidate%d raw oracle mismatch: %v", mask, err))
		}
		initialHIR := m.Dump()
		steps, err := CompileHIRSteps(m, options)
		if err != nil {
			return fail(err)
		}
		finalProofs, err := hir.ValidateOrderedByteRWCompiledChoices(source, m, choices)
		if err != nil {
			return fail(err)
		}
		finalRaw, err := jointRunRaw(m, workload)
		if err != nil || !jointSameState(finalRaw, oracle) {
			return fail(fmt.Errorf("candidate%d postcompile oracle mismatch: %v", mask, err))
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil {
			return fail(fmt.Errorf("candidate%d assembly: %w", mask, err))
		}
		if assembled == nil {
			return fail(fmt.Errorf("candidate%d assembly: no result", mask))
		}
		if len(assembled.Errors) > 0 || len(assembled.Binary) == 0 {
			return fail(fmt.Errorf("candidate%d assembly: %v %v", mask, err, assembled.Errors))
		}
		run, err := RunZ80ProfileWorkload(steps, source, workload, *oracle)
		if err != nil {
			return fail(err)
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))
		if run == nil || run.BinarySHA256 != digest || len(run.Invocations) != len(workload.Calls) {
			return fail(fmt.Errorf("incomplete/mismatched target evidence"))
		}
		var total int64
		for i, inv := range run.Invocations {
			if !jointEqual(inv.Results, oracle.Results[i]) || !jointEqual(inv.Snapshot, oracle.Snapshots[i]) || (workload.ExactDataAccess && (inv.Access == nil || !jointEqual(*inv.Access, oracle.Accesses[i]))) {
				return fail(fmt.Errorf("candidate%d target oracle mismatch call%d", mask, i))
			}
			if inv.TStates < 0 || inv.TStates > int64(^uint64(0)>>1)-total {
				return fail(fmt.Errorf("invalid measured cost"))
			}
			total += inv.TStates
		}
		if total != run.TotalTStates {
			return fail(fmt.Errorf("target total cost mismatch"))
		}
		e.Candidates = append(e.Candidates, OrderedByteRWFunctionCandidate{RequestedSites: choices, Proofs: proofs, FinalProofs: finalProofs, HIR: initialHIR, FinalHIR: steps.HIR, RawRun: raw, FinalRawRun: finalRaw, Run: run, Binary: assembled.Binary, BinarySHA256: digest, ModuleBytes: len(assembled.Binary), TotalTStates: total})
		compiled[mask] = steps
	}
	for i := 1; i < len(e.Candidates); i++ {
		if orderedByteRWCandidateBetter(e.Candidates[i], e.Candidates[e.Selected], options.OptSize) {
			e.Selected = i
		}
	}
	data, err = json.Marshal(e)
	if err != nil {
		return fail(err)
	}
	e.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	runtime.KeepAlive(retained)
	return &OrderedByteRWFunctionResult{Steps: compiled[e.Selected], Binary: e.Candidates[e.Selected].Binary, Evidence: e}, nil
}

// Equal objective costs prefer fewer rewritten functions before the stable
// enumeration order. This keeps unobserved changes out of an equal-cost winner.
func orderedByteRWCandidateBetter(candidate, best OrderedByteRWFunctionCandidate, size bool) bool {
	value, bestValue := candidate.TotalTStates, best.TotalTStates
	if size {
		value, bestValue = int64(candidate.ModuleBytes), int64(best.ModuleBytes)
	}
	if value != bestValue {
		return value < bestValue
	}
	count := func(choices map[string]hir.FunctionChoice) int {
		n := 0
		for _, enabled := range choices {
			if enabled.Enabled {
				n++
			}
		}
		return n
	}
	return count(candidate.RequestedSites) < count(best.RequestedSites)
}
