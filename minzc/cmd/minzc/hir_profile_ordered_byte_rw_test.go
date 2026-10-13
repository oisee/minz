package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func orderedByteRWCLIState(t *testing.T) {
	t.Helper()
	hirProfileTestState(t)
	oldOrdered, oldRegion, oldCopy, oldMixed, oldJoint := hirProfileOrderedByteFunctions, hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint
	oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent := hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules
	oldComposition := hirProfileMixedCopyRegionSites
	oldRW := hirProfileOrderedByteRWFunctions
	hirProfileOrderedByteRWFunctions = true
	oldDir, oldFn, oldWork := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileMixedCopyRegionSites = oldComposition
		hirProfileOrderedByteRWFunctions = oldRW
		hirProfileOrderedByteFunctions, hirProfileRegionCompareSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileJoint = oldOrdered, oldRegion, oldCopy, oldMixed, oldJoint
		hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldDir, oldFn, oldWork
	})
	hirProfileOrderedByteFunctions, hirProfileJoint = false, true
	hirProfileRegionCompareSites, hirProfileMixedCopyRegionSites, hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites = false, false, false, false
	hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false
	hirProfileAutoFunction = ""
}
func TestHIRProfileOrderedByteRWValidation(t *testing.T) {
	orderedByteRWCLIState(t)
	for _, name := range []string{"joint", "readonly", "region", "composition", "copy", "mixed", "invariant", "site", "jointsites", "weighted", "independent", "auto", "workload", "restriction", "separate"} {
		t.Run(name, func(t *testing.T) {
			hirProfileAutoDirectory = "evidence"
			hirProfileAutoWorkload = "workload.json"
			hirProfileAutoFunction = ""
			hirProfile = hirProfileFlags{}
			hirProfileJoint = true
			hirProfileOrderedByteFunctions = false
			hirProfileRegionCompareSites, hirProfileMixedCopyRegionSites = false, false
			hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
			switch name {
			case "readonly":
				hirProfileOrderedByteFunctions = true
			case "joint":
				hirProfileJoint = false
			case "region":
				hirProfileRegionCompareSites = true
			case "composition":
				hirProfileMixedCopyRegionSites = true
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
				hirProfileAutoFunction = "read_word"
			case "separate":
				hirProfile.profile = "profile.json"
			}
			if validateHIRProfileAuto("ordered-byte-rw.nanz") == nil {
				t.Fatal("incompatible ordered-byte read/write mode accepted", name)
			}
		})
	}
	// Shared package globals are restored by the outer state helper; each sibling
	// validation mode also returns to a clean state before this successful check.
	hirProfileAutoDirectory, hirProfileAutoWorkload, hirProfileAutoFunction = "evidence", "workload.json", ""
	hirProfile = hirProfileFlags{}
	hirProfileJoint = true
	hirProfileOrderedByteFunctions = false
	hirProfileRegionCompareSites, hirProfileMixedCopyRegionSites = false, false
	hirProfileForwardCopySites, hirProfileMixedCopyWeightedSites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false, false
	if err := validateHIRProfileAuto("ordered-byte-rw.nanz"); err != nil {
		t.Fatal("valid ordered-byte read/write mode refused", err)
	}
}

func TestHIRProfileOrderedByteRWCompilation(t *testing.T) {
	orderedByteRWCLIState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "ordered-byte-rw.nanz")
	const src = `fun read_word(arr:^u8,idx:u16)->u16 {
 var base:u16=idx*2
 return u16(arr[base])+u16(arr[base+1])*256
}
fun write_word(arr:^u8,idx:u16,value:u16) {
 var base:u16=idx*2
 arr[base]=u8(value)
 arr[base+1]=u8(value/256)
}`
	if err := os.WriteFile(source, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	zero, value := int64(0), int64(0x1234)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"data": {0, 0, 99}}, Calls: []hir.ProfileInvocation{
		{Function: "write_word", Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &zero}, {Value: &value}}},
		{Function: "read_word", Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &zero}}},
	}}
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
		var e pipeline.OrderedByteRWFunctionEvidence
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatal(err)
		}
		if e.Schema != "minz.ordered-byte-rw-function-variant-sets/v1" || len(e.Candidates) != 4 || len(e.EligibleSites) != 2 {
			t.Fatal("incorrect read/write CLI scope")
		}
		for i, c := range e.Candidates {
			if len(c.Proofs) != 2 || len(c.FinalProofs) != 2 {
				t.Fatal("read/write proofs missing")
			}
			for j, site := range e.EligibleSites {
				choice := c.RequestedSites[site.ID]
				if choice.Kind != site.Kind || choice.Enabled != (i&(1<<(1-j)) != 0) {
					t.Fatal("typed read/write mask")
				}
			}
			for j, inv := range c.Run.Invocations {
				if !reflect.DeepEqual(inv.Snapshot.Resources["data"], []byte{0x34, 0x12, 99}) || inv.Access == nil {
					t.Fatal("independent resource state")
				}
				if j == 0 {
					if len(inv.Results) != 0 || !reflect.DeepEqual(inv.Access.Resources["data"].Reads, []uint64{0, 0, 0}) || !reflect.DeepEqual(inv.Access.Resources["data"].Writes, []uint64{1, 1, 0}) {
						t.Fatal("independent write traffic")
					}
				} else {
					if len(inv.Results) != 1 || inv.Results[0].I != 0x1234 || !reflect.DeepEqual(inv.Access.Resources["data"].Reads, []uint64{1, 1, 0}) || !reflect.DeepEqual(inv.Access.Resources["data"].Writes, []uint64{0, 0, 0}) {
						t.Fatal("independent read traffic")
					}
				}
			}
		}
		selected := e.Candidates[e.Selected]
		binary, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"))
		if err != nil || !reflect.DeepEqual(binary, selected.Binary) {
			t.Fatal("retained binary", err)
		}
		assembly, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatal(err)
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + string(assembly))
		if err != nil || assembled == nil || len(assembled.Errors) != 0 || !reflect.DeepEqual(assembled.Binary, binary) {
			t.Fatal("emitted ASM binary differs", err)
		}
		var d struct {
			Schema      string                        `json:"schema"`
			Requested   map[string]hir.FunctionChoice `json:"requested_functions"`
			Proofs      []hir.OrderedByteRWProof      `json:"proofs"`
			FinalProofs []hir.OrderedByteRWProof      `json:"postcompile_proofs"`
			Selected    int                           `json:"selected"`
			Binary      string                        `json:"binary_sha256"`
			Evidence    string                        `json:"evidence_sha256"`
		}
		b, err = os.ReadFile(filepath.Join(hirProfileAutoDirectory, "decisions.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		if d.Schema != "minz.ordered-byte-rw-function-decisions/v1" || d.Selected != e.Selected || !reflect.DeepEqual(d.Requested, selected.RequestedSites) || !reflect.DeepEqual(d.Proofs, selected.Proofs) || !reflect.DeepEqual(d.FinalProofs, selected.FinalProofs) || d.Binary != selected.BinarySHA256 || d.Evidence != e.SHA256 {
			t.Fatal("unbound read/write decisions")
		}
		saved := e.SHA256
		e.SHA256 = ""
		encoded, _ := json.Marshal(e)
		if saved != fmt.Sprintf("%x", sha256.Sum256(encoded)) {
			t.Fatal("typed evidence digest")
		}
		t.Logf("read/write CLI objective=%s selected%d %dT/%dB SHA%s", label, d.Selected, selected.TotalTStates, selected.ModuleBytes, selected.BinarySHA256)
	}
}
