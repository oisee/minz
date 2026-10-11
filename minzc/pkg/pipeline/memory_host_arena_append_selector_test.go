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

func arenaAppendSmallWorkload(t *testing.T) hir.MemoryHostWorkload {
	_, w := memoryHostParserFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x8000, 0, 0}}, {Function: "intern_region", Arguments: []int64{0, 0}, Globals: map[string][]byte{"str_arena": {0, 128}, "str_bump": {0, 128}}}}
	return w
}
func TestMemoryHostArenaAppendSelectorOriginal30(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	serialized := arenaSelectorProfile(t, w)
	for _, goal := range []string{"speed", "size"} {
		t.Run(goal, func(t *testing.T) {
			calls := 0
			factory := func() (*hir.Module, error) { calls++; runtime.GC(); m, _ := memoryHostParserFixture(t); return m, nil }
			e, err := MeasureMemoryHostArenaAppend(factory, w, serialized, MemoryHostArenaAppendSelectorOptions{Goal: goal, MaxSites: 2})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 24 || len(e.Candidates) != 4 {
				t.Fatal("lifecycle", calls)
			}
			want := 0
			for j, c := range e.Candidates {
				choices := hir.MemoryHostArenaAppendChoices{ArenaPeel: j&1 != 0, AppendHoist: j&2 != 0}
				if c.Choices != choices || bindMemoryHostArenaAppendProof(c.PrecompileProof, e.SourceSites, choices, e.Candidates[0].Evidence, c.Evidence) != nil {
					t.Fatal("typedcandidateproofbinding")
				}
				if selectMemoryHostArenaCandidate(goal, e.Candidates[want].Evidence.Target, c.Evidence.Target) == 1 {
					want = j
				}
			}
			if e.Selected != want {
				t.Fatal("wholefourobjective")
			}
			if d := os.Getenv("MINZ_ARENA_APPEND_SELECTOR_ARTIFACTS"); d != "" {
				p := filepath.Join(d, goal)
				if err := os.MkdirAll(p, 0755); err != nil {
					t.Fatal(err)
				}
				b, _ := json.MarshalIndent(e, "", "  ")
				os.WriteFile(filepath.Join(p, "selection.json"), b, 0644)
				os.WriteFile(filepath.Join(p, "source-profile.json"), serialized, 0644)
				for j, c := range e.Candidates {
					q := filepath.Join(p, []string{"original", "arena", "append", "both"}[j])
					os.MkdirAll(q, 0755)
					b, _ := json.MarshalIndent(c.Evidence, "", "  ")
					for n, b := range map[string][]byte{"factory-baseline.json": b, "target.bin": c.Evidence.Target.Binary, "target.a80": []byte(c.Evidence.Target.Assembly), "allocation.json": c.Evidence.Target.AllocationJSON, "final.mir2": []byte(c.Evidence.Target.FinalMIRDump)} {
						if err := os.WriteFile(filepath.Join(q, n), b, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			for j, c := range e.Candidates {
				t.Logf("%s mask%d %dT/%dB", goal, j, c.Evidence.Target.TotalTStates, len(c.Evidence.Target.Binary))
			}
			t.Logf("selected%d calls%d", e.Selected, calls)
		})
	}
}
func TestMemoryHostArenaAppendSelectorForgedReplay(t *testing.T) {
	source, _ := memoryHostParserFixture(t)
	w := arenaAppendSmallWorkload(t)
	p, err := hir.RunMemoryHostLoopProfile(source, w)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range p.Sites.Sites {
		if s.Function == "intern_region" && s.HIRPath == "body/s5" {
			p.Counts[i].Entries++
			p.Invocations[1].Counts[i].Entries++
		}
	}
	b, _ := json.Marshal(p)
	if _, err = hir.LoadMemoryHostLoopProfile(bytes.NewReader(b), source, w); err != nil {
		t.Fatal("consistentforgeryloader", err)
	}
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	if e, err := MeasureMemoryHostArenaAppend(factory, w, b, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}); err == nil || e != nil || !strings.Contains(err.Error(), "serialized counts") {
		t.Fatal("forgeryaccepted", err)
	}
}
func TestMemoryHostArenaAppendSelectorLateCrossMask(t *testing.T) {
	w := arenaAppendSmallWorkload(t)
	b := arenaSelectorProfile(t, w)
	for _, phase := range []int{6, 12, 18, 24} {
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
					o := first.SourceLocations[stmt]
					o.ByteOffset++
					first.SourceLocations[stmt] = o
				}
				return m, nil
			}
			if e, err := MeasureMemoryHostArenaAppend(factory, w, b, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}); err == nil || e != nil {
				t.Fatal("lateoriginaccepted")
			}
			if calls < phase {
				t.Fatal("phaseneverreached", calls)
			}
		})
	}
}
func TestMemoryHostArenaAppendSelectorAcrossMaskSharing(t *testing.T) {
	w := arenaAppendSmallWorkload(t)
	b := arenaSelectorProfile(t, w)
	calls := 0
	var first *hir.Module
	factory := func() (*hir.Module, error) {
		m, _ := memoryHostParserFixture(t)
		calls++
		if calls == 1 {
			first = m
		}
		if calls == 14 {
			runtime.GC()
			m.FuncByName("region_match").Body = first.FuncByName("region_match").Body
		}
		return m, nil
	}
	if e, err := MeasureMemoryHostArenaAppend(factory, w, b, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}); err == nil || e != nil {
		t.Fatal("crossmasksharedaccepted")
	}
	if calls != 14 {
		t.Fatal("mask2sharingnotreached", calls)
	}
}
func TestMemoryHostArenaAppendSelectorBoundsAndUnobserved(t *testing.T) {
	w := arenaAppendSmallWorkload(t)
	calls := 0
	factory := func() (*hir.Module, error) { calls++; m, _ := memoryHostParserFixture(t); return m, nil }
	for _, o := range []MemoryHostArenaAppendSelectorOptions{{Goal: "bad", MaxSites: 2}, {Goal: "speed", MaxSites: 1}} {
		if e, err := MeasureMemoryHostArenaAppend(factory, w, nil, o); err == nil || e != nil || calls != 0 {
			t.Fatal("invalidoptionscallback")
		}
	}
	if e, err := MeasureMemoryHostArenaAppend(factory, w, make([]byte, hir.MaxMemoryHostLoopProfileBytes+1), MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}); err == nil || e != nil || calls != 0 {
		t.Fatal("capcallback")
	}
	w.Calls = w.Calls[:1]
	b := arenaSelectorProfile(t, w)
	if e, err := MeasureMemoryHostArenaAppend(factory, w, b, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}); err == nil || e != nil || !strings.Contains(err.Error(), "both exact sites") {
		t.Fatal("unobservedappendaccepted", err)
	}
}
func TestMemoryHostArenaAppendSelectorProofChoices(t *testing.T) {
	source, w := memoryHostParserFixture(t)
	for mask := 0; mask < 4; mask++ {
		actual, _ := memoryHostParserFixture(t)
		choices := hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}
		if err := constructMemoryHostArenaAppend(actual, w, choices.ArenaPeel, choices.AppendHoist); err != nil {
			t.Fatal(err)
		}
		p, err := buildMemoryHostArenaAppendPrecompileProof(source, actual, w, choices)
		if err != nil {
			t.Fatal(err)
		}
		original := &MemoryHostBaselineEvidence{Compatibility: p.OriginalCompatibility, FunctionSites: p.OriginalFunctions}
		variant := &MemoryHostBaselineEvidence{Compatibility: p.ActualCompatibility, FunctionSites: p.ActualFunctions}
		if bindMemoryHostArenaAppendProof(p, p.SourceSites, choices, original, variant) != nil {
			t.Fatal("validproofrefused")
		}
		wrong := choices
		wrong.ArenaPeel = !wrong.ArenaPeel
		if bindMemoryHostArenaAppendProof(p, p.SourceSites, wrong, original, variant) == nil {
			t.Fatal("wrongmaskproofaccepted")
		}
		bad := p.SourceSites
		bad.Append.HIRPath = "body/s1"
		if bindMemoryHostArenaAppendProof(p, bad, choices, original, variant) == nil {
			t.Fatal("searchsiteproofaccepted")
		}
		changed := *variant
		changed.FunctionSites = original.FunctionSites
		if mask != 0 && reflect.DeepEqual(changed.FunctionSites, p.ActualFunctions) {
			t.Fatal("testneedsdistinctactualidentity")
		}
		if mask != 0 && bindMemoryHostArenaAppendProof(p, p.SourceSites, choices, original, &changed) == nil {
			t.Fatal("wrongactualidentityaccepted")
		}
	}
}
