package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"testing"
)

const sitePolicySource = `fun two(a:u16,b:u16,n:u8,m:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n {s=s+(a+b)
 i=i+1}
 var j:u8=0
 while j<m {s=s+(a+b)
 j=j+1}
 return s
}`

func TestHIRInvariantSitePolicySkipsDisabledFirstLoop(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			m, err := nanz.Parse(sitePolicySource, "site-policy.nanz")
			if err != nil {
				t.Fatal(err)
			}
			sites, err := hir.BuildLoopSiteMap(m)
			if err != nil || len(sites.Sites) != 2 {
				t.Fatal("source sites", err)
			}
			if err = hir.SeedLoopLineage(m); err != nil {
				t.Fatal(err)
			}
			m.PointerWalkPolicy = map[string]bool{}
			m.GraceRulePolicy = map[string]bool{"hir-invariant-base": true}
			m.InvariantBasePolicy = map[string]bool{}
			for i, s := range sites.Sites {
				m.InvariantBasePolicy[s.ID] = mask&(1<<i) != 0
			}
			if _, err = hir.RunGrace(m, 0); err != nil {
				t.Fatal(err)
			}
			verify := func() {
				found := map[string]bool{}
				for _, l := range hir.CurrentLoopLineage(m) {
					if l.Lineage.Rule == "hir-invariant-base" {
						found[l.Lineage.SiteID] = true
					}
				}
				for i, s := range sites.Sites {
					if found[s.ID] != (mask&(1<<i) != 0) {
						t.Fatal("incorrect realized selection", mask, i, found)
					}
				}
			}
			verify()
			steps, err := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true})
			if err != nil {
				t.Fatal(err)
			}
			verify() // Sticky through the second Grace pass.
			remaining, err := hir.RemainingInvariantBaseSites(m)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range remaining {
				if m.InvariantBasePolicy[id] {
					t.Fatal("selected site not completely hoisted")
				}
			}
			for _, nm := range [][2]int64{{0, 0}, {0, 255}, {255, 0}, {1, 5}, {255, 255}} {
				input, err := nanz.Parse(sitePolicySource, "site-policy.nanz")
				if err != nil {
					t.Fatal(err)
				}
				w := workloadSpec()
				w.Calls = []hir.ProfileInvocation{{Function: "two", Arguments: []hir.ProfileArgument{workloadLiteral(65535), workloadLiteral(2), workloadLiteral(nm[0]), workloadLiteral(nm[1])}}}
				raw, err := hir.RunLoopProfile(input, w)
				if err != nil {
					t.Fatal(err)
				}
				want := (nm[0] + nm[1]) & 65535
				if raw.Results[0][0].I != want {
					t.Fatal("raw oracle")
				}
				run, err := RunZ80ProfileWorkload(steps, input, w, *raw)
				if err != nil {
					t.Fatal(err)
				}
				if run.Invocations[0].Results[0].I != want {
					t.Fatal("target independent oracle")
				}
			}
		})
	}
}

func TestHIRInvariantSitePolicyRequiresAttribution(t *testing.T) {
	m, err := nanz.Parse(sitePolicySource, "site-policy.nanz")
	if err != nil {
		t.Fatal(err)
	}
	sites, err := hir.BuildLoopSiteMap(m)
	if err != nil {
		t.Fatal(err)
	}
	m.InvariantBasePolicy = map[string]bool{sites.Sites[0].ID: true}
	result, err := hir.RunGrace(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.ByRule["hir-invariant-base"] != 0 {
		t.Fatal("unattributed loop rewritten")
	}
}

func TestHIRScalarInvariantDiscoverySeesBothLoops(t *testing.T) {
	m, err := nanz.Parse(sitePolicySource, "site-policy.nanz")
	if err != nil {
		t.Fatal(err)
	}
	before := m.Dump()
	ids, err := hir.EligibleScalarInvariantSites(m)
	if err != nil || len(ids) != 2 || ids[0] == ids[1] {
		t.Fatal("incomplete original legality discovery", ids, err)
	}
	if m.Dump() != before || m.LoopLineage != nil {
		t.Fatal("discovery mutated source")
	}
	m.InvariantBasePolicy = map[string]bool{}
	ids, err = hir.EligibleScalarInvariantSites(m)
	if err != nil || len(ids) != 2 {
		t.Fatal("profitability policy changed legality")
	}
}

func TestHIRInvariantPolicyCannotEnableMutation(t *testing.T) {
	source := `fun f(a:u16,b:u16,n:u8)->u16 {var s:u16=0
 var i:u8=0
 while i<n {s=s+(a+b)
 a.0=1
 i=i+1}
 return s}`
	m, err := nanz.Parse(source, "mutating-site.nanz")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := hir.EligibleInvariantBaseSites(m)
	if err != nil || len(ids) != 0 {
		t.Fatal("mutating loop declared legal")
	}
	if err = hir.SeedLoopLineage(m); err != nil {
		t.Fatal(err)
	}
	m.InvariantBasePolicy = map[string]bool{}
	for _, l := range hir.CurrentLoopLineage(m) {
		m.InvariantBasePolicy[l.Lineage.SiteID] = true
	}
	result, err := hir.RunGrace(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.ByRule["hir-invariant-base"] != 0 {
		t.Fatal("selection overrode legality")
	}
}
