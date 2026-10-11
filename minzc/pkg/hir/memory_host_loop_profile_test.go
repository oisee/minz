package hir_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
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

func TestMemoryHostLoopProfileGasIsIncomplete(t *testing.T) {
	m, w := memoryLoopFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "nested", Arguments: []int64{255, 0}}}
	w.MaxSteps = 25
	r, e := hir.RunMemoryHostLoopProfile(m, w)
	if e == nil || r == nil || r.CompletedCalls != 0 || r.FailedCalls != 1 || len(r.Invocations) != 1 || r.Invocations[0].Completed || r.Invocations[0].Observation.Failure != nil || r.Invocations[0].Observation.Error == "" {
		t.Fatal("gas failure attested as success or byte denial", r, e)
	}
	if r.Counts[0].Entries != 1 || r.Counts[0].BodyEntries == 0 || r.Counts[0].BodyEntries >= 255 {
		t.Fatal("gas prefix lost", r.Counts)
	}
}
func TestMemoryHostLoopProfileDetachedOriginsAndSourceChanges(t *testing.T) {
	m, w := memoryLoopFixture(t)
	w.Calls = w.Calls[:1]
	a, e := hir.RunMemoryHostLoopProfile(m, w)
	if e != nil {
		t.Fatal(e)
	}
	changed, e := nanz.Parse(memoryLoopSrc+"\n// changed source unit\n", "memory-loop.nanz")
	if e != nil {
		t.Fatal(e)
	}
	b, e := hir.RunMemoryHostLoopProfile(changed, w)
	if e != nil || reflect.DeepEqual(a.Compatibility, b.Compatibility) || a.Sites.Sites[0].Origin.SourceSHA256 == b.Sites.Sites[0].Origin.SourceSHA256 {
		t.Fatal("source fingerprint missing", e)
	}
	var origin hir.SourceLocation
	for _, l := range m.SourceLocations {
		origin = l
		break
	}
	m.SourceLocations[&hir.ReturnStmt{}] = origin
	if r, e := hir.RunMemoryHostLoopProfile(m, w); e == nil || r != nil {
		t.Fatal("detached provenance accepted")
	}
}
func TestMemoryHostLoopProfileUserHookNamespaceIsNotAuthority(t *testing.T) {
	m, w := memoryLoopFixture(t)
	forged, e := nanz.Parse(memoryLoopSrc+"\n@extern fun __minz_loop_profile_0_entry()->void\n", "memory-loop.nanz")
	if e != nil {
		t.Fatal(e)
	}
	if r, e := hir.RunMemoryHostLoopProfile(forged, w); e == nil || r != nil {
		t.Fatal("user hook prefix accepted")
	}
	// A normal same-named function is harmless: instrumentation must pick fresh
	// names, not replace its definition or give it counter authority.
	legal, e := nanz.Parse(memoryLoopSrc+"\nfun __minz_loop_profile_0_entry()->u8{return 7}\n", "memory-loop.nanz")
	if e != nil {
		t.Fatal(e)
	}
	w.Calls = w.Calls[:2]
	a, e := hir.RunMemoryHostLoopProfile(m, w)
	if e != nil {
		t.Fatal(e)
	}
	b, e := hir.RunMemoryHostLoopProfile(legal, w)
	if e != nil || len(b.Counts) != len(a.Counts) || b.Counts[0].Entries != a.Counts[0].Entries || b.Counts[0].BodyEntries != a.Counts[0].BodyEntries {
		t.Fatal("fresh hook namespace failure", e)
	}
}

func TestMemoryHostLoopProfileGlobalHookCollision(t *testing.T) {
	m, e := nanz.Parse(memoryLoopSrc+"\nglobal __minz_loop_profile_0_entry:u8=77\n", "memory-loop.nanz")
	if e != nil {
		t.Fatal(e)
	}
	_, w := memoryLoopFixture(t)
	w.Calls = w.Calls[:2]
	w.Globals = map[string][]byte{"__minz_loop_profile_0_entry": {77}}
	r, e := hir.RunMemoryHostLoopProfile(m, w)
	if e != nil || r == nil || r.Counts[0].Entries != 4 || r.Counts[0].BodyEntries != 4 || r.Invocations[1].Observation.Globals["__minz_loop_profile_0_entry"][0] != 77 {
		t.Fatal("global/hook namespace collision", r, e)
	}
}

func TestMemoryHostLoopProfileRefusesGotoCounterBypass(t *testing.T) {
	m, w := memoryLoopFixture(t)
	f := m.FuncByName("nested")
	var body *hir.Block
	for _, stmt := range f.Body.Body {
		if loop, ok := stmt.(*hir.WhileStmt); ok {
			body = loop.Body
		}
	}
	if body == nil {
		t.Fatal("missing loop")
	}
	body.Body = []hir.Stmt{&hir.LabelStmt{Name: "inside"}, &hir.ReturnStmt{Val: hir.U8(7)}}
	f.Body.Body = append([]hir.Stmt{&hir.GotoStmt{Label: "inside"}}, f.Body.Body...)
	if e := hir.Verify(m); e != nil {
		t.Fatal("fixture must be valid HIR", e)
	}
	if r, e := hir.RunMemoryHostLoopProfile(m, w); e == nil || r != nil {
		t.Fatal("inward goto bypass accepted", r, e)
	}
}

func TestMemoryHostLoopProfileInstrumentedSymbolExtent(t *testing.T) {
	m, w := memoryLoopFixture(t)
	w.Calls = w.Calls[:1]
	baseEnd := mir2.NewVM(hir.LowerModule(m)).AllocHeap(nil).I
	w.Regions["ram"] = hir.MemoryHostRegion{Address: uint16(baseEnd + 1), Bytes: []byte{9}, Writable: true}
	if r, e := hir.RunMemoryHostBaseline(m, w); e != nil || r == nil {
		t.Fatal("baseline storage fixture", e)
	}
	if r, e := hir.RunMemoryHostLoopProfile(m, w); e == nil || r != nil {
		t.Fatal("instrumented symbols overlapped configured RAM", r, e)
	}
}
func TestMemoryHostLoopProfileConditionFailureAndReset(t *testing.T) {
	src := `@extern fun peek(addr:u16)->u8
@extern fun poke(addr:u16,val:u8)->void
fun condition()->u8{while peek(32769)!=0 {poke(32768,1)} return 7}
fun visit()->u8{while peek(32768)>0 {poke(32768,peek(32768)-1)} return peek(32768)}
`
	m, e := nanz.Parse(src, "memory-condition.nanz")
	if e != nil {
		t.Fatal(e)
	}
	_, w := memoryLoopFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "visit"}, {Function: "visit", Patches: []hir.MemoryHostPatch{{Region: "ram", Clear: true, Bytes: []byte{2}}}}, {Function: "condition"}}
	w.Regions["ram"] = hir.MemoryHostRegion{Address: 0x8000, Bytes: []byte{1}, Writable: true}
	r, e := hir.RunMemoryHostLoopProfile(m, w)
	if e == nil || r == nil || len(r.Invocations) != 3 {
		t.Fatal(r, e)
	}
	for i, s := range r.Sites.Sites {
		if s.Function == "condition" && (r.Counts[i].Entries != 1 || r.Counts[i].BodyEntries != 0) {
			t.Fatal("condition failure counted body", r.Counts)
		}
		if s.Function == "visit" && (r.Invocations[0].Counts[i].BodyEntries != 1 || r.Invocations[1].Counts[i].BodyEntries != 2) {
			t.Fatal("reset delta wrong", r.Invocations)
		}
	}
	if r.Invocations[1].Observation.Regions["ram"][0] != 0 || r.Invocations[2].Observation.Failure == nil || len(r.Invocations[2].Observation.Events) != 0 {
		t.Fatal(r.Invocations)
	}
}
