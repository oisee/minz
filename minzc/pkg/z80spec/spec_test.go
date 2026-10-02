package z80spec_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/lir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80spec"
	"github.com/minz/minzc/pkg/z80spec/mir2loc"
)

func TestNamespaceRoundTrips(t *testing.T) {
	for _, ns := range []struct {
		name  string
		names []string
		from  func(int) (z80spec.Location, bool)
		to    func(string) (int, bool)
	}{
		{"LIR", lirNames(), z80spec.FromLIR, z80spec.ToLIR},
		{"VIR", virNames(), z80spec.FromVIR, z80spec.ToVIR},
	} {
		for i, name := range ns.names {
			loc, ok := ns.from(i)
			want := strings.Replace(name, "_vir_mem", "mem", 1)
			if !ok || loc.Name != want {
				t.Fatalf("%s[%d] = %v, want %s", ns.name, i, loc, want)
			}
			back, ok := ns.to(loc.Name)
			if !ok || back != i {
				t.Fatalf("%s round trip %s: %d", ns.name, name, back)
			}
		}
		for _, i := range []int{-1, len(ns.names), 100} {
			if _, ok := ns.from(i); ok {
				t.Errorf("%s accepted %d", ns.name, i)
			}
		}
	}
	for _, phys := range mir2.Z80PhysLocs {
		loc, ok := mir2loc.From(phys)
		if !ok {
			t.Fatalf("unmapped MIR2: %+v", phys)
		}
		back, ok := mir2loc.To(loc.Name)
		if !ok || back != phys {
			t.Fatalf("MIR2 round trip %+v => %v => %+v", phys, loc, back)
		}
	}
	for _, loc := range z80spec.Locations() {
		for _, ns := range []struct {
			to   func(string) (int, bool)
			from func(int) (z80spec.Location, bool)
		}{
			{z80spec.ToLIR, z80spec.FromLIR}, {z80spec.ToVIR, z80spec.FromVIR}, {z80spec.ToGPU, z80spec.FromGPU},
		} {
			if i, ok := ns.to(loc.Name); ok {
				back, valid := ns.from(i)
				if !valid || back != loc {
					t.Errorf("spec round trip %v => %v", loc, back)
				}
			}
		}
		if phys, ok := mir2loc.To(loc.Name); ok {
			back, valid := mir2loc.From(phys)
			if !valid || back != loc {
				t.Errorf("MIR2 spec round trip %v", loc)
			}
		}
	}
	if _, ok := mir2loc.From(mir2.PhysLoc{Kind: mir2.LocReg, Name: "IX"}); ok {
		t.Error("accepted incorrect MIR2 kind")
	}
	if _, ok := mir2loc.From(mir2.PhysLoc{Kind: mir2.LocMem, Name: "mem", Offset: 1}); ok {
		t.Error("accepted dynamic offset")
	}
	if _, ok := z80spec.ToLIR("stk0"); ok {
		t.Error("LIR has no stack slot")
	}
	if _, ok := z80spec.ToVIR("gpu_mem"); ok {
		t.Error("GPU spill has no VIR equivalent")
	}
	for _, from := range []func(int) (z80spec.Location, bool){z80spec.FromGPU} {
		for _, i := range []int{-1, 15, 100} {
			if _, ok := from(i); ok {
				t.Errorf("GPU accepted %d", i)
			}
		}
	}
}
func lirNames() []string {
	var names []string
	for _, loc := range lir.Z80.Locs {
		names = append(names, loc.Name)
	}
	return names
}
func virNames() []string {
	var names []string
	for _, loc := range vir.Z80.Locs {
		names = append(names, loc.Name)
	}
	return names
}

func TestSharedIndices(t *testing.T) {
	if len(lir.Z80.Locs) != 37 || len(vir.Z80.Locs) != 41 {
		t.Fatal("namespace size changed")
	}
	for i := 0; i <= 32; i++ {
		a, b := lir.Z80.Locs[i], vir.Z80.Locs[i]
		if a.Name != b.Name || a.Width != b.Width {
			t.Errorf("index %d: LIR %+v vs VIR %+v", i, a, b)
		}
	}
}

func TestOverlapProperties(t *testing.T) {
	names := map[string]bool{}
	for _, a := range z80spec.Locations() {
		if names[a.Name] {
			t.Errorf("duplicate %s", a.Name)
		}
		names[a.Name] = true
		if !z80spec.Overlaps(a.Name, a.Name) {
			t.Errorf("not reflexive: %s", a.Name)
		}
		if z80spec.Width(a.Name) != a.Bits {
			t.Errorf("width: %s", a.Name)
		}
		for _, b := range z80spec.Locations() {
			if z80spec.Overlaps(a.Name, b.Name) != z80spec.Overlaps(b.Name, a.Name) {
				t.Errorf("asymmetric: %s/%s", a.Name, b.Name)
			}
		}
		if hi, lo, ok := z80spec.Halves(a.Name); ok {
			if !z80spec.Overlaps(a.Name, hi) || !z80spec.Overlaps(a.Name, lo) || z80spec.Overlaps(hi, lo) {
				t.Errorf("halves: %s", a.Name)
			}
			for _, half := range []string{hi, lo} {
				if parent, ok := z80spec.Parent(half); !ok || parent != a.Name {
					t.Errorf("parent: %s", half)
				}
			}
			if a.Bits != z80spec.Width(hi)+z80spec.Width(lo) {
				t.Errorf("half widths: %s", a.Name)
			}
		}
	}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"B", "BC", true}, {"H", "HL", true}, {"IXH", "IX", true}, {"B", "DE", false},
		{"A", "AF", true}, {"F", "AF", true}, {"H'", "HL32", true}, {"HL", "HL32", true},
		{"HL", "HL'", false}, {"mem0", "mem1", false}, {"mem0", "mem", false}, {"gpu_mem", "mem0", false},
		{"unknown", "unknown", false},
	} {
		if got := z80spec.Overlaps(tc.a, tc.b); got != tc.want {
			t.Errorf("overlap %s/%s = %v", tc.a, tc.b, got)
		}
	}
	if z80spec.Width("unknown") != 0 {
		t.Error("unknown width")
	}
	if _, ok := z80spec.Parent(""); ok {
		t.Error("empty parent")
	}
	if _, _, ok := z80spec.Halves("SP"); ok {
		t.Error("SP has no modelled halves")
	}
}

// Read unexported VIR tables directly from their Go initializers. This checks
// the production definitions without linkname, copied tables or production edits.
func initializer(t *testing.T, file, name string) *ast.CompositeLit {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "vir", file), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok {
			for _, s := range g.Specs {
				if v, ok := s.(*ast.ValueSpec); ok {
					for i, n := range v.Names {
						if n.Name == name {
							lit, ok := v.Values[i].(*ast.CompositeLit)
							if !ok {
								t.Fatalf("%s initializer changed", name)
							}
							return lit
						}
					}
				}
			}
		}
	}
	t.Fatalf("missing %s", name)
	return nil
}
func integer(t *testing.T, e ast.Expr) int {
	t.Helper()
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.SUB {
		return -integer(t, u.X)
	}
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		t.Fatalf("expected literal, got %T", e)
	}
	n, err := strconv.Atoi(lit.Value)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGPUTables(t *testing.T) {
	forward := initializer(t, "gpu.go", "z80ToGPULoc")
	reverse := initializer(t, "regalloc_table.go", "gpuToZ80Loc")
	if len(reverse.Elts) != 15 || len(forward.Elts) != 14 {
		t.Fatal("GPU namespace changed")
	}
	seen := map[int]bool{}
	for _, e := range forward.Elts {
		kv := e.(*ast.KeyValueExpr)
		v, g := integer(t, kv.Key), integer(t, kv.Value)
		spec, ok := z80spec.FromGPU(g)
		canonical, valid := z80spec.FromVIR(v)
		if !ok || !valid || spec != canonical {
			t.Errorf("GPU %d vs VIR %d", g, v)
		}
		if seen[g] {
			t.Errorf("duplicate GPU index %d", g)
		}
		seen[g] = true
	}
	for g, e := range reverse.Elts {
		v := integer(t, e)
		spec, ok := z80spec.FromGPU(g)
		if !ok {
			t.Fatalf("unmapped GPU %d", g)
		}
		back, ok := z80spec.ToGPU(spec.Name)
		if !ok || back != g {
			t.Errorf("GPU round trip %d", g)
		}
		if g == 14 {
			if v != -1 || spec.Name != "gpu_mem" {
				t.Error("GPU spill sentinel changed")
			}
			continue
		}
		canonical, ok := z80spec.FromVIR(v)
		if !ok || canonical != spec || !seen[g] {
			t.Errorf("GPU reverse %d => %d", g, v)
		}
	}
}

func TestVIRPairAliases(t *testing.T) {
	actual := map[string][2]string{}
	for _, e := range initializer(t, "vir.go", "pairAliases").Elts {
		lit := e.(*ast.CompositeLit)
		if len(lit.Elts) != 3 {
			t.Fatal("pairAlias format changed")
		}
		pair, ok := z80spec.FromVIR(integer(t, lit.Elts[0]))
		if !ok {
			t.Fatal("unknown pair")
		}
		hi, h := z80spec.FromVIR(integer(t, lit.Elts[1]))
		lo, l := z80spec.FromVIR(integer(t, lit.Elts[2]))
		wantHi, wantLo, valid := z80spec.Halves(pair.Name)
		if !h || !l || !valid || hi.Name != wantHi || lo.Name != wantLo {
			t.Fatalf("VIR pair disagrees: %s", pair.Name)
		}
		if _, exists := actual[pair.Name]; exists {
			t.Fatal("duplicate pair")
		}
		actual[pair.Name] = [2]string{hi.Name, lo.Name}
	}
	// Known VIR gap: pairAliases omits both index registers. A future fix must
	// remove these entries, so newly fixed or newly missing relations fail.
	known := map[string][2]string{"IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}
	missing := map[string][2]string{}
	for _, loc := range vir.Z80.Locs {
		hi, lo, ok := z80spec.Halves(loc.Name)
		if !ok {
			continue
		}
		if _, found := actual[loc.Name]; !found {
			missing[loc.Name] = [2]string{hi, lo}
		}
	}
	if !reflect.DeepEqual(missing, known) {
		t.Errorf("VIR missing aliases: %v, known %v", missing, known)
	}
}

func TestMIR2KnownAliasGaps(t *testing.T) {
	// Directed gaps: PBQP has no IX/IY containment edges; shadow bytes omit
	// their DWord container even though DWord -> shadow edges are present.
	known := map[string][]string{
		"IX": {"IXH", "IXL"}, "IXH": {"IX"}, "IXL": {"IX"},
		"IY": {"IYH", "IYL"}, "IYH": {"IY"}, "IYL": {"IY"},
		"B'": {"BC32"}, "C'": {"BC32"}, "D'": {"DE32"}, "E'": {"DE32"}, "H'": {"HL32"}, "L'": {"HL32"},
	}
	visited := map[string]bool{}
	for _, a := range mir2.Z80PhysLocs {
		spec, valid := mir2loc.From(a)
		if !valid {
			t.Fatalf("unmapped MIR2 location: %+v", a)
		}
		visited[spec.Name] = true
		t.Run(spec.Name, func(t *testing.T) {
			aliases := mir2.PhysicalAliasesForTest(a)
			got := map[mir2.PhysLoc]bool{}
			for _, b := range aliases {
				if got[b] {
					t.Errorf("duplicate alias %+v", b)
				}
				got[b] = true
				other, ok := mir2loc.From(b)
				if !ok {
					t.Errorf("unmapped alias %+v", b)
				} else if a == b || !z80spec.Overlaps(spec.Name, other.Name) {
					t.Errorf("unexpected alias %s -> %s", spec.Name, other.Name)
				}
			}
			for _, b := range mir2.Z80PhysLocs {
				other, _ := mir2loc.From(b)
				want := a != b && z80spec.Overlaps(spec.Name, other.Name)
				isKnown := false
				for _, name := range known[spec.Name] {
					if name == other.Name {
						isKnown = true
					}
				}
				differs := got[b] != want
				if differs != isKnown {
					t.Errorf("%s -> %s: MIR2 %v spec %v known gap %v", spec.Name, other.Name, got[b], want, isKnown)
				}
			}
		})
	}
	for name, targets := range known {
		if !visited[name] {
			t.Errorf("known-gap key no longer visited: %s", name)
		}
		for _, target := range targets {
			if !visited[target] {
				t.Errorf("known-gap target no longer visited: %s -> %s", name, target)
			}
		}
	}
}

func TestLIRKnownEmptyAliasGaps(t *testing.T) {
	// Known LIR gap: every Loc.Alias is empty. Explicitly list each missing
	// non-self relation; fixes require updating this list instead of passing silently.
	known := map[string][]string{
		"B": {"BC"}, "C": {"BC"}, "BC": {"B", "C"},
		"D": {"DE"}, "E": {"DE"}, "DE": {"D", "E"},
		"H": {"HL"}, "L": {"HL"}, "HL": {"H", "L"},
		"IX": {"IXH", "IXL"}, "IXH": {"IX"}, "IXL": {"IX"},
		"IY": {"IYH", "IYL"}, "IYH": {"IY"}, "IYL": {"IY"},
	}
	for i, a := range lir.Z80.Locs {
		if a.Alias != 0 {
			t.Errorf("known empty LIR Alias changed: %s = %x", a.Name, a.Alias)
		}
		for j, b := range lir.Z80.Locs {
			want := i != j && z80spec.Overlaps(a.Name, b.Name)
			isKnown := false
			for _, name := range known[a.Name] {
				if name == b.Name {
					isKnown = true
				}
			}
			if (a.Alias.Has(j) != want) != isKnown {
				t.Errorf("LIR %s -> %s: alias %v spec %v known %v", a.Name, b.Name, a.Alias.Has(j), want, isKnown)
			}
		}
	}
}

func TestDocumentedWidthsAndKinds(t *testing.T) {
	for _, ns := range []struct {
		name      string
		flagWidth int
		hlIndex   bool
	}{
		{"LIR", lir.Z80.Locs[13].Width, lir.Z80.Locs[9].Kind == lir.LocIndex},
		{"VIR", vir.Z80.Locs[13].Width, vir.Z80.Locs[9].Kind == vir.LocIndex},
	} {
		if ns.flagWidth != 1 || z80spec.Width("F") != 8 {
			t.Errorf("%s F width disagreement changed", ns.name)
		}
		hl, _ := z80spec.Lookup("HL")
		if !ns.hlIndex || hl.Kind != z80spec.Pair16 {
			t.Errorf("%s HL kind disagreement changed", ns.name)
		}
	}
	for i := 14; i <= 17; i++ {
		loc, _ := z80spec.FromLIR(i)
		if lir.Z80.Locs[i].Kind != lir.LocIndex || vir.Z80.Locs[i].Kind != vir.LocIXHalf || loc.Kind != z80spec.IndexHalf {
			t.Errorf("index-half kind disagreement changed: %s", loc.Name)
		}
	}
	for _, tc := range []struct {
		name string
		bits int
		kind z80spec.Kind
	}{
		{"SP", 16, z80spec.StackPointer}, {"I", 8, z80spec.Special8},
		{"R", 8, z80spec.Refresh}, {"PC", 16, z80spec.ProgramCounter},
	} {
		loc, ok := z80spec.Lookup(tc.name)
		if !ok || loc.Bits != tc.bits || loc.Kind != tc.kind {
			t.Errorf("location %s: %+v", tc.name, loc)
		}
	}
}

func TestContainmentAndAliases(t *testing.T) {
	for _, a := range z80spec.Locations() {
		seen := map[string]bool{}
		for _, alias := range z80spec.Aliases(a.Name) {
			if seen[alias.Name] {
				t.Errorf("duplicate alias %s", alias.Name)
			}
			seen[alias.Name] = true
		}
		for _, b := range z80spec.Locations() {
			if seen[b.Name] != z80spec.Overlaps(a.Name, b.Name) {
				t.Errorf("aliases %s/%s", a.Name, b.Name)
			}
			if z80spec.Contains(a.Name, b.Name) && (!seen[b.Name] || a.Bits < b.Bits) {
				t.Errorf("containment %s/%s", a.Name, b.Name)
			}
		}
		if !z80spec.Contains(a.Name, a.Name) {
			t.Errorf("containment not reflexive: %s", a.Name)
		}
		if hi, lo, ok := z80spec.Halves(a.Name); ok {
			for _, half := range []string{hi, lo} {
				if !z80spec.Contains(a.Name, half) || z80spec.Contains(half, a.Name) {
					t.Errorf("directional containment %s/%s", a.Name, half)
				}
			}
		}
	}
	for _, pair := range []string{"BC", "DE", "HL"} {
		if !z80spec.Contains(pair+"32", pair) || !z80spec.Contains(pair+"32", pair+"'") || z80spec.Contains(pair, pair+"32") {
			t.Errorf("DWord containment %s", pair)
		}
	}
	if z80spec.Contains("BC", "DE") || z80spec.Contains("unknown", "unknown") || z80spec.Contains("BC", "unknown") || z80spec.Contains("unknown", "BC") || len(z80spec.Aliases("unknown")) != 0 {
		t.Error("invalid containment/aliases")
	}
	aliases := z80spec.Aliases("BC")
	aliases[0].Name = "changed"
	if z80spec.Aliases("BC")[0].Name == "changed" {
		t.Error("aliases returned shared storage")
	}
}
