package hir_test

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"strings"
	"testing"
)

func TestSelectLoopVariantsWithModels(t *testing.T) {
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
	source := ""
	for _, name := range []string{"short_sum", "long_sum", "uncovered_sum"} {
		source += strings.ReplaceAll(sum, "NAME", name)
	}
	m, err := nanz.Parse(source, "multi.nanz")
	if err != nil {
		t.Fatal(err)
	}
	short, long := int64(1), int64(40)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "z80-pbqp-default-jrs/v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"input": make([]byte, 40)}, Calls: []hir.ProfileInvocation{
		{Function: "short_sum", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &short}}},
		{Function: "long_sum", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &long}}},
		{Function: "uncovered_sum", Arguments: []hir.ProfileArgument{{Resource: "input"}, {Value: &long}}},
	}}
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	c, err := hir.BuildWorkloadProfileContext(m, w)
	if err != nil {
		t.Fatal(err)
	}
	model := hir.LoopCostModel{CompilerID: c.CompilerID, SemanticsID: c.SemanticsID, TargetID: c.TargetID, HIRSHA256: run.Profile.Compatibility.HIRSHA256, EvidenceID: "test-fixture", OriginalBytes: 128, PointerBytes: 177, OriginalEntry: 69, OriginalIteration: 141, FastEntry: 189, FastIteration: 89}
	models := []hir.LoopCostModel{model, model}
	models[0].Function, models[1].Function = "short_sum", "long_sum"
	selectModels := func(p hir.LoopProfile, models []hir.LoopCostModel, size bool) ([]hir.ProfileDecision, error) {
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		return hir.SelectLoopVariantsWithModels(bytes.NewReader(data), m, c, models, size)
	}
	functions := map[string]string{}
	for _, site := range run.Profile.Sites.Sites {
		functions[site.ID] = site.Function
	}
	decisions, err := selectModels(run.Profile, models, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 3 {
		t.Fatal(decisions)
	}
	seen := map[string]bool{}
	for _, d := range decisions {
		if seen[d.SiteID] || d.Compatibility == nil {
			t.Fatal(d)
		}
		seen[d.SiteID] = true
		switch functions[d.SiteID] {
		case "short_sum":
			if d.Variant != "original" || d.Model != models[0] || d.OriginalTStates != "210" || d.PointerTStates != "278" {
				t.Fatal(d)
			}
		case "long_sum":
			if d.Variant != "pointer-walk" || d.Model != models[1] || d.OriginalTStates != "5709" || d.PointerTStates != "3749" {
				t.Fatal(d)
			}
		case "uncovered_sum":
			if d.Variant != "original" || d.Model != (hir.LoopCostModel{}) || d.OriginalTStates != "" || d.PointerTStates != "" {
				t.Fatal(d)
			}
		default:
			t.Fatal(d)
		}
	}
	reversed, err := selectModels(run.Profile, []hir.LoopCostModel{models[1], models[0]}, false)
	if err != nil || !reflect.DeepEqual(decisions, reversed) {
		t.Fatalf("order changed decisions: %v %v", reversed, err)
	}
	sized, err := selectModels(run.Profile, models, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range sized {
		if d.Variant != "original" {
			t.Fatal(d)
		}
	}
	noGuards := run.Profile
	noGuards.Guards = []hir.GuardCounts{}
	unguarded, err := selectModels(noGuards, models, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range unguarded {
		if d.Variant != "original" || d.PointerTStates != "" {
			t.Fatal(d)
		}
	}
	data, _ := json.Marshal(run.Profile)
	single, err := hir.SelectLoopVariants(bytes.NewReader(data), m, c, models[0], false)
	if err != nil || len(single) != 3 {
		t.Fatalf("single: %v %v", single, err)
	}
	for _, d := range single {
		if d.Model != models[0] {
			t.Fatal(d)
		}
	}
	before, err := hir.BuildProfileCompatibility(m, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"empty", "duplicate", "function", "unknown", "hir", "compiler", "semantics", "target", "evidence", "bytes"} {
		t.Run(field, func(t *testing.T) {
			bad := append([]hir.LoopCostModel{}, models...)
			switch field {
			case "empty":
				bad = nil
			case "duplicate":
				bad[1] = bad[0]
			case "function":
				bad[1].Function = ""
			case "unknown":
				bad[1].Function = "missing"
			case "hir":
				bad[1].HIRSHA256 = "stale"
			case "compiler":
				bad[1].CompilerID = "stale"
			case "semantics":
				bad[1].SemanticsID = "stale"
			case "target":
				bad[1].TargetID = "stale"
			case "evidence":
				bad[1].EvidenceID = ""
			case "bytes":
				bad[1].PointerBytes = 0
			}
			d, err := selectModels(run.Profile, bad, false)
			if err == nil || d != nil {
				t.Fatalf("accepted invalid models: %v %v", d, err)
			}
			after, err := hir.BuildProfileCompatibility(m, c)
			if err != nil || !reflect.DeepEqual(before, after) || m.LoopLineage != nil || m.PointerWalkPolicy != nil {
				t.Fatalf("rejection mutated HIR: %v", err)
			}
		})
	}
	joint := append([]hir.LoopCostModel(nil), models...)
	joint[0].FastEntry = 0
	joint[0].FastIteration = 0
	if d, err := selectModels(run.Profile, joint, false); err == nil || d != nil {
		t.Fatal("independent models authorized simultaneous rewrites")
	}
	stale := run.Profile
	stale.Compatibility = nil
	if d, err := selectModels(stale, models, false); err == nil || d != nil {
		t.Fatal("accepted stale profile")
	}
	if _, err := hir.SelectLoopVariantsWithModels(bytes.NewReader(append(data, data...)), m, c, models, false); err == nil {
		t.Fatal("accepted trailing JSON")
	}
}
