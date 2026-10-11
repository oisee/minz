package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestMemoryHostAppendSelectorOriginal30Objectives(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	serialized := arenaSelectorProfile(t, w)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	for _, goal := range []string{"speed", "size"} {
		t.Run(goal, func(t *testing.T) {
			selected, e := MeasureMemoryHostAppendCopyHoist(factory, w, serialized, MemoryHostAppendSelectorOptions{Goal: goal, MaxSites: 1})
			if e != nil {
				t.Fatal(e)
			}
			if len(selected.Candidates) != 2 {
				t.Fatal("bounded two-candidate contract")
			}
			proof := selected.Candidates[1].PrecompileProof
			if proof == nil || proof.Schema != "minz.exact-memory-host-append-hoist-precompile/v1" || proof.Function != "intern_region" || !reflect.DeepEqual(proof.OriginalCompatibility, selected.Candidates[0].Evidence.Compatibility) || !reflect.DeepEqual(proof.ActualCompatibility, selected.Candidates[1].Evidence.Compatibility) || !reflect.DeepEqual(proof.OriginalFunctions, selected.Candidates[0].Evidence.FunctionSites) || !reflect.DeepEqual(proof.ActualFunctions, selected.Candidates[1].Evidence.FunctionSites) {
				t.Fatal("precompile proof not bound to measured candidates")
			}

			a, b := selected.Candidates[0].Evidence.Target, selected.Candidates[1].Evidence.Target
			want := 0
			if goal == "speed" && b.TotalTStates < a.TotalTStates || goal == "size" && len(b.Binary) < len(a.Binary) {
				want = 1
			}
			if selected.Selected != want {
				t.Fatal("actual whole-caller objective mismatch")
			}
			if dir := os.Getenv("MINZ_APPEND_SELECTOR_ARTIFACTS"); dir != "" {
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
					v := filepath.Join(p, []string{"original", "hoisted"}[j])
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
			t.Logf("goal%s selected%d original%dT/%dB hoisted%dT/%dB", goal, selected.Selected, a.TotalTStates, len(a.Binary), b.TotalTStates, len(b.Binary))
		})
	}
}
func TestMemoryHostAppendSelectorConsistentCountForgery(t *testing.T) {
	source, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "intern_region", Arguments: []int64{0, 0}, Globals: map[string][]byte{"str_arena": {0, 128}, "str_bump": {0, 128}}}}
	profile, e := hir.RunMemoryHostLoopProfile(source, w)
	if e != nil {
		t.Fatal(e)
	}
	changed := false
	for i, s := range profile.Sites.Sites {
		if s.Function == "intern_region" && s.HIRPath == "body/s5" {
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
	if evidence, e := MeasureMemoryHostAppendCopyHoist(factory, w, serialized, MemoryHostAppendSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil || !strings.Contains(e.Error(), "serialized counts/observations differ") {
		t.Fatal("consistent forged counts not rejected by fresh replay", e)
	}
}
func TestMemoryHostAppendSelectorLateFactoryMutation(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "intern_region", Arguments: []int64{0, 0}, Globals: map[string][]byte{"str_arena": {0, 128}, "str_bump": {0, 128}}}}
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
			if evidence, e := MeasureMemoryHostAppendCopyHoist(factory, w, serialized, MemoryHostAppendSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil {
				t.Fatal("late factory mutation accepted", mode)
			}
			if calls < 8 {
				t.Fatal("test did not reach late original/candidate factory phase", calls)
			}
		})
	}
}

func TestMemoryHostAppendSelectorSentinelOriginMutation(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "intern_region", Arguments: []int64{0, 0}, Globals: map[string][]byte{"str_arena": {0, 128}, "str_bump": {0, 128}}}}
	serialized := arenaSelectorProfile(t, w)
	for _, phase := range []int{3, 6, 12} {
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
					stmt := first.FuncByName("intern_region").Body.Body[5]
					origin, ok := first.SourceLocations[stmt]
					if !ok {
						t.Fatal("known intern_region While lacks source origin")
					}
					origin.ByteOffset++
					first.SourceLocations[stmt] = origin
				}
				return m, nil
			}
			if evidence, e := MeasureMemoryHostAppendCopyHoist(factory, w, serialized, MemoryHostAppendSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil {
				t.Fatal("retained source origin mutation accepted")
			}
			if calls < phase {
				t.Fatal("did not reach requested sentinel phase", calls)
			}
		})
	}
}
func TestMemoryHostAppendSelectorStaleHostsAndWorkload(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "intern_region", Arguments: []int64{0, 0}, Globals: map[string][]byte{"str_arena": {0, 128}, "str_bump": {0, 128}}}}
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
				changed.Calls[0].Arguments[1] = 1
			}
			factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
			if evidence, e := MeasureMemoryHostAppendCopyHoist(factory, changed, serialized, MemoryHostAppendSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || evidence != nil {
				t.Fatal("stale host/workload accepted", mode)
			}
		})
	}
}

func TestMemoryHostAppendSelectorUnobservedAndOptions(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "skip_ws", Arguments: []int64{}, Globals: map[string][]byte{"src_len": {0, 0}}}}
	serialized := arenaSelectorProfile(t, w)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	for _, o := range []MemoryHostAppendSelectorOptions{{Goal: "speed", MaxSites: 1}, {Goal: "invalid", MaxSites: 1}, {Goal: "speed", MaxSites: 2}} {
		if e, err := MeasureMemoryHostAppendCopyHoist(factory, w, serialized, o); err == nil || e != nil {
			t.Fatal("unobserved or invalid bounded mode accepted")
		}
	}
}
func TestMemoryHostAppendSelectorObservedZeroBody(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "intern_region", Arguments: []int64{0, 0}, Globals: map[string][]byte{"str_arena": {0, 128}, "str_bump": {0, 128}}}}
	serialized := arenaSelectorProfile(t, w)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	e, err := MeasureMemoryHostAppendCopyHoist(factory, w, serialized, MemoryHostAppendSelectorOptions{Goal: "speed", MaxSites: 1})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i, s := range e.Profile.Sites.Sites {
		if s.ID == e.SiteID {
			found = true
			if s.HIRPath != "body/s5" || e.Profile.Counts[i].Entries != 1 || e.Profile.Counts[i].BodyEntries != 0 {
				t.Fatal("wrong source loop or zero-body count")
			}
		}
	}
	if !found {
		t.Fatal("missingselectedsite")
	}
	a, b := e.Candidates[0].Evidence.Target, e.Candidates[1].Evidence.Target
	want := selectMemoryHostArenaCandidate("speed", a, b)
	if e.Selected != want {
		t.Fatal("actualobjective")
	}
	t.Logf("zero-body original%dT hoisted%dT selected%d", a.TotalTStates, b.TotalTStates, e.Selected)
}

func TestMemoryHostAppendSelectorProofBinding(t *testing.T) {
	for _, mutation := range []string{"valid", "search_site", "actual_origin", "actual_shape"} {
		t.Run(mutation, func(t *testing.T) {
			source, w := memoryHostParserFixture(t)
			actual, _ := memoryHostParserFixture(t)
			if e := hoistMemoryHostAppendBases(actual, w); e != nil {
				t.Fatal(e)
			}
			sites, e := hir.BuildLoopSiteMap(source)
			if e != nil {
				t.Fatal(e)
			}
			id := ""
			for _, s := range sites.Sites {
				if s.Function == "intern_region" && s.HIRPath == "body/s5" {
					id = s.ID
				}
			}
			if mutation == "search_site" {
				for _, s := range sites.Sites {
					if s.Function == "intern_region" && s.HIRPath == "body/s1" {
						id = s.ID
					}
				}
			}
			if mutation == "actual_origin" {
				loop := actual.FuncByName("intern_region").Body.Body[7]
				o := actual.SourceLocations[loop]
				o.ByteOffset++
				actual.SourceLocations[loop] = o
			}
			if mutation == "actual_shape" {
				actual.FuncByName("intern_region").Body.Body[4], actual.FuncByName("intern_region").Body.Body[5] = actual.FuncByName("intern_region").Body.Body[5], actual.FuncByName("intern_region").Body.Body[4]
			}
			proof, e := buildMemoryHostAppendPrecompileProof(source, actual, w, id)
			if mutation == "valid" {
				if e != nil || proof == nil || proof.SourceSite.HIRPath != "body/s5" {
					t.Fatal(e)
				}
			} else if e == nil || proof != nil {
				t.Fatal("forgedsourceoractualproofaccepted")
			}
		})
	}
}
func TestMemoryHostAppendSelectorCapAndGC(t *testing.T) {
	calls := 0
	factory := func() (*hir.Module, error) { calls++; m, _ := memoryHostParserFixture(t); return m, nil }
	_, w := memoryHostParserFixture(t)
	if e, err := MeasureMemoryHostAppendCopyHoist(factory, w, make([]byte, hir.MaxMemoryHostLoopProfileBytes+1), MemoryHostAppendSelectorOptions{Goal: "speed", MaxSites: 1}); err == nil || e != nil || calls != 0 {
		t.Fatal("oversizedprofile reached factory")
	}
	shared, _ := memoryHostParserFixture(t)
	seen := map[uintptr]any{}
	roots := []any{}
	if e := jointCheckFreshHIR(shared, seen); e != nil {
		t.Fatal(e)
	}
	retainArenaSourceGraph(shared, &roots)
	body := shared.FuncByName("intern_region").Body
	runtime.GC()
	fresh, _ := memoryHostParserFixture(t)
	fresh.FuncByName("intern_region").Body = body
	if e := jointCheckFreshHIR(fresh, seen); e == nil {
		t.Fatal("true retained shared body acceptedafterGC")
	}
	runtime.KeepAlive(roots)
}
