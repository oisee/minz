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

func mixedCopyRegionCLIState(t *testing.T) {
	t.Helper()
	hirProfileTestState(t)
	oldMixedRegion := hirProfileMixedCopyRegionSites
	oldRegion, oldCopy, oldMixed, oldJoint := hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint
	oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent := hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules
	oldDir, oldFn, oldWork := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileMixedCopyRegionSites = oldMixedRegion
		hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint = oldRegion, oldCopy, oldMixed, oldJoint
		hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldDir, oldFn, oldWork
	})
	hirProfileMixedCopyRegionSites, hirProfileJoint = true, true
	hirProfileRegionCompareSites = false
	hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites = false, false
	hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false
	hirProfileAutoFunction = ""
}
func TestHIRProfileMixedCopyRegionValidation(t *testing.T) {
	mixedCopyRegionCLIState(t)
	for _, name := range []string{"joint", "region", "copy", "mixed", "invariant", "site", "jointsites", "weighted", "independent", "auto", "workload", "restriction", "separate"} {
		t.Run(name, func(t *testing.T) {
			hirProfileAutoDirectory = "evidence"
			hirProfileAutoWorkload = "workload.json"
			hirProfileAutoFunction = ""
			hirProfile = hirProfileFlags{}
			hirProfileJoint = true
			hirProfileRegionCompareSites = false
			hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
			switch name {
			case "joint":
				hirProfileJoint = false
			case "region":
				hirProfileRegionCompareSites = true
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
	hirProfileRegionCompareSites = false
	hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
	if err := validateHIRProfileAuto("region.nanz"); err != nil {
		t.Fatal("valid region mode refused", err)
	}
}
func TestHIRProfileMixedCopyRegionCompilation(t *testing.T) {
	mixedCopyRegionCLIState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "region.nanz")
	const src = `fun region_eq(src:^u8,arena:^u8,src_start:u16,arena_ptr:u16,len:u8)->u8 {
 var i:u8=0
 while i<len {
  if src[src_start+i]!=arena[arena_ptr+1+i]{return 0}
  i=i+1
 }
 return 1
}
fun copy_forward(dst:^u8,src:^u8,n:u8)->u8 {var i:u8=0 while i<n {dst[i]=src[i] i=i+1} return i}`
	if err := os.WriteFile(source, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	left, right := make([]byte, 256), make([]byte, 257)
	for i := range left {
		left[i] = byte(i*37 + 11)
		right[i+1] = left[i]
	}
	n, zero := int64(255), int64(0)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 300000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"left": left, "right": right, "dest": make([]byte, 256)}, Calls: []hir.ProfileInvocation{{Function: "copy_forward", Arguments: []hir.ProfileArgument{{Resource: "dest"}, {Resource: "left"}, {Value: &n}}}, {Function: "region_eq", Arguments: []hir.ProfileArgument{{Resource: "left"}, {Resource: "right"}, {Value: &zero}, {Value: &zero}, {Value: &n}}}}}
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
		var e pipeline.MixedCopyRegionSiteEvidence
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		if e.Schema != "minz.mixed-copy-region-site-variant-sets/v1" || len(e.Candidates) != 4 || len(e.EligibleCopySites) != 1 || len(e.EligibleRegionSites) != 1 {
			t.Fatal("incorrect region CLI scope")
		}
		best := 0
		for i, c := range e.Candidates {
			if (!size && (c.TotalTStates < e.Candidates[best].TotalTStates || (c.TotalTStates == e.Candidates[best].TotalTStates && c.ModuleBytes < e.Candidates[best].ModuleBytes))) || (size && (c.ModuleBytes < e.Candidates[best].ModuleBytes || (c.ModuleBytes == e.Candidates[best].ModuleBytes && c.TotalTStates < e.Candidates[best].TotalTStates))) {
				best = i
			}
			if len(c.Proofs.Copies) != 1 || len(c.Proofs.Regions) != 1 || !reflect.DeepEqual(c.Proofs, c.FinalProofs) {
				t.Fatal("combined proofs missing")
			}
			if c.Proofs.Copies[0].Enabled != c.RequestedSites.Copy[e.EligibleCopySites[0]] || c.Proofs.Regions[0].Enabled != c.RequestedSites.Region[e.EligibleRegionSites[0]] {
				t.Fatal("typed requested choices/proofs differ")
			}

			copyInv := c.Run.Invocations[0]
			if copyInv.Results[0].I != 255 {
				t.Fatal("copy independent length")
			}
			dest := append([]byte(nil), w.Resources["dest"]...)
			copy(dest[:255], left[:255])
			if !reflect.DeepEqual(copyInv.Snapshot.Resources["dest"], dest) {
				t.Fatal("copy sequential memory")
			}
			for j := range dest {
				want := uint64(0)
				if j < 255 {
					want = 1
				}
				if copyInv.Access.Resources["dest"].Writes[j] != want || copyInv.Access.Resources["left"].Reads[j] != want {
					t.Fatal("copy exact traffic", j)
				}
			}
			inv := c.Run.Invocations[1]
			if inv.Results[0].I != 1 || !reflect.DeepEqual(inv.Snapshot.Resources["left"], left) || !reflect.DeepEqual(inv.Snapshot.Resources["right"], right) || !reflect.DeepEqual(inv.Snapshot.Resources["dest"], dest) || inv.Access == nil {
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
			Schema        string                     `json:"schema"`
			Selected      int                        `json:"selected"`
			Requested     hir.MixedCopyRegionChoices `json:"requested_choices"`
			Proofs        hir.MixedCopyRegionProof   `json:"proofs"`
			FinalProofs   hir.MixedCopyRegionProof   `json:"postcompile_proofs"`
			Binary        string                     `json:"binary_sha256"`
			Evidence      string                     `json:"evidence_sha256"`
			Compatibility *hir.ProfileCompatibility  `json:"compatibility"`
		}
		b, err = os.ReadFile(filepath.Join(hirProfileAutoDirectory, "decisions.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		if d.Schema != "minz.mixed-copy-region-site-decisions/v1" || d.Selected != best || !reflect.DeepEqual(d.Requested, selected.RequestedSites) || !reflect.DeepEqual(d.Proofs, selected.Proofs) || !reflect.DeepEqual(d.FinalProofs, selected.FinalProofs) || d.Binary != selected.BinarySHA256 || d.Evidence != e.SHA256 || !reflect.DeepEqual(d.Compatibility, e.Compatibility) {
			t.Fatal("region decisions overwritten or unbound")
		}
		digest := e
		e.SHA256 = ""
		encoded, _ := json.Marshal(e)
		if digest.SHA256 != fmt.Sprintf("%x", sha256.Sum256(encoded)) {
			t.Fatal("region CLI evidence SHA")
		}
		if artifactDir := os.Getenv("MINZ_MIXED_COPY_REGION_CLI_ARTIFACTS"); artifactDir != "" {
			dest := filepath.Join(artifactDir, label)
			if err := os.MkdirAll(dest, 0755); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(hirProfileAutoDirectory)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dest, entry.Name()), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dest, "source.nanz"), []byte(src), 0644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("mixed copy region CLI objective=%s selected%d %dT/%dB sha%s", label, best, selected.TotalTStates, selected.ModuleBytes, selected.BinarySHA256)
	}
}
