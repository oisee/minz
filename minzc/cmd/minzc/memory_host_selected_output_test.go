package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/pipeline"
)

func TestMemoryHostSelectedOutputExtensions(t *testing.T) {
	source, _ := withMemoryHostCLITest(t)
	for _, ext := range []string{".com", ".json", "", ".asm"} {
		outputFile = filepath.Join(t.TempDir(), "out"+ext)
		if err := validateMemoryHostCLI(source); err == nil {
			t.Fatal("invalid extension accepted", ext)
		}
		// The source/workload are intentionally not a valid measured fixture:
		// rejection must occur before decoding or measurement.
		if err := runMemoryHostCLI(source); err == nil || !strings.Contains(err.Error(), "-o must end") {
			t.Fatal("invalid output did not fail before measurement", ext, err)
		}
		if _, err := os.Stat(memoryHostCLI.Directory); !os.IsNotExist(err) {
			t.Fatal("rejection emitted artifacts", err)
		}
	}
	for _, ext := range []string{".bin", ".a80", ".BIN", ".A80"} {
		outputFile = filepath.Join(t.TempDir(), "out"+ext)
		if err := validateMemoryHostCLI(source); err != nil {
			t.Fatal(ext, err)
		}
	}
}

func TestMemoryHostSelectedOutputBytes(t *testing.T) {
	binary := []byte{0, 255, 201}
	assembly := "entry:\n ret\n"
	for _, ext := range []string{".bin", ".a80", ".BIN", ".A80"} {
		path := filepath.Join(t.TempDir(), "out"+ext)
		if err := writeMemoryHostSelectedOutput(path, assembly, binary); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := []byte(assembly)
		if ext == ".bin" || ext == ".BIN" {
			want = binary
		}
		if !bytes.Equal(got, want) {
			t.Fatal("selected bytes differ", ext)
		}
	}
}

func TestMemoryHostLegacySelectedOutputCollectReplay(t *testing.T) {
	source, _ := policySmallFixture(t)
	memoryHostCLI.Directory = filepath.Join(filepath.Dir(source), "legacy-collect")
	memoryHostPolicyCLI = memoryHostPolicyCLIFlags{}
	outputFile = filepath.Join(filepath.Dir(source), "collect.bin")
	if err := runMemoryHostCLI(source); err != nil {
		t.Fatal(err)
	}
	check := func(dir, path string, bin bool) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, "selection.json"))
		if err != nil {
			t.Fatal(err)
		}
		var s pipeline.MemoryHostArenaAppendSelection
		if err = json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		winner := s.Candidates[s.Selected].Evidence.Target
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := []byte(winner.Assembly)
		if bin {
			want = winner.Binary
		}
		if !bytes.Equal(got, want) {
			t.Fatal("output differs from actual measured winner")
		}
		// Independent zero-length oracle: no comparison/copy body executes.
		if len(winner.Invocations) != 2 || winner.Invocations[0].Results[0].I != 1 || winner.Invocations[1].Results[0].I != 0x9000 {
			t.Fatal("independent zero-length oracle")
		}
	}
	check(memoryHostCLI.Directory, outputFile, true)
	memoryHostCLI.Profile = filepath.Join(memoryHostCLI.Directory, "source-profile.json")
	memoryHostCLI.Directory = filepath.Join(filepath.Dir(source), "legacy-replay")
	outputFile = filepath.Join(filepath.Dir(source), "replay.a80")
	if err := runMemoryHostCLI(source); err != nil {
		t.Fatal(err)
	}
	check(memoryHostCLI.Directory, outputFile, false)
}
