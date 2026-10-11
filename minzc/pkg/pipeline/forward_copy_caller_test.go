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

const forwardCopyCallerSource = forwardCopySource + `
global appended:u8=250
 global arena_used:u16=0
 fun append_token(dst:^u8,src:^u8,n:u8)->u8 {
 dst[0]=n
 var copied:u8=copy_forward(dst+1,src,n)
 appended=appended+1
 arena_used=arena_used+u16(n)+1
 return copied
 }`

func TestForwardCopyPrototypeMeasuredCaller(t *testing.T) {
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 3000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{"appended": {250}, "arena_used": {0xfe, 0xff}}}
	memory := map[string][]byte{}
	for _, pattern := range []string{"ramp", "ff", "zero", "alternating", "overlap"} {
		data := make([]byte, 600)
		for i := range data {
			switch pattern {
			case "ramp", "overlap":
				data[i] = byte(i*37 + 11)
			case "ff":
				data[i] = 255
			case "alternating":
				if i%2 != 0 {
					data[i] = 255
				}
			}
		}
		w.Resources[pattern] = data
		memory[pattern] = append([]byte(nil), data...)
	}
	snapshots := []hir.ProfileMemorySnapshot{}
	reads := []map[string][]uint64{}
	writes := []map[string][]uint64{}
	results := []int64{}
	calls := byte(250)
	used := uint16(65534)
	for pi, pattern := range []string{"ramp", "ff", "zero", "alternating", "overlap"} {
		for _, n := range []int64{0, 1, 5, 16, 64, 255} {
			dst, src := 300, 0
			if pattern == "overlap" {
				dst, src = 0, 0
			}
			if pi == 3 {
				dst, src = 0, 300
			}
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "append_token", Arguments: []hir.ProfileArgument{{Resource: pattern, Offset: dst}, {Resource: pattern, Offset: src}, literal(n)}})
			rd, wr := map[string][]uint64{}, map[string][]uint64{}
			for name, data := range memory {
				rd[name] = make([]uint64, len(data))
				wr[name] = make([]uint64, len(data))
			}
			memory[pattern][dst] = byte(n)
			wr[pattern][dst]++
			for i := 0; i < int(n); i++ {
				value := memory[pattern][src+i]
				rd[pattern][src+i]++
				memory[pattern][dst+1+i] = value
				wr[pattern][dst+1+i]++
			}
			calls++
			used += uint16(n) + 1
			copyMemory := map[string][]byte{}
			for name, data := range memory {
				copyMemory[name] = append([]byte(nil), data...)
			}
			snapshots = append(snapshots, hir.ProfileMemorySnapshot{Globals: map[string][]byte{"appended": {calls}, "arena_used": {byte(used), byte(used >> 8)}}, Resources: copyMemory})
			results = append(results, n)
			reads = append(reads, rd)
			writes = append(writes, wr)
		}
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(forwardCopyCallerSource, "typed-token-allocation.nanz") }
	source, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	oracle, err := hir.RunLoopProfile(source, w)
	if err != nil {
		t.Fatal(err)
	}
	for j, got := range oracle.Results {
		if !reflect.DeepEqual(got, []mir2.Value{{I: results[j]}}) || !reflect.DeepEqual(oracle.Snapshots[j], snapshots[j]) {
			t.Fatal("raw independent allocation oracle", j, got)
		}
	}
	dir := os.Getenv("MINZ_FORWARD_COPY_ARTIFACTS")
	write := func(name string, v any) {
		if dir == "" {
			return
		}
		os.MkdirAll(dir, 0755)
		var data []byte
		switch x := v.(type) {
		case string:
			data = []byte(x)
		case []byte:
			data = x
		default:
			data, _ = json.MarshalIndent(v, "", "  ")
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("source.nanz", forwardCopyCallerSource)
	write("workload.json", w)
	write("oracle.json", map[string]any{"snapshots": snapshots, "results": results, "reads": reads, "writes": writes})
	costs := []map[string]any{}
	failures := []string{}
	for _, enabled := range []bool{false, true} {
		m, _ := factory()
		m.GraceRulePolicy = map[string]bool{hir.ForwardCopyRule: enabled}
		if err := hir.SeedLoopLineage(m); err != nil {
			t.Fatal(err)
		}
		rewrite, err := hir.RunGrace(m, 0)
		if err != nil {
			t.Fatal(err)
		}
		if enabled && rewrite.ByRule[hir.ForwardCopyRule] != 1 {
			t.Fatal("missing copy rewrite")
		}
		raw, err := jointRunRaw(m, w)
		if err != nil || !jointSameState(raw, oracle) {
			t.Fatal("raw transformed caller", err)
		}
		steps, err := CompileHIRSteps(m, Options{AssertMode: "none", UseHIRGrace: true})
		if err != nil {
			t.Fatal(err)
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil || len(assembled.Errors) > 0 {
			t.Fatal("copy assembly", err, assembled.Errors)
		}
		prefix := fmt.Sprintf("enabled-%v", enabled)
		write(prefix+".a80", steps.Assembly)
		write(prefix+".mir", steps.MIR2Opt)
		write(prefix+".bin", assembled.Binary)
		run, targetErr := RunZ80ProfileWorkload(steps, source, w, *oracle)
		write(prefix+"-target.json", run)
		cost := map[string]any{"enabled": enabled, "bytes": len(assembled.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))}
		if targetErr != nil {
			cost["error"] = targetErr.Error()
			failures = append(failures, targetErr.Error())
			diagnostics := []map[string]any{}
			for j, call := range w.Calls {
				probe := w
				probe.Calls = []hir.ProfileInvocation{call}
				if j > 0 {
					probe.Globals = snapshots[j-1].Globals
					probe.Resources = snapshots[j-1].Resources
				}
				probeSource, _ := factory()
				probeOracle, err := hir.RunLoopProfile(probeSource, probe)
				if err != nil {
					t.Fatal(err)
				}
				probeRun, err := RunZ80ProfileWorkload(steps, probeSource, probe, *probeOracle)
				d := map[string]any{"call": j, "run": probeRun}
				if err != nil {
					d["error"] = err.Error()
				}
				diagnostics = append(diagnostics, d)
			}
			write(prefix+"-isolated.json", diagnostics)

		} else {
			var total int64
			for j, inv := range run.Invocations {
				if inv.Results[0].I != results[j] || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) {
					t.Fatal("target allocation oracle", enabled, j)
				}
				for name, access := range inv.Access.Resources {
					if !reflect.DeepEqual(access.Reads, reads[j][name]) || !reflect.DeepEqual(access.Writes, writes[j][name]) {
						t.Fatal("exact sequentialcopy traffic", enabled, j, name)
					}
				}
				total += inv.TStates
			}
			cost["total_t_states"] = total
			t.Logf("enabled%v %dT/%dB", enabled, total, len(assembled.Binary))
		}
		costs = append(costs, cost)
	}
	write("costs.json", costs)
	observations, _ := json.Marshal(oracle.Profile)
	for _, size := range []bool{false, true} {
		selected, err := MeasureForwardCopySiteVariantSets(factory, observations, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal("copy measuredselector", err)
		}
		e := selected.Evidence
		if len(e.Candidates) != 2 || len(e.EligibleSites) != 1 {
			t.Fatal("incomplete copy candidates")
		}
		best := 0
		for i, c := range e.Candidates {
			if len(c.Proofs) != 1 || len(c.FinalProofs) != 1 {
				t.Fatal("missing copyproof")
			}
			for j, inv := range c.Run.Invocations {
				if inv.Results[0].I != results[j] || !reflect.DeepEqual(inv.Snapshot, snapshots[j]) {
					t.Fatal("selectorhostoracle", i, j)
				}
				for name, a := range inv.Access.Resources {
					if !reflect.DeepEqual(a.Reads, reads[j][name]) || !reflect.DeepEqual(a.Writes, writes[j][name]) {
						t.Fatal("selectorexacttraffic", i, j, name)
					}
				}
			}
			better := c.TotalTStates < e.Candidates[best].TotalTStates
			if size {
				better = c.ModuleBytes < e.Candidates[best].ModuleBytes
			}
			if better {
				best = i
			}
		}
		if e.Selected != best {
			t.Fatal("unmeasured copywinner")
		}
		replay, err := RunZ80ProfileWorkload(selected.Steps, source, w, *oracle)
		if err != nil || !reflect.DeepEqual(replay, e.Candidates[best].Run) {
			t.Fatal("copyretainedbinary", err)
		}
		copyEvidence := *e
		copyEvidence.SHA256 = ""
		data, _ := json.Marshal(copyEvidence)
		if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) {
			t.Fatal("copyevidencehash")
		}
		write(fmt.Sprintf("size-%v-evidence.json", size), e)
		write(fmt.Sprintf("size-%v-selected.bin", size), selected.Binary)
		t.Logf("copyselectorsize%v winner%d %dT/%dB", size, best, e.Candidates[best].TotalTStates, e.Candidates[best].ModuleBytes)
	}

	if len(failures) > 0 {
		t.Fatal(failures)
	}
}
