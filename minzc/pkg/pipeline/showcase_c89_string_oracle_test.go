package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ISO C90 §6.2.2.1 integer promotions and §6.3.6 pointer arithmetic;
// the target ABI uses 8-bit unsigned char and 16-bit signed int/pointers.
// Original C functions are never copied into the oracle or rewritten.
func TestOriginalC89StringMemoryVMZ80(t *testing.T) {
	const path = "../../../examples/c89/string8.c"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const adapter = `unsigned char peek(unsigned int addr);
int oracle_strlen(unsigned int a) {return my_strlen((unsigned char*)a);}
void oracle_memset(unsigned int a,unsigned char v,int n) {my_memset((unsigned char*)a,v,n);}
void oracle_memcpy(unsigned int a,unsigned int b,int n) {my_memcpy((unsigned char*)a,(unsigned char*)b,n);}
int oracle_memcmp(unsigned int a,unsigned int b,int n) {return my_memcmp((unsigned char*)a,(unsigned char*)b,n);}
unsigned char oracle_observe(unsigned int a) {return peek(a);}
`
	compiledSource := string(source) + "\n" + adapter
	factory := func() *hir.Module {
		h, e := c89.CompileWithOpts(compiledSource, path, c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		// C89 emits only definitions into HIR, so the workload's explicit host
		// declaration is installed with its declared C signature, not an algorithm.
		h.SourceUnits = map[string]string{path: fmt.Sprintf("%x", sha256.Sum256([]byte(compiledSource)))}
		h.SourceUnitsComplete = true // Test-owned include-free input; not automatic C89 PGO evidence.
		h.Funcs = append(h.Funcs, &hir.Func{Name: "peek", IsExtern: true, Params: []hir.Param{{Name: "addr", Ty: mir2.TyU16}}, RetTy: mir2.TyU8})
		return h
	}
	input := make([]byte, 1030)
	for i := range input {
		input[i] = byte(i*37 + 19)
	}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8}, Regions: map[string]hir.MemoryHostRegion{"input": {Address: 0x9000, Bytes: input, Writable: true}}}
	appendCall := func(f string, args []int64, patch []byte) {
		c := hir.MemoryHostInvocation{Function: f, Arguments: args}
		if patch != nil {
			c.Patches = []hir.MemoryHostPatch{{Region: "input", Bytes: patch}}
		}
		w.Calls = append(w.Calls, c)
	}
	for _, n := range []int64{0, 1, 2, 127, 128, 255, 256} {
		p := append([]byte(nil), input...)
		for i := int64(0); i < n; i++ {
			p[1+i] = byte(1 + i%255)
		}
		p[1+n] = 0
		appendCall("oracle_strlen", []int64{0x9001}, p)
	}
	for v := int64(0); v < 256; v++ {
		appendCall("oracle_memset", []int64{0x9201, v, 256}, nil)
	}
	for _, n := range []int64{-1, 0, 1, 2, 127, 128, 255, 256} {
		appendCall("oracle_memset", []int64{0x9201, 73, n}, nil)
		appendCall("oracle_memcpy", []int64{0x9201, 0x9001, n}, nil)
		for _, variant := range []int{0, 1, 2} {
			p := append([]byte(nil), input...)
			copy(p[513:769], p[1:257])
			if variant == 1 {
				p[1] = 0x80
				p[513] = 0x7f
			}
			if variant == 2 {
				p[128] = 0x7f
				p[640] = 0x80
			}
			appendCall("oracle_memcmp", []int64{0x9001, 0x9201, n}, p)
		}
	}
	appendCall("oracle_observe", []int64{0x9000}, nil)
	dir := os.Getenv("MINZ_C89_STRING_ORACLE_ARTIFACTS")
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
	save("source.c", source)
	save("runtime-adapter.c", adapter)
	save("compiled-source.c", compiledSource)
	save("source-provenance.json", map[string]any{"original_source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "compiled_source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(compiledSource))), "algorithm_source_edits": 0, "adapter": "integer-address wrappers and observer"})
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
			want := int64(0)
			hasResult := true
			read := func(addr int64) byte {
				v := ram[addr-0x9000]
				events = append(events, hir.MemoryHostAccess{Kind: "read", Address: uint16(addr), Value: v})
				return v
			}
			write := func(addr int64, v byte) {
				ram[addr-0x9000] = v
				events = append(events, hir.MemoryHostAccess{Kind: "write", Address: uint16(addr), Value: v})
			}
			switch c.Function {
			case "oracle_observe":
				want = int64(read(c.Arguments[0]))
			case "oracle_strlen":
				for read(c.Arguments[0]+want) != 0 {
					want++
				}
			case "oracle_memset":
				hasResult = false
				for j := int64(0); j < c.Arguments[2]; j++ {
					write(c.Arguments[0]+j, byte(c.Arguments[1]))
				}
			case "oracle_memcpy":
				hasResult = false
				for j := int64(0); j < c.Arguments[2]; j++ {
					v := read(c.Arguments[1] + j)
					write(c.Arguments[0]+j, v)
				}
			case "oracle_memcmp":
				for j := int64(0); j < c.Arguments[2]; j++ {
					a, b := read(c.Arguments[0]+j), read(c.Arguments[1]+j)
					if a != b {
						a, b = read(c.Arguments[0]+j), read(c.Arguments[1]+j)
						want = 1
						if a < b {
							want = -1
						}
						break
					}
				}
			}
			results := []mir2.Value{}
			if hasResult {
				// VM values encode signed i16 as a 16-bit bit pattern, just as native HL.
				results = append(results, mir2.Value{I: int64(uint16(want))})
			}

			if !bytes.Equal(o.Regions["input"], ram) || !reflect.DeepEqual(o.Events, events) || len(o.Globals) != 0 || !reflect.DeepEqual(o.Results, results) || o.Failure != nil || o.Error != "" {
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
		r, e := vm.Call(c.Function, func() []mir2.Value {
			var a []mir2.Value
			for _, v := range c.Arguments {
				a = append(a, mir2.Value{I: v})
			}
			return a
		}())
		o := expected[i]
		if e != nil || !reflect.DeepEqual(r, o.Results) || !reflect.DeepEqual(events, o.Events) || !bytes.Equal(vm.ReadHeap(0x9000, 1030), o.Regions["input"]) {
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
	t.Logf("exact original C89 string four entries:303 calls + observer/full1030RAM/orderedreads-writes/raw-finalVM/nativeSPcode; %d T/%d B SHA %x", native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}
