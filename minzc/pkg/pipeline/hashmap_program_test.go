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
	"testing"
)

type hashmapHost struct {
	memory map[string][]byte
	access hir.ProfileAccessSnapshot
}

func newHashmapHost() *hashmapHost {
	return &hashmapHost{memory: map[string][]byte{"map_keys": make([]byte, 32), "map_vals": make([]byte, 32), "map_used": make([]byte, 16), "map_count": make([]byte, 2)}}
}
func (h *hashmapHost) read(name string, p int) byte {
	a := h.access.Globals[name]
	a.Reads[p]++
	h.access.Globals[name] = a
	return h.memory[name][p]
}
func (h *hashmapHost) write(name string, p int, v byte) {
	a := h.access.Globals[name]
	a.Writes[p]++
	h.access.Globals[name] = a
	h.memory[name][p] = v
}

// fill models the compiled map_clear, whose native lowering is fill_bytes16:
// n byte stores followed, for n >= 2, by the n-1 overlapping-LDIR reloads of the
// bytes just stored (see minzc/pkg/mir2/byte_loop.go). The read-back values are
// the stored bytes, so only the traffic model differs from a plain store loop.
func (h *hashmapHost) fill(name string, n int, v byte) {
	for i := 0; i < n; i++ {
		h.write(name, i, v)
		if i+1 < n {
			h.read(name, i)
		}
	}
}
func (h *hashmapHost) rd(name string, p int) uint16 {
	return uint16(h.read(name, 2*p)) + uint16(h.read(name, 2*p+1))*256
}
func (h *hashmapHost) wr(name string, p int, v uint16) {
	h.write(name, 2*p, byte(v))
	h.write(name, 2*p+1, byte(v>>8))
}
func (h *hashmapHost) run(call hir.ProfileInvocation) ([]mir2.Value, hir.ProfileMemorySnapshot, hir.ProfileAccessSnapshot) {
	h.access = hir.ProfileAccessSnapshot{Globals: map[string]hir.ProfileByteAccess{}, Resources: map[string]hir.ProfileByteAccess{}}
	for n, b := range h.memory {
		h.access.Globals[n] = hir.ProfileByteAccess{Reads: make([]uint64, len(b)), Writes: make([]uint64, len(b))}
	}
	key, val := uint16(0), uint16(0)
	if len(call.Arguments) > 0 {
		key = uint16(*call.Arguments[0].Value)
	}
	if len(call.Arguments) > 1 {
		val = uint16(*call.Arguments[1].Value)
	}
	get := func() uint16 {
		slot := int((key ^ (key + 37)) & 15)
		for probes := 0; probes < 16; probes++ {
			if h.read("map_used", slot) == 0 {
				return 65535
			}
			if h.rd("map_keys", slot) == key {
				return h.rd("map_vals", slot)
			}
			slot = (slot + 1) & 15
		}
		return 65535
	}
	result := []mir2.Value{}
	switch call.Function {
	case "map_clear":
		h.wr("map_count", 0, 0)
		h.fill("map_used", 16, 0)
	case "map_put":
		slot := int((key ^ (key + 37)) & 15)
		value := int64(0)
		for probes := 0; probes < 16; probes++ {
			if h.read("map_used", slot) == 0 {
				h.write("map_used", slot, 1)
				h.wr("map_keys", slot, key)
				h.wr("map_vals", slot, val)
				count := h.rd("map_count", 0)
				h.wr("map_count", 0, count+1)
				value = 1
				break
			}
			if h.rd("map_keys", slot) == key {
				h.wr("map_vals", slot, val)
				value = 1
				break
			}
			slot = (slot + 1) & 15
		}
		result = []mir2.Value{{I: value}}
	case "map_get":
		result = []mir2.Value{{I: int64(get())}}
	case "map_has":
		value := int64(1)
		if get() == 65535 {
			value = 0
		}
		result = []mir2.Value{{I: value}}
	default:
		panic("unknown original function")
	}
	snapshot := hir.ProfileMemorySnapshot{Globals: map[string][]byte{}, Resources: map[string][]byte{}}
	for n, b := range h.memory {
		snapshot.Globals[n] = append([]byte(nil), b...)
	}
	return result, snapshot, h.access
}
func TestOriginalHashmapPersistentProgram(t *testing.T) {
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
	dir := os.Getenv("MINZ_HASHMAP_ARTIFACTS")
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
			data, e = json.MarshalIndent(value, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
			t.Fatal(e)
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
	pre, e := hir.RunLoopProfile(candidate, workload)
	write("precompile-raw.json", pre)
	if e != nil {
		t.Fatal(e)
	}
	check("precompile", pre)
	steps, e := CompileHIRSteps(candidate, Options{AssertMode: "none", UseHIRGrace: true})
	write("baseline.a80", []byte(steps.Assembly))
	write("baseline.mir", []byte(steps.MIR2Opt))
	write("baseline-raw.mir", []byte(steps.MIR2Raw))
	if e != nil {
		write("failure.txt", []byte(e.Error()))
		t.Fatal(e)
	}
	write("postcompile.hir", []byte(candidate.Dump()))
	if _, e = hir.ValidateRegionCompareCompiledChoices(original, candidate, map[string]bool{}); e != nil {
		write("failure.txt", []byte(e.Error()))
		t.Fatal(e)
	}
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
	write("baseline.bin", assembled.Binary)
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
	t.Logf("original persistent hashmap%d calls %dT/%dB", len(results), total, len(assembled.Binary))
}
