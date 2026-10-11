package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func withPolicyCLI(t *testing.T) (string, string) {
	t.Helper()
	old := memoryHostPolicyCLI
	oldSize := optSize
	changed := map[string]bool{}
	for _, name := range []string{"memory-host-arena-append-policy", "memory-host-compare-policy", "memory-host-association", "memory-host-arena-append", "memory-host-profile"} {
		f := rootCmd.Flags().Lookup(name)
		changed[name] = f.Changed
		f.Changed = false
	}
	memoryHostPolicyCLI = memoryHostPolicyCLIFlags{}
	source, workload := withMemoryHostCLITest(t)
	memoryHostCLI.Directory = ""
	memoryHostPolicyCLI = memoryHostPolicyCLIFlags{Directory: filepath.Join(filepath.Dir(source), "policy-artifacts"), Policy: "preserve-speed"}
	optSize = false
	t.Cleanup(func() {
		memoryHostPolicyCLI = old
		optSize = oldSize
		for name, v := range changed {
			rootCmd.Flags().Lookup(name).Changed = v
		}
	})
	return source, workload
}
func TestPolicyCLIAdmission(t *testing.T) {
	source, _ := withPolicyCLI(t)
	if e := validateMemoryHostPolicyCLI(source); e != nil {
		t.Fatal(e)
	}
	for _, text := range []string{"", "1", "Preserve-Speed", "other"} {
		old := memoryHostPolicyCLI.Policy
		memoryHostPolicyCLI.Policy = text
		if e := validateMemoryHostPolicyCLI(source); e == nil {
			t.Fatal("bad policy", text)
		}
		memoryHostPolicyCLI.Policy = old
	}
	for _, name := range []string{"memory-host-arena-append", "memory-host-profile"} {
		f := rootCmd.Flags().Lookup(name)
		f.Changed = true
		if e := validateMemoryHostPolicyCLI(source); e == nil {
			t.Fatal("empty legacy flag", name)
		}
		f.Changed = false
	}
	old := memoryHostPolicyCLI
	memoryHostPolicyCLI = memoryHostPolicyCLIFlags{}
	for _, name := range []string{"memory-host-arena-append-policy", "memory-host-compare-policy", "memory-host-association"} {
		f := rootCmd.Flags().Lookup(name)
		f.Changed = true
		if !memoryHostPolicyCLIRequested() {
			t.Fatal("empty flag lost")
		}
		if e := compile(source); e == nil {
			t.Fatal("empty flag entered ordinary compile")
		}
		f.Changed = false
	}
	memoryHostPolicyCLI = old
}
func TestPolicyCLIPathAliases(t *testing.T) {
	source, workload := withPolicyCLI(t)
	dir := memoryHostPolicyCLI.Directory
	os.MkdirAll(dir, 0755)
	for _, kind := range []string{"source", "workload", "association", "output"} {
		t.Run(kind, func(t *testing.T) {
			p := filepath.Join(dir, "selected.bin")
			oldAssoc, oldOut := memoryHostPolicyCLI.Association, outputFile
			defer func() { os.Remove(p); memoryHostPolicyCLI.Association = oldAssoc; outputFile = oldOut }()
			switch kind {
			case "source":
				if e := os.Link(source, p); e != nil {
					t.Fatal(e)
				}
			case "workload":
				if e := os.Symlink(workload, p); e != nil {
					t.Fatal(e)
				}
			case "association":
				memoryHostPolicyCLI.Association = filepath.Join(filepath.Dir(source), "a.json")
				os.WriteFile(memoryHostPolicyCLI.Association, []byte("{}"), 0644)
				if e := os.Link(memoryHostPolicyCLI.Association, p); e != nil {
					t.Fatal(e)
				}
			case "output":
				outputFile = source
			}
			if e := validateMemoryHostPolicyCLI(source); e == nil {
				t.Fatal("alias accepted", kind)
			}
		})
	}
}
func policySmallFixture(t *testing.T) (string, string) {
	source, path := withPolicyCLI(t)
	b, e := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(source, b, 0644)
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 2000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"source": {Address: 0xc000, Bytes: make([]byte, 4096)}, "strings": {Address: 0x9000, Bytes: make([]byte, 4096), Writable: true}, "nodes": {Address: 0xa000, Bytes: make([]byte, 4096), Writable: true}}, Globals: map[string][]byte{"src_buf": {0, 192}, "src_len": {0, 0}, "str_arena": {0, 144}, "str_bump": {0, 144}, "node_arena": {0, 160}, "node_bump": {0, 160}}, Calls: []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x9000, 0, 0}}, {Function: "intern_region", Arguments: []int64{0, 0}}}}
	encoded, e := json.Marshal(w)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(path, encoded, 0644)
	return source, path
}
func TestPolicyCLICollectReplay(t *testing.T) {
	source, _ := policySmallFixture(t)
	if e := runMemoryHostPolicyCLI(source); e != nil {
		t.Fatal(e)
	}
	collected := memoryHostPolicyCLI.Directory
	original, e := os.ReadFile(filepath.Join(collected, "association-v2.json"))
	if e != nil {
		t.Fatal(e)
	}
	check := func(dir string) {
		t.Helper()
		sb, e := os.ReadFile(filepath.Join(dir, "selection-v2.json"))
		if e != nil {
			t.Fatal(e)
		}
		ab, _ := os.ReadFile(filepath.Join(dir, "association-v2.json"))
		var s pipeline.MemoryHostArenaAppendSelectionV2
		if e = json.Unmarshal(sb, &s); e != nil {
			t.Fatal(e)
		}
		var d struct {
			Schema            string                             `json:"schema"`
			CompileOptions    pipeline.MemoryHostBaselineOptions `json:"compile_options"`
			SelectionSHA256   string                             `json:"selection_sha256"`
			AssociationSHA256 string                             `json:"association_sha256"`
			BinarySHA256      string                             `json:"binary_sha256"`
		}
		db, _ := os.ReadFile(filepath.Join(dir, "decisions-v2.json"))
		if e = json.Unmarshal(db, &d); e != nil {
			t.Fatal(e)
		}
		target := s.Candidates[s.Selected].Evidence.Baseline.Target
		asm, _ := os.ReadFile(filepath.Join(dir, "selected.a80"))
		bin, _ := os.ReadFile(filepath.Join(dir, "selected.bin"))
		if d.Schema != "minz.memory-host-arena-append-cli-decision/v2" || d.CompileOptions != s.CompileOptions || d.SelectionSHA256 != fmt.Sprintf("%x", sha256.Sum256(sb)) || d.AssociationSHA256 != fmt.Sprintf("%x", sha256.Sum256(ab)) || d.BinarySHA256 != target.BinarySHA256 || string(asm) != target.Assembly || !reflect.DeepEqual(bin, target.Binary) {
			t.Fatal("output binding")
		}
	}
	check(collected)
	memoryHostPolicyCLI.Association = filepath.Join(collected, "association-v2.json")
	memoryHostPolicyCLI.Directory = filepath.Join(filepath.Dir(source), "replay")
	outputFile = filepath.Join(filepath.Dir(source), "external.a80")
	if e := runMemoryHostPolicyCLI(source); e != nil {
		t.Fatal(e)
	}
	check(memoryHostPolicyCLI.Directory)
	after, _ := os.ReadFile(filepath.Join(memoryHostPolicyCLI.Directory, "association-v2.json"))
	if !reflect.DeepEqual(original, after) {
		t.Fatal("submitted bytes changed")
	}
	external, _ := os.ReadFile(outputFile)
	selected, _ := os.ReadFile(filepath.Join(memoryHostPolicyCLI.Directory, "selected.a80"))
	if !reflect.DeepEqual(external, selected) {
		t.Fatal("external output")
	}
	memoryHostPolicyCLI.Policy = "legacy"
	memoryHostPolicyCLI.Directory = filepath.Join(filepath.Dir(source), "wrong-policy")
	if e := runMemoryHostPolicyCLI(source); e == nil {
		t.Fatal("wrong policy accepted")
	}
	if _, e := os.Stat(memoryHostPolicyCLI.Directory); !os.IsNotExist(e) {
		t.Fatal("refusal wrote files")
	}
}

func TestPolicyCLIMalformedAssociationAndImports(t *testing.T) {
	source, _ := policySmallFixture(t)
	input := filepath.Join(filepath.Dir(source), "supplied.json")
	memoryHostPolicyCLI.Association = input
	for _, text := range []string{"null", "{}", `{"Schema":"v2"}`, `{"schema":"x","schema":"y"}`, `{"schema":"minz.memory-host-loop-baseline-association/v1"}`} {
		if e := os.WriteFile(input, []byte(text), 0644); e != nil {
			t.Fatal(e)
		}
		if e := runMemoryHostPolicyCLI(source); e == nil {
			t.Fatal("malformed input accepted")
		}
		if _, e := os.Stat(memoryHostPolicyCLI.Directory); !os.IsNotExist(e) {
			t.Fatal("refusal wrote outputs")
		}
	}
	// Parse may refuse an unresolved import before the one-unit check; both are
	// admission refusals and must preserve the source/dependency namespace.
	if e := os.WriteFile(source, []byte("import dep\nfun main()->u8{return 0}"), 0644); e != nil {
		t.Fatal(e)
	}
	memoryHostPolicyCLI.Association = ""
	if e := runMemoryHostPolicyCLI(source); e == nil {
		t.Fatal("imported source admitted")
	}
	if _, e := os.Stat(memoryHostPolicyCLI.Directory); !os.IsNotExist(e) {
		t.Fatal("import refusal wrote outputs")
	}
}

func TestPolicyCLILegacyRouteUnchanged(t *testing.T) {
	source, _ := withPolicyCLI(t)
	memoryHostPolicyCLI = memoryHostPolicyCLIFlags{}
	memoryHostCLI.Directory = filepath.Join(filepath.Dir(source), "old-artifacts")
	if memoryHostPolicyCLIRequested() {
		t.Fatal("no new flags changed ordinary/legacy route")
	}
	if e := validateMemoryHostCLI(source); e != nil {
		t.Fatal("legacy admission changed", e)
	}
	old := memoryHostCLI
	memoryHostCLI = memoryHostCLIFlags{}
	if e := validateMemoryHostCLI("ordinary.nanz"); e != nil {
		t.Fatal("ordinary empty mode changed", e)
	}
	memoryHostCLI = old
}
