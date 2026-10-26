package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureConfig(t *testing.T) runConfig {
	t.Helper()
	root := filepath.Clean("../../..")
	base := filepath.Join(root, "research/autosiril-native/midi/fixtures")
	var p struct {
		Source     map[string]string `json:"source_sha256"`
		Fixtures   map[string]string `json:"fixture_sha256"`
		Executions int               `json:"target_executions"`
	}
	b, e := os.ReadFile(filepath.Join(base, "provenance.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	if p.Executions != 342 || len(p.Source) == 0 || len(p.Fixtures) != 3 {
		t.Fatal("invalid frozen provenance")
	}
	for _, group := range []map[string]string{p.Source, p.Fixtures} {
		for path, want := range group {
			data, e := os.ReadFile(filepath.Join(root, path))
			if e != nil || hash(data) != want {
				t.Fatalf("reference provenance changed: %s; rerun fresh Ruby/Crystal gate", path)
			}
		}
	}
	return runConfig{filepath.Join(root, "research/autosiril-native/midi/pair.nanz"), filepath.Join(base, "music.lanz"), filepath.Join(base, "music-manifest.json"), filepath.Join(root, "stdlib"), filepath.Join(base, "cases.json"), t.TempDir()}
}

func TestNativeMIDIReplayFrozenRubyAndBoundaryCases(t *testing.T) {
	cfg := fixtureConfig(t)
	if e := run(cfg); e != nil {
		t.Fatal(e)
	}
	var receipt struct {
		Executions int `json:"target_executions"`
		Cases      []struct {
			Name    string            `json:"name"`
			Engines []json.RawMessage `json:"engines"`
		} `json:"cases"`
	}
	if e := json.Unmarshal(read(filepath.Join(cfg.Out, "receipt.json")), &receipt); e != nil {
		t.Fatal(e)
	}
	if receipt.Executions != 342 || len(receipt.Cases) != 114 {
		t.Fatal("missing case/chunk/backend execution")
	}
	seen := map[string]int{}
	for _, c := range receipt.Cases {
		if len(c.Engines) != 3 {
			t.Fatal("missing backend")
		}
		seen[c.Name]++
	}
	if len(seen) != 38 {
		t.Fatal("missing wire cases")
	}
	for name, n := range seen {
		if n != 3 {
			t.Fatal("missing chunk geometry", name)
		}
	}
}

func oneCase(t *testing.T, cfg runConfig, name string) runConfig {
	t.Helper()
	var in struct {
		Cases []testCase `json:"cases"`
	}
	if e := json.Unmarshal(read(cfg.Input), &in); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, c := range in.Cases {
		if c.Name == name {
			c.Chunks = []int{256}
			in.Cases = []testCase{c}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing case", name)
	}
	cfg.Input = filepath.Join(t.TempDir(), "input.json")
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(cfg.Input, b, 0600); e != nil {
		t.Fatal(e)
	}
	return cfg
}

func TestNativeMIDIGuardsRejectRestoringWrites(t *testing.T) {
	cfg := oneCase(t, fixtureConfig(t), "single")
	original := string(read(cfg.Source))
	anchor := "input_consumed = 0\n    while input_consumed < input_length"
	if strings.Count(original, anchor) != 1 {
		t.Fatal("mutation anchor changed")
	}
	for _, test := range []struct{ name, write, want string }{
		{"input", "input_chunk[0] = input_chunk[0]", "native wrote transport input"},
		{"padding", "note_bytes[4092] = 218", "native wrote arena padding"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := cfg
			c.Out = t.TempDir()
			c.Source = filepath.Join(t.TempDir(), "mutant.nanz")
			changed := strings.Replace(original, anchor, "input_consumed = 0\n    "+test.write+"\n    while input_consumed < input_length", 1)
			if e := os.WriteFile(c.Source, []byte(changed), 0600); e != nil {
				t.Fatal(e)
			}
			if e := run(c); e == nil || !strings.Contains(e.Error(), test.want) {
				t.Fatalf("restoring write guard failed: %v", e)
			}
		})
	}
}

func TestNativeMIDIRejectsIncompatibleQuantizerABI(t *testing.T) {
	cfg := fixtureConfig(t)
	source, music, meta := read(cfg.Source), read(cfg.Music), read(cfg.Manifest)
	for _, name := range []string{"missing", "duplicate", "argument", "result", "fallible", "actual-fallible"} {
		t.Run(name, func(t *testing.T) {
			var m struct {
				Functions []binding `json:"functions"`
			}
			if e := json.Unmarshal(meta, &m); e != nil {
				t.Fatal(e)
			}
			at := -1
			for i, b := range m.Functions {
				if b.Source == "crystal_quantize" {
					at = i
				}
			}
			if at < 0 {
				t.Fatal("missing actual binding")
			}
			code := string(music)
			switch name {
			case "missing":
				m.Functions = append(m.Functions[:at], m.Functions[at+1:]...)
			case "duplicate":
				m.Functions = append(m.Functions, m.Functions[at])
			case "argument":
				m.Functions[at].Args[0] = "u16"
			case "result":
				m.Functions[at].Result = "u16"
			case "fallible":
				m.Functions[at].Fallible = true
			case "actual-fallible":
				old := m.Functions[at].Symbol
				m.Functions[at].Symbol = old + "?"
				m.Functions[at].Fallible = false
				code = strings.ReplaceAll(code, old, old+"?")
			}
			b, e := json.Marshal(m)
			if e != nil {
				t.Fatal(e)
			}
			_, e = build(source, []byte(code), b, cfg.Stdlib)
			if e == nil {
				t.Fatal("incompatible quantizer admitted")
			}
			if name == "actual-fallible" && !strings.Contains(e.Error(), "error ABI") && !strings.Contains(e.Error(), "actual quantizer is fallible") {
				t.Fatalf("actual fallibility not checked by preflight: %v", e)
			}
		})
	}
}

func TestNativeMIDIRejectsUnsafeMatrixTransport(t *testing.T) {
	cfg := oneCase(t, fixtureConfig(t), "single")
	for _, name := range []string{"zero-chunk", "long-chunk", "duplicate", "reference-pitch", "reference-source"} {
		t.Run(name, func(t *testing.T) {
			var in struct {
				Cases []testCase `json:"cases"`
			}
			if e := json.Unmarshal(read(cfg.Input), &in); e != nil {
				t.Fatal(e)
			}
			switch name {
			case "zero-chunk":
				in.Cases[0].Chunks = []int{0}
			case "long-chunk":
				in.Cases[0].Chunks = []int{257}
			case "duplicate":
				in.Cases = append(in.Cases, in.Cases[0])
			case "reference-pitch":
				in.Cases[0].Expected.Notes[0].Pitch = 128
			case "reference-source":
				in.Cases[0].Expected.Notes[0].Source = 64
			}
			c := cfg
			c.Input = filepath.Join(t.TempDir(), "input.json")
			c.Out = t.TempDir()
			b, e := json.Marshal(in)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(c.Input, b, 0600); e != nil {
				t.Fatal(e)
			}
			if e = run(c); e == nil {
				t.Fatal("unsafe matrix admitted")
			}
		})
	}
}

func TestNativeMIDIRejectsFreeGlobalsInActualExport(t *testing.T) {
	cfg := fixtureConfig(t)
	source, music, meta := read(cfg.Source), string(read(cfg.Music)), read(cfg.Manifest)
	anchor := "(var whole u32"
	if strings.Count(music, anchor) != 1 {
		t.Fatal("actual quantizer body anchor changed")
	}
	for _, expr := range []string{"(set fault 0)", "(var hidden u32 (cast fault u32))", "(var hidden u32 (cast (addr fault) u32))"} {
		t.Run(expr, func(t *testing.T) {
			mutant := strings.Replace(music, anchor, expr+"\n"+anchor, 1)
			if _, e := build(source, []byte(mutant), meta, cfg.Stdlib); e == nil {
				t.Fatal("actual export captured a parent global before binding")
			}
		})
	}
}
