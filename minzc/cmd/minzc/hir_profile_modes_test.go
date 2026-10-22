package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func compilerModeTestState(t *testing.T) {
	hirProfileTestState(t)
	oldMode, oldDir, oldWork := hirProfileCompilerModes, hirProfileAutoDirectory, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileCompilerModes, hirProfileAutoDirectory, hirProfileAutoWorkload = oldMode, oldDir, oldWork
	})
	hirProfileCompilerModes = true
	hirProfileAutoDirectory = filepath.Join(t.TempDir(), "evidence")
	hirProfileAutoWorkload = "workload.json"
}
func TestCompilerModeCLIPathsAndFlags(t *testing.T) {
	compilerModeTestState(t)
	source := filepath.Join(t.TempDir(), "source.nanz")
	if e := os.WriteFile(source, []byte("fun test()->u8{return 1}"), 0644); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"missing-workload", "joint", "other-output", "no-hir-grace", "alias-input", "hardlink-input", "table-pin"} {
		t.Run(kind, func(t *testing.T) {
			oldPin := checkedLocalTableSHA256
			defer func() { checkedLocalTableSHA256 = oldPin }()
			oldW, oldO, oldJ, oldG := hirProfileAutoWorkload, outputFile, hirProfileJoint, useHIRGrace
			defer func() { hirProfileAutoWorkload, outputFile, hirProfileJoint, useHIRGrace = oldW, oldO, oldJ, oldG }()
			switch kind {
			case "table-pin":
				checkedLocalTableSHA256 = fmt.Sprintf("%064x", 1)
			case "missing-workload":
				hirProfileAutoWorkload = ""
			case "joint":
				hirProfileJoint = true
			case "other-output":
				outputFile = "out.com"
			case "no-hir-grace":
				useHIRGrace = false
			case "alias-input":
				outputFile = source
			case "hardlink-input":
				outputFile = filepath.Join(t.TempDir(), "out.a80")
				if e := os.Link(source, outputFile); e != nil {
					t.Fatal(e)
				}
			}
			if e := validateHIRProfileAuto(source); e == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}
func TestCompilerModeCLIActualSelectedArtifacts(t *testing.T) {
	compilerModeTestState(t)
	source := "../../../examples/nanz/pfcco_weighted_packet.nanz"
	workload := "../../../docs/grace-pfcco-prospective-heldout/artifacts/training-workload.json"
	hirProfileAutoWorkload = workload
	for _, goal := range []string{"speed", "size"} {
		for _, ext := range []string{".a80", ".bin"} {
			optSize = goal == "size"
			dir := t.TempDir()
			hirProfileAutoDirectory = filepath.Join(dir, "evidence")
			outputFile = filepath.Join(dir, "out"+ext)
			if e := compile(source); e != nil {
				t.Fatal(e)
			}
			b, e := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selection.json"))
			if e != nil {
				t.Fatal(e)
			}
			var evidence pipeline.MeasuredModeEvidence
			if e = json.Unmarshal(b, &evidence); e != nil {
				t.Fatal(e)
			}
			for i, candidate := range evidence.Candidates {
				t.Logf("%s mode %d: %d T, %d bytes", goal, i, candidate.Run.TotalTStates, len(candidate.Binary))
			}
			// origin/main 840b1c465 and Grace defaults both measure modes 0/2
			// at 2220 T / 46 B and modes 1/3 at 2255 T / 52 B. Stable
			// tie-breaking therefore selects mode 0 for both objectives.
			expected := 0
			if evidence.Selected != expected || evidence.Goal != goal {
				t.Fatal("actual goal mismatch")
			}
			selected := evidence.Candidates[evidence.Selected]
			out, e := os.ReadFile(outputFile)
			if e != nil {
				t.Fatal(e)
			}
			want := selected.Binary
			if ext == ".a80" {
				want = []byte(selected.Assembly)
			}
			if !bytes.Equal(out, want) {
				t.Fatal("output differs from measured winner")
			}
			var manifest map[string]string
			b, e = os.ReadFile(filepath.Join(hirProfileAutoDirectory, "manifest.json"))
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(b, &manifest); e != nil {
				t.Fatal(e)
			}
			// Refuse reruns into existing evidence before writing either output.
			if e := compile(source); e == nil {
				t.Fatal("existing evidence accepted")
			}
			preserved, e := os.ReadFile(outputFile)
			if e != nil || !bytes.Equal(preserved, out) {
				t.Fatal("failed rerun changed winner", e)
			}
			for name, hash := range manifest {
				b, e = os.ReadFile(filepath.Join(hirProfileAutoDirectory, name))
				if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != hash {
					t.Fatal("artifact hash", name, e)
				}
			}
		}
	}
}

func TestCompilerModeCLIImportedSource(t *testing.T) {
	compilerModeTestState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "caller.nanz")
	library := filepath.Join(dir, "arithmetic.nanz")
	if e := os.WriteFile(library, []byte("fun advance(n:u8)->u8{return n+7}"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(source, []byte("import arithmetic { advance }\nfun arithmetic(n:u8)->u8{return advance(n)}"), 0644); e != nil {
		t.Fatal(e)
	}
	n := int64(250)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}, Calls: []hir.ProfileInvocation{{Function: "arithmetic", Arguments: []hir.ProfileArgument{{Value: &n}}}}}
	hirProfileAutoWorkload = filepath.Join(dir, "workload.json")
	if e := writeHIRProfileJSON(hirProfileAutoWorkload, w); e != nil {
		t.Fatal(e)
	}
	outputFile = filepath.Join(dir, "result.bin")
	if e := compile(source); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selection.json"))
	if e != nil {
		t.Fatal(e)
	}
	var result pipeline.MeasuredModeEvidence
	if e = json.Unmarshal(b, &result); e != nil {
		t.Fatal(e)
	}
	observations := result.Candidates[result.Selected].Run.Invocations
	if len(observations) != 1 || len(observations[0].Results) != 1 || observations[0].Results[0].I != 1 {
		t.Fatal("independent modular arithmetic oracle failed", observations)
	}
	var saved map[string]string
	b, e = os.ReadFile(filepath.Join(hirProfileAutoDirectory, "source-paths.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &saved); e != nil {
		t.Fatal(e)
	}
	for unit, file := range saved {
		b, e = os.ReadFile(filepath.Join(hirProfileAutoDirectory, file))
		if e != nil || result.Compatibility.Context.Dependencies[unit] != fmt.Sprintf("%x", sha256.Sum256(b)) {
			t.Fatal("saved source identity mismatch", unit, e)
		}
	}
	if len(saved) < 2 || len(result.Compatibility.Context.Dependencies) < 2 {
		t.Fatal("imported dependency absent from measured identity")
	}
}

func TestCompilerModeCLIImportedOutputAlias(t *testing.T) {
	compilerModeTestState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "caller.nanz")
	library := filepath.Join(dir, "arithmetic.nanz")
	original := []byte("fun advance(n:u8)->u8{return n+7}")
	if e := os.WriteFile(library, original, 0644); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(source, []byte("import arithmetic { advance }\nfun arithmetic(n:u8)->u8{return advance(n)}"), 0644); e != nil {
		t.Fatal(e)
	}
	n := int64(250)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}, Calls: []hir.ProfileInvocation{{Function: "arithmetic", Arguments: []hir.ProfileArgument{{Value: &n}}}}}
	hirProfileAutoWorkload = filepath.Join(dir, "workload.json")
	if e := writeHIRProfileJSON(hirProfileAutoWorkload, w); e != nil {
		t.Fatal(e)
	}
	outputFile = filepath.Join(dir, "out.bin")
	if e := os.Link(library, outputFile); e != nil {
		t.Fatal(e)
	}
	if e := compile(source); e == nil {
		t.Fatal("imported output alias accepted")
	}
	got, e := os.ReadFile(library)
	if e != nil || !bytes.Equal(got, original) {
		t.Fatal("import overwritten", e)
	}
	if _, e := os.Stat(hirProfileAutoDirectory); !os.IsNotExist(e) {
		t.Fatal("failed admission emitted artifacts", e)
	}
}
