package hir

import (
	"fmt"
	"reflect"
)

// MixedCopyWeightedChoices contains independent bits by original source kind.
// Weighted sites contribute two bits. Unknown false keys are invalid too.
type MixedCopyWeightedChoices struct {
	Copy    map[string]bool `json:"copy_sites"`
	Scalar  map[string]bool `json:"weighted_scalar_sites"`
	Pointer map[string]bool `json:"weighted_pointer_sites"`
}
type MixedCopyWeightedProof struct {
	Copies   []ForwardCopyProof          `json:"copy_proofs"`
	Weighted []WeightedCompositionRecord `json:"weighted_proofs"`
}

func MixedCopyWeightedSites(m *Module) ([]string, []string, error) {
	copies, err := EligibleForwardCopySites(m)
	if err != nil {
		return nil, nil, err
	}
	weighted, err := EligibleWeightedCompositionSites(m)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	for _, id := range copies {
		seen[id] = true
	}
	for _, id := range weighted {
		if seen[id] {
			return nil, nil, fmt.Errorf("mixedcopyweighted: overlapping copy source site")
		}
	}
	return copies, weighted, nil
}
func validateMixedKeys(m *Module, choices MixedCopyWeightedChoices) ([]string, []string, error) {
	copies, weighted, err := MixedCopyWeightedSites(m)
	if err != nil {
		return nil, nil, err
	}
	sets := []map[string]bool{{}, {}, {}}
	for _, id := range copies {
		sets[0][id] = true
	}
	for _, id := range weighted {
		sets[1][id] = true
		sets[2][id] = true
	}
	for i, requested := range []map[string]bool{choices.Copy, choices.Scalar, choices.Pointer} {
		for id := range requested {
			if !sets[i][id] {
				return nil, nil, fmt.Errorf("mixedcopyweighted: unknown/mismatched kind site %s", id)
			}
		}
	}
	return copies, weighted, nil
}

// ApplyMixedCopyWeightedChoices installs owned maps together before seeding once
// and applying Grace. Legality is exact copy/weighted source, never profile data.
func ApplyMixedCopyWeightedChoices(m *Module, compatibility *ProfileCompatibility, choices MixedCopyWeightedChoices) error {
	if m == nil || compatibility == nil || m.RegionComparePolicy != nil || m.OrderedBytePolicy != nil || m.OrderedByteRWPolicy != nil || m.LoopLineage != nil || m.ForwardCopyPolicy != nil || m.PointerWalkPolicy != nil || m.InvariantBasePolicy != nil || m.GraceRulePolicy != nil {
		return fmt.Errorf("mixedcopyweighted: requires fresh original HIR")
	}
	current, err := BuildProfileCompatibility(m, compatibility.Context)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, compatibility) {
		return fmt.Errorf("mixedcopyweighted: stale compatibility")
	}
	if _, _, err := validateMixedKeys(m, choices); err != nil {
		return err
	}
	own := func(x map[string]bool) map[string]bool {
		out := map[string]bool{}
		for id, want := range x {
			out[id] = want
		}
		return out
	}
	if err := SeedLoopLineage(m); err != nil {
		return err
	}
	m.ForwardCopyPolicy = own(choices.Copy)
	m.InvariantBasePolicy = own(choices.Scalar)
	m.PointerWalkPolicy = own(choices.Pointer)
	m.GraceRulePolicy = map[string]bool{"hir-constant-if": false, "hir-invariant-base": true, "hir-guarded-pointer-walk": true, ForwardCopyRule: true}
	_, err = RunGraceWithOptions(m, GraceOptions{OptimizeForSize: false})
	return err
}

// ValidateMixedCopyWeightedProof checks every legal site, including unobserved
// disabled sites, then requires exact reachable attribution multiplicities.
func ValidateMixedCopyWeightedProof(source, actual *Module, choices MixedCopyWeightedChoices) (MixedCopyWeightedProof, error) {
	if source == nil || actual == nil {
		return MixedCopyWeightedProof{}, fmt.Errorf("mixedcopyweighted: nil module")
	}
	if err := Verify(source); err != nil {
		return MixedCopyWeightedProof{}, err
	}
	if err := Verify(actual); err != nil {
		return MixedCopyWeightedProof{}, err
	}

	copies, weighted, err := validateMixedKeys(source, choices)
	if err != nil {
		return MixedCopyWeightedProof{}, err
	}
	out := MixedCopyWeightedProof{Copies: []ForwardCopyProof{}, Weighted: []WeightedCompositionRecord{}}
	for _, id := range copies {
		p, err := ValidateForwardCopyProof(source, actual, id, choices.Copy[id])
		if err != nil {
			return out, err
		}
		out.Copies = append(out.Copies, p)
	}
	for _, id := range weighted {
		p, err := ValidateWeightedComposition(source, actual, id, choices.Scalar[id], choices.Pointer[id])
		if err != nil {
			return out, err
		}
		out.Weighted = append(out.Weighted, p)
	}
	type key struct{ site, function, role, rule, previous string }
	expected := map[key]int{}
	for _, p := range out.Copies {
		if p.Enabled {
			expected[key{p.SiteID, p.Function, "fast", ForwardCopyRule, ""}] = 1
			expected[key{p.SiteID, p.Function, "fallback", ForwardCopyRule, ""}] = 1
		} else {
			expected[key{p.SiteID, p.Function, "original", "", ""}] = 1
		}
	}
	for _, p := range out.Weighted {
		if p.Pointer {
			previous := ""
			if p.Scalar {
				previous = "hir-invariant-base"
			}
			expected[key{p.SiteID, p.Function, "fast", "hir-guarded-pointer-walk", previous}] = 1
			expected[key{p.SiteID, p.Function, "fallback", "hir-guarded-pointer-walk", previous}] = 1
		} else {
			role, rule := "original", ""
			if p.Scalar {
				role, rule = "invariant-base", "hir-invariant-base"
			}
			expected[key{p.SiteID, p.Function, role, rule, ""}] = 1
		}
	}
	sites, err := BuildLoopSiteMap(source)
	if err != nil {
		return out, err
	}
	if sites.UnmappedLoops != 0 {
		return out, fmt.Errorf("mixedcopyweighted: unmapped source loops")
	}
	var lineageErr error
	for _, f := range actual.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for _, stmt := range b.Body {
				isLoop := false
				switch stmt.(type) {
				case *WhileStmt, *ForRangeStmt, *ForEachStmt:
					isLoop = true
				}
				_, marked := actual.LoopLineage[stmt]
				if isLoop != marked {
					lineageErr = fmt.Errorf("mixedcopyweighted: loop attribution attached to wrong node or missing: %s/%s", f.Name, path)
				}
			}
		})
	}
	if lineageErr != nil {
		return out, lineageErr
	}
	special := map[string]bool{}
	for _, id := range copies {
		special[id] = true
	}
	for _, id := range weighted {
		special[id] = true
	}
	for _, site := range sites.Sites {
		if !special[site.ID] {
			expected[key{site.ID, site.Function, "original", "", ""}] = 1
		}
	}
	// Every function outside the exact owned kernels must remain structurally
	// unchanged, not merely keep the same loop attribution or sampled result.
	ownedFunctions := map[string]bool{}
	for _, p := range out.Copies {
		ownedFunctions[p.Function] = true
	}
	for _, p := range out.Weighted {
		ownedFunctions[p.Function] = true
	}
	if len(source.Funcs) != len(actual.Funcs) || !reflect.DeepEqual(source.Globals, actual.Globals) || source.Target != actual.Target || !reflect.DeepEqual(source.Strings, actual.Strings) || !reflect.DeepEqual(source.StrKinds, actual.StrKinds) || !reflect.DeepEqual(source.Structs, actual.Structs) || !reflect.DeepEqual(source.Interfaces, actual.Interfaces) {
		return out, fmt.Errorf("mixedcopyweighted: unrelated module definitions changed")
	}
	for i, f := range source.Funcs {
		if actual.Funcs[i].Name != f.Name || (!ownedFunctions[f.Name] && !reflect.DeepEqual(f, actual.Funcs[i])) {
			return out, fmt.Errorf("mixedcopyweighted: unrelated function changed: %s", f.Name)
		}
	}
	sourcePaths := map[string]string{}
	for _, site := range sites.Sites {
		if !special[site.ID] {
			sourcePaths[site.Function+"/"+site.HIRPath] = site.ID
		}
	}
	var pathErr error
	for _, f := range actual.Funcs {
		if ownedFunctions[f.Name] {
			continue
		}
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for i, stmt := range b.Body {
				if id := sourcePaths[f.Name+"/"+fmt.Sprintf("%s/s%d", path, i)]; id != "" {
					if l, ok := actual.LoopLineage[stmt]; !ok || l.SiteID != id || l.Role != "original" || l.Rule != "" || l.PreviousRule != "" {
						pathErr = fmt.Errorf("mixedcopyweighted: original loop path attribution changed: %s/%s", f.Name, path)
					}
				}
			}
		})
	}
	if pathErr != nil {
		return out, pathErr
	}
	seen := map[key]int{}
	for _, record := range CurrentLoopLineage(actual) {
		l := record.Lineage
		k := key{l.SiteID, record.Function, l.Role, l.Rule, l.PreviousRule}
		if expected[k] == 0 {
			return out, fmt.Errorf("mixedcopyweighted: unexpected reachable lineage %v", k)
		}
		seen[k]++
	}
	for k, count := range expected {
		if seen[k] != count {
			return out, fmt.Errorf("mixedcopyweighted: missing/duplicate reachable lineage %v", k)
		}
	}
	return out, nil
}
