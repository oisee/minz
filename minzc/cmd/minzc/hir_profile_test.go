package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func hirProfileTestState(t *testing.T) {
	t.Helper()
	outputTestState(t)
	old, oldTarget, oldSize := hirProfile, target, optSize
	t.Cleanup(func() { hirProfile, target, optSize = old, oldTarget, oldSize })
	hirProfile, target, optSize, assertMode = hirProfileFlags{}, "cpm", false, "none"
}

func TestHIRProfileStrictJSON(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		model       bool
		limit       int64
	}{
		{"unknown", `{"surprise":1}`, false, 1024},
		{"nested_unknown", `{"calls":[{"function":"f","arguments":[{"value":1,"surprise":true}]}]}`, false, 1024},
		{"trailing", `{} {}`, false, 1024},
		{"trailing_garbage", `{}x`, false, 1024},
		{"duplicate", `{"calls":[],"calls":[]}`, false, 1024},
		{"case_duplicate", `{"calls":[],"Calls":[]}`, false, 1024},
		{"nested_duplicate", `{"resources":{"a":"","a":""}}`, false, 1024},
		{"null", `null`, false, 1024},
		{"oversized", `{"schema":"abcdef"}`, false, 8},
		{"model_unknown", `{"estimated":true}`, true, 1024},
		{"model_trailing", `{} []`, true, 1024},
		{"model_overflow", `{"original_bytes":18446744073709551616}`, true, 1024},
		{"model_null", `null`, true, 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(path, []byte(tc.input), 0644); err != nil {
				t.Fatal(err)
			}
			var workload hir.ProfileWorkload
			var model hir.LoopCostModel
			var out any = &workload
			if tc.model {
				out = &model
			}
			if err := decodeHIRProfileJSON(path, tc.limit, out); err == nil {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
}

func TestHIRProfileFlagValidation(t *testing.T) {
	hirProfileTestState(t)
	valid := hirProfileFlags{collect: "profile.json", workload: "workload.json"}
	for _, tc := range []struct {
		name  string
		setup func()
		want  string
	}{
		{"missing_mode", func() { hirProfile = hirProfileFlags{workload: "w"} }, "choose exactly one"},
		{"both_modes", func() { hirProfile.profile = "p" }, "choose exactly one"},
		{"missing_workload", func() { hirProfile.workload = "" }, "workload is required"},
		{"collect_model", func() { hirProfile.model = "m" }, "collection cannot"},
		{"collect_output", func() { outputFile = "out.a80" }, "-o is incompatible"},
		{"apply_missing_model", func() { hirProfile = hirProfileFlags{profile: "p", workload: "w", decisions: "d"} }, "application requires"},
		{"apply_run", func() {
			hirProfile = hirProfileFlags{profile: "p", workload: "w", model: "m", decisions: "d", run: "r"}
		}, "requires collection"},
		{"legacy", func() { pgoProfile = "legacy.tas" }, "legacy --pgo"},
		{"backend", func() { backend = "ez80" }, "only the z80"},
		{"emit", func() { emitFormat = "nanz" }, "--emit"},
		{"disabled", func() { disableOptimize = true }, "optimization-disable"},
		{"collision", func() { hirProfile.collect = "workload.json" }, "paths must be distinct"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hirProfile = valid
			outputFile = ""
			backend = "z80"
			emitFormat = ""
			oldPGO, oldDisable := pgoProfile, disableOptimize
			defer func() { pgoProfile, disableOptimize = oldPGO, oldDisable }()
			tc.setup()
			if err := validateHIRProfileFlags("source.nanz"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	hirProfile = valid
	outputFile = ""
	backend = "z80"
	emitFormat = ""
	for _, source := range []string{"source.old", "source.asm", "source.mir"} {
		if err := compile(source); err == nil || !strings.Contains(err.Error(), "old frontend") {
			t.Fatalf("old frontend not refused: %v", err)
		}
	}
	hirProfile = hirProfileFlags{}
	pgoProfile = "legacy.tas"
	defer func() { pgoProfile = "" }()
	if err := validateHIRProfileFlags("source.old"); err != nil {
		t.Fatalf("changed legacy behavior: %v", err)
	}
}

func TestHIRProfileCollectionAndApplication(t *testing.T) {
	hirProfileTestState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "sum.nanz")
	src, err := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, src, 0644); err != nil {
		t.Fatal(err)
	}
	workloadPath, profilePath, runPath := filepath.Join(dir, "workload.json"), filepath.Join(dir, "profile.json"), filepath.Join(dir, "run.json")
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"array": {1, 2, 3, 4, 5}}, Calls: []hir.ProfileInvocation{{Function: "sum_array", Arguments: []hir.ProfileArgument{{Resource: "array"}, literal(5)}}}}
	if err := writeHIRProfileJSON(workloadPath, workload); err != nil {
		t.Fatal(err)
	}
	hirProfile = hirProfileFlags{collect: profilePath, workload: workloadPath, run: runPath}
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sum.a80")); !os.IsNotExist(err) {
		t.Fatal("collection compiled assembly")
	}
	var run hir.ProfileRun
	if err := decodeHIRProfileJSON(runPath, hirObservationJSONLimit, &run); err != nil {
		t.Fatal(err)
	}
	if len(run.Results) != 1 || len(run.Results[0]) != 1 || run.Results[0][0].I != 15 {
		t.Fatalf("wrong run results: %v", run.Results)
	}
	if string(run.Resources["array"]) != string(workload.Resources["array"]) {
		t.Fatal("resource changed")
	}
	context := run.Profile.Compatibility.Context
	// Independent measured fixture from docs/grace-pgo-decision.md. These costs
	// belong only to this test; production CLI supplies no default costs.
	model := hir.LoopCostModel{CompilerID: context.CompilerID, SemanticsID: context.SemanticsID, Function: "sum_array", HIRSHA256: run.Profile.Compatibility.HIRSHA256, TargetID: context.TargetID, EvidenceID: "docs/grace-pgo-decision.md", OriginalBytes: 128, PointerBytes: 177, OriginalEntry: 69, OriginalIteration: 141, EqualEntryExtra: 7, FastEntry: 189, FastIteration: 89, FallbackEntry: 208, FallbackIteration: 141, FallbackMeasured: true}
	model.CompilationID, err = pipeline.ProfileCompilationID(pipeline.Options{ContractOpt: true, Backend: "z80", AssertMode: "none"}, "default")
	if err != nil {
		t.Fatal(err)
	}
	modelPath, decisionPath := filepath.Join(dir, "model.json"), filepath.Join(dir, "decisions.json")
	if err := writeHIRProfileJSON(modelPath, model); err != nil {
		t.Fatal(err)
	}
	hirProfile = hirProfileFlags{profile: profilePath, workload: workloadPath, model: modelPath, decisions: decisionPath}
	outputFile = filepath.Join(dir, "selected.a80")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	var decisions []hir.ProfileDecision
	data, err := os.ReadFile(decisionPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &decisions); err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 || decisions[0].Variant != "pointer-walk" || decisions[0].Compatibility.Context.CompilerID != context.CompilerID {
		t.Fatalf("decisions: %+v", decisions)
	}
	if data, err := os.ReadFile(outputFile); err != nil || len(data) == 0 {
		t.Fatalf("no assembly: %v", err)
	}
	outputFile = filepath.Join(dir, "rejected.a80")
	hirProfile.decisions = filepath.Join(dir, "rejected.json")
	reject := func() {
		t.Helper()
		if err := compile(source); err == nil {
			t.Fatal("incompatible profile application succeeded")
		}
		for _, path := range []string{outputFile, hirProfile.decisions} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("failed application wrote %s", path)
			}
		}
	}
	workload.MaxSteps++
	if err := writeHIRProfileJSON(workloadPath, workload); err != nil {
		t.Fatal(err)
	}
	reject() // current workload context must be recomputed, not trusted from JSON
	workload.MaxSteps--
	if err := writeHIRProfileJSON(workloadPath, workload); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, append(src, []byte("\n// changed source manifest\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	reject()
	if err := os.WriteFile(source, src, 0644); err != nil {
		t.Fatal(err)
	}
	optSize = true
	reject() // compilation fingerprint includes the size policy
	optSize = false
	// A different executable identity is refused, never replaced from the file.
	model.CompilerID = "stale executable"
	if err := writeHIRProfileJSON(modelPath, model); err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "rejected.a80")
	hirProfile.decisions = filepath.Join(dir, "rejected.json")
	if err := compile(source); err == nil || !strings.Contains(err.Error(), "incompatible cost model") {
		t.Fatalf("stale model accepted: %v", err)
	}
	for _, path := range []string{outputFile, hirProfile.decisions} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("failed application wrote %s", path)
		}
	}
}

func TestHIRProfileIncompleteManifest(t *testing.T) {
	hirProfileTestState(t)
	dir := t.TempDir()
	n := int64(0)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-v1", MaxSteps: 100, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "f", Arguments: []hir.ProfileArgument{{Value: &n}}}}}
	hirProfile = hirProfileFlags{collect: filepath.Join(dir, "p.json"), workload: filepath.Join(dir, "w.json")}
	if err := writeHIRProfileJSON(hirProfile.workload, w); err != nil {
		t.Fatal(err)
	}
	if err := collectHIRProfile(&hir.Module{}); err == nil || !strings.Contains(err.Error(), "complete source manifest unavailable") {
		t.Fatalf("incomplete frontend accepted: %v", err)
	}
}

func TestHIRProfileFailedRun(t *testing.T) {
	hirProfileTestState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "loop.nanz")
	if err := os.WriteFile(source, []byte("fun loop(n:u8)->u8 {\nvar i:u8=0\nwhile i<n { i=i+1 }\nreturn i\n}"), 0644); err != nil {
		t.Fatal(err)
	}
	n := int64(100)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-v1", MaxSteps: 1, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "loop", Arguments: []hir.ProfileArgument{{Value: &n}}}}}
	hirProfile = hirProfileFlags{collect: filepath.Join(dir, "p.json"), workload: filepath.Join(dir, "w.json"), run: filepath.Join(dir, "r.json")}
	if err := writeHIRProfileJSON(hirProfile.workload, w); err != nil {
		t.Fatal(err)
	}
	if err := compile(source); err == nil {
		t.Fatal("failed run succeeded")
	} else {
		t.Log(err)
	}
	var profile hir.LoopProfile
	if err := decodeHIRProfileJSON(hirProfile.collect, hirObservationJSONLimit, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.FailedCalls != 1 {
		t.Fatalf("failed snapshot not marked: %+v", profile)
	}
}
