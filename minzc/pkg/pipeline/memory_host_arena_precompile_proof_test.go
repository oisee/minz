package pipeline

import (
	"github.com/minz/minzc/pkg/hir"
	"testing"
)

// Independently validate the privately constructed source delta before any
// compiler preparation or observed target execution.
func TestMemoryHostArenaPeelIndependentProof(t *testing.T) {
	source, w := memoryHostParserFixture(t)
	actual, _ := memoryHostParserFixture(t)
	if e := peelMemoryHostArenaFirstByte(actual, w); e != nil {
		t.Fatal(e)
	}
	sites, e := hir.BuildLoopSiteMap(source)
	if e != nil {
		t.Fatal(e)
	}
	site := ""
	for _, s := range sites.Sites {
		if s.Function == "region_match" {
			site = s.ID
		}
	}
	proof, e := buildMemoryHostArenaPrecompileProof(source, actual, w, site)
	if e != nil {
		t.Fatal(e)
	}
	if proof.SourceSite.HIRPath != "body/s1" || proof.OriginalCompatibility.HIRSHA256 == proof.ActualCompatibility.HIRSHA256 {
		t.Fatal("proof identity missing")
	}
}
