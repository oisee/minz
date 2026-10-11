package hir_test

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

const mixedCopyRegionSource = regionSource + `
fun copy_forward(dst:^u8,src:^u8,n:u8)->u8 {var i:u8=0
while i<n {dst[i]=src[i]
i=i+1}
return i}
fun ordinary(n:u8)->u8 {var i:u8=0
while i<n {i=i+1}
return i}
fun same_a()->u8 {return 7}
fun same_b()->u8 {return 7}`

func mixedCopyRegionModule(t *testing.T) (*hir.Module, *hir.ProfileCompatibility, string, string) {
	t.Helper()
	m, e := nanz.Parse(mixedCopyRegionSource, "mixed.nanz")
	if e != nil {
		t.Fatal(e)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(mixedCopyRegionSource)))
	c, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "test", WorkloadSHA256: hash, HostsSHA256: hash, Dependencies: map[string]string{"mixed.nanz": hash}})
	if e != nil {
		t.Fatal(e)
	}
	copies, regions, e := hir.MixedCopyRegionSites(m)
	if e != nil || len(copies) != 1 || len(regions) != 1 {
		t.Fatal(copies, regions, e)
	}
	return m, c, copies[0], regions[0]
}
func TestMixedCopyRegionFourChoices(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		source, _, copyID, regionID := mixedCopyRegionModule(t)
		actual, c, _, _ := mixedCopyRegionModule(t)
		choices := hir.MixedCopyRegionChoices{Copy: map[string]bool{copyID: mask&1 != 0}, Region: map[string]bool{regionID: mask&2 != 0}}
		if e := hir.ApplyMixedCopyRegionChoices(actual, c, choices); e != nil {
			t.Fatal(mask, e)
		}
		proof, e := hir.ValidateMixedCopyRegionChoices(source, actual, choices)
		if e != nil || len(proof.Copies) != 1 || len(proof.Regions) != 1 {
			t.Fatal(mask, proof, e)
		}
		if _, e = hir.ValidateRegionCompareChoices(source, actual, choices.Region); e == nil {
			t.Fatal("standalone region accepted combined policies")
		}
	}
}
func TestMixedCopyRegionProofRefusals(t *testing.T) {
	for _, name := range []string{"copyrule", "regionrule", "ordinarymarker", "nonloop", "policy", "disabledcopy", "guard", "metadata", "origin_nonloop_swap"} {
		t.Run(name, func(t *testing.T) {
			source, _, copyID, regionID := mixedCopyRegionModule(t)
			actual, c, _, _ := mixedCopyRegionModule(t)
			choices := hir.MixedCopyRegionChoices{Copy: map[string]bool{copyID: true}, Region: map[string]bool{regionID: true}}
			if name == "origin_nonloop_swap" {
				for _, fn := range []string{"same_a", "same_b"} {
					s := source.FuncByName(fn).Body.Body[0]
					a := actual.FuncByName(fn).Body.Body[0]
					start := strings.Index(mixedCopyRegionSource, "fun "+fn)
					offset := start + strings.Index(mixedCopyRegionSource[start:], "return 7")
					before := mixedCopyRegionSource[:offset]
					line := strings.Count(before, "\n") + 1
					column := offset - strings.LastIndex(before, "\n")
					loc := hir.SourceLocation{Unit: "mixed.nanz", ByteOffset: offset, Line: line, Column: column, SourceSHA256: source.SourceUnits["mixed.nanz"]}
					source.SourceLocations[s] = loc
					actual.SourceLocations[a] = loc
				}
				var e error
				c, e = hir.BuildProfileCompatibility(actual, c.Context)
				if e != nil {
					t.Fatal(e)
				}
			}
			if e := hir.ApplyMixedCopyRegionChoices(actual, c, choices); e != nil {
				t.Fatal(e)
			}
			switch name {
			case "copyrule", "regionrule":
				for stmt, l := range actual.LoopLineage {
					if (name == "copyrule" && l.SiteID == copyID) || (name == "regionrule" && l.SiteID == regionID) {
						l.Rule = "wrong"
						actual.LoopLineage[stmt] = l
						break
					}
				}
			case "ordinarymarker":
				s := actual.FuncByName("ordinary").Body.Body[1]
				delete(actual.LoopLineage, s)
			case "nonloop":
				s := actual.FuncByName("ordinary").Body.Body[2]
				actual.LoopLineage[s] = hir.LoopLineage{SiteID: copyID, Role: "fast", Rule: hir.ForwardCopyRule}
			case "policy":
				actual.RegionComparePolicy["unknown"] = false
			case "disabledcopy":
				choices.Copy[copyID] = false
			case "guard":
				actual.FuncByName("copy_forward").Body.Body[1].(*hir.IfStmt).Cond = hir.U8(1)
			case "metadata":
				actual.Strings = append(actual.Strings, "forged")
			case "origin_nonloop_swap":
				a, b := actual.FuncByName("same_a").Body.Body[0], actual.FuncByName("same_b").Body.Body[0]
				actual.SourceLocations[a], actual.SourceLocations[b] = actual.SourceLocations[b], actual.SourceLocations[a]
			}
			if _, e := hir.ValidateMixedCopyRegionChoices(source, actual, choices); e == nil {
				t.Fatal("forged combined proof accepted")
			}
		})
	}
}
func TestMixedCopyRegionFreshKeys(t *testing.T) {
	for _, name := range []string{"false_unknown", "wrong_kind", "prior", "stale"} {
		t.Run(name, func(t *testing.T) {
			m, c, copyID, regionID := mixedCopyRegionModule(t)
			choices := hir.MixedCopyRegionChoices{Copy: map[string]bool{copyID: true}, Region: map[string]bool{regionID: true}}
			switch name {
			case "false_unknown":
				choices.Region["unknown"] = false
			case "wrong_kind":
				choices.Copy[regionID] = false
			case "prior":
				m.RegionComparePolicy = map[string]bool{}
			case "stale":
				m.Globals = append(m.Globals, mir2.Global{Name: "new", Ty: mir2.TyU8})
			}
			before := m.Dump()
			if e := hir.ApplyMixedCopyRegionChoices(m, c, choices); e == nil || m.LoopLineage != nil || m.Dump() != before {
				t.Fatal("invalid input accepted or mutated")
			}
		})
	}
}

func TestMixedCopyRegionDetachedOriginalOrigin(t *testing.T) {
	source, _, copyID, regionID := mixedCopyRegionModule(t)
	actual, c, _, _ := mixedCopyRegionModule(t)
	choices := hir.MixedCopyRegionChoices{Copy: map[string]bool{copyID: true}, Region: map[string]bool{regionID: true}}
	if e := hir.ApplyMixedCopyRegionChoices(actual, c, choices); e != nil {
		t.Fatal(e)
	}
	source.SourceLocations[&hir.ReturnStmt{Val: hir.U8(7)}] = hir.SourceLocation{Unit: "mixed.nanz", ByteOffset: 999, Line: 1, Column: 1, SourceSHA256: source.SourceUnits["mixed.nanz"]}
	if _, e := hir.ValidateMixedCopyRegionChoices(source, actual, choices); e == nil {
		t.Fatal("detached original origin accepted")
	}
}

func TestMixedCopyRegionNoKernelOriginProof(t *testing.T) {
	const text = "fun a()->u8 {return 7}\nfun b()->u8 {return 7}"
	factory := func() (*hir.Module, *hir.ProfileCompatibility) {
		m, e := nanz.Parse(text, "plain.nanz")
		if e != nil {
			t.Fatal(e)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
		for _, fn := range []string{"a", "b"} {
			start := strings.Index(text, "fun "+fn)
			offset := start + strings.Index(text[start:], "return 7")
			before := text[:offset]
			m.SourceLocations[m.FuncByName(fn).Body.Body[0]] = hir.SourceLocation{Unit: "plain.nanz", ByteOffset: offset, Line: strings.Count(before, "\n") + 1, Column: offset - strings.LastIndex(before, "\n"), SourceSHA256: hash}
		}
		c, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "test", WorkloadSHA256: hash, HostsSHA256: hash, Dependencies: map[string]string{"plain.nanz": hash}})
		if e != nil {
			t.Fatal(e)
		}
		return m, c
	}
	for _, swap := range []bool{false, true} {
		source, _ := factory()
		actual, c := factory()
		choices := hir.MixedCopyRegionChoices{Copy: map[string]bool{}, Region: map[string]bool{}}
		if e := hir.ApplyMixedCopyRegionChoices(actual, c, choices); e != nil {
			t.Fatal(e)
		}
		if swap {
			a, b := actual.FuncByName("a").Body.Body[0], actual.FuncByName("b").Body.Body[0]
			actual.SourceLocations[a], actual.SourceLocations[b] = actual.SourceLocations[b], actual.SourceLocations[a]
		}
		_, e := hir.ValidateMixedCopyRegionChoices(source, actual, choices)
		if swap == (e == nil) {
			t.Fatal("no-kernel origin proof", swap, e)
		}
	}
}
