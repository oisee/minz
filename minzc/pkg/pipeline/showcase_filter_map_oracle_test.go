package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Book v7 5.1: map transforms each element, filter keeps true elements.
// Section 5.2 loads successive addresses, doubles u8, then CALLs process.
// Only the declared process runtime is supplied; both source entries are intact.
func TestOriginalFilterMapOrderedTraceVMZ80(t *testing.T) {
	const path = "../../../examples/nanz/03_filter_map_chain.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const adapter = `global oracle_trace_count: u16 = 0
 @extern fun poke(addr: u16, value: u8) -> void
 fun process(x: u8) -> void {
  poke(0xA000 + oracle_trace_count, x)
  oracle_trace_count = oracle_trace_count + 1
 }
 `
	factory := func() *hir.Module {
		h, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		a, e := nanz.Parse(adapter, "explicit-process-runtime.nanz")
		if e != nil {
			t.Fatal(e)
		}
		var fs []*hir.Func
		for _, f := range h.Funcs {
			if f.Name != "process" {
				fs = append(fs, f)
			}
		}
		h.Funcs = append(fs, a.Funcs...)
		h.Globals = append(h.Globals, a.Globals...)
		return h
	}
	input := make([]byte, 260)
	for i := range input {
		input[i] = byte(i*37 + 19)
	}
	input[0], input[1], input[257], input[258], input[259] = 0x71, 0x83, 0x95, 0xa7, 0xb9
	initial := bytes.Repeat([]byte{0xa5}, 258)
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"input": {Address: 0x8ffe, Bytes: input}, "trace": {Address: 0xa000, Bytes: initial, Writable: true}}}
	for _, f := range []string{"filter_map_forEach", "filter_map_explicit"} {
		for _, n := range []int64{0, 1, 2, 127, 128, 255} {
			for _, th := range []int64{0, 1, 126, 127, 128, 254, 255} {
				w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: f, Arguments: []int64{0x9000, n, th}, Globals: map[string][]byte{"oracle_trace_count": {0, 0}}, Patches: []hir.MemoryHostPatch{{Region: "trace", Bytes: initial}}})
			}
		}
	}
	dir := os.Getenv("MINZ_FILTER_MAP_ORACLE_ARTIFACTS")
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
		for i, o := range obs {
			c := w.Calls[i]
			n, th := c.Arguments[1], byte(c.Arguments[2])
			trace := append([]byte(nil), initial...)
			var count uint16
			events := []hir.MemoryHostAccess{}
			for j := int64(0); j < n; j++ {
				x := input[j+2]
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: uint16(0x9000 + j), Value: x})
				v := byte(uint16(x) * 2)
				if v > th {
					trace[count] = v
					events = append(events, hir.MemoryHostAccess{Kind: "write", Address: 0xa000 + count, Value: v})
					count++
				}
			}
			g := map[string][]byte{"oracle_trace_count": {byte(count), byte(count >> 8)}}
			if !bytes.Equal(o.Regions["input"], input) || !bytes.Equal(o.Regions["trace"], trace) || !reflect.DeepEqual(o.Globals, g) || !reflect.DeepEqual(o.Events, events) || len(o.Results) != 0 || o.Failure != nil || o.Error != "" {
				save(label+"-failure.json", map[string]any{"call": i, "entry": c.Function, "n": n, "threshold": th, "got": o, "trace": trace, "events": events, "globals": g})
				t.Fatalf("%s call%d %s n%d threshold%d input=%v trace=%v globals=%v events=%v", label, i, c.Function, n, th, bytes.Equal(o.Regions["input"], input), bytes.Equal(o.Regions["trace"], trace), reflect.DeepEqual(o.Globals, g), reflect.DeepEqual(o.Events, events))
			}
			if label == "raw" {
				expected = append(expected, hir.MemoryHostObservation{Globals: g, Regions: map[string][]byte{"input": append([]byte(nil), input...), "trace": trace}, Events: events})
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
	save("unadapted.a80", steps.Assembly)
	native, e := runZ80MemoryHostWorkload(steps, original, w, *raw)
	if e != nil {
		save("native-error.txt", e.Error())
		t.Fatal(e)
	}
	save("native.json", native)
	check("native", native.Invocations)
	save("target.a80", native.Assembly)
	save("target.bin", native.Binary)
	t.Logf("exact original map/filter entries: %d calls ordered reads/callback writes/full input+trace/rawVM+Z80; %d T/%d B SHA %x", len(w.Calls), native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}
