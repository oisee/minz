package pipeline

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

func TestStructuredFillLoopProfileProvenance(t *testing.T) {
	const body = `layout "linear" 24576 256 1 256; value 255;`
	m := hardwareFillParse(t, body)
	ctx := hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "vm", WorkloadSHA256: strings.Repeat("a", 64), HostsSHA256: strings.Repeat("b", 64), Dependencies: m.SourceUnits}
	profiler, e := hir.NewLoopProfilerWithContext(m, ctx)
	if e != nil {
		t.Fatal(e)
	}
	profiler.VM.EnsureHeap(65536)
	before := profiler.VM.ReadHeap(0, 65536)
	want := append([]byte(nil), before...)
	for address := 0x6000; address < 0x6100; address++ {
		want[address] = 255
	}
	if _, e = profiler.Call("fill_test", nil); e != nil {
		t.Fatal(e)
	}
	profile := profiler.Snapshot()
	if profile.Sites.UnmappedLoops != 0 || len(profile.Counts) != 1 || profile.Counts[0].Entries != 1 || profile.Counts[0].BodyEntries != 256 || profile.Sites.Sites[0].Origin.Metafunction != "fill" {
		t.Fatalf("generated fill profile %+v", profile)
	}
	if !bytes.Equal(profiler.VM.ReadHeap(0, 65536), want) {
		t.Fatal("profile instrumentation changed fill memory")
	}
	data, e := json.MarshalIndent(profile, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = hir.LoadLoopProfile(bytes.NewReader(data), m, ctx); e != nil {
		t.Fatal("generated fill profile cannot replay", e)
	}
	hardwareFillExport(t, "structured-loop-profile.json", data)
	for _, grace := range []bool{false, true} {
		steps, e := CompileHIRSteps(hardwareFillParse(t, body), Options{AssertMode: "none", CollectHIRSites: true, UseHIRGrace: grace, UseGrace: grace})
		if e != nil {
			t.Fatal(e)
		}
		if steps.HIRLoopSites.UnmappedLoops != 0 || len(steps.HIRLoopSites.Sites) != 1 || steps.HIRLoopSites.Sites[0].ID != profile.Sites.Sites[0].ID {
			t.Fatal("input provenance lost before Grace", steps.HIRLoopSites)
		}
	}
	t.Logf("fill: one callsite loop, 1 entry / 256 iterations; strict profile reload and full RAM oracle PASS")
}
