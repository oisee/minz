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

// Book v7 5.1/5.3 and chapter6: array forEach reads successive bytes;
// mutable captures thread sum/max through the loop. Host uint8 sum/max and
// expected ordered reads never execute HIR to calculate the oracle.
// Explicit pointer drivers call both unchanged source entries.
func TestOriginalForeachReadOrderVMZ80(t *testing.T) {
	const path = "../../../reports/showcase-src/2026-03-10/ex6_foreach.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const adapter = `@extern fun peek(addr:u16)->u8
fun oracle_sum(addr:u16,n:u8)->u8 {var p:^u8=addr
 return sum_chain(p,n)}
fun oracle_max(addr:u16,n:u8)->u8 {var p:^u8=addr
 return max_chain(p,n)}
fun oracle_observe(addr:u16)->u8 {return peek(addr)}
`
	factory := func() *hir.Module {
		h, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		a, e := nanz.Parse(string(source)+"\n"+adapter, "explicit-pointer-driver.nanz")
		if e != nil {
			t.Fatal(e)
		}
		h.Funcs = a.Funcs
		return h
	}
	input := make([]byte, 514)
	for i := range input {
		input[i] = byte(i*37 + 19)
	}
	input[0], input[513] = 0x71, 0xb9
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"input": {Address: 0x9000, Bytes: input}}}
	for _, f := range []string{"oracle_sum", "oracle_max"} {
		for _, offset := range []int64{1, 2, 127, 128, 255} {
			for _, n := range []int64{0, 1, 2, 127, 128, 255} {
				w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: f, Arguments: []int64{0x9000 + offset, n}})
			}
		}
	}
	w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_observe", Arguments: []int64{0x9000}})
	dir := os.Getenv("MINZ_FOREACH_ORACLE_ARTIFACTS")
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
			events := []hir.MemoryHostAccess{}
			want := byte(0)
			if c.Function == "oracle_observe" {
				want = input[0]
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: 0x9000, Value: want})
			} else {
				for j := int64(0); j < c.Arguments[1]; j++ {
					addr := c.Arguments[0] + j
					x := input[addr-0x9000]
					events = append(events, hir.MemoryHostAccess{Kind: "read", Address: uint16(addr), Value: x})
					if c.Function == "oracle_sum" {
						want += x
					} else if x > want {
						want = x
					}
				}
			}
			if !bytes.Equal(o.Regions["input"], input) || !reflect.DeepEqual(o.Events, events) || len(o.Globals) != 0 || len(o.Results) != 1 || o.Results[0].I != int64(want) || o.Failure != nil || o.Error != "" {
				save(label+"-failure.json", map[string]any{"call": i, "got": o, "events": events, "expected": want})
				t.Fatalf("%s call%d%s result%v want%d events%v", label, i, c.Function, o.Results, want, reflect.DeepEqual(o.Events, events))
			}
			if label == "raw" {
				expected = append(expected, hir.MemoryHostObservation{Results: o.Results, Regions: map[string][]byte{"input": append([]byte(nil), input...)}, Events: events})
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
	t.Logf("exact original sum/max forEach:60 calls + observer, full514RAM/orderedreads/rawVM+nativeSPcode; %d T/%d B SHA %x", native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}
