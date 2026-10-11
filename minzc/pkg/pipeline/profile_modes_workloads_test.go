package pipeline

import (
	"errors"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestMeasuredModeWorkloadsOwnsAllInputsAndTies(t *testing.T) {
	for _, goal := range []string{"speed", "size"} {
		fresh, w := modeFixture()
		v := int64(1)
		second := w
		second.Calls = []hir.ProfileInvocation{{Function: "arithmetic", Arguments: []hir.ProfileArgument{{Value: &v}}}}
		calls := 0
		factory := func() (*hir.Module, error) {
			calls++
			if calls == 1 {
				v = 200
			}
			return fresh()
		}
		r, e := MeasureCompilerModeWorkloads(factory, []hir.ProfileWorkload{w, second}, goal, false)
		if e != nil {
			t.Fatal(e)
		}
		if r.Evidence.Selected != 0 || len(r.Evidence.Runs) != 2 || r.Evidence.Runs[1].Candidates[0].Run.Invocations[0].Results[0].I != 8 {
			t.Fatal("input snapshot/tie changed", r.Evidence)
		}
		for i, total := range r.Evidence.TotalTStates {
			want := r.Evidence.Runs[0].Candidates[i].Run.TotalTStates + r.Evidence.Runs[1].Candidates[i].Run.TotalTStates
			if total != want {
				t.Fatal("bad aggregate", total, want)
			}
		}
	}
}
func TestMeasuredModeWorkloadsRejectsCrossRunDriftAndFailure(t *testing.T) {
	for _, kind := range []string{"source", "ABI", "failure"} {
		t.Run(kind, func(t *testing.T) {
			fresh, w := modeFixture()
			probeCalls := 0
			probe := func() (*hir.Module, error) { probeCalls++; return fresh() }
			if _, e := MeasureCompilerModeVariants(probe, w, "speed", false); e != nil {
				t.Fatal(e)
			}
			calls := 0
			factory := func() (*hir.Module, error) {
				calls++
				if calls > probeCalls {
					switch kind {
					case "failure":
						return nil, errors.New("second frozen workload failure")
					case "source":
						return nanz.Parse(`fun arithmetic(n:u8)->u8{return n+8}`, "mode.nanz")
					case "ABI":
						return nanz.Parse(`fun arithmetic(n:u8)->u16{return u16(n)+7}`, "mode.nanz")
					}
				}
				return fresh()
			}
			r, e := MeasureCompilerModeWorkloads(factory, []hir.ProfileWorkload{w, w}, "speed", false)
			if r != nil || e == nil {
				t.Fatal("partial aggregate admitted", kind)
			}
		})
	}
}
