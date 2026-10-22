package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestHIRProfileAutoMeasuredCompilation(t *testing.T) {
	hirProfileTestState(t)
	oldDir, oldFn, oldWork := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldDir, oldFn, oldWork
	})
	dir := t.TempDir()
	source := filepath.Join(dir, "sum.nanz")
	src, err := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, src, 0644); err != nil {
		t.Fatal(err)
	}
	n := int64(5)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"array": {1, 2, 3, 4, 5}}, Calls: []hir.ProfileInvocation{{Function: "sum_array", Arguments: []hir.ProfileArgument{{Resource: "array"}, {Value: &n}}}}}
	hirProfileAutoDirectory = filepath.Join(dir, "evidence")
	hirProfileAutoFunction = "sum_array"
	hirProfileAutoWorkload = filepath.Join(dir, "workload.json")
	if err := writeHIRProfileJSON(hirProfileAutoWorkload, w); err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "selected.a80")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	var evidence pipeline.LoopCostEvidence
	if err := decodeHIRProfileJSON(filepath.Join(hirProfileAutoDirectory, "evidence.json"), 1<<20, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Options.UseGrace || evidence.Options.UseHIRGrace {
		t.Fatal("linear measurement must use its fixed compilation policy", evidence.Options)
	}
	if len(evidence.Rows) != 1022 {
		t.Fatalf("rows=%d", len(evidence.Rows))
	}
	data, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "decisions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decisions []hir.ProfileDecision
	if err := json.Unmarshal(data, &decisions); err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 {
		t.Fatal(decisions)
	}
	// Costs belong to the current assembled output, not a historical backend.
	// The measured fast-path row for this workload must equal both decision totals.
	matched := 0
	for _, row := range evidence.Rows {
		if row.Path != "fast" || row.N != int(n) || row.Base != 0x4000 {
			continue
		}
		matched++
		want := "original"
		if row.Pointer < row.Original {
			want = "pointer-walk"
		}
		t.Logf("measured original=%d T pointer=%d T selected=%s", row.Original, row.Pointer, decisions[0].Variant)
		if decisions[0].Variant != want {
			t.Fatalf("selected %s, measured winner %s", decisions[0].Variant, want)
		}
		if decisions[0].OriginalTStates != strconv.Itoa(row.Original) ||
			decisions[0].PointerTStates != strconv.Itoa(row.Pointer) ||
			row.OriginalExecutionSHA256 == "" || row.PointerExecutionSHA256 == "" {
			t.Fatalf("decision does not match measured native execution: decision=%+v row=%+v", decisions[0], row)
		}
	}
	if matched != 1 {
		t.Fatalf("workload native evidence rows=%d", matched)
	}
	// Saved profile/model must work with the same binary and compilation settings.
	hirProfile = hirProfileFlags{profile: filepath.Join(hirProfileAutoDirectory, "profile.json"), workload: hirProfileAutoWorkload, model: filepath.Join(hirProfileAutoDirectory, "model.json"), decisions: filepath.Join(dir, "reused.json")}
	hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = "", "", ""
	outputFile = filepath.Join(dir, "reused.a80")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(dir, "selected.a80"))
	b, _ := os.ReadFile(outputFile)
	if string(a) != string(b) {
		t.Fatal("saved model changes emitted assembly")
	}
	optSize = true
	outputFile = filepath.Join(dir, "invalid.a80")
	hirProfile.decisions = filepath.Join(dir, "invalid.json")
	if err := compile(source); err == nil {
		t.Fatal("accepted different compilation settings")
	}
	if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
		t.Fatal("refusal wrote output")
	}
}
