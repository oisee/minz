package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Book v7 §2.6 pointer bytes; four independent ordered reads, even when
// addresses alias. Host sum modulo256 never evaluates compiler IR.
func TestOriginalFourPointerReadAliasVMZ80(t *testing.T) {
	const path = "../../../examples/nanz/05_four_pointers.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const adapter = `@extern fun peek(addr:u16)->u8
fun oracle_four(a:u16,b:u16,c:u16,d:u16)->u8 {var p:ptr=a
 var q:ptr=b
 var r:ptr=c
 var s:ptr=d
 return four_ptrs(p,q,r,s)}
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
	layouts := [][4]int64{{0x9001, 0x907f, 0x9080, 0x9100}, {0x9001, 0x9001, 0x9001, 0x9001}, {0x9001, 0x9001, 0x9100, 0x9100}, {0x9001, 0x907f, 0x9001, 0x9100}, {0x9001, 0x9001, 0x9001, 0x9100}, {0x9001, 0x907f, 0x9080, 0xffff}, {0xffff, 0xffff, 0x9001, 0x9001}}
	for _, p := range layouts {
		for v := 0; v < 256; v++ {
			patch := append([]byte(nil), input...)
			patch[1] = byte(v)
			patch[127] = byte(v*3 + 7)
			patch[128] = byte(v*5 + 11)
			patch[256] = byte(v*7 + 13)
			w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_four", Arguments: p[:], Patches: []hir.MemoryHostPatch{{Region: "input", Bytes: patch}, {Region: "last", Bytes: []byte{byte(v*11 + 17)}}}})
		}
	}
	w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_observe", Arguments: []int64{0xffff}})
	dir := os.Getenv("MINZ_FOUR_POINTER_ORACLE_ARTIFACTS")
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
			for _, p := range c.Patches {
				r := w.Regions[p.Region]
				copy(ram[int(r.Address)+p.Offset:], p.Bytes)
			}
			if c.Function == "oracle_observe" {
				want = ram[0xffff]
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: 0xffff, Value: want})
			} else {
				for _, addr := range c.Arguments {
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
	// Explicit final VM proof in addition to raw VM and assembled native runner.
	vm := mir2.NewVM(steps.MIR2Module)
	vm.EnsureHeap(65536)
	vm.MaxSteps = 1000000
	for _, r := range w.Regions {
		vm.WriteHeapSlice(int64(r.Address), r.Bytes)
	}
	for i, c := range w.Calls[:len(w.Calls)-1] {
		for _, p := range c.Patches {
			r := w.Regions[p.Region]
			vm.WriteHeapSlice(int64(r.Address)+int64(p.Offset), p.Bytes)
		}
		events := []hir.MemoryHostAccess{}
		vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
			kind := "read"
			if a.Op == mir2.OpStore {
				kind = "write"
			}
			events = append(events, hir.MemoryHostAccess{Kind: kind, Address: uint16(a.Addr), Value: a.Value})
		}
		args := []mir2.Value{}
		for _, a := range c.Arguments {
			args = append(args, mir2.Value{I: a})
		}
		r, e := vm.Call(c.Function, args)
		o := expected[i]
		if e != nil || len(r) != 1 || r[0].I != o.Results[0].I || !reflect.DeepEqual(events, o.Events) || !reflect.DeepEqual(vm.ReadHeap(0x9000, 258), o.Regions["input"]) || vm.ReadHeap(0xffff, 1)[0] != o.Regions["last"][0] {
			t.Fatalf("finalVMcall%d result%v err%v", i, r, e)
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
	t.Logf("exact original four_ptrs:1792calls+observer/full259RAM/orderedreads/aliases/raw-finalVM/nativeSPcode; %d T/%d B SHA %x", native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}
