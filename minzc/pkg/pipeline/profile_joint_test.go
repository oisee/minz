package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

const jointSource = `global calls:u8=0
fun left_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
fun right_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
fun caller(ptr:^u8,n:u8)->u8 {
 var a:u8=left_sum(ptr,n)
 var b:u8=right_sum(ptr,n)
 calls=calls+1
 return a+b
}`

func jointFixture(t *testing.T) (func() (*hir.Module, error), hir.ProfileWorkload, []byte) {
	t.Helper()
	factory := func() (*hir.Module, error) { return nanz.Parse(jointSource, "joint.nanz") }
	n := int64(5)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"array": {1, 2, 3, 4, 5}}, Globals: map[string][]byte{"calls": {9}}, Calls: []hir.ProfileInvocation{{Function: "caller", Arguments: []hir.ProfileArgument{{Resource: "array"}, {Value: &n}}}, {Function: "caller", Arguments: []hir.ProfileArgument{{Resource: "array"}, {Value: &n}}}}}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(run.Profile)
	if err != nil {
		t.Fatal(err)
	}
	return factory, w, data
}

func TestMeasureProfileVariantSetsCaller(t *testing.T) {
	factory, w, profile := jointFixture(t)
	for _, size := range []bool{false, true} {
		t.Run(fmt.Sprintf("size=%v", size), func(t *testing.T) {
			options := Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}
			result, err := MeasureProfileVariantSets(factory, profile, w, options, 4)
			if err != nil {
				t.Fatal(err)
			}
			e := result.Evidence
			if len(e.EligibleSites) != 2 || len(e.Candidates) != 4 {
				t.Fatal("incomplete enumeration", e.EligibleSites, len(e.Candidates))
			}
			for mask, c := range e.Candidates {
				for j, id := range e.EligibleSites {
					want := "original"
					if mask&(1<<(1-j)) != 0 {
						want = "pointer-walk"
					}
					found := false
					for _, d := range c.Decisions {
						if d.SiteID == id {
							found = true
							if d.Variant != want {
								t.Fatalf("mask %d site %s: %s", mask, id, d.Variant)
							}
						}
					}
					if !found {
						t.Fatal("missing site")
					}
				}
				if !jointSameState(c.RawRun, e.Oracle) || !jointEqual([][]mir2.Value{c.Run.Invocations[0].Results, c.Run.Invocations[1].Results}, e.Oracle.Results) {
					t.Fatal("semantic mismatch")
				}
				if !jointEqual(c.Run.Globals, e.Oracle.Globals) || !jointEqual(c.Run.Resources, e.Oracle.Resources) {
					t.Fatal("target memory mismatch")
				}
				if len(c.Run.Invocations) != 2 || c.TotalTStates <= 0 || c.ModuleBytes != len(c.Binary) {
					t.Fatal("missing whole-workload measurement")
				}
				if c.TotalTStates != c.Run.Invocations[0].TStates+c.Run.Invocations[1].TStates {
					t.Fatal("cost not entry-through-RET total")
				}
				if c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) {
					t.Fatal("binary hash mismatch")
				}
				t.Logf("set %d: measured %d T-states, %d module bytes", mask, c.TotalTStates, c.ModuleBytes)
			}
			best := 0
			for i := 1; i < 4; i++ {
				better := e.Candidates[i].TotalTStates < e.Candidates[best].TotalTStates
				if size {
					better = e.Candidates[i].ModuleBytes < e.Candidates[best].ModuleBytes
				}
				if better {
					best = i
				}
			}
			if e.Selected != best || !bytes.Equal(result.Binary, e.Candidates[best].Binary) || !jointEqual(result.Decisions, result.Steps.ProfileDecisions) {
				t.Fatal("selected outputs mismatch")
			}
			assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + result.Steps.Assembly)
			if err != nil || len(assembled.Errors) > 0 || !bytes.Equal(assembled.Binary, result.Binary) {
				t.Fatal("selected steps differ from measured binary", err)
			}
			copyEvidence := *e
			copyEvidence.SHA256 = ""
			data, err := json.Marshal(copyEvidence)
			if err != nil {
				t.Fatal(err)
			}
			if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
				t.Fatal("evidence self-excluded hash mismatch")
			}
			key, err := ProfileCompilationID(options, "default")
			if err != nil || key != e.CompilationID {
				t.Fatal("missing compilation key", err)
			}
			if e.Oracle.Results[0][0].I != 30 || e.Oracle.Globals["calls"][0] != 11 || !bytes.Equal(e.Oracle.Resources["array"], w.Resources["array"]) {
				t.Fatal("unexpected caller semantics")
			}
		})
	}
}

func TestMeasureProfileVariantSetsRefusals(t *testing.T) {
	factory, w, profile := jointFixture(t)
	options := Options{AssertMode: "none", UseHIRGrace: true}
	cases := []struct {
		name     string
		factory  func() (*hir.Module, error)
		data     []byte
		cap      int
		contains string
	}{
		{"invalid-profile", factory, []byte(`{}`), 4, "profile"},
		{"cap", factory, profile, 1, "exceed cap"},
		{"oversized-cap", factory, profile, 5, "site cap"},
		{"zero-cap", factory, profile, 0, "site cap"},
		{"nil-factory", nil, profile, 4, "factory"},
		{"nil-module", func() (*hir.Module, error) { return nil, nil }, profile, 4, "fresh input"},
		{"factory-error", func() (*hir.Module, error) { return nil, fmt.Errorf("factory broken") }, profile, 4, "factory broken"},
	}
	shared, _ := factory()
	cases = append(cases, struct {
		name     string
		factory  func() (*hir.Module, error)
		data     []byte
		cap      int
		contains string
	}{"shared-module", func() (*hir.Module, error) { return shared, nil }, profile, 4, "shared HIR"})
	sharedAST, _ := factory()
	sharedCalls := 0
	cases = append(cases, struct {
		name     string
		factory  func() (*hir.Module, error)
		data     []byte
		cap      int
		contains string
	}{"shared-AST", func() (*hir.Module, error) {
		sharedCalls++
		if sharedCalls == 1 {
			return sharedAST, nil
		}
		m, err := factory()
		if err == nil {
			m.Funcs[0] = sharedAST.Funcs[0]
		}
		return m, err
	}, profile, 4, "shared HIR"})
	calls := 0
	cases = append(cases, struct {
		name     string
		factory  func() (*hir.Module, error)
		data     []byte
		cap      int
		contains string
	}{"stale-source", func() (*hir.Module, error) {
		calls++
		m, err := factory()
		if calls > 1 {
			m.SourceUnits["joint.nanz"] = strings.Repeat("0", 64)
		}
		return m, err
	}, profile, 4, "stale dependency"})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := MeasureProfileVariantSets(c.factory, c.data, w, options, c.cap)
			if out != nil || err == nil || !strings.Contains(err.Error(), c.contains) {
				t.Fatalf("out=%v error=%v", out, err)
			}
		})
	}
}

func TestMeasureProfileVariantSetsUnobservedStayOriginal(t *testing.T) {
	_, w, _ := jointFixture(t)
	source := jointSource + `\nfun unused_sum(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}`
	source = strings.ReplaceAll(source, `\nfun`, "\nfun")
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "joint-unused.nanz") }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := json.Marshal(run.Profile)
	if err != nil {
		t.Fatal(err)
	}
	result, err := MeasureProfileVariantSets(factory, profile, w, Options{AssertMode: "none", UseHIRGrace: true}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Evidence.Candidates) != 4 {
		t.Fatal("unobserved site enumerated")
	}
	var unused string
	for _, s := range run.Profile.Sites.Sites {
		if s.Function == "unused_sum" {
			unused = s.ID
		}
	}
	if unused == "" {
		t.Fatal("missing unused site")
	}
	for _, c := range result.Evidence.Candidates {
		found := false
		for _, d := range c.Decisions {
			if d.SiteID == unused {
				found = true
				if d.Variant != "original" {
					t.Fatal("unobserved site rewritten")
				}
			}
		}
		if !found {
			t.Fatal("missing sticky original decision")
		}
	}
}

func TestMeasureProfileVariantSetsRefusesMoreThanFour(t *testing.T) {
	source := ""
	calls := ""
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("sum%d", i)
		source += fmt.Sprintf(`fun %s(ptr:^u8,n:u8)->u8 {
 var s:u8=0
 var i:u8=0
 while i<n {s=s+ptr[i]
 i=i+1}
 return s
}
`, name)
		calls += fmt.Sprintf(" var s%d:u8=%s(ptr,n)\n", i, name)
	}
	source += "fun caller(ptr:^u8,n:u8)->u8 {\n" + calls + " return s0+s1+s2+s3+s4\n}"
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "five.nanz") }
	_, w, _ := jointFixture(t)
	w.Globals = nil
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	run, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := json.Marshal(run.Profile)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	counted := func() (*hir.Module, error) { count++; return factory() }
	result, err := MeasureProfileVariantSets(counted, profile, w, Options{AssertMode: "none"}, 4)
	if result != nil || err == nil || !strings.Contains(err.Error(), "5 observed eligible sites exceed cap 4") || count != 1 {
		t.Fatalf("result=%v err=%v factory calls=%d", result, err, count)
	}
}

func TestMeasureProfileVariantSetsRejectsForgedCounts(t *testing.T) {
	factory, w, profile := jointFixture(t)
	var p hir.LoopProfile
	if err := json.Unmarshal(profile, &p); err != nil {
		t.Fatal(err)
	}
	for i := range p.Counts {
		p.Counts[i].Entries = 0
		p.Counts[i].BodyEntries = 0
	}
	for i := range p.Guards {
		p.Guards[i] = hir.GuardCounts{SiteID: p.Guards[i].SiteID}
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	out, err := MeasureProfileVariantSets(factory, data, w, Options{AssertMode: "none"}, 4)
	if out != nil || err == nil || !strings.Contains(err.Error(), "actual controlled workload") {
		t.Fatalf("out=%v err=%v", out, err)
	}
}

func TestMeasureProfileVariantSetsFactoryMutatesPrevious(t *testing.T) {
	factory, w, profile := jointFixture(t)
	var first *hir.Module
	calls := 0
	malicious := func() (*hir.Module, error) {
		calls++
		m, err := factory()
		if calls == 1 {
			first = m
		}
		if calls == 3 {
			first.Funcs[0].Name = "changed_sum"
		}
		return m, err
	}
	result, err := MeasureProfileVariantSets(malicious, profile, w, Options{AssertMode: "none"}, 4)
	if result != nil || err == nil || !strings.Contains(err.Error(), "factory mutated previously returned") {
		t.Fatalf("result=%v err=%v", result, err)
	}
}
