package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func forwardCopyCLIState(t *testing.T) {
	t.Helper()
	hirProfileTestState(t)
	oldMixed := hirProfileMixedCopyWeightedSites
	oldCopy, oldJoint, oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent := hirProfileForwardCopySites, hirProfileJoint, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules
	oldDir, oldFn, oldWork, oldSize := hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload, optSize
	t.Cleanup(func() {
		hirProfileMixedCopyWeightedSites = oldMixed
		hirProfileForwardCopySites, hirProfileJoint, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = oldCopy, oldJoint, oldInvariant, oldSite, oldJointSites, oldWeighted, oldIndependent
		hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload, optSize = oldDir, oldFn, oldWork, oldSize
	})
	hirProfileMixedCopyWeightedSites = false
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
		for i, c := range e.Candidates {
			t.Logf("copy size=%v candidate=%d: %d T / %d B", size, i, c.TotalTStates, c.ModuleBytes)
		}
		// origin/main 76e8070d4 selects the original after byte-loop lowering.
		expected := 0
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

func mixedCopyWeightedCLIState(t *testing.T) {
	forwardCopyCLIState(t)
	hirProfileForwardCopySites = false
	hirProfileMixedCopyWeightedSites = true
}
func TestHIRProfileMixedCopyWeightedValidation(t *testing.T) {
	mixedCopyWeightedCLIState(t)
	hirProfileAutoDirectory = "evidence"
	hirProfileAutoWorkload = "workload.json"
	for _, name := range []string{"joint", "auto", "workload", "copy", "invariant", "site", "jointsites", "weighted", "independent", "separate"} {
		t.Run(name, func(t *testing.T) {
			old := hirProfile
			switch name {
			case "joint":
				hirProfileJoint = false
			case "auto":
				hirProfileAutoDirectory = ""
			case "workload":
				hirProfileAutoWorkload = ""
			case "copy":
				hirProfileForwardCopySites = true
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
			case "separate":
				hirProfile.decisions = "else.json"
			}
			if validateHIRProfileAuto("mixed.nanz") == nil {
				t.Fatal("incompatible mixed mode", name)
			}
			hirProfile = old
			hirProfileJoint = true
			hirProfileAutoDirectory = "evidence"
			hirProfileAutoWorkload = "workload.json"
			hirProfileForwardCopySites, hirProfileInvariantSites, hirProfileSiteRules, hirProfileJointSites, hirProfileWeightedSites, hirProfileIndependentRules = false, false, false, false, false, false
		})
	}
}
func TestHIRProfileMixedCopyWeightedCompilation(t *testing.T) {
	mixedCopyWeightedCLIState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "mixed.nanz")
	src := `global packet_calls:u8=250
 global packet_last:u16=0
 fun copy_forward(dst:^u8,src:^u8,n:u8)->u8 {var i:u8=0
 while i<n {dst[i]=src[i]
 i=i+1}
 return i}
 fun weighted_checksum(ptr:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {var sum:u16=seed
 var i:u8=0
 while i<n {sum=sum+(a+b)
 sum=sum+u16(ptr[i])
 i=i+1}
 return sum}
 fun copied_packet(dst:^u8,src:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {
 var copied:u8=copy_forward(dst,src,n)
 var value:u16=weighted_checksum(dst,copied,a,b,seed)
 packet_calls=packet_calls+1
 packet_last=value
 return value}`
	if err := os.WriteFile(source, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 600)
	for i := range data {
		data[i] = byte(i*37 + 11)
	}
	n, a, b, seed := int64(255), int64(65535), int64(2), int64(65530)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Globals: map[string][]byte{"packet_calls": {250}, "packet_last": {0, 0}}, Resources: map[string][]byte{"data": data}, Calls: []hir.ProfileInvocation{{Function: "copied_packet", Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 1}, {Resource: "data"}, {Value: &n}, {Value: &a}, {Value: &b}, {Value: &seed}}}}}
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
		dataJSON, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "joint-evidence.json"))
		if err != nil {
			t.Fatal(err)
		}
		var e pipeline.MixedCopyWeightedEvidence
		if err := json.Unmarshal(dataJSON, &e); err != nil {
			t.Fatal(err)
		}
		if e.Schema != "minz.mixed-copy-weighted-variant-sets/v1" || len(e.Candidates) != 8 || len(e.EligibleCopySites) != 1 || len(e.EligibleWeightedSites) != 1 {
			t.Fatal("mixed CLI scope")
		}
		for i, c := range e.Candidates {
			t.Logf("mixed size=%v candidate=%d: %d T / %d B", size, i, c.TotalTStates, c.ModuleBytes)
		}
		// origin/main 76e8070d4 measures mask 3 as the speed winner.
		expected := 3
		if size {
			expected = 2
		}
		if e.Selected != expected {
			t.Fatal("mixed winner", size, e.Selected)
		}
		binary, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"))
		if err != nil || !reflect.DeepEqual(binary, e.Candidates[expected].Binary) {
			t.Fatal("mixed retained binary", err)
		}
		dJSON, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "decisions.json"))
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Schema      string                       `json:"schema"`
			Selected    int                          `json:"selected"`
			Choices     hir.MixedCopyWeightedChoices `json:"requested_choices"`
			Proofs      hir.MixedCopyWeightedProof   `json:"proofs"`
			FinalProofs hir.MixedCopyWeightedProof   `json:"postcompile_proofs"`
			BinarySHA   string                       `json:"binary_sha256"`
			EvidenceSHA string                       `json:"evidence_sha256"`
		}
		if err := json.Unmarshal(dJSON, &d); err != nil {
			t.Fatal(err)
		}
		c := e.Candidates[expected]
		if d.Schema != "minz.mixed-copy-weighted-site-decisions/v1" || d.Selected != expected || !reflect.DeepEqual(d.Choices, c.RequestedSites) || !reflect.DeepEqual(d.Proofs, c.Proofs) || !reflect.DeepEqual(d.FinalProofs, c.FinalProofs) || d.BinarySHA != c.BinarySHA256 || d.EvidenceSHA != e.SHA256 {
			t.Fatal("mixed decisions overwritten/fabricated")
		}
		digest := e
		digest.SHA256 = ""
		encoded, _ := json.Marshal(digest)
		if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(encoded)) {
			t.Fatal("mixed CLI evidence digest")
		}
		result := uint16(seed)
		for i := 0; i < int(n); i++ {
			result += uint16(a) + uint16(b) + uint16(data[0])
		}
		for _, candidate := range e.Candidates {
			inv := candidate.Run.Invocations[0]
			if inv.Results[0].I != int64(result) || !reflect.DeepEqual(inv.Snapshot.Globals["packet_calls"], []byte{251}) || !reflect.DeepEqual(inv.Snapshot.Globals["packet_last"], []byte{byte(result), byte(result >> 8)}) {
				t.Fatal("mixed independent CLI oracle")
			}
			expectedMemory := append([]byte(nil), data...)
			for i := 1; i <= 255; i++ {
				expectedMemory[i] = data[0]
			}
			if !reflect.DeepEqual(inv.Snapshot.Resources["data"], expectedMemory) {
				t.Fatal("mixed sequential alias")
			}
			accesses := inv.Access.Resources["data"]
			reads, writes := make([]uint64, len(data)), make([]uint64, len(data))
			for i := 0; i < 255; i++ {
				reads[i]++
				writes[i+1]++
				reads[i+1]++
			}
			if !reflect.DeepEqual(accesses.Reads, reads) || !reflect.DeepEqual(accesses.Writes, writes) {
				t.Fatal("mixed CLI exact traffic")
			}
		}
	}
}
