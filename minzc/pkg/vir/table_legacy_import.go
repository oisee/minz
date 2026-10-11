package vir

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

const MaxLegacyCandidateBytes = 1 << 20
const MaxLegacyCandidates = 1024

// LegacyCandidateImportEvidence is provenance for untrusted assignments tried
// against a CURRENT problem. Old shape hashes without graphs cannot prove a
// shape match. Imported cost/labels never serve as a checked certificate.
type LegacyCandidateImportEvidence struct {
	RemapLimit   int                       `json:"remap_limit"`
	Attempts     int                       `json:"attempts"`
	Schema       string                    `json:"schema"`
	Format       string                    `json:"format"`
	SourceSHA256 string                    `json:"source_sha256"`
	Entries      int                       `json:"entries"`
	Compatible   int                       `json:"compatible"`
	Admitted     int                       `json:"admitted"`
	Decisions    []LegacyCandidateDecision `json:"decisions"`
}
type LegacyCandidateDecision struct {
	Signature   string `json:"signature"`
	ClaimedCost int    `json:"claimed_cost"`
	Reason      string `json:"reason"`
}

// ImportLegacyCandidates tries a bounded legacy assignment pool, independently
// certifying each compatible proposal through AddChecked. It changes neither
// production lookup nor the source data. Dense positions map to sorted current
// vregs; no guessed/permuted operand correspondence is silently introduced.
func ImportLegacyCandidates(data []byte, format string, ops []VIROp, desc *MachineDesc, opts SolverOptions, target *RegAllocTable) (*LegacyCandidateImportEvidence, error) {
	return ImportLegacyCandidatesWithRemap(data, format, ops, desc, opts, target, 0)
}

// ImportLegacyCandidatesWithRemap optionally tries unique permutations of the
// old register multiset. maxVregs is bounded at six, zero means direct mapping.
// This is explicit proposal search, not evidence of historical shape matching.
func ImportLegacyCandidatesWithRemap(data []byte, format string, ops []VIROp, desc *MachineDesc, opts SolverOptions, target *RegAllocTable, maxVregs int) (*LegacyCandidateImportEvidence, error) {
	if maxVregs < 0 || maxVregs > 6 {
		return nil, fmt.Errorf("legacy remap limit must be 0..6")
	}
	if desc == nil || desc.Name != "z80" {
		return nil, fmt.Errorf("legacy import requires current Z80 descriptor")
	}
	if len(data) == 0 || len(data) > MaxLegacyCandidateBytes || target == nil {
		return nil, fmt.Errorf("legacy import invalid size/target")
	}
	if format != "legacy-native-json" && format != "legacy-gpu-json" {
		return nil, fmt.Errorf("legacy import unsupported format")
	}
	var entries []RegAllocEntry
	if format == "legacy-native-json" {
		if e := json.Unmarshal(data, &entries); e != nil {
			return nil, e
		}
	} else {
		var m map[string]RegAllocEntry
		if e := json.Unmarshal(data, &m); e != nil {
			return nil, e
		}
		for key, v := range m {
			v.Signature = key
			entries = append(entries, v)
		}
	}
	if len(entries) == 0 || len(entries) > MaxLegacyCandidates {
		return nil, fmt.Errorf("legacy import bounded entries exceeded")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Signature < entries[j].Signature })
	// Validate the entire envelope before mutating the destination.
	for _, entry := range entries {
		if entry.Schema != "" || entry.CostModel != "" || entry.Signature == "" || len(entry.Assignment) > 64 {
			return nil, fmt.Errorf("legacy import malformed/schema-labelled entry")
		}
		if entry.NVregs != 0 && entry.NVregs != len(entry.Assignment) {
			return nil, fmt.Errorf("legacy import contradictory vreg metadata")
		}
	}
	var ids []int
	for v := range buildProblem(ops, desc).vregs {
		ids = append(ids, v)
	}
	sort.Ints(ids)
	ev := &LegacyCandidateImportEvidence{Schema: "vir-legacy-candidate-import/v1", Format: format, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Entries: len(entries), RemapLimit: maxVregs}
	for _, entry := range entries {
		d := LegacyCandidateDecision{Signature: entry.Signature, ClaimedCost: entry.Cost}
		if entry.Schema != "" || entry.CostModel != "" {
			return nil, fmt.Errorf("legacy import refuses checked/schema-labelled input")
		}
		if len(entry.Assignment) != len(ids) {
			d.Reason = "different dense vreg count"
			ev.Decisions = append(ev.Decisions, d)
			continue
		}
		if entry.NOps != 0 && entry.NOps != len(ops) {
			d.Reason = "different operation count"
			ev.Decisions = append(ev.Decisions, d)
			continue
		}
		if entry.NVregs != 0 && entry.NVregs != len(ids) {
			return nil, fmt.Errorf("legacy import contradictory vreg metadata")
		}
		ev.Compatible++
		assignment := map[int]int{}
		invalid := false
		for i, loc := range entry.Assignment {
			if format == "legacy-gpu-json" {
				if loc < 0 || loc >= len(gpuToZ80Loc) {
					invalid = true
					break
				}
				loc = gpuToZ80Loc[loc]
			}
			if loc < 0 || loc >= len(desc.Locs) {
				invalid = true
				break
			}
			assignment[ids[i]] = loc
		}
		if invalid {
			d.Reason = "unmappable physical location"
		} else {
			try := func() {
				ev.Attempts++
				if e := target.AddChecked(ops, assignment, desc, opts); e != nil {
					if d.Reason == "" {
						d.Reason = e.Error()
					}
				} else {
					d.Reason = "independently admitted current problem"
					ev.Admitted++
				}
			}
			if maxVregs > 0 && len(ids) <= maxVregs {
				locs := make([]int, len(ids))
				for i, v := range ids {
					locs[i] = assignment[v]
				}
				sort.Ints(locs)
				var visit func(int)
				visit = func(i int) {
					if i == len(ids) {
						try()
						return
					}
					seen := map[int]bool{}
					for j := i; j < len(locs); j++ {
						if seen[locs[j]] {
							continue
						}
						seen[locs[j]] = true
						locs[i], locs[j] = locs[j], locs[i]
						assignment[ids[i]] = locs[i]
						visit(i + 1)
						locs[i], locs[j] = locs[j], locs[i]
					}
				}
				visit(0)
			} else {
				try()
			}
		}
		ev.Decisions = append(ev.Decisions, d)
	}
	// Evidence owns no alias into mutable caller payload.
	return ev, nil
}
