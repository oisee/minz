package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withSearchCLI(t *testing.T) (string, string) {
	t.Helper()
	old := memoryHostSearchCLI
	oldPolicy := memoryHostPolicyCLI
	changed := map[string]bool{}
	for _, n := range append(append([]string{}, memoryHostSearchFlagNames...), "memory-host-arena-append-policy", "memory-host-compare-policy", "memory-host-association", "memory-host-arena-append", "memory-host-profile") {
		f := rootCmd.Flags().Lookup(n)
		changed[n] = f.Changed
		f.Changed = false
	}
	source, workload := withMemoryHostCLITest(t)
	memoryHostCLI.Directory = ""
	memoryHostPolicyCLI = memoryHostPolicyCLIFlags{}
	memoryHostSearchCLI = memoryHostSearchCLIFlags{Directory: filepath.Join(filepath.Dir(source), "search-artifacts")}
	t.Cleanup(func() {
		memoryHostSearchCLI = old
		memoryHostPolicyCLI = oldPolicy
		for n, v := range changed {
			rootCmd.Flags().Lookup(n).Changed = v
		}
	})
	return source, workload
}
func TestSearchCLIAdmission(t *testing.T) {
	source, _ := withSearchCLI(t)
	if err := validateMemoryHostSearchCLI(source); err != nil {
		t.Fatal(err)
	}
	if len(memoryHostSearchCLIOutputNames()) != 50 {
		t.Fatal("complete output inventory")
	}
	old := memoryHostSearchCLI
	for _, kind := range []string{"missing-directory", "legacy-only", "preserve-only"} {
		memoryHostSearchCLI = old
		switch kind {
		case "missing-directory":
			memoryHostSearchCLI.Directory = ""
		case "legacy-only":
			memoryHostSearchCLI.LegacyAssociation = "a.json"
		case "preserve-only":
			memoryHostSearchCLI.PreserveAssociation = "b.json"
		}
		if validateMemoryHostSearchCLI(source) == nil {
			t.Fatal("partial mode accepted", kind)
		}
	}
	memoryHostSearchCLI = old
	for _, n := range []string{"memory-host-arena-append-policy", "memory-host-compare-policy", "memory-host-association", "memory-host-arena-append", "memory-host-profile"} {
		f := rootCmd.Flags().Lookup(n)
		f.Changed = true
		if validateMemoryHostSearchCLI(source) == nil {
			t.Fatal("explicit empty incompatible flag", n)
		}
		f.Changed = false
	}
	memoryHostSearchCLI = memoryHostSearchCLIFlags{}
	for _, n := range memoryHostSearchFlagNames {
		f := rootCmd.Flags().Lookup(n)
		f.Changed = true
		if !memoryHostSearchCLIRequested() {
			t.Fatal("explicit empty dispatch lost")
		}
		if compile(source) == nil {
			t.Fatal("empty search flag entered ordinary compile")
		}
		f.Changed = false
	}
	memoryHostSearchCLI = old
	for _, n := range memoryHostSearchFlagNames[1:] {
		f := rootCmd.Flags().Lookup(n)
		f.Changed = true
		if validateMemoryHostSearchCLI(source) == nil {
			t.Fatal("empty supplied input", n)
		}
		f.Changed = false
	}
}
func TestSearchCLIPathAliases(t *testing.T) {
	source, workload := withSearchCLI(t)
	dir := memoryHostSearchCLI.Directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	a, b := filepath.Join(filepath.Dir(source), "legacy.json"), filepath.Join(filepath.Dir(source), "preserve.json")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	memoryHostSearchCLI.LegacyAssociation = a
	memoryHostSearchCLI.PreserveAssociation = b
	for i, input := range []string{source, workload, a, b} {
		t.Run(filepath.Base(input), func(t *testing.T) {
			output := filepath.Join(dir, []string{"selected.bin", "policy0-mask3/target.bin", "association-legacy-v2.json", "policy1-mask3/allocation.json"}[i])
			if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
				t.Fatal(err)
			}
			if i%2 == 0 {
				if err := os.Link(input, output); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Symlink(input, output); err != nil {
					t.Fatal(err)
				}
			}
			defer os.Remove(output)
			if validateMemoryHostSearchCLI(source) == nil {
				t.Fatal("input/artifact alias accepted")
			}
		})
	}
	oldOut := outputFile
	outputFile = a
	if validateMemoryHostSearchCLI(source) == nil {
		t.Fatal("external output overwrites association")
	}
	outputFile = oldOut
}
func TestSearchCLIKnownSourceBeforeCollection(t *testing.T) {
	source, _ := withSearchCLI(t)
	err := runMemoryHostSearchCLI(source)
	if err == nil || !strings.Contains(err.Error(), "exact original SelfLanz") {
		t.Fatal("substituted source reached collection", err)
	}
	if _, err = os.Stat(memoryHostSearchCLI.Directory); !os.IsNotExist(err) {
		t.Fatal("refusal created output")
	}
}
func TestSearchCLISuppliedPreflight(t *testing.T) {
	source, _ := withSearchCLI(t)
	bytes, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, bytes, 0644); err != nil {
		t.Fatal(err)
	}
	memoryHostSearchCLI.LegacyAssociation = filepath.Join(filepath.Dir(source), "legacy.json")
	memoryHostSearchCLI.PreserveAssociation = filepath.Join(filepath.Dir(source), "preserve.json")
	for _, p := range []string{memoryHostSearchCLI.LegacyAssociation, memoryHostSearchCLI.PreserveAssociation} {
		if err = os.WriteFile(p, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err = runMemoryHostSearchCLI(source); err == nil {
		t.Fatal("invalid whole associations accepted")
	}
	if _, err = os.Stat(memoryHostSearchCLI.Directory); !os.IsNotExist(err) {
		t.Fatal("admission refusal created output")
	}
}

func TestSearchCLIOrdinaryRoutesUnaffected(t *testing.T) {
	source, _ := withSearchCLI(t)
	memoryHostSearchCLI = memoryHostSearchCLIFlags{}
	if memoryHostSearchCLIRequested() {
		t.Fatal("new mode enabled without flags")
	}
	memoryHostCLI.Directory = filepath.Join(filepath.Dir(source), "legacy")
	if err := validateMemoryHostCLI(source); err != nil {
		t.Fatal("legacy admission changed", err)
	}
	memoryHostCLI.Directory = ""
	memoryHostPolicyCLI = memoryHostPolicyCLIFlags{Directory: filepath.Join(filepath.Dir(source), "fixed"), Policy: "legacy"}
	if err := validateMemoryHostPolicyCLI(source); err != nil {
		t.Fatal("fixed admission changed", err)
	}
}
func TestSearchCLIAssociationCaps(t *testing.T) {
	source, _ := withSearchCLI(t)
	b, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, b, 0644); err != nil {
		t.Fatal(err)
	}
	memoryHostSearchCLI.LegacyAssociation = filepath.Join(filepath.Dir(source), "legacy.json")
	memoryHostSearchCLI.PreserveAssociation = filepath.Join(filepath.Dir(source), "preserve.json")
	for _, p := range []string{memoryHostSearchCLI.LegacyAssociation, memoryHostSearchCLI.PreserveAssociation} {
		if err = os.WriteFile(p, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{memoryHostSearchCLI.LegacyAssociation, memoryHostSearchCLI.PreserveAssociation} {
		f, err := os.OpenFile(p, os.O_WRONLY, 0644)
		if err != nil {
			t.Fatal(err)
		}
		if err = f.Truncate(int64(pipeline.MaxMemoryHostPolicyAssociationBytes + 1)); err != nil {
			t.Fatal(err)
		}
		f.Close()
		if err = runMemoryHostSearchCLI(source); err == nil {
			t.Fatal("oversized named input accepted")
		}
		if _, err = os.Stat(memoryHostSearchCLI.Directory); !os.IsNotExist(err) {
			t.Fatal("oversized refusal created output")
		}
		if err = os.WriteFile(p, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSearchCLICollectAndPairedReplay(t *testing.T) {
	source, workload := withSearchCLI(t)
	b, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, b, 0644); err != nil {
		t.Fatal(err)
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 2000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"source": {Address: 0xc000, Bytes: make([]byte, 4096)}, "strings": {Address: 0x9000, Bytes: make([]byte, 4096), Writable: true}, "nodes": {Address: 0xa000, Bytes: make([]byte, 4096), Writable: true}}, Globals: map[string][]byte{"src_buf": {0, 192}, "src_len": {0, 0}, "str_arena": {0, 144}, "str_bump": {0, 144}, "node_arena": {0, 160}, "node_bump": {0, 160}}, Calls: []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x9000, 0, 0}}, {Function: "intern_region", Arguments: []int64{0, 0}}}}
	b, err = json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(workload, b, 0644); err != nil {
		t.Fatal(err)
	}
	collect := memoryHostSearchCLI.Directory
	if err = runMemoryHostSearchCLI(source); err != nil {
		t.Fatal(err)
	}
	read := func(p string) []byte {
		t.Helper()
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	verify := func(dir, goal string) *pipeline.MemoryHostPolicySearchEvidence {
		t.Helper()
		payload := read(filepath.Join(dir, "search.json"))
		var e pipeline.MemoryHostPolicySearchEvidence
		if err := json.Unmarshal(payload, &e); err != nil {
			t.Fatal(err)
		}
		var d struct {
			Schema, SourceName, SourceSHA256, Goal, SearchSHA256, BinarySHA256 string
			Selected                                                           pipeline.MemoryHostPolicySearchChoice
			AssociationSHA256                                                  [2]string
			TStates                                                            int64
			Bytes                                                              int
		}
		// Decode with exact JSON tags from a generic object to keep checks explicit.
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(read(filepath.Join(dir, "decisions.json")), &fields); err != nil {
			t.Fatal(err)
		}
		for name, out := range map[string]any{"schema": &d.Schema, "source_name": &d.SourceName, "source_sha256": &d.SourceSHA256, "goal": &d.Goal, "search_sha256": &d.SearchSHA256, "binary_sha256": &d.BinarySHA256, "selected": &d.Selected, "association_sha256": &d.AssociationSHA256, "t_states": &d.TStates, "bytes": &d.Bytes} {
			if err := json.Unmarshal(fields[name], out); err != nil {
				t.Fatal(err)
			}
		}
		hash := func(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
		if d.Goal != goal || d.Selected != e.Selected || d.SearchSHA256 != hash(payload) || d.SourceName != source || d.SourceSHA256 != hash(read(source)) {
			t.Fatal("decision source/search binding")
		}
		for policy, name := range []string{"association-legacy-v2.json", "association-preserve-v2.json"} {
			if d.AssociationSHA256[policy] != hash(read(filepath.Join(dir, name))) {
				t.Fatal("input byte binding")
			}
		}
		target := e.Selections[int(e.Selected.Policy)].Candidates[e.Selected.Mask].Evidence.Baseline.Target
		if d.BinarySHA256 != target.BinarySHA256 || d.TStates != target.TotalTStates || d.Bytes != len(target.Binary) || !bytes.Equal(read(filepath.Join(dir, "selected.bin")), target.Binary) || string(read(filepath.Join(dir, "selected.a80"))) != target.Assembly {
			t.Fatal("selected native output binding")
		}
		return &e
	}
	first := verify(collect, "speed")
	memoryHostSearchCLI.LegacyAssociation = filepath.Join(collect, "association-legacy-v2.json")
	memoryHostSearchCLI.PreserveAssociation = filepath.Join(collect, "association-preserve-v2.json")
	memoryHostSearchCLI.Directory = filepath.Join(filepath.Dir(source), "search-replay")
	oldSize, oldOutput := optSize, outputFile
	optSize = true
	outputFile = filepath.Join(filepath.Dir(source), "external.a80")
	defer func() { optSize = oldSize; outputFile = oldOutput }()
	if err = runMemoryHostSearchCLI(source); err != nil {
		t.Fatal(err)
	}
	second := verify(memoryHostSearchCLI.Directory, "size")
	for _, name := range []string{"association-legacy-v2.json", "association-preserve-v2.json"} {
		if !bytes.Equal(read(filepath.Join(collect, name)), read(filepath.Join(memoryHostSearchCLI.Directory, name))) {
			t.Fatal("supplied presentation changed")
		}
	}
	if !bytes.Equal(read(outputFile), read(filepath.Join(memoryHostSearchCLI.Directory, "selected.a80"))) {
		t.Fatal("external output differs")
	}
	want := pipeline.MemoryHostPolicySearchChoice{}
	for _, c := range second.Candidates {
		current := second.Selections[int(c.Choice.Policy)].Candidates[c.Choice.Mask].Evidence.Baseline.Target
		best := second.Selections[int(want.Policy)].Candidates[want.Mask].Evidence.Baseline.Target
		if len(current.Binary) < len(best.Binary) {
			want = c.Choice
		}
	}
	if second.Selected != want {
		t.Fatal("actual Size objective")
	}
	for policy, s := range first.Selections {
		for mask, c := range s.Candidates {
			a, z := c.Evidence.Baseline.Target, second.Selections[policy].Candidates[mask].Evidence.Baseline.Target
			if a.BinarySHA256 != z.BinarySHA256 || a.TotalTStates != z.TotalTStates {
				t.Fatal("goal changed fixed candidate")
			}
		}
	}
}
