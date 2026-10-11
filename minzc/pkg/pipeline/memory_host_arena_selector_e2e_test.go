package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func arenaSelectorProfile(t *testing.T, w hir.MemoryHostWorkload) []byte {
	t.Helper()
	source, _ := memoryHostParserFixture(t)
	profile, e := hir.RunMemoryHostLoopProfile(source, w)
	if e != nil {
		t.Fatal(e)
	}
	payload, e := json.Marshal(profile)
	if e != nil {
		t.Fatal(e)
	}
	return payload
}
func TestMemoryHostArenaSelectorOriginal30Objectives(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	serialized := arenaSelectorProfile(t, w)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	for _, goal := range []string{"speed", "size"} {
		t.Run(goal, func(t *testing.T) {
			selected, e := MeasureMemoryHostArenaPeel(factory, w, serialized, MemoryHostArenaSelectorOptions{Goal: goal, MaxSites: 1})
			if e != nil {
				t.Fatal(e)
			}
			if len(selected.Candidates) != 2 {
				t.Fatal("bounded two-candidate contract")
			}
			a, b := selected.Candidates[0].Evidence.Target, selected.Candidates[1].Evidence.Target
			want := 0
			if goal == "speed" && b.TotalTStates < a.TotalTStates || goal == "size" && len(b.Binary) < len(a.Binary) {
				want = 1
			}
			if selected.Selected != want {
				t.Fatal("actual whole-caller objective mismatch")
			}
			if dir := os.Getenv("MINZ_ARENA_SELECTOR_ARTIFACTS"); dir != "" {
				p := filepath.Join(dir, goal)
				if e = os.MkdirAll(p, 0755); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(p, "source-profile.json"), serialized, 0644); e != nil {
					t.Fatal(e)
				}
				payload, e := json.MarshalIndent(selected, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(p, "selection.json"), payload, 0644); e != nil {
					t.Fatal(e)
				}
				for j, c := range selected.Candidates {
					v := filepath.Join(p, []string{"original", "peeled"}[j])
					if e = os.MkdirAll(v, 0755); e != nil {
						t.Fatal(e)
					}
					b, e := json.MarshalIndent(c.Evidence, "", "  ")
					if e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(filepath.Join(v, "factory-baseline.json"), b, 0644); e != nil {
						t.Fatal(e)
					}
					for n, b := range map[string][]byte{"target.bin": c.Evidence.Target.Binary, "target.a80": []byte(c.Evidence.Target.Assembly), "final.mir2": []byte(c.Evidence.Target.FinalMIRDump), "allocation.json": c.Evidence.Target.AllocationJSON} {
						if e = os.WriteFile(filepath.Join(v, n), b, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
			}
			t.Logf("goal%s selected%d original%dT/%dB peeled%dT/%dB", goal, selected.Selected, a.TotalTStates, len(a.Binary), b.TotalTStates, len(b.Binary))
		})
	}
}
func TestMemoryHostArenaSelectorUnobservedRefusal(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "skip_ws", Arguments: []int64{}, Globals: map[string][]byte{"src_len": {0, 0}}}}
	serialized := arenaSelectorProfile(t, w)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	if evidence, e := MeasureMemoryHostArenaPeel(factory, w, serialized, MemoryHostArenaSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil {
		t.Fatal("unobserved arena helper accepted")
	}
}

func TestMemoryHostArenaSelectorZeroLengthObjective(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x8fff, 0, 0}}}
	serialized := arenaSelectorProfile(t, w)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	selected, e := MeasureMemoryHostArenaPeel(factory, w, serialized, MemoryHostArenaSelectorOptions{Goal: "speed", MaxSites: 1})
	if e != nil {
		t.Fatal(e)
	}
	a, b := selected.Candidates[0].Evidence.Target, selected.Candidates[1].Evidence.Target
	want := 0
	if b.TotalTStates < a.TotalTStates {
		want = 1
	}
	if selected.Selected != want {
		t.Fatal("zero-length actual objective")
	}
	for _, candidate := range selected.Candidates {
		if len(candidate.Evidence.Target.Invocations) != 1 || len(candidate.Evidence.Target.Invocations[0].Events) != 0 || candidate.Evidence.Target.Invocations[0].Results[0].I != 1 {
			t.Fatal("zero-length speculated host access")
		}
	}
	t.Logf("zero-length actual original%dT peeled%dT selected%d", a.TotalTStates, b.TotalTStates, selected.Selected)
}

func TestMemoryHostArenaSelectorEqualCostRetainsOriginal(t *testing.T) {
	original := &MemoryHostTargetEvidence{TotalTStates: 100, Binary: make([]byte, 40)}
	peeled := &MemoryHostTargetEvidence{TotalTStates: 100, Binary: make([]byte, 40)}
	for _, goal := range []string{"speed", "size"} {
		if selected := selectMemoryHostArenaCandidate(goal, original, peeled); selected != 0 {
			t.Fatal("equal actual primary costs must retain original")
		}
	}
}

func TestMemoryHostArenaSelectorConsistentCountForgery(t *testing.T) {
	source, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x8fff, 0, 0}}}
	profile, e := hir.RunMemoryHostLoopProfile(source, w)
	if e != nil {
		t.Fatal(e)
	}
	changed := false
	for i, s := range profile.Sites.Sites {
		if s.Function == "region_match" {
			profile.Counts[i].Entries++
			profile.Invocations[0].Counts[i].Entries++
			changed = true
		}
	}
	if !changed {
		t.Fatal("missing source helper loop")
	}
	serialized, e := json.Marshal(profile)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = hir.LoadMemoryHostLoopProfile(bytes.NewReader(serialized), source, w); e != nil {
		t.Fatal("self-consistent forged counts failed loader before replay", e)
	}
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	if evidence, e := MeasureMemoryHostArenaPeel(factory, w, serialized, MemoryHostArenaSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil || !strings.Contains(e.Error(), "serialized counts/observations differ") {
		t.Fatal("consistent forged counts not rejected by fresh replay", e)
	}
}
func TestMemoryHostArenaSelectorLateFactoryMutation(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x8fff, 0, 0}}}
	serialized := arenaSelectorProfile(t, w)
	for _, mode := range []string{"retained_global", "candidate_global", "candidate_shared"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			var first *hir.Module
			factory := func() (*hir.Module, error) {
				m, _ := memoryHostParserFixture(t)
				calls++
				if calls == 1 {
					first = m
				}
				if calls == 8 {
					switch mode {
					case "retained_global":
						first.Globals[0].Init[0] ^= 1
					case "candidate_global":
						m.Globals[0].Init[0] ^= 1
					case "candidate_shared":
						m.FuncByName("cur").Body = first.FuncByName("cur").Body
					}
				}
				return m, nil
			}
			if evidence, e := MeasureMemoryHostArenaPeel(factory, w, serialized, MemoryHostArenaSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil {
				t.Fatal("late factory mutation accepted", mode)
			}
			if calls < 8 {
				t.Fatal("test did not reach late original/candidate factory phase", calls)
			}
		})
	}
}

func TestMemoryHostArenaSelectorSentinelOriginMutation(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x8fff, 0, 0}}}
	serialized := arenaSelectorProfile(t, w)
	for _, phase := range []int{3, 12} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			calls := 0
			var first *hir.Module
			factory := func() (*hir.Module, error) {
				m, _ := memoryHostParserFixture(t)
				calls++
				if calls == 1 {
					first = m
				}
				if calls == phase {
					stmt := first.FuncByName("region_match").Body.Body[1]
					origin, ok := first.SourceLocations[stmt]
					if !ok {
						t.Fatal("known region_match While lacks source origin")
					}
					origin.ByteOffset++
					first.SourceLocations[stmt] = origin
				}
				return m, nil
			}
			if evidence, e := MeasureMemoryHostArenaPeel(factory, w, serialized, MemoryHostArenaSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil {
				t.Fatal("retained source origin mutation accepted")
			}
			if calls < phase {
				t.Fatal("did not reach requested sentinel phase", calls)
			}
		})
	}
}
func TestMemoryHostArenaSelectorStaleHostsAndWorkload(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x8fff, 0, 0}}}
	serialized := arenaSelectorProfile(t, w)
	for _, mode := range []string{"host", "workload"} {
		t.Run(mode, func(t *testing.T) {
			b, e := json.Marshal(w)
			if e != nil {
				t.Fatal(e)
			}
			var changed hir.MemoryHostWorkload
			if e = json.Unmarshal(b, &changed); e != nil {
				t.Fatal(e)
			}
			if mode == "host" {
				changed.Hosts["peek"] = hir.MemoryHostWriteU8
			} else {
				changed.Calls[0].Arguments[2] = 1
			}
			factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
			if evidence, e := MeasureMemoryHostArenaPeel(factory, changed, serialized, MemoryHostArenaSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil {
				t.Fatal("stale host/workload accepted", mode)
			}
		})
	}
}
