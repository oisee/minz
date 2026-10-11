package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const mixedSimpleSource = forwardCopySource + `
 global packet_calls:u8=250
 global packet_last:u16=0
 fun weighted_checksum(ptr:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {
 var sum:u16=seed
 var i:u8=0
 while i<n {sum=sum+(a+b)
 sum=sum+u16(ptr[i])
 i=i+1}
 return sum
 }
 fun copied_packet(dst:^u8,src:^u8,n:u8,a:u16,b:u16,seed:u16)->u16 {
 var copied:u8=copy_forward(dst,src,n)
 var value:u16=weighted_checksum(dst,copied,a,b,seed)
 packet_calls=packet_calls+1
 packet_last=value
 return value
 }`

func TestMixedCopyWeightedCompleteCaller(t *testing.T) {
	factory := func() (*hir.Module, error) { return nanz.Parse(mixedSimpleSource, "mixed-packet.nanz") }
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 4000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{"packet_calls": {250}, "packet_last": {0, 0}}}
	memory := map[string][]byte{}
	for _, pattern := range []string{"ramp", "ff", "zero", "alternating", "alias"} {
		data := make([]byte, 600)
		for i := range data {
			switch pattern {
			case "ramp", "alias":
				data[i] = byte(i*37 + 11)
			case "ff":
				data[i] = 255
			case "alternating":
				if i%2 != 0 {
					data[i] = 255
				}
			}
		}
		w.Resources[pattern] = data
		memory[pattern] = append([]byte(nil), data...)
	}
	snapshots := []hir.ProfileMemorySnapshot{}
	results := []int64{}
	readsAll, writesAll := []map[string][]uint64{}, []map[string][]uint64{}
	calls := byte(250)
	for pi, pattern := range []string{"ramp", "ff", "zero", "alternating", "alias"} {
		for _, n := range []int64{0, 1, 5, 16, 64, 255} {
			dst, src := 300, 0
			if pattern == "alias" {
				dst = 1
			}
			a, b, seed := int64(65535-pi), int64(pi+2), int64(65530+pi)
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "copied_packet", Arguments: []hir.ProfileArgument{{Resource: pattern, Offset: dst}, {Resource: pattern, Offset: src}, literal(n), literal(a), literal(b), literal(seed)}})
			reads, writes := map[string][]uint64{}, map[string][]uint64{}
			for name, data := range memory {
				reads[name] = make([]uint64, len(data))
				writes[name] = make([]uint64, len(data))
			}
			for i := 0; i < int(n); i++ {
				v := memory[pattern][src+i]
				reads[pattern][src+i]++
				memory[pattern][dst+i] = v
				writes[pattern][dst+i]++
			}
			sum := uint16(seed)
			for i := 0; i < int(n); i++ {
				sum += uint16(a) + uint16(b) + uint16(memory[pattern][dst+i])
				reads[pattern][dst+i]++
			}
			calls++
			snap := hir.ProfileMemorySnapshot{Globals: map[string][]byte{"packet_calls": {calls}, "packet_last": {byte(sum), byte(sum >> 8)}}, Resources: map[string][]byte{}}
			for name, data := range memory {
				snap.Resources[name] = append([]byte(nil), data...)
			}
			snapshots = append(snapshots, snap)
			results = append(results, int64(sum))
			readsAll = append(readsAll, reads)
			writesAll = append(writesAll, writes)
		}
	}
	source, _ := factory()
	raw, err := hir.RunLoopProfile(source, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(raw.Profile)
	check := func(run *hir.ProfileRun) {
		for j, r := range run.Results {
			if r[0].I != results[j] || !reflect.DeepEqual(run.Snapshots[j], snapshots[j]) {
				t.Fatal("mixed raw hostoracle", j)
			}
		}
	}
	check(raw)
	dir := os.Getenv("MINZ_MIXED_ARTIFACTS")
	write := func(name string, v any) {
		if dir == "" {
			return
		}
		os.MkdirAll(dir, 0755)
		data, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("workload.json", w)
	write("oracle.json", map[string]any{"results": results, "snapshots": snapshots, "reads": readsAll, "writes": writesAll})
	if dir != "" {
		os.WriteFile(filepath.Join(dir, "source.nanz"), []byte(mixedSimpleSource), 0644)
	}
	for _, size := range []bool{false, true} {
		r, err := MeasureMixedCopyWeightedVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if e.Schema != "minz.mixed-copy-weighted-variant-sets/v1" || len(e.Candidates) != 8 || len(e.EligibleCopySites) != 1 || len(e.EligibleWeightedSites) != 1 {
			t.Fatal("mixed8candidatecontract")
		}
		best := 0
		for i, c := range e.Candidates {
			if len(c.Proofs.Copies) != 1 || len(c.Proofs.Weighted) != 1 || len(c.FinalProofs.Copies) != 1 || len(c.FinalProofs.Weighted) != 1 {
				t.Fatal("mixedmissingproofs")
			}
			check(c.RawRun)
			check(c.FinalRawRun)
			for j, inv := range c.Run.Invocations {
				if inv.Results[0].I != results[j] || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) {
					t.Fatal("mixedtargethostoracle", i, j)
				}
				for name, access := range inv.Access.Resources {
					if !reflect.DeepEqual(access.Reads, readsAll[j][name]) || !reflect.DeepEqual(access.Writes, writesAll[j][name]) {
						t.Fatal("mixedexacttraffic", i, j, name)
					}
				}
			}
			better := c.TotalTStates < e.Candidates[best].TotalTStates
			if size {
				better = c.ModuleBytes < e.Candidates[best].ModuleBytes
			}
			if better {
				best = i
			}
			t.Logf("size%v candidate%d%dT/%dB", size, i, c.TotalTStates, c.ModuleBytes)
		}
		if e.Selected != best || !bytes.Equal(r.Binary, e.Candidates[best].Binary) {
			t.Fatal("mixedunmeasuredwinner")
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, source, w, *raw)
		if err != nil || !reflect.DeepEqual(replay, e.Candidates[best].Run) {
			t.Fatal("mixedretainedbinary", err)
		}
		copyE := *e
		copyE.SHA256 = ""
		data, _ := json.Marshal(copyE)
		if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
			t.Fatal("mixeddigest")
		}
		write(fmt.Sprintf("size-%v-evidence.json", size), e)
	}
}

func TestMixedCopyWeightedApplicationRefusals(t *testing.T) {
	factory := func() (*hir.Module, error) { return nanz.Parse(mixedSimpleSource, "mixed-apply.nanz") }
	source, _ := factory()
	context, err := hir.BuildAutomaticProfileContext(source, hir.ProfileInputs{TargetModel: "test"})
	if err != nil {
		t.Fatal(err)
	}
	key, err := hir.BuildProfileCompatibility(source, context)
	if err != nil {
		t.Fatal(err)
	}
	copies, weights, _ := hir.MixedCopyWeightedSites(source)
	for _, name := range []string{"unknown_false", "wrong_kind", "preseed", "prior_copy", "stale"} {
		t.Run(name, func(t *testing.T) {
			m, _ := factory()
			choices := hir.MixedCopyWeightedChoices{Copy: map[string]bool{}, Scalar: map[string]bool{}, Pointer: map[string]bool{}}
			k := key
			switch name {
			case "unknown_false":
				choices.Copy["bad"] = false
			case "wrong_kind":
				choices.Copy[weights[0]] = true
			case "preseed":
				hir.SeedLoopLineage(m)
			case "prior_copy":
				m.ForwardCopyPolicy = map[string]bool{}
			case "stale":
				copy := *key
				copy.HIRSHA256 = "bad"
				k = &copy
			}
			if err := hir.ApplyMixedCopyWeightedChoices(m, k, choices); err == nil {
				t.Fatal("unsafe mixed application")
			}
		})
	}
	actual, _ := factory()
	choices := hir.MixedCopyWeightedChoices{Copy: map[string]bool{copies[0]: true}, Scalar: map[string]bool{weights[0]: true}, Pointer: map[string]bool{weights[0]: true}}
	if err := hir.ApplyMixedCopyWeightedChoices(actual, key, choices); err != nil {
		t.Fatal(err)
	}
	choices.Copy[copies[0]] = false
	if !actual.ForwardCopyPolicy[copies[0]] {
		t.Fatal("mapnotowned")
	}
	choices.Copy[copies[0]] = true
	for _, name := range []string{"previous", "copy_onto_weighted", "duplicate", "caller_body"} {
		t.Run(name, func(t *testing.T) {
			m, _ := factory()
			if err := hir.ApplyMixedCopyWeightedChoices(m, key, choices); err != nil {
				t.Fatal(err)
			}
			if name == "caller_body" {
				root := m.FuncByName("copied_packet").Body
				ret := root.Body[len(root.Body)-1].(*hir.ReturnStmt)
				ret.Val = &hir.IntLitExpr{Val: 0, Ty: ret.Val.ExprTy()}
				if err := hir.Verify(m); err != nil {
					t.Fatal("mutant must remain valid", err)
				}
			}
			for stmt, l := range m.LoopLineage {
				if name != "caller_body" && l.Role == "fast" && (name == "previous" || l.SiteID == weights[0]) {
					if name == "previous" {
						l.PreviousRule = "forged"
					} else if name == "copy_onto_weighted" {
						l.Rule = hir.ForwardCopyRule
					} else {
						l.SiteID = copies[0]
					}
					m.LoopLineage[stmt] = l
					break
				}
			}
			if _, err := hir.ValidateMixedCopyWeightedProof(source, m, choices); err == nil {
				t.Fatal("forgedmixedattribution")
			}
		})
	}
}

func TestMixedCopyWeightedSelectorFreshAndDimensions(t *testing.T) {
	factory := func() (*hir.Module, error) { return nanz.Parse(mixedSimpleSource, "mixed-refusal.nanz") }
	n := int64(0)
	v := int64(65535)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"data": {7, 0}}, Calls: []hir.ProfileInvocation{{Function: "copied_packet", Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 1}, {Resource: "data"}, {Value: &n}, {Value: &v}, {Value: &v}, {Value: &v}}}}}
	source, _ := factory()
	run, err := hir.RunLoopProfile(source, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(run.Profile)
	for _, name := range []string{"nil", "shared", "shared_ast", "prior_policy", "mutated_prior", "stale_source", "forged", "cap_two"} {
		t.Run(name, func(t *testing.T) {
			f := factory
			observations := p
			cap := 4
			shared, _ := factory()
			switch name {
			case "nil":
				f = nil
			case "shared":
				f = func() (*hir.Module, error) { return shared, nil }
			case "shared_ast":
				f = func() (*hir.Module, error) { m, _ := factory(); m.Funcs = shared.Funcs; return m, nil }
			case "prior_policy":
				f = func() (*hir.Module, error) { m, _ := factory(); m.PointerWalkPolicy = map[string]bool{}; return m, nil }
			case "mutated_prior":
				calls := 0
				var first *hir.Module
				f = func() (*hir.Module, error) {
					m, _ := factory()
					calls++
					if calls == 1 {
						first = m
					} else {
						first.ForwardCopyPolicy = map[string]bool{}
					}
					return m, nil
				}
			case "stale_source":
				calls := 0
				f = func() (*hir.Module, error) { m, _ := factory(); calls++; m.Name = fmt.Sprint(calls); return m, nil }
			case "forged":
				var forged hir.LoopProfile
				json.Unmarshal(p, &forged)
				forged.Counts[0].Entries++
				observations, _ = json.Marshal(forged)
			case "cap_two":
				cap = 2
			}
			if result, err := MeasureMixedCopyWeightedVariantSets(f, observations, w, Options{AssertMode: "none", UseHIRGrace: true}, cap); err == nil || result != nil {
				t.Fatal("unsafe mixedselector request")
			}
		})
	}
	if _, err := hir.ValidateMixedCopyWeightedProof(source, nil, hir.MixedCopyWeightedChoices{}); err == nil {
		t.Fatal("nil actual accepted")
	}
	// One observed copy plus two weighted sites needs five dimensions, although
	// there are only three unique source SiteIDs. Refuse before factory fanout.
	extra := strings.Replace(weightedPrototypeSource, "fun weighted(", "fun extra_weighted(", 1)
	larger := mixedSimpleSource + extra
	capFactory := func() (*hir.Module, error) { return nanz.Parse(larger, "mixed-cap.nanz") }
	w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "extra_weighted", Arguments: []hir.ProfileArgument{{Resource: "data"}, {Value: &n}, {Value: &v}, {Value: &v}, {Value: &v}}})
	m, _ := capFactory()
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ = json.Marshal(raw.Profile)
	if result, err := MeasureMixedCopyWeightedVariantSets(capFactory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4); err == nil || result != nil {
		t.Fatal("weightedtwo-bit cap bypass")
	}
	// Calling only the copy kernel leaves weighted original and checked by proof.
	w.Calls = []hir.ProfileInvocation{{Function: "copy_forward", Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 1}, {Resource: "data"}, {Value: &n}}}}
	m, _ = factory()
	raw, err = hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ = json.Marshal(raw.Profile)
	r, err := MeasureMixedCopyWeightedVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil || len(r.Evidence.Candidates) != 2 {
		t.Fatal("unobservedweighted dimensions", err)
	}
	for _, c := range r.Evidence.Candidates {
		if len(c.Proofs.Weighted) != 1 || c.Proofs.Weighted[0].Scalar || c.Proofs.Weighted[0].Pointer {
			t.Fatal("unobservedweighted changed")
		}
	}
}

func TestMixedCopyWeightedTypedInternCaller(t *testing.T) {
	f := buildTypedInternFixture(t)
	source, err := f.factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(source, f.workload)
	if err != nil {
		t.Fatal(err)
	}
	f.checkRaw(t, raw)
	observations, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("MINZ_MIXED_INTERN_ARTIFACTS")
	write := func(name string, v any) {
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		var data []byte
		if b, ok := v.([]byte); ok {
			data = b
		} else {
			data, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("source.nanz", f.source)
	write("workload.json", f.workload)
	write("observations.json", raw.Profile)
	write("oracle.json", map[string]any{"results": f.results, "snapshots": f.snapshots, "reads": f.reads, "writes": f.writes})
	for _, size := range []bool{false, true} {
		r, err := MeasureMixedCopyWeightedVariantSets(f.factory, observations, f.workload, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			write(fmt.Sprintf("size-%t-failure.txt", size), []byte(err.Error()))
			t.Fatal(err)
		}
		e := r.Evidence
		if len(e.Candidates) != 8 || len(e.EligibleCopySites) != 1 || len(e.EligibleWeightedSites) != 1 {
			t.Fatal("intern candidate scope")
		}
		best := 0
		for i, c := range e.Candidates {
			f.checkRaw(t, c.RawRun)
			f.checkRaw(t, c.FinalRawRun)
			f.checkTarget(t, c.Run)
			if len(c.Proofs.Copies) != 1 || len(c.Proofs.Weighted) != 1 || len(c.FinalProofs.Copies) != 1 || len(c.FinalProofs.Weighted) != 1 {
				t.Fatal("intern missing proof")
			}
			if c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) || c.Run.BinarySHA256 != c.BinarySHA256 || c.ModuleBytes != len(c.Binary) {
				t.Fatal("intern binary evidence")
			}
			if (!size && c.TotalTStates < e.Candidates[best].TotalTStates) || (size && c.ModuleBytes < e.Candidates[best].ModuleBytes) {
				best = i
			}
			t.Logf("intern size=%t mask%d %dT/%dB", size, i, c.TotalTStates, c.ModuleBytes)
		}
		if e.Selected != best || !reflect.DeepEqual(r.Binary, e.Candidates[best].Binary) {
			t.Fatal("intern measured selection")
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, source, f.workload, *raw)
		if err != nil {
			t.Fatal(err)
		}
		f.checkTarget(t, replay)
		if replay.BinarySHA256 != e.Candidates[best].BinarySHA256 {
			t.Fatal("intern retained binary")
		}
		digest := *e
		digest.SHA256 = ""
		data, _ := json.Marshal(digest)
		if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
			t.Fatal("intern evidence digest")
		}
		write(fmt.Sprintf("size-%t-evidence.json", size), e)
		write(fmt.Sprintf("size-%t-selected.a80", size), []byte(r.Steps.Assembly))
		write(fmt.Sprintf("size-%t-selected.bin", size), r.Binary)
	}
}

func TestMixedCopyWeightedFourDimensionBoundary(t *testing.T) {
	source := mixedSimpleSource + strings.Replace(forwardCopySource, "fun copy_forward(", "fun extra_copy(", 1)
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "mixed-four.nanz") }
	zero := int64(0)
	one := int64(1)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Resources: map[string][]byte{"data": {7, 0}}, Calls: []hir.ProfileInvocation{
		{Function: "copied_packet", Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 1}, {Resource: "data"}, {Value: &zero}, {Value: &one}, {Value: &one}, {Value: &one}}},
		{Function: "extra_copy", Arguments: []hir.ProfileArgument{{Resource: "data", Offset: 1}, {Resource: "data"}, {Value: &zero}}},
	}}
	m, _ := factory()
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(raw.Profile)
	calls := 0
	counted := func() (*hir.Module, error) { calls++; return factory() }
	r, err := MeasureMixedCopyWeightedVariantSets(counted, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Evidence.Candidates) != 16 || len(r.Evidence.EligibleCopySites) != 2 || len(r.Evidence.EligibleWeightedSites) != 1 || calls != 17 {
		t.Fatal("four dimension bound", calls)
	}
	for _, c := range r.Evidence.Candidates {
		if len(c.Proofs.Copies) != 2 || len(c.FinalProofs.Copies) != 2 {
			t.Fatal("missing boundary proof")
		}
	}
}

func TestMixedCopyWeightedLoopNodeAttributionRefusals(t *testing.T) {
	src := mixedSimpleSource + `fun ordinary(n:u8)->u8 {var i:u8=0
 while i<n {i=i+1}
 var j:u8=0
 while j<n {j=j+1}
 return i}`
	factory := func() *hir.Module {
		m, e := nanz.Parse(src, "mixed-attribution.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return m
	}
	for _, mutant := range []string{"return_marker", "unmapped_source", "missing_loop_marker", "swapped"} {
		t.Run(mutant, func(t *testing.T) {
			source, actual := factory(), factory()
			if mutant == "unmapped_source" {
				delete(source.SourceLocations, source.FuncByName("ordinary").Body.Body[1])
				delete(actual.SourceLocations, actual.FuncByName("ordinary").Body.Body[1])
			}
			if err := hir.SeedLoopLineage(actual); err != nil {
				t.Fatal(err)
			}
			loop := actual.FuncByName("ordinary").Body.Body[1]
			if mutant == "swapped" {
				other := actual.FuncByName("ordinary").Body.Body[3]
				actual.LoopLineage[loop], actual.LoopLineage[other] = actual.LoopLineage[other], actual.LoopLineage[loop]
			} else if mutant != "unmapped_source" {
				l := actual.LoopLineage[loop]
				delete(actual.LoopLineage, loop)
				if mutant == "return_marker" {
					actual.LoopLineage[actual.FuncByName("ordinary").Body.Body[4]] = l
				}
			}
			if err := hir.Verify(actual); err != nil {
				t.Fatal(err)
			}
			if _, err := hir.ValidateMixedCopyWeightedProof(source, actual, hir.MixedCopyWeightedChoices{}); err == nil {
				t.Fatal("forged loop node attribution", mutant)
			}
		})
	}
}
