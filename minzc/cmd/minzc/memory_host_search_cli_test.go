package main

import (
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
