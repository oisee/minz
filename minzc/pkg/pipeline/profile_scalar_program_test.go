package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// This host oracle uses Go integer arithmetic and the workload bytes, never HIR
// evaluation, compiler dumps, or agreement between the two compiled candidates.
func TestMeasuredScalarChecksumProgram(t *testing.T) {
	sourcePath := "../../../examples/nanz/grace_weighted_checksum.nanz"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("MINZ_PGO_SCALAR_ARTIFACT_DIR")
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
	profile, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{AssertMode: "none", UseHIRGrace: true}
	measured, measureErr := MeasureGraceVariantSets(factory, profile, w, options, 4)
	// Selector failures discard partial evidence. Reproduce both exact policies
	// independently and retain binaries/dumps/results without weakening workload.
	failures := []string{}
	costs := []map[string]any{}
	for i, enabled := range []bool{false, true} {
		input, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		input.GraceRulePolicy = map[string]bool{"hir-constant-if": enabled, "hir-invariant-base": enabled}
		input.PointerWalkPolicy = map[string]bool{}
		if err := hir.SeedLoopLineage(input); err != nil {
			t.Fatal(err)
		}
		steps, err := CompileHIRSteps(input, options)
		if err != nil {
			t.Fatal(err)
		}
		prefix := fmt.Sprintf("candidate-%d", i)
		write(prefix+".a80", steps.Assembly)
		write(prefix+".mir", steps.MIR2Opt)
		write(prefix+".hir", input.Dump())
		lineage := hir.CurrentLoopLineage(input)
		write(prefix+"-lineage.json", lineage)
		candidateRaw, err := jointRunRaw(input, w)
		if err != nil {
			t.Fatal(err)
		}
		checkRaw(candidateRaw)
		if !reflect.DeepEqual(candidateRaw.Accesses, raw.Accesses) {
			t.Fatal("scalar hoist changed HIR data accesses")
		}
		write(prefix+"-raw-run.json", candidateRaw)
		hoisted := false
		for _, l := range lineage {
			if l.Function == "weighted_checksum" && l.Lineage.Role == "invariant-base" && l.Lineage.Rule == "hir-invariant-base" {
				hoisted = true
			}
		}
		if hoisted != enabled || strings.Contains(input.Dump(), "__grace_base_") != enabled {
			t.Fatal("actual invariant-base rewrite/lineage missing or baseline rewritten")
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil {
			t.Fatal(err)
		}
		if len(assembled.Errors) > 0 {
			t.Fatal(assembled.Errors)
		}
		write(prefix+".bin", assembled.Binary)
		write(prefix+"-symbols.json", assembled.Symbols)
		target, targetErr := RunZ80ProfileWorkload(steps, m, w, *raw)
		write(prefix+"-results.json", target)
		cost := map[string]any{"candidate": i, "common_grace_rules": enabled, "module_bytes": len(assembled.Binary), "binary_sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))}
		if targetErr != nil {
			// Earlier failures must not hide the caller or n=255 reproducer.
			// Each probe uses the original call's entry memory and exact binary.
			diagnostics := []map[string]any{}
			for j, call := range w.Calls {
				probe := w
				probe.Calls = []hir.ProfileInvocation{call}
				probe.Globals = w.Globals
				if j > 0 {
					probe.Globals = oracle[j-1].Snapshot.Globals
				}
				probeSource, err := factory()
				if err != nil {
					t.Fatal(err)
				}
				probeRaw, err := hir.RunLoopProfile(probeSource, probe)
				if err != nil {
					t.Fatal(err)
				}
				want := oracle[j]
				if len(probeRaw.Results) != 1 || len(probeRaw.Results[0]) != 1 || probeRaw.Results[0][0].I != int64(want.Result) || !reflect.DeepEqual(probeRaw.Snapshots[0], want.Snapshot) {
					t.Fatalf("isolated host oracle mismatch at %d", j)
				}
				probeRun, probeErr := RunZ80ProfileWorkload(steps, probeSource, probe, *probeRaw)
				diagnostic := map[string]any{"original_call": j, "call": call, "entry_globals": probe.Globals, "oracle": want, "run": probeRun}
				if probeErr != nil {
					diagnostic["error"] = probeErr.Error()
				} else {
					inv := probeRun.Invocations[0]
					if len(inv.Results) != 1 || inv.Results[0].I != int64(want.Result) || !reflect.DeepEqual(inv.Snapshot, want.Snapshot) {
						t.Fatalf("isolated target host oracle mismatch at %d", j)
					}
				}
				diagnostics = append(diagnostics, diagnostic)
			}
			write(prefix+"-isolated-calls.json", diagnostics)
			failures = append(failures, fmt.Sprintf("candidate %d: %v", i, targetErr))
			cost["error"] = targetErr.Error()
		} else {
			if len(target.Invocations) != len(oracle) {
				t.Fatal("incomplete target run")
			}
			var total int64
			for j, inv := range target.Invocations {
				want := oracle[j]
				if len(inv.Results) != 1 || inv.Results[0].I != int64(want.Result) || !reflect.DeepEqual(inv.Snapshot, want.Snapshot) {
					t.Fatalf("target host oracle mismatch at call %d", j)
				}
				if inv.TStates <= 0 {
					t.Fatal("missing measured invocation cost")
				}
				total += inv.TStates
			}
			cost["total_t_states"] = total
			if measured != nil {
				c := measured.Evidence.Candidates[i]
				checkRaw(c.RawRun)
				if !bytes.Equal(c.Binary, assembled.Binary) || !reflect.DeepEqual(c.Run, target) || c.TotalTStates != total {
					t.Fatal("exact measurement reproduction mismatch")
				}
			}
		}
		costs = append(costs, cost)
	}
	write("costs.json", costs)
	if measureErr != nil {
		rejection := map[string]any{"status": "rejected", "error": measureErr.Error(), "direct_failures": failures, "calls": len(oracle), "options": options}
		write("evidence.json", rejection)
		write("results.json", rejection)
		write("decisions.json", map[string]any{"status": "rejected", "reason": measureErr.Error(), "selected": nil})
		t.Fatalf("measurement: %v; direct: %v; artifacts %s", measureErr, failures, dir)
	}
	if len(failures) != 0 {
		t.Fatal(failures)
	}
	e := measured.Evidence
	write("evidence.json", e)
	write("decisions.json", measured.Decisions)
	if len(e.EligibleSites) != 0 || len(e.Candidates) != 2 || e.Candidates[0].CommonGraceRules || !e.Candidates[1].CommonGraceRules {
		t.Fatal("expected scalar baseline/common comparison")
	}
	checkRaw(e.Oracle)
	for i, c := range e.Candidates {
		hoisted := false
		for _, l := range c.Lineage {
			if l.Function == "weighted_checksum" && l.Lineage.Rule == "hir-invariant-base" && l.Lineage.Role == "invariant-base" {
				hoisted = true
			}
		}
		if hoisted != (i == 1) || strings.Contains(c.HIR, "__grace_base_") != (i == 1) {
			t.Fatal("measured candidate lacks actual rewrite")
		}
	}
	best := 0
	if e.Candidates[1].TotalTStates < e.Candidates[0].TotalTStates {
		best = 1
	}
	if e.Selected != best || !bytes.Equal(measured.Binary, e.Candidates[best].Binary) {
		t.Fatal("winner is not exact minimum measured T candidate")
	}
	replay, err := RunZ80ProfileWorkload(measured.Steps, m, w, *raw)
	if err != nil || !reflect.DeepEqual(replay, e.Candidates[best].Run) {
		t.Fatalf("winner replay: %v", err)
	}
	write("selected.bin", measured.Binary)
	write("selected.a80", measured.Steps.Assembly)
	write("selected.mir", measured.Steps.MIR2Opt)
	write("results.json", map[string]any{"status": "verified", "calls": len(oracle), "selected": best})
	t.Logf("%d calls: baseline %dT/%dB hoisted %dT/%dB selected %d", len(oracle), e.Candidates[0].TotalTStates, e.Candidates[0].ModuleBytes, e.Candidates[1].TotalTStates, e.Candidates[1].ModuleBytes, best)
}
