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

func TestHashmapRD16PrototypeProgram(t *testing.T) {
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
	dir := os.Getenv("MINZ_HASHMAP_RD16_ARTIFACTS")
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
	ids, e := hir.EligibleRegionCompareSites(candidate)
	if e != nil || len(ids) != 0 {
		t.Fatal("unexpected legal region", ids, e)
	}
	if e = hir.ApplyRegionCompareSiteChoices(candidate, compat, map[string]bool{}); e != nil {
		t.Fatal(e)
	}
	if _, e = hir.ValidateRegionCompareChoices(original, candidate, map[string]bool{}); e != nil {
		t.Fatal(e)
	}
	if e = hir.ApplyHashmapRD16Prototype(candidate); e != nil {
		t.Fatal(e)
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
	t.Logf("rd16 candidate persistent hashmap%d calls %dT/%dB", len(results), total, len(assembled.Binary))
}

func TestHashmapRD16PrototypeRawDomain(t *testing.T) {
	source := `fun rd16(arr:^u8,idx:u16)->u16 {var base:u16=idx*2
return u16(arr[base])+u16(arr[base+1])*256}`
	a, err := nanz.Parse(source, "rd16-domain.nanz")
	if err != nil {
		t.Fatal(err)
	}
	b, err := nanz.Parse(source, "rd16-domain.nanz")
	if err != nil {
		t.Fatal(err)
	}
	if err = hir.ApplyHashmapRD16Prototype(b); err != nil {
		t.Fatal(err)
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
			vm.WriteHeapSlice(0, memory)
			vms = append(vms, vm)
		}
		check := func(arr, idx int64) {
			base := uint16(uint16(idx) * 2)
			low := int(uint16(uint16(arr) + base))
			high := int(uint16(uint16(arr) + base + 1))
			want := int64(0)
			wantErr := ""
			events := []mir2.VMDataAccess{}
			for j, address := range []int{low, high} {
				if address >= heap {
					wantErr = fmt.Sprintf("load: out-of-bounds address %d (heap size %d)", address, heap)
					break
				}
				value := memory[address]
				events = append(events, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(address), Value: value})
				if j == 0 {
					want = int64(value)
				} else {
					want += int64(value) * 256
				}
			}
			for variant, vm := range vms {
				observed := []mir2.VMDataAccess{}
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { observed = append(observed, a) }
				got, err := vm.Call("rd16", []mir2.Value{{I: arr}, {I: idx}})
				gotErr := ""
				if err != nil {
					gotErr = err.Error()
					if j := strings.Index(gotErr, "load: "); j >= 0 {
						gotErr = gotErr[j:]
					}
				}
				if gotErr != wantErr || !reflect.DeepEqual(observed, events) || (err == nil && !reflect.DeepEqual(got, []mir2.Value{{I: want}})) {
					t.Fatal("ordered rd16 oracle", heap, arr, idx, variant, got, err, observed, want, wantErr, events)
				}
			}
			count++
		}
		for _, idx := range []int64{0, 1, 2, 7, 8, 15, 16, 255, 256, 32767, 32768, 32769, 65533, 65534, 65535} {
			for _, arr := range []int64{0, 1, 2, 4, 5, 14, 15, 65534, 65535} {
				check(arr, idx)
			}
		}
		if heap == 65536 {
			for idx := int64(0); idx < 65536; idx++ {
				check(0, idx)
				check(65535, idx)
			}
		}
		for _, vm := range vms {
			if !reflect.DeepEqual(vm.ReadHeap(0, heap), memory) {
				t.Fatal("unexpected memory write")
			}
		}
	}
	t.Logf("independent ordered-byte oracle %d cases, both variants; exhaustive u16 indices at bases 0/65535, plus tiny heap first/second traps", count)
}

func TestHashmapRD16PrototypeSourceRefusals(t *testing.T) {
	source := `fun rd16(arr:^u8,idx:u16)->u16 {var base:u16=idx*2
return u16(arr[base])+u16(arr[base+1])*256}`
	for _, mutate := range []func(*hir.Module){
		func(m *hir.Module) { m.Funcs[0].Params[1].SMC = true },
		func(m *hir.Module) { m.Funcs[0].IsIO = true },
		func(m *hir.Module) { m.Funcs[0].Body.Body[0].(*hir.VarDeclStmt).Init = hir.U16(65535) },
	} {
		m, e := nanz.Parse(source, "rd16-refusal.nanz")
		if e != nil {
			t.Fatal(e)
		}
		mutate(m)
		before := m.Dump()
		if e = hir.ApplyHashmapRD16Prototype(m); e == nil {
			t.Fatal("unsupported source accepted")
		}
		if m.Dump() != before {
			t.Fatal("refused input mutated")
		}
	}
}

// Target RAM has wrap semantics and no heap trap. This checks both actual byte
// addresses, results, and full RAM preservation excluding explicit code/stack.
func TestHashmapRD16PrototypeTargetBoundaries(t *testing.T) {
	source := `fun rd16(arr:^u8,idx:u16)->u16 {var base:u16=idx*2
 return u16(arr[base])+u16(arr[base+1])*256}`
	for variant := 0; variant < 2; variant++ {
		m, e := nanz.Parse(source, "rd16-target.nanz")
		if e != nil {
			t.Fatal(e)
		}
		m.GraceRulePolicy = map[string]bool{}
		m.PointerWalkPolicy = map[string]bool{}
		m.InvariantBasePolicy = map[string]bool{}
		m.ForwardCopyPolicy = map[string]bool{}
		m.RegionComparePolicy = map[string]bool{}
		if variant == 1 {
			if e = hir.ApplyHashmapRD16Prototype(m); e != nil {
				t.Fatal(e)
			}
		}
		steps, e := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true})
		if e != nil {
			t.Fatal(e)
		}
		asm := z80asm.NewAssembler()
		assembled, e := asm.AssembleString("ORG 0x8000\nCALL rd16\nDI\nHALT\n" + steps.Assembly)
		if e != nil || len(assembled.Errors) > 0 {
			t.Fatal(e, assembled.Errors)
		}
		for _, idx := range []uint16{0, 1, 255, 256, 32767, 32768, 32769, 65534, 65535} {
			for _, arr := range []uint16{0, 1, 65534, 65535} {
				low := arr + idx*2
				high := low + 1
				// Avoid overwriting the executing program or harness stack with test data.
				if (low >= 0x8000 && low < 0xA000) || (high >= 0x8000 && high < 0xA000) || (low >= 0xEF00 && low < 0xF100) || (high >= 0xEF00 && high < 0xF100) {
					continue
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				z.SetSMCTracker(nil)
				memory := make([]byte, 65536)
				for j := range memory {
					memory[j] = byte(j*37 + 11)
				}
				if e = z.LoadMemory(0, memory); e != nil {
					t.Fatal(e)
				}
				if e = z.LoadMemory(0x8000, assembled.Binary); e != nil {
					t.Fatal(e)
				}
				profiler := emulator.NewProfiler()
				z.SetProfiler(profiler)
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xF000, HL: arr, DE: idx})
				for n := 0; !z.IsHalted(); n++ {
					if n > 1000 {
						t.Fatal("no HALT")
					}
					z.Step()
				}
				want := uint16(memory[low]) + uint16(memory[high])*256
				if z.GetRegisters().HL != want || profiler.ReadCount[low] != 1 || profiler.ReadCount[high] != 1 {
					t.Fatal("target bytes", variant, arr, idx, z.GetRegisters().HL, want, profiler.ReadCount[low], profiler.ReadCount[high])
				}
				for j := range memory {
					if (j >= 0x8000 && j < 0x8000+len(assembled.Binary)) || (j >= 0xEF00 && j < 0xF100) {
						continue
					}
					if profiler.WriteCount[j] != 0 {
						t.Fatal("unexpected target data store", variant, arr, idx, j)
					}
				}
			}
		}
	}
}
