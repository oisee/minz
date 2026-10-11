package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"os"
	"path/filepath"
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
