package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func memoryHostParserFixture(t *testing.T) (*hir.Module, hir.MemoryHostWorkload) {
	t.Helper()
	// This fixture replays the frozen 0e92689 source experiment. Current exact
	// example/main behavior has separate baseline and whole-main tests.
	source, e := os.ReadFile("../../../docs/grace-parser-prospective-heldout/artifacts/source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(source)) != "0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526" {
		t.Fatal("historical source identity")
	}
	m, e := nanz.Parse(string(source), "../../../examples/nanz/self_lanz_parser.nanz")
	if e != nil {
		t.Fatal(e)
	}
	word := func(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"nodes": {Address: 0x8000, Bytes: make([]byte, 4096), Writable: true}, "strings": {Address: 0x9000, Bytes: make([]byte, 8192), Writable: true}, "source": {Address: 0xc000, Bytes: make([]byte, 4096)}}, Globals: map[string][]byte{"str_bump": word(0x9000)}, Calls: []hir.MemoryHostInvocation{}}
	for _, input := range lanzBaselineInputs() {
		w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "parse_sexpr", Arguments: []int64{}, Globals: map[string][]byte{"src_len": word(uint16(len(input))), "pos": word(0), "node_bump": word(0x8000)}, Patches: []hir.MemoryHostPatch{{Region: "source", Clear: true, Bytes: []byte(input)}}})
	}
	return m, w
}
func TestMemoryHostOriginalSelfLanzTarget(t *testing.T) {
	m, w := memoryHostParserFixture(t)
	raw, e := hir.RunMemoryHostBaseline(m, w)
	if e != nil {
		t.Fatal(e)
	}
	candidate, _ := memoryHostParserFixture(t)
	steps, e := CompileHIRSteps(candidate, Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	target, e := runZ80MemoryHostWorkload(steps, m, w, *raw)
	if e != nil {
		t.Fatal(e)
	}
	exportMemoryHostReferenceBeforeAssert(t, "target", target, map[string]any{"workload.json": w, "raw.json": raw})
	assertMemoryHostParserIndependentOracle(t, raw.Invocations, target.Invocations)

	if fmt.Sprintf("%x", sha256.Sum256([]byte(target.FinalMIRDump))) != target.MIRSHA256 || fmt.Sprintf("%x", sha256.Sum256(target.AllocationJSON)) != target.AllocationSHA256 {
		t.Fatal("diagnostic payload hash mismatch")
	}
	assertCurrentMemoryHostParserReference(t, target)
	if dir := os.Getenv("MINZ_MEMORY_HOST_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0755)
		for n, v := range map[string]any{"workload.json": w, "raw.json": raw, "target.json": target} {
			b, e := json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(dir, n), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
		os.WriteFile(filepath.Join(dir, "target.bin"), target.Binary, 0644)
		os.WriteFile(filepath.Join(dir, "target.a80"), []byte(target.Assembly), 0644)
		os.WriteFile(filepath.Join(dir, "final.mir2"), []byte(target.FinalMIRDump), 0644)
		os.WriteFile(filepath.Join(dir, "allocation.json"), target.AllocationJSON, 0644)
	}
	t.Logf("production memory host original30 %dT/%dB SHA%s", target.TotalTStates, len(target.Binary), target.BinarySHA256)
}
func TestMemoryHostTargetLayoutRefusals(t *testing.T) {
	for _, name := range []string{"code_region", "stack_region", "read_only", "oracle", "hostABI"} {
		t.Run(name, func(t *testing.T) {
			m, w := memoryHostParserFixture(t)
			raw, e := hir.RunMemoryHostBaseline(m, w)
			if e != nil {
				t.Fatal(e)
			}
			candidate, _ := memoryHostParserFixture(t)
			steps, e := CompileHIRSteps(candidate, Options{AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			switch name {
			case "code_region":
				w.Layout.CodeOrigin = 0x8000
			case "stack_region":
				w.Layout.StackLow = 0x9000
			case "read_only":
				r := w.Regions["nodes"]
				r.Writable = false
				w.Regions["nodes"] = r
			case "oracle":
				raw.Invocations[1].Results[0].I++
			case "hostABI":
				reg := steps.MIR2Module.FuncByName("peek").Contract.Params[0].Reg
				steps.Allocation.Locs[reg] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "DE"}
			}
			if r, e := runZ80MemoryHostWorkload(steps, m, w, *raw); e == nil || r != nil {
				t.Fatal("unsafe target accepted", name, fmt.Sprint(e))
			}
		})
	}
}

func TestMemoryHostTargetPartialFailureAndPersistence(t *testing.T) {
	const src = `@extern fun peek(addr:u16)->u8
@extern fun poke(addr:u16,val:u8)->void
global state:u16=0
fun sequence(addr:u16,value:u8)->u8 {
 poke(addr,value)
 return peek(addr+1)
}
fun load(addr:u16)->u8{return peek(addr)}
`
	parse := func() *hir.Module {
		m, e := nanz.Parse(src, "memory-target.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return m
	}
	for _, bad := range []bool{false, true} {
		m := parse()
		w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"data": {Address: 0x8000, Bytes: []byte{4, 7}, Writable: true}}, Calls: []hir.MemoryHostInvocation{{Function: "sequence", Arguments: []int64{0x8000, 255}}, {Function: "load", Arguments: []int64{0x8000}}}}
		if bad {
			w.Calls[0].Arguments[0] = 0x8001
		}
		raw, err := hir.RunMemoryHostBaseline(m, w)
		if (err != nil) != bad {
			t.Fatal("raw failure shape", err)
		}
		steps, err := CompileHIRSteps(parse(), Options{AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		target, err := runZ80MemoryHostWorkload(steps, m, w, *raw)
		if err != nil {
			t.Fatal(err)
		}
		if bad {
			if len(target.Invocations) != 1 || target.Invocations[0].Error == "" || len(target.Invocations[0].Events) != 1 || target.Invocations[0].Regions["data"][1] != 255 {
				t.Fatal("target partial prefix")
			}
			failure := target.Invocations[0].Failure
			if failure == nil || failure.Kind != "undeclared" || failure.Address != 0x8002 || failure.Write {
				t.Fatalf("target denial cause: %+v", failure)
			}
			for _, field := range []string{"kind", "address", "direction"} {
				t.Run("forged_denial_"+field, func(t *testing.T) {
					payload, e := json.Marshal(raw)
					if e != nil {
						t.Fatal(e)
					}
					var forged hir.MemoryHostRun
					if e = json.Unmarshal(payload, &forged); e != nil {
						t.Fatal(e)
					}
					switch field {
					case "kind":
						forged.Invocations[0].Failure.Kind = "read-only"
					case "address":
						forged.Invocations[0].Failure.Address = 0x8003
					case "direction":
						forged.Invocations[0].Failure.Write = true
					}
					if evidence, e := runZ80MemoryHostWorkload(steps, m, w, forged); e == nil || evidence != nil {
						t.Fatalf("different denial cause accepted: %s, %v", field, e)
					}
				})
			}
			public, e := RunZ80MemoryHostBaseline(func() (*hir.Module, error) { return parse(), nil }, w)
			if e != nil {
				t.Fatal(e)
			}
			publicFailure := public.Target.Invocations[0].Failure
			if publicFailure == nil || *publicFailure != *failure {
				t.Fatalf("public denial cause: %+v", publicFailure)
			}
			limited := w
			limited.MaxSteps = 1
			if evidence, e := RunZ80MemoryHostBaseline(func() (*hir.Module, error) { return parse(), nil }, limited); e == nil || evidence != nil {
				t.Fatalf("execution limit accepted as memory denial: %v", e)
			}
		} else {
			if target.Invocations[1].Results[0].I != 255 {
				t.Fatal("two-call RAM persistence")
			}
		}
	}
}

func assertMemoryHostParserIndependentOracle(t *testing.T, runs ...[]hir.MemoryHostObservation) {
	t.Helper()
	inputs := lanzBaselineInputs()
	for _, run := range runs {
		if len(run) != len(inputs) {
			t.Fatalf("independent parser invocation count %d, want %d", len(run), len(inputs))
		}
	}
	host := newLanzHost()
	for i, input := range inputs {
		clear(host.ram[0xc000:0xd000])
		copy(host.ram[0xc000:], []byte(input))
		host.pos = 0
		host.len = uint16(len(input))
		host.node = 0x8000
		host.events = []lanzAccess{}
		root := host.parse()
		events := []hir.MemoryHostAccess{}
		for _, v := range host.events {
			events = append(events, hir.MemoryHostAccess{Kind: v.Op, Address: v.Address, Value: v.Value})
		}
		for _, run := range runs {
			o := run[i]
			if len(o.Results) != 1 || o.Results[0].I != int64(root) || !reflect.DeepEqual(o.Events, events) || !reflect.DeepEqual(o.Regions["nodes"], host.ram[0x8000:0x9000]) || !reflect.DeepEqual(o.Regions["strings"], host.ram[0x9000:0xb000]) || !reflect.DeepEqual(o.Regions["source"], host.ram[0xc000:0xd000]) {
				t.Fatal("independent original parser", i)
			}
			for n, v := range map[string]uint16{"pos": host.pos, "src_len": host.len, "node_bump": host.node, "str_bump": host.str, "src_buf": 0xc000, "node_arena": 0x8000, "str_arena": 0x9000, "SEXP_ATOM": 1, "SEXP_LIST": 2, "SEXP_INT": 3} {
				b := o.Globals[n]
				got := uint16(b[0])
				if len(b) > 1 {
					got |= uint16(b[1]) << 8
				}
				if got != v {
					t.Fatal("independent parser global", i, n, got, v)
				}
			}
		}
	}
}
