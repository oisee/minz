package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Book v7 §11.3: mapInPlace adds two to successive bytes. The model owns
// cumulative memory and computes modulo256 independently of the compiler.
func TestOriginalMapInPlaceCumulativeVMZ80(t *testing.T) {
	const path = "../../../reports/showcase-src/2026-03-10/ex7_mapinplace.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The explicit read-only observer makes the result observable to the harness
	// and satisfies its memory-host contract. add2_inplace itself is unchanged.
	const adapter = `@extern fun peek(addr: u16) -> u8
 fun oracle_observe(addr: u16) -> u8 { return peek(addr) }
fun oracle_call(addr: u16, n: u8) -> void {
 var p: ^u8 = addr
 add2_inplace(p, n)
}
`
	factory := func() *hir.Module {
		h, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		a, e := nanz.Parse(string(source)+"\n"+adapter, "explicit-address-driver.nanz")
		if e != nil {
			t.Fatal(e)
		}
		h.Funcs = a.Funcs
		return h
	}
	main := make([]byte, 514)
	for i := range main {
		main[i] = byte(i)
	}
	low := make([]byte, 258)
	high := make([]byte, 258)
	for i := range low {
		low[i] = byte(i*53 + 7)
		high[i] = byte(i*71 + 11)
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x300}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"array": {Address: 0x9000, Bytes: main, Writable: true}, "low": {Address: 0x8000, Bytes: low, Writable: true}, "high": {Address: 0xfefe, Bytes: high, Writable: true}}}
	pairs := [][2]int64{{0x9001, 0}, {0x9001, 1}, {0x9002, 2}, {0x9001, 127}, {0x9080, 128}, {0x9001, 255}, {0x9100, 255}, {0x9001, 255}, {0x9080, 255}, {0x9002, 1}, {0x9001, 0}, {0xfefe, 0}, {0xfefe, 1}, {0xfefe, 2}, {0xfefe, 127}, {0xff00, 128}, {0xff00, 255}, {0xff01, 255}, {0xfffe, 2}, {0x8000, 255}}
	for _, p := range pairs {
		w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_call", Arguments: p[:]})
	}
	// Independent observer call is separate from all twenty exact source calls.
	w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "oracle_observe", Arguments: []int64{0xffff}})
	dir := os.Getenv("MINZ_MAPINPLACE_ORACLE_ARTIFACTS")
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
	save("observer.nanz", adapter)
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
			events := []hir.MemoryHostAccess{}
			c := w.Calls[i]
			if c.Function == "oracle_call" {
				for j := int64(0); j < c.Arguments[1]; j++ {
					addr := uint16(c.Arguments[0] + j)
					v := ram[addr]
					events = append(events, hir.MemoryHostAccess{Kind: "read", Address: addr, Value: v})
					ram[addr] = v + 2
					events = append(events, hir.MemoryHostAccess{Kind: "write", Address: addr, Value: ram[addr]})
				}
			} else {
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: 0xffff, Value: ram[0xffff]})
				if len(o.Results) != 1 || o.Results[0].I != int64(ram[0xffff]) {
					t.Fatalf("%s observer result %v", label, o.Results)
				}
			}
			regions := map[string][]byte{}
			for n, r := range w.Regions {
				regions[n] = append([]byte(nil), ram[int(r.Address):int(r.Address)+len(r.Bytes)]...)
			}
			if !reflect.DeepEqual(o.Regions, regions) || !reflect.DeepEqual(o.Events, events) || len(o.Globals) != 0 || o.Failure != nil || o.Error != "" || (c.Function == "oracle_call" && len(o.Results) != 0) {
				save(label+"-failure.json", map[string]any{"call": i, "got": o, "regions": regions, "events": events})
				t.Fatalf("%s call%d addr%04x regions=%v events=%v", label, i, c.Arguments[0], reflect.DeepEqual(o.Regions, regions), reflect.DeepEqual(o.Events, events))
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
	if !bytes.Equal(native.Invocations[len(pairs)-1].Regions["array"], raw.Invocations[len(pairs)-1].Regions["array"]) {
		t.Fatal("native final array")
	}
	t.Logf("exact original mapInPlace:20 persistent calls + separate observer, full1030RAM/order/stack/code; wrap separately; %d T/%d B SHA%x", native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}

// Separate direct-entry execution keeps address zero available as target RAM.
// The bounded memory-host runner reserves VM heap-zero function tags; we do
// not loosen that production admission rule to manufacture wrap coverage.
func TestOriginalMapInPlaceAddressWrapVMZ80(t *testing.T) {
	const path = "../../../reports/showcase-src/2026-03-10/ex7_mapinplace.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	pairs := [][2]int{{65535, 0}, {65535, 1}, {65535, 2}, {65535, 127}, {65408, 128}, {65408, 255}, {65535, 255}, {65534, 2}, {0, 255}}
	text := string(source)
	for i, p := range pairs {
		text += fmt.Sprintf("\nfun oracle_wrap_%d() -> void { var p: ^u8 = %d\n add2_inplace(p, %d) }\n", i, p[0], p[1])
	}
	factory := func() *hir.Module {
		h, e := nanz.Parse(text, path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	raw := mir2.NewVM(hir.LowerModule(factory()))
	raw.EnsureHeap(65536)
	steps, e := CompileHIRSteps(factory(), Options{AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	asm := "ORG 0x400\n" + steps.Assembly
	assembled, e := z80asm.NewAssembler().AssembleString(asm)
	if e != nil || len(assembled.Errors) != 0 {
		t.Fatalf("assembly %v %v", e, assembled.Errors)
	}
	if d := os.Getenv("MINZ_MAPINPLACE_ORACLE_ARTIFACTS"); d != "" {
		os.WriteFile(filepath.Join(d, "wrap-before.a80"), []byte(asm), 0644)
	}
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0) // Explicit flat all-RAM target; default emulator is ZX ROM protected.
	native := platform.RemogattoZ80
	native.SetSMCTracker(nil) // Data bytes resembling RST opcodes must not trigger screen hooks.
	if e = native.LoadMemory(0x400, assembled.Binary); e != nil {
		t.Fatal(e)
	}
	expected := map[uint16]byte{}
	for j := 0; j < 258; j++ {
		expected[uint16(j)] = byte(j*53 + 7)
		expected[uint16(0xfefe+j)] = byte(j*71 + 11)
	}
	for a, v := range expected {
		raw.WriteHeap(int64(a), v)
		native.Memory()[a] = v
	}
	for i, p := range pairs {
		events := []mir2.VMDataAccess{}
		want := []mir2.VMDataAccess{}
		for j := 0; j < p[1]; j++ {
			a := uint16(p[0] + j)
			v := expected[a]
			want = append(want, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(a), Value: v})
			expected[a] = v + 2
			want = append(want, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(a), Value: v + 2})
		}
		raw.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
		entry := fmt.Sprintf("oracle_wrap_%d", i)
		if r, e := raw.Call(entry, nil); e != nil || len(r) != 0 {
			t.Fatalf("raw %s %v %v", entry, r, e)
		}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("raw %s ordered events", entry)
		}
		mem := native.Memory()
		mem[0xeff0], mem[0xeff1] = 0, 3
		native.SetRegisters(emulator.Registers{PC: uint16(assembled.Symbols[entry]), SP: 0xeff0})
		done := false
		for j := 0; j < 1000000; j++ {
			if native.GetPC() == 0x300 && native.GetSP() == 0xeff2 {
				done = true
				break
			}
			if native.GetSP() < 0xe000 || native.GetSP() > 0xeff2 {
				t.Fatal("native stack bounds")
			}
			native.Step()
		}
		if !done {
			t.Fatalf("native %s no return PC%04x SP%04x", entry, native.GetPC(), native.GetSP())
		}
		for a, v := range expected {
			if raw.ReadHeap(int64(a), 1)[0] != v || mem[a] != v {
				t.Fatalf("%s addr%04x VM%02x native%02x want%02x", entry, a, raw.ReadHeap(int64(a), 1)[0], mem[a], v)
			}
		}
		if !bytes.Equal(mem[0x400:0x400+len(assembled.Binary)], assembled.Binary) {
			t.Fatal("immutable native image")
		}
	}
	if dir := os.Getenv("MINZ_MAPINPLACE_ORACLE_ARTIFACTS"); dir != "" {
		for name, b := range map[string][]byte{"wrap-driver.nanz": []byte(text), "wrap.a80": []byte(asm), "wrap.bin": assembled.Binary} {
			if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
		snapshot := map[string][]byte{"low": append([]byte(nil), native.Memory()[:258]...), "high": append([]byte(nil), native.Memory()[0xfefe:]...)}
		b, _ := json.MarshalIndent(snapshot, "", "  ")
		if e := os.WriteFile(filepath.Join(dir, "wrap-final-memory.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("9 cumulative exact original add2 calls via constant-address drivers, ffff→0000; raw ordered events + native full516 RAM/guards/SP/immutable image PASS; %d B SHA%x", len(assembled.Binary), sha256.Sum256(assembled.Binary))
}
