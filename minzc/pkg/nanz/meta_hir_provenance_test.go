package nanz

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

const structuredLoopDefinition = `fun @build() {
 let n:u16=hir_ref(c"n",c"u8")
 let b:u16=hir_block()
 hir_append(b,hir_var(c"n",c"u8",hir_int(3,c"u8")))
 let body:u16=hir_block()
 hir_append(body,hir_assign(n,hir_binary(c"-",n,hir_int(1,c"u8"))))
 hir_append(b,hir_while(hir_binary(c"!=",n,hir_int(0,c"u8")),body))
 emit_hir(b)
 emit_hir(b)
}
`

func TestStructuredMetaLoopCallsiteProvenance(t *testing.T) {
	source := structuredLoopDefinition + "fun main()->u8 { @build() @build() return 7 }"
	parse := func() *hir.Module {
		m, e := Parse(source, "generated-loop.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return m
	}
	m := parse()
	sites, e := hir.BuildLoopSiteMap(m)
	if e != nil {
		t.Fatal(e)
	}
	other, e := hir.BuildLoopSiteMap(parse())
	if e != nil {
		t.Fatal(e)
	}
	a, _ := json.Marshal(sites)
	b, _ := json.Marshal(other)
	if !bytes.Equal(a, b) || len(sites.Sites) != 4 || sites.UnmappedLoops != 0 {
		t.Fatalf("unstable/incomplete map %s", a)
	}
	offsets := map[int]map[int]bool{}
	ids := map[string]bool{}
	for _, site := range sites.Sites {
		origin := site.Origin
		if ids[site.ID] || origin.Metafunction != "build" || len(origin.MetafunctionSHA256) != 64 || origin.Line != 11 || origin.Unit != "generated-loop.nanz" || !strings.HasPrefix(source[origin.ByteOffset:], "@build()") {
			t.Fatalf("incorrect generated origin %+v", site)
		}
		ids[site.ID] = true
		if offsets[origin.ByteOffset] == nil {
			offsets[origin.ByteOffset] = map[int]bool{}
		}
		offsets[origin.ByteOffset][site.Ordinal] = true
	}
	if len(offsets) != 2 {
		t.Fatal("invocations shared a source offset", offsets)
	}
	for _, ordinals := range offsets {
		if len(ordinals) != 2 || !ordinals[0] || !ordinals[1] {
			t.Fatal("snapshots lost identity", offsets)
		}
	}
	ctx := hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "vm", WorkloadSHA256: strings.Repeat("a", 64), HostsSHA256: strings.Repeat("b", 64), Dependencies: m.SourceUnits}
	profiler, e := hir.NewLoopProfilerWithContext(m, ctx)
	if e != nil {
		t.Fatal(e)
	}
	got, e := profiler.Call("main", nil)
	if e != nil || len(got) != 1 || got[0].I != 7 {
		t.Fatal(got, e)
	}
	profile := profiler.Snapshot()
	if len(profile.Counts) != 4 {
		t.Fatal(profile.Counts)
	}
	for _, count := range profile.Counts {
		if count.Entries != 1 || count.BodyEntries != 3 {
			t.Fatalf("count %+v, want 1 entry / 3 iterations", count)
		}
	}
	encoded, e := json.Marshal(profile)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = hir.LoadLoopProfile(bytes.NewReader(encoded), m, ctx); e != nil {
		t.Fatal("generated profile refused", e)
	}
	for node, origin := range m.SourceLocations {
		if origin.Metafunction != "" {
			origin.MetafunctionSHA256 = strings.Repeat("c", 64)
			m.SourceLocations[node] = origin
			break
		}
	}
	if _, e = hir.LoadLoopProfile(bytes.NewReader(encoded), m, ctx); e == nil {
		t.Fatal("changed definition fingerprint accepted")
	}
	for node, origin := range m.SourceLocations {
		if _, ok := node.(*hir.WhileStmt); ok {
			origin.MetafunctionSHA256 = "bad"
			m.SourceLocations[node] = origin
			break
		}
	}
	if _, e = hir.BuildLoopSiteMap(m); e == nil {
		t.Fatal("malformed generated origin accepted")
	}
}

func TestStructuredMetaImportedDefinitionFingerprint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generator.nanz")
	caller := "import generator\nfun main()->u8 { @build() return 7 }"
	parse := func(def string) *hir.LoopSiteMap {
		if e := os.WriteFile(path, []byte(def), 0600); e != nil {
			t.Fatal(e)
		}
		m, e := ParseWithOpts(caller, "caller.nanz", ParseOpts{BaseDir: dir})
		if e != nil {
			t.Fatal(e)
		}
		sites, e := hir.BuildLoopSiteMap(m)
		if e != nil {
			t.Fatal(e)
		}
		if len(sites.Sites) != 2 || sites.UnmappedLoops != 0 {
			t.Fatal(sites)
		}
		return sites
	}
	before := parse(structuredLoopDefinition)
	after := parse(strings.Replace(structuredLoopDefinition, "let n:u16", "// definition-only edit\n let n:u16", 1))
	for i, site := range before.Sites {
		changed := after.Sites[i]
		if site.ID != changed.ID || site.Origin.SourceSHA256 != changed.Origin.SourceSHA256 || site.Origin.MetafunctionSHA256 == changed.Origin.MetafunctionSHA256 {
			t.Fatal("import edit lost or caller origin changed", site, changed)
		}
	}
}

func TestStructuredMetaNestedTextKeepsUnknownOrigin(t *testing.T) {
	m, e := Parse(structuredLoopDefinition+`fun @outer(){emit(c"@build()")}
fun main()->u8 {@outer() return 7}`, "nested-text.nanz")
	if e != nil {
		t.Fatal(e)
	}
	sites, e := hir.BuildLoopSiteMap(m)
	if e != nil {
		t.Fatal(e)
	}
	if len(sites.Sites) != 0 || sites.UnmappedLoops != 2 {
		t.Fatal("temporary lexer assigned caller offsets", sites)
	}
}
