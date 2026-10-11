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
	sourcePath := "../../../examples/nanz/grace_weighted_checksum.nanz"
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
		for _, v := range w.Resources[a[0].Resource][:int(*a[1].Value)] {
			sum += uint64(*a[2].Value) + uint64(*a[3].Value) + uint64(v)
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
	p, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
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
			if hoisted != (i&2 != 0) {
				t.Fatal("policy was not reflected in actual lineage")
			}
			for j, inv := range c.Run.Invocations {
				if inv.Results[0].I != expected[j] || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) {
					t.Fatal("independent host oracle mismatch", i, j)
				}
			}
			t.Logf("size=%v candidate=%d policy=%v cost=%dT/%dB", size, i, c.RulePolicy, c.TotalTStates, c.ModuleBytes)
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, m, w, *raw)
		if err != nil || !reflect.DeepEqual(replay, r.Evidence.Candidates[r.Evidence.Selected].Run) {
			t.Fatal("retained winner replay mismatch", err)
		}
		if dir := os.Getenv("MINZ_PGO_RULE_ARTIFACT_DIR"); dir != "" {
			if err = os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			prefix := "speed"
			if size {
				prefix = "size"
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
