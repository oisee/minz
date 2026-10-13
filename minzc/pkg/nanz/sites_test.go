package nanz_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func TestLoopSourceSiteIdentity(t *testing.T) {
	src := `fun test(n:u8)->u8 {
 var sum:u8=0
 while n>0 {sum=sum+n
  n=n-1}
 return sum
}`
	parse := func(src string) *hir.Module {
		m, err := nanz.Parse(src, "site.nanz")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	build := func(m *hir.Module) *hir.LoopSiteMap {
		sites, err := hir.BuildLoopSiteMap(m)
		if err != nil {
			t.Fatal(err)
		}
		return sites
	}
	a := build(parse(src))
	b := build(parse(src))
	if len(a.Sites) != 1 || a.UnmappedLoops != 0 {
		t.Fatalf("map: %+v", a)
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("nondeterministic site map")
	}
	site := a.Sites[0]
	if site.Function != "test" || site.Kind != "while" || site.Origin.Line != 3 || site.Origin.Column != 2 || site.Origin.ByteOffset != strings.Index(src, "while") || site.Origin.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(src))) {
		t.Fatalf("origin: %+v", site)
	}
	// Identity can survive a same-position edit; the source fingerprint cannot.
	changed := build(parse(strings.Replace(src, "sum+n", "sum+1", 1))).Sites[0]
	if changed.ID != site.ID || changed.Origin.SourceSHA256 == site.Origin.SourceSHA256 {
		t.Fatal("source edit not fingerprinted")
	}
	moved := build(parse("\n" + src)).Sites[0]
	if moved.ID == site.ID {
		t.Fatal("shifted source offset reused")
	}
	// A synthetic replacement has no invented attribution.
	m := parse(src)
	loop := m.FuncByName("test").Body.Body[1].(*hir.WhileStmt)
	copy := *loop
	m.FuncByName("test").Body.Body[1] = &copy
	missing := build(m)
	if len(missing.Sites) != 0 || missing.UnmappedLoops != 1 {
		t.Fatalf("synthetic origin guessed: %+v", missing)
	}
	m = parse(src)
	for node, loc := range m.SourceLocations {
		loc.SourceSHA256 = "bad"
		m.SourceLocations[node] = loc
	}
	if _, err := hir.BuildLoopSiteMap(m); err == nil {
		t.Fatal("invalid origins accepted")
	}
}

func TestImportedLoopSourceSites(t *testing.T) {
	dir := t.TempDir()
	dep := `fun helper(n:u8)->u8 {while n>0 {n=n-1}
 return n}`
	if err := os.WriteFile(filepath.Join(dir, "helper.nanz"), []byte(dep), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := nanz.ParseWithOpts("import helper\nfun test(n:u8)->u8 {return helper.helper(n)}", "main.nanz", nanz.ParseOpts{BaseDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	sites, err := hir.BuildLoopSiteMap(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites.Sites) != 1 || sites.Sites[0].Origin.Unit != "helper" || sites.Sites[0].Origin.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(dep))) || sites.Sites[0].Function != "helper__helper" {
		t.Fatalf("imported origin: %+v", sites)
	}
}
