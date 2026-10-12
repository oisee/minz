package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestHIRProfileJointCallerCompilation(t *testing.T) {
	hirProfileTestState(t)
	oldJoint, oldDir, oldFn, oldWork := hirProfileJoint, hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload
	t.Cleanup(func() {
		hirProfileJoint, hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload = oldJoint, oldDir, oldFn, oldWork
	})
	dir := t.TempDir()
	src := filepath.Join(dir, "caller.nanz")
	source := `global data:[u8;5]=[1,2,3,4,5]
global output:u8
fun sum_array(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
fun run()->u8 {
 output=sum_array(&data,5)
 return output
}`
	if err := os.WriteFile(src, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	hirProfileJoint = true
	hirProfileAutoDirectory = filepath.Join(dir, "evidence")
	hirProfileAutoFunction = ""
	hirProfileAutoWorkload = filepath.Join(dir, "workload.json")
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "run"}, {Function: "run"}}}
	if err := writeHIRProfileJSON(hirProfileAutoWorkload, w); err != nil {
		t.Fatal(err)
	}
	outputFile = filepath.Join(dir, "selected.bin")
	if err := compile(src); err != nil {
		t.Fatal(err)
	}
	var e pipeline.ProfileVariantSetEvidence
	data, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "joint-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	expectedCandidates := 4
	if hirProfileIndependentRules {
		expectedCandidates = 8
	}
	if len(e.Candidates) != expectedCandidates {
		t.Fatalf("candidates=%d", len(e.Candidates))
	}
	bin, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(bin) != string(e.Candidates[e.Selected].Binary) {
		t.Fatal("emitted binary differs from measured winner")
	}
	for _, c := range e.Candidates {
		if c.Run.Globals["output"][0] != 15 || len(c.Run.Invocations) != 2 || c.Run.Invocations[0].Results[0].I != 15 {
			t.Fatal(c.Run)
		}
	}
	// All-origin wins in size mode, with its own measured candidates.
	optSize = true
	hirProfileAutoDirectory = filepath.Join(dir, "size")
	outputFile = filepath.Join(dir, "size.bin")
	if err := compile(src); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(hirProfileAutoDirectory, "joint-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	if e.Selected != 0 {
		t.Fatalf("size chose %d", e.Selected)
	}
}

func TestHIRProfileIndependentRulesCallerCompilation(t *testing.T) {
	old := hirProfileIndependentRules
	t.Cleanup(func() { hirProfileIndependentRules = old })
	hirProfileIndependentRules = true
	TestHIRProfileJointCallerCompilation(t)
}

func TestHIRProfileIndependentRulesRequiresJoint(t *testing.T) {
	oldRules, oldJoint := hirProfileIndependentRules, hirProfileJoint
	t.Cleanup(func() { hirProfileIndependentRules, hirProfileJoint = oldRules, oldJoint })
	hirProfileIndependentRules = true
	hirProfileJoint = false
	if err := validateHIRProfileAuto("input.nanz"); err == nil {
		t.Fatal("independent choices accepted outside joint measurement")
	}
}
