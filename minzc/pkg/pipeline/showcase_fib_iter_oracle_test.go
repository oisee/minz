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
	"testing"
)

// Book v7 §2.5 and EBNF ForRangeStmt; HIR defines integer Start..End
// as ascending start-inclusive/end-exclusive, separately from range iterator
// countdown in chapter6. Host recurrence never executes compiler IR.
func TestOriginalFibIterAllBytesVMZ80(t *testing.T) {
	const path = "../../../reports/showcase-src/2026-03-11/ex10b_fib_iter.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const adapter = `@extern fun peek(addr:u16)->u8
fun oracle_observe(addr:u16)->u8 {return peek(addr)}
`
	compiledSource := string(source) + "\n" + adapter
	factory := func() *hir.Module {
		h, e := nanz.Parse(compiledSource, path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	input := []byte{0x71, 0xa5, 0xb9}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"input": {Address: 0x9000, Bytes: input}}}
	for n := int64(0); n < 256; n++ {
		w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "fib_iter", Arguments: []int64{n}})
	}
	w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_observe", Arguments: []int64{0x9000}})
	dir := os.Getenv("MINZ_FIB_ITER_ORACLE_ARTIFACTS")
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
	save("source-provenance.json", map[string]any{"original_source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "compiled_source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(compiledSource))), "removed_declarations": 0, "algorithm_source_edits": 0, "adapter": "observer only"})
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
			want := uint16(0)
			if c.Function == "oracle_observe" {
				want = uint16(input[0])
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: 0x9000, Value: input[0]})
			} else {
				a, b := uint16(0), uint16(1)
				for j := int64(0); j < c.Arguments[0]; j++ {
					a, b = b, a+b
				}
				want = a
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
	for _, mode := range []struct {
		name    string
		options Options
	}{{"ordinary", Options{}}, {"grace-speed", Options{UseGrace: true, UseHIRGrace: true}}, {"grace-size", Options{UseGrace: true, UseHIRGrace: true, OptSize: true}}} {
		steps, e := CompileHIRSteps(factory(), mode.options)
		if e != nil {
			t.Fatal(e)
		}
		save(mode.name+"-final.mir2", steps.MIR2Module.Dump())
		save(mode.name+"-pre-host-link.a80", steps.Assembly)
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
			r, e := vm.Call(c.Function, []mir2.Value{{I: c.Arguments[0]}})
			o := expected[i]
			if e != nil || len(r) != 1 || r[0].I != o.Results[0].I || !reflect.DeepEqual(events, o.Events) || !bytes.Equal(vm.ReadHeap(0x9000, 3), o.Regions["input"]) {
				t.Fatalf("final VM call %d result %v err %v", i, r, e)
			}
		}
		native, e := runZ80MemoryHostWorkload(steps, original, w, *raw)
		if e != nil {
			save(mode.name+"-native-error.txt", e.Error())
			t.Fatal(e)
		}
		save(mode.name+"-native.json", native)
		check("native", native.Invocations)
		save(mode.name+"-target.a80", native.Assembly)
		save(mode.name+"-target.bin", native.Binary)
		t.Logf("exact original fib_iter:all256 n + observer/u16 recurrence/RAMguards/raw-finalVM/nativeSPcode mode%s; %d T/%d B SHA %x", mode.name, native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
	}
}
