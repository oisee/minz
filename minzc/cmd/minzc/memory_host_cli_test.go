package main

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withMemoryHostCLITest(t *testing.T) (string, string) {
	t.Helper()
	old := memoryHostCLI
	oldBackend, oldTarget, oldFormat, oldOutput := backend, target, outputFormat, outputFile
	memoryHostCLI = memoryHostCLIFlags{}
	backend, target, outputFormat, outputFile = "z80", "cpm", "", ""
	t.Cleanup(func() {
		memoryHostCLI = old
		backend, target, outputFormat, outputFile = oldBackend, oldTarget, oldFormat, oldOutput
	})
	dir := t.TempDir()
	source := filepath.Join(dir, "input.nanz")
	workload := filepath.Join(dir, "input-workload.json")
	os.WriteFile(source, []byte("fun main()->u8{return 0}"), 0644)
	os.WriteFile(workload, []byte("{}"), 0644)
	memoryHostCLI = memoryHostCLIFlags{Directory: filepath.Join(dir, "artifacts"), Workload: workload}
	return source, workload
}
func TestMemoryHostCLIValidation(t *testing.T) {
	source, _ := withMemoryHostCLITest(t)
	if e := validateMemoryHostCLI(source); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"target", "format", "profilemode", "no-hirgrace", "partialflags"} {
		t.Run(name, func(t *testing.T) {
			p := memoryHostCLI
			oldTarget, oldFormat, oldJoint, oldGrace := target, outputFormat, hirProfileJoint, useHIRGrace
			defer func() {
				memoryHostCLI = p
				target, outputFormat, hirProfileJoint, useHIRGrace = oldTarget, oldFormat, oldJoint, oldGrace
			}()
			switch name {
			case "target":
				target = "zxspectrum"
			case "format":
				outputFormat = "sna"
			case "profilemode":
				hirProfileJoint = true
			case "no-hirgrace":
				useHIRGrace = false
			case "partialflags":
				memoryHostCLI.Directory = ""
			}
			if e := validateMemoryHostCLI(source); e == nil {
				t.Fatal("unsupportedmodeaccepted")
			}
		})
	}
	if e := validateMemoryHostCLI(strings.TrimSuffix(source, ".nanz") + ".a80"); e == nil {
		t.Fatal("assemblyinputaccepted")
	}
}
func TestMemoryHostCLIPathAliases(t *testing.T) {
	source, workload := withMemoryHostCLITest(t)
	os.MkdirAll(memoryHostCLI.Directory, 0755)
	for _, kind := range []string{"hardlink", "symlink", "output"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(memoryHostCLI.Directory, "selected.bin")
			os.Remove(path)
			switch kind {
			case "hardlink":
				if e := os.Link(source, path); e != nil {
					t.Fatal(e)
				}
			case "symlink":
				if e := os.Symlink(workload, path); e != nil {
					t.Fatal(e)
				}
			case "output":
				outputFile = source
				defer func() { outputFile = "" }()
			}
			if e := validateMemoryHostCLI(source); e == nil {
				t.Fatal("aliasaccepted")
			}
			os.Remove(path)
		})
	}
	nested := filepath.Join(memoryHostCLI.Directory, "both")
	if e := os.Symlink(filepath.Dir(source), nested); e != nil {
		t.Fatal(e)
	}
	memoryHostCLI.Profile = filepath.Join(filepath.Dir(source), "target.bin")
	os.WriteFile(memoryHostCLI.Profile, []byte("profile"), 0644)
	if e := validateMemoryHostCLI(source); e == nil {
		t.Fatal("resolvedparentaliasaccepted")
	}
}
func TestMemoryHostCLIWorkloadKeys(t *testing.T) {
	_, path := withMemoryHostCLITest(t)
	for _, text := range []string{`null`, `[]`, `{"Schema":"x"}`, `{"schema":"x","schema":"y"}`, `{"schema":"x","calls":[{"Function":"main"}]}`, `{"schema":"x","regions":{"r":{"Address":1}}}`} {
		os.WriteFile(path, []byte(text), 0644)
		var w hir.MemoryHostWorkload
		if e := decodeMemoryHostCLIWorkload(path, &w); e == nil {
			t.Fatal("ambiguousorcasealiasaccepted", text)
		}
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", Calls: []hir.MemoryHostInvocation{{Function: "main", Arguments: []int64{}}}}
	b, _ := json.Marshal(w)
	os.WriteFile(path, b, 0644)
	var got hir.MemoryHostWorkload
	if e := decodeMemoryHostCLIWorkload(path, &got); e != nil {
		t.Fatal(e)
	}
}
