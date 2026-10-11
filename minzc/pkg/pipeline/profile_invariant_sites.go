package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
)

// InvariantSiteVariantSetResult retains the exact measured winner.
type InvariantSiteVariantSetResult struct {
	Steps     Steps
	Decisions []hir.ProfileDecision
	Binary    []byte
	Evidence  *InvariantSiteVariantSetEvidence
}

// InvariantSiteVariantSetCandidate records explicit source-site requests in
// addition to before/after HIR, lineage, semantic runs and exact target costs.
type InvariantSiteVariantSetCandidate struct {
	ProfileVariantSetCandidate
	RequestedSitePolicy map[string]bool `json:"requested_site_policy"`
}

// InvariantSiteVariantSetEvidence binds all candidates and the selected index.
// SHA256 is the digest of its JSON with SHA256 omitted.
type InvariantSiteVariantSetEvidence struct {
	Schema        string                             `json:"schema"`
	Compatibility *hir.ProfileCompatibility          `json:"compatibility"`
	CompilationID string                             `json:"compilation_id"`
	Options       Options                            `json:"options"`
	AssemblerMode string                             `json:"assembler_mode"`
	MaxSites      int                                `json:"max_sites"`
	EligibleSites []string                           `json:"eligible_invariant_sites"`
	Oracle        *hir.ProfileRun                    `json:"oracle"`
	Candidates    []InvariantSiteVariantSetCandidate `json:"candidates"`
	Selected      int                                `json:"selected"`
	SHA256        string                             `json:"sha256,omitempty"`
}

// MeasureInvariantSiteVariantSets independently enumerates observed legal scalar
// invariant source loops (Entries > 0, including zero trips), up to four choices
// and sixteen candidates. Constant-if is explicitly disabled and every pointer
// choice remains original. Crossing overlapping pointer choices is outside this
// contract. Legality comes from the original source proof, never observations.
// All factory inputs are preflighted before compilation; forged observations,
// unrealized requests and semantic mismatches fail closed. Actual assembled Z80
// invocation costs (or module bytes for OptSize) choose the retained winner;
// strict improvement is required, so the all-original baseline wins ties.
func MeasureInvariantSiteVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*InvariantSiteVariantSetResult, error) {
	return measureInvariantSiteVariantSets(factory, observations, workload, options, maxSites, false)
}

// MeasureInvariantSiteRuleVariantSets crosses per-source-loop scalar hoists with
// constant-if off/on. At most four sites give 32 measured candidates. Pointer
// choices remain original; incomplete first-pass requests fail closed, including
// requests deleted or starved by constant-if. The v1 API remains unchanged.
func MeasureInvariantSiteRuleVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*InvariantSiteVariantSetResult, error) {
	return measureInvariantSiteVariantSets(factory, observations, workload, options, maxSites, true)
}

func measureInvariantSiteVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int, independentRules bool) (*InvariantSiteVariantSetResult, error) {
	fail := func(err error) (*InvariantSiteVariantSetResult, error) {
		return nil, fmt.Errorf("measure invariant site variant sets: %w", err)
	}
	if factory == nil || maxSites < 1 || maxSites > 4 || options.ProfileSelection != nil || options.SkipZ80Emission || (options.Backend != "" && options.Backend != "z80") {
		return fail(fmt.Errorf("unsupported factory/backend/configuration or site cap (must be 1..4)"))
	}
	config, err := ProfileCompilationID(options, "default")
	if err != nil {
		return fail(err)
	}
	// Own the request, so evidence hashes the workload actually executed.
	data, err := json.Marshal(workload)
	if err != nil {
		return fail(err)
	}
	var ownedWorkload hir.ProfileWorkload
	if err = json.Unmarshal(data, &ownedWorkload); err != nil {
		return fail(err)
	}
	workload = ownedWorkload
	seen := map[uintptr]bool{}
	retained := []*hir.Module{}
	var key *hir.ProfileCompatibility
	fresh := func() (*hir.Module, error) {
		m, err := factory()
		if err != nil {
			return nil, err
		}
		if m == nil || m.PointerWalkPolicy != nil || m.LoopLineage != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil {
			return nil, fmt.Errorf("factory must return fresh input HIR without policy/lineage")
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
		} else if !jointEqual(key, k) {
			return nil, fmt.Errorf("factory returned non-equivalent HIR/source/context")
		}
		retained = append(retained, m) // Keep pointers alive while checking independence.
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
	legal, err := hir.EligibleInvariantBaseSites(source)
	if err != nil {
		return fail(err)
	}
	observed := map[string]bool{}
	for _, c := range profile.Counts {
		observed[c.SiteID] = c.Entries > 0
	}
	eligible := []string{}
	known := map[string]bool{}
	for _, s := range profile.Sites.Sites {
		known[s.ID] = true
	}
	seenLegal := map[string]bool{}
	for _, id := range legal {
		if !known[id] || seenLegal[id] {
			return fail(fmt.Errorf("invalid eligible invariant source site %q", id))
		}
		seenLegal[id] = true
		if observed[id] {
			eligible = append(eligible, id)
		}
	}
	sort.Strings(eligible)
	if len(eligible) > maxSites {
		return fail(fmt.Errorf("%d observed eligible sites exceed cap %d (maximum 4 sites/16 sets)", len(eligible), maxSites))
	}
	// Preflight all factory inputs before any compilation or execution.
	candidateCount := 1 << len(eligible)
	if independentRules {
		candidateCount *= 2
	}
	modules := make([]*hir.Module, candidateCount)
	for i := range modules {
		modules[i], err = fresh()
		if err != nil {
			return fail(err)
		}
	}
	// A factory may mutate an earlier returned module while producing the next.
	for _, m := range retained {
		c, err := hir.BuildWorkloadProfileContext(m, workload)
		if err != nil {
			return fail(err)
		}
		k, err := hir.BuildProfileCompatibility(m, c)
		if err != nil {
			return fail(err)
		}
		if !jointEqual(k, key) || m.PointerWalkPolicy != nil || m.LoopLineage != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil {
			return fail(fmt.Errorf("factory mutated previously returned HIR/source"))
		}
	}
	oracle, err := hir.RunLoopProfile(source, workload)
	if err != nil {
		return fail(err)
	}
	// A compatible but forged observation must not introduce unobserved choices.
	if !jointEqual(profile.Counts, jointSortedCounts(oracle.Profile.Counts)) || !jointEqual(jointSortedGuards(profile.Guards), jointSortedGuards(oracle.Profile.Guards)) || profile.CompletedCalls != oracle.Profile.CompletedCalls {
		return fail(fmt.Errorf("observations do not match actual controlled workload"))
	}
	e := &InvariantSiteVariantSetEvidence{Schema: "minz.invariant-site-variant-sets/v1", Compatibility: key, CompilationID: config, Options: options, AssemblerMode: "default", MaxSites: maxSites, EligibleSites: eligible, Oracle: oracle, Candidates: []InvariantSiteVariantSetCandidate{}}
	if independentRules {
		e.Schema = "minz.invariant-site-rule-variant-sets/v2"
	}
	e.Options.GraceStats = nil
	compiled := make([]Steps, len(modules))
	for mask, m := range modules {
		constantIf := independentRules && mask >= 1<<len(eligible)
		m.GraceRulePolicy = map[string]bool{"hir-constant-if": constantIf, "hir-invariant-base": true}
		choices := map[string]bool{}
		for j, id := range eligible {
			choices[id] = mask&(1<<(len(eligible)-1-j)) != 0
		}
		m.InvariantBasePolicy = choices
		decisions := make([]hir.ProfileDecision, 0, len(profile.Sites.Sites))
		for _, s := range profile.Sites.Sites {
			decisions = append(decisions, hir.ProfileDecision{Compatibility: key, SiteID: s.ID, Variant: "original", Reason: "independent scalar invariant site search; pointer choices original"})
		}
		if err := hir.ApplyProfileDecisions(m, decisions); err != nil {
			return fail(err)
		}
		lineage := hir.CurrentLoopLineage(m)
		if err := invariantSitesFullyRealized(m, lineage, choices); err != nil {
			return fail(fmt.Errorf("candidate %d: %w", mask, err))
		}
		hirDump := m.Dump()
		raw, err := jointRunRaw(m, workload)
		if err != nil {
			return fail(fmt.Errorf("candidate %d raw VM: %w", mask, err))
		}
		if !jointSameState(raw, oracle) {
			return fail(fmt.Errorf("candidate %d raw VM semantic mismatch", mask))
		}
		steps, err := CompileHIRSteps(m, options)
		if err != nil {
			return fail(err)
		}
		steps.ProfileDecisions = decisions
		finalHIR := steps.HIR // After Grace, before function splitting.
		finalLineage := hir.CurrentLoopLineage(m)
		finalRaw, err := jointRunRaw(m, workload)
		if err != nil || !jointSameState(finalRaw, oracle) {
			return fail(fmt.Errorf("candidate %d postcompile raw semantic mismatch: %v", mask, err))
		}
		if err := invariantSitesFullyRealized(m, finalLineage, choices); err != nil {
			return fail(fmt.Errorf("candidate %d postcompile: %w", mask, err))
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil {
			return fail(err)
		}
		if len(assembled.Errors) > 0 || len(assembled.Binary) == 0 {
			return fail(fmt.Errorf("candidate %d assembly failed: %v", mask, assembled.Errors))
		}
		run, err := RunZ80ProfileWorkload(steps, source, workload, *oracle)
		if err != nil {
			return fail(fmt.Errorf("candidate %d target workload: %w", mask, err))
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))
		if run == nil || run.BinarySHA256 != digest || run.TotalTStates < 0 {
			return fail(fmt.Errorf("candidate %d target evidence/binary mismatch", mask))
		}
		// Sum entry-through-RET invocation costs: the harness may additionally
		// charge bootstrap cycles in its cumulative execution evidence.
		var total int64
		if len(run.Invocations) != len(workload.Calls) {
			return fail(fmt.Errorf("candidate %d incomplete invocation evidence", mask))
		}
		for _, invocation := range run.Invocations {
			if invocation.TStates < 0 || invocation.TStates > int64(^uint64(0)>>1)-total {
				return fail(fmt.Errorf("candidate %d invalid invocation costs", mask))
			}
			total += invocation.TStates
		}
		e.Candidates = append(e.Candidates, InvariantSiteVariantSetCandidate{RequestedSitePolicy: choices, ProfileVariantSetCandidate: ProfileVariantSetCandidate{HIR: hirDump, Lineage: lineage, FinalHIR: finalHIR, FinalLineage: finalLineage, FinalRawRun: finalRaw, RulePolicy: map[string]bool{"hir-constant-if": constantIf, "hir-invariant-base": true}, CommonGraceRules: false, TotalTStates: total, Decisions: decisions, Binary: assembled.Binary, BinarySHA256: digest, ModuleBytes: len(assembled.Binary), RawRun: raw, Run: run}})
		compiled[mask] = steps
	}
	for i := 1; i < len(e.Candidates); i++ {
		candidate, best := e.Candidates[i], e.Candidates[e.Selected]
		better := candidate.TotalTStates < best.TotalTStates
		if options.OptSize {
			better = candidate.ModuleBytes < best.ModuleBytes
		}
		if better {
			e.Selected = i
		}
	}
	data, err = json.Marshal(e)
	if err != nil {
		return fail(err)
	}
	e.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	best := e.Candidates[e.Selected]
	// retained remains live through all mutations: pointer identity is not a GC key.
	runtime.KeepAlive(retained)
	return &InvariantSiteVariantSetResult{Steps: compiled[e.Selected], Decisions: best.Decisions, Binary: best.Binary, Evidence: e}, nil
}

// Reject any unrequested hoist or pointer rewrite as well as missing requests.
func invariantSitesRealized(lineage []hir.LoopLineageRecord, requested map[string]bool) error {
	applied := map[string]bool{}
	for _, l := range lineage {
		if l.Lineage.Rule == "hir-guarded-pointer-walk" {
			return fmt.Errorf("unexpected pointer rewrite at %s", l.Lineage.SiteID)
		}
		if l.Lineage.Rule == "hir-invariant-base" || l.Lineage.Role == "invariant-base" {
			if l.Lineage.Rule != "hir-invariant-base" || l.Lineage.Role != "invariant-base" || !requested[l.Lineage.SiteID] {
				return fmt.Errorf("unrequested invariant hoist at %s", l.Lineage.SiteID)
			}
			applied[l.Lineage.SiteID] = true
		}
	}
	for id, want := range requested {
		if applied[id] != want {
			return fmt.Errorf("requested invariant choice was not realized for %s", id)
		}
	}
	return nil
}

// A lineage marker proves a rewrite occurred, but not that every legal addition
// at the selected site was hoisted before a Grace rewrite budget ran out.
func invariantSitesFullyRealized(m *hir.Module, lineage []hir.LoopLineageRecord, requested map[string]bool) error {
	if err := invariantSitesRealized(lineage, requested); err != nil {
		return err
	}
	remaining, err := hir.RemainingInvariantBaseSites(m)
	if err != nil {
		return err
	}
	for _, id := range remaining {
		if requested[id] {
			return fmt.Errorf("requested invariant choice was not completely realized for %s", id)
		}
	}
	return nil
}
