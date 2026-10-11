package hir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
)

// LoopCostModel is a measured target model, supplied independently of the profile.
// Positive-trip affine costs and distinct zero-trip costs must be
// validated for the supported domain by assembled execution.
// Cost models never establish rewrite legality. EvidenceID names saved measurements.
type LoopCostModel struct {
	CompilerID        string `json:"compiler_id"`
	SemanticsID       string `json:"semantics_id"`
	Function          string `json:"function"`
	HIRSHA256         string `json:"hir_sha256"`
	TargetID          string `json:"target_id"`
	CompilationID     string `json:"compilation_id"`
	EvidenceID        string `json:"evidence_id"`
	OriginalBytes     uint64 `json:"original_bytes"`
	PointerBytes      uint64 `json:"pointer_bytes"`
	OriginalEntry     uint64 `json:"original_entry_t_states"`
	OriginalIteration uint64 `json:"original_iteration_t_states"`
	EqualEntryExtra   uint64 `json:"equal_entry_extra_t_states"`
	FastEntry         uint64 `json:"fast_entry_t_states"`
	FastZeroEntry     uint64 `json:"fast_zero_entry_t_states"`
	FastZeroMeasured  bool   `json:"fast_zero_measured"`
	FallbackEntry     uint64 `json:"fallback_entry_t_states"`
	FallbackIteration uint64 `json:"fallback_iteration_t_states"`
	FallbackMeasured  bool   `json:"fallback_measured"`
	FastIteration     uint64 `json:"fast_iteration_t_states"`
}

type ProfileDecision struct {
	Compatibility   *ProfileCompatibility `json:"compatibility"`
	SiteID          string                `json:"site_id"`
	Variant         string                `json:"variant"`
	Reason          string                `json:"reason"`
	OriginalTStates string                `json:"original_t_states"`
	PointerTStates  string                `json:"pointer_t_states,omitempty"`
	Model           LoopCostModel         `json:"model"`
}

// SelectLoopVariants loads/refuses a profile against current HIR/context before
// estimating cost. Guard misses require explicitly measured fallback costs;
// missing coverage conservatively retains original. Size mode retains smaller code; speed compares total T-states.
func SelectLoopVariants(r io.Reader, m *Module, c ProfileContext, model LoopCostModel, size bool) ([]ProfileDecision, error) {
	decisions, err := SelectLoopVariantsWithModels(r, m, c, []LoopCostModel{model}, size)
	// Retain the single-model API's attribution on unrelated original decisions.
	for i := range decisions {
		if decisions[i].Model.Function == "" {
			decisions[i].Model = model
		}
	}
	return decisions, err
}

// SelectLoopVariantsWithModels loads one fully compatible profile and matches
// unique measured models by function. Uncovered functions retain original with
// no model or invented cost totals. All model identities are checked before
// producing decisions; compilation configuration is checked by the pipeline.
// Independent models permit at most one pointer rewrite; simultaneous choices
// require a future joint variant-set measurement contract.
func SelectLoopVariantsWithModels(r io.Reader, m *Module, c ProfileContext, models []LoopCostModel, size bool) ([]ProfileDecision, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("loop policy: missing cost models")
	}
	profile, err := LoadLoopProfile(r, m, c)
	if err != nil {
		return nil, err
	}
	byFunction := map[string]LoopCostModel{}
	for _, model := range models {
		if model.CompilerID != c.CompilerID || model.SemanticsID != c.SemanticsID || model.TargetID != c.TargetID || model.EvidenceID == "" || model.OriginalBytes == 0 || model.PointerBytes == 0 || !model.FastZeroMeasured {
			return nil, fmt.Errorf("loop policy: missing or incompatible cost model")
		}
		if model.Function == "" || m.FuncByName(model.Function) == nil || model.HIRSHA256 != profile.Compatibility.HIRSHA256 {
			return nil, fmt.Errorf("loop policy: model does not cover current typed HIR")
		}
		if _, exists := byFunction[model.Function]; exists {
			return nil, fmt.Errorf("loop policy: duplicate cost model for function %q", model.Function)
		}
		byFunction[model.Function] = model
	}

	siteFunctions := map[string]string{}
	for _, site := range profile.Sites.Sites {
		siteFunctions[site.ID] = site.Function
	}
	guards := map[string]GuardCounts{}
	for _, g := range profile.Guards {
		guards[g.SiteID] = g
	}
	total := func(entries, iterations, entryCost, iterationCost uint64) string {
		x := new(big.Int).Mul(new(big.Int).SetUint64(entries), new(big.Int).SetUint64(entryCost))
		x.Add(x, new(big.Int).Mul(new(big.Int).SetUint64(iterations), new(big.Int).SetUint64(iterationCost)))
		return x.String()
	}
	out := []ProfileDecision{}
	for _, count := range profile.Counts {
		model, covered := byFunction[siteFunctions[count.SiteID]]
		d := ProfileDecision{Compatibility: profile.Compatibility, SiteID: count.SiteID, Variant: "original", Reason: "no complete measured guard coverage", Model: model, OriginalTStates: ""}
		if covered {
			d.OriginalTStates = total(count.Entries, count.BodyEntries, model.OriginalEntry, model.OriginalIteration)
		}
		g, ok := guards[count.SiteID]
		if ok && covered && (g.FallbackEntries == 0 || model.FallbackMeasured) && g.FastEntries <= count.Entries && g.FallbackEntries == count.Entries-g.FastEntries && g.FastBodyEntries <= count.BodyEntries && g.FallbackBodyEntries == count.BodyEntries-g.FastBodyEntries && count.Entries > 0 {
			d.PointerTStates = total(g.FastEntries-g.FastZeroEntries, g.FastBodyEntries, model.FastEntry, model.FastIteration)
			fast, _ := new(big.Int).SetString(d.PointerTStates, 10)
			fast.Add(fast, new(big.Int).Mul(new(big.Int).SetUint64(g.FastZeroEntries), new(big.Int).SetUint64(model.FastZeroEntry)))
			fallback, _ := new(big.Int).SetString(total(g.FallbackEntries, g.FallbackBodyEntries, model.FallbackEntry, model.FallbackIteration), 10)
			fast.Add(fast, fallback)
			fast.Add(fast, new(big.Int).Mul(new(big.Int).SetUint64(g.EqualEntries), new(big.Int).SetUint64(model.EqualEntryExtra)))
			d.PointerTStates = fast.String()
			if size {
				d.Reason = "smaller measured code"
				if model.PointerBytes < model.OriginalBytes {
					d.Variant = "pointer-walk"
				}
			} else {
				original, _ := new(big.Int).SetString(d.OriginalTStates, 10)
				pointer, _ := new(big.Int).SetString(d.PointerTStates, 10)
				d.Reason = "lower measured workload T-states"
				if pointer.Cmp(original) < 0 {
					d.Variant = "pointer-walk"
				} else {
					d.Reason = "setup/guard outweigh iteration savings"
				}
			}
		}
		out = append(out, d)
	}
	selectedCount := 0
	for _, decision := range out {
		if decision.Variant == "pointer-walk" {
			selectedCount++
		}
	}
	if selectedCount > 1 {
		return nil, fmt.Errorf("loop policy: simultaneous pointer-walk choices require jointly measured evidence")
	}
	return out, nil
}

// ApplyProfileDecisions validates attribution before running ordinary Grace
// legality predicates. The decisions do not bypass pointer/address guards.
func ApplyProfileDecisions(m *Module, decisions []ProfileDecision) error {
	if m != nil && m.RegionComparePolicy != nil {
		return fmt.Errorf("loop policy: prior region policy")
	}
	sites, err := BuildLoopSiteMap(m)
	if err != nil {
		return err
	}
	// Recheck full source/dependency and typed-HIR compatibility at application.
	for _, decision := range decisions {
		if decision.Compatibility == nil {
			return fmt.Errorf("loop policy: missing decision compatibility")
		}
		current, err := BuildProfileCompatibility(m, decision.Compatibility.Context)
		if err != nil {
			return err
		}
		a, _ := json.Marshal(current)
		b, _ := json.Marshal(decision.Compatibility)
		if !bytes.Equal(a, b) {
			return fmt.Errorf("loop policy: decision refers to stale HIR or source units")
		}
	}
	known := map[string]bool{}
	for _, site := range sites.Sites {
		known[site.ID] = true
	}
	selected := map[string]bool{}
	seen := map[string]bool{}
	for _, d := range decisions {
		if !known[d.SiteID] || seen[d.SiteID] || (d.Variant != "original" && d.Variant != "pointer-walk") {
			return fmt.Errorf("loop policy: invalid decision site/variant")
		}
		seen[d.SiteID] = true
		selected[d.SiteID] = d.Variant == "pointer-walk"
	}
	if err := SeedLoopLineage(m); err != nil {
		return err
	}
	m.PointerWalkPolicy = selected
	_, err = RunGraceWithOptions(m, GraceOptions{PointerWalkSites: selected})
	return err
}
