package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixturePaths(t *testing.T) (string, string, string, string) {
	t.Helper()
	root := filepath.Clean("../../..")
	base := filepath.Join(root, "research/autosiril-native/fixtures")
	var p struct {
		Source  map[string]string `json:"source_sha256"`
		Fixture map[string]string `json:"fixture_sha256"`
	}
	b, e := os.ReadFile(filepath.Join(base, "provenance.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	for _, files := range []map[string]string{p.Source, p.Fixture} {
		for file, want := range files {
			b, e := os.ReadFile(filepath.Join(root, file))
			if e != nil {
				t.Fatal(e)
			}
			if digest(b) != want {
				t.Fatalf("reference provenance changed: %s; rerun fresh Ruby/Crystal gate and review snapshots", file)
			}
		}
	}
	return filepath.Join(root, "research/autosiril-vm/lazy_flatten.nanz"), filepath.Join(base, "music.lanz"), filepath.Join(base, "music-manifest.json"), filepath.Join(base, "cases.json")
}
func TestNativeFlatFrozenRubyVectors(t *testing.T) {
	engine, music, meta, cases := fixturePaths(t)
	for _, group := range []struct {
		name, path string
		count      int
	}{{"one-bank", cases, 50}, {"two-bank", filepath.Join(filepath.Dir(cases), "two-bank-cases.json"), 7}} {
		t.Run(group.name, func(t *testing.T) {
			out := t.TempDir()
			if e := run(engine, music, meta, group.path, out); e != nil {
				t.Fatal(e)
			}
			var r struct {
				Cases []caseResult `json:"cases"`
			}
			b, e := os.ReadFile(filepath.Join(out, "receipt.json"))
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(b, &r); e != nil {
				t.Fatal(e)
			}
			if len(r.Cases) != group.count {
				t.Fatal("frozen corpus changed")
			}
			for _, c := range r.Cases {
				if len(c.Engines) != 3 {
					t.Fatal("missing execution backend")
				}
			}
		})
	}

}

func TestNativeFlatWholeChronosAndThreeBankBounds(t *testing.T) {
	engine, music, meta, _ := fixturePaths(t)
	root := filepath.Clean("../../..")
	base := filepath.Join(root, "research/autosiril-native/chronos/evidence")
	data, err := os.ReadFile(filepath.Join(base, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Sources    map[string]string `json:"source_sha256"`
		Input      string            `json:"input_sha256"`
		Music      string            `json:"music_sha256"`
		Manifest   string            `json:"manifest_sha256"`
		Notes      int               `json:"paired_notes"`
		Events     []int             `json:"event_counts"`
		Executions int               `json:"target_executions"`
	}
	if err := json.Unmarshal(data, &proof); err != nil {
		t.Fatal(err)
	}
	for file, want := range proof.Sources {
		b, err := os.ReadFile(filepath.Join(root, file))
		if err != nil || digest(b) != want {
			t.Fatalf("Chronos source provenance changed: %s; rerun fresh Ruby/Crystal verifier", file)
		}
	}
	inputPath := filepath.Join(base, "input.json")
	for file, want := range map[string]string{inputPath: proof.Input, music: proof.Music, meta: proof.Manifest} {
		b, err := os.ReadFile(file)
		if err != nil || digest(b) != want {
			t.Fatalf("Chronos reference provenance changed: %s", file)
		}
	}
	if proof.Notes != 1485 || len(proof.Events) != 3 || proof.Events[0] != 771 || proof.Events[1] != 719 || proof.Events[2] != 663 || proof.Executions != 21 {
		t.Fatal("whole-score reference corpus changed")
	}
	out := t.TempDir()
	if err := run(engine, music, meta, inputPath, out); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(out, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Cases []caseResult `json:"cases"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Cases) != 7 {
		t.Fatal("missing whole-song/capacity cases")
	}
	for _, c := range result.Cases {
		if len(c.Engines) != 3 {
			t.Fatal("missing whole-song execution backend")
		}
	}
}

func TestNativeFlatThreeBankTransportBounds(t *testing.T) {
	for _, c := range []config{{0, 1024}, {4, 1024}, {3, 0}, {3, 1025}} {
		if checkedConfig(c) == nil {
			t.Fatalf("out-of-range profile admitted: %+v", c)
		}
	}
	c := config{3, 1024}
	if err := checkedConfig(c); err != nil {
		t.Fatal(err)
	}
	tc := testCase{Name: "bounds", Notes: make([]note, 2046), Mode: "p", Limit: 1024}
	if err := checkCase(tc, c); err != nil {
		t.Fatal(err)
	}
	tc.Limit = 1025
	if checkCase(tc, c) == nil {
		t.Fatal("output limit above capacity admitted")
	}
	tc.Limit = 1024
	tc.Notes = append(tc.Notes, note{})
	if checkCase(tc, c) == nil {
		t.Fatal("note beyond third-bank capacity admitted")
	}
}
func TestNativeFlatRejectsUnsafeTransport(t *testing.T) {
	engine, music, meta, cases := fixturePaths(t)
	data, e := os.ReadFile(cases)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"row", "pitch", "source", "limit", "capacity", "banks"} {
		t.Run(name, func(t *testing.T) {
			var in input
			if e := json.Unmarshal(data, &in); e != nil {
				t.Fatal(e)
			}
			in.Cases = []testCase{{Name: "unsafe", Notes: []note{{Start: 0, Off: 1, Pitch: 60}}, Mode: "p", Limit: 1}}
			switch name {
			case "row":
				in.Cases[0].Notes[0].Off = 65536
			case "pitch":
				in.Cases[0].Notes[0].Pitch = 128
			case "source":
				in.Cases[0].Notes[0].Source = 256
			case "limit":
				in.Cases[0].Limit = 129
			case "capacity":
				in.Cases[0].Notes = make([]note, 683)
			case "banks":
				in.Config.Banks = 0
			}
			p := filepath.Join(t.TempDir(), "input.json")
			b, e := json.Marshal(in)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			if e = run(engine, music, meta, p, t.TempDir()); e == nil {
				t.Fatal("unsafe transport admitted")
			}
		})
	}
}
func TestNativeFlatRejectsIncompatibleSelectorABI(t *testing.T) {
	engine, music, meta, _ := fixturePaths(t)
	source, e := os.ReadFile(engine)
	if e != nil {
		t.Fatal(e)
	}
	rendered, e := render(string(source), config{1, 128})
	if e != nil {
		t.Fatal(e)
	}
	code, e := os.ReadFile(music)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(meta)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"missing", "duplicate", "fallible", "result", "argument", "symbol"} {
		t.Run(name, func(t *testing.T) {
			var m manifest
			if e := json.Unmarshal(data, &m); e != nil {
				t.Fatal(e)
			}
			at := -1
			for i, f := range m.Functions {
				if f.Source == "crystal_prefer" {
					at = i
				}
			}
			if at < 0 {
				t.Fatal("missing real selector fixture")
			}
			switch name {
			case "missing":
				m.Functions = append(m.Functions[:at], m.Functions[at+1:]...)
			case "duplicate":
				m.Functions = append(m.Functions, m.Functions[at])
			case "fallible":
				m.Functions[at].Fallible = true
			case "result":
				m.Functions[at].Result = "u16"
			case "argument":
				m.Functions[at].Args[0] = "u16"
			case "symbol":
				m.Functions[at].Symbol = "missing_definition"
			}
			b, e := json.Marshal(m)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = buildHIR(rendered, string(code), string(b)); e == nil {
				t.Fatal("incompatible selector admitted")
			}
		})
	}
}
func TestNativeFlatRejectsUnreviewedSpillStorage(t *testing.T) {
	for _, asm := range []string{"_spill_bad: DB 1", "_spill_bad: DS 4", "_spill_bad: DB 0,0,0,0,0"} {
		if _, e := backendSpills(asm, map[string]int{"_spill_bad": origin}, make([]byte, 8)); e == nil {
			t.Fatal("unreviewed backend storage admitted", asm)
		}
	}
	source, e := os.ReadFile("../../../research/autosiril-vm/lazy_flatten.nanz")
	if e != nil {
		t.Fatal(e)
	}
	altered := strings.Replace(string(source), "global note_bytes:[u8;49152]", "global note_bytes:[u8;4000]", 1)
	if _, e = render(altered, config{1, 128}); e == nil {
		t.Fatal("changed arena anchor silently accepted")
	}
}

func TestNativeFlatRejectsActualFallibleSelector(t *testing.T) {
	engine, music, meta, _ := fixturePaths(t)
	source, e := os.ReadFile(engine)
	if e != nil {
		t.Fatal(e)
	}
	rendered, e := render(string(source), config{1, 128})
	if e != nil {
		t.Fatal(e)
	}
	code, e := os.ReadFile(music)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(meta)
	if e != nil {
		t.Fatal(e)
	}
	var m manifest
	if e = json.Unmarshal(data, &m); e != nil {
		t.Fatal(e)
	}
	old := ""
	for i, b := range m.Functions {
		if b.Source == "crystal_prefer" {
			old = b.Symbol
			m.Functions[i].Symbol = b.Symbol + "?"
			m.Functions[i].Fallible = false
		}
	}
	if old == "" {
		t.Fatal("missing selector")
	}
	mutated := strings.ReplaceAll(string(code), old, old+"?")
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = buildHIR(rendered, mutated, string(b)); e == nil || (!strings.Contains(e.Error(), "native selector error ABI") && !strings.Contains(e.Error(), "fallible actual selector")) {
		t.Fatalf("actual ABI was not rejected by preflight: %v", e)
	}
}
func TestNativeFlatGuardRejectsRestoringInputWrite(t *testing.T) {
	engine, music, meta, cases := fixturePaths(t)
	source, e := os.ReadFile(engine)
	if e != nil {
		t.Fatal(e)
	}
	old := "fun consume(source:u8,mode:u8,limit:u32) {"
	if strings.Count(string(source), old) != 1 {
		t.Fatal("source shape changed")
	}
	// Same final bytes and outputs: only the independent native write heatmap
	// distinguishes this forbidden store from a read-only stage execution.
	changed := strings.Replace(string(source), old, old+"\n    note_bytes[0] = note_bytes[0]", 1)
	dir := t.TempDir()
	badEngine := filepath.Join(dir, "bad.nanz")
	if e = os.WriteFile(badEngine, []byte(changed), 0600); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(cases)
	if e != nil {
		t.Fatal(e)
	}
	var in input
	if e = json.Unmarshal(data, &in); e != nil {
		t.Fatal(e)
	}
	selected := []testCase{}
	for _, c := range in.Cases {
		if c.Name == "overlap_zero_p" {
			selected = append(selected, c)
		}
	}
	if len(selected) != 1 {
		t.Fatal("missing vector")
	}
	in.Cases = selected
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "input.json")
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	if e = run(badEngine, music, meta, path, filepath.Join(dir, "out")); e == nil || !strings.Contains(e.Error(), "native wrote read-only note arena") {
		t.Fatalf("restoring write not rejected by native guard: %v", e)
	}
}
