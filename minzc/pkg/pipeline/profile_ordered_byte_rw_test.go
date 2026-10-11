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

func TestOrderedByteRWOriginalHashmapMeasured(t *testing.T) {
	path := "../../../examples/nanz/21_hashmap_test.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}}
	add := func(name string, args ...int64) {
		call := hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{}}
		for _, v := range args {
			owned := v
			call.Arguments = append(call.Arguments, hir.ProfileArgument{Value: &owned})
		}
		workload.Calls = append(workload.Calls, call)
	}
	add("map_clear")
	add("map_get", 99)
	add("map_put", 5, 100)
	add("map_put", 13, 200)
	add("map_get", 13)
	add("map_put", 5, 300)
	add("map_has", 5)
	add("map_put", 0, 999)
	add("map_put", 65535, 65535)
	add("map_has", 65535)
	add("map_get", 65535)
	for _, k := range []int64{21, 29, 37, 45, 53, 61, 69, 77, 85, 93, 101, 109} {
		add("map_put", k, k*3)
	}
	add("map_put", 117, 123)
	add("map_get", 117)
	add("map_put", 13, 65535)
	add("map_has", 13)
	add("map_get", 0)
	add("map_clear")
	add("map_get", 5)
	add("map_put", 65535, 0)
	add("map_get", 65535)
	add("map_has", 65535)
	add("map_put", 0, 65535)
	add("map_has", 0)
	add("map_get", 0)

	host := newHashmapHost()
	original, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	oracle, err := hir.RunLoopProfile(original, workload)
	if err != nil {
		t.Fatal(err)
	}
	for i, call := range workload.Calls {
		result, snapshot, access := host.run(call)
		if !reflect.DeepEqual(oracle.Results[i], result) || !reflect.DeepEqual(oracle.Snapshots[i], snapshot) || !reflect.DeepEqual(oracle.Accesses[i], access) {
			t.Fatal("independent original hashmap", i)
		}
	}
	observations, err := json.Marshal(oracle.Profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []bool{false, true} {
		r, err := MeasureOrderedByteRWFunctionVariantSets(factory, observations, workload, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if len(e.EligibleSites) != 2 || len(e.Candidates) != 4 {
			t.Fatal("exact original helper scope")
		}
		best := 0
		for i, c := range e.Candidates {
			host := newHashmapHost()
			for j, call := range workload.Calls {
				result, snapshot, access := host.run(call)
				for _, raw := range []*hir.ProfileRun{c.RawRun, c.FinalRawRun} {
					if !reflect.DeepEqual(raw.Results[j], result) || !reflect.DeepEqual(raw.Snapshots[j], snapshot) || !reflect.DeepEqual(raw.Accesses[j], access) {
						t.Fatal("independent raw hashmap function variant", i, j)
					}
				}
				inv := c.Run.Invocations[j]
				if !reflect.DeepEqual(inv.Results, result) || !reflect.DeepEqual(inv.Snapshot, snapshot) || inv.Access == nil || !reflect.DeepEqual(*inv.Access, access) {
					t.Fatal("independent actual hashmap function variant", i, j)
				}
			}
			for j, site := range e.EligibleSites {
				choice := c.RequestedSites[site.ID]
				enabled := i&(1<<(len(e.EligibleSites)-1-j)) != 0
				if choice.Kind != site.Kind || choice.Enabled != enabled {
					t.Fatal("typed read/write choices")
				}
			}
			if len(c.Proofs) != 2 || len(c.FinalProofs) != 2 {
				t.Fatal("read/write aggregate proof missing")
			}
			if (!size && c.TotalTStates < e.Candidates[best].TotalTStates) || (size && c.ModuleBytes < e.Candidates[best].ModuleBytes) {
				best = i
			}
			t.Logf("ordered byte read/write hashmap size=%t candidate%d %dT/%dB", size, i, c.TotalTStates, c.ModuleBytes)
		}
		if e.Selected != best {
			t.Fatal("actual function objective choice")
		}
		copy := *e
		copy.SHA256 = ""
		b, err := json.Marshal(copy)
		if err != nil || e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(b)) {
			t.Fatal("typed evidence digest")
		}
		if dir := os.Getenv("MINZ_ORDERED_BYTE_RW_ARTIFACTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]any{fmt.Sprintf("size-%t-evidence.json", size): e, "workload.json": workload} {
				b, err := json.MarshalIndent(value, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("size-%t-selected.bin", size)), r.Binary, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("size-%t-selected.a80", size)), []byte(r.Steps.Assembly), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func orderedByteRWFixture(t *testing.T, reads, writes int) (func() (*hir.Module, error), hir.ProfileWorkload, []byte) {
	t.Helper()
	const writeSource = `fun write_word(arr:^u8,idx:u16,value:u16) {
 var base:u16=idx*2
 arr[base]=u8(value)
 arr[base+1]=u8(value/256)
}`
	source := ""
	for i := 0; i < reads; i++ {
		source += strings.Replace(orderedByteSource, "fun read_word(", fmt.Sprintf("fun read_word%d(", i), 1) + "\n"
	}
	for i := 0; i < writes; i++ {
		source += strings.Replace(writeSource, "fun write_word(", fmt.Sprintf("fun write_word%d(", i), 1) + "\n"
	}
	source += "fun untouched()->u16{return 7}\n"
	zero := int64(0)
	workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Resources: map[string][]byte{"data": {0x34, 0x12, 0, 0}}, Calls: []hir.ProfileInvocation{{Function: "read_word0", Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &zero}}}}}
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "ordered-byte-rw.nanz") }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, workload)
	if err != nil {
		t.Fatal(err)
	}
	p, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	return factory, workload, p
}
func TestOrderedByteRWCapAndUnobserved(t *testing.T) {
	for _, writes := range []int{2, 3} {
		factory, w, p := orderedByteRWFixture(t, 2, writes)
		calls := 0
		fresh := func() (*hir.Module, error) { calls++; return factory() }
		result, err := MeasureOrderedByteRWFunctionVariantSets(fresh, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if writes == 3 {
			if err == nil || result != nil || calls != 1 {
				t.Fatal("five legal read/write helpers accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Evidence.Candidates) != 16 || len(result.Evidence.EligibleSites) != 4 || calls != 17 {
			t.Fatal("four typed dimensions boundary")
		}
		for _, c := range result.Evidence.Candidates {
			if len(c.Proofs) != 4 || len(c.FinalProofs) != 4 {
				t.Fatal("disabled unobserved typed proof omitted")
			}
		}
		w.Calls = []hir.ProfileInvocation{{Function: "untouched"}}
		m, _ := factory()
		raw, err := hir.RunLoopProfile(m, w)
		if err != nil {
			t.Fatal(err)
		}
		p, _ = json.Marshal(raw.Profile)
		result, err = MeasureOrderedByteRWFunctionVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if err != nil || result.Evidence.Selected != 0 {
			t.Fatal("unobserved read/write change won equal speed", err)
		}
	}
}
func TestOrderedByteRWUnknownKindRequests(t *testing.T) {
	factory, w, _ := orderedByteRWFixture(t, 1, 1)
	source, _ := factory()
	context, err := hir.BuildWorkloadProfileContext(source, w)
	if err != nil {
		t.Fatal(err)
	}
	compatibility, err := hir.BuildProfileCompatibility(source, context)
	if err != nil {
		t.Fatal(err)
	}
	sites, err := hir.EligibleOrderedByteRWHelpers(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, choices := range []map[string]hir.FunctionChoice{{"unknown": {Kind: hir.OrderedByteReadKind, Enabled: false}}, {sites[0].ID: {Kind: "wrong", Enabled: false}}, {sites[0].ID: {Kind: "wrong", Enabled: true}}} {
		m, _ := factory()
		if err := hir.ApplyOrderedByteRWChoices(m, compatibility, choices); err == nil {
			t.Fatal("unknown/incorrect helper kind accepted")
		}
	}
}
func TestOrderedByteRWRejectsPriorPolicy(t *testing.T) {
	factory, w, p := orderedByteRWFixture(t, 1, 1)
	for _, kind := range []string{"rw", "readonly", "copy", "region", "pointer", "invariant", "rules", "lineage"} {
		t.Run(kind, func(t *testing.T) {
			sticky := func() (*hir.Module, error) {
				m, err := factory()
				switch kind {
				case "rw":
					m.OrderedByteRWPolicy = map[string]hir.FunctionChoice{}
				case "readonly":
					m.OrderedBytePolicy = map[string]bool{}
				case "copy":
					m.ForwardCopyPolicy = map[string]bool{}
				case "region":
					m.RegionComparePolicy = map[string]bool{}
				case "pointer":
					m.PointerWalkPolicy = map[string]bool{}
				case "invariant":
					m.InvariantBasePolicy = map[string]bool{}
				case "rules":
					m.GraceRulePolicy = map[string]bool{}
				case "lineage":
					m.LoopLineage = map[hir.Stmt]hir.LoopLineage{}
				}
				return m, err
			}
			if result, err := MeasureOrderedByteRWFunctionVariantSets(sticky, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4); err == nil || result != nil {
				t.Fatal("prior sticky policy accepted")
			}
		})
	}
}

func TestOlderProfileAPIsRejectOrderedByteRWPolicy(t *testing.T) {
	factory, w, p := orderedByteFixture(t, 1)
	sticky := func() (*hir.Module, error) {
		m, err := factory()
		if m != nil {
			m.OrderedByteRWPolicy = map[string]hir.FunctionChoice{}
		}
		return m, err
	}
	options := Options{AssertMode: "none", UseHIRGrace: true}
	checks := []struct {
		name string
		run  func() error
	}{
		{"readonly", func() error { _, err := MeasureOrderedByteFunctionVariantSets(sticky, p, w, options, 4); return err }},
		{"sum_model", func() error { _, _, err := MeasureLoopCostModel(sticky, "read_word0", w, options); return err }},
		{"pointer", func() error { _, err := MeasureProfileVariantSets(sticky, p, w, options, 4); return err }},
		{"grace", func() error { _, err := MeasureGraceVariantSets(sticky, p, w, options, 4); return err }},
		{"grace_rules", func() error { _, err := MeasureGraceRuleVariantSets(sticky, p, w, options, 4); return err }},
		{"invariant", func() error { _, err := MeasureInvariantSiteVariantSets(sticky, p, w, options, 4); return err }},
		{"invariant_rules", func() error { _, err := MeasureInvariantSiteRuleVariantSets(sticky, p, w, options, 4); return err }},
		{"joint", func() error { _, err := MeasureJointSourceSiteVariantSets(sticky, p, w, options, 4); return err }},
		{"weighted", func() error {
			_, err := MeasureWeightedJointSourceSiteVariantSets(sticky, p, w, options, 4)
			return err
		}},
		{"copy", func() error { _, err := MeasureForwardCopySiteVariantSets(sticky, p, w, options, 4); return err }},
		{"region", func() error { _, err := MeasureRegionCompareSiteVariantSets(sticky, p, w, options, 4); return err }},
		{"mixed_weighted", func() error { _, err := MeasureMixedCopyWeightedVariantSets(sticky, p, w, options, 4); return err }},
		{"copy_region", func() error { _, err := MeasureMixedCopyRegionSiteVariantSets(sticky, p, w, options, 4); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err == nil {
				t.Fatal("legacy API accepted ordered byte read/write policy")
			}
		})
	}
}

func TestOrderedByteRWFactoryCannotLaunderOrigins(t *testing.T) {
	const source = orderedByteSource + "\nfun same_one()->u16{return 7}\nfun same_two()->u16{return 7}\n"
	parse := func() (*hir.Module, error) { return nanz.Parse(source, "origin-bound.nanz") }
	original, err := parse()
	if err != nil {
		t.Fatal(err)
	}
	zero := int64(0)
	workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Resources: map[string][]byte{"data": {0x34, 0x12}}, Calls: []hir.ProfileInvocation{{Function: "read_word", Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &zero}}}}}
	// Exact fixture origins bind two structurally identical returns to distinct
	// tokens in the same source. The parser need not automatically collect them.
	first, second := original.FuncByName("same_one").Body.Body[0], original.FuncByName("same_two").Body.Body[0]
	hash := original.SourceUnits["origin-bound.nanz"]
	location := func(token string) hir.SourceLocation {
		offset := strings.Index(source, token) + strings.Index(token, "return")
		line := strings.Count(source[:offset], "\n") + 1
		column := offset - strings.LastIndex(source[:offset], "\n")
		return hir.SourceLocation{Unit: "origin-bound.nanz", ByteOffset: offset, Line: line, Column: column, SourceSHA256: hash}
	}
	one, two := location("fun same_one()->u16{return 7}"), location("fun same_two()->u16{return 7}")
	attach := func(m *hir.Module) {
		m.SourceLocations[m.FuncByName("same_one").Body.Body[0]] = one
		m.SourceLocations[m.FuncByName("same_two").Body.Body[0]] = two
	}
	attach(original)
	if first == second {
		t.Fatal("fixture must contain independent identical statements")
	}
	raw, err := hir.RunLoopProfile(original, workload)
	if err != nil {
		t.Fatal(err)
	}
	observations, _ := json.Marshal(raw.Profile)
	for _, mode := range []string{"fresh_and_prior", "prior_only"} {
		t.Run(mode, func(t *testing.T) {
			retained := []*hir.Module{}
			factory := func() (*hir.Module, error) {
				m, err := parse()
				if err != nil {
					return nil, err
				}
				attach(m)
				if len(retained) > 0 {
					for _, prior := range retained {
						prior.SourceLocations[prior.FuncByName("same_one").Body.Body[0]] = two
						prior.SourceLocations[prior.FuncByName("same_two").Body.Body[0]] = one
					}
					if mode == "fresh_and_prior" {
						m.SourceLocations[m.FuncByName("same_one").Body.Body[0]] = two
						m.SourceLocations[m.FuncByName("same_two").Body.Body[0]] = one
					}
				}
				retained = append(retained, m)
				return m, nil
			}
			if result, err := MeasureOrderedByteRWFunctionVariantSets(factory, observations, workload, Options{AssertMode: "none", UseHIRGrace: true}, 4); err == nil || result != nil {
				t.Fatal("factory laundered function/path origins by mutating previous inputs")
			}
		})
	}
}

func TestOrderedByteRWEqualCostPrefersFewerChanges(t *testing.T) {
	earlier := OrderedByteRWFunctionCandidate{TotalTStates: 100, ModuleBytes: 20, RequestedSites: map[string]hir.FunctionChoice{"a": {Kind: hir.OrderedByteReadKind}, "b": {Kind: hir.OrderedByteReadKind, Enabled: true}, "c": {Kind: hir.OrderedByteWriteKind, Enabled: true}}}
	later := OrderedByteRWFunctionCandidate{TotalTStates: 100, ModuleBytes: 20, RequestedSites: map[string]hir.FunctionChoice{"a": {Kind: hir.OrderedByteReadKind, Enabled: true}, "b": {Kind: hir.OrderedByteReadKind}, "c": {Kind: hir.OrderedByteWriteKind}}}
	for _, size := range []bool{false, true} {
		if !orderedByteRWCandidateBetter(later, earlier, size) || orderedByteRWCandidateBetter(earlier, later, size) {
			t.Fatal("equal objective retained unnecessary helper rewrites", size)
		}
	}
}

func TestOrderedByteRWFactoryRefusals(t *testing.T) {
	factory, w, p := orderedByteRWFixture(t, 1, 1)
	for _, name := range []string{"nil", "shared", "shared_ast", "ordered_policy", "region_policy", "copy_policy", "pointer_policy", "invariant_policy", "grace_policy", "lineage", "mutated_prior", "source_changed", "helper_type_changed", "retained_body_changed", "workload_changed", "forged_call_count", "cap", "backend", "skip"} {
		t.Run(name, func(t *testing.T) {
			w := w
			f := factory
			observations := p
			limit := 4
			options := Options{AssertMode: "none", UseHIRGrace: true}
			shared, _ := factory()
			switch name {
			case "nil":
				f = nil
			case "shared":
				f = func() (*hir.Module, error) { return shared, nil }
			case "shared_ast":
				f = func() (*hir.Module, error) { m, _ := factory(); m.Funcs = shared.Funcs; return m, nil }
			case "ordered_policy", "region_policy", "copy_policy", "pointer_policy", "invariant_policy", "grace_policy", "lineage":
				f = func() (*hir.Module, error) {
					m, _ := factory()
					switch name {
					case "ordered_policy":
						m.OrderedBytePolicy = map[string]bool{}
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
						first.OrderedBytePolicy = map[string]bool{}
					}
					return m, nil
				}
			case "source_changed":
				n := 0
				f = func() (*hir.Module, error) { m, _ := factory(); n++; m.Name = fmt.Sprint(n); return m, nil }
			case "helper_type_changed":
				n := 0
				f = func() (*hir.Module, error) {
					m, _ := factory()
					n++
					if n > 1 {
						m.FuncByName("write_word0").Params[2].Ty = mir2.TyU8
					}
					return m, nil
				}
			case "retained_body_changed":
				var prior *hir.Module
				f = func() (*hir.Module, error) {
					m, _ := factory()
					if prior == nil {
						prior = m
					} else {
						prior.FuncByName("write_word0").Body.Body = nil
					}
					return m, nil
				}
			case "workload_changed":
				w.MaxSteps++
			case "forged_call_count":
				var profile hir.LoopProfile
				_ = json.Unmarshal(p, &profile)
				profile.CompletedCalls++
				observations, _ = json.Marshal(profile)
			case "cap":
				limit = 0
			case "backend":
				options.Backend = "ez80"
			case "skip":
				options.SkipZ80Emission = true
			}
			if r, err := MeasureOrderedByteRWFunctionVariantSets(f, observations, w, options, limit); err == nil || r != nil {
				t.Fatal("unsafe function measurement accepted")
			}
		})
	}
}
