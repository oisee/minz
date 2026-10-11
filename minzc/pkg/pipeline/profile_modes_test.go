package pipeline

import (
	"errors"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"testing"
)

func modeFixture() (func() (*hir.Module, error), hir.ProfileWorkload) {
	src := `fun arithmetic(n:u8)->u8{return n+7}`
	n := int64(250)
	return func() (*hir.Module, error) { return nanz.Parse(src, "mode.nanz") }, hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}, Calls: []hir.ProfileInvocation{{Function: "arithmetic", Arguments: []hir.ProfileArgument{{Value: &n}}}}}
}
func TestMeasuredCompilerModesAdmission(t *testing.T) {
	for _, kind := range []string{"shared", "mutated-source", "changed-ABI", "sticky-policy", "late-failure"} {
		t.Run(kind, func(t *testing.T) {
			fresh, w := modeFixture()
			shared, _ := fresh()
			calls := 0
			var prior *hir.Module
			factory := func() (*hir.Module, error) {
				calls++
				if kind == "late-failure" && calls == 6 {
					return nil, errors.New("after candidate measurements")
				}
				m, e := fresh()
				if e != nil {
					return nil, e
				}
				if kind == "shared" {
					return shared, nil
				}
				if kind == "mutated-source" && prior != nil {
					prior.Name = "mutated"
				}
				if kind == "changed-ABI" && calls == 2 {
					m.FuncByName("arithmetic").Params[0].Ty = nil
				}
				if kind == "sticky-policy" {
					m.GraceRulePolicy = map[string]bool{}
				}
				prior = m
				return m, nil
			}
			r, e := MeasureCompilerModeVariants(factory, w, "speed", false)
			if e == nil || r != nil {
				t.Fatal("unsafe partial result admitted", kind)
			}
		})
	}
}
func TestMeasuredCompilerModesCompleteAndTie(t *testing.T) {
	f, w := modeFixture()
	for _, goal := range []string{"speed", "size"} {
		r, e := MeasureCompilerModeVariants(f, w, goal, false)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Evidence.Candidates) != 4 || r.Evidence.Selected != 0 {
			t.Fatal("complete fixed ordering/tie")
		}
		if !reflect.DeepEqual(r.Binary, r.Evidence.Candidates[0].Binary) || r.Steps.Assembly != r.Evidence.Candidates[0].Assembly {
			t.Fatal("winner is not measured artifact")
		}
		for i, c := range r.Evidence.Candidates {
			if c.Options.ContractOpt != (i&1 != 0) || c.Options.UseGrace != (i&2 != 0) || c.Run.Invocations[0].Results[0].I != 1 {
				t.Fatal("mode/oracle mismatch")
			}
		}
	}
}

func TestMeasuredModesRejectsLateCompiledTypeMutation(t *testing.T) {
	// Separate mutable range objects share the same initial semantic identity.
	// The factory retains the candidate type and mutates it at the final sentinel.
	_, w := modeFixture()
	fresh := func() (*hir.Module, error) {
		return nanz.Parse(`fun arithmetic(n:u8<0..255>)->u8{return n+7}`, "mode.nanz")
	}
	calls := 0
	var candidateType *mir2.RangedTy
	factory := func() (*hir.Module, error) {
		calls++
		m, e := fresh()
		if e != nil {
			return nil, e
		}
		if calls == 2 {
			candidateType = m.FuncByName("arithmetic").Params[0].Ty.(*mir2.RangedTy)
		}
		if calls == 6 && candidateType != nil {
			candidateType.Hi = 8
		}
		return m, nil
	}
	r, e := MeasureCompilerModeVariants(factory, w, "speed", false)
	if e == nil || r != nil {
		t.Fatal("late compiled semantic mutation admitted")
	}
}
