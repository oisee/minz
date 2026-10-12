package pipeline

import (
	"encoding/json"
	"testing"
)

func TestInputHIRLoopSitesAreOptional(t *testing.T) {
	for _, collect := range []bool{false, true} {
		for _, grace := range []bool{false, true} {
			h := graceMeasureParse(t, graceSumSource)
			steps, err := CompileHIRSteps(h, Options{CollectHIRSites: collect, UseHIRGrace: grace, SkipZ80Emission: true, AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			if !collect {
				if steps.HIRLoopSites != nil {
					t.Fatal("collection enabled by default")
				}
				continue
			}
			if steps.HIRLoopSites == nil || len(steps.HIRLoopSites.Sites) != 1 || steps.HIRLoopSites.Stage != "input-hir" || steps.HIRLoopSites.Sites[0].Kind != "while" {
				t.Fatalf("sites: %+v", steps.HIRLoopSites)
			}
			data, err := json.MarshalIndent(steps.HIRLoopSites, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("grace=%v input-sites=%s", grace, data)
		}
	}
}
