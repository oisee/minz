package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/minz/minzc/pkg/hir"
)

type ConditionalLayoutModesEvidence struct {
	Schema       string                       `json:"schema"`
	Goal         string                       `json:"goal"`
	Options      []Options                    `json:"options"`
	Runs         []*ConditionalLayoutEvidence `json:"runs"`
	SelectedMode int                          `json:"selected_mode"`
	SHA256       string                       `json:"sha256,omitempty"`
}
type ConditionalLayoutModesResult struct {
	Steps    Steps
	Binary   []byte
	Evidence *ConditionalLayoutModesEvidence
}

// MeasureConditionalLayoutModes compares at most eight caller-supplied compiler
// configurations and each configuration's bounded conditional layouts. The same
// frozen training workload selects one complete measured executable. Heldout is
// deliberately separate. Every mode must succeed; no partial winner is returned.
func MeasureConditionalLayoutModes(factory func() (*hir.Module, error), function string, workload hir.ProfileWorkload, modes []Options, goal string, limit int) (*ConditionalLayoutModesResult, error) {
	fail := func(e error) (*ConditionalLayoutModesResult, error) {
		return nil, fmt.Errorf("conditional layout modes: %w", e)
	}
	if factory == nil || len(modes) < 1 || len(modes) > 8 || function == "" || (goal != "speed" && goal != "size") || limit < 1 || limit > 4 {
		return fail(fmt.Errorf("invalid request/bounds"))
	}
	modes = append([]Options(nil), modes...)
	for i := range modes {
		modes[i].GraceStats = nil
		modes[i].DisableVariantSearch = true
	}
	payload, e := json.Marshal(struct {
		Workload hir.ProfileWorkload
		Modes    []Options
	}{workload, modes})
	if e != nil {
		return fail(e)
	}
	var owned struct {
		Workload hir.ProfileWorkload
		Modes    []Options
	}
	if e = json.Unmarshal(payload, &owned); e != nil {
		return fail(e)
	}
	workload, modes = owned.Workload, owned.Modes
	// Validate all configurations before invoking a possibly stateful factory.
	for _, options := range modes {
		if options.SkipZ80Emission || options.ProfileSelection != nil || options.Backend != "" && options.Backend != "z80" {
			return fail(fmt.Errorf("unsupported mode configuration"))
		}
		if _, e = ProfileCompilationID(options, "default"); e != nil {
			return fail(e)
		}
	}
	type retainedInput struct {
		module   *hir.Module
		snapshot []byte
	}
	retained := []retainedInput{}
	batch := []*hir.Module{}
	all := []*hir.Module{}
	defer func() { runtime.KeepAlive(all) }()
	snapshot := func(m *hir.Module) ([]byte, error) {
		policies := []any{m.OrderedByteRWPolicy, m.OrderedBytePolicy, m.RegionComparePolicy, m.ForwardCopyPolicy, m.PointerWalkPolicy, m.GraceRulePolicy, m.InvariantBasePolicy}
		return json.Marshal(struct {
			Module     *hir.Module
			Units      map[string]string
			Complete   bool
			Policies   []any
			Lineage    []hir.LoopLineageRecord
			LineageSet bool
		}{m, m.SourceUnits, m.SourceUnitsComplete, policies, hir.CurrentLoopLineage(m), m.LoopLineage != nil})
	}
	check := func() error {
		for _, old := range retained {
			current, e := snapshot(old.module)
			if e != nil {
				return e
			}
			if !bytes.Equal(current, old.snapshot) {
				return fmt.Errorf("earlier mode HIR/source/policy changed")
			}
		}
		return nil
	}
	originalFactory := factory
	seen := map[uintptr]any{}
	var identity *hir.ProfileCompatibility
	factory = func() (*hir.Module, error) {
		m, e := originalFactory()
		if e != nil {
			return nil, e
		}
		if e = check(); e != nil {
			return nil, e
		}
		if m == nil {
			return nil, fmt.Errorf("nil factory module")
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		ctx, e := hir.BuildWorkloadProfileContext(m, workload)
		if e != nil {
			return nil, e
		}
		key, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return nil, e
		}
		if identity == nil {
			identity = key
		} else if !jointEqual(identity, key) {
			return nil, fmt.Errorf("source/HIR/ABI changed across modes")
		}
		all = append(all, m)
		batch = append(batch, m)
		return m, nil
	}
	evidence := &ConditionalLayoutModesEvidence{Schema: "minz.measured-conditional-layout-modes/v1", Goal: goal, Options: modes}
	results := []*ConditionalLayoutResult{}
	for i, options := range modes {
		result, e := MeasureConditionalLayoutWorkload(factory, function, workload, options, goal, limit)
		if e != nil {
			return fail(fmt.Errorf("mode%d: %w", i, e))
		}
		if e = check(); e != nil {
			return fail(e)
		}
		for _, m := range batch {
			saved, e := snapshot(m)
			if e != nil {
				return fail(e)
			}
			retained = append(retained, retainedInput{m, saved})
		}
		batch = nil
		results = append(results, result)
		evidence.Runs = append(evidence.Runs, result.Evidence)
		best := results[evidence.SelectedMode]
		chosen := result.Evidence.Candidates[result.Evidence.Selected]
		bestChosen := best.Evidence.Candidates[best.Evidence.Selected]
		if goal == "speed" && chosen.Run.TotalTStates < bestChosen.Run.TotalTStates || goal == "size" && len(result.Binary) < len(best.Binary) {
			evidence.SelectedMode = i
		}
	}
	// Later factories may alter an earlier table file without touching HIR.
	for i, options := range modes {
		id, e := ProfileCompilationID(options, "default")
		if e != nil {
			return fail(e)
		}
		if id != evidence.Runs[i].CompilationID {
			return fail(fmt.Errorf("mode%d configuration changed", i))
		}
	}
	if e = check(); e != nil {
		return fail(e)
	}
	payload, e = json.Marshal(evidence)
	if e != nil {
		return fail(e)
	}
	evidence.SHA256 = fmt.Sprintf("%x", sha256.Sum256(payload))
	chosen := results[evidence.SelectedMode]
	return &ConditionalLayoutModesResult{Steps: chosen.Steps, Binary: append([]byte(nil), chosen.Binary...), Evidence: evidence}, nil
}
