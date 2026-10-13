package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

// The same unedited source supplies zero, one or two observed legal loops.
const invariantSitesSource = `fun first(a:u16,b:u16,n:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n { s=s+(a+b)
 i=i+1 }
 return s
}
fun second(a:u16,b:u16,n:u8)->u16 {
 var s:u16=0
 var i:u8=0
 while i<n { s=s+(a+b)
 i=i+1 }
 return s
}
fun idle()->u16 { return 7 }
`

func invariantSitesFixture(t *testing.T, observed int, zeroOnly bool) (func() (*hir.Module, error), hir.ProfileWorkload, []byte) {
	t.Helper()
	factory := func() (*hir.Module, error) { return nanz.Parse(invariantSitesSource, "invariant-sites.nanz") }
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 200000, MaxMemory: 65536}
	if observed == 0 {
		w.Calls = []hir.ProfileInvocation{{Function: "idle"}}
	} else {
		lengths := []int64{0, 1, 5, 255}
		if zeroOnly {
			lengths = lengths[:1]
		}
		for _, n := range lengths {
			for _, name := range []string{"first", "second"}[:observed] {
				w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{literal(65535), literal(2), literal(n)}})
			}
		}
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	legal, err := hir.EligibleInvariantBaseSites(m)
	if err != nil || len(legal) != 2 {
		t.Fatalf("source proof: %v %v", legal, err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	return factory, w, p
}

func TestInvariantSiteVariantSets(t *testing.T) {
	for _, observed := range []int{0, 1, 2} {
		for _, zeroOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("sites%d/zero%v", observed, zeroOnly), func(t *testing.T) {
				factory, w, p := invariantSitesFixture(t, observed, zeroOnly)
				for _, size := range []bool{false, true} {
					r, err := MeasureInvariantSiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
					if err != nil {
						t.Fatal(err)
					}
					e := r.Evidence
					if e.Schema != "minz.invariant-site-variant-sets/v1" || len(e.EligibleSites) != observed || len(e.Candidates) != 1<<observed {
						t.Fatal("incomplete bounded enumeration")
					}
					best := 0
					for mask, c := range e.Candidates {
						if len(c.RequestedSitePolicy) != observed || len(c.RulePolicy) != 2 || c.RulePolicy["hir-constant-if"] || !c.RulePolicy["hir-invariant-base"] {
							t.Fatal("missing explicit policy", mask)
						}
						for j, id := range e.EligibleSites {
							if c.RequestedSitePolicy[id] != (mask&(1<<(observed-1-j)) != 0) {
								t.Fatal("wrong site request", mask, id)
							}
						}
						if err := invariantSitesRealized(c.Lineage, c.RequestedSitePolicy); err != nil {
							t.Fatal(err)
						}
						if err := invariantSitesRealized(c.FinalLineage, c.RequestedSitePolicy); err != nil {
							t.Fatal(err)
						}
						if c.FinalHIR == "" || !jointSameState(c.RawRun, e.Oracle) || !jointSameState(c.FinalRawRun, e.Oracle) {
							t.Fatal("missing semantic evidence")
						}
						for _, d := range c.Decisions {
							if d.Variant != "original" {
								t.Fatal("pointer choice enabled")
							}
						}
						if len(c.Binary) == 0 || c.ModuleBytes != len(c.Binary) || c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) || c.Run.BinarySHA256 != c.BinarySHA256 {
							t.Fatal("exact binary mismatch")
						}
						var total int64
						for j, inv := range c.Run.Invocations {
							total += inv.TStates
							want := int64(7)
							if observed > 0 {
								args := w.Calls[j].Arguments
								// Independent modulo-65536 arithmetic, including long trips.
								sum := uint64(*args[0].Value) + uint64(*args[1].Value)
								want = int64(uint16(sum * uint64(*args[2].Value)))
							}
							if len(inv.Results) != 1 || inv.Results[0].I != want {
								t.Fatalf("Z80 oracle: candidate %d call %d got %v want %d", mask, j, inv.Results, want)
							}
						}
						if total != c.TotalTStates {
							t.Fatal("invocation cost mismatch")
						}
						better := c.TotalTStates < e.Candidates[best].TotalTStates
						if size {
							better = c.ModuleBytes < e.Candidates[best].ModuleBytes
						}
						if better {
							best = mask
						}
					}
					if e.Selected != best || !bytes.Equal(r.Binary, e.Candidates[best].Binary) || r.Steps.Assembly == "" {
						t.Fatal("retained winner mismatch")
					}
					if observed == 2 && !zeroOnly && !size {
						input, err := factory()
						if err != nil {
							t.Fatal(err)
						}
						replay, err := RunZ80ProfileWorkload(r.Steps, input, w, *e.Oracle)
						if err != nil || !jointEqual(replay, e.Candidates[best].Run) {
							t.Fatalf("retained Steps replay mismatch: %v", err)
						}
					}
					copied := *e
					copied.SHA256 = ""
					data, err := json.Marshal(copied)
					if err != nil || e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
						t.Fatal("evidence digest mismatch")
					}
				}
			})
		}
	}
}

func TestInvariantSiteVariantSetsRefusals(t *testing.T) {
	factory, w, p := invariantSitesFixture(t, 2, false)
	opts := Options{AssertMode: "none", UseHIRGrace: true}
	refuse := func(name string, f func() (*hir.Module, error), data []byte, cap int, reason string) {
		t.Helper()
		r, err := MeasureInvariantSiteVariantSets(f, data, w, opts, cap)
		if r != nil || err == nil || (reason != "" && !strings.Contains(err.Error(), reason)) {
			t.Fatalf("%s accepted or wrong refusal: %v", name, err)
		}
	}
	for _, cap := range []int{0, 1, 5} {
		refuse("cap", factory, p, cap, "cap")
	}
	var forged hir.LoopProfile
	if err := json.Unmarshal(p, &forged); err != nil {
		t.Fatal(err)
	}
	forged.Counts[0].Entries++
	bad, err := json.Marshal(forged)
	if err != nil {
		t.Fatal(err)
	}
	refuse("forged counts", factory, bad, 4, "observations do not match")
	refuse("stale source", func() (*hir.Module, error) {
		return nanz.Parse(strings.Replace(invariantSitesSource, "return 7", "return 8", 1), "invariant-sites.nanz")
	}, p, 4, "")
	shared, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	refuse("shared", func() (*hir.Module, error) { return shared, nil }, p, 4, "shared")
	refuse("shared child nodes", func() (*hir.Module, error) {
		m, err := factory()
		if err == nil {
			m.Funcs = shared.Funcs
			m.SourceLocations = map[hir.Stmt]hir.SourceLocation{}
			for node, location := range shared.SourceLocations {
				m.SourceLocations[node] = location
			}
		}
		return m, err
	}, p, 4, "shared")
	var prior *hir.Module
	refuse("mutated prior HIR", func() (*hir.Module, error) {
		m, err := factory()
		if prior != nil {
			prior.Funcs[0].Name = "changed"
		}
		prior = m
		return m, err
	}, p, 4, "")
	var retained []*hir.Module
	refuse("mutated prior", func() (*hir.Module, error) {
		m, err := factory()
		if len(retained) > 0 {
			retained[0].InvariantBasePolicy = map[string]bool{}
		}
		retained = append(retained, m)
		return m, err
	}, p, 4, "mutated previously")
	for _, field := range []string{"pointer", "invariant", "rules", "lineage"} {
		refuse(field, func() (*hir.Module, error) {
			m, err := factory()
			if err != nil {
				return nil, err
			}
			switch field {
			case "pointer":
				m.PointerWalkPolicy = map[string]bool{}
			case "invariant":
				m.InvariantBasePolicy = map[string]bool{}
			case "rules":
				m.GraceRulePolicy = map[string]bool{}
			case "lineage":
				err = hir.SeedLoopLineage(m)
			}
			return m, err
		}, p, 4, "fresh input")
	}
}

func TestInvariantSiteVariantSetsCountBoundary(t *testing.T) {
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	for _, count := range []int{4, 5} {
		source := ""
		w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536}
		for i := 0; i < count; i++ {
			name := fmt.Sprintf("scalar%d", i)
			source += fmt.Sprintf("fun %s(a:u16,b:u16,n:u8)->u16 {var s:u16=0\nvar i:u8=0\nwhile i<n {s=s+(a+b)\ni=i+1}\nreturn s}\n", name)
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{literal(65535), literal(2), literal(0)}})
		}
		factoryCalls := 0
		factory := func() (*hir.Module, error) { factoryCalls++; return nanz.Parse(source, "invariant-boundary.nanz") }
		input, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := hir.RunLoopProfile(input, w)
		if err != nil {
			t.Fatal(err)
		}
		p, err := json.Marshal(raw.Profile)
		if err != nil {
			t.Fatal(err)
		}
		factoryCalls = 0
		r, err := MeasureInvariantSiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if count == 5 {
			if r != nil || err == nil || !strings.Contains(err.Error(), "exceed cap") || factoryCalls != 1 {
				t.Fatalf("count cap did not stop before candidates: calls=%d err=%v", factoryCalls, err)
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Evidence.EligibleSites) != 4 || len(r.Evidence.Candidates) != 16 || factoryCalls != 17 {
				t.Fatal("four-site boundary incomplete")
			}
			for _, c := range r.Evidence.Candidates {
				for _, inv := range c.Run.Invocations {
					if inv.Results[0].I != 0 {
						t.Fatal("zero-trip host oracle")
					}
				}
			}
		}
	}
}
