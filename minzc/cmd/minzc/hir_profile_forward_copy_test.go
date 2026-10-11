package main

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func forwardCopyCLIState(t *testing.T) {
	t.Helper()
	hirProfileTestState(t)
	oldCopy, oldJoint, oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent := hirProfileForwardCopySites, hirProfileJoint, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules
	oldDir, oldFn, oldWork, oldSize := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload, optSize
	t.Cleanup(func() {
		hirProfileForwardCopySites, hirProfileJoint, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = oldCopy, oldJoint, oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload, optSize = oldDir, oldFn, oldWork, oldSize
	})
	hirProfileForwardCopySites, hirProfileJoint = true, true
	hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false
	hirProfileAutoFunction = ""
}

func TestHIRProfileForwardCopyValidation(t *testing.T) {
	forwardCopyCLIState(t)
	hirProfileAutoDirectory = "evidence"
	hirProfileAutoWorkload = "workload.json"
	for _, name := range []string{"joint", "invariant", "site", "jointsites", "weighted", "independent"} {
		t.Run(name, func(t *testing.T) {
			switch name {
			case "joint":
				hirProfileJoint = false
			case "invariant":
				hirProfileInvariantSites = true
			case "site":
				hirProfileSiteRules = true
			case "jointsites":
				hirProfileJointSites = true
			case "weighted":
				hirProfileWeightedSites = true
			case "independent":
				hirProfileIndependentRules = true
			}
			if validateHIRProfileAuto("copy.nanz") == nil {
				t.Fatal("accepted incompatible copymode", name)
			}
			hirProfileJoint = true
			hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false
		})
	}
}

func TestHIRProfileForwardCopyCompilation(t *testing.T) {
	forwardCopyCLIState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "copy.nanz")
	src := `fun copy_forward(dst:^u8,src:^u8,n:u8)->u8 {
 var i:u8=0
 while i<n {dst[i]=src[i]
 i=i+1}
 return i
}`
	if err := os.WriteFile(source, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 600)
	for i := range data {
		data[i] = byte(i*37 + 11)
	}
	n := int64(255)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 300000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"data": data}, Calls: []hir.ProfileInvocation{{Function: "copy_forward", Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 1}, {Resource: "data"}, {Value: &n}}}}}
	hirProfileAutoWorkload = filepath.Join(dir, "workload.json")
	if err := writeHIRProfileJSON(hirProfileAutoWorkload, w); err != nil {
		t.Fatal(err)
	}
	for _, size := range []bool{false, true} {
		optSize = size
		label := "speed"
		if size {
			label = "size"
		}
		hirProfileAutoDirectory = filepath.Join(dir, label)
		outputFile = filepath.Join(dir, label+".a80")
		if err := compile(source); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "joint-evidence.json"))
		if err != nil {
			t.Fatal(err)
		}
		var e pipeline.ForwardCopySiteEvidence
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		if e.Schema != "minz.forward-copy-site-variant-sets/v1" || len(e.Candidates) != 2 || len(e.EligibleSites) != 1 {
			t.Fatal("copyCLIevidence")
		}
		expected := 1
		if size {
			expected = 0
		}
		if e.Selected != expected {
			t.Fatal("copyCLIwinner", size, e.Selected)
		}
		binary, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"))
		if err != nil || string(binary) != string(e.Candidates[e.Selected].Binary) {
			t.Fatal("copyCLIretainedbinary", err)
		}
		decisions, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "decisions.json"))
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Schema       string          `json:"schema"`
			Selected     int             `json:"selected"`
			Requested    map[string]bool `json:"requested_copy_sites"`
			BinarySHA256 string          `json:"binary_sha256"`
		}
		if err := json.Unmarshal(decisions, &d); err != nil {
			t.Fatal(err)
		}
		if d.Schema != "minz.forward-copy-site-decisions/v1" || d.Selected != e.Selected || d.Requested[e.EligibleSites[0]] != (expected == 1) || d.BinarySHA256 != e.Candidates[e.Selected].BinarySHA256 {
			t.Fatal("copydecisions overwritten/fabricated", string(decisions))
		}
		for _, candidate := range e.Candidates {
			result := candidate.Run.Invocations[0]
			if result.Results[0].I != 255 {
				t.Fatal("copyindex")
			}
			got := result.Snapshot.Resources["data"]
			for i := 1; i <= 255; i++ {
				if got[i] != data[0] {
					t.Fatal("sequential overlapping CLI oracle", i, got[i])
				}
			}
		}
	}
}
