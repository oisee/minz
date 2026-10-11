package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const regionCompareSource = `fun region_eq(src:^u8,arena:^u8,src_start:u16,arena_ptr:u16,len:u8)->u8 {
 var i:u8=0
 while i<len {if src[src_start+i]!=arena[arena_ptr+1+i] {return 0}
 i=i+1}
 return 1}`

func TestRegionComparePrototypeRaw(t *testing.T) {
	a, e := nanz.Parse(regionCompareSource, "region.nanz")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := nanz.Parse(regionCompareSource, "region.nanz")
	if e = hir.ApplyRegionComparePrototype(b, "region_eq"); e != nil {
		t.Fatal(e)
	}
	mods := []*mir2.Module{hir.LowerModule(a), hir.LowerModule(b)}
	count := 0
	for _, heap := range []int{16, 65536} {
		memory := make([]byte, heap)
		for j := range memory {
			memory[j] = byte(j*37 + 11)
		}
		for n := int64(0); n < 256; n++ {
			for _, args := range [][4]int64{{0, 0, 0, 0}, {1, 0, 0, 0}, {0, 1, 0, 0}, {3, 7, 2, 0}, {15, 0, 0, 0}, {0, 15, 0, 0}, {65535, 0, 0, 0}, {0, 65535, 0, 0}, {0, 0, 65535, 0}, {0, 0, 0, 65535}, {0, 0, 65535 - n, 65534 - n}, {65535 - n, 65534 - n, 0, 0}} {
				want := int64(1)
				wantErr := ""
				wantAccess := []mir2.VMDataAccess{}
				for i := int64(0); i < n; i++ {
					left := int(uint16(args[0] + int64(uint16(args[2]+i))))
					right := int(uint16(args[1] + int64(uint16(args[3]+1+i))))
					if left >= heap {
						wantErr = fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", left, heap)
						break
					}
					lv := memory[left]
					wantAccess = append(wantAccess, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(left), Value: lv})
					if right >= heap {
						wantErr = fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", right, heap)
						break
					}
					wantAccess = append(wantAccess, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(right), Value: memory[right]})
					if lv != memory[right] {
						want = 0
						break
					}
				}
				for variant, m := range mods {
					vm := mir2.NewVM(m)
					vm.EnsureHeap(int64(heap))
					vm.WriteHeapSlice(0, memory)
					gotAccess := []mir2.VMDataAccess{}
					vm.ObserveDataAccess = func(a mir2.VMDataAccess) { gotAccess = append(gotAccess, a) }
					r, e := vm.Call("region_eq", []mir2.Value{{I: args[0]}, {I: args[1]}, {I: args[2]}, {I: args[3]}, {I: n}})
					gotErr := ""
					if e != nil {
						gotErr = e.Error()
						if k := strings.Index(gotErr, "load: "); k >= 0 {
							gotErr = gotErr[k:]
						}
					}
					if !reflect.DeepEqual(gotAccess, wantAccess) || gotErr != wantErr || (e == nil && !reflect.DeepEqual(r, []mir2.Value{{I: want}})) || !reflect.DeepEqual(vm.ReadHeap(0, heap), memory) {
						t.Fatal("region oracle", heap, n, args, variant, r, e, want, wantErr)
					}
				}
				count++
			}
		}
	}
	t.Log("raw domain", count)
}
func TestRegionComparePrototypeTokenizer(t *testing.T) {
	source, e := os.ReadFile("../../../examples/nanz/grace_tokenizer_checked.nanz")
	if e != nil {
		t.Fatal(e)
	}
	m, e := nanz.Parse(string(source), "../../../examples/nanz/grace_tokenizer_checked.nanz")
	if e != nil {
		t.Fatal(e)
	}
	if e = hir.ApplyRegionComparePrototype(m, "region_eq"); e != nil {
		t.Fatal(e)
	}
}
func TestRegionComparePrototypeWholeTokenizer(t *testing.T) {
	path := "../../../examples/nanz/grace_tokenizer_checked.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	inputs := []string{"", "fun add(a: u8, b: u8) -> u8 { return a + b }", "fn if else while let var true false return", "enum global struct import assert", "alpha alpha beta alpha", "0 007 123456789", "(){}:,-> == = + - * / < > <= >= != !", "// comment\nfun", "// unterminated", " \t\r\n", "\"hello\" \"hello\" \"\"", "\"unterminated", "abc\x00ignored", "_ _a a1 A9", "!@?#$fun", "/", "-", "=", "<", ">", "!", "\"a\\b\"", "// one\n// two\nvar x", "abc abcd abc abd", "123abc", "funny function fn fun", strings.Repeat("x", 255), strings.Repeat("y", 256), strings.Repeat("z", 257), "fun add(a: u8, b: u8) -> u8 { return a + b }"}
	arena, tokens := make([]byte, 4096), make([]byte, 2048)
	for i := range arena {
		arena[i] = 0xa5
	}
	for i := range tokens {
		tokens[i] = 0xc3
	}
	resources := map[string][]byte{"arena": arena, "tokens": tokens}
	for i, s := range inputs {
		resources[fmt.Sprintf("src%02d", i)] = append([]byte(s), 0, 0x5a)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: resources}
	host := tokenizerHost{arena: append([]byte(nil), arena...), tokens: append([]byte(nil), tokens...), keywords: map[string]int{}}
	snapshots := []hir.ProfileMemorySnapshot{}
	results := []int64{}
	arenaWrites, tokensWrites, tokensReads := [][]uint64{}, [][]uint64{}, [][]uint64{}
	arenaReads, sourceReads := [][]uint64{}, [][]uint64{}
	for i, s := range inputs {
		n := int64(len(s))
		name := fmt.Sprintf("src%02d", i)
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "tokenizer_checked", Arguments: []hir.ProfileArgument{{Resource: name}, {Resource: "arena"}, {Resource: "tokens"}, {Value: &n}}})
		host.run(resources[name], len(s))
		memory := map[string][]byte{}
		for k, v := range resources {
			memory[k] = append([]byte(nil), v...)
		}
		memory["arena"] = append([]byte(nil), host.arena...)
		memory["tokens"] = append([]byte(nil), host.tokens...)
		snapshots = append(snapshots, hir.ProfileMemorySnapshot{Globals: host.globals(len(s)), Resources: memory})
		results = append(results, int64(host.count))
		arenaWrites = append(arenaWrites, append([]uint64(nil), host.arenaWrites...))
		tokensWrites = append(tokensWrites, append([]uint64(nil), host.tokensWrites...))
		tokensReads = append(tokensReads, append([]uint64(nil), host.tokensReads...))
		arenaReads = append(arenaReads, append([]uint64(nil), host.arenaReads...))
		sourceReads = append(sourceReads, append([]uint64(nil), host.sourceReads...))
	}
	dir := os.Getenv("MINZ_REGION_TOKENIZER_ARTIFACTS")
	write := func(name string, v any) {
		if dir == "" {
			return
		}
		os.MkdirAll(dir, 0755)
		var b []byte
		if data, ok := v.([]byte); ok {
			b = data
		} else {
			b, _ = json.MarshalIndent(v, "", "  ")
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("source.nanz", source)
	write("workload.json", w)
	write("oracle.json", map[string]any{"results": results, "snapshots": snapshots, "arena_writes": arenaWrites, "token_writes": tokensWrites, "token_reads": tokensReads, "arena_reads": arenaReads, "source_reads": sourceReads})
	checkAccess := func(call int, access hir.ProfileAccessSnapshot) {
		ar, tok := access.Resources["arena"], access.Resources["tokens"]
		if !reflect.DeepEqual(ar.Reads, arenaReads[call]) || !reflect.DeepEqual(ar.Writes, arenaWrites[call]) || !reflect.DeepEqual(tok.Writes, tokensWrites[call]) || !reflect.DeepEqual(tok.Reads, tokensReads[call]) {
			t.Fatal("independent tokenizer arena/token traffic", call)
		}
		for name, a := range access.Resources {
			if strings.HasPrefix(name, "src") {
				for off, n := range a.Writes {
					if n != 0 {
						t.Fatal("source modified", call, name, off)
					}
				}
				if name == fmt.Sprintf("src%02d", call) && !reflect.DeepEqual(a.Reads, sourceReads[call]) {
					t.Fatal("independent source reads", call)
				}
				if name != fmt.Sprintf("src%02d", call) {
					for _, n := range a.Reads {
						if n != 0 {
							t.Fatal("wrong source resource read", call, name)
						}
					}
				}
				if a.Reads[len(a.Reads)-1] != 0 {
					t.Fatal("read beyond source sentinel", call, name)
				}
			}
		}
	}
	m, err := factory()
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	write("raw.json", raw)
	write("original.hir", []byte(m.Dump()))
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	if len(raw.Results) != 30 {
		t.Fatal("incomplete tokenizer")
	}
	for i, r := range raw.Results {
		checkAccess(i, raw.Accesses[i])
		if len(r) != 1 || r[0].I != results[i] || !reflect.DeepEqual(raw.Snapshots[i], snapshots[i]) {
			write("failure.txt", []byte(fmt.Sprintf("rawcall%d got%d want%d", i, r[0].I, results[i])))
			t.Fatal("raw tokenizer independentoracle", i, r, results[i])
		}
	}
	candidate, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	candidate.GraceRulePolicy = map[string]bool{}
	candidate.ForwardCopyPolicy = map[string]bool{}
	candidate.InvariantBasePolicy = map[string]bool{}
	candidate.PointerWalkPolicy = map[string]bool{}
	if err := hir.ApplyRegionComparePrototype(candidate, "region_eq"); err != nil {
		t.Fatal(err)
	}
	pre, e := hir.RunLoopProfile(candidate, w)
	if e != nil {
		t.Fatal(e)
	}
	write("precompile-raw.json", pre)
	for i, r := range pre.Results {
		checkAccess(i, pre.Accesses[i])
		if r[0].I != results[i] || !reflect.DeepEqual(pre.Snapshots[i], snapshots[i]) {
			t.Fatal("precompile host oracle", i)
		}
	}
	write("candidate.hir", []byte(candidate.Dump()))
	steps, err := CompileHIRSteps(candidate, Options{AssertMode: "none", UseHIRGrace: true})
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	transformed, e := hir.RunLoopProfile(candidate, w)
	if e != nil {
		t.Fatal(e)
	}
	write("postcompile-raw.json", transformed)
	for i, r := range transformed.Results {
		checkAccess(i, transformed.Accesses[i])
		if r[0].I != results[i] || !reflect.DeepEqual(transformed.Snapshots[i], snapshots[i]) {
			t.Fatal("transformed hostoracle", i)
		}
	}
	write("baseline.a80", []byte(steps.Assembly))
	write("baseline.mir", []byte(steps.MIR2Opt))
	assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || assembled == nil {
		t.Fatal(err)
	}
	if len(assembled.Errors) > 0 {
		t.Fatal(assembled.Errors)
	}
	write("baseline.bin", assembled.Binary)
	run, err := RunZ80ProfileWorkload(steps, m, w, *raw)
	write("target.json", run)
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	var total int64
	for i, inv := range run.Invocations {
		if inv.Access == nil {
			t.Fatal("missing exact access")
		}
		checkAccess(i, *inv.Access)
		if inv.Results[0].I != results[i] || !reflect.DeepEqual(inv.Snapshot, snapshots[i]) {
			t.Fatal("target tokenizer independentoracle", i)
		}
		total += inv.TStates
	}
	write("cost.json", map[string]any{"t_states": total, "bytes": len(assembled.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary)), "calls": 30, "arena_bump": host.bump})
	t.Logf("tokenizer30calls %dT/%dB arena%d", total, len(assembled.Binary), host.bump)
}

func TestRegionComparePrototypeExactRefusals(t *testing.T) {
	for name, src := range map[string]string{"extra_read": strings.Replace(regionCompareSource, "i=i+1", "var ignored:u8=src[0]\n i=i+1", 1), "stride": strings.Replace(regionCompareSource, "i=i+1", "i=i+2", 1), "reverse_reads": strings.Replace(regionCompareSource, "src[src_start+i]!=arena[arena_ptr+1+i]", "arena[arena_ptr+1+i]!=src[src_start+i]", 1), "wrong_return": strings.Replace(regionCompareSource, "return 1", "return 0", 1)} {
		t.Run(name, func(t *testing.T) {
			m, e := nanz.Parse(src, "region-refuse.nanz")
			if e != nil {
				t.Fatal(e)
			}
			if hir.ApplyRegionComparePrototype(m, "region_eq") == nil {
				t.Fatal("unsupported exactshape admitted")
			}
		})
	}
}

func TestRegionComparePrototypeSecondReadTraps(t *testing.T) {
	for _, tc := range []struct {
		name        string
		left, right int64
		events      []int64
		trap        int64
	}{
		{"left_second", 15, 0, []int64{15, 0}, 16},
		{"right_second", 0, 15, []int64{0, 15, 1}, 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, optimized := range []bool{false, true} {
				m, e := nanz.Parse(regionCompareSource, "region-second-trap.nanz")
				if e != nil {
					t.Fatal(e)
				}
				if optimized {
					if e = hir.ApplyRegionComparePrototype(m, "region_eq"); e != nil {
						t.Fatal(e)
					}
				}
				vm := mir2.NewVM(hir.LowerModule(m))
				vm.EnsureHeap(16)
				memory := make([]byte, 16)
				for i := range memory {
					memory[i] = 7
				}
				vm.WriteHeapSlice(0, memory)
				events := []int64{}
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
					if a.Op != mir2.OpLoad || a.Value != 7 {
						t.Fatal("unexpected event", a)
					}
					events = append(events, a.Addr)
				}
				_, e = vm.Call("region_eq", []mir2.Value{{I: tc.left}, {I: tc.right}, {I: 0}, {I: 65535}, {I: 2}})
				want := fmt.Sprintf("load: out-of-bounds address %d (heap size 16)", tc.trap)
				if e == nil || !strings.Contains(e.Error(), want) || !reflect.DeepEqual(events, tc.events) || !reflect.DeepEqual(vm.ReadHeap(0, 16), memory) {
					t.Fatalf("optimized=%v events=%v error=%v", optimized, events, e)
				}
			}
		})
	}
}
