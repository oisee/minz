package hir

import "fmt"

// LoopLineage records attribution, never legality or equivalence evidence.
// Roles are original, invariant-base, fast, fallback; path is current HIR.
type LoopLineage struct {
	SiteID       string `json:"site_id"`
	Role         string `json:"role"`
	Rule         string `json:"rule,omitempty"`
	PreviousRule string `json:"previous_rule,omitempty"`
}

type LoopLineageRecord struct {
	Function string      `json:"function"`
	HIRPath  string      `json:"hir_path"`
	Lineage  LoopLineage `json:"lineage"`
}

// SeedLoopLineage must run on input HIR before rewriting. It refuses reseeding.
func SeedLoopLineage(m *Module) error {
	if m.LoopLineage != nil {
		return fmt.Errorf("HIR lineage: already seeded")
	}
	sites, err := BuildLoopSiteMap(m)
	if err != nil {
		return err
	}
	paths := map[string]map[string]string{}
	for _, s := range sites.Sites {
		if paths[s.Function] == nil {
			paths[s.Function] = map[string]string{}
		}
		paths[s.Function][s.HIRPath] = s.ID
	}
	m.LoopLineage = map[Stmt]LoopLineage{}
	for _, f := range m.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for i, s := range b.Body {
				if id := paths[f.Name][fmt.Sprintf("%s/s%d", path, i)]; id != "" {
					m.LoopLineage[s] = LoopLineage{SiteID: id, Role: "original"}
				}
			}
		})
	}
	return nil
}

// CurrentLoopLineage reports only nodes still in the current HIR, in traversal
// order. Missing attribution is left missing, never guessed from a source path.
func CurrentLoopLineage(m *Module) []LoopLineageRecord {
	out := []LoopLineageRecord{}
	for _, f := range m.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for i, s := range b.Body {
				if l, ok := m.LoopLineage[s]; ok {
					out = append(out, LoopLineageRecord{Function: f.Name, HIRPath: fmt.Sprintf("%s/s%d", path, i), Lineage: l})
				}
			}
		})
	}
	return out
}
