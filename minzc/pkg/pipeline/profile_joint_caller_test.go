package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

func TestJointOrdinaryCallerAllCombinations(t *testing.T) {
	factory := func() (*hir.Module, error) { return nanz.Parse(profileCallerSource, "caller.nanz") }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "run"}, {Function: "run"}}}
	oracle, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	observations, err := json.Marshal(oracle.Profile)
	if err != nil {
		t.Fatal(err)
	}
	result, err := MeasureProfileVariantSets(factory, observations, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Evidence.Candidates) != 4 {
		t.Fatal(result.Evidence)
	}
	for i, c := range result.Evidence.Candidates {
		t.Logf("candidate%d: bytes=%d cycles=%d decisions=%v", i, c.ModuleBytes, c.TotalTStates, c.Decisions)
		if c.Run.Globals["output"][0] != 16 || len(c.Run.Invocations) != 2 || c.Run.Invocations[0].Results[0].I != 16 {
			t.Fatal(c.Run)
		}
	}
	best := result.Evidence.Candidates[result.Evidence.Selected]
	for _, c := range result.Evidence.Candidates {
		if c.Run.TotalTStates < best.Run.TotalTStates {
			t.Fatal("did not select measured minimum")
		}
	}
	if directory := os.Getenv("MINZ_PGO_ARTIFACT_DIR"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		for name, value := range map[string]any{"joint-evidence.json": result.Evidence, "workload.json": w, "profile.json": oracle.Profile, "decisions.json": result.Decisions} {
			data, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, name), append(data, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
		}
		for name, data := range map[string][]byte{"caller.nanz": []byte(profileCallerSource), "selected.bin": result.Binary, "selected.a80": []byte(result.Steps.Assembly), "input.hir": []byte(result.Steps.HIR), "raw.mir": []byte(result.Steps.MIR2Raw), "optimized.mir": []byte(result.Steps.MIR2Opt)} {
			if err := os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
