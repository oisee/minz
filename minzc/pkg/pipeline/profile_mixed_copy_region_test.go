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
	"github.com/minz/minzc/pkg/nanz"
)

func mixedCopyRegionFixture(t *testing.T, copies, regions int) (func() (*hir.Module, error), hir.ProfileWorkload, []byte) {
	t.Helper()
	source := ""
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"left": {7, 9, 3}, "right": {2, 7, 9, 4}, "dest": {0, 0, 0}}}
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	for i := 0; i < copies; i++ {
		name := fmt.Sprintf("copy%d", i)
		source += strings.Replace(forwardCopySource, "fun copy_forward(", "fun "+name+"(", 1) + "\n"
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{{Resource: "dest"}, {Resource: "left"}, literal(2)}})
	}
	for i := 0; i < regions; i++ {
		name := fmt.Sprintf("compare%d", i)
		source += strings.Replace(regionCompareSource, "fun region_eq(", "fun "+name+"(", 1) + "\n"
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{{Resource: "left"}, {Resource: "right"}, literal(0), literal(0), literal(2)}})
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "copy-region-profile.nanz") }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	r, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, err := json.Marshal(r.Profile)
	if err != nil {
		t.Fatal(err)
	}
	return factory, w, p
}
func TestMixedCopyRegionMeasuredBoundary(t *testing.T) {
	for _, regions := range []int{1, 2, 3} {
		factory, w, p := mixedCopyRegionFixture(t, 2, regions)
		calls := 0
		f := func() (*hir.Module, error) { calls++; return factory() }
		r, err := MeasureMixedCopyRegionSiteVariantSets(f, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if regions == 3 {
			if err == nil || r != nil || calls != 1 {
				t.Fatal("five dimensions accepted before candidate construction", err, calls)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if len(e.Candidates) != 1<<(2+regions) || calls != 1+len(e.Candidates) || len(e.EligibleCopySites) != 2 || len(e.EligibleRegionSites) != regions || e.Schema != "minz.mixed-copy-region-site-variant-sets/v1" {
			t.Fatal("combined dimension scope", calls)
		}
		for _, c := range e.Candidates {
			if len(c.Proofs.Copies) != 2 || len(c.Proofs.Regions) != regions || len(c.FinalProofs.Copies) != 2 || len(c.FinalProofs.Regions) != regions {
				t.Fatal("all legal typed proofs missing")
			}
			for i, inv := range c.Run.Invocations {
				want := int64(1)
				if i < 2 {
					want = 2
				}
				if len(inv.Results) != 1 || inv.Results[0].I != want {
					t.Fatal("independent copy/compare oracle")
				}
			}
		}
	}
	factory, w, _ := mixedCopyRegionFixture(t, 1, 1)
	w.Calls = w.Calls[:1]
	m, _ := factory()
	r, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(r.Profile)
	result, err := MeasureMixedCopyRegionSiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil || len(result.Evidence.Candidates) != 2 || len(result.Evidence.EligibleRegionSites) != 0 {
		t.Fatal("unobserved region selected", err)
	}
	for _, c := range result.Evidence.Candidates {
		if len(c.Proofs.Regions) != 1 || c.Proofs.Regions[0].Enabled {
			t.Fatal("unobserved region disabled proof missing")
		}
	}
}
func TestMixedCopyRegionFactoryRefusals(t *testing.T) {
	factory, w, p := mixedCopyRegionFixture(t, 1, 1)
	for _, name := range []string{"nil", "shared", "shared_ast", "region_policy", "copy_policy", "pointer_policy", "invariant_policy", "grace_policy", "lineage", "mutated_prior", "source_change", "forged_count", "cap", "backend", "skip"} {
		t.Run(name, func(t *testing.T) {
			f := factory
			obs := p
			cap := 4
			opts := Options{AssertMode: "none", UseHIRGrace: true}
			shared, _ := factory()
			switch name {
			case "nil":
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
						first.RegionComparePolicy = map[string]bool{}
					}
					return m, nil
				}
			case "source_change":
				calls := 0
				f = func() (*hir.Module, error) { m, _ := factory(); calls++; m.Name = fmt.Sprint(calls); return m, nil }
			case "forged_count":
				var profile hir.LoopProfile
				_ = json.Unmarshal(p, &profile)
				profile.Counts[0].Entries++
				obs, _ = json.Marshal(profile)
			case "cap":
				cap = 0
			case "backend":
				opts.Backend = "ez80"
			case "skip":
				opts.SkipZ80Emission = true
			}
			if r, err := MeasureMixedCopyRegionSiteVariantSets(f, obs, w, opts, cap); err == nil || r != nil {
				t.Fatal("unsafe composition request accepted")
			}
		})
	}
}
func TestMixedCopyRegionWrongKindRequests(t *testing.T) {
	factory, w, _ := mixedCopyRegionFixture(t, 1, 1)
	source, _ := factory()
	ctx, err := hir.BuildWorkloadProfileContext(source, w)
	if err != nil {
		t.Fatal(err)
	}
	key, err := hir.BuildProfileCompatibility(source, ctx)
	if err != nil {
		t.Fatal(err)
	}
	copies, regions, err := hir.MixedCopyRegionSites(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, choices := range []hir.MixedCopyRegionChoices{{Copy: map[string]bool{"unknown": false}}, {Copy: map[string]bool{regions[0]: false}}, {Region: map[string]bool{copies[0]: true}}} {
		m, _ := factory()
		if err := hir.ApplyMixedCopyRegionChoices(m, key, choices); err == nil {
			t.Fatal("unknown or wrong-kind request accepted")
		}
	}
}

func TestMixedCopyRegionCompiledForgeries(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/grace_tokenizer_checked.nanz")
	if err != nil {
		t.Fatal(err)
	}
	factory := func() *hir.Module {
		m, err := nanz.Parse(string(source), "tokenizer-composition-proof.nanz")
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
	copies, regions, err := hir.MixedCopyRegionSites(original)
	if err != nil {
		t.Fatal(err)
	}
	choices := hir.MixedCopyRegionChoices{Copy: map[string]bool{copies[0]: true}, Region: map[string]bool{regions[0]: true}}
	for _, name := range []string{"argument", "suffix", "metadata", "lineage_kind", "origin", "unrequested_policy", "unrelated_body"} {
		t.Run(name, func(t *testing.T) {
			m := factory()
			if err := hir.ApplyMixedCopyRegionChoices(m, key, choices); err != nil {
				t.Fatal(err)
			}
			if _, err := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true}); err != nil {
				t.Fatal(err)
			}
			if _, err := hir.ValidateMixedCopyRegionCompiledChoices(original, m, choices); err != nil {
				t.Fatal("valid combined compiled proof refused", err)
			}
			scan := m.FuncByName("scan")
			call := scan.Body.Body[len(scan.Body.Body)-1].(*hir.ReturnStmt).Val.(*hir.CallExpr)
			helper := m.FuncByName(call.Fn)
			switch name {
			case "argument":
				call.Args[0] = hir.U16(0)
			case "suffix":
				helper.Body.Body = append([]hir.Stmt{&hir.ReturnStmt{Val: hir.U8(42)}}, helper.Body.Body...)
			case "metadata":
				helper.IsIO = true
			case "lineage_kind":
				for stmt, l := range m.LoopLineage {
					if l.Rule == hir.RegionCompareRule {
						l.Rule = hir.ForwardCopyRule
						m.LoopLineage[stmt] = l
						break
					}
				}
			case "origin":
				for stmt, loc := range m.SourceLocations {
					loc.ByteOffset++
					m.SourceLocations[stmt] = loc
					break
				}
			case "unrequested_policy":
				m.RegionComparePolicy[regions[0]] = false
			case "unrelated_body":
				f := m.FuncByName("intern")
				f.Body.Body = append([]hir.Stmt{&hir.ReturnStmt{Val: hir.U16(7)}}, f.Body.Body...)
			}
			if _, err := hir.ValidateMixedCopyRegionCompiledChoices(original, m, choices); err == nil {
				t.Fatal("forged combined compiled proof accepted")
			}
		})
	}
}

func TestMixedCopyRegionTypedTokenizerMeasured(t *testing.T) {
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
		r, err := MeasureMixedCopyRegionSiteVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if len(e.Candidates) != 4 || len(e.EligibleCopySites) != 1 || len(e.EligibleRegionSites) != 1 {
			t.Fatal("copy-region tokenizer candidate scope")
		}
		best := 0
		for i, c := range e.Candidates {
			checkRaw(c.RawRun)
			checkRaw(c.FinalRawRun)
			checkTarget(c.Run)
			if len(c.Proofs.Copies) != 1 || len(c.Proofs.Regions) != 1 || len(c.FinalProofs.Copies) != 1 || len(c.FinalProofs.Regions) != 1 || c.Proofs.Copies[0].Enabled != (i&2 != 0) || c.Proofs.Regions[0].Enabled != (i&1 != 0) || c.FinalProofs.Copies[0].Enabled != (i&2 != 0) || c.FinalProofs.Regions[0].Enabled != (i&1 != 0) {
				t.Fatal("combined proof binding")
			}
			if (!size && c.TotalTStates < e.Candidates[best].TotalTStates) || (size && c.ModuleBytes < e.Candidates[best].ModuleBytes) {
				best = i
			}
			t.Logf("copy-region tokenizer size=%t candidate%d %dT/%dB", size, i, c.TotalTStates, c.ModuleBytes)
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
		if dir := os.Getenv("MINZ_COPY_REGION_ARTIFACTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			b, _ := json.MarshalIndent(e, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("copy-region-size-%t-evidence.json", size)), b, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("copy-region-size-%t-selected.a80", size)), []byte(r.Steps.Assembly), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("copy-region-size-%t-selected.bin", size)), r.Binary, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
