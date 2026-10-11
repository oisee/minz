package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

const regionCompareSource = `fun region_eq(src: ^u8, arena: ^u8, src_start: u16, arena_ptr: u16, len: u8) -> u8 {
 var i: u8 = 0
 while i < len {
  if src[src_start + i] != arena[arena_ptr + 1 + i] { return 0 }
  i = i + 1
 }
 return 1
}`

func regionCompareFixture(t *testing.T, count int) (func() (*hir.Module, error), hir.ProfileWorkload, []byte) {
	t.Helper()
	source := ""
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"left": {7, 9, 3}, "right": {2, 7, 9, 4}}}
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("compare%d", i)
		source += strings.Replace(regionCompareSource, "fun region_eq(", "fun "+name+"(", 1) + "\n"
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{{Resource: "left"}, {Resource: "right"}, literal(0), literal(0), literal(2)}})
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "region-profile.nanz") }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
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

func TestRegionCompareMeasuredSites(t *testing.T) {
	_, w, p := regionCompareFixture(t, 2)
	ordinarySource := "\nfun untouched(n:u8)->u8{var i:u8=0\nwhile i<n{i=i+1}\nreturn i}\n"
	f := func() (*hir.Module, error) {
		source := ""
		for i := 0; i < 2; i++ {
			source += strings.Replace(regionCompareSource, "fun region_eq(", fmt.Sprintf("fun compare%d(", i), 1) + "\n"
		}
		return nanz.Parse(source+ordinarySource, "region-profile.nanz")
	}
	original, err := f()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(original, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ = json.Marshal(raw.Profile)
	for _, size := range []bool{false, true} {
		r, err := MeasureRegionCompareSiteVariantSets(f, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if e.Schema != "minz.region-compare-site-variant-sets/v1" || len(e.EligibleSites) != 2 || len(e.Candidates) != 4 {
			t.Fatal("wrong measured site scope")
		}
		best := 0
		for i, c := range e.Candidates {
			if len(c.Proofs) != 2 || len(c.FinalProofs) != 2 {
				t.Fatal("missing independent proof")
			}
			if c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) || c.Run.BinarySHA256 != c.BinarySHA256 || len(c.Binary) != c.ModuleBytes {
				t.Fatal("binary mismatch")
			}
			for _, inv := range c.Run.Invocations {
				if len(inv.Results) != 1 || inv.Results[0].I != 1 {
					t.Fatal("independent equality oracle")
				}
			}
			if (!size && c.TotalTStates < e.Candidates[best].TotalTStates) || (size && c.ModuleBytes < e.Candidates[best].ModuleBytes) {
				best = i
			}
		}
		if e.Selected != best || !reflect.DeepEqual(r.Binary, e.Candidates[best].Binary) {
			t.Fatal("measured objective selection")
		}
		copy := *e
		copy.SHA256 = ""
		b, err := json.Marshal(copy)
		if err != nil || e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(b)) {
			t.Fatal("evidence digest")
		}
	}
	w.Calls = w.Calls[:1]
	m, _ := f()
	raw, err = hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ = json.Marshal(raw.Profile)
	r, err := MeasureRegionCompareSiteVariantSets(f, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil || len(r.Evidence.Candidates) != 2 || len(r.Evidence.EligibleSites) != 1 {
		t.Fatal("unobserved site selected", err)
	}
}

func TestRegionCompareMeasuredCapBoundary(t *testing.T) {
	for _, count := range []int{4, 5} {
		factory, w, p := regionCompareFixture(t, count)
		calls := 0
		f := func() (*hir.Module, error) { calls++; return factory() }
		r, err := MeasureRegionCompareSiteVariantSets(f, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if count == 5 {
			if err == nil || r != nil || calls != 1 {
				t.Fatal("five observed region sites cap not refused before candidates", err, calls)
			}
			continue
		}
		if err != nil || len(r.Evidence.Candidates) != 16 || calls != 17 {
			t.Fatal("four-site sixteen-candidate boundary", err, calls)
		}
		for _, c := range r.Evidence.Candidates {
			if len(c.Proofs) != 4 || len(c.FinalProofs) != 4 || len(c.Run.Invocations) != 4 {
				t.Fatal("boundary proof or target observations missing")
			}
		}
	}
}

func TestRegionCompareMeasuredRefusals(t *testing.T) {
	factory, w, p := regionCompareFixture(t, 2)
	for _, name := range []string{"nil_factory", "shared", "shared_ast", "region_policy", "copy_policy", "pointer_policy", "invariant_policy", "grace_policy", "lineage", "mutated_prior", "changed_source", "forged_count", "bad_cap", "site_cap", "backend", "skip"} {
		t.Run(name, func(t *testing.T) {
			f := factory
			obs := p
			cap := 4
			opts := Options{AssertMode: "none", UseHIRGrace: true}
			shared, _ := factory()
			switch name {
			case "nil_factory":
				f = nil
			case "shared":
				f = func() (*hir.Module, error) { return shared, nil }
			case "shared_ast":
				f = func() (*hir.Module, error) { m, _ := factory(); m.Funcs = shared.Funcs; return m, nil }
			case "region_policy", "copy_policy", "pointer_policy", "invariant_policy", "grace_policy", "lineage":
				f = func() (*hir.Module, error) {
					m, _ := factory()
					switch name {
					case "region_policy":
						m.RegionComparePolicy = map[string]bool{}
					case "copy_policy":
						m.ForwardCopyPolicy = map[string]bool{}
					case "pointer_policy":
						m.PointerWalkPolicy = map[string]bool{}
					case "invariant_policy":
						m.InvariantBasePolicy = map[string]bool{}
					case "grace_policy":
						m.GraceRulePolicy = map[string]bool{}
					case "lineage":
						m.LoopLineage = map[hir.Stmt]hir.LoopLineage{}
					}
					return m, nil
				}
			case "mutated_prior":
				var first *hir.Module
				f = func() (*hir.Module, error) {
					m, _ := factory()
					if first == nil {
						first = m
					} else {
						first.PointerWalkPolicy = map[string]bool{}
					}
					return m, nil
				}
			case "changed_source":
				calls := 0
				f = func() (*hir.Module, error) { m, _ := factory(); calls++; m.Name = fmt.Sprint(calls); return m, nil }
			case "forged_count":
				var profile hir.LoopProfile
				if err := json.Unmarshal(p, &profile); err != nil {
					t.Fatal(err)
				}
				profile.Counts[0].Entries++
				obs, _ = json.Marshal(profile)
			case "bad_cap":
				cap = 0
			case "site_cap":
				cap = 1
			case "backend":
				opts.Backend = "ez80"
			case "skip":
				opts.SkipZ80Emission = true
			}
			if r, err := MeasureRegionCompareSiteVariantSets(f, obs, w, opts, cap); err == nil || r != nil {
				t.Fatal("unsafe comparison request accepted")
			}
		})
	}
}

func TestOlderMeasuredAPIsRefuseRegionPolicy(t *testing.T) {
	factory, w, p := regionCompareFixture(t, 1)
	sticky := func() (*hir.Module, error) {
		m, err := factory()
		if m != nil {
			m.RegionComparePolicy = map[string]bool{}
		}
		return m, err
	}
	opts := Options{AssertMode: "none", UseHIRGrace: true}
	checks := []struct {
		name string
		run  func() error
	}{
		{"sum_cost_model", func() error { _, _, e := MeasureLoopCostModel(sticky, "compare0", w, opts); return e }},
		{"profile", func() error { _, e := MeasureProfileVariantSets(sticky, p, w, opts, 4); return e }},
		{"grace", func() error { _, e := MeasureGraceVariantSets(sticky, p, w, opts, 4); return e }},
		{"grace_rules", func() error { _, e := MeasureGraceRuleVariantSets(sticky, p, w, opts, 4); return e }},
		{"invariant", func() error { _, e := MeasureInvariantSiteVariantSets(sticky, p, w, opts, 4); return e }},
		{"invariant_rules", func() error { _, e := MeasureInvariantSiteRuleVariantSets(sticky, p, w, opts, 4); return e }},
		{"joint", func() error { _, e := MeasureJointSourceSiteVariantSets(sticky, p, w, opts, 4); return e }},
		{"weighted", func() error { _, e := MeasureWeightedJointSourceSiteVariantSets(sticky, p, w, opts, 4); return e }},
		{"copy", func() error { _, e := MeasureForwardCopySiteVariantSets(sticky, p, w, opts, 4); return e }},
		{"mixed", func() error { _, e := MeasureMixedCopyWeightedVariantSets(sticky, p, w, opts, 4); return e }},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if err := c.run(); err == nil {
				t.Fatal("older API accepted sticky region policy")
			}
		})
	}
}

func TestRegionCompareUnknownAndEffectfulSites(t *testing.T) {
	f, w, _ := regionCompareFixture(t, 1)
	m, _ := f()
	ctx, err := hir.BuildWorkloadProfileContext(m, w)
	if err != nil {
		t.Fatal(err)
	}
	key, err := hir.BuildProfileCompatibility(m, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.ApplyRegionCompareSiteChoices(m, key, map[string]bool{"unknown": true}); err == nil {
		t.Fatal("unknown site accepted")
	}
	stale, err := nanz.Parse(regionCompareSource+"\n// source revision\n", "region-profile.nanz")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := hir.EligibleRegionCompareSites(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.ApplyRegionCompareSiteChoices(stale, key, map[string]bool{ids[0]: true}); err == nil {
		t.Fatal("stale source compatibility accepted")
	}

	m, err = nanz.Parse("global calls: u8 = 0\n"+strings.Replace(regionCompareSource, "while i < len {", "while i < len {\n calls = calls + 1", 1), "region-effects.nanz")
	if err != nil {
		t.Fatal(err)
	}
	ids, err = hir.EligibleRegionCompareSites(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatal("effectful comparison eligible")
	}
}

func TestRegionCompareCompiledHelperRefusals(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/grace_tokenizer_checked.nanz")
	if err != nil {
		t.Fatal(err)
	}
	factory := func() *hir.Module {
		m, err := nanz.Parse(string(source), "tokenizer-compiled-proof.nanz")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	original := factory()
	ctx, err := hir.BuildAutomaticProfileContext(original, hir.ProfileInputs{TargetModel: "cpm-z80-v1", Workload: []byte("fixture"), Hosts: []byte("controlled")})
	if err != nil {
		t.Fatal(err)
	}
	key, err := hir.BuildProfileCompatibility(original, ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"argument", "suffix", "helper_metadata", "extra_helper", "origin", "lineage"} {
		t.Run(name, func(t *testing.T) {
			actual := factory()
			if err := hir.ApplyRegionCompareSiteChoices(actual, key, map[string]bool{}); err != nil {
				t.Fatal(err)
			}
			if _, err := CompileHIRSteps(actual, Options{AssertMode: "none", UseHIRGrace: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := hir.ValidateRegionCompareCompiledChoices(original, actual, map[string]bool{}); err != nil {
				t.Fatal("valid compiled proof refused", err)
			}
			scan := actual.FuncByName("scan")
			if scan == nil {
				t.Fatal("missing scan")
			}
			last := scan.Body.Body[len(scan.Body.Body)-1].(*hir.ReturnStmt)
			call := last.Val.(*hir.CallExpr)
			helper := actual.FuncByName(call.Fn)
			if helper == nil {
				t.Fatal("missing actual split helper")
			}
			switch name {
			case "argument":
				call.Args[0] = hir.U16(0)
			case "suffix":
				helper.Body.Body = append([]hir.Stmt{&hir.ReturnStmt{Val: hir.U8(42)}}, helper.Body.Body...)
			case "helper_metadata":
				helper.IsIO = true
			case "extra_helper":
				actual.Funcs = append(actual.Funcs, &hir.Func{Name: "unused$split_0", RetTy: mir2.TyU8, Body: hir.Blk(&hir.ReturnStmt{Val: hir.U8(0)})})
			case "origin":
				for stmt, loc := range actual.SourceLocations {
					loc.ByteOffset++
					actual.SourceLocations[stmt] = loc
					break
				}
			case "lineage":
				for stmt, lin := range actual.LoopLineage {
					lin.SiteID = "forged"
					actual.LoopLineage[stmt] = lin
					break
				}
			}
			if _, err := hir.ValidateRegionCompareCompiledChoices(original, actual, map[string]bool{}); err == nil {
				t.Fatal("mutated split helper accepted")
			}
		})
	}
}

func TestRegionCompareTypedTokenizerMeasured(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/grace_tokenizer_checked.nanz")
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) {
		return nanz.Parse(string(source), "../../../examples/nanz/grace_tokenizer_checked.nanz")
	}
	inputs := []string{"", "fun add(a: u8, b: u8) -> u8 { return a + b }", "fn if else while let var true false return", "enum global struct import assert", "alpha alpha beta alpha", "0 007 123456789", "(){}:,-> == = + - * / < > <= >= != !", "// comment\nfun", "// unterminated", " \t\r\n", "\"hello\" \"hello\" \"\"", "\"unterminated", "abc\x00ignored", "_ _a a1 A9", "!@?#$fun", "/", "-", "=", "<", ">", "!", "\"a\\b\"", "// one\n// two\nvar x", "abc abcd abc abd", "123abc", "funny function fn fun", strings.Repeat("x", 255), strings.Repeat("y", 256), strings.Repeat("z", 257), "fun add(a: u8, b: u8) -> u8 { return a + b }"}
	arena, tokens := make([]byte, 4096), make([]byte, 2048)
	for i := range arena {
		arena[i] = 0xa5
	}
	for i := range tokens {
		tokens[i] = 0xc3
	}
	resources := map[string][]byte{"arena": arena, "tokens": tokens}
	for i, s := range inputs {
		resources[fmt.Sprintf("src%02d", i)] = append([]byte(s), 0, 0x5a)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: resources}
	host := tokenizerHost{arena: append([]byte(nil), arena...), tokens: append([]byte(nil), tokens...), keywords: map[string]int{}}
	snapshots := []hir.ProfileMemorySnapshot{}
	results := []int64{}
	accesses := []map[string]hir.ProfileByteAccess{}
	for i, s := range inputs {
		n := int64(len(s))
		name := fmt.Sprintf("src%02d", i)
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "tokenizer_checked", Arguments: []hir.ProfileArgument{{Resource: name}, {Resource: "arena"}, {Resource: "tokens"}, {Value: &n}}})
		host.run(resources[name], len(s))
		memory := map[string][]byte{}
		access := map[string]hir.ProfileByteAccess{}
		for k, v := range resources {
			memory[k] = append([]byte(nil), v...)
			access[k] = hir.ProfileByteAccess{Reads: make([]uint64, len(v)), Writes: make([]uint64, len(v))}
		}
		memory["arena"] = append([]byte(nil), host.arena...)
		memory["tokens"] = append([]byte(nil), host.tokens...)
		access["arena"] = hir.ProfileByteAccess{Reads: append([]uint64(nil), host.arenaReads...), Writes: append([]uint64(nil), host.arenaWrites...)}
		access["tokens"] = hir.ProfileByteAccess{Reads: append([]uint64(nil), host.tokensReads...), Writes: append([]uint64(nil), host.tokensWrites...)}
		access[name] = hir.ProfileByteAccess{Reads: append([]uint64(nil), host.sourceReads...), Writes: make([]uint64, len(resources[name]))}
		snapshots = append(snapshots, hir.ProfileMemorySnapshot{Globals: host.globals(len(s)), Resources: memory})
		accesses = append(accesses, access)
		results = append(results, int64(host.count))
	}
	checkRaw := func(r *hir.ProfileRun) {
		if r == nil || len(r.Results) != 30 || len(r.Snapshots) != 30 || len(r.Accesses) != 30 {
			t.Fatal("incomplete raw tokenizer")
		}
		for i, v := range r.Results {
			if len(v) != 1 || v[0].I != results[i] || !reflect.DeepEqual(r.Snapshots[i], snapshots[i]) || !reflect.DeepEqual(r.Accesses[i].Resources, accesses[i]) {
				t.Fatal("independent raw tokenizer oracle", i)
			}
		}
	}
	checkTarget := func(r *Z80ProfileWorkloadEvidence) {
		if r == nil || len(r.Invocations) != 30 {
			t.Fatal("incomplete target tokenizer")
		}
		for i, v := range r.Invocations {
			if len(v.Results) != 1 || v.Results[0].I != results[i] || !reflect.DeepEqual(v.Snapshot, snapshots[i]) || v.Access == nil || !reflect.DeepEqual(v.Access.Resources, accesses[i]) {
				t.Fatal("independent target tokenizer oracle", i)
			}
		}
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	checkRaw(raw)
	p, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []bool{false, true} {
		r, err := MeasureRegionCompareSiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if len(e.Candidates) != 2 || len(e.EligibleSites) != 1 {
			t.Fatal("region-only tokenizer candidate scope")
		}
		best := 0
		for i, c := range e.Candidates {
			checkRaw(c.RawRun)
			checkRaw(c.FinalRawRun)
			checkTarget(c.Run)
			if len(c.Proofs) != 1 || len(c.FinalProofs) != 1 || c.Proofs[0].Enabled != (i == 1) || c.FinalProofs[0].Enabled != (i == 1) {
				t.Fatal("tokenizer region proof binding")
			}
			if (!size && c.TotalTStates < e.Candidates[best].TotalTStates) || (size && c.ModuleBytes < e.Candidates[best].ModuleBytes) {
				best = i
			}
			t.Logf("region-only tokenizer size=%t candidate%d %dT/%dB", size, i, c.TotalTStates, c.ModuleBytes)
		}
		if e.Selected != best {
			t.Fatal("region tokenizer objective choice")
		}
		digest := *e
		digest.SHA256 = ""
		b, _ := json.Marshal(digest)
		if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(b)) {
			t.Fatal("region tokenizer evidence digest")
		}
		if dir := os.Getenv("MINZ_REGION_COMPARE_ARTIFACTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			b, _ := json.MarshalIndent(e, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("region-size-%t-evidence.json", size)), b, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("region-size-%t-selected.a80", size)), []byte(r.Steps.Assembly), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("region-size-%t-selected.bin", size)), r.Binary, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
