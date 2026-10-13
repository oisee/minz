package hir

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// FunctionSite binds a complete input function, not a loop or invented source
// line. SourceUnitsSHA256 hashes the sorted actual input-unit/hash pairs.
type FunctionSite struct {
	ID                string `json:"id"`
	Function          string `json:"function"`
	SourceUnitsSHA256 string `json:"source_units_sha256"`
	ABISHA256         string `json:"abi_sha256"`
	BodySHA256        string `json:"body_sha256"`
}
type FunctionSiteMap struct {
	OriginsSHA256 string            `json:"origins_sha256"`
	Schema        string            `json:"schema"`
	Stage         string            `json:"stage"`
	SourceUnits   map[string]string `json:"source_units"`
	Sites         []FunctionSite    `json:"sites"`
}

func orderedByteDigest(v any) (string, error) {
	var b bytes.Buffer
	if e := fingerprintValue(&b, reflect.ValueOf(v), map[uintptr]bool{}); e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", sha256.Sum256(b.Bytes())), nil
}
func orderedByteStatements(m *Module) (map[string]Stmt, error) {
	out := map[string]Stmt{}
	seen := map[Stmt]bool{}
	var bad error
	for _, f := range m.Funcs {
		walkGraceBlocks(f.Body, "body", func(p string, b *Block) {
			for j, s := range b.Body {
				if seen[s] {
					bad = fmt.Errorf("function sites: shared statement identity")
				}
				seen[s] = true
				out[fmt.Sprintf("%s/%s/s%d", f.Name, p, j)] = s
			}
		})
	}
	for s, l := range m.SourceLocations {
		if !seen[s] || l.Unit == "" || l.ByteOffset < 0 || l.Line < 1 || l.Column < 1 || !validSHA256(l.SourceSHA256) || m.SourceUnits[l.Unit] != l.SourceSHA256 {
			bad = fmt.Errorf("function sites: detached or invalid provenance")
		}
	}
	return out, bad
}
func BuildFunctionSiteMap(m *Module) (*FunctionSiteMap, error) {
	if m == nil || !m.SourceUnitsComplete || len(m.SourceUnits) == 0 {
		return nil, fmt.Errorf("function sites: complete source inputs required")
	}
	if e := Verify(m); e != nil {
		return nil, e
	}
	paths, e := orderedByteStatements(m)
	if e != nil {
		return nil, e
	}
	originKeys := []string{}
	for path, stmt := range paths {
		if _, ok := m.SourceLocations[stmt]; ok {
			originKeys = append(originKeys, path)
		}
	}
	sort.Strings(originKeys)
	originPairs := []any{}
	for _, path := range originKeys {
		originPairs = append(originPairs, []any{path, m.SourceLocations[paths[path]]})
	}
	originBytes, _ := json.Marshal(originPairs)
	originHash := fmt.Sprintf("%x", sha256.Sum256(originBytes))
	keys := []string{}
	for unit, h := range m.SourceUnits {
		if unit == "" || !validSHA256(h) {
			return nil, fmt.Errorf("function sites: invalid source hash")
		}
		keys = append(keys, unit)
	}
	sort.Strings(keys)
	pairs := [][2]string{}
	for _, unit := range keys {
		pairs = append(pairs, [2]string{unit, m.SourceUnits[unit]})
	}
	data, _ := json.Marshal(pairs)
	sourceHash := fmt.Sprintf("%x", sha256.Sum256(data))
	out := &FunctionSiteMap{OriginsSHA256: originHash, Schema: "minz.function-sites/v1", Stage: "input-hir", SourceUnits: map[string]string{}, Sites: []FunctionSite{}}
	for unit, h := range m.SourceUnits {
		out.SourceUnits[unit] = h
	}
	names := map[string]bool{}
	for _, f := range m.Funcs {
		if names[f.Name] {
			return nil, fmt.Errorf("function sites: duplicate function")
		}
		names[f.Name] = true
		if f.Body == nil {
			continue
		}
		abi := *f
		abi.Body = nil
		ah, e := orderedByteDigest(abi)
		if e != nil {
			return nil, e
		}
		bh, e := orderedByteDigest(f.Body)
		if e != nil {
			return nil, e
		}
		identity, _ := json.Marshal([]string{out.Schema, sourceHash, originHash, f.Name, ah, bh})
		out.Sites = append(out.Sites, FunctionSite{ID: fmt.Sprintf("%x", sha256.Sum256(identity)), Function: f.Name, SourceUnitsSHA256: sourceHash, ABISHA256: ah, BodySHA256: bh})
	}
	sort.Slice(out.Sites, func(i, j int) bool { return out.Sites[i].ID < out.Sites[j].ID })
	return out, nil
}
