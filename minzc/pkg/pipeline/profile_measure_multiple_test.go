package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

func TestMeasureOneFunctionKeepsOtherSitesOriginal(t *testing.T) {
	const source = `fun short_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
fun long_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}`
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "two-sums.nanz") }
	n := int64(5)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"array": {1, 2, 3, 4, 5}}, Calls: []hir.ProfileInvocation{{Function: "long_sum", Arguments: []hir.ProfileArgument{{Resource: "array"}, {Value: &n}}}}}
	model, e, err := MeasureLoopCostModel(factory, "long_sum", w, Options{AssertMode: "none", UseHIRGrace: true})
	if err != nil {
		t.Fatal(err)
	}
	if model.Function != "long_sum" || len(e.Rows) != 1022 {
		t.Fatal(model)
	}
	// The unrelated function is unchanged in both binaries: only requested site
	// may be rewritten, even when ordinary Grace would select both sum idioms.
	if len(e.PointerBinary) <= len(e.OriginalBinary) {
		t.Fatal("missing guarded variant evidence")
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	sites, err := hir.BuildLoopSiteMap(m)
	if err != nil {
		t.Fatal(err)
	}
	decisions := []hir.ProfileDecision{}
	for _, site := range sites.Sites {
		variant := "original"
		if site.Function == "long_sum" {
			variant = "pointer-walk"
		}
		decisions = append(decisions, hir.ProfileDecision{Compatibility: e.Compatibility, SiteID: site.ID, Variant: variant})
	}
	if err := hir.ApplyProfileDecisions(m, decisions); err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(m)
	short := raw.FuncByName("short_sum").LoopOrigins
	if len(short) != 1 {
		t.Fatal(short)
	}
	for _, origin := range short {
		if origin.Role != "original" {
			t.Fatal("unrelated site rewritten", origin)
		}
	}
	if len(raw.FuncByName("long_sum").LoopOrigins) != 2 {
		t.Fatal("requested site not rewritten")
	}
	for _, row := range e.Rows {
		if row.Path == "fast" && row.N == 5 && row.Result != 169 {
			t.Fatal(row)
		}
	}
}
