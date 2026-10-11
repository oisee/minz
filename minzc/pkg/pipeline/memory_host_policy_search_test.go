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
	for _, kind := range []string{"first", "second", "swapped", "unknown", "bare", "source", "unit", "oversized", "missing-first", "missing-second", "goal", "sites"} {
		t.Run(kind, func(t *testing.T) {
			x := input
			opts := o
			switch kind {
			case "missing-first":
				x.Legacy = nil
			case "missing-second":
				x.PreserveSpeed = nil
			case "goal":
				opts.Goal = "other"
			case "sites":
				opts.MaxSites = 1
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
			e, err := MeasureMemoryHostArenaAppendPolicySearch(func() (*hir.Module, error) { calls++; return fresh() }, w, x, opts)
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

func TestMemoryHostPolicySearchOwnedInputsAndSize(t *testing.T) {
	input, w, fresh := policySearchFixture(t)
	calls := 0
	e, err := MeasureMemoryHostArenaAppendPolicySearch(func() (*hir.Module, error) {
		calls++
		if calls == 1 {
			input.Source[0] ^= 1
			input.Legacy[0] = '!'
			input.PreserveSpeed[0] = '!'
			w.Calls[0].Arguments[0] = 65535
			w.Regions["nodes"].Bytes[0] = 99
		}
		return fresh()
	}, w, input, MemoryHostPolicySearchOptions{Goal: "size", MaxSites: 2})
	if err != nil || e == nil || calls != 48 {
		t.Fatal("caller input mutation affected owned execution", calls, err)
	}
	want := MemoryHostPolicySearchChoice{}
	best := len(e.Selections[0].Candidates[0].Evidence.Baseline.Target.Binary)
	for policy, s := range e.Selections {
		for mask, c := range s.Candidates {
			if len(c.Evidence.Baseline.Target.Binary) < best {
				best = len(c.Evidence.Baseline.Target.Binary)
				want = MemoryHostPolicySearchChoice{Policy: mir2.UnsignedPairComparePolicy(policy), Mask: mask}
			}
		}
	}
	if e.Selected != want {
		t.Fatal("actual Size rank", e.Selected, want)
	}
	for _, kind := range []string{"error", "failure", "choices", "policy"} {
		t.Run(kind, func(t *testing.T) {
			b, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			var forged MemoryHostPolicySearchEvidence
			if err = json.Unmarshal(b, &forged); err != nil {
				t.Fatal(err)
			}
			c := &forged.Selections[1].Candidates[3]
			switch kind {
			case "error":
				c.Evidence.Baseline.Target.Invocations[0].Error = "fabricated failure"
			case "failure":
				c.Evidence.Baseline.Target.Invocations[0].Failure = &hir.MemoryHostAccessError{Kind: "undeclared", Address: 0xc000}
			case "choices":
				c.Choices.ArenaPeel = false
			case "policy":
				c.Evidence.CompileOptions.UnsignedPairComparePolicy = 0
			}
			if validateMemoryHostPolicySearch(&forged) == nil {
				t.Fatal("forged candidate accepted", kind)
			}
		})
	}
	// Synthetic cost-only control: no native evidence claim. Stable earlier ties.
	for _, goal := range []string{"speed", "size"} {
		for _, s := range e.Selections {
			for _, c := range s.Candidates {
				c.Evidence.Baseline.Target.TotalTStates = 100
				c.Evidence.Baseline.Target.Binary = make([]byte, 10)
			}
		}
		e.Options.Goal = goal
		for _, s := range e.Selections {
			s.Options.Goal = goal
		}
		if err := validateMemoryHostPolicySearch(e); err != nil {
			t.Fatal(err)
		}
		if e.Selected != (MemoryHostPolicySearchChoice{}) {
			t.Fatal("ties must retain first Legacy original")
		}
		e.Selections[1].Candidates[2].Evidence.Baseline.Target.TotalTStates = 99
		e.Selections[1].Candidates[2].Evidence.Baseline.Target.Binary = make([]byte, 9)
		if err := validateMemoryHostPolicySearch(e); err != nil {
			t.Fatal(err)
		}
		if e.Selected != (MemoryHostPolicySearchChoice{Policy: 1, Mask: 2}) {
			t.Fatal("cross-policy strict improvement")
		}
	}
}
