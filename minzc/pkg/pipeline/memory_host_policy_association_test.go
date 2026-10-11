package pipeline

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const policyAssociationSource = `@extern fun peek(addr:u16)->u8
fun loop(n:u8)->u8 {
 var i:u8=0
 while i<n { i=i+1 }
 return i
}`

func policyAssociationFixture(t *testing.T) (func() (*hir.Module, error), hir.MemoryHostWorkload) {
	t.Helper()
	f := func() (*hir.Module, error) {
		m, e := nanz.Parse(policyAssociationSource, "policy-association.nanz")
		if e == nil {
			m.Target = hir.TargetCPM
		}
		return m, e
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"data": {Address: 0x8000, Bytes: []byte{7}}}, Calls: []hir.MemoryHostInvocation{{Function: "loop", Arguments: []int64{0}}, {Function: "loop", Arguments: []int64{3}}}}
	return f, w
}
func policyAssociationData(t *testing.T) ([]byte, func() (*hir.Module, error), hir.MemoryHostWorkload) {
	t.Helper()
	f, w := policyAssociationFixture(t)
	a, e := RunZ80MemoryHostLoopProfileWithOptions(f, w, MemoryHostBaselineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(a)
	if e != nil {
		t.Fatal(e)
	}
	return b, f, w
}
func TestMemoryHostPolicyAssociationControl(t *testing.T) {
	f, w := policyAssociationFixture(t)
	old, e := RunZ80MemoryHostLoopProfile(f, w)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []mir2.UnsignedPairComparePolicy{0, 1} {
		o := MemoryHostBaselineOptions{UnsignedPairComparePolicy: p}
		a, e := RunZ80MemoryHostLoopProfileWithOptions(f, w, o)
		if e != nil {
			t.Fatal(e)
		}
		if p == 0 && (!reflect.DeepEqual(a.Profile, old.Profile) || !reflect.DeepEqual(a.Baseline.Baseline, old.Baseline)) {
			t.Fatal("Legacy changed")
		}
		if len(a.Profile.Counts) != 1 || a.Profile.Counts[0].Entries != 2 || a.Profile.Counts[0].BodyEntries != 3 {
			t.Fatal("independent counts")
		}
		for i, want := range []int64{0, 3} {
			if a.Baseline.Baseline.Target.Invocations[i].Results[0].I != want {
				t.Fatal("independent result")
			}
		}
		b, _ := json.MarshalIndent(a, "", "  ")
		s, _ := f()
		l, e := LoadMemoryHostLoopProfileEvidenceV2(bytes.NewReader(b), s, w, o)
		if e != nil || !reflect.DeepEqual(l, a) {
			t.Fatal("load", e)
		}
		calls := 0
		fresh := func() (*hir.Module, error) { calls++; runtime.GC(); return f() }
		r, e := ReplayMemoryHostLoopProfileEvidenceV2(fresh, w, b, o)
		if e != nil || !reflect.DeepEqual(r, a) || calls != 6 {
			t.Fatal("replay", calls, e)
		}
	}
}
func TestMemoryHostPolicyAssociationStrictEnvelope(t *testing.T) {
	b, f, w := policyAssociationData(t)
	changes := map[string]func(*MemoryHostLoopProfileEvidenceV2){"schema": func(a *MemoryHostLoopProfileEvidenceV2) { a.Schema = "v1" }, "policy": func(a *MemoryHostLoopProfileEvidenceV2) { a.CompileOptions.UnsignedPairComparePolicy = 1 }, "nestedPolicy": func(a *MemoryHostLoopProfileEvidenceV2) { a.Baseline.CompileOptions.UnsignedPairComparePolicy = 1 }, "nilBaseline": func(a *MemoryHostLoopProfileEvidenceV2) { a.Baseline = nil }, "context": func(a *MemoryHostLoopProfileEvidenceV2) { a.Baseline.Baseline.Target.Context.CompilerID = "stale" }, "cost": func(a *MemoryHostLoopProfileEvidenceV2) { a.Baseline.Baseline.Target.TotalTStates++ }, "binary": func(a *MemoryHostLoopProfileEvidenceV2) { a.Baseline.Baseline.Target.Binary[0] ^= 1 }, "result": func(a *MemoryHostLoopProfileEvidenceV2) { a.Baseline.Baseline.Target.Invocations[0].Results[0].I = 99 }, "sums": func(a *MemoryHostLoopProfileEvidenceV2) { a.Profile.Counts[0].Entries++ }, "failed": func(a *MemoryHostLoopProfileEvidenceV2) { a.Profile.FailedCalls = 1 }}
	for n, c := range changes {
		t.Run(n, func(t *testing.T) {
			var a MemoryHostLoopProfileEvidenceV2
			json.Unmarshal(b, &a)
			c(&a)
			x, _ := json.Marshal(a)
			s, _ := f()
			if l, e := LoadMemoryHostLoopProfileEvidenceV2(bytes.NewReader(x), s, w, MemoryHostBaselineOptions{}); e == nil || l != nil {
				t.Fatal("forgery accepted")
			}
		})
	}
	for _, x := range [][]byte{[]byte("null"), []byte("{}"), append(append([]byte{}, b...), []byte(" {}")...), bytes.Replace(b, []byte(`"unsigned_pair_compare_policy":0`), []byte(`"unsigned_pair_compare_policy":0,"unsigned_pair_compare_policy":0`), 1), bytes.Replace(b, []byte(`"unsigned_pair_compare_policy":0`), []byte(`"Unsigned_pair_compare_policy":0`), 1), bytes.Replace(b, []byte(`"unsigned_pair_compare_policy":0`), []byte(`"extra":0`), 1), bytes.Replace(b, []byte(`"unsigned_pair_compare_policy":0`), []byte(`"unsigned_pair_compare_policy":0.0`), 1)} {
		s, _ := f()
		if l, e := LoadMemoryHostLoopProfileEvidenceV2(bytes.NewReader(x), s, w, MemoryHostBaselineOptions{}); e == nil || l != nil {
			t.Fatal("noncanonical accepted")
		}
	}
}
func TestMemoryHostPolicyAssociationFreshReplayRefusals(t *testing.T) {
	b, f, w := policyAssociationData(t)
	var a MemoryHostLoopProfileEvidenceV2
	json.Unmarshal(b, &a)
	a.Profile.Counts[0].Entries++
	a.Profile.Invocations[0].Counts[0].Entries++
	x, _ := json.Marshal(a)
	s, _ := f()
	if _, e := LoadMemoryHostLoopProfileEvidenceV2(bytes.NewReader(x), s, w, MemoryHostBaselineOptions{}); e != nil {
		t.Fatal("mustselfconsistent", e)
	}
	if r, e := ReplayMemoryHostLoopProfileEvidenceV2(f, w, x, MemoryHostBaselineOptions{}); e == nil || r != nil || !strings.Contains(e.Error(), "fresh closed replay") {
		t.Fatal("forged counts", e)
	}
	for _, at := range []int{3, 6} {
		calls := 0
		var first *hir.Module
		drift := func() (*hir.Module, error) {
			calls++
			if calls == at {
				first.Target = hir.TargetZXSpectrum
			}
			m, e := f()
			if calls == 1 {
				first = m
			}
			return m, e
		}
		if r, e := ReplayMemoryHostLoopProfileEvidenceV2(drift, w, b, MemoryHostBaselineOptions{}); e == nil || r != nil {
			t.Fatal("drift", at, e)
		}
	}
	shared, _ := f()
	if r, e := ReplayMemoryHostLoopProfileEvidenceV2(func() (*hir.Module, error) { return shared, nil }, w, b, MemoryHostBaselineOptions{}); e == nil || r != nil {
		t.Fatal("shared")
	}
	calls := 0
	factory := func() (*hir.Module, error) { calls++; return f() }
	if _, e := ReplayMemoryHostLoopProfileEvidenceV2(factory, w, make([]byte, MaxMemoryHostPolicyAssociationBytes+1), MemoryHostBaselineOptions{}); e == nil || calls != 0 {
		t.Fatal("cap")
	}
	if _, e := RunZ80MemoryHostLoopProfileWithOptions(factory, w, MemoryHostBaselineOptions{UnsignedPairComparePolicy: 255}); e == nil || calls != 0 {
		t.Fatal("invalidpolicy")
	}
}

func TestMemoryHostPolicyAssociationPreflight(t *testing.T) {
	b, f, w := policyAssociationData(t)
	calls := 0
	factory := func() (*hir.Module, error) { calls++; return f() }
	for _, bad := range [][]byte{[]byte("null"), []byte("{}"), bytes.Replace(b, []byte(`"unsigned_pair_compare_policy":0`), []byte(`"unsigned_pair_compare_policy":1`), 1), bytes.Replace(b, []byte(`"schema":"minz.memory-host-baseline/v2"`), []byte(`"schema":"v1"`), 1), bytes.Replace(b, []byte(`"unsigned_pair_compare_policy":0`), []byte(`"unsigned_pair_compare_policy":0,"unsigned_pair_compare_policy":0`), 1)} {
		calls = 0
		if a, e := ReplayMemoryHostLoopProfileEvidenceV2(factory, w, bad, MemoryHostBaselineOptions{}); e == nil || a != nil || calls != 0 {
			t.Fatal("bad envelope invoked factory", calls, e)
		}
	}
	source, _ := f()
	if a, e := LoadMemoryHostLoopProfileEvidenceV2(nil, source, w, MemoryHostBaselineOptions{}); e == nil || a != nil {
		t.Fatal("nil reader accepted")
	}
}
