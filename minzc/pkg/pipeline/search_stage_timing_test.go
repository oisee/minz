package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"testing"
	"time"
)

// This explicit diagnostic uses a bounded frozen source workload, never
// replaces production search admission and asserts no performance threshold.
func TestSearchBoundedStageTiming(t *testing.T) {
	dir := os.Getenv("MINZ_SEARCH_STAGE_TIMINGS")
	if dir == "" {
		t.Skip("explicit diagnostic output required")
	}
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	factory := func() (*hir.Module, error) { h, _ := memoryHostParserFixture(t); return h, nil }
	w := arenaAppendSmallWorkload(t)
	timings := map[string]float64{}
	stage := func(name string, fn func()) { begin := time.Now(); fn(); timings[name] = time.Since(begin).Seconds() }
	var source *hir.Module
	var raw *hir.MemoryHostRun
	var steps Steps
	stage("parse_fresh_source", func() {
		var e error
		source, e = factory()
		if e != nil {
			t.Fatal(e)
		}
	})
	stage("raw_vm_with_admission", func() {
		var e error
		raw, e = hir.RunMemoryHostBaseline(source, w)
		if e != nil {
			t.Fatal(e)
		}
	})
	stage("compile_fresh_to_asm", func() {
		candidate, e := factory()
		if e != nil {
			t.Fatal(e)
		}
		steps, e = CompileHIRSteps(candidate, Options{AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
	})
	stage("native_run_annotations_admission", func() {
		if _, e := runZ80MemoryHostWorkload(steps, source, w, *raw); e != nil {
			t.Fatal(e)
		}
	})
	var association *MemoryHostLoopProfileEvidenceV2
	stage("closed_association_including_fullbaseline", func() {
		var e error
		association, e = RunZ80MemoryHostLoopProfileWithOptions(factory, w, MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairCompareLegacy})
		if e != nil {
			t.Fatal(e)
		}
	})
	serialized, e := json.Marshal(association)
	if e != nil {
		t.Fatal(e)
	}
	stage("fixed_policy_four_candidates_and_guards", func() {
		if _, e := MeasureMemoryHostArenaAppendWithOptions(factory, w, serialized, MemoryHostArenaAppendSelectorOptions{Goal: "speed", MaxSites: 2}, MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairCompareLegacy}); e != nil {
			t.Fatal(e)
		}
	})
	b, e := json.MarshalIndent(timings, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(dir+"/timings.json", b, 0644); e != nil {
		t.Fatal(e)
	}
	t.Log(string(b))
}
