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

func forwardCopyProfileFixture(t *testing.T) (func() (*hir.Module, error), hir.ProfileWorkload, []byte) {
	t.Helper()
	factory := func() (*hir.Module, error) { return nanz.Parse(forwardCopySource, "copy-profile.nanz") }
	n := int64(1)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"data": {7, 9, 0, 0}}, Calls: []hir.ProfileInvocation{{Function: "copy_forward", Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 2}, {Resource: "data"}, {Value: &n}}}}}
	m, _ := factory()
	r, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(r.Profile)
	return factory, w, p
}

func TestForwardCopySelectorRefusals(t *testing.T) {
	factory, w, p := forwardCopyProfileFixture(t)
	for _, name := range []string{"nil_factory", "shared", "shared_ast", "prior_policy", "mutated_prior_policy", "source_changed", "forged_count", "bad_cap", "backend", "skip"} {
		t.Run(name, func(t *testing.T) {
			f := factory
			observations := p
			cap := 4
			options := Options{AssertMode: "none", UseHIRGrace: true}
			shared, _ := factory()
			switch name {
			case "nil_factory":
				f = nil
			case "shared":
				f = func() (*hir.Module, error) { return shared, nil }
			case "shared_ast":
				f = func() (*hir.Module, error) { m, _ := factory(); m.Funcs = shared.Funcs; return m, nil }
			case "prior_policy":
				f = func() (*hir.Module, error) { m, _ := factory(); m.ForwardCopyPolicy = map[string]bool{}; return m, nil }
			case "mutated_prior_policy":
				calls := 0
				var first *hir.Module
				f = func() (*hir.Module, error) {
					m, _ := factory()
					calls++
					if calls == 1 {
						first = m
					} else {
						first.ForwardCopyPolicy = map[string]bool{}
					}
					return m, nil
				}
			case "source_changed":
				calls := 0
				f = func() (*hir.Module, error) { m, _ := factory(); calls++; m.Name = fmt.Sprint(calls); return m, nil }
			case "forged_count":
				observations = []byte(strings.Replace(string(p), `"entries":1`, `"entries":2`, 1))
			case "bad_cap":
				cap = 0
			case "backend":
				options.Backend = "ez80"
			case "skip":
				options.SkipZ80Emission = true
			}
			result, err := MeasureForwardCopySiteVariantSets(f, observations, w, options, cap)
			if err == nil || result != nil {
				t.Fatal("acceptedunsafe copyrequest")
			}
		})
	}
	// Every older factory API must reject the new sticky policy before using it.
	sticky := func() (*hir.Module, error) { m, _ := factory(); m.ForwardCopyPolicy = map[string]bool{}; return m, nil }
	if r, err := MeasureGraceVariantSets(sticky, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4); err == nil || r != nil {
		t.Fatal("legacyjoint acceptedcopypolicy")
	}
	if r, err := MeasureJointSourceSiteVariantSets(sticky, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4); err == nil || r != nil {
		t.Fatal("legacysites acceptedcopypolicy")
	}
}

func TestForwardCopyIndependentStructuralRefusals(t *testing.T) {
	for _, name := range []string{"guard", "guard_base", "length", "mutate_store", "store_order", "load_type", "step", "final_index", "fallback", "marker", "fresh_collision", "prefix"} {
		t.Run(name, func(t *testing.T) {
			factory, w, _ := forwardCopyProfileFixture(t)
			source, _ := factory()
			actual, _ := factory()
			key, err := hir.BuildWorkloadProfileContext(source, w)
			if err != nil {
				t.Fatal(err)
			}
			compat, err := hir.BuildProfileCompatibility(source, key)
			if err != nil {
				t.Fatal(err)
			}
			ids, _ := hir.EligibleForwardCopySites(source)
			if err := hir.ApplyForwardCopySiteChoices(actual, compat, map[string]bool{ids[0]: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := hir.ValidateForwardCopyProof(source, actual, ids[0], true); err != nil {
				t.Fatal(err)
			}
			f := actual.FuncByName("copy_forward")
			guard := f.Body.Body[1].(*hir.IfStmt)
			each := guard.Then.Body[1].(*hir.ForEachStmt)
			switch name {
			case "guard":
				guard.Cond.(*hir.BinExpr).Op = "||"
			case "guard_base":
				guard.Cond.(*hir.BinExpr).L.(*hir.BinExpr).L = hir.Cast(hir.Var("dst", mir2.TyPtr), mir2.TyU16)
			case "length":
				each.Len = hir.U8(255)
			case "mutate_store":
				each.MutateInPlace = true
			case "store_order":
				each.Body.Body[0], each.Body.Body[1] = each.Body.Body[1], each.Body.Body[0]
			case "load_type":
				each.Body.Body[0].(*hir.StoreStmt).Val.(*hir.LoadExpr).Ty = mir2.TyU16
			case "step":
				each.Body.Body[1].(*hir.AssignStmt).Val.(*hir.BinExpr).R = hir.U16(2)
			case "final_index":
				guard.Then.Body[2].(*hir.AssignStmt).Val = hir.U8(0)
			case "fallback":
				guard.Else.Body[0].(*hir.WhileStmt).Body.Body[0].(*hir.AssignStmt).Val = hir.U8(0)
			case "marker":
				l := actual.LoopLineage[each]
				l.Rule = "hir-guarded-pointer-walk"
				actual.LoopLineage[each] = l
			case "fresh_collision":
				each.Var = "src"
			case "prefix":
				guard.Then.Body[0].(*hir.VarDeclStmt).Init = hir.Var("src", mir2.TyPtr)
			}
			if _, err := hir.ValidateForwardCopyProof(source, actual, ids[0], true); err == nil {
				t.Fatal("forgedcopyshapeaccepted")
			}
		})
	}
}

func TestForwardCopySelectorSitesBoundaryAndUnobserved(t *testing.T) {
	for _, count := range []int{4, 5} {
		source := ""
		n := int64(0)
		w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"data": {7, 0}}}
		for i := 0; i < count; i++ {
			name := fmt.Sprintf("copy%d", i)
			source += strings.Replace(forwardCopySource, "fun copy_forward(", "fun "+name+"(", 1) + "\n"
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 1}, {Resource: "data"}, {Value: &n}}})
		}
		calls := 0
		factory := func() (*hir.Module, error) { calls++; return nanz.Parse(source, "copy-boundary.nanz") }
		m, _ := factory()
		raw, err := hir.RunLoopProfile(m, w)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := json.Marshal(raw.Profile)
		calls = 0
		result, err := MeasureForwardCopySiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if count == 5 {
			if err == nil || result != nil || calls != 1 {
				t.Fatal("copycap notrefusedearly")
			}
			continue
		}
		if err != nil || len(result.Evidence.Candidates) != 16 || calls != 17 {
			t.Fatal("copy16candidate17freshboundary", err, calls)
		}
		for _, candidate := range result.Evidence.Candidates {
			if len(candidate.Proofs) != 4 || len(candidate.FinalProofs) != 4 {
				t.Fatal("copymissingproof")
			}
			for _, inv := range candidate.Run.Invocations {
				if inv.Results[0].I != 0 {
					t.Fatal("zerotriporacle")
				}
			}
		}
		w.Calls = w.Calls[:1]
		m, _ = factory()
		raw, err = hir.RunLoopProfile(m, w)
		if err != nil {
			t.Fatal(err)
		}
		p, _ = json.Marshal(raw.Profile)
		result, err = MeasureForwardCopySiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if err != nil || len(result.Evidence.Candidates) != 2 {
			t.Fatal("unobservedcopiesnotexcluded", err)
		}
	}
}
