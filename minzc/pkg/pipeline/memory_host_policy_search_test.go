package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"runtime"
	"strings"
	"testing"
)

func policySearchFixture(t *testing.T) (MemoryHostPolicySearchInputs, hir.MemoryHostWorkload, func() (*hir.Module, error)) {
	t.Helper()
	source, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	w := arenaAppendSmallWorkload(t)
	fresh := func() (*hir.Module, error) {
		m, _ := memoryHostParserFixture(t)
		m.Target = hir.TargetCPM
		return m, nil
	}
	input := MemoryHostPolicySearchInputs{Source: source, SourceName: "../../../examples/nanz/self_lanz_parser.nanz"}
	for policy := 0; policy < 2; policy++ {
		e, err := RunZ80MemoryHostLoopProfileWithOptions(fresh, w, MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairComparePolicy(policy)})
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if policy == 0 {
			input.Legacy = b
		} else {
			input.PreserveSpeed = b
		}
	}
	return input, w, fresh
}
func TestMemoryHostPolicySearchPreflightAndExactOriginal(t *testing.T) {
	input, w, fresh := policySearchFixture(t)
	o := MemoryHostPolicySearchOptions{Goal: "speed", MaxSites: 2}
	for _, kind := range []string{"first", "second", "swapped", "unknown", "bare", "source", "unit", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			x := input
			switch kind {
			case "first":
				x.Legacy = []byte("{")
			case "second":
				x.PreserveSpeed = []byte("{")
			case "swapped":
				x.Legacy, x.PreserveSpeed = x.PreserveSpeed, x.Legacy
			case "source":
				x.Source = append(append([]byte{}, x.Source...), '\n')
			case "unit":
				x.SourceName = "other.nanz"
			case "oversized":
				x.PreserveSpeed = make([]byte, MaxMemoryHostPolicyAssociationBytes+1)
			default:
				var a MemoryHostLoopProfileEvidenceV2
				if err := json.Unmarshal(x.PreserveSpeed, &a); err != nil {
					t.Fatal(err)
				}
				if kind == "unknown" {
					a.CompileOptions.UnsignedPairComparePolicy = 255
					x.PreserveSpeed, _ = json.Marshal(a)
				} else {
					x.PreserveSpeed, _ = json.Marshal(a.Profile)
				}
			}
			calls := 0
			e, err := MeasureMemoryHostArenaAppendPolicySearch(func() (*hir.Module, error) { calls++; return fresh() }, w, x, o)
			if e != nil || err == nil || calls != 0 {
				t.Fatal("preflight reached factory", kind, calls, err)
			}
		})
	}
	for _, kind := range []string{"unrelated-body", "assert", "sandbox", "warnings", "stats", "origin"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			factory := func() (*hir.Module, error) {
				calls++
				m, err := fresh()
				if err != nil {
					return nil, err
				}
				before := m.SourceUnits[input.SourceName]
				switch kind {
				case "unrelated-body":
					f := m.FuncByName("is_space")
					ret := f.Body.Body[len(f.Body.Body)-1].(*hir.ReturnStmt)
					ret.Val.(*hir.IntLitExpr).Val = 1
					if e := hir.Verify(m); e != nil {
						t.Fatal("mutated source must remain valid", e)
					}
				case "assert":
					m.Asserts = append(m.Asserts, hir.Assert{FuncName: "cur", Expected: 123})
				case "sandbox":
					m.Sandboxes = append(m.Sandboxes, hir.Sandbox{Name: "injected"})
				case "warnings":
					m.Warnings = append(m.Warnings, "injected")
				case "stats":
					m.AssertStats = &hir.AssertStats{Executed: 1}
				case "origin":
					s := m.FuncByName("intern_region").Body.Body[5]
					loc := m.SourceLocations[s]
					loc.ByteOffset++
					m.SourceLocations[s] = loc
				}
				if before != m.SourceUnits[input.SourceName] {
					t.Fatal("negative changed declared SHA")
				}
				return m, nil
			}
			e, err := MeasureMemoryHostArenaAppendPolicySearch(factory, w, input, o)
			if e != nil || err == nil || calls != 1 || !strings.Contains(err.Error(), "independently parsed original") {
				t.Fatal("modified original", kind, calls, err)
			}
		})
	}
}
func TestMemoryHostPolicySearchObservationIdentity(t *testing.T) {
	a := []hir.MemoryHostObservation{{Error: "first denial", Steps: 1, Failure: &hir.MemoryHostAccessError{Kind: "undeclared", Address: 0x8000}}}
	b := append([]hir.MemoryHostObservation{}, a...)
	b[0].Steps = 99
	if err := policySearchSameObservations(a, b); err != nil {
		t.Fatal(err)
	}
	b[0].Error = "other denial"
	if policySearchSameObservations(a, b) == nil {
		t.Fatal("error text laundered")
	}
	b[0] = a[0]
	b[0].Failure = &hir.MemoryHostAccessError{Kind: "read-only", Address: 0x8000}
	if policySearchSameObservations(a, b) == nil {
		t.Fatal("failure laundered")
	}
}
func TestMemoryHostPolicySearchLifecycle(t *testing.T) {
	input, w, fresh := policySearchFixture(t)
	o := MemoryHostPolicySearchOptions{Goal: "speed", MaxSites: 2}
	calls := 0
	result, err := MeasureMemoryHostArenaAppendPolicySearch(func() (*hir.Module, error) { calls++; runtime.GC(); return fresh() }, w, input, o)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 48 || len(result.Candidates) != 8 {
		t.Fatal("lifecycle", calls)
	}
	want := MemoryHostPolicySearchChoice{}
	best := result.Selections[0].Candidates[0].Evidence.Baseline.Target.TotalTStates
	for i, c := range result.Candidates {
		if c.Choice.Policy != mir2.UnsignedPairComparePolicy(i/4) || c.Choice.Mask != i%4 {
			t.Fatal("stable order")
		}
		v := result.Selections[i/4].Candidates[i%4].Evidence.Baseline.Target.TotalTStates
		if v < best {
			best = v
			want = c.Choice
		}
	}
	if result.Selected != want {
		t.Fatal("actual primary rank", result.Selected, want)
	}
	for _, phase := range []int{25, 48} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			calls := 0
			var first *hir.Module
			factory := func() (*hir.Module, error) {
				m, err := fresh()
				calls++
				if calls == 1 {
					first = m
				}
				if calls == phase {
					s := first.FuncByName("intern_region").Body.Body[5]
					loc := first.SourceLocations[s]
					loc.ByteOffset++
					first.SourceLocations[s] = loc
				}
				return m, err
			}
			e, err := MeasureMemoryHostArenaAppendPolicySearch(factory, w, input, o)
			if e != nil || err == nil || calls != phase {
				t.Fatal("cross-policy mutation", calls, err)
			}
		})
	}
	calls = 0
	var first *hir.Module
	e, err := MeasureMemoryHostArenaAppendPolicySearch(func() (*hir.Module, error) {
		m, err := fresh()
		calls++
		if calls == 1 {
			first = m
		}
		if calls == 25 {
			m.FuncByName("region_match").Body = first.FuncByName("region_match").Body
		}
		return m, err
	}, w, input, o)
	if e != nil || err == nil || calls != 25 {
		t.Fatal("cross-policy sharing", calls, err)
	}
}
