package pipeline

import (
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

	"strings"
	"testing"
)

func TestHashmapWR16PrototypeProgram(t *testing.T) {
	path := "../../../examples/nanz/21_hashmap_test.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	workload := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}}
	add := func(name string, args ...int64) {
		call := hir.ProfileInvocation{Function: name, Arguments: []hir.ProfileArgument{}}
		for _, v := range args {
			owned := v
			call.Arguments = append(call.Arguments, hir.ProfileArgument{Value: &owned})
		}
		workload.Calls = append(workload.Calls, call)
	}
	add("map_clear")
	add("map_get", 99)
	add("map_put", 5, 100)
	add("map_put", 13, 200)
	add("map_get", 13)
	add("map_put", 5, 300)
	add("map_has", 5)
	add("map_put", 0, 999)
	add("map_put", 65535, 65535)
	add("map_has", 65535)
	add("map_get", 65535)
	for _, k := range []int64{21, 29, 37, 45, 53, 61, 69, 77, 85, 93, 101, 109} {
		add("map_put", k, k*3)
	}
	add("map_put", 117, 123)
	add("map_get", 117)
	add("map_put", 13, 65535)
	add("map_has", 13)
	add("map_get", 0)
	add("map_clear")
	add("map_get", 5)
	add("map_put", 65535, 0)
	add("map_get", 65535)
	add("map_has", 65535)
	add("map_put", 0, 65535)
	add("map_has", 0)
	add("map_get", 0)
	dir := os.Getenv("MINZ_HASHMAP_WR16_ARTIFACTS")
	if dir != "" {
		if e = os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
	}
	write := func(name string, value any) {
		if dir == "" {
			return
		}
		var data []byte
		if b, ok := value.([]byte); ok {
			data = b
		} else {
			var err error
			data, err = json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("source.nanz", source)
	write("workload.json", workload)
	host := newHashmapHost()
	expected := []map[string]any{}
	results := [][]mir2.Value{}
	snapshots := []hir.ProfileMemorySnapshot{}
	accesses := []hir.ProfileAccessSnapshot{}
	for _, call := range workload.Calls {
		result, snapshot, access := host.run(call)
		results = append(results, result)
		snapshots = append(snapshots, snapshot)
		accesses = append(accesses, access)
		expected = append(expected, map[string]any{"function": call.Function, "results": result, "snapshot": snapshot, "access": access})
	}
	write("host-oracle.json", expected)
	check := func(label string, run *hir.ProfileRun) {
		if run == nil || len(run.Results) != len(results) {
			t.Fatal(label, "incomplete")
		}
		for j := range results {
			if !reflect.DeepEqual(run.Results[j], results[j]) || !reflect.DeepEqual(run.Snapshots[j], snapshots[j]) || !reflect.DeepEqual(run.Accesses[j], accesses[j]) {
				write("failure.txt", []byte(fmt.Sprintf("%s call%d @%s gotresults%v want%v snapshots=%t traffic=%t", label, j, workload.Calls[j].Function, run.Results[j], results[j], reflect.DeepEqual(run.Snapshots[j], snapshots[j]), reflect.DeepEqual(run.Accesses[j], accesses[j]))))
				t.Fatal(label, j, workload.Calls[j].Function, run.Results[j], results[j], "snapshot", reflect.DeepEqual(run.Snapshots[j], snapshots[j]), "access", reflect.DeepEqual(run.Accesses[j], accesses[j]))
			}
		}
	}
	original, e := factory()
	if e != nil {
		t.Fatal(e)
	}
	raw, e := hir.RunLoopProfile(original, workload)
	write("raw.json", raw)
	if e != nil {
		write("failure.txt", []byte(e.Error()))
		t.Fatal(e)
	}
	check("original", raw)
	candidate, e := factory()
	if e != nil {
		t.Fatal(e)
	}
	context, e := hir.BuildWorkloadProfileContext(candidate, workload)
	if e != nil {
		t.Fatal(e)
	}
	compat, e := hir.BuildProfileCompatibility(candidate, context)
	if e != nil {
		t.Fatal(e)
	}
	ids, e := hir.EligibleOrderedByteHelpers(candidate)
	if e != nil || len(ids) != 1 {
		t.Fatal(ids, e)
	}
	if e = hir.ApplyOrderedByteHelperChoices(candidate, compat, map[string]bool{ids[0]: true}); e != nil {
		t.Fatal(e)
	}
	if os.Getenv("MINZ_HASHMAP_WR16_DISABLE") != "1" {
		if e = hir.ApplyHashmapWR16Prototype(candidate); e != nil {
			t.Fatal(e)
		}
	}
	pre, e := hir.RunLoopProfile(candidate, workload)
	write("precompile-raw.json", pre)
	if e != nil {
		t.Fatal(e)
	}
	check("precompile", pre)
	steps, e := CompileHIRSteps(candidate, Options{AssertMode: "none", UseHIRGrace: true})
	write("candidate.a80", []byte(steps.Assembly))
	write("candidate.mir", []byte(steps.MIR2Opt))
	write("baseline-raw.mir", []byte(steps.MIR2Raw))
	if e != nil {
		write("failure.txt", []byte(e.Error()))
		t.Fatal(e)
	}
	write("postcompile.hir", []byte(candidate.Dump()))
	post, e := hir.RunLoopProfile(candidate, workload)
	write("postcompile-raw.json", post)
	if e != nil {
		t.Fatal(e)
	}
	check("postcompile", post)
	assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil || len(assembled.Errors) != 0 {
		t.Fatal(e, assembled.Errors)
	}
	write("candidate.bin", assembled.Binary)
	target, e := RunZ80ProfileWorkload(steps, original, workload, *raw)
	write("target.json", target)
	if e != nil {
		write("failure.txt", []byte(e.Error()))
		t.Fatal(e)
	}
	var total int64
	for j, inv := range target.Invocations {
		if !reflect.DeepEqual(inv.Results, results[j]) || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) || inv.Access == nil || !reflect.DeepEqual(*inv.Access, accesses[j]) {
			t.Fatal("target independent host", j)
		}
		total += inv.TStates
	}
	write("cost.json", map[string]any{"calls": len(results), "t_states": total, "bytes": len(assembled.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))})
	t.Logf("rd16+wr16 candidate persistent hashmap%d calls %dT/%dB", len(results), total, len(assembled.Binary))
}

func TestHashmapWR16PrototypeRawDomain(t *testing.T) {
	src := `fun wr16(arr:^u8,idx:u16,val:u16){var base:u16=idx*2
 arr[base]=u8(val)
 arr[base+1]=u8(val/256)}`
	a, e := nanz.Parse(src, "wr16-domain.nanz")
	if e != nil {
		t.Fatal(e)
	}
	b, e := nanz.Parse(src, "wr16-domain.nanz")
	if e != nil {
		t.Fatal(e)
	}
	if e = hir.ApplyHashmapWR16Prototype(b); e != nil {
		t.Fatal(e)
	}
	mods := []*mir2.Module{hir.LowerModule(a), hir.LowerModule(b)}
	count := 0
	for _, heap := range []int{5, 6, 16, 65536} {
		memory := make([]byte, heap)
		for j := range memory {
			memory[j] = byte(j*37 + 11)
		}
		vms := []*mir2.VM{}
		for _, m := range mods {
			vm := mir2.NewVM(m)
			vm.EnsureHeap(int64(heap))
			vms = append(vms, vm)
		}
		check := func(arr, idx, val uint16) {
			wantMemory := append([]byte(nil), memory...)
			events := []mir2.VMDataAccess{}
			wantErr := ""
			low := int(arr + idx*2)
			high := int(arr + idx*2 + 1)
			for j, address := range []int{low, high} {
				if address >= heap {
					wantErr = fmt.Sprintf("store: out-of-bounds address %d (heap size %d)", address, heap)
					break
				}
				value := byte(val)
				if j == 1 {
					value = byte(val / 256)
				}
				wantMemory[address] = value
				events = append(events, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(address), Value: value})
			}
			for variant, vm := range vms {
				vm.WriteHeapSlice(0, memory)
				observed := []mir2.VMDataAccess{}
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { observed = append(observed, a) }
				_, e := vm.Call("wr16", []mir2.Value{{I: int64(arr)}, {I: int64(idx)}, {I: int64(val)}})
				err := ""
				if e != nil {
					err = e.Error()
					if j := strings.Index(err, "store: "); j >= 0 {
						err = err[j:]
					}
				}
				if err != wantErr || !reflect.DeepEqual(observed, events) || !reflect.DeepEqual(vm.ReadHeap(0, heap), wantMemory) {
					t.Fatal("ordered partialstore", heap, arr, idx, val, variant, e, wantErr, observed, events)
				}
			}
			count++
		}
		for _, idx := range []uint16{0, 1, 2, 7, 8, 255, 256, 32767, 32768, 32769, 65534, 65535} {
			for _, arr := range []uint16{0, 1, 4, 5, 14, 15, 65534, 65535} {
				for _, val := range []uint16{0, 1, 255, 256, 257, 0xABCD, 65535} {
					check(arr, idx, val)
				}
			}
		}
		if heap == 65536 {
			for idx := 0; idx < 65536; idx++ {
				check(0, uint16(idx), uint16(idx*257))
				check(65535, uint16(idx), uint16(idx*257))
			}
		}
	}
	t.Logf("independent ordered-store/partial-memory oracle %d cases per variant", count)
}

func TestHashmapWR16PrototypeTargetStoreOrder(t *testing.T) {
	src := `fun wr16(arr:^u8,idx:u16,val:u16){var base:u16=idx*2
 arr[base]=u8(val)
 arr[base+1]=u8(val/256)}`
	count := 0
	for variant := 0; variant < 2; variant++ {
		m, e := nanz.Parse(src, "wr16-target.nanz")
		if e != nil {
			t.Fatal(e)
		}
		m.GraceRulePolicy = map[string]bool{}
		m.PointerWalkPolicy = map[string]bool{}
		m.InvariantBasePolicy = map[string]bool{}
		m.ForwardCopyPolicy = map[string]bool{}
		m.RegionComparePolicy = map[string]bool{}
		if variant == 1 {
			if e = hir.ApplyHashmapWR16Prototype(m); e != nil {
				t.Fatal(e)
			}
		}
		steps, e := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true})
		if e != nil {
			t.Fatal(e)
		}
		assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL wr16\nDI\nHALT\n" + steps.Assembly)
		if e != nil || len(assembled.Errors) > 0 {
			t.Fatal(e, assembled.Errors)
		}
		ignored := func(j int) bool {
			return (j >= 0x8000 && j < 0x8000+len(assembled.Binary)) || (j >= 0xEF00 && j < 0xF100)
		}
		for _, idx := range []uint16{0, 1, 255, 256, 32767, 32768, 32769, 65534, 65535} {
			for _, arr := range []uint16{0, 1, 65534, 65535} {
				for _, val := range []uint16{0, 255, 256, 0xABCD, 65535} {
					low := arr + idx*2
					high := low + 1
					if ignored(int(low)) || ignored(int(high)) {
						continue
					}
					memory := make([]byte, 65536)
					for j := range memory {
						memory[j] = byte(j*37 + 11)
					}
					want := append([]byte(nil), memory...)
					want[low] = byte(val)
					want[high] = byte(val >> 8)
					platform := emulator.NewRemogattoZ80WithScreen()
					platform.SetROMEnd(0)
					z := platform.RemogattoZ80
					z.Reset()
					z.SetSMCTracker(nil)
					if e = z.LoadMemory(0, memory); e != nil {
						t.Fatal(e)
					}
					if e = z.LoadMemory(0x8000, assembled.Binary); e != nil {
						t.Fatal(e)
					}
					profiler := emulator.NewProfiler()
					z.SetProfiler(profiler)
					z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xF000})
					mf := steps.MIR2Module.FuncByName("wr16")
					for k, arg := range []uint16{arr, idx, val} {
						loc := assertParamLocation(mf.Contract.Params[k], steps.Allocation)
						if loc.Kind != mir2.LocReg || loc.Name == "" {
							t.Fatal("unsupported argument location", loc)
						}
						r := z.GetRegisters()
						switch loc.Name {
						case "HL":
							r.HL = arg
						case "DE":
							r.DE = arg
						case "BC":
							r.BC = arg
						case "IX":
							r.IX = arg
						case "IY":
							r.IY = arg
						default:
							t.Fatal("unknown register", loc.Name)
						}
						z.SetRegisters(r)
					}
					writes := []mir2.VMDataAccess{}
					previous := [65536]uint32{}
					for n := 0; !z.IsHalted(); n++ {
						if n > 1000 {
							t.Fatal("no HALT")
						}
						z.Step()
						for j := 0; j < 65536; j++ {
							if ignored(j) {
								continue
							}
							delta := profiler.WriteCount[j] - previous[j]
							for k := uint32(0); k < delta; k++ {
								writes = append(writes, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(j), Value: z.GetMemory(uint16(j))})
							}
							previous[j] = profiler.WriteCount[j]
						}
					}
					expected := []mir2.VMDataAccess{{Op: mir2.OpStore, Addr: int64(low), Value: byte(val)}, {Op: mir2.OpStore, Addr: int64(high), Value: byte(val >> 8)}}
					if !reflect.DeepEqual(writes, expected) {
						t.Fatal("actual exact ordered byte stores", variant, arr, idx, val, writes, expected)
					}
					for j := range want {
						if !ignored(j) && z.GetMemory(uint16(j)) != want[j] {
							t.Fatal("target fullRAM", variant, arr, idx, val, j)
						}
					}
					count++
				}
			}
		}
	}
	t.Logf("actual Z80 exact two ordered stores +fullRAM %d cases", count)
}
