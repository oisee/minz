package nanz_test

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestSumGuardObservationsAndLineage(t *testing.T) {
	src := `fun sum_array(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}`
	m, e := nanz.Parse(src, "guard.nanz")
	if e != nil {
		t.Fatal(e)
	}
	c, e := hir.BuildAutomaticProfileContext(m, hir.ProfileInputs{TargetModel: "cpm"})
	if e != nil {
		t.Fatal(e)
	}
	p, e := hir.NewLoopProfilerWithContext(m, c)
	if e != nil {
		t.Fatal(e)
	}
	p.VM.EnsureHeap(65536)
	p.VM.WriteHeap(65534, 7)
	for _, args := range [][]mir2.Value{{{I: 65534}, {I: 1}}, {{I: 65535}, {I: 0}}, {{I: 65535}, {I: 1}}} {
		if _, e = p.Call("sum_array", args); e != nil {
			t.Fatal(e)
		}
	}
	s := p.Snapshot()
	if len(s.Guards) != 1 {
		t.Fatal(s.Guards)
	}
	g := s.Guards[0]
	if g.FastEntries != 2 || g.FallbackEntries != 1 || g.FastBodyEntries != 1 || g.FallbackBodyEntries != 1 {
		t.Fatal(g)
	}
	data, _ := json.Marshal(s)
	if _, e := hir.LoadLoopProfile(bytes.NewReader(data), m, c); e != nil {
		t.Fatal(e)
	}
	s.Guards[0].FastEntries++
	data, _ = json.Marshal(s)
	if _, e := hir.LoadLoopProfile(bytes.NewReader(data), m, c); e == nil {
		t.Fatal("inconsistent guard accepted")
	}
	if e := hir.SeedLoopLineage(m); e != nil {
		t.Fatal(e)
	}
	original := hir.CurrentLoopLineage(m)
	if len(original) != 1 {
		t.Fatal(original)
	}
	stats, e := hir.RunGrace(m, 0)
	if e != nil || stats.ByRule["hir-guarded-pointer-walk"] != 1 {
		t.Fatalf("%v %v", stats, e)
	}
	transformed := hir.CurrentLoopLineage(m)
	if len(transformed) != 2 || transformed[0].Lineage.Role != "fast" || transformed[1].Lineage.Role != "fallback" {
		t.Fatal(transformed)
	}
	for _, l := range transformed {
		if l.Lineage.SiteID != original[0].Lineage.SiteID {
			t.Fatal("lost site identity")
		}
	}
	if e := hir.SeedLoopLineage(m); e == nil {
		t.Fatal("reseed accepted")
	}
	// Lowering must not mutate the input lineage map; raw block origins survive.
	before := len(m.LoopLineage)
	lowered := hir.LowerModule(m)
	if len(m.LoopLineage) != before || len(lowered.FuncByName("sum_array").LoopOrigins) != 2 {
		t.Fatal("lowering lost attribution or mutated input")
	}
	t.Logf("guard=%+v lineage=%+v", g, transformed)
}
