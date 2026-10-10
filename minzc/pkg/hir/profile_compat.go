package hir

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// ProfileContext is supplied by the execution harness. IDs must change when
// compiler/VM semantics, target costs/ABI, hosts or workload inputs change.
// Dependencies includes source units with no loops and external data/resources.
// This API checks equality; it cannot attest the harness's declarations.
type ProfileContext struct {
	CompilerID     string            `json:"compiler_id"`
	SemanticsID    string            `json:"semantics_id"`
	TargetID       string            `json:"target_id"`
	WorkloadSHA256 string            `json:"workload_sha256"`
	HostsSHA256    string            `json:"hosts_sha256"`
	Dependencies   map[string]string `json:"dependencies"`
}

type ProfileCompatibility struct {
	Schema        string         `json:"schema"`
	Context       ProfileContext `json:"context"`
	HIRSHA256     string         `json:"hir_sha256"`
	OriginsSHA256 string         `json:"origins_sha256"`
}

func validSHA256(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == sha256.Size && s == hex.EncodeToString(b)
}

func validateProfileContext(c ProfileContext) error {
	if c.CompilerID == "" || c.SemanticsID == "" || c.TargetID == "" || !validSHA256(c.WorkloadSHA256) || !validSHA256(c.HostsSHA256) || c.Dependencies == nil {
		return fmt.Errorf("loop profile: incomplete execution context")
	}
	for name, hash := range c.Dependencies {
		if name == "" || !validSHA256(hash) {
			return fmt.Errorf("loop profile: invalid dependency %q", name)
		}
	}
	return nil
}

// BuildProfileCompatibility conservatively fingerprints all semantic HIR fields,
// including concrete node/type tags, type internals, global initial bytes, ABI
// attributes and imported function bodies. Diagnostic fields are excluded.
// A cycle or unsupported representation refuses hashing rather than omitting it.
func BuildProfileCompatibility(m *Module, c ProfileContext) (*ProfileCompatibility, error) {
	if err := Verify(m); err != nil {
		return nil, err
	}
	if err := validateProfileContext(c); err != nil {
		return nil, err
	}
	semantic := *m
	semantic.SourceLocations = nil
	semantic.LoopLineage = nil
	semantic.PointerWalkPolicy = nil
	semantic.SourceUnits = nil
	semantic.SourceUnitsComplete = false
	semantic.AssertStats = nil
	semantic.Warnings = nil
	semantic.Asserts = nil
	semantic.Sandboxes = nil
	var buf bytes.Buffer
	if err := fingerprintValue(&buf, reflect.ValueOf(semantic), map[uintptr]bool{}); err != nil {
		return nil, err
	}
	hirHash := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
	// Include all attached statement origins, not just loops. Sort encoded pairs
	// so allocation address and map iteration order never affect the fingerprint.
	origins := []string{}
	for unit, hash := range m.SourceUnits {
		if !validSHA256(hash) || c.Dependencies[unit] != hash {
			return nil, fmt.Errorf("loop profile: missing or stale source unit %q", unit)
		}
	}
	for stmt, loc := range m.SourceLocations {
		if loc.Unit == "" || loc.ByteOffset < 0 || loc.Line < 1 || loc.Column < 1 || !validSHA256(loc.SourceSHA256) {
			return nil, fmt.Errorf("loop profile: invalid source origin")
		}
		if h, ok := c.Dependencies[loc.Unit]; !ok || h != loc.SourceSHA256 {
			return nil, fmt.Errorf("loop profile: missing or stale dependency %q", loc.Unit)
		}
		var node bytes.Buffer
		if err := fingerprintValue(&node, reflect.ValueOf(stmt), map[uintptr]bool{}); err != nil {
			return nil, err
		}
		encoded, _ := json.Marshal([]any{node.String(), loc})
		origins = append(origins, string(encoded))
	}
	sort.Strings(origins)
	data, _ := json.Marshal(origins)
	copyContext := c
	copyContext.Dependencies = map[string]string{}
	for n, h := range c.Dependencies {
		copyContext.Dependencies[n] = h
	}
	return &ProfileCompatibility{Schema: "minz.loop-profile-compatibility/v1", Context: copyContext, HIRSHA256: hirHash, OriginsSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, nil
}

// The encoding carries Go concrete type names and field names, length-delimited
// strings and container lengths. CompilerID makes this representation versioned.
func fingerprintValue(w *bytes.Buffer, v reflect.Value, active map[uintptr]bool) error {
	if !v.IsValid() {
		w.WriteString("invalid;")
		return nil
	}
	fmt.Fprintf(w, "%q:%q:", v.Type().PkgPath(), v.Type().String())
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			w.WriteString("nil;")
			return nil
		}
		return fingerprintValue(w, v.Elem(), active)
	case reflect.Pointer:
		if v.IsNil() {
			w.WriteString("nil;")
			return nil
		}
		addr := v.Pointer()
		if active[addr] {
			return fmt.Errorf("loop profile: cyclic HIR representation %s", v.Type())
		}
		active[addr] = true
		err := fingerprintValue(w, v.Elem(), active)
		delete(active, addr)
		return err
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fmt.Fprintf(w, "%q=", v.Type().Field(i).Name)
			if err := fingerprintValue(w, v.Field(i), active); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			w.WriteString("nil;")
			return nil
		}
		fmt.Fprintf(w, "%d[", v.Len())
		for i := 0; i < v.Len(); i++ {
			if err := fingerprintValue(w, v.Index(i), active); err != nil {
				return err
			}
		}
		w.WriteString("];")
	case reflect.Map:
		if v.IsNil() {
			w.WriteString("nil;")
			return nil
		}
		return fmt.Errorf("loop profile: unsupported non-nil map %s", v.Type())
	case reflect.String:
		fmt.Fprintf(w, "%q;", v.String())
	case reflect.Bool:
		fmt.Fprintf(w, "%t;", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprintf(w, "%d;", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		fmt.Fprintf(w, "%d;", v.Uint())
	default:
		return fmt.Errorf("loop profile: unsupported HIR representation %s", v.Type())
	}
	return nil
}

const maxLoopProfileBytes = 8 << 20

// LoadLoopProfile accepts only complete, mapped, compatible input-HIR VM
// observations. It never enables rewrites or interprets counts as legality.
// Expected compatibility is recomputed from current HIR and harness context.
func LoadLoopProfile(r io.Reader, m *Module, c ProfileContext) (*LoopProfile, error) {
	expected, err := BuildProfileCompatibility(m, c)
	if err != nil {
		return nil, err
	}
	sites, err := BuildLoopSiteMap(m)
	if err != nil {
		return nil, err
	}
	if sites.UnmappedLoops != 0 {
		return nil, fmt.Errorf("loop profile: current HIR has unmapped loops")
	}
	data, err := io.ReadAll(io.LimitReader(r, maxLoopProfileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLoopProfileBytes {
		return nil, fmt.Errorf("loop profile: exceeds size limit")
	}
	// A preliminary token walk rejects duplicate object keys, including nested
	// objects; encoding/json otherwise silently accepts last-key-wins documents.
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := checkJSONValue(dec); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("loop profile: trailing JSON")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for _, k := range []string{"schema", "stage", "compatibility", "sites", "counts", "guards", "completed_calls", "failed_calls"} {
		value, ok := raw[k]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("loop profile: missing %s", k)
		}
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw["counts"], &rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		for _, k := range []string{"site_id", "entries", "body_entries"} {
			v, ok := row[k]
			if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return nil, fmt.Errorf("loop profile: missing count %s", k)
			}
		}
	}
	var guardRows []map[string]json.RawMessage
	if err := json.Unmarshal(raw["guards"], &guardRows); err != nil {
		return nil, err
	}
	for _, row := range guardRows {
		for _, k := range []string{"site_id", "equal_entries", "fast_entries", "fallback_entries", "fast_body_entries", "fallback_body_entries"} {
			v, ok := row[k]
			if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return nil, fmt.Errorf("loop profile: missing guard %s", k)
			}
		}
	}
	var p LoopProfile
	dec = json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("loop profile: %w", err)
	}
	if p.Schema != "minz.input-hir-loop-observations/v4" || p.Stage != "unoptimised-mir2-vm" {
		return nil, fmt.Errorf("loop profile: unsupported schema or stage")
	}
	equalJSON := func(a, b any) bool { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return bytes.Equal(x, y) }
	if !equalJSON(p.Compatibility, expected) {
		return nil, fmt.Errorf("loop profile: incompatible execution context or HIR")
	}
	if !equalJSON(p.Sites, sites) {
		return nil, fmt.Errorf("loop profile: site map mismatch")
	}
	if p.CompletedCalls == 0 || p.FailedCalls != 0 {
		return nil, fmt.Errorf("loop profile: empty or failed sample")
	}
	if len(p.Counts) != len(sites.Sites) {
		return nil, fmt.Errorf("loop profile: count coverage mismatch")
	}
	seen := map[string]bool{}
	known := map[string]bool{}
	for _, s := range sites.Sites {
		known[s.ID] = true
	}
	for _, count := range p.Counts {
		if !known[count.SiteID] || seen[count.SiteID] {
			return nil, fmt.Errorf("loop profile: unknown or duplicate count site")
		}
		seen[count.SiteID] = true
		if count.Entries == 0 && count.BodyEntries != 0 {
			return nil, fmt.Errorf("loop profile: body entries without loop entry")
		}
	}
	counts := map[string]LoopCounts{}
	for _, count := range p.Counts {
		counts[count.SiteID] = count
	}
	eligible := map[string]bool{}
	reserved := map[string]bool{}
	for _, global := range m.Globals {
		reserved[global.Name] = true
	}
	for _, f := range m.Funcs {
		if plan := findGracePointerWalk(f, f.Body, reserved); plan != nil {
			for _, site := range sites.Sites {
				if site.Function == f.Name && site.HIRPath == "body/s2" {
					eligible[site.ID] = true
				}
			}
		}
	}
	guardSeen := map[string]bool{}
	for _, g := range p.Guards {
		count, ok := counts[g.SiteID]
		if !ok || !eligible[g.SiteID] || guardSeen[g.SiteID] {
			return nil, fmt.Errorf("loop profile: unknown or duplicate guard site")
		}
		guardSeen[g.SiteID] = true
		if g.EqualEntries > g.FastEntries || g.FastEntries > count.Entries || g.FallbackEntries != count.Entries-g.FastEntries || g.FastBodyEntries > count.BodyEntries || g.FallbackBodyEntries != count.BodyEntries-g.FastBodyEntries {
			return nil, fmt.Errorf("loop profile: incomplete or inconsistent guard observations")
		}
		if (g.FastEntries == 0 && g.FastBodyEntries != 0) || (g.FallbackEntries == 0 && g.FallbackBodyEntries != 0) {
			return nil, fmt.Errorf("loop profile: guard iterations without entry")
		}
	}
	sort.Slice(p.Counts, func(i, j int) bool { return p.Counts[i].SiteID < p.Counts[j].SiteID })
	return &p, nil
}

func checkJSONValue(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[strings.ToLower(name)] {
				return fmt.Errorf("loop profile: duplicate or invalid JSON key %v", key)
			}
			seen[strings.ToLower(name)] = true
			if err := checkJSONValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := checkJSONValue(d); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("loop profile: invalid JSON delimiter")
	}
	_, err = d.Token()
	return err
}
