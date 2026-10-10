package hir

import (
	"fmt"
	"io"
	"math/big"
)

// LoopCostModel is a measured target model, supplied independently of the profile.
// Linear costs must be validated for the supported domain by assembled execution.
// Cost models never establish rewrite legality. EvidenceID names saved measurements.
type LoopCostModel struct {
	Function          string `json:"function"`
	HIRSHA256         string `json:"hir_sha256"`
	TargetID          string `json:"target_id"`
	EvidenceID        string `json:"evidence_id"`
	OriginalBytes     uint64 `json:"original_bytes"`
	PointerBytes      uint64 `json:"pointer_bytes"`
	OriginalEntry     uint64 `json:"original_entry_t_states"`
	OriginalIteration uint64 `json:"original_iteration_t_states"`
	FastEntry         uint64 `json:"fast_entry_t_states"`
	FastIteration     uint64 `json:"fast_iteration_t_states"`
}

type ProfileDecision struct {
	SiteID          string        `json:"site_id"`
	Variant         string        `json:"variant"`
	Reason          string        `json:"reason"`
	OriginalTStates string        `json:"original_t_states"`
	PointerTStates  string        `json:"pointer_t_states,omitempty"`
	Model           LoopCostModel `json:"model"`
}

// SelectLoopVariants loads/refuses a profile against current HIR/context before
// estimating cost. The initial measured model covers all-fast sum idioms; guard
// misses or missing coverage conservatively retain original until fallback costs
// are measured. Size mode retains smaller code; speed compares total T-states.
func SelectLoopVariants(r io.Reader, m *Module, c ProfileContext, model LoopCostModel, size bool) ([]ProfileDecision, error) {
	if model.TargetID != c.TargetID || model.EvidenceID == "" || model.OriginalBytes == 0 || model.PointerBytes == 0 {
		return nil, fmt.Errorf("loop policy: missing or incompatible cost model")
	}
	profile, err := LoadLoopProfile(r, m, c)
	if err != nil {
		return nil, err
	}
	if model.Function == "" || model.HIRSHA256 != profile.Compatibility.HIRSHA256 {
		return nil, fmt.Errorf("loop policy: model does not cover current typed HIR")
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
		d := ProfileDecision{SiteID: count.SiteID, Variant: "original", Reason: "no complete measured guard coverage", Model: model, OriginalTStates: total(count.Entries, count.BodyEntries, model.OriginalEntry, model.OriginalIteration)}
		g, ok := guards[count.SiteID]
		if ok && siteFunctions[count.SiteID] == model.Function && g.FallbackEntries == 0 && g.FastEntries == count.Entries && g.FastBodyEntries == count.BodyEntries && count.Entries > 0 {
			d.PointerTStates = total(g.FastEntries, g.FastBodyEntries, model.FastEntry, model.FastIteration)
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
	return out, nil
}

// ApplyProfileDecisions validates attribution before running ordinary Grace
// legality predicates. The decisions do not bypass pointer/address guards.
func ApplyProfileDecisions(m *Module, decisions []ProfileDecision) error {
	sites, err := BuildLoopSiteMap(m)
	if err != nil {
		return err
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
	_, err = RunGraceWithOptions(m, GraceOptions{PointerWalkSites: selected})
	return err
}
