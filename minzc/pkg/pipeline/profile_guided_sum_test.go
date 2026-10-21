package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestProfileGuidedSumVMZ80(t *testing.T) {
	source, e := os.ReadFile("../../../examples/nanz/01_sum_array.nanz")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(source), "examples/nanz/01_sum_array.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	resource := make([]byte, 255)
	memory := map[uint16]byte{}
	for i := range resource {
		resource[i] = byte(i + 1)
		memory[uint16(0x4000+i)] = resource[i]
	}
	lengths := []int64{0, 1, 5, 16, 64, 255}
	workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"array": resource}, Calls: []hir.ProfileInvocation{}}
	for _, n := range lengths {
		workload.Calls = append(workload.Calls, hir.ProfileInvocation{Function: "sum_array", Arguments: []hir.ProfileArgument{{Resource: "array"}, literal(n)}})
	}
	input := parse()
	profileRun, e := hir.RunLoopProfile(input, workload)
	if e != nil {
		t.Fatal(e)
	}
	context, e := hir.BuildWorkloadProfileContext(input, workload)
	if e != nil {
		t.Fatal(e)
	}
	profileJSON, _ := json.MarshalIndent(profileRun.Profile, "", "  ")
	// The counted-pointer pass improves the ordinary loop as well as copy.
	// These coefficients were remeasured over all 1022 length/address cases;
	// pair-move codegen also removes 27 module bytes and 13T per fallback
	// iteration. Guarded fast-path costs are unchanged.
	model := hir.LoopCostModel{CompilerID: context.CompilerID, SemanticsID: context.SemanticsID, Function: "sum_array", HIRSHA256: profileRun.Profile.Compatibility.HIRSHA256, TargetID: context.TargetID, EvidenceID: "sum-array-z80-pair-moves-v9", OriginalBytes: 171, PointerBytes: 250, OriginalEntry: 41, OriginalIteration: 54, EqualEntryExtra: 7, FastEntry: 241, FastIteration: 37, FastZeroEntry: 232, FastZeroMeasured: true, FallbackEntry: 258, FallbackIteration: 178, FallbackMeasured: true}
	decisions, e := hir.SelectLoopVariants(bytes.NewReader(profileJSON), input, context, model, false)
	if e != nil {
		t.Fatal(e)
	}
	if len(decisions) != 1 || decisions[0].Variant != "pointer-walk" {
		t.Fatalf("decisions=%+v", decisions)
	}
	// Cost evidence is specific to the current typed module, never silently reused.
	badModel := model
	badModel.HIRSHA256 = "stale"
	if _, e := hir.SelectLoopVariants(bytes.NewReader(profileJSON), input, context, badModel, false); e == nil {
		t.Fatal("stale model accepted")
	}
	badDecision := append([]hir.ProfileDecision{}, decisions...)
	badDecision[0].SiteID = "unknown"
	untouched := parse()
	if e := hir.ApplyProfileDecisions(untouched, badDecision); e == nil || untouched.LoopLineage != nil {
		t.Fatal("invalid decision mutated input")
	}
	// Source edits retaining site positions still expire saved decisions.
	sourceEdited := parse()
	for node, origin := range sourceEdited.SourceLocations {
		origin.SourceSHA256 = fmt.Sprintf("%064x", 1)
		sourceEdited.SourceLocations[node] = origin
	}
	sourceEdited.SourceUnits["examples/nanz/01_sum_array.nanz"] = fmt.Sprintf("%064x", 1)
	if e := hir.ApplyProfileDecisions(sourceEdited, decisions); e == nil {
		t.Fatal("stale source decision accepted")
	}
	pipelineOptions := Options{UseHIRGrace: true, AssertMode: "none"}
	config, e := ProfileCompilationID(pipelineOptions, "default")
	if e != nil {
		t.Fatal(e)
	}
	pipelineModel := model
	pipelineModel.CompilationID = config
	pipelineOptions.ProfileSelection = &ProfileSelection{Observations: profileJSON, Workload: workload, Model: pipelineModel, AssemblerMode: "default"}
	compiled, e := CompileHIRSteps(parse(), pipelineOptions)
	if e != nil {
		t.Fatal(e)
	}
	if len(compiled.ProfileDecisions) != 1 || compiled.ProfileDecisions[0].Variant != "pointer-walk" {
		t.Fatal(compiled.ProfileDecisions)
	}
	wrongOptions := pipelineOptions
	wrongOptions.OptSize = true
	if _, e := CompileHIRSteps(parse(), wrongOptions); e == nil {
		t.Fatal("different compilation settings accepted")
	}
	if _, e := ProfileCompilationID(pipelineOptions, "speed"); e == nil {
		t.Fatal("unmeasured assembler mode accepted")
	}
	for _, n := range []int64{0, 1, 5, 16, 64, 255} {
		measureProfileZ80(t, parse(), "sum_array", pipelineOptions, []int64{0x4000, n}, memory, n*(n+1)/2&255, "")
	}
	output := os.Getenv("MINZ_PGO_ARTIFACT_DIR")
	writeJSON := func(name string, value any) {
		if output == "" {
			return
		}
		if e := os.MkdirAll(output, 0755); e != nil {
			t.Fatal(e)
		}
		data, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(output, name), append(data, '\n'), 0644); e != nil {
			t.Fatal(e)
		}
	}
	writeJSON("workload.json", workload)
	writeJSON("profile.json", profileRun.Profile)
	writeJSON("decision.json", decisions)
	if output != "" {
		os.WriteFile(filepath.Join(output, "sum-array.nanz"), source, 0644)
	}
	totals := map[string]int{}
	rows := []map[string]any{}
	for _, variant := range []string{"original", "pointer-walk", "selected"} {
		for _, n := range lengths {
			h := parse()
			switch variant {
			case "pointer-walk":
				if e := hir.SeedLoopLineage(h); e != nil {
					t.Fatal(e)
				}
				if _, e := hir.RunGrace(h, 0); e != nil {
					t.Fatal(e)
				}
			case "selected":
				if e := hir.ApplyProfileDecisions(h, decisions); e != nil {
					t.Fatal(e)
				}
			}
			lineage := hir.CurrentLoopLineage(h)
			if variant != "original" && (len(lineage) != 2 || lineage[0].Lineage.SiteID != decisions[0].SiteID) {
				t.Fatalf("lineage=%+v", lineage)
			}
			raw := hir.LowerModule(h)
			if variant != "original" && len(raw.FuncByName("sum_array").LoopOrigins) != 2 {
				t.Fatal("raw MIR attribution lost")
			}
			vm := mir2.NewVM(raw)
			vm.EnsureHeap(0x4100)
			vm.WriteHeapBytes(0x4000, resource)
			want := n * (n + 1) / 2 & 255
			result, e := vm.Call("sum_array", []mir2.Value{{I: 0x4000}, {I: n}})
			if e != nil || len(result) != 1 || result[0].I != want {
				t.Fatalf("%s n=%d VM=%v err=%v", variant, n, result, e)
			}
			if !bytes.Equal(vm.ReadHeap(0x4000, 255), resource) {
				t.Fatal("VM changed input memory")
			}
			dir := ""
			if output != "" {
				dir = filepath.Join(output, variant, fmt.Sprintf("n%d", n))
			}
			cycles, size := measureProfileZ80(t, h, "sum_array", Options{AssertMode: "none"}, []int64{0x4000, n}, memory, want, dir)
			expected := 41 + 54*int(n)
			expectedSize := int(model.OriginalBytes)
			if variant != "original" {
				expected = 241 + 37*int(n)
				if n == 0 {
					expected = 232
				}
				expectedSize = int(model.PointerBytes)
			}
			if cycles != expected || size != expectedSize {
				t.Fatalf("cost model disproved: %s n=%d got=%d/%d expected=%d/%d", variant, n, cycles, size, expected, expectedSize)
			}
			totals[variant] += cycles
			rows = append(rows, map[string]any{"variant": variant, "length": n, "result": want, "t_states": cycles, "module_bytes": size, "vm_result": result[0].I, "lineage": lineage})
			t.Logf("%s n=%d result=%d T-states=%d module_bytes=%d", variant, n, want, cycles, size)
		}
	}
	if fmt.Sprint(totals["original"]) != decisions[0].OriginalTStates || fmt.Sprint(totals["selected"]) != decisions[0].PointerTStates {
		t.Fatal("profile cost totals differ from executed cost", totals, decisions)
	}
	writeJSON("measurements.json", rows)
	// Short workload reverses the speed decision; size retains original too.
	short := workload
	short.Calls = short.Calls[:2]
	run, e := hir.RunLoopProfile(parse(), short)
	if e != nil {
		t.Fatal(e)
	}
	c, e := hir.BuildWorkloadProfileContext(parse(), short)
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(run.Profile)
	for _, size := range []bool{false, true} {
		d, e := hir.SelectLoopVariants(bytes.NewReader(data), parse(), c, model, size)
		if e != nil || d[0].Variant != "original" {
			t.Fatalf("short/size decision %v %v", d, e)
		}
		writeJSON(fmt.Sprintf("short-size-%v-decision.json", size), d)
	}
	sizeDecisions, e := hir.SelectLoopVariants(bytes.NewReader(profileJSON), parse(), context, model, true)
	if e != nil || sizeDecisions[0].Variant != "original" {
		t.Fatalf("size=%v %v", sizeDecisions, e)
	}
	writeJSON("size-decision.json", sizeDecisions)
	// Execute both sides of the profitability boundary, including retained original.
	for _, n := range []int64{0, 1, 11, 12, 13} {
		one := workload
		one.Calls = []hir.ProfileInvocation{{Function: "sum_array", Arguments: []hir.ProfileArgument{{Resource: "array"}, literal(n)}}}
		run, e := hir.RunLoopProfile(parse(), one)
		if e != nil {
			t.Fatal(e)
		}
		c, e := hir.BuildWorkloadProfileContext(parse(), one)
		if e != nil {
			t.Fatal(e)
		}
		data, _ := json.Marshal(run.Profile)
		for _, size := range []bool{false, true} {
			d, e := hir.SelectLoopVariants(bytes.NewReader(data), parse(), c, model, size)
			if e != nil {
				t.Fatal(e)
			}
			expectedVariant := "original"
			if n >= 12 && !size {
				expectedVariant = "pointer-walk"
			}
			if d[0].Variant != expectedVariant {
				t.Fatal(d)
			}
			h := parse()
			if e := hir.ApplyProfileDecisions(h, d); e != nil {
				t.Fatal(e)
			}
			cycles, codeBytes := measureProfileZ80(t, h, "sum_array", Options{UseHIRGrace: true, AssertMode: "none"}, []int64{0x4000, n}, memory, n*(n+1)/2&255, "")
			wantCycles := 41 + 54*int(n)
			wantBytes := int(model.OriginalBytes)
			if expectedVariant == "pointer-walk" {
				wantCycles = 241 + 37*int(n)
				if n == 0 {
					wantCycles = 232
				}
				wantBytes = int(model.PointerBytes)
			}
			if cycles != wantCycles || codeBytes != wantBytes {
				t.Fatalf("retained decision not executed correctly: n=%d size=%v cycles=%d bytes=%d", n, size, cycles, codeBytes)
			}
		}
	}
	t.Logf("totals=%v decisions=%+v", totals, decisions)
}
