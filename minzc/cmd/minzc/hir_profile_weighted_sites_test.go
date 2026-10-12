package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestHIRProfileWeightedSitesCompilation(t *testing.T) {
	hirProfileTestState(t)
	oldWeighted, oldJointSites := hirProfileWeightedSites, hirProfileJointSites
	oldSite, oldJoint, oldRules, oldSiteRules := hirProfileInvariantSites, hirProfileJoint, hirProfileIndependentRules, hirProfileSiteRules
	oldDir, oldFn, oldWork := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileWeightedSites, hirProfileJointSites = oldWeighted, oldJointSites
		hirProfileInvariantSites, hirProfileJoint, hirProfileIndependentRules, hirProfileSiteRules = oldSite, oldJoint, oldRules, oldSiteRules
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldDir, oldFn, oldWork
	})
	dir := t.TempDir()
	source := filepath.Join(dir, "weighted.nanz")
	b, err := os.ReadFile("../../../examples/nanz/grace_weighted_checksum.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(source, b, 0644); err != nil {
		t.Fatal(err)
	}
	var w hir.ProfileWorkload
	b, err = os.ReadFile("../../../docs/grace-pgo-scalar-artifacts/workload.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	hirProfileWeightedSites, hirProfileJointSites, hirProfileInvariantSites, hirProfileJoint = true, true, true, true
	hirProfileIndependentRules, hirProfileSiteRules = false, false
	hirProfileAutoFunction = ""
	hirProfileAutoWorkload = filepath.Join(dir, "workload.json")
	if err = writeHIRProfileJSON(hirProfileAutoWorkload, w); err != nil {
		t.Fatal(err)
	}
	hirProfileAutoDirectory = filepath.Join(dir, "evidence")
	outputFile = filepath.Join(dir, "selected.a80")
	if err = compile(source); err != nil {
		t.Fatal(err)
	}
	var e pipeline.InvariantSiteVariantSetEvidence
	b, err = os.ReadFile(filepath.Join(hirProfileAutoDirectory, "joint-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	if e.Schema != "minz.weighted-joint-source-site-variant-sets/v4" || len(e.Candidates) != 8 || e.Selected != 3 || len(e.EligibleWeightedOverlapSites) != 1 {
		t.Fatal("weighted search", e.Schema, len(e.Candidates), e.Selected)
	}
	retained, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"))
	if err != nil || string(retained) != string(e.Candidates[e.Selected].Binary) {
		t.Fatal("unmeasured binary", err)
	}
	for _, c := range e.Candidates {
		if len(c.WeightedProof) != 1 || len(c.FinalWeightedProof) != 1 {
			t.Fatal("missing structural proofs")
		}
		for j, inv := range c.Run.Invocations {
			args := w.Calls[j].Arguments
			sum := uint16(*args[4].Value)
			for _, value := range w.Resources[args[0].Resource][:int(*args[1].Value)] {
				sum += uint16(*args[2].Value) + uint16(*args[3].Value) + uint16(value)
			}
			if len(inv.Results) != 1 || inv.Results[0].I != int64(sum) {
				t.Fatal("independent checksum oracle", j, inv.Results, sum)
			}
		}
	}
	hirProfileWeightedSites = false
	hirProfileAutoDirectory = filepath.Join(dir, "legacy")
	if err = compile(source); err == nil {
		t.Fatal("v3 accepted weighted overlap")
	}
}

func TestHIRProfileWeightedSitesValidation(t *testing.T) {
	oldWeighted, oldJoint := hirProfileWeightedSites, hirProfileJointSites
	t.Cleanup(func() { hirProfileWeightedSites, hirProfileJointSites = oldWeighted, oldJoint })
	hirProfileWeightedSites, hirProfileJointSites = true, false
	if validateHIRProfileAuto("a.nanz") == nil {
		t.Fatal("weighted mode without joint sites accepted")
	}
}
