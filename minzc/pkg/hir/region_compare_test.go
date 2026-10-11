package hir_test

import (
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"strings"
	"testing"
)

const regionSource = `fun region_eq(src:^u8,arena:^u8,src_start:u16,arena_ptr:u16,len:u8)->u8 {
var i:u8=0
while i<len {if src[src_start+i]!=arena[arena_ptr+1+i] {return 0}
i=i+1}
return 1}`

func regionModule(t *testing.T) (*hir.Module, *hir.ProfileCompatibility, string) {
	t.Helper()
	m, e := nanz.Parse(regionSource, "region.nanz")
	if e != nil {
		t.Fatal(e)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(regionSource)))
	c, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "vm-test", TargetID: "z80-test", WorkloadSHA256: hash, HostsSHA256: hash, Dependencies: map[string]string{"region.nanz": hash}})
	if e != nil {
		t.Fatal(e)
	}
	ids, e := hir.EligibleRegionCompareSites(m)
	if e != nil || len(ids) != 1 {
		t.Fatal(ids, e)
	}
	return m, c, ids[0]
}
func TestRegionCompareContractRawDomain(t *testing.T) {
	source, _, _ := regionModule(t)
	candidate, c, id := regionModule(t)
	if e := hir.ApplyRegionCompareSiteChoices(candidate, c, map[string]bool{id: true}); e != nil {
		t.Fatal(e)
	}
	if _, e := hir.ValidateRegionCompareChoices(source, candidate, map[string]bool{id: true}); e != nil {
		t.Fatal(e)
	}
	mods := []*mir2.Module{hir.LowerModule(source), hir.LowerModule(candidate)}
	cases := 0
	// All counts and every mismatch position plus complete equality. Disjoint
	// windows use offsets beyond255 so no byte offset assumption can hide errors.
	for n := 0; n < 256; n++ {
		for mismatch := -1; mismatch < n; mismatch++ {
			memory := make([]byte, 65536)
			for j := 0; j < n; j++ {
				memory[513+j] = byte(j*37 + 11)
				memory[2049+j] = memory[513+j]
			}
			if mismatch >= 0 {
				memory[2049+mismatch] ^= 1
			}
			checkRegionCase(t, mods, memory, [5]int64{500, 2000, 13, 48, int64(n)})
			cases++
		}
	}
	// Original6144boundary-domain, preserving wrapping and first successful reads.
	for _, heap := range []int{16, 65536} {
		memory := make([]byte, heap)
		for j := range memory {
			memory[j] = byte(j*37 + 11)
		}
		for n := int64(0); n < 256; n++ {
			for _, a := range [][4]int64{{0, 0, 0, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}, {3, 7, 2, 0}, {15, 0, 0, 0}, {0, 15, 0, 0}, {65535, 0, 0, 0}, {0, 65535, 0, 0}, {0, 0, 65535, 0}, {0, 0, 0, 65535}, {0, 0, 65535 - n, 65534 - n}, {65535 - n, 65534 - n, 0, 0}} {
				checkRegionCase(t, mods, memory, [5]int64{a[0], a[1], a[2], a[3], n})
				cases++
			}
		}
	}
	// Equal first pair, followed by left or right second read trapping. Same
	// addresses alias; overlapping full equal regions exercise the successful tail.
	for _, a := range [][5]int64{{15, 0, 0, 65535, 2}, {0, 15, 0, 65535, 2}, {0, 0, 0, 65535, 16}, {65534, 65534, 1, 0, 3}} {
		memory := make([]byte, 16)
		for j := range memory {
			memory[j] = 7
		}
		checkRegionCase(t, mods, memory, a)
		cases++
	}
	t.Log("independent ordered-read cases", cases)
}
func checkRegionCase(t *testing.T, mods []*mir2.Module, memory []byte, args [5]int64) {
	t.Helper()
	want := int64(1)
	events := []mir2.VMDataAccess{}
	trap := ""
	for i := int64(0); i < args[4]; i++ {
		left := int(uint16(args[0] + int64(uint16(args[2]+i))))
		right := int(uint16(args[1] + int64(uint16(args[3]+1+i))))
		if left >= len(memory) {
			trap = fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", left, len(memory))
			break
		}
		lv := memory[left]
		events = append(events, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(left), Value: lv})
		if right >= len(memory) {
			trap = fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", right, len(memory))
			break
		}
		rv := memory[right]
		events = append(events, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(right), Value: rv})
		if lv != rv {
			want = 0
			break
		}
	}
	for variant, m := range mods {
		vm := mir2.NewVM(m)
		vm.EnsureHeap(int64(len(memory)))
		vm.WriteHeapSlice(0, memory)
		got := []mir2.VMDataAccess{}
		vm.ObserveDataAccess = func(a mir2.VMDataAccess) { got = append(got, a) }
		values := []mir2.Value{}
		for _, a := range args {
			values = append(values, mir2.Value{I: a})
		}
		ret, e := vm.Call("region_eq", values)
		if !reflect.DeepEqual(got, events) || !reflect.DeepEqual(vm.ReadHeap(0, len(memory)), memory) || (trap != "" && (e == nil || !strings.Contains(e.Error(), trap))) || (trap == "" && (e != nil || len(ret) != 1 || ret[0].I != want)) {
			t.Fatalf("variant%d args%v events%v want%v ret%v error%v trap%s", variant, args, got, events, ret, e, trap)
		}
	}
}
func TestRegionCompareContractRefusals(t *testing.T) {
	for _, name := range []string{"global_i", "metadata", "first_order", "skip_first", "step", "fallback", "limit", "lineage_nonloop", "origin"} {
		t.Run(name, func(t *testing.T) {
			source, _, _ := regionModule(t)
			actual, c, id := regionModule(t)
			if name == "global_i" {
				source.Globals = append(source.Globals, mir2.Global{Name: "i", Ty: mir2.TyU8})
				ids, e := hir.EligibleRegionCompareSites(source)
				if e == nil && len(ids) != 0 {
					t.Fatal("global induction collision eligible")
				}
				return
			}
			if e := hir.ApplyRegionCompareSiteChoices(actual, c, map[string]bool{id: true}); e != nil {
				t.Fatal(e)
			}
			b := actual.Funcs[0].Body.Body[1].(*hir.Block)
			version := b.Body[6].(*hir.IfStmt)
			tail := version.Then.Body[2].(*hir.ForRangeStmt)
			switch name {
			case "metadata":
				actual.Funcs[0].IsIO = true
			case "first_order":
				expr := b.Body[1].(*hir.IfStmt).Cond.(*hir.BinExpr)
				expr.L, expr.R = expr.R, expr.L
			case "skip_first":
				b.Body[2] = hir.Assign(hir.Var("i", mir2.TyU8), hir.U8(2))
			case "step":
				tail.Body.Body[1] = hir.Assign(hir.Var(version.Then.Body[0].(*hir.VarDeclStmt).Name, mir2.TyPtr), hir.U16(0))
			case "fallback":
				version.Else = hir.Blk(&hir.ReturnStmt{Val: hir.U8(1)})
			case "limit":
				b.Body[4].(*hir.VarDeclStmt).Init = hir.U16(65535)
			case "lineage_nonloop":
				actual.LoopLineage[b.Body[0]] = actual.LoopLineage[tail]
			case "origin":
				for s, l := range actual.SourceLocations {
					l.SourceSHA256 = strings.Repeat("0", 64)
					actual.SourceLocations[s] = l
				}
			}
			if _, e := hir.ValidateRegionCompareChoices(source, actual, map[string]bool{id: true}); e == nil {
				t.Fatal("forged proof accepted")
			}
		})
	}
}

func TestRegionCompareZeroSiteLineageAndFreshness(t *testing.T) {
	const text = `fun ordinary(n:u8)->u8 {var i:u8=0
while i<n {i=i+1}
while i<n {i=i+1}
return i}`
	factory := func() (*hir.Module, *hir.ProfileCompatibility) {
		m, e := nanz.Parse(text, "ordinary.nanz")
		if e != nil {
			t.Fatal(e)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
		c, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "test", WorkloadSHA256: hash, HostsSHA256: hash, Dependencies: map[string]string{"ordinary.nanz": hash}})
		if e != nil {
			t.Fatal(e)
		}
		return m, c
	}
	for _, name := range []string{"valid", "missing", "nonloop", "swap", "orphanorigin", "policy"} {
		t.Run(name, func(t *testing.T) {
			source, _ := factory()
			actual, c := factory()
			if e := hir.ApplyRegionCompareSiteChoices(actual, c, map[string]bool{}); e != nil {
				t.Fatal(e)
			}
			one, two := actual.Funcs[0].Body.Body[1], actual.Funcs[0].Body.Body[2]
			switch name {
			case "missing":
				delete(actual.LoopLineage, one)
			case "nonloop":
				actual.LoopLineage[actual.Funcs[0].Body.Body[3]] = actual.LoopLineage[one]
				delete(actual.LoopLineage, one)
			case "swap":
				actual.LoopLineage[one], actual.LoopLineage[two] = actual.LoopLineage[two], actual.LoopLineage[one]
			case "orphanorigin":
				origin := actual.SourceLocations[one]
				delete(actual.SourceLocations, one)
				actual.SourceLocations[&hir.WhileStmt{Cond: one.(*hir.WhileStmt).Cond, Body: one.(*hir.WhileStmt).Body}] = origin
			case "policy":
				actual.GraceRulePolicy[hir.ForwardCopyRule] = true
			}
			_, e := hir.ValidateRegionCompareChoices(source, actual, map[string]bool{})
			if (name == "valid") != (e == nil) {
				t.Fatalf("%s: %v", name, e)
			}
		})
	}
	m, c := factory()
	before := m.Dump()
	if e := hir.ApplyRegionCompareSiteChoices(m, c, map[string]bool{"unknown": false}); e == nil || m.LoopLineage != nil || m.Dump() != before {
		t.Fatal("unknown false choice mutated source")
	}
}

func TestRegionCompareIndependentTailSplitProof(t *testing.T) {
	const text = `fun ordinary(a:u8,b:u8)->u8 {var x:u8=a
var y:u8=b
return x+y}`
	factory := func() (*hir.Module, *hir.ProfileCompatibility) {
		m, e := nanz.Parse(text, "split.nanz")
		if e != nil {
			t.Fatal(e)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
		c, e := hir.BuildProfileCompatibility(m, hir.ProfileContext{CompilerID: "test", SemanticsID: "test", TargetID: "test", WorkloadSHA256: hash, HostsSHA256: hash, Dependencies: map[string]string{"split.nanz": hash}})
		if e != nil {
			t.Fatal(e)
		}
		return m, c
	}
	for _, name := range []string{"valid", "arg", "param", "suffix", "callkind", "extrahelper", "metadata"} {
		t.Run(name, func(t *testing.T) {
			source, _ := factory()
			actual, c := factory()
			if e := hir.ApplyRegionCompareSiteChoices(actual, c, map[string]bool{}); e != nil {
				t.Fatal(e)
			}
			f := actual.Funcs[0]
			suffix := f.Body.Body[2]
			helper := &hir.Func{Name: "ordinary$split_1", Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}, {Name: "y", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(suffix)}
			call := &hir.CallExpr{Fn: helper.Name, Args: []hir.Expr{hir.Var("x", mir2.TyU8), hir.Var("y", mir2.TyU8)}, Ty: mir2.TyU8}
			f.Body = hir.Blk(f.Body.Body[0], f.Body.Body[1], &hir.ReturnStmt{Val: call})
			actual.Funcs = append(actual.Funcs, helper)
			switch name {
			case "arg":
				call.Args[0], call.Args[1] = call.Args[1], call.Args[0]
			case "param":
				helper.Params[0].Ty = mir2.TyU16
			case "suffix":
				helper.Body = hir.Blk(&hir.ReturnStmt{Val: hir.U8(0)})
			case "callkind":
				f.Body.Body[2] = &hir.ExprStmt{Expr: call}
			case "extrahelper":
				actual.Funcs = append(actual.Funcs, &hir.Func{Name: "alien", RetTy: mir2.TyU8, Body: hir.Blk(&hir.ReturnStmt{Val: hir.U8(0)})})
			case "metadata":
				helper.IsIO = true
			}
			_, e := hir.ValidateRegionCompareCompiledChoices(source, actual, map[string]bool{})
			if (name == "valid") != (e == nil) {
				t.Fatalf("%s: %v", name, e)
			}
		})
	}
}

func TestRegionCompareSplitRejectsPrefixOnlyReturn(t *testing.T) {
	source := &hir.Module{SourceUnitsComplete: true, SourceUnits: map[string]string{}, SourceLocations: map[hir.Stmt]hir.SourceLocation{}, Funcs: []*hir.Func{{Name: "f", Params: []hir.Param{{Name: "flag", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(hir.If(hir.Var("flag", mir2.TyU8), hir.Blk(&hir.ReturnStmt{Val: hir.U8(7)}), nil), &hir.ExprStmt{Expr: hir.U8(0)}, &hir.ExprStmt{Expr: hir.U8(1)}, &hir.ExprStmt{Expr: hir.U8(2)})}}}
	actual := *source
	f := *source.Funcs[0]
	actual.Funcs = []*hir.Func{&f}
	actual.RegionComparePolicy = map[string]bool{}
	actual.GraceRulePolicy = map[string]bool{hir.RegionCompareRule: true}
	actual.PointerWalkPolicy = map[string]bool{}
	actual.InvariantBasePolicy = map[string]bool{}
	actual.ForwardCopyPolicy = map[string]bool{}
	actual.LoopLineage = map[hir.Stmt]hir.LoopLineage{}
	helper := &hir.Func{Name: "f$split_1", RetTy: mir2.TyU8, Body: hir.Blk(source.Funcs[0].Body.Body[2:]...)}
	f.Body = hir.Blk(source.Funcs[0].Body.Body[0], source.Funcs[0].Body.Body[1], &hir.ReturnStmt{Val: hir.Call(helper.Name, mir2.TyU8)})
	actual.Funcs = append(actual.Funcs, helper)
	if _, e := hir.ValidateRegionCompareCompiledChoices(source, &actual, map[string]bool{}); e == nil {
		t.Fatal("prefix return manufactured suffix return value")
	}
}

func TestRegionCompareGraceRequiresBothOptIns(t *testing.T) {
	for _, tc := range []struct {
		name               string
		rule, policy, seed bool
		want               int
	}{
		{"default", false, false, false, 0}, {"rule_only", true, false, true, 0}, {"policy_only", false, true, true, 0}, {"no_lineage", true, true, false, 0}, {"both", true, true, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _, id := regionModule(t)
			m.GraceRulePolicy = map[string]bool{}
			if tc.rule {
				m.GraceRulePolicy[hir.RegionCompareRule] = true
			}
			if tc.policy {
				m.RegionComparePolicy = map[string]bool{id: true}
			}
			if tc.seed {
				if e := hir.SeedLoopLineage(m); e != nil {
					t.Fatal(e)
				}
			}
			result, e := hir.RunGraceWithOptions(m, hir.GraceOptions{OptimizeForSize: true})
			if e != nil {
				t.Fatal(e)
			}
			if result.ByRule[hir.RegionCompareRule] != tc.want {
				t.Fatal(result)
			}
		})
	}
}

func TestRegionCompareSplitRejectsNoRegAliasInterface(t *testing.T) {
	target := &hir.Func{Name: "target", RetTy: mir2.TyU8, Body: hir.Blk(&hir.ReturnStmt{Val: hir.U8(7)})}
	body := hir.Blk(hir.Decl("alias", mir2.TyU8, hir.Var("target", mir2.TyU8)), hir.Decl("unused", mir2.TyU8, hir.U8(0)), &hir.ReturnStmt{Val: &hir.CallIndirectExpr{FnPtr: hir.Var("alias", mir2.TyU8), Ty: mir2.TyU8}})
	source := &hir.Module{SourceUnitsComplete: true, SourceUnits: map[string]string{}, SourceLocations: map[hir.Stmt]hir.SourceLocation{}, Funcs: []*hir.Func{{Name: "f", RetTy: mir2.TyU8, Body: body}, target}}
	actual := *source
	f := *source.Funcs[0]
	actual.Funcs = []*hir.Func{&f, target}
	actual.RegionComparePolicy = map[string]bool{}
	actual.GraceRulePolicy = map[string]bool{hir.RegionCompareRule: true}
	actual.PointerWalkPolicy = map[string]bool{}
	actual.InvariantBasePolicy = map[string]bool{}
	actual.ForwardCopyPolicy = map[string]bool{}
	actual.LoopLineage = map[hir.Stmt]hir.LoopLineage{}
	helper := &hir.Func{Name: "f$split_1", Params: []hir.Param{{Name: "alias", Ty: mir2.TyU8}}, RetTy: mir2.TyU8, Body: hir.Blk(body.Body[2])}
	f.Body = hir.Blk(body.Body[0], body.Body[1], &hir.ReturnStmt{Val: hir.Call(helper.Name, mir2.TyU8, hir.Var("alias", mir2.TyU8))})
	actual.Funcs = append(actual.Funcs, helper)
	if _, e := hir.ValidateRegionCompareCompiledChoices(source, &actual, map[string]bool{}); e == nil {
		t.Fatal("NoReg alias forwarded as scalar")
	}
}
