package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

func weightedBothValidationFixture(t *testing.T) (*hir.Module, *hir.Module, string) {
	t.Helper()
	source := graceMeasureParse(t, weightedPrototypeSource)
	ids, err := hir.EligibleWeightedCompositionSites(source)
	if err != nil || len(ids) != 1 {
		t.Fatal(ids, err)
	}
	actual := graceMeasureParse(t, weightedPrototypeSource)
	if err := hir.SeedLoopLineage(actual); err != nil {
		t.Fatal(err)
	}
	actual.GraceRulePolicy = map[string]bool{"hir-invariant-base": true}
	if _, err := hir.RunGrace(actual, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := hir.ValidateWeightedComposition(source, actual, ids[0], true, true); err != nil {
		t.Fatal(err)
	}
	return source, actual, ids[0]
}

func TestWeightedCompositionValidatorRefusesForgedShapes(t *testing.T) {
	for _, name := range []string{"mutate_store", "return_metadata", "globals", "guard_comparison", "guard_limit", "zero_start", "counter_length", "read_order", "stride", "read_type", "sum_target", "final_index", "fallback_weight", "initializer", "binding_collision", "marker_missing", "marker_forged", "duplicate_use", "source_hoisted"} {
		t.Run(name, func(t *testing.T) {
			source, actual, id := weightedBothValidationFixture(t)
			f := actual.FuncByName("weighted")
			guard := f.Body.Body[3].(*hir.IfStmt)
			each := guard.Then.Body[0].(*hir.ForEachStmt)
			switch name {
			case "mutate_store":
				each.MutateInPlace = true
			case "return_metadata":
				f.RetTys = []mir2.Ty{mir2.TyU8}
			case "globals":
				actual.Globals = append(actual.Globals, mir2.Global{Name: "new_global", Ty: mir2.TyU8})
			case "guard_comparison":
				guard.Cond.(*hir.BinExpr).Op = "<"
			case "guard_limit":
				guard.Cond.(*hir.BinExpr).R.(*hir.BinExpr).L = hir.U16(65534)
			case "zero_start":
				each.Start = hir.U8(1)
			case "counter_length":
				each.Len = hir.U8(255)
			case "read_order":
				each.Body.Body[0], each.Body.Body[1] = each.Body.Body[1], each.Body.Body[0]
			case "stride":
				each.ElemStride = 2
			case "read_type":
				each.Body.Body[1].(*hir.AssignStmt).Val.(*hir.BinExpr).R.(*hir.CastExpr).X.(*hir.LoadExpr).Ty = mir2.TyU16
			case "sum_target":
				each.Body.Body[1].(*hir.AssignStmt).Target = hir.Var("a", mir2.TyU16)
			case "final_index":
				guard.Then.Body[1].(*hir.AssignStmt).Val = hir.U8(0)
			case "fallback_weight":
				guard.Else.Body[0].(*hir.WhileStmt).Body.Body[0].(*hir.AssignStmt).Val.(*hir.BinExpr).R = hir.U16(0)
			case "initializer":
				f.Body.Body[2].(*hir.VarDeclStmt).Init = hir.Add(hir.Var("a", mir2.TyU16), hir.U16(1), mir2.TyU16)
			case "binding_collision":
				f.Body.Body[2].(*hir.VarDeclStmt).Name = "a"
			case "marker_missing":
				l := actual.LoopLineage[each]
				l.PreviousRule = ""
				actual.LoopLineage[each] = l
			case "marker_forged":
				l := actual.LoopLineage[each]
				l.PreviousRule = "other"
				actual.LoopLineage[each] = l
			case "duplicate_use":
				f.Body.Body[4].(*hir.ReturnStmt).Val = hir.Var(f.Body.Body[2].(*hir.VarDeclStmt).Name, mir2.TyU16)
			case "source_hoisted":
				source = graceMeasureParse(t, weightedHoistedSource)
			}
			if _, err := hir.ValidateWeightedComposition(source, actual, id, true, true); err == nil {
				t.Fatal("accepted forged weighted composition")
			}
		})
	}
}

func TestWeightedV4FreshFactoryAndObservationRefusals(t *testing.T) {
	factory := func() (*hir.Module, error) { return graceMeasureParse(t, weightedPrototypeSource), nil }
	n := int64(1)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Resources: map[string][]byte{"data": {7}}, Calls: []hir.ProfileInvocation{{Function: "weighted", Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &n}, {Value: &n}, {Value: &n}, {Value: &n}}}}}
	source, _ := factory()
	run, err := hir.RunLoopProfile(source, w)
	if err != nil {
		t.Fatal(err)
	}
	observations, _ := json.Marshal(run.Profile)
	for _, name := range []string{"shared", "hoisted_prefix", "prior_lineage", "forged_zero", "cap"} {
		t.Run(name, func(t *testing.T) {
			f := factory
			p := observations
			cap := 4
			switch name {
			case "shared":
				f = func() (*hir.Module, error) { return source, nil }
			case "hoisted_prefix":
				f = func() (*hir.Module, error) { return graceMeasureParse(t, weightedHoistedSource), nil }
			case "prior_lineage":
				f = func() (*hir.Module, error) { m, _ := factory(); hir.SeedLoopLineage(m); return m, nil }
			case "forged_zero":
				p = []byte(strings.Replace(string(p), `"fast_zero_entries":0`, `"fast_zero_entries":1`, 1))
			case "cap":
				cap = 1
			}
			if result, err := MeasureWeightedJointSourceSiteVariantSets(f, p, w, Options{AssertMode: "none", UseHIRGrace: true}, cap); err == nil || result != nil {
				t.Fatal("accepted unsafe weighted request")
			}
		})
	}
}

func TestWeightedV4ChoiceDimensionsBoundary(t *testing.T) {
	for _, functions := range []int{2, 3} {
		source := ""
		w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"data": {7}}}
		n := int64(0)
		a := int64(65535)
		b := int64(2)
		for i := 0; i < functions; i++ {
			name := fmt.Sprintf("weighted%d", i)
			source += strings.Replace(weightedPrototypeSource, "fun weighted(", "fun "+name+"(", 1) + "\n"
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &n}, {Value: &a}, {Value: &b}, {Value: &a}}})
		}
		calls := 0
		factory := func() (*hir.Module, error) { calls++; return nanz.Parse(source, "weighted-boundary.nanz") }
		m, _ := factory()
		raw, err := hir.RunLoopProfile(m, w)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(raw.Profile)
		calls = 0
		result, err := MeasureWeightedJointSourceSiteVariantSets(factory, data, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if functions == 3 {
			if err == nil || result != nil || calls != 1 {
				t.Fatal("weighted dimensions cap not refused early", err, calls)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Evidence.Candidates) != 32 || calls != 33 || len(result.Evidence.EligibleWeightedOverlapSites) != 2 {
			t.Fatal("weighted32candidate/33fresh boundary", calls)
		}
		for _, c := range result.Evidence.Candidates {
			if len(c.WeightedProof) != 2 || len(c.FinalWeightedProof) != 2 {
				t.Fatal("missing weighted boundary proofs")
			}
			for _, inv := range c.Run.Invocations {
				if inv.Results[0].I != 65535 {
					t.Fatal("weighted zero-trip seed oracle")
				}
			}
		}
	}
}
