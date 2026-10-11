package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// This host oracle uses Go integer arithmetic and the workload bytes, never HIR
// evaluation, compiler dumps, or agreement between the two compiled candidates.
func TestWeightedPointerPrototypeMeasuredCaller(t *testing.T) {
	sourcePath := "../../../examples/nanz/grace_weighted_checksum.nanz"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("MINZ_WEIGHTED_PROTOTYPE_ARTIFACT_DIR")
	write := func(name string, value any) {
		t.Helper()
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		var data []byte
		switch v := value.(type) {
		case []byte:
			data = v
		case string:
			data = []byte(v)
		default:
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
	literal := func(n int64) hir.ProfileArgument { return hir.ProfileArgument{Value: &n} }
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", ExactDataAccess: true, MaxSteps: 2000000, MaxMemory: 65536,
		Resources: map[string][]byte{}, Globals: map[string][]byte{"checksum_calls": {250}, "checksum_last": {0x34, 0x12}}}
	for _, pattern := range []string{"zero", "ff", "ramp", "alternating", "mismatch"} {
		buf := make([]byte, 256) // Last byte is an unread guard, even at n=255.
		for i := range buf {
			switch pattern {
			case "ff":
				buf[i] = 255
			case "ramp", "mismatch":
				buf[i] = byte(i*37 + 11)
			case "alternating":
				if i%2 != 0 {
					buf[i] = 255
				}
			}
		}
		if pattern == "mismatch" {
			buf[0] ^= 1
			buf[15] ^= 0x80
			buf[254] ^= 0xff
		}
		buf[255] = 0xa5
		w.Resources[pattern] = buf
	}
	type oracleCall struct {
		Function string                    `json:"function"`
		Result   uint16                    `json:"result"`
		Snapshot hir.ProfileMemorySnapshot `json:"snapshot"`
	}
	oracle := []oracleCall{}
	calls := uint8(250)
	last := uint16(0x1234)
	// Distinct weights include carry, zero after wrapping, and accumulator wrap.
	for pi, pattern := range []string{"zero", "ff", "ramp", "alternating", "mismatch"} {
		for _, n := range []int64{0, 1, 5, 16, 64, 255} {
			a := []uint16{65535, 32768, 0, 65530, 12345}[pi]
			b := []uint16{2, 32768, 257, 17, 54321}[pi]
			seed := uint16(65500 + pi*7)
			// Most invocations go through the caller; direct calls also check the core
			// leaves observable globals alone, including zero-trip direct calls.
			fn := "checksum_packet"
			if pi == 0 {
				fn = "weighted_checksum"
			}
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: fn, Arguments: []hir.ProfileArgument{{Resource: pattern}, literal(n), literal(int64(a)), literal(int64(b)), literal(int64(seed))}})
			sum := uint64(seed)
			for _, value := range w.Resources[pattern][:int(n)] {
				sum += uint64(a) + uint64(b) + uint64(value)
			}
			result := uint16(sum & 65535)
			if fn == "checksum_packet" {
				calls++
				last = result
			}
			resources := map[string][]byte{}
			for name, data := range w.Resources {
				resources[name] = append([]byte{}, data...)
			}
			oracle = append(oracle, oracleCall{fn, result, hir.ProfileMemorySnapshot{Globals: map[string][]byte{"checksum_calls": {calls}, "checksum_last": {byte(last), byte(last >> 8)}}, Resources: resources}})
		}
	}
	write("source.nanz", source)
	write("workload.json", w)
	write("oracle.json", oracle)
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), sourcePath) }
	m, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	write("source-manifest.json", m.SourceUnits)
	raw, err := hir.RunLoopProfile(m, w)
	write("profile-run.json", raw)
	if err != nil {
		write("failure.txt", err.Error())
		t.Fatal(err)
	}
	checkRaw := func(run *hir.ProfileRun) {
		t.Helper()
		if len(run.Results) != len(oracle) || len(run.Snapshots) != len(oracle) {
			t.Fatal("incomplete raw run")
		}
		for i, want := range oracle {
			if len(run.Results[i]) != 1 || run.Results[i][0].I != int64(want.Result) || !reflect.DeepEqual(run.Snapshots[i], want.Snapshot) {
				t.Fatalf("host oracle mismatch at call %d (%s): result %v snapshot %+v want %+v", i, want.Function, run.Results[i], run.Snapshots[i], want)
			}
		}
		final := oracle[len(oracle)-1].Snapshot
		if !reflect.DeepEqual(run.Globals, final.Globals) || !reflect.DeepEqual(run.Resources, final.Resources) {
			t.Fatal("raw final memory mismatch")
		}
	}
	checkRaw(raw)
	write("profile.json", raw.Profile)
	options := Options{AssertMode: "none", UseHIRGrace: true}
	costs := []map[string]any{}
	failures := []string{}
	for index, variant := range []string{"original", "scalar-only", "pointer-only", "both"} {
		input, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		input.GraceRulePolicy = map[string]bool{"hir-invariant-base": (variant == "scalar-only" || variant == "both"), "hir-constant-if": false, "hir-guarded-pointer-walk": (variant == "pointer-only" || variant == "both")}
		if variant != "pointer-only" && variant != "both" {
			input.PointerWalkPolicy = map[string]bool{}
		}
		if err := hir.SeedLoopLineage(input); err != nil {
			t.Fatal(err)
		}
		prefix := fmt.Sprintf("candidate-%d-%s", index, variant)
		r, err := hir.RunGrace(input, 0)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if (variant == "pointer-only" || variant == "both") && r.ByRule["hir-guarded-pointer-walk"] != 1 {
			t.Fatal("weighted rewrite not realized")
		}
		lowered := hir.LowerModule(input)
		if err := mir2.Verify(lowered); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		candidateRaw, err := jointRunRaw(input, w)
		write(prefix+"-raw-run.json", candidateRaw)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		checkRaw(candidateRaw)
		if !reflect.DeepEqual(candidateRaw.Accesses, raw.Accesses) {
			t.Fatal("candidate changed exact reads")
		}
		steps, err := CompileHIRSteps(input, options)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		write(prefix+".a80", steps.Assembly)
		write(prefix+".hir", input.Dump())
		write(prefix+".mir", steps.MIR2Opt)
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil || len(assembled.Errors) > 0 {
			failures = append(failures, fmt.Sprintf("assembly %v %+v", err, assembled.Errors))
			continue
		}
		write(prefix+".bin", assembled.Binary)
		target, targetErr := RunZ80ProfileWorkload(steps, m, w, *raw)
		write(prefix+"-results.json", target)
		cost := map[string]any{"variant": variant, "bytes": len(assembled.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))}
		if targetErr != nil {
			cost["error"] = targetErr.Error()
			failures = append(failures, variant+": "+targetErr.Error())
			diagnostics := []map[string]any{}
			for j, call := range w.Calls {
				probe := w
				probe.Calls = []hir.ProfileInvocation{call}
				if j > 0 {
					probe.Globals = oracle[j-1].Snapshot.Globals
				}
				probeSource, _ := factory()
				probeRaw, err := hir.RunLoopProfile(probeSource, probe)
				if err != nil {
					t.Fatal(err)
				}
				run, err := RunZ80ProfileWorkload(steps, probeSource, probe, *probeRaw)
				d := map[string]any{"call": j, "run": run}
				if err != nil {
					d["error"] = err.Error()
				}
				diagnostics = append(diagnostics, d)
			}
			write(prefix+"-isolated-calls.json", diagnostics)
		} else {
			var total int64
			if len(target.Invocations) != len(oracle) {
				t.Fatal("incomplete target")
			}
			for j, inv := range target.Invocations {
				if len(inv.Results) != 1 || inv.Results[0].I != int64(oracle[j].Result) || !reflect.DeepEqual(inv.Snapshot, oracle[j].Snapshot) {
					t.Fatal("independent target oracle", variant, j)
				}
				total += inv.TStates
			}
			cost["total_t_states"] = total
			t.Logf("%s%dT/%dB", variant, total, len(assembled.Binary))
		}
		costs = append(costs, cost)
	}
	write("prototype-costs.json", costs)
	profileData, _ := json.Marshal(raw.Profile)
	if result, err := MeasureJointSourceSiteVariantSets(factory, profileData, w, options, 4); err == nil || result != nil {
		t.Fatal("v3 admitted weighted overlap")
	}
	if result, err := MeasureGraceVariantSets(factory, profileData, w, options, 4); err == nil || result != nil {
		t.Fatal("legacy module API admitted weighted composition")
	}
	for _, size := range []bool{false, true} {
		selected, err := MeasureWeightedJointSourceSiteVariantSets(factory, profileData, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal("weighted v4", err)
		}
		e := selected.Evidence
		if e.Schema != "minz.weighted-joint-source-site-variant-sets/v4" || len(e.Candidates) != 8 || len(e.EligibleWeightedOverlapSites) != 1 {
			t.Fatal("weighted v4 contract", e.Schema, len(e.Candidates))
		}
		best := 0
		for i, c := range e.Candidates {
			if len(c.WeightedProof) != 1 || len(c.FinalWeightedProof) != 1 {
				t.Fatal("missing structural proofs")
			}
			for j, inv := range c.Run.Invocations {
				if len(inv.Results) != 1 || inv.Results[0].I != int64(oracle[j].Result) || !reflect.DeepEqual(inv.Snapshot, oracle[j].Snapshot) {
					t.Fatal("v4 independent oracle", i, j)
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
			t.Fatal("weighted v4 did not select actual min")
		}
		replay, err := RunZ80ProfileWorkload(selected.Steps, m, w, *raw)
		if err != nil || !reflect.DeepEqual(replay, e.Candidates[best].Run) {
			t.Fatal("v4 retained binary replay", err)
		}
		write(fmt.Sprintf("v4-size-%v-evidence.json", size), e)
		t.Logf("v4 size%v selected%d %dT/%dB", size, best, e.Candidates[best].TotalTStates, e.Candidates[best].ModuleBytes)
	}

	if len(failures) > 0 {
		t.Fatal(failures)
	}
}
