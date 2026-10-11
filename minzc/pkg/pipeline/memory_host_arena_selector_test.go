package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"runtime"
	"strings"
	"testing"
)

func TestMemoryHostArenaSelectorOriginal30(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	original, e := RunZ80MemoryHostLoopProfile(factory, w)
	if e != nil {
		t.Fatal(e)
	}
	data, e := json.Marshal(original.Profile)
	if e != nil {
		t.Fatal(e)
	}
	for _, goal := range []string{"speed", "size"} {
		r, e := MeasureMemoryHostArenaPeel(factory, w, data, MemoryHostArenaSelectorOptions{Goal: goal, MaxSites: 1})
		if e != nil {
			t.Fatal(goal, e)
		}
		if len(r.Candidates) != 2 || r.Candidates[0].Evidence.Target.TotalTStates != 1221494 {
			t.Fatal("baseline changed", r)
		}
		t.Logf("%s selected%d original%dT/%dB peeled%dT/%dB", goal, r.Selected, r.Candidates[0].Evidence.Target.TotalTStates, len(r.Candidates[0].Evidence.Target.Binary), r.Candidates[1].Evidence.Target.TotalTStates, len(r.Candidates[1].Evidence.Target.Binary))
		if goal == "speed" && r.Selected != 1 || goal == "size" && r.Selected != 0 {
			t.Fatal("wrong winner", goal, r.Selected)
		}
	}
}
func TestMemoryHostArenaProfileLoaderStrict(t *testing.T) {
	const src = `@extern fun peek(addr:u16)->u8
fun run()->u8{var i:u8=0 while i<1{i=i+1} return i}`
	m, e := nanz.Parse(src, "loader.nanz")
	if e != nil {
		t.Fatal(e)
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"r": {Address: 0x8000, Bytes: []byte{1}}}, Calls: []hir.MemoryHostInvocation{{Function: "run"}}}
	p, e := hir.RunMemoryHostLoopProfile(m, w)
	if e != nil {
		t.Fatal(e)
	}
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = hir.LoadMemoryHostLoopProfile(strings.NewReader(string(b)), m, w); e != nil {
		t.Fatal(e)
	}
	for name, data := range map[string]string{"duplicate": strings.Replace(string(b), `"failed_calls":0`, `"failed_calls":0,"failed_calls":0`, 1), "missingzero": strings.Replace(string(b), `"failed_calls":0,`, "", 1), "alias": strings.Replace(string(b), `"failed_calls"`, `"Failed_Calls"`, 1), "unknown": strings.Replace(string(b), `"schema":`, `"unknown":0,"schema":`, 1), "v5": strings.Replace(string(b), "minz.memory-host-loop-observations/v1", "minz.input-hir-loop-observations/v5", 1)} {
		t.Run(name, func(t *testing.T) {
			if _, e := hir.LoadMemoryHostLoopProfile(strings.NewReader(data), m, w); e == nil {
				t.Fatal("forged JSON accepted")
			}
		})
	}
}

func TestMemoryHostArenaSourceGraphRetention(t *testing.T) {
	m, e := nanz.Parse("fun run()->u8{return 7}", "retention.nanz")
	if e != nil {
		t.Fatal(e)
	}
	seen := map[uintptr]bool{}
	if e = jointCheckFreshHIR(m, seen); e != nil {
		t.Fatal(e)
	}
	roots := []any{}
	retainArenaSourceGraph(m, &roots)
	old := m.Funcs[0].Body.Body[0]
	m.Funcs[0].Body = nil
	runtime.GC()
	for i := 0; i < 32; i++ {
		fresh, e := nanz.Parse("fun run()->u8{return 7}", "retention.nanz")
		if e != nil {
			t.Fatal(e)
		}
		runtime.GC()
		if e = jointCheckFreshHIR(fresh, seen); e != nil {
			t.Fatal("GC address reuse escaped retention", e)
		}
		retainArenaSourceGraph(fresh, &roots)
	}
	shared, e := nanz.Parse("fun run()->u8{return 7}", "retention.nanz")
	if e != nil {
		t.Fatal(e)
	}
	shared.Funcs[0].Body.Body[0] = old
	if e = jointCheckFreshHIR(shared, seen); e == nil {
		t.Fatal("real shared statement accepted")
	}
	runtime.KeepAlive(roots)
}

func TestMemoryHostArenaSelectorOversizeBeforeFactory(t *testing.T) {
	called := false
	factory := func() (*hir.Module, error) { called = true; return nil, nil }
	if r, e := MeasureMemoryHostArenaPeel(factory, hir.MemoryHostWorkload{}, make([]byte, hir.MaxMemoryHostLoopProfileBytes+1), MemoryHostArenaSelectorOptions{Goal: "speed", MaxSites: 1}); e == nil || r != nil || called {
		t.Fatal("oversize payload entered factory")
	}
}
