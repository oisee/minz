package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestMultiFunctionProfileVMZ80(t *testing.T) {
	const sum = `fun NAME(ptr: ^u8, n: u8) -> u8 {
 var s: u8 = 0
 var i: u8 = 0
 while i < n {
 s = s + ptr[i]
 i = i + 1
 }
 return s
 }
 `
	source := strings.ReplaceAll(sum, "NAME", "short_sum") + strings.ReplaceAll(sum, "NAME", "long_sum")
	parse := func() *hir.Module {
		m, err := nanz.Parse(source, "multi.nanz")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	short, long := int64(1), int64(40)
	input := make([]byte, 40)
	memory := map[uint16]byte{}
	for i := range input {
		input[i] = byte(i + 1)
		memory[uint16(0x4000+i)] = input[i]
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-pbqp-default-jrs/v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"input": input}, Calls: []hir.ProfileInvocation{
		{Function: "short_sum", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &short}}},
		{Function: "long_sum", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &long}}},
	}}
	m := parse()
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	c, err := hir.BuildWorkloadProfileContext(m, w)
	if err != nil {
		t.Fatal(err)
	}
	// Preserve this fixture's measured contract-optimized compilation context.
	// ContractOpt=false now correctly leaves contracts untouched.
	options := Options{UseHIRGrace: true, AssertMode: "none", ContractOpt: true}
	config, err := ProfileCompilationID(options, "default")
	if err != nil {
		t.Fatal(err)
	}
	// Coefficients come from the existing exact-sum Z80 fixture. This test
	// executes the selected variants with current backend preservation costs;
	// these synthetic coefficients do not mint production measurement evidence.
	model := hir.LoopCostModel{CompilerID: c.CompilerID, SemanticsID: c.SemanticsID, TargetID: c.TargetID, HIRSHA256: run.Profile.Compatibility.HIRSHA256, CompilationID: config, EvidenceID: "test-fixture", OriginalBytes: 128, PointerBytes: 177, OriginalEntry: 69, OriginalIteration: 141, FastEntry: 189, FastZeroEntry: 189, FastZeroMeasured: true, FastIteration: 89}
	models := []hir.LoopCostModel{model, model}
	models[0].Function, models[1].Function = "short_sum", "long_sum"
	observations, err := json.Marshal(run.Profile)
	if err != nil {
		t.Fatal(err)
	}
	options.ProfileSelection = &ProfileSelection{Observations: observations, Workload: w, Models: models, AssemblerMode: "default"}
	selected, err := applyPipelineProfile(m, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || len(m.PointerWalkPolicy) != 2 {
		t.Fatalf("lost decisions: %+v", selected)
	}
	byFunction := map[string]hir.ProfileDecision{}
	for _, d := range selected {
		byFunction[d.Model.Function] = d
	}
	if byFunction["short_sum"].Variant != "original" || byFunction["long_sum"].Variant != "pointer-walk" {
		t.Fatal(selected)
	}
	vm := mir2.NewVM(hir.LowerModule(m))
	vm.EnsureHeap(0x4100)
	vm.WriteHeapBytes(0x4000, input)
	for _, call := range []struct {
		name   string
		n      int64
		cycles int
	}{{"short_sum", 1, 95}, {"long_sum", 40, 1436}} {
		// short_sum now uses the ordinary counted-pointer pass: 41 + 54*n.
		// The guarded long_sum remains unchanged. The synthetic selector
		// coefficients deliberately test dispatch, not production cost evidence.
		want := call.n * (call.n + 1) / 2 & 255
		values, err := vm.Call(call.name, []mir2.Value{{I: 0x4000}, {I: call.n}})
		if err != nil || len(values) != 1 || values[0].I != want {
			t.Fatalf("VM %s: %v %v", call.name, values, err)
		}
		directory := ""
		if root := os.Getenv("MINZ_MULTI_PROFILE_ARTIFACTS"); root != "" {
			directory = filepath.Join(root, call.name)
		}
		cycles, _ := measureProfileZ80(t, parse(), call.name, options, []int64{0x4000, call.n}, memory, want, directory)
		if cycles != call.cycles {
			t.Fatalf("%s cycles=%d want=%d", call.name, cycles, call.cycles)
		}
	}
	if !reflect.DeepEqual(vm.ReadHeap(0x4000, 40), input) {
		t.Fatal("VM changed input")
	}
	steps, err := CompileHIRSteps(parse(), options)
	if err != nil || !reflect.DeepEqual(steps.ProfileDecisions, selected) {
		t.Fatalf("pipeline lost per-site decisions: %v %v", steps, err)
	}
	// Every request field is excluded from the compilation ID, including Models.
	id, err := ProfileCompilationID(options, "default")
	if err != nil || id != config {
		t.Fatalf("selection changed compilation ID: %s %v", id, err)
	}
	changed := options
	changed.ProfileSelection = &ProfileSelection{Models: []hir.LoopCostModel{{Function: "arbitrary"}}, Model: model, Observations: []byte("arbitrary"), AssemblerMode: "other"}
	id, err = ProfileCompilationID(changed, "default")
	if err != nil || id != config {
		t.Fatalf("request leaked into compilation ID: %s %v", id, err)
	}
	changed = options
	changed.OptSize = true
	id, err = ProfileCompilationID(changed, "default")
	if err != nil || id == config {
		t.Fatal("real options excluded from compilation ID")
	}
	for _, kind := range []string{"stale first config", "stale second config", "duplicate", "stale hir", "stale compiler", "missing function", "both", "empty", "missing", "stale profile", "size config", "contract strategy"} {
		t.Run(kind, func(t *testing.T) {
			request := *options.ProfileSelection
			request.Models = append([]hir.LoopCostModel{}, models...)
			badOptions := options
			switch kind {
			case "stale first config":
				request.Models[0].CompilationID = "stale"
			case "stale second config":
				request.Models[1].CompilationID = "stale"
			case "duplicate":
				request.Models[1] = request.Models[0]
			case "stale hir":
				request.Models[1].HIRSHA256 = "stale"
			case "stale compiler":
				request.Models[1].CompilerID = "stale"
			case "missing function":
				request.Models[1].Function = ""
			case "both":
				request.Model = model
			case "empty":
				request.Models = []hir.LoopCostModel{}
			case "missing":
				request.Models = nil
			case "stale profile":
				request.Observations = []byte("{}")
			case "contract strategy":
				badOptions.ContractStrategy = "pbqp"
			case "size config":
				badOptions.OptSize = true
			}
			badOptions.ProfileSelection = &request
			fresh := parse()
			before, err := hir.BuildProfileCompatibility(fresh, c)
			if err != nil {
				t.Fatal(err)
			}
			d, err := applyPipelineProfile(fresh, badOptions)
			if err == nil || d != nil {
				t.Fatalf("accepted invalid request: %v %v", d, err)
			}
			after, err := hir.BuildProfileCompatibility(fresh, c)
			if err != nil || !reflect.DeepEqual(before, after) || fresh.PointerWalkPolicy != nil || fresh.LoopLineage != nil {
				t.Fatalf("rejection mutated HIR: %v", err)
			}
		})
	}
}
