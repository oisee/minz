package pascal

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"testing"
)

func TestPascalProfileSourceAndLoopOrigins(t *testing.T) {
	s := "program P;\nvar i:byte;\nbegin\n repeat\n for i:=0 to 2 do begin end;\n until true;\n while false do begin end\nend."
	m, e := Compile(s, "p.pas")
	if e != nil {
		t.Fatal(e)
	}
	if !m.SourceUnitsComplete || m.SourceUnits["p.pas"] != fmt.Sprintf("%x", sha256.Sum256([]byte(s))) {
		t.Fatal("source manifest", m.SourceUnits)
	}
	sites, e := hir.BuildLoopSiteMap(m)
	if e != nil || sites.UnmappedLoops != 0 || len(sites.Sites) != 3 {
		t.Fatal(sites, e)
	}
	for _, site := range sites.Sites {
		if site.Origin.Unit != "p.pas" || site.Origin.SourceSHA256 != m.SourceUnits["p.pas"] || site.Origin.ByteOffset >= len(s) || site.Origin.Column < 1 {
			t.Fatal(site)
		}
	}
	for _, s := range []string{"program P; uses Crt; begin end.", "program P; {$I other.pas} begin end.", "program P; (*$include other.pas*) begin end."} {
		m, e := Compile(s, "incomplete.pas")
		if e != nil {
			t.Fatal(e)
		}
		if m.SourceUnitsComplete {
			t.Fatal("ignored source dependency claimed complete", s)
		}
	}
	m, e = Compile("program P; {$I+} begin end.", "noinclude.pas")
	if e != nil || !m.SourceUnitsComplete {
		t.Fatal("nondependency directive rejected", m, e)
	}
}
