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

type ForwardCopySiteCandidate struct {
	RequestedSites map[string]bool             `json:"requested_copy_sites"`
	Proofs         []hir.ForwardCopyProof      `json:"proofs"`
	FinalProofs    []hir.ForwardCopyProof      `json:"postcompile_proofs"`
	HIR            string                      `json:"hir"`
	FinalHIR       string                      `json:"postcompile_hir"`
	RawRun         *hir.ProfileRun             `json:"raw_run"`
	FinalRawRun    *hir.ProfileRun             `json:"postcompile_raw_run"`
	Run            *Z80ProfileWorkloadEvidence `json:"target_run"`
	Binary         []byte                      `json:"binary"`
	BinarySHA256   string                      `json:"binary_sha256"`
	ModuleBytes    int                         `json:"module_bytes"`
	TotalTStates   int64                       `json:"total_t_states"`
}
type ForwardCopySiteEvidence struct {
	Schema        string                     `json:"schema"`
	Compatibility *hir.ProfileCompatibility  `json:"compatibility"`
	CompilationID string                     `json:"compilation_id"`
	Options       Options                    `json:"options"`
	MaxSites      int                        `json:"max_sites"`
	EligibleSites []string                   `json:"eligible_copy_sites"`
	Oracle        *hir.ProfileRun            `json:"oracle"`
	Candidates    []ForwardCopySiteCandidate `json:"candidates"`
	Selected      int                        `json:"selected"`
	SHA256        string                     `json:"sha256,omitempty"`
}
type ForwardCopySiteResult struct {
	Steps    Steps
	Binary   []byte
	Evidence *ForwardCopySiteEvidence
}

// MeasureForwardCopySiteVariantSets compares actual complete caller execution
// for at most four observed exact copy sites (16 candidates). Current compatible
// source-loop counts select observed sites; no guard/cost summary is extrapolated.
// Other Grace rules remain off. Every choice is structurally checked before and
// after compilation, then measured on assembled Z80 with full oracle agreement.
func MeasureForwardCopySiteVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*ForwardCopySiteResult, error) {
	fail := func(err error) (*ForwardCopySiteResult, error) {
		return nil, fmt.Errorf("measure forward copy sites: %w", err)
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
	clean := func(m *hir.Module) bool {
		return m != nil && m.RegionComparePolicy == nil && m.ForwardCopyPolicy == nil && m.PointerWalkPolicy == nil && m.GraceRulePolicy == nil && m.InvariantBasePolicy == nil && m.LoopLineage == nil
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
	legal, err := hir.EligibleForwardCopySites(source)
	if err != nil {
		return fail(err)
	}
	observed := map[string]bool{}
	for _, count := range profile.Counts {
		observed[count.SiteID] = count.Entries > 0
	}
	eligible := []string{}
	for _, id := range legal {
		if observed[id] {
			eligible = append(eligible, id)
		}
	}
	sort.Strings(eligible)
	if len(eligible) > maxSites {
		return fail(fmt.Errorf("observed copy sites exceed cap"))
	}
	modules := make([]*hir.Module, 1<<len(eligible))
	for i := range modules {
		modules[i], err = fresh()
		if err != nil {
			return fail(err)
		}
	}
	for _, m := range retained {
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
	e := &ForwardCopySiteEvidence{Schema: "minz.forward-copy-site-variant-sets/v1", Compatibility: key, CompilationID: config, Options: options, MaxSites: maxSites, EligibleSites: eligible, Oracle: oracle, Candidates: []ForwardCopySiteCandidate{}}
	e.Options.GraceStats = nil
	check := func(m *hir.Module, choices map[string]bool) ([]hir.ForwardCopyProof, error) {
		proofs := []hir.ForwardCopyProof{}
		for _, id := range legal {
			proof, err := hir.ValidateForwardCopyProof(source, m, id, choices[id])
			if err != nil {
				return nil, err
			}
			proofs = append(proofs, proof)
		}
		for _, l := range hir.CurrentLoopLineage(m) {
			if l.Lineage.Rule != "" && l.Lineage.Rule != hir.ForwardCopyRule {
				return nil, fmt.Errorf("unrequested Grace rule")
			}
			if l.Lineage.Rule == hir.ForwardCopyRule && !choices[l.Lineage.SiteID] {
				return nil, fmt.Errorf("unrequested copy rewrite")
			}
		}
		return proofs, nil
	}
	compiled := make([]Steps, len(modules))
	for mask, m := range modules {
		choices := map[string]bool{}
		for j, id := range eligible {
			choices[id] = mask&(1<<(len(eligible)-1-j)) != 0
		}
		if err := hir.ApplyForwardCopySiteChoices(m, key, choices); err != nil {
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
		finalProofs, err := check(m, choices)
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
		for _, inv := range run.Invocations {
			if inv.TStates < 0 || inv.TStates > int64(^uint64(0)>>1)-total {
				return fail(fmt.Errorf("invalid measured cost"))
			}
			total += inv.TStates
		}
		e.Candidates = append(e.Candidates, ForwardCopySiteCandidate{RequestedSites: choices, Proofs: proofs, FinalProofs: finalProofs, HIR: initialHIR, FinalHIR: steps.HIR, RawRun: raw, FinalRawRun: finalRaw, Run: run, Binary: assembled.Binary, BinarySHA256: digest, ModuleBytes: len(assembled.Binary), TotalTStates: total})
		compiled[mask] = steps
	}
	for i := 1; i < len(e.Candidates); i++ {
		a, b := e.Candidates[i], e.Candidates[e.Selected]
		better := a.TotalTStates < b.TotalTStates
		if options.OptSize {
			better = a.ModuleBytes < b.ModuleBytes
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
	runtime.KeepAlive(retained)
	return &ForwardCopySiteResult{Steps: compiled[e.Selected], Binary: e.Candidates[e.Selected].Binary, Evidence: e}, nil
}
