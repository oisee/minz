package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func TestInvariantSiteRulesMeasuredChoices(t *testing.T) {
	source := invariantSitesSource + `fun choose(x:u16)->u16 { if true { return x+1 } else {return 9} }`
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "site-rules.nanz") }
	_, w, _ := invariantSitesFixture(t, 2, false)
	v := int64(65535)
	w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "choose", Arguments: []hir.ProfileArgument{{Value: &v}}})
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
		r, err := MeasureInvariantSiteRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		e := r.Evidence
		if e.Schema != "minz.invariant-site-rule-variant-sets/v2" || len(e.Candidates) != 8 {
			t.Fatal("incomplete joint choices")
		}
		best := 0
		for i, c := range e.Candidates {
			// The requested rule must change the measured HIR even when MIR folds
			// the same branch later and the two machine-code costs tie.
			if strings.Contains(c.HIR, "if true:bool") != (i < 4) {
				t.Fatal("constant-if request not reflected in HIR", i)
			}
			if c.RulePolicy["hir-constant-if"] != (i >= 4) || !c.RulePolicy["hir-invariant-base"] {
				t.Fatal("incorrect rule mask")
			}
			for j, id := range e.EligibleSites {
				if c.RequestedSitePolicy[id] != (i&(1<<(1-j)) != 0) {
					t.Fatal("incorrect site mask")
				}
			}
			if err := invariantSitesRealized(c.FinalLineage, c.RequestedSitePolicy); err != nil {
				t.Fatal(err)
			}
			if !jointSameState(c.FinalRawRun, e.Oracle) || c.FinalHIR == "" {
				t.Fatal("missing final behavior")
			}
			var total int64
			for j, inv := range c.Run.Invocations {
				want := int64(0)
				if j < len(w.Calls)-1 {
					want = *w.Calls[j].Arguments[2].Value
				}
				if len(inv.Results) != 1 || inv.Results[0].I != want {
					t.Fatalf("host arithmetic candidate%d call%d: %v want%d", i, j, inv.Results, want)
				}
				total += inv.TStates
			}
			if total != c.TotalTStates || c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) || c.Run.BinarySHA256 != c.BinarySHA256 {
				t.Fatal("cost/binary mismatch")
			}
			better := c.TotalTStates < e.Candidates[best].TotalTStates
			if size {
				better = c.ModuleBytes < e.Candidates[best].ModuleBytes
			}
			if better {
				best = i
			}
		}
		if best != e.Selected || !bytes.Equal(r.Binary, e.Candidates[best].Binary) {
			t.Fatal("unmeasured winner")
		}
		copy := *e
		copy.SHA256 = ""
		b, _ := json.Marshal(copy)
		if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(b)) {
			t.Fatal("unbound policy")
		}
		fresh, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, fresh, w, *e.Oracle)
		if err != nil || !jointEqual(replay, e.Candidates[best].Run) {
			t.Fatal("retained binary replay", err)
		}
		t.Logf("size%v winner%d baseline%dT/%dB selected%dT/%dB", size, best, e.Candidates[0].TotalTStates, e.Candidates[0].ModuleBytes, e.Candidates[best].TotalTStates, e.Candidates[best].ModuleBytes)
		if dir := os.Getenv("MINZ_SITE_RULE_ARTIFACTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			b, err := json.MarshalIndent(e, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{fmt.Sprintf("size-%v-evidence.json", size): b, "source.nanz": []byte(source), "profile.json": p} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			b, _ = json.MarshalIndent(w, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, "workload.json"), b, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestInvariantSiteRulesBudgetRefusal(t *testing.T) {
	// Exactly 100 earlier constant-if firings exhaust the first pass. The new
	// dimension must not quietly accept the selected but unrealized scalar hoist.
	source := invariantSitesSource + "fun idle_branches()->u16 {\n" + strings.Repeat("if false {return 1}\n", 100) + "return 7\n}"
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "site-rule-budget.nanz") }
	_, w, _ := invariantSitesFixture(t, 1, true)
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(raw.Profile)
	r, err := MeasureInvariantSiteRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
	if r != nil || err == nil || !strings.Contains(err.Error(), "not realized") {
		t.Fatalf("incomplete first-pass request accepted: %v", err)
	}
}

func TestInvariantSiteRulesConditionalCandidates(t *testing.T) {
	for _, six := range []bool{false, true} {
		source := conditionalInvariantSource
		if six {
			source = conditionalInvariantSixParamSource
		}
		factory := func() (*hir.Module, error) { return nanz.Parse(source, "site-rules-conditional.nanz") }
		w, wants := conditionalInvariantWorkload(six)
		m, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := hir.RunLoopProfile(m, w)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := json.Marshal(raw.Profile)
		r, err := MeasureInvariantSiteRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, UseGrace: true}, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Evidence.Candidates) != 8 {
			t.Fatal("missing conditional masks")
		}
		for _, c := range r.Evidence.Candidates {
			for i, want := range wants {
				if c.Run.Invocations[i].Results[0].I != want {
					t.Fatal("conditional host oracle")
				}
			}
		}
	}
}

func TestInvariantSiteRulesBoundary(t *testing.T) {
	for _, count := range []int{4, 5} {
		source := ""
		w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536}
		for i := 0; i < count; i++ {
			name := fmt.Sprintf("scalar%d", i)
			source += fmt.Sprintf("fun %s(a:u16,b:u16,n:u8)->u16 {var s:u16=0\nvar i:u8=0\nwhile i<n {s=s+(a+b)\ni=i+1}\nreturn s}\n", name)
			a, b, n := int64(65535), int64(2), int64(0)
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{{Value: &a}, {Value: &b}, {Value: &n}}})
		}
		calls := 0
		factory := func() (*hir.Module, error) { calls++; return nanz.Parse(source, "site-rule-boundary.nanz") }
		m, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := hir.RunLoopProfile(m, w)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := json.Marshal(raw.Profile)
		calls = 0
		r, err := MeasureInvariantSiteRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true}, 4)
		if count == 5 {
			if r != nil || err == nil || calls != 1 {
				t.Fatal("cap not rejected before enumeration")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Evidence.Candidates) != 32 || calls != 33 {
			t.Fatal("four sites incomplete")
		}
		for _, c := range r.Evidence.Candidates {
			for _, v := range c.Run.Invocations {
				if v.Results[0].I != 0 {
					t.Fatal("zero-trip host oracle")
				}
			}
		}
	}
}

func TestInvariantSiteRulesRealChecksum(t *testing.T) {
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
	if err := json.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	wants := make([]int64, len(w.Calls))
	snapshots := make([]hir.ProfileMemorySnapshot, len(w.Calls))
	calls := w.Globals["checksum_calls"][0]
	last := append([]byte{}, w.Globals["checksum_last"]...)
	for i, c := range w.Calls {
		args := c.Arguments
		sum := uint64(*args[4].Value)
		for _, b := range w.Resources[args[0].Resource][:int(*args[1].Value)] {
			sum += uint64(*args[2].Value) + uint64(*args[3].Value) + uint64(b)
		}
		wants[i] = int64(uint16(sum))
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
	p, _ := json.Marshal(raw.Profile)
	for _, size := range []bool{false, true} {
		r, err := MeasureInvariantSiteRuleVariantSets(factory, p, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Evidence.Candidates) != 4 {
			t.Fatal("incomplete real-program choices")
		}
		for mask, c := range r.Evidence.Candidates {
			for i, inv := range c.Run.Invocations {
				if inv.Results[0].I != wants[i] || !jointEqual(inv.Snapshot, snapshots[i]) {
					t.Fatal("independent checksum/state", mask, i)
				}
				if inv.Access == nil || !jointEqual(*inv.Access, c.FinalRawRun.Accesses[i]) {
					t.Fatal("exact byte access comparison", mask, i)
				}
			}
		}
		best := r.Evidence.Candidates[r.Evidence.Selected]
		if r.Evidence.Selected != 1 || best.TotalTStates >= r.Evidence.Candidates[0].TotalTStates || best.ModuleBytes >= r.Evidence.Candidates[0].ModuleBytes {
			t.Fatal("checksum winner not strict hoist improvement")
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, m, w, *raw)
		if err != nil || !jointEqual(replay, best.Run) {
			t.Fatal("checksum retained binary replay", err)
		}
		t.Logf("checksum size%v selected%d %dT/%dB -> %dT/%dB", size, r.Evidence.Selected, r.Evidence.Candidates[0].TotalTStates, r.Evidence.Candidates[0].ModuleBytes, best.TotalTStates, best.ModuleBytes)
		if dir := os.Getenv("MINZ_SITE_RULE_ARTIFACTS"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			e, _ := json.MarshalIndent(r.Evidence, "", "  ")
			host, _ := json.MarshalIndent(map[string]any{"results": wants, "snapshots": snapshots}, "", "  ")
			for name, b := range map[string][]byte{fmt.Sprintf("checksum-size-%v-evidence.json", size): e, "checksum-source.nanz": source, "checksum-workload.json": data, "checksum-host-oracle.json": host, "checksum-profile.json": p, fmt.Sprintf("checksum-size-%v-selected.bin", size): r.Binary, fmt.Sprintf("checksum-size-%v-selected.a80", size): []byte(r.Steps.Assembly)} {
				if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestInvariantSiteRulesRefusals(t *testing.T) {
	factory, w, p := invariantSitesFixture(t, 2, false)
	opts := Options{AssertMode: "none", UseHIRGrace: true}
	shared, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	if r, err := MeasureInvariantSiteRuleVariantSets(func() (*hir.Module, error) { return shared, nil }, p, w, opts, 4); r != nil || err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatal("shared input accepted", err)
	}
	var forged hir.LoopProfile
	if err := json.Unmarshal(p, &forged); err != nil {
		t.Fatal(err)
	}
	forged.Counts[0].Entries++
	bad, _ := json.Marshal(forged)
	if r, err := MeasureInvariantSiteRuleVariantSets(factory, bad, w, opts, 4); r != nil || err == nil || !strings.Contains(err.Error(), "observations do not match") {
		t.Fatal("forged profile accepted", err)
	}
	var prior *hir.Module
	mutated := func() (*hir.Module, error) {
		m, err := factory()
		if prior != nil {
			prior.InvariantBasePolicy = map[string]bool{}
		}
		prior = m
		return m, err
	}
	if r, err := MeasureInvariantSiteRuleVariantSets(mutated, p, w, opts, 4); r != nil || err == nil || !strings.Contains(err.Error(), "mutated previously") {
		t.Fatal("mutated previous HIR accepted", err)
	}
}
