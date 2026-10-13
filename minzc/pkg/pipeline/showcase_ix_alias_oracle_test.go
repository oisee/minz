package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Independent byte-address model applies the actual source store before all
// three loads, so aliasing is visible. Book v7 §2.6 pointer indexed bytes.
func TestOriginalIXPointerAliasVMZ80(t *testing.T) {
	const path = "../../../examples/nanz/07_ix_load_store.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const adapter = `@extern fun peek(addr:u16)->u8
fun oracle_sum(a:u16,b:u16,c:u16)->u8 {var p:ptr=a
 var q:ptr=b
 var r:ptr=c
 return sum_with_ix(p,q,r)}
fun oracle_round(a:u16,b:u16,c:u16,v:u8)->u8 {var p:ptr=a
 var q:ptr=b
 var r:ptr=c
 return roundtrip_ix(p,q,r,v)}
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
	input := make([]byte, 258)
	for i := range input {
		input[i] = byte(i*37 + 19)
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"input": {Address: 0x9000, Bytes: input, Writable: true}, "last": {Address: 0xffff, Bytes: []byte{0xa5}, Writable: true}}}
	layouts := [][3]int64{{0x9001, 0x907f, 0x9100}, {0x9001, 0x907f, 0x9001}, {0x9001, 0x907f, 0x907f}, {0x9001, 0x9001, 0x9100}, {0x9100, 0x9100, 0x9100}, {0x9001, 0x907f, 0xffff}}
	for _, p := range layouts {
		w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_sum", Arguments: p[:]})
		for v := int64(0); v < 256; v++ {
			w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_round", Arguments: []int64{p[0], p[1], p[2], v}})
		}
	}
	w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_observe", Arguments: []int64{0xffff}})
	dir := os.Getenv("MINZ_IX_ORACLE_ARTIFACTS")
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
		ram := make([]byte, 65536)
		for _, r := range w.Regions {
			copy(ram[int(r.Address):], r.Bytes)
		}
		if len(obs) != len(w.Calls) {
			t.Fatalf("%s missing calls", label)
		}
		for i, o := range obs {
			c := w.Calls[i]
			events := []hir.MemoryHostAccess{}
			want := byte(0)
			if c.Function == "oracle_observe" {
				want = ram[0xffff]
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: 0xffff, Value: want})
			} else {
				if c.Function == "oracle_round" {
					addr := uint16(c.Arguments[2])
					v := byte(c.Arguments[3])
					ram[addr] = v
					events = append(events, hir.MemoryHostAccess{Kind: "write", Address: addr, Value: v})
				}
				for _, addr := range c.Arguments[:3] {
					v := ram[addr]
					want += v
					events = append(events, hir.MemoryHostAccess{Kind: "read", Address: uint16(addr), Value: v})
				}
			}
			regions := map[string][]byte{"input": append([]byte(nil), ram[0x9000:0x9102]...), "last": {ram[0xffff]}}
			if !reflect.DeepEqual(o.Regions, regions) || !reflect.DeepEqual(o.Events, events) || len(o.Globals) != 0 || len(o.Results) != 1 || o.Results[0].I != int64(want) || o.Failure != nil || o.Error != "" {
				save(label+"-failure.json", map[string]any{"call": i, "got": o, "events": events, "expected": want, "regions": regions})
				t.Fatalf("%s call%d%s result%v want%d events%v", label, i, c.Function, o.Results, want, reflect.DeepEqual(o.Events, events))
			}
			if label == "raw" {
				expected = append(expected, hir.MemoryHostObservation{Results: o.Results, Regions: regions, Events: events})
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
	t.Logf("exact original IX source:1536 roundtrip +6sum+observer; full259RAM/orderedwrite-read aliases/rawVM/nativeSPcode; %d T/%d B SHA %x", native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}
