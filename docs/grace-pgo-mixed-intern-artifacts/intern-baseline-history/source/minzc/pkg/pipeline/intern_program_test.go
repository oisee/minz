package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTypedInternCheckedProgramBaseline(t *testing.T) {
	sourcePath := "../../../examples/nanz/grace_intern_checked.nanz"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), sourcePath) }
	resources := map[string][]byte{"arena": make([]byte, 2048)}
	for i := range resources["arena"] {
		resources["arena"][i] = 0xa5
	}
	inputs := [][]byte{{}, {'f', 'u', 'n'}, {'f', 'n'}, {'f', 'u', 'n'}, {'f', 'u', 'm'}, {'f', 'o', 'n'}, {'r', 'e', 't', 'u', 'r', 'n'}, {'v', 'a', 'r'}, {'f', 'u', 'n'}, {'v', 'a', 'r'}, {'f', 'u'}, {'f', 'u', 'n', 'n'}, {'f', 'u', 'm'}, {}, {'r', 'e', 't', 'u', 'r', 'n'}}
	max := make([]byte, 255)
	for i := range max {
		max[i] = byte(i*37 + 11)
	}
	changed := append([]byte(nil), max...)
	changed[127] ^= 0x80
	inputs = append(inputs, max, append([]byte(nil), max...), changed, append([]byte(nil), changed...), []byte{0}, []byte{255}, []byte{0}, []byte{255}, []byte{'f', 'u', 'n'}, []byte{'f', 'u', 'n', 'n'}, []byte{}, []byte{'a', 'b', 'c'}, []byte{'a', 'b', 'd'}, []byte{'a', 'b', 'c'}, append([]byte(nil), max...))
	for i, input := range inputs {
		data := append([]byte(nil), input...)
		data = append(data, 0x5a)
		resources[fmt.Sprintf("input%02d", i)] = data
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 4000000, MaxMemory: 65536, ExactDataAccess: true, Resources: resources, Globals: map[string][]byte{"intern_bump": {0, 0}, "intern_calls": {250}, "intern_hits": {0}, "intern_last": {0, 0}, "intern_checksum": {0, 0}}}
	literal := func(x int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &x} }
	arena := append([]byte(nil), resources["arena"]...)
	bump := 0
	hits, calls := byte(0), byte(250)
	snapshots := []hir.ProfileMemorySnapshot{}
	results := []int64{}
	allReads := []map[string][]uint64{}
	allWrites := []map[string][]uint64{}
	for j, input := range inputs {
		name := fmt.Sprintf("input%02d", j)
		a := uint16([]int{65535, 32768, 0, 65530, 12345}[j%5])
		b := uint16([]int{2, 32768, 257, 17, 54321}[j%5])
		seed := uint16(65500 + j)
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "intern_checked", Arguments: []hir.ProfileArgument{{Resource: "arena"}, {Resource: name}, literal(int64(len(input))), literal(int64(a)), literal(int64(b)), literal(int64(seed))}})
		reads, writes := map[string][]uint64{}, map[string][]uint64{}
		for key, data := range resources {
			reads[key] = make([]uint64, len(data))
			writes[key] = make([]uint64, len(data))
		}
		offset := 0
		found := false
		for p := 0; p < bump; {
			slen := int(arena[p])
			reads["arena"][p]++
			if slen == len(input) {
				equal := true
				for i := 0; i < slen; i++ {
					reads[name][i]++
					reads["arena"][p+1+i]++
					if input[i] != arena[p+1+i] {
						equal = false
						break
					}
				}
				if equal {
					offset = p
					found = true
					break
				}
			}
			p += slen + 1
		}
		if found {
			hits++
		} else {
			offset = bump
			arena[offset] = byte(len(input))
			writes["arena"][offset]++
			for i, value := range input {
				reads[name][i]++
				arena[offset+1+i] = value
				writes["arena"][offset+1+i]++
			}
			bump += len(input) + 1
		}
		sum := uint16(seed)
		for i := 0; i < len(input); i++ {
			reads["arena"][offset+1+i]++
			sum += a + b + uint16(arena[offset+1+i])
		}
		calls++
		memory := map[string][]byte{}
		for key, data := range resources {
			memory[key] = append([]byte(nil), data...)
		}
		memory["arena"] = append([]byte(nil), arena...)
		globals := map[string][]byte{"intern_bump": {byte(bump), byte(bump >> 8)}, "intern_calls": {calls}, "intern_hits": {hits}, "intern_last": {byte(offset), byte(offset >> 8)}, "intern_checksum": {byte(sum), byte(sum >> 8)}}
		snapshots = append(snapshots, hir.ProfileMemorySnapshot{Globals: globals, Resources: memory})
		results = append(results, int64(sum))
		allReads = append(allReads, reads)
		allWrites = append(allWrites, writes)
	}
	dir := os.Getenv("MINZ_INTERN_PROGRAM_ARTIFACTS")
	write := func(name string, v any) {
		if dir == "" {
			return
		}
		os.MkdirAll(dir, 0755)
		var data []byte
		switch x := v.(type) {
		case []byte:
			data = x
		case string:
			data = []byte(x)
		default:
			data, _ = json.MarshalIndent(v, "", "  ")
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("source.nanz", source)
	write("workload.json", w)
	write("oracle.json", map[string]any{"results": results, "snapshots": snapshots, "reads": allReads, "writes": allWrites})
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	write("raw.json", raw)
	if err != nil {
		write("failure.txt", err.Error())
		t.Fatal(err)
	}
	if len(raw.Results) != 30 {
		t.Fatal("incomplete intern corpus")
	}
	for j, result := range raw.Results {
		if result[0].I != results[j] || !reflect.DeepEqual(raw.Snapshots[j], snapshots[j]) {
			write("failure.txt", fmt.Sprintf("raw call%d", j))
			t.Fatal("raw independent intern oracle", j, result, results[j])
		}
		for name, access := range raw.Accesses[j].Resources {
			if !reflect.DeepEqual(access.Reads, allReads[j][name]) || !reflect.DeepEqual(access.Writes, allWrites[j][name]) {
				t.Fatal("raw exact intern traffic", j, name)
			}
		}
	}
	candidate, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	candidate.GraceRulePolicy = map[string]bool{}
	candidate.PointerWalkPolicy = map[string]bool{}
	candidate.ForwardCopyPolicy = map[string]bool{}
	candidate.InvariantBasePolicy = map[string]bool{}
	steps, err := CompileHIRSteps(candidate, Options{AssertMode: "none", UseHIRGrace: true})
	if err != nil {
		write("failure.txt", err.Error())
		t.Fatal(err)
	}
	write("baseline.a80", steps.Assembly)
	write("baseline.mir", steps.MIR2Opt)
	assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(assembled.Errors) > 0 {
		t.Fatal("intern assembly", err, assembled.Errors)
	}
	write("baseline.bin", assembled.Binary)
	run, targetErr := RunZ80ProfileWorkload(steps, m, w, *raw)
	write("target.json", run)
	if targetErr != nil {
		write("failure.txt", targetErr.Error())
		diagnostics := []map[string]any{}
		for j, call := range w.Calls {
			probe := w
			probe.Calls = []hir.ProfileInvocation{call}
			if j > 0 {
				probe.Globals = snapshots[j-1].Globals
				probe.Resources = snapshots[j-1].Resources
			}
			pm, _ := factory()
			pr, err := hir.RunLoopProfile(pm, probe)
			if err != nil {
				t.Fatal(err)
			}
			zr, err := RunZ80ProfileWorkload(steps, pm, probe, *pr)
			d := map[string]any{"call": j, "run": zr}
			if err != nil {
				d["error"] = err.Error()
			}
			diagnostics = append(diagnostics, d)
		}
		write("isolated-calls.json", diagnostics)
		t.Fatal(targetErr)
	}
	var total int64
	for j, inv := range run.Invocations {
		if inv.Results[0].I != results[j] || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) {
			t.Fatal("target independentinternoracle", j)
		}
		for name, access := range inv.Access.Resources {
			if !reflect.DeepEqual(access.Reads, allReads[j][name]) || !reflect.DeepEqual(access.Writes, allWrites[j][name]) {
				t.Fatal("target exactinterntraffic", j, name)
			}
		}
		total += inv.TStates
	}
	write("baseline-cost.json", map[string]any{"total_t_states": total, "bytes": len(assembled.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary)), "calls": 30, "bump": bump, "hits": hits})
	t.Logf("internbaseline30calls%dT/%dB finalbump%d hits%d", total, len(assembled.Binary), bump, hits)
}
