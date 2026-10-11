package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Book v7 §2.5 array for-each syntax and §5.1/5.3: execute the callback
// for each of n successive array bytes, threading the mutable byte sum.
// This is array traversal, not chapter6 counter-based range(lo..hi).
func TestOriginalIdiomaticSumReadOrderVMZ80(t *testing.T) {
	const path = "../../../examples/nanz/02_sum_array_idiomatic.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const adapter = `@extern fun peek(addr:u16)->u8
fun oracle_observe(addr:u16)->u8 {return peek(addr)}
`
	const deadDeclaration = "@extern fun add_to_acc(x: u8) -> void"
	if strings.Count(string(source), deadDeclaration) != 1 {
		t.Fatal("unused extern declaration changed")
	}
	compiledSource := strings.Replace(string(source), deadDeclaration, "", 1) + "\n" + adapter
	factory := func() *hir.Module {
		h, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		// The historical example declares an unused external callback whose ABI
		// has no bounded-memory host equivalent. Audit the COMPLETE lowered source
		// before omitting only that dead linker declaration; no body is edited.
		full := hir.LowerModule(h)
		for _, f := range full.Funcs {
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if i.Sym == "add_to_acc" || i.Op == mir2.OpCallIndirect {
						t.Fatal("original source now depends on unsupported add_to_acc runtime")
					}
				}
			}
		}
		h, e = nanz.Parse(compiledSource, path+"+explicit-linker-adapter")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	input := make([]byte, 514)
	for i := range input {
		input[i] = byte(i*37 + 19)
	}
	input[0], input[513] = 0x71, 0xb9
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"input": {Address: 0x9000, Bytes: input, Writable: true}}}
	for _, f := range []string{"sum_array_idiomatic", "sum_array_chain"} {
		for _, offset := range []int64{1, 255} {
			for n := int64(0); n < 256; n++ {
				patch := append([]byte(nil), input...)
				for i := 1; i < len(patch)-1; i++ {
					patch[i] = byte(i*37 + int(n)*17 + 19)
				}
				w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: f, Arguments: []int64{0x9000 + offset, n}, Patches: []hir.MemoryHostPatch{{Region: "input", Bytes: patch}}})
			}
		}
	}
	w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_observe", Arguments: []int64{0x9000}})
	dir := os.Getenv("MINZ_IDIOMATIC_SUM_ORACLE_ARTIFACTS")
	save := func(name string, v any) {
		if dir == "" {
			return
		}
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		default:
			b, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", source)
	save("runtime-adapter.nanz", adapter)
	save("compiled-source.nanz", compiledSource)
	originalFull, e := nanz.Parse(string(source), path)
	if e != nil {
		t.Fatal(e)
	}
	save("original-full.mir2", hir.LowerModule(originalFull).Dump())
	save("linker-boundary.json", map[string]any{"removed_declaration": deadDeclaration, "original_source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "compiled_source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(compiledSource))), "original_callback_references": 0, "original_indirect_calls": 0, "algorithm_source_edits": 0})
	save("workload.json", w)
	original := factory()
	raw, e := hir.RunMemoryHostBaseline(original, w)
	if e != nil {
		save("raw-error.txt", e.Error())
		t.Fatal(e)
	}
	save("raw.json", raw)
	expected := []hir.MemoryHostObservation{}
	check := func(label string, obs []hir.MemoryHostObservation) {
		if len(obs) != len(w.Calls) {
			t.Fatalf("%s missing calls", label)
		}
		ram := append([]byte(nil), input...)
		for i, o := range obs {
			c := w.Calls[i]
			for _, p := range c.Patches {
				copy(ram[p.Offset:], p.Bytes)
			}
			events := []hir.MemoryHostAccess{}
			want := byte(0)
			if c.Function == "oracle_observe" {
				want = ram[0]
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: 0x9000, Value: want})
			} else {
				for j := int64(0); j < c.Arguments[1]; j++ {
					addr := c.Arguments[0] + j
					x := ram[addr-0x9000]
					events = append(events, hir.MemoryHostAccess{Kind: "read", Address: uint16(addr), Value: x})
					want += x
				}
			}
			if !bytes.Equal(o.Regions["input"], ram) || !reflect.DeepEqual(o.Events, events) || len(o.Globals) != 0 || len(o.Results) != 1 || o.Results[0].I != int64(want) || o.Failure != nil || o.Error != "" {
				save(label+"-failure.json", map[string]any{"call": i, "got": o, "events": events, "expected": want})
				t.Fatalf("%s call%d%s result%v want%d events%v", label, i, c.Function, o.Results, want, reflect.DeepEqual(o.Events, events))
			}
			if label == "raw" {
				expected = append(expected, hir.MemoryHostObservation{Results: o.Results, Regions: map[string][]byte{"input": append([]byte(nil), ram...)}, Events: events})
			}
		}
	}
	check("raw", raw.Invocations)
	save("independent-model.json", expected)
	steps, e := CompileHIRSteps(factory(), Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	save("final.mir2", steps.MIR2Module.Dump())
	save("pre-host-link.a80", steps.Assembly)
	vm := mir2.NewVM(steps.MIR2Module)
	vm.EnsureHeap(65536)
	vm.MaxSteps = 1000000
	vm.WriteHeapSlice(0x9000, input)
	for i, c := range w.Calls[:len(w.Calls)-1] {
		for _, p := range c.Patches {
			vm.WriteHeapSlice(0x9000+int64(p.Offset), p.Bytes)
		}
		events := []hir.MemoryHostAccess{}
		vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
			kind := "read"
			if a.Op == mir2.OpStore {
				kind = "write"
			}
			events = append(events, hir.MemoryHostAccess{Kind: kind, Address: uint16(a.Addr), Value: a.Value})
		}
		r, e := vm.Call(c.Function, []mir2.Value{{I: c.Arguments[0]}, {I: c.Arguments[1]}})
		o := expected[i]
		if e != nil || len(r) != 1 || r[0].I != o.Results[0].I || !reflect.DeepEqual(events, o.Events) || !bytes.Equal(vm.ReadHeap(0x9000, 514), o.Regions["input"]) {
			t.Fatalf("final VM call %d result %v err %v", i, r, e)
		}
	}
	native, e := runZ80MemoryHostWorkload(steps, original, w, *raw)
	if e != nil {
		save("native-error.txt", e.Error())
		t.Fatal(e)
	}
	save("native.json", native)
	check("native", native.Invocations)
	save("target.a80", native.Assembly)
	save("target.bin", native.Binary)
	t.Logf("exact original idiomatic/chain sum:1024 calls + observer, full514RAM/all256lengths/orderedreads/raw-finalVM+nativeSPcode; %d T/%d B SHA %x", native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}
