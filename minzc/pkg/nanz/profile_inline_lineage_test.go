package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestCallbackLoopsRetainLineageWhenInlined(t *testing.T) {
	const source = `global hits:u8
fun visit(x:u8) {
 var i:u8=0
 while i<x { hits=hits+1
 i=i+1 }
}
fun main()->u8 {
 hits=0
 range(0..3).forEach(visit)
 range(0..2).forEach(visit)
 return hits
}`
	m, err := nanz.Parse(source, "inline.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.SeedLoopLineage(m); err != nil {
		t.Fatal(err)
	}
	records := hir.CurrentLoopLineage(m)
	var site string
	for _, r := range records {
		if r.Function == "visit" {
			site = r.Lineage.SiteID
		}
	}
	if site == "" {
		t.Fatal("missing callback site")
	}
	before := len(m.LoopLineage)
	raw := hir.LowerModule(m)
	if len(m.LoopLineage) != before {
		t.Fatal("lowering changed input metadata")
	}
	if len(raw.FuncByName("visit").LoopOrigins) != 1 {
		t.Fatal("callback lost standalone origin")
	}
	count := 0
	for _, origin := range raw.FuncByName("main").LoopOrigins {
		if origin.SiteID == site {
			count++
			if origin.Role != "original" {
				t.Fatal(origin)
			}
		}
	}
	if count != 2 {
		t.Fatalf("two inline copies must retain callback site, got %d: %+v", count, raw.FuncByName("main").LoopOrigins)
	}
	vm := mir2.NewVM(raw)
	result, err := vm.Call("main", nil)
	if err != nil || len(result) != 1 || result[0].I != 9 {
		t.Fatalf("result=%v err=%v", result, err)
	}
}
