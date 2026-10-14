package hir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// SourceLocation uses byte offsets and byte columns in the frontend input unit.
// Unit is a caller-provided logical name, not necessarily an absolute filename.
// The whole input fingerprint conservatively expires all its sites after edits.
type SourceLocation struct {
	Unit         string `json:"unit"`
	ByteOffset   int    `json:"byte_offset"`
	Line         int    `json:"line"`
	Column       int    `json:"byte_column"`
	SourceSHA256 string `json:"source_sha256"`
	// Generated statements point at the actual invocation, not an invented
	// source token. The definition fingerprint expires their profile attribution.
	Metafunction       string `json:"metafunction,omitempty"`
	MetafunctionSHA256 string `json:"metafunction_sha256,omitempty"`
}

// LoopSite identifies an input structured HIR loop, including an explicitly
// attributed metafunction expansion. It does not certify
// legality or claim correspondence to transformed/duplicated MIR instructions.
type LoopSite struct {
	ID       string         `json:"id"`
	Function string         `json:"function"`
	Kind     string         `json:"kind"`
	Ordinal  int            `json:"ordinal"`
	HIRPath  string         `json:"hir_path"`
	Origin   SourceLocation `json:"origin"`
}

type LoopSiteMap struct {
	Schema        string     `json:"schema"`
	Stage         string     `json:"stage"`
	Sites         []LoopSite `json:"sites"`
	UnmappedLoops int        `json:"unmapped_loops"`
}

// BuildLoopSiteMap is a diagnostic/source attribution layer for future PGO.
// Missing origins refuse attribution, rather than manufacturing source sites.
// Call before HIR rewrites/splitting; lineage across those passes is not supplied.
func BuildLoopSiteMap(m *Module) (*LoopSiteMap, error) {
	if err := Verify(m); err != nil {
		return nil, err
	}
	result := &LoopSiteMap{Schema: "minz.hir-loop-sites/v1", Stage: "input-hir", Sites: []LoopSite{}}
	ordinals := map[string]int{}
	var mapErr error
	for _, f := range m.Funcs {
		walkGraceBlocks(f.Body, "body", func(path string, b *Block) {
			for i, s := range b.Body {
				kind := ""
				switch s.(type) {
				case *WhileStmt:
					kind = "while"
				case *ForRangeStmt:
					kind = "for-range"
				case *ForEachStmt:
					kind = "for-each"
				}
				if kind == "" {
					continue
				}
				origin, ok := m.SourceLocations[s]
				if !ok {
					result.UnmappedLoops++
					continue
				}
				rawHash, err := hex.DecodeString(origin.SourceSHA256)
				if origin.Unit == "" || origin.ByteOffset < 0 || origin.Line < 1 || origin.Column < 1 || err != nil || len(rawHash) != sha256.Size {
					mapErr = fmt.Errorf("HIR site: invalid source origin for %s %s", f.Name, kind)
					continue
				}
				metaHash, metaErr := hex.DecodeString(origin.MetafunctionSHA256)
				if (origin.Metafunction == "" && origin.MetafunctionSHA256 != "") ||
					(origin.Metafunction != "" && (metaErr != nil || len(metaHash) != sha256.Size)) {
					mapErr = fmt.Errorf("HIR site: invalid metafunction origin for %s %s", f.Name, kind)
					continue
				}
				// JSON tuple encoding avoids ambiguity between names containing separators.
				key, _ := json.Marshal([]any{origin.Unit, f.Name, origin.ByteOffset, kind})
				ordinal := ordinals[string(key)]
				ordinals[string(key)]++
				identity, _ := json.Marshal([]any{string(key), ordinal})
				id := fmt.Sprintf("%x", sha256.Sum256(identity))
				result.Sites = append(result.Sites, LoopSite{ID: id, Function: f.Name, Kind: kind, Ordinal: ordinal, HIRPath: fmt.Sprintf("%s/s%d", path, i), Origin: origin})
			}
		})
	}
	if mapErr != nil {
		return nil, mapErr
	}
	sort.Slice(result.Sites, func(i, j int) bool { return result.Sites[i].ID < result.Sites[j].ID })
	return result, nil
}
