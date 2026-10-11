package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func TestMemoryHostLoopAssociationOriginalParser(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	factory := func() (*hir.Module, error) { m, _ := memoryHostParserFixture(t); return m, nil }
	r, err := RunZ80MemoryHostLoopProfile(factory, w)
	if err != nil {
		t.Fatal(err)
	}
	exportMemoryHostReferenceBeforeAssert(t, "association", r.Baseline.Target, map[string]any{"loop-baseline-association.json": r})
	if r.Profile.CompletedCalls != 30 || r.Profile.FailedCalls != 0 || len(r.Profile.Counts) != 11 {
		t.Fatal("wrong invocation/site boundary")
	}
	found := false
	for i, s := range r.Profile.Sites.Sites {
		if s.Function == "region_match" {
			found = true
			if r.Profile.Counts[i].Entries != 49 || r.Profile.Counts[i].BodyEntries != 59 {
				t.Fatal(r.Profile.Counts[i])
			}
		}
	}
	if !found {
		t.Fatal("region_match absent")
	}
	if r.Baseline.Target.TotalTStates != 1269009 || r.Baseline.Target.BinarySHA256 != "27a7c8ea7b77725ee5f61e9f4be625bd1b582b1b03e1013021e0ab3766ee8545" {
		t.Fatal("instrumentation changed native baseline")
	}
	if dir := os.Getenv("MINZ_MEMORY_HOST_LOOP_ARTIFACTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "loop-baseline-association.json"), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMemoryHostLoopAssociationFailureAndGas(t *testing.T) {
	const source = `@extern fun peek(addr:u16)->u8
@extern fun poke(addr:u16,val:u8)->void
fun run()->u8 {
 var i:u8=0
 while i<3 {
  poke(32768,i)
  peek(32769)
  i=i+1
 }
 return i
}`
	factory := func() (*hir.Module, error) { return nanz.Parse(source, "association-failure.nanz") }
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"ram": {Address: 0x8000, Bytes: []byte{9}, Writable: true}}, Calls: []hir.MemoryHostInvocation{{Function: "run"}, {Function: "run"}}}
	r, err := RunZ80MemoryHostLoopProfile(factory, w)
	if err != nil {
		t.Fatal(err)
	}
	if r.Profile.CompletedCalls != 0 || r.Profile.FailedCalls != 1 || len(r.Profile.Invocations) != 1 {
		t.Fatal("failed prefix boundary")
	}
	o := r.Profile.Invocations[0]
	if o.Observation.Failure == nil || o.Observation.Failure.Address != 0x8001 || o.Observation.Regions["ram"][0] != 0 || o.Counts[0].Entries != 1 || o.Counts[0].BodyEntries != 1 {
		t.Fatal(o)
	}
	w.MaxSteps = 1
	if r, err := RunZ80MemoryHostLoopProfile(factory, w); err == nil || r != nil {
		t.Fatal("gas attested as semantic failure")
	}
}

func TestMemoryHostLoopAssociationRejectsDelayedSourceMutation(t *testing.T) {
	_, w := memoryHostParserFixture(t)
	for _, delayed := range []int{4, 5} {
		t.Run(string(rune('0'+delayed)), func(t *testing.T) {
			calls := 0
			var original *hir.Module
			factory := func() (*hir.Module, error) {
				m, _ := memoryHostParserFixture(t)
				calls++
				if calls == 1 {
					original = m
				}
				if calls == delayed {
					original.Name = "mutated-retained-source"
				}
				return m, nil
			}
			if r, err := RunZ80MemoryHostLoopProfile(factory, w); err == nil || r != nil {
				t.Fatal("retained source mutation accepted")
			}
		})
	}
}
