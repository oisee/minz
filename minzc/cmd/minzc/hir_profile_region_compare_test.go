package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func regionCompareCLIState(t *testing.T) {
	t.Helper()
	hirProfileTestState(t)
	oldRegion, oldCopy, oldMixed, oldJoint := hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint
	oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent := hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules
	oldDir, oldFn, oldWork := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint = oldRegion, oldCopy, oldMixed, oldJoint
		hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldDir, oldFn, oldWork
	})
	hirProfileRegionCompareSites, hirProfileJoint = true, true
	hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites = false, false
	hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false
	hirProfileAutoFunction = ""
}
func TestHIRProfileRegionCompareValidation(t *testing.T) {
	regionCompareCLIState(t)
	for _, name := range []string{"joint", "copy", "mixed", "invariant", "site", "jointsites", "weighted", "independent", "auto", "workload", "restriction", "separate"} {
		t.Run(name, func(t *testing.T) {
			hirProfileAutoDirectory = "evidence"
			hirProfileAutoWorkload = "workload.json"
			hirProfileAutoFunction = ""
			hirProfile = hirProfileFlags{}
			hirProfileJoint = true
			hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
			switch name {
			case "joint":
				hirProfileJoint = false
			case "copy":
				hirProfileForwardCopySites = true
			case "mixed":
				hirProfileMixedCopyWeightedSites = true
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
			case "auto":
				hirProfileAutoDirectory = ""
			case "workload":
				hirProfileAutoWorkload = ""
			case "restriction":
				hirProfileAutoFunction = "region_eq"
			case "separate":
				hirProfile.profile = "profile.json"
			}
			if validateHIRProfileAuto("region.nanz") == nil {
				t.Fatal("incompatible region mode accepted", name)
			}
		})
	}
	// Shared package globals are restored by the outer state helper; each sibling
	// validation mode also returns to a clean state before this successful check.
	hirProfileAutoDirectory, hirProfileAutoWorkload, hirProfileAutoFunction = "evidence", "workload.json", ""
	hirProfile = hirProfileFlags{}
	hirProfileJoint = true
	hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
	if err := validateHIRProfileAuto("region.nanz"); err != nil {
		t.Fatal("valid region mode refused", err)
	}
}
func TestHIRProfileRegionCompareCompilation(t *testing.T) {
	regionCompareCLIState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "region.nanz")
	const src = `fun region_eq(src:^u8,arena:^u8,src_start:u16,arena_ptr:u16,len:u8)->u8 {
 var i:u8=0
 while i<len {
  if src[src_start+i]!=arena[arena_ptr+1+i]{return 0}
  i=i+1
 }
 return 1
}`
	if err := os.WriteFile(source, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	left, right := make([]byte, 256), make([]byte, 257)
	for i := range left {
		left[i] = byte(i*37 + 11)
		right[i+1] = left[i]
	}
	n, zero := int64(255), int64(0)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 300000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"left": left, "right": right}, Calls: []hir.ProfileInvocation{{Function: "region_eq", Arguments: []hir.ProfileArgument{{Resource: "left"}, {Resource: "right"}, {Value: &zero}, {Value: &zero}, {Value: &n}}}}}
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
		var e pipeline.RegionCompareSiteEvidence
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		if e.Schema != "minz.region-compare-site-variant-sets/v1" || len(e.Candidates) != 2 || len(e.EligibleSites) != 1 {
			t.Fatal("incorrect region CLI scope")
		}
		best := 0
		for i, c := range e.Candidates {
			if (!size && c.TotalTStates < e.Candidates[best].TotalTStates) || (size && c.ModuleBytes < e.Candidates[best].ModuleBytes) {
				best = i
			}
			if len(c.Proofs) != 1 || len(c.FinalProofs) != 1 || c.Proofs[0].Enabled != (i == 1) || c.FinalProofs[0].Enabled != (i == 1) {
				t.Fatal("region proofs missing")
			}
			inv := c.Run.Invocations[0]
			if inv.Results[0].I != 1 || !reflect.DeepEqual(inv.Snapshot.Resources, w.Resources) || inv.Access == nil {
				t.Fatal("independent equal-region state")
			}
			for j := range left {
				want := uint64(0)
				if j < 255 {
					want = 1
				}
				if inv.Access.Resources["left"].Reads[j] != want || inv.Access.Resources["left"].Writes[j] != 0 {
					t.Fatal("left ordered read bounds", j)
				}
			}
			for j := range right {
				want := uint64(0)
				if j > 0 && j <= 255 {
					want = 1
				}
				if inv.Access.Resources["right"].Reads[j] != want || inv.Access.Resources["right"].Writes[j] != 0 {
					t.Fatal("right ordered read bounds", j)
				}
			}
		}
		if e.Selected != best {
			t.Fatal("region actual objective selection")
		}
		selected := e.Candidates[best]
		binary, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"))
		if err != nil || !reflect.DeepEqual(binary, selected.Binary) {
			t.Fatal("retained region binary", err)
		}
		assembly, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatal(err)
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + string(assembly))
		if err != nil || assembled == nil || len(assembled.Errors) != 0 || !reflect.DeepEqual(assembled.Binary, binary) {
			t.Fatal("emitted assembly differs from measured binary", err)
		}
		var d struct {
			Schema        string                    `json:"schema"`
			Selected      int                       `json:"selected"`
			Requested     map[string]bool           `json:"requested_region_compare_sites"`
			Proofs        []hir.RegionCompareProof  `json:"proofs"`
			FinalProofs   []hir.RegionCompareProof  `json:"postcompile_proofs"`
			Binary        string                    `json:"binary_sha256"`
			Evidence      string                    `json:"evidence_sha256"`
			Compatibility *hir.ProfileCompatibility `json:"compatibility"`
		}
		b, err = os.ReadFile(filepath.Join(hirProfileAutoDirectory, "decisions.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		if d.Schema != "minz.region-compare-site-decisions/v1" || d.Selected != best || !reflect.DeepEqual(d.Requested, selected.RequestedSites) || !reflect.DeepEqual(d.Proofs, selected.Proofs) || !reflect.DeepEqual(d.FinalProofs, selected.FinalProofs) || d.Binary != selected.BinarySHA256 || d.Evidence != e.SHA256 || !reflect.DeepEqual(d.Compatibility, e.Compatibility) {
			t.Fatal("region decisions overwritten or unbound")
		}
		digest := e
		e.SHA256 = ""
		encoded, _ := json.Marshal(e)
		if digest.SHA256 != fmt.Sprintf("%x", sha256.Sum256(encoded)) {
			t.Fatal("region CLI evidence SHA")
		}
		t.Logf("region CLI objective=%s selected%d %dT/%dB sha%s", label, best, selected.TotalTStates, selected.ModuleBytes, selected.BinarySHA256)
	}
}
