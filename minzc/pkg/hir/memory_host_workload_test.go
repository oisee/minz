package hir_test

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"reflect"
	"strings"
	"testing"
)

const memorySrc = `@extern fun peek(addr:u16)->u8
@extern fun poke(addr:u16,val:u8)->void
global calls:u8=0
fun step(addr:u16,v:u8)->u8 { poke(addr,v)
 calls=calls+1
 return peek(addr) }
fun fail(addr:u16)->u8 { poke(addr,7)
 return peek(addr+1) }
`

func memoryFixture(t *testing.T) (*hir.Module, hir.MemoryHostWorkload) {
	t.Helper()
	m, e := nanz.Parse(memorySrc, "memory-host.nanz")
	if e != nil {
		t.Fatal(e)
	}
	return m, hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xefee, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"ram": {Address: 0x8000, Bytes: []byte{9}, Writable: true}}, Globals: map[string][]byte{"calls": {3}}, Calls: []hir.MemoryHostInvocation{{Function: "step", Arguments: []int64{0x8000, 9}}, {Function: "step", Arguments: []int64{0x8000, 255}}}}
}
func TestMemoryHostPersistenceResetAndOwnership(t *testing.T) {
	m, w := memoryFixture(t)
	run, e := hir.RunMemoryHostBaseline(m, w)
	if e != nil {
		t.Fatal(e)
	}
	if len(run.Invocations) != 2 {
		t.Fatal(run)
	}
	for i, o := range run.Invocations {
		v := byte(9)
		if i == 1 {
			v = 255
		}
		if o.Results[0].I != int64(v) || o.Globals["calls"][0] != byte(4+i) || o.Regions["ram"][0] != v || !reflect.DeepEqual(o.Events, []hir.MemoryHostAccess{{Kind: "write", Address: 0x8000, Value: v}, {Kind: "read", Address: 0x8000, Value: v}}) {
			t.Fatal(o)
		}
	}
	owned, ctx, e := hir.PrepareMemoryHostWorkload(m, w)
	if e != nil {
		t.Fatal(e)
	}
	w.Regions["ram"].Bytes[0] = 1
	w.Globals["calls"][0] = 7
	w.Calls[0].Arguments[1] = 1
	if owned.Regions["ram"].Bytes[0] != 9 || owned.Globals["calls"][0] != 3 || owned.Calls[0].Arguments[1] != 9 {
		t.Fatal("ownership lost")
	}
	_, different, e := hir.PrepareMemoryHostWorkload(m, w)
	if e != nil || reflect.DeepEqual(ctx, different) {
		t.Fatal("stale workload context accepted", e)
	}
	owned.Calls[1].Globals = map[string][]byte{"calls": {11}}
	owned.Calls[1].Patches = []hir.MemoryHostPatch{{Region: "ram", Clear: true, Bytes: []byte{2}}}
	owned.Calls[1].Arguments[1] = 2
	run, e = hir.RunMemoryHostBaseline(m, owned)
	if e != nil || run.Invocations[1].Globals["calls"][0] != 12 || run.Invocations[1].Regions["ram"][0] != 2 {
		t.Fatal(run, e)
	}
}
func TestMemoryHostPartialFailurePrefix(t *testing.T) {
	m, w := memoryFixture(t)
	w.Calls = []hir.MemoryHostInvocation{{Function: "fail", Arguments: []int64{0x8000}}}
	run, e := hir.RunMemoryHostBaseline(m, w)
	if e == nil || run == nil || len(run.Invocations) != 1 {
		t.Fatal(run, e)
	}
	o := run.Invocations[0]
	if len(o.Results) != 0 || o.Error == "" || o.Regions["ram"][0] != 7 || !reflect.DeepEqual(o.Events, []hir.MemoryHostAccess{{Kind: "write", Address: 0x8000, Value: 7}}) {
		t.Fatal(o)
	}
	r := w.Regions["ram"]
	r.Writable = false
	w.Regions["ram"] = r
	run, e = hir.RunMemoryHostBaseline(m, w)
	if e == nil || len(run.Invocations[0].Events) != 0 || run.Invocations[0].Regions["ram"][0] != 9 {
		t.Fatal(run, e)
	}
}
func TestMemoryHostStrictSetupRefusals(t *testing.T) {
	mutations := map[string]func(*hir.Module, *hir.MemoryHostWorkload){
		"old_schema":   func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Schema = "minz.profile-workload/v1" },
		"unknown_host": func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Hosts["peek"] = "callback" },
		"extra_host":   func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Hosts["other"] = hir.MemoryHostReadU8 },
		"missing_host": func(m *hir.Module, w *hir.MemoryHostWorkload) { delete(w.Hosts, "poke") },
		"overlap":      func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Regions["other"] = w.Regions["ram"] },
		"overflow": func(m *hir.Module, w *hir.MemoryHostWorkload) {
			w.Regions["other"] = hir.MemoryHostRegion{Address: 65535, Bytes: []byte{0, 0}}
		},
		"stack": func(m *hir.Module, w *hir.MemoryHostWorkload) {
			w.Regions["other"] = hir.MemoryHostRegion{Address: 0xe000, Bytes: []byte{0}}
		},
		"patch": func(m *hir.Module, w *hir.MemoryHostWorkload) {
			w.Calls[0].Patches = []hir.MemoryHostPatch{{Region: "ram", Offset: 1, Bytes: []byte{1}}}
		},
		"u8_argument":   func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Calls[0].Arguments[1] = 256 },
		"u16_argument":  func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Calls[0].Arguments[0] = -1 },
		"global_width":  func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Globals["calls"] = []byte{0, 0} },
		"source_policy": func(m *hir.Module, w *hir.MemoryHostWorkload) { m.PointerWalkPolicy = map[string]bool{} },
	}
	for name, change := range mutations {
		t.Run(name, func(t *testing.T) {
			m, w := memoryFixture(t)
			change(m, &w)
			if _, _, e := hir.PrepareMemoryHostWorkload(m, w); e == nil {
				t.Fatal("accepted invalid setup")
			}
		})
	}
	m, w := memoryFixture(t)
	bad := strings.Replace(memorySrc, "peek(addr:u16)->u8", "peek(addr:u8)->u8", 1)
	m, e := nanz.Parse(bad, "bad.nanz")
	if e == nil {
		if _, _, e = hir.PrepareMemoryHostWorkload(m, w); e == nil {
			t.Fatal("wrong signature accepted")
		}
	}
	// A memory-host JSON document cannot be silently interpreted as a v1 no-host workload.
	m, w = memoryFixture(t)
	b, _ := json.Marshal(w)
	var old hir.ProfileWorkload
	json.Unmarshal(b, &old)
	if _, e := hir.RunLoopProfile(m, old); e == nil {
		t.Fatal("old API accepted foreign schema")
	}
}

func TestMemoryHostDeadBuiltinReachabilityAndMetadata(t *testing.T) {
	m, w := memoryFixture(t)
	dead, e := nanz.Parse("fun unused()->void { @print(\"unused\") }\n", "unused.nanz")
	if e != nil {
		t.Fatal(e)
	}
	// MIR print builtin is deliberately absent from the closed host registry.
	raw := hir.LowerModule(m)
	raw.Funcs = append(raw.Funcs, hir.LowerModule(dead).Funcs...)
	if e = hir.ValidateMemoryHostMIR(m, raw, w); e != nil {
		t.Fatal("dead builtin refused", e)
	}
	w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "unused"})
	if e = hir.ValidateMemoryHostMIR(m, raw, w); e == nil {
		t.Fatal("reachable builtin accepted")
	}
	m, w = memoryFixture(t)
	run, e := hir.RunMemoryHostBaseline(m, w)
	if e != nil {
		t.Fatal(e)
	}
	m.Globals[0].Init = []byte{17}
	current, e := hir.BuildProfileCompatibility(m, run.Context)
	if e != nil {
		t.Fatal(e)
	}
	if reflect.DeepEqual(current, run.Compatibility) {
		t.Fatal("mutated typed source retained compatibility")
	}
}

func TestMemoryHostUnknownNumericGlobalDoesNotEscapeRegions(t *testing.T) {
	m, w := memoryFixture(t)
	raw := hir.LowerModule(m)
	f := raw.FuncByName("step")
	// A numeric pointer to VM global address zero must not become a compiler-owned
	// global access merely because its runtime integer matches global storage.
	f.Blocks = []*mir2.Block{{Label: "entry", Insts: []*mir2.Inst{{Op: mir2.OpConst, Dst: 9000, Imm: 0, Ty: mir2.TyPtr}, {Op: mir2.OpLoad, Dst: 9001, Src: [2]mir2.Reg{9000}, Ty: mir2.TyU8}}, Term: &mir2.TermRet{Vals: []mir2.Reg{9001}}}}
	w.Calls = w.Calls[:1]
	out, err := hir.RunMemoryHostMIRBaseline(m, raw, w)
	if err == nil || out == nil || len(out.Invocations[0].Events) != 0 || out.Invocations[0].Globals["calls"][0] != 3 {
		t.Fatal(out, err)
	}
}

func TestMemoryHostContextBindsSourceAndRegions(t *testing.T) {
	for name, change := range map[string]func(*hir.Module, *hir.MemoryHostWorkload){
		"region_permission": func(m *hir.Module, w *hir.MemoryHostWorkload) {
			r := w.Regions["ram"]
			r.Writable = false
			w.Regions["ram"] = r
		},
		"region_address": func(m *hir.Module, w *hir.MemoryHostWorkload) {
			r := w.Regions["ram"]
			r.Address++
			w.Regions["ram"] = r
		},
		"reset": func(m *hir.Module, w *hir.MemoryHostWorkload) {
			w.Calls[0].Patches = []hir.MemoryHostPatch{{Region: "ram", Clear: true}}
		},
		"layout": func(m *hir.Module, w *hir.MemoryHostWorkload) { w.Layout.CodeOrigin++ },
	} {
		t.Run(name, func(t *testing.T) {
			m, w := memoryFixture(t)
			_, ctx, e := hir.PrepareMemoryHostWorkload(m, w)
			if e != nil {
				t.Fatal(e)
			}
			change(m, &w)
			_, changed, e := hir.PrepareMemoryHostWorkload(m, w)
			if e != nil || reflect.DeepEqual(ctx, changed) {
				t.Fatal("stale context", e)
			}
		})
	}
	m, w := memoryFixture(t)
	w.Regions["ram"] = hir.MemoryHostRegion{Address: 65535, Bytes: []byte{0x12}, Writable: true}
	w.Calls = []hir.MemoryHostInvocation{{Function: "fail", Arguments: []int64{65535}}}
	run, e := hir.RunMemoryHostBaseline(m, w)
	if e == nil || len(run.Invocations[0].Events) != 1 || run.Invocations[0].Events[0].Address != 65535 || run.Invocations[0].Regions["ram"][0] != 7 {
		t.Fatal("wrapped0 must be undeclared after committed65535write", run, e)
	}
}

func TestMemoryHostCompiledMetadataRefusals(t *testing.T) {
	for name, mutate := range map[string]func(*mir2.Module){
		"target":             func(m *mir2.Module) { m.Target++ },
		"duplicate_function": func(m *mir2.Module) { m.Funcs = append(m.Funcs, m.Funcs[0]) },
		"changed_global":     func(m *mir2.Module) { m.Globals[0].Init = []byte{12} },
		"host_body": func(m *mir2.Module) {
			m.FuncByName("peek").Blocks = []*mir2.Block{{Label: "entry", Term: &mir2.TermRet{}}}
		},
		"host_return":    func(m *mir2.Module) { m.FuncByName("peek").Contract.Returns[0].Ty = mir2.TyU16 },
		"host_parameter": func(m *mir2.Module) { m.FuncByName("peek").Contract.Params[0].Ty = mir2.TyU8 },
		"dynamic_call": func(m *mir2.Module) {
			f := m.FuncByName("step")
			f.Blocks[0].Insts = append(f.Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCallIndirect, Ty: mir2.TyVoid})
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, w := memoryFixture(t)
			raw := hir.LowerModule(m)
			mutate(raw)
			if err := hir.ValidateMemoryHostMIR(m, raw, w); err == nil {
				t.Fatal("forged metadata accepted")
			}
		})
	}
	m, w := memoryFixture(t)
	m.Globals[0].Ty = &mir2.ArrayTy{Elem: mir2.TyPtr, Len: 1}
	m.Globals[0].Init = []byte{0, 0}
	w.Globals = map[string][]byte{"calls": {0, 0}}
	if _, _, err := hir.PrepareMemoryHostWorkload(m, w); err == nil {
		t.Fatal("nested pointer storage accepted")
	}
}

func TestMemoryHostGlobalProofRejectsModularOffsetLaundering(t *testing.T) {
	m, w := memoryFixture(t)
	raw := hir.LowerModule(m)
	f := raw.FuncByName("step")
	f.Blocks = []*mir2.Block{{Label: "entry", Insts: []*mir2.Inst{{Op: mir2.OpAddrOf, Dst: 9000, Sym: "calls", Ty: mir2.TyPtr}, {Op: mir2.OpPtrBump, Dst: 9001, Src: [2]mir2.Reg{9000}, Imm: 65536, Ty: mir2.TyPtr}, {Op: mir2.OpLoad, Dst: 9002, Src: [2]mir2.Reg{9001}, Ty: mir2.TyU8}}, Term: &mir2.TermRet{Vals: []mir2.Reg{9002}}}}
	if _, ok := hir.MemoryHostGlobalAccesses(raw)[f.Blocks[0].Insts[2]]; ok {
		t.Fatal("modular nonzero offset treated as exact global")
	}
	w.Calls = w.Calls[:1]
	out, err := hir.RunMemoryHostMIRBaseline(m, raw, w)
	if err == nil || out == nil || len(out.Invocations[0].Events) != 0 {
		t.Fatal(out, err)
	}
}

func TestMemoryHostGlobalOriginsMeetAllCFGPredecessors(t *testing.T) {
	// One path defines a global, another reaches the same load via BrIf2 carrying
	// an unknown numeric pointer. Ignoring that edge would launder global access.
	f := &mir2.Func{Name: "join", Blocks: []*mir2.Block{
		{Label: "entry", Term: &mir2.TermBrIf{Then: "known", Else: "unknown"}},
		{Label: "known", Insts: []*mir2.Inst{{Op: mir2.OpAddrOf, Dst: 1, Sym: "g", Ty: mir2.TyPtr}}, Term: &mir2.TermJmp{Target: "join"}},
		{Label: "unknown", Insts: []*mir2.Inst{{Op: mir2.OpConst, Dst: 1, Imm: 0, Ty: mir2.TyPtr}}, Term: &mir2.TermBrIf2{Eq: "join", Lt: "exit", Gt: "exit"}},
		{Label: "join", Insts: []*mir2.Inst{{Op: mir2.OpLoad, Dst: 2, Src: [2]mir2.Reg{1}, Ty: mir2.TyU8}}, Term: &mir2.TermRet{Vals: []mir2.Reg{2}}},
		{Label: "exit", Term: &mir2.TermRet{}}}}
	m := &mir2.Module{Funcs: []*mir2.Func{f}, Globals: []mir2.Global{{Name: "g", Ty: mir2.TyU8}}}
	if _, ok := hir.MemoryHostGlobalOrigins(m)[f.Blocks[3].Insts[0]]; ok {
		t.Fatal("unknown BrIf2 predecessor omitted")
	}
	// A later coalesced register definition changes the symbolic value locally.
	f.Blocks[3].Insts = append([]*mir2.Inst{{Op: mir2.OpAddrOf, Dst: 1, Sym: "g", Ty: mir2.TyPtr}}, f.Blocks[3].Insts...)
	if _, ok := hir.MemoryHostGlobalOrigins(m)[f.Blocks[3].Insts[1]]; !ok {
		t.Fatal("actual local reachingglobaldefinition not recognized")
	}
}
