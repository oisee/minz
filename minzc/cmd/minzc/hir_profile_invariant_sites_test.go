package main

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func TestHIRProfileInvariantSitesCompilation(t *testing.T) {
	testHIRProfileInvariantCompilation(t, false)
}

func TestHIRProfileInvariantSiteRulesCompilation(t *testing.T) {
	testHIRProfileInvariantCompilation(t, true)
}

func testHIRProfileInvariantCompilation(t *testing.T, siteRules bool) {
	hirProfileTestState(t)
	oldSiteRules := hirProfileSiteRules
	oldSite, oldJoint, oldRules := hirProfileInvariantSites, hirProfileJoint, hirProfileIndependentRules
	oldDir, oldFn, oldWork := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileSiteRules = oldSiteRules
		hirProfileInvariantSites, hirProfileJoint, hirProfileIndependentRules = oldSite, oldJoint, oldRules
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldDir, oldFn, oldWork
	})
	dir := t.TempDir()
	source := filepath.Join(dir, "scalar.nanz")
	if err := os.WriteFile(source, []byte(`fun scalar(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 var i:u8=0
 while i<n {s=s+(a+b)
 i=i+1}
 return s}`), 0644); err != nil {
		t.Fatal(err)
	}
	hirProfileSiteRules = siteRules
	hirProfileInvariantSites = true
	hirProfileJoint = true
	hirProfileIndependentRules = false
	hirProfileAutoDirectory = filepath.Join(dir, "evidence")
	hirProfileAutoFunction = ""
	hirProfileAutoWorkload = filepath.Join(dir, "workload.json")
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 200000, MaxMemory: 65536}
	for _, n := range []int64{0, 1, 5, 255} {
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "scalar", Arguments: []hir.ProfileArgument{literal(65535), literal(2), literal(n)}})
	}
	if err := writeHIRProfileJSON(hirProfileAutoWorkload, w); err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "compiled.a80")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	var e pipeline.InvariantSiteVariantSetEvidence
	b, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "joint-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	schema, count := "minz.invariant-site-variant-sets/v1", 2
	if siteRules {
		schema, count = "minz.invariant-site-rule-variant-sets/v2", 4
	}
	if e.Schema != schema || len(e.Candidates) != count || e.Selected != 1 {
		t.Fatal("incorrect site selection", e.Selected)
	}
	retained, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"))
	if err != nil || string(retained) != string(e.Candidates[e.Selected].Binary) {
		t.Fatal("CLI winner not measured binary", err)
	}
	for _, c := range e.Candidates {
		for i, n := range []int64{0, 1, 5, 255} {
			if c.Run.Invocations[i].Results[0].I != n {
				t.Fatal("CLI independent oracle")
			}
		}
	}
}

func TestHIRProfileInvariantSitesValidation(t *testing.T) {
	oldSite, oldJoint, oldRules := hirProfileInvariantSites, hirProfileJoint, hirProfileIndependentRules
	t.Cleanup(func() {
		hirProfileInvariantSites, hirProfileJoint, hirProfileIndependentRules = oldSite, oldJoint, oldRules
	})
	hirProfileInvariantSites = true
	hirProfileJoint = false
	if validateHIRProfileAuto("a.nanz") == nil {
		t.Fatal("site mode without joint accepted")
	}
	hirProfileJoint = true
	hirProfileIndependentRules = true
	if validateHIRProfileAuto("a.nanz") == nil {
		t.Fatal("conflicting search modes accepted")
	}
}

func TestHIRProfileInvariantSiteRulesValidation(t *testing.T) {
	oldSite, oldRules := hirProfileInvariantSites, hirProfileSiteRules
	t.Cleanup(func() { hirProfileInvariantSites, hirProfileSiteRules = oldSite, oldRules })
	hirProfileInvariantSites, hirProfileSiteRules = false, true
	if validateHIRProfileAuto("a.nanz") == nil {
		t.Fatal("site rules accepted without site selection")
	}
}
