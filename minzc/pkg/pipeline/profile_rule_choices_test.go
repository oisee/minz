package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func checkIndependentRuleEvidence(t *testing.T, r *ProfileVariantSetResult, pointers int, size bool) {
	t.Helper()
	e := r.Evidence
	if e.Schema != "minz.joint-profile-rule-variant-sets/v2" || len(e.Candidates) != 4*(1<<pointers) {
		t.Fatal("incomplete rule enumeration")
	}
	best := 0
	for i, c := range e.Candidates {
		mask := i >> pointers
		if len(c.RulePolicy) != 2 || c.RulePolicy["hir-constant-if"] != (mask&1 != 0) || c.RulePolicy["hir-invariant-base"] != (mask&2 != 0) || c.CommonGraceRules != (mask == 3) {
			t.Fatal("incorrect policy", i, c.RulePolicy)
		}
		if c.FinalHIR == "" || !jointSameState(c.FinalRawRun, e.Oracle) {
			t.Fatal("missing final compiled HIR/oracle evidence")
		}
		if !jointSameState(c.RawRun, e.Oracle) || c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) {
			t.Fatal("candidate evidence mismatch")
		}
		var total int64
		for _, inv := range c.Run.Invocations {
			total += inv.TStates
		}
		if total != c.TotalTStates {
			t.Fatal("incorrect whole-program cost")
		}
		better := c.TotalTStates < e.Candidates[best].TotalTStates
		if size {
			better = c.ModuleBytes < e.Candidates[best].ModuleBytes
		}
		if better {
			best = i
		}
	}
	if e.Selected != best || !bytes.Equal(r.Binary, e.Candidates[best].Binary) {
		t.Fatal("winner does not match measured minimum")
	}
	copyEvidence := *e
	copyEvidence.SHA256 = ""
	data, err := json.Marshal(copyEvidence)
	if err != nil || e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal("rule choices are not bound by evidence hash")
	}
}

func TestIndependentGraceRulesJointCaller(t *testing.T) {
	factory, w, p := jointFixture(t)
	for _, size := range []bool{false, true} {
		r, err := MeasureGraceRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		checkIndependentRuleEvidence(t, r, 2, size)
		for _, c := range r.Evidence.Candidates {
			for i, inv := range c.Run.Invocations {
				if inv.Results[0].I != 30 || inv.Snapshot.Globals["calls"][0] != byte(10+i) {
					t.Fatal("independent caller oracle mismatch")
				}
			}
		}
	}
}

func TestIndependentGraceRulesChecksum(t *testing.T) {
	testIndependentChecksum(t, false)
}

func TestIndependentGraceRulesBudgetCompetition(t *testing.T) {
	// Every Grace rewrite rebuilds facts and verifies lowering. Keep this deliberate
	// competition regression intact; run with a suitable test timeout on slow hosts.
	testIndependentChecksum(t, true)
}

func testIndependentChecksum(t *testing.T, budget bool) {
	t.Helper()
	sourcePath := "../../../examples/nanz/grace_weighted_checksum.nanz"
	if budget {
		sourcePath = "testdata/grace_rule_budget.nanz"
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), sourcePath) }
	data, err := os.ReadFile("../../../docs/grace-pgo-scalar-artifacts/workload.json")
	if err != nil {
		t.Fatal(err)
	}
	var w hir.ProfileWorkload
	if err = json.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	// Independent arithmetic and persistent recording state, recomputed from inputs.
	expected := make([]int64, len(w.Calls))
	snapshots := make([]hir.ProfileMemorySnapshot, len(w.Calls))
	calls := w.Globals["checksum_calls"][0]
	last := append([]byte{}, w.Globals["checksum_last"]...)
	for i, c := range w.Calls {
		a := c.Arguments
		sum := uint64(*a[4].Value)
		repetitions := 1
		if budget && c.Function == "checksum_packet" {
			repetitions = 2
		}
		for traversal := 0; traversal < repetitions; traversal++ {
			for _, v := range w.Resources[a[0].Resource][:int(*a[1].Value)] {
				sum += uint64(*a[2].Value) + uint64(*a[3].Value) + uint64(v)
			}
		}
		expected[i] = int64(uint16(sum))
		if c.Function == "checksum_packet" {
			calls++
			last = []byte{byte(sum), byte(sum >> 8)}
		}
		snapshots[i] = hir.ProfileMemorySnapshot{Globals: map[string][]byte{"checksum_calls": {calls}, "checksum_last": append([]byte{}, last...)}, Resources: w.Resources}
	}
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	if budget {
		if len(raw.Profile.Sites.Sites) != 2 || len(raw.Profile.Guards) != 0 {
			t.Fatal("fixture sites/guards changed")
		}
		checkBudgetAccessOracle(t, raw, w)
	}
	p, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if !budget {
		// The real checksum now has overlapping pointer/scalar opportunities;
		// legacy module searches must require the explicit structural v4 proof.
		for _, size := range []bool{false, true} {
			options := Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}
			if r, err := MeasureGraceRuleVariantSets(factory, p, w, options, 4); err == nil || r != nil {
				t.Fatal("legacy weighted overlap accepted")
			}
			r, err := MeasureWeightedJointSourceSiteVariantSets(factory, p, w, options, 4)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range r.Evidence.Candidates {
				for j, inv := range c.Run.Invocations {
					if inv.Results[0].I != expected[j] || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) {
						t.Fatal("weighted independent host oracle", j)
					}
				}
			}
			want := 3
			if size {
				want = 2
			}
			if r.Evidence.Selected != want || !bytes.Equal(r.Binary, r.Evidence.Candidates[want].Binary) {
				t.Fatal("weighted winner", r.Evidence.Selected)
			}
		}
		return
	}
	for _, size := range []bool{false, true} {
		r, err := MeasureGraceRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		checkIndependentRuleEvidence(t, r, 0, size)
		for i, c := range r.Evidence.Candidates {
			hoisted := false
			for _, l := range c.Lineage {
				hoisted = hoisted || l.Lineage.Rule == "hir-invariant-base"
			}
			wantHoisted := i&2 != 0
			if budget {
				wantHoisted = i == 2
			}
			if hoisted != wantHoisted {
				t.Fatal("policy was not reflected in actual lineage")
			}
			for j, inv := range c.Run.Invocations {
				if inv.Results[0].I != expected[j] || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) {
					t.Fatal("independent host oracle mismatch", i, j)
				}
			}
			if budget {
				count := 0
				for _, l := range c.FinalLineage {
					if l.Lineage.Rule == "hir-invariant-base" {
						count++
					}
				}
				wantCount := 0
				if i == 2 {
					wantCount = 2
				}
				if count != wantCount {
					t.Fatal("final compiled hoist count", i, count, wantCount)
				}
				checkBudgetAccessOracle(t, c.FinalRawRun, w)
				targetAccess := &hir.ProfileRun{}
				for _, inv := range c.Run.Invocations {
					if inv.Access == nil {
						t.Fatal("target access evidence missing")
					}
					targetAccess.Accesses = append(targetAccess.Accesses, *inv.Access)
				}
				checkBudgetAccessOracle(t, targetAccess, w)
			}
			t.Logf("size=%v candidate=%d policy=%v cost=%dT/%dB", size, i, c.RulePolicy, c.TotalTStates, c.ModuleBytes)
		}
		if budget {
			winner := r.Evidence.Candidates[2]
			if r.Evidence.Selected != 2 {
				t.Fatal("mixed policy did not win", r.Evidence.Selected)
			}
			for _, i := range []int{0, 3} {
				c := r.Evidence.Candidates[i]
				if winner.TotalTStates >= c.TotalTStates || winner.ModuleBytes >= c.ModuleBytes {
					t.Fatal("mixed policy must strictly beat both bundled policies")
				}
			}
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, m, w, *raw)
		if err != nil || !reflect.DeepEqual(replay, r.Evidence.Candidates[r.Evidence.Selected].Run) {
			t.Fatal("retained winner replay mismatch", err)
		}
		if dir := os.Getenv("MINZ_PGO_RULE_ARTIFACT_DIR"); dir != "" {
			if err = os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if budget {
				if err = os.WriteFile(filepath.Join(dir, "budget-source.nanz"), source, 0644); err != nil {
					t.Fatal(err)
				}
				b, err := json.MarshalIndent(map[string]any{"results": expected, "snapshots": snapshots}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, "budget-host-oracle.json"), b, 0644); err != nil {
					t.Fatal(err)
				}
			}
			prefix := "speed"
			if size {
				prefix = "size"
			}
			if budget {
				prefix = "budget-" + prefix
			}
			for name, value := range map[string]any{"evidence": r.Evidence, "workload": w, "profile": raw.Profile} {
				b, err := json.MarshalIndent(value, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, prefix+"-"+name+".json"), b, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err = os.WriteFile(filepath.Join(dir, prefix+"-selected.bin"), r.Binary, 0644); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, prefix+"-selected.a80"), []byte(r.Steps.Assembly), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestIndependentGraceRulesRefuseInvalidInputs(t *testing.T) {
	factory, w, p := jointFixture(t)
	options := Options{AssertMode: "none", UseHIRGrace: true}
	for _, cap := range []int{0, 1, 5} {
		r, err := MeasureGraceRuleVariantSets(factory, p, w, options, cap)
		if err == nil || r != nil {
			t.Fatal("invalid cap accepted", cap)
		}
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
	if r, err := MeasureGraceRuleVariantSets(factory, bad, w, options, 4); err == nil || r != nil {
		t.Fatal("forged observations accepted")
	}
	shared, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	if r, err := MeasureGraceRuleVariantSets(func() (*hir.Module, error) { return shared, nil }, p, w, options, 4); err == nil || r != nil {
		t.Fatal("shared factory accepted")
	}
}

// Independent byte counts: source caller traverses twice and records once.
func checkBudgetAccessOracle(t *testing.T, r *hir.ProfileRun, w hir.ProfileWorkload) {
	t.Helper()
	if r == nil || len(r.Accesses) != len(w.Calls) {
		t.Fatal("missing access evidence")
	}
	for j, c := range w.Calls {
		n := int(*c.Arguments[1].Value)
		reps := uint64(1)
		caller := c.Function == "checksum_packet"
		if caller {
			reps = 2
		}
		snapshot := r.Accesses[j]
		if len(snapshot.Resources) != len(w.Resources) || len(snapshot.Globals) != 2 {
			t.Fatal("incomplete access snapshot")
		}
		for name, a := range snapshot.Resources {
			if len(a.Reads) != len(w.Resources[name]) || len(a.Writes) != len(w.Resources[name]) {
				t.Fatal("resource access widths")
			}
			for i, reads := range a.Reads {
				var want uint64
				if name == c.Arguments[0].Resource && i < n {
					want = reps
				}
				if reads != want || a.Writes[i] != 0 {
					t.Fatal("resource access oracle", j, name, i)
				}
			}
		}
		for name, a := range snapshot.Globals {
			if len(a.Reads) != len(w.Globals[name]) || len(a.Writes) != len(w.Globals[name]) {
				t.Fatal("global access widths")
			}
			for i, reads := range a.Reads {
				var rd, wr uint64
				if caller {
					wr = 1
					if name == "checksum_calls" {
						rd = 1
					}
				}
				if reads != rd || a.Writes[i] != wr {
					t.Fatal("global access oracle", j, name, i)
				}
			}
		}
	}
}

func TestIndependentGraceRulesFourPointerSites(t *testing.T) {
	source := ""
	body := ""
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("sum%d", i)
		source += fmt.Sprintf("fun %s(ptr:^u8,n:u8)->u8 {var s:u8=0\nvar i:u8=0\nwhile i<n {s=s+ptr[i]\ni=i+1}\nreturn s}\n", name)
		body += fmt.Sprintf("var s%d:u8=%s(ptr,n)\n", i, name)
	}
	source += "fun caller(ptr:^u8,n:u8)->u8 {" + body + "return s0+s1+s2+s3}"
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "four-rule-sites.nanz") }
	_, w, _ := jointFixture(t)
	w.Globals = nil
	w.Calls = w.Calls[:1]
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
	r, err := MeasureGraceRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if err != nil {
		t.Fatal(err)
	}
	checkIndependentRuleEvidence(t, r, 4, false)
	for _, c := range r.Evidence.Candidates {
		if c.Run.Invocations[0].Results[0].I != 60 {
			t.Fatal("four-site host oracle")
		}
	}
}
