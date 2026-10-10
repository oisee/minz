package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeasuredCommonGraceRulesRealSource(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/grace_hir_constant_if.nanz")
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) {
		return nanz.Parse(string(source), "examples/nanz/grace_hir_constant_if.nanz")
	}
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Calls: []hir.ProfileInvocation{{Function: "wrap", Arguments: []hir.ProfileArgument{literal(255)}}, {Function: "choose", Arguments: []hir.ProfileArgument{literal(42)}}}}
	input, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(input, w)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(run.Profile)
	if err != nil {
		t.Fatal(err)
	}
	result, err := MeasureGraceVariantSets(factory, data, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		m, _ := factory()
		m.GraceRulePolicy = map[string]bool{}
		m.PointerWalkPolicy = map[string]bool{}
		steps, _ := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true})
		t.Fatalf("%v\n%s\n%s", err, steps.MIR2Opt, steps.Assembly)
	}
	if len(result.Evidence.Candidates) != 2 || result.Evidence.Candidates[0].CommonGraceRules || !result.Evidence.Candidates[1].CommonGraceRules {
		t.Fatal("missing genuine baseline/common rule comparison")
	}
	for _, c := range result.Evidence.Candidates {
		if c.Run.Invocations[0].Results[0].I != 0 || c.Run.Invocations[1].Results[0].I != 42 {
			t.Fatal(c)
		}
	}
	t.Logf("baseline %dT/%dB common %dT/%dB selected %d", result.Evidence.Candidates[0].TotalTStates, result.Evidence.Candidates[0].ModuleBytes, result.Evidence.Candidates[1].TotalTStates, result.Evidence.Candidates[1].ModuleBytes, result.Evidence.Selected)
}

func TestMeasuredU16LoopAccumulator(t *testing.T) {
	const source = `fun add_loop(a:u16,b:u16,n:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n {
 s=s+(a+b)
 i=i+1
 }
 return s
}`
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "invariant.nanz") }
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Calls: []hir.ProfileInvocation{}}
	for _, n := range []int64{0, 1, 5, 255} {
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "add_loop", Arguments: []hir.ProfileArgument{literal(65535), literal(2), literal(n)}})
	}
	input, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(input, w)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(run.Profile)
	result, err := MeasureGraceVariantSets(factory, data, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		m, _ := factory()
		m.GraceRulePolicy = map[string]bool{}
		m.PointerWalkPolicy = map[string]bool{}
		steps, _ := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true})
		t.Fatalf("%v\n%s\n%s", err, steps.MIR2Opt, steps.Assembly)
	}
	for _, c := range result.Evidence.Candidates {
		for i, n := range []int64{0, 1, 5, 255} {
			if c.Run.Invocations[i].Results[0].I != n {
				t.Fatal(c.Run)
			}
		}
	}
	t.Logf("baseline %dT/%dB common %dT/%dB selected %d", result.Evidence.Candidates[0].TotalTStates, result.Evidence.Candidates[0].ModuleBytes, result.Evidence.Candidates[1].TotalTStates, result.Evidence.Candidates[1].ModuleBytes, result.Evidence.Selected)
}

func TestMeasuredInvariantBaseGraceRule(t *testing.T) {
	factory := func() (*hir.Module, error) { return nanz.Parse(graceRegionSource, "region.nanz") }
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	ptr := func(s string) hir.ProfileArgument { return hir.ProfileArgument{Resource: s} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"buf": {9, 1, 2, 3, 4, 5}, "target": {1, 2, 3, 4, 5}}, Calls: []hir.ProfileInvocation{}}
	for _, n := range []int64{0, 1, 5} {
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "region_match", Arguments: []hir.ProfileArgument{ptr("buf"), literal(1), ptr("target"), literal(n)}})
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(raw.Profile)
	r, err := MeasureGraceVariantSets(factory, data, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Evidence.Candidates) != 2 {
		t.Fatal("expected disabled/enabled common rules")
	}
	if strings.Contains(r.Evidence.Candidates[0].HIR, "__grace_base_") || !strings.Contains(r.Evidence.Candidates[1].HIR, "__grace_base_") {
		t.Fatal("invariant rewrite missing or baseline already rewritten")
	}
	for _, c := range r.Evidence.Candidates {
		for _, call := range c.Run.Invocations {
			if call.Results[0].I != 1 {
				t.Fatal(c.Run)
			}
		}
	}
	t.Logf("baseline %dT/%dB hoisted %dT/%dB selected %d", r.Evidence.Candidates[0].TotalTStates, r.Evidence.Candidates[0].ModuleBytes, r.Evidence.Candidates[1].TotalTStates, r.Evidence.Candidates[1].ModuleBytes, r.Evidence.Selected)
}

func TestMeasuredGraceRejectsUnrealizedPointerChoice(t *testing.T) {
	source := "fun budget(){\n" + strings.Repeat("if false {}\n", 100) + "}\n" + jointSource
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "budget.nanz") }
	_, w, _ := jointFixture(t)
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(run.Profile)
	result, err := MeasureGraceVariantSets(factory, data, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if result != nil || err == nil || !strings.Contains(err.Error(), "requested pointer choice was not realized") {
		t.Fatalf("got %v, %v", result, err)
	}
}

func TestMeasuredCommonGraceArtifacts(t *testing.T) {
	dir := os.Getenv("MINZ_PGO_ARTIFACT_DIR")
	if dir == "" {
		t.Skip("artifact export only")
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(graceRegionSource, "region.nanz") }
	n := int64(5)
	offset := int64(1)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"buf": {9, 1, 2, 3, 4, 5}, "target": {1, 2, 3, 4, 5}}}
	for _, size := range []int64{0, 1, n} {
		x := size
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "region_match", Arguments: []hir.ProfileArgument{{Resource: "buf"}, {Value: &offset}, {Resource: "target"}, {Value: &x}}})
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(run.Profile)
	r, err := MeasureGraceVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"workload.json": w, "profile.json": run.Profile, "run.json": run, "evidence.json": r.Evidence} {
		b, _ := json.MarshalIndent(value, "", "  ")
		if err = os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for name, b := range map[string][]byte{"region.nanz": []byte(graceRegionSource), "selected.a80": []byte(r.Steps.Assembly), "selected.mir": []byte(r.Steps.MIR2Opt), "selected.bin": r.Binary} {
		if err = os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
