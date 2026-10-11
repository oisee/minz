package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"testing"
)

const memoryLoopSrc = `@extern fun peek(addr:u16)->u8
@extern fun poke(addr:u16,val:u8)->void
fun nested(n:u8,depth:u8)->u8 {
 var i:u8=0
 while i<n {
  if depth>0 { nested(1,depth-1) }
  i=i+1
 }
 return i
}
fun fail_loop()->u8 {
 var i:u8=0
 while i<3 {
  poke(32768,i)
  peek(32769)
  i=i+1
 }
 return i
}
`

func memoryLoopFixture(t *testing.T) (*hir.Module, hir.MemoryHostWorkload) {
	t.Helper()
	m, e := nanz.Parse(memoryLoopSrc, "memory-loop.nanz")
	if e != nil {
		t.Fatal(e)
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xefee, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"ram": {Address: 0x8000, Bytes: []byte{9}, Writable: true}}, Calls: []hir.MemoryHostInvocation{{Function: "nested", Arguments: []int64{0, 2}}, {Function: "nested", Arguments: []int64{2, 1}}, {Function: "fail_loop"}, {Function: "nested", Arguments: []int64{3, 0}}}}
	return m, w
}
func TestMemoryHostLoopProfileRecursiveFailurePrefix(t *testing.T) {
	m, w := memoryLoopFixture(t)
	r, e := hir.RunMemoryHostLoopProfile(m, w)
	if e == nil || r == nil {
		t.Fatal(r, e)
	}
	if r.CompletedCalls != 2 || r.FailedCalls != 1 || len(r.Invocations) != 3 || len(r.Counts) != 2 {
		t.Fatal(r)
	}
	if r.Counts[0].Entries != 4 || r.Counts[0].BodyEntries != 4 || r.Counts[1].Entries != 1 || r.Counts[1].BodyEntries != 1 {
		t.Fatal(r.Counts)
	}
	if r.Invocations[0].Counts[0].Entries != 1 || r.Invocations[0].Counts[0].BodyEntries != 0 || r.Invocations[1].Counts[0].Entries != 3 || r.Invocations[1].Counts[0].BodyEntries != 4 {
		t.Fatal(r.Invocations)
	}
	failed := r.Invocations[2]
	if failed.Completed || failed.Observation.Failure == nil || failed.Observation.Failure.Address != 0x8001 || failed.Observation.Regions["ram"][0] != 0 || !reflect.DeepEqual(failed.Observation.Events, []hir.MemoryHostAccess{{Kind: "write", Address: 0x8000, Value: 0}}) {
		t.Fatal(failed)
	}
	baseline, be := hir.RunMemoryHostBaseline(m, w)
	if be == nil || baseline == nil {
		t.Fatal(baseline, be)
	}
	for i, o := range baseline.Invocations {
		got := r.Invocations[i].Observation
		o.Steps = 0
		got.Steps = 0
		o.Error = ""
		got.Error = ""
		if !reflect.DeepEqual(o, got) {
			t.Fatalf("baseline semantic mismatch %d: %#v %#v", i, o, got)
		}
	}
	w.Regions["ram"].Bytes[0] = 88
	w.Calls[0].Arguments[0] = 8
	if r.Workload.Regions["ram"].Bytes[0] != 9 || r.Workload.Calls[0].Arguments[0] != 0 {
		t.Fatal("returned workload aliases caller")
	}
}
func TestMemoryHostLoopProfileSourceIdentityAndMissingOrigins(t *testing.T) {
	m, w := memoryLoopFixture(t)
	w.Calls = w.Calls[:2]
	a, e := hir.RunMemoryHostLoopProfile(m, w)
	if e != nil {
		t.Fatal(e)
	}
	again, e := hir.RunMemoryHostLoopProfile(m, w)
	if e != nil || !reflect.DeepEqual(a, again) {
		t.Fatal("nondeterministic profile", e)
	}
	w.Calls[1].Arguments[0] = 3
	b, e := hir.RunMemoryHostLoopProfile(m, w)
	if e != nil || reflect.DeepEqual(a.Compatibility, b.Compatibility) {
		t.Fatal("workload identity absent", e)
	}
	m.SourceLocations = nil
	if r, e := hir.RunMemoryHostLoopProfile(m, w); e == nil || r != nil {
		t.Fatal("unmapped source accepted")
	}
}
