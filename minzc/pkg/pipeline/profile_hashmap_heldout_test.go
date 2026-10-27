package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func hashmapHeldoutWorkloads() (hir.ProfileWorkload, hir.ProfileWorkload) {
	makeW := func() hir.ProfileWorkload {
		return hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}}
	}
	train, heldout := makeW(), makeW()
	add := func(w *hir.ProfileWorkload, name string, args ...int64) {
		call := hir.ProfileInvocation{Function: name}
		for _, v := range args {
			v := v
			call.Arguments = append(call.Arguments, hir.ProfileArgument{Value: &v})
		}
		w.Calls = append(w.Calls, call)
	}
	add(&train, "map_clear")
	for _, pair := range [][2]int64{{5, 100}, {13, 200}, {21, 300}, {42, 400}, {77, 500}} {
		add(&train, "map_put", pair[0], pair[1])
		add(&train, "map_get", pair[0])
	}
	add(&train, "map_put", 13, 999)
	add(&train, "map_get", 13)
	add(&train, "map_get", 98)
	add(&train, "map_has", 42)
	add(&heldout, "map_clear")
	add(&heldout, "map_get", 65535)
	for i := int64(0); i < 16; i++ {
		add(&heldout, "map_put", i*256, (65535-i*4093)&65535)
	}
	for i := int64(15); i >= 0; i-- {
		add(&heldout, "map_get", i*256)
	}
	add(&heldout, "map_put", 65535, 123)
	add(&heldout, "map_get", 65535)
	add(&heldout, "map_put", 0, 65535)
	add(&heldout, "map_has", 0)
	add(&heldout, "map_get", 0)
	add(&heldout, "map_put", 2048, 0)
	add(&heldout, "map_has", 2048)
	add(&heldout, "map_get", 2048)
	add(&heldout, "map_clear")
	add(&heldout, "map_get", 2048)
	add(&heldout, "map_put", 65535, 0)
	add(&heldout, "map_get", 65535)
	add(&heldout, "map_put", 32768, 32768)
	add(&heldout, "map_get", 32768)
	return train, heldout
}

func checkHashmapIndependentRun(t *testing.T, w hir.ProfileWorkload, raw *hir.ProfileRun, target *Z80ProfileWorkloadEvidence) {
	t.Helper()
	host := newHashmapHost()
	for i, call := range w.Calls {
		result, snapshot, access := host.run(call)
		if raw != nil && (!reflect.DeepEqual(raw.Results[i], result) || !reflect.DeepEqual(raw.Snapshots[i], snapshot) || !reflect.DeepEqual(raw.Accesses[i], access)) {
			t.Fatalf("independent raw oracle differs at call%d", i)
		}
		if target != nil {
			inv := target.Invocations[i]
			if !reflect.DeepEqual(inv.Results, result) || !reflect.DeepEqual(inv.Snapshot, snapshot) || inv.Access == nil || !reflect.DeepEqual(*inv.Access, access) {
				t.Fatalf("independent native oracle differs at call%d", i)
			}
		}
	}
}

func TestHashmapMeasuredTrainingAndUnseenWorkload(t *testing.T) {
	path := "../../../examples/nanz/21_hashmap_test.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	// Both workloads exist and are saved before profiling or choosing candidates.
	train, heldout := hashmapHeldoutWorkloads()
	dir := os.Getenv("MINZ_HASHMAP_HELDOUT_EVIDENCE")
	save := func(name string, value any) {
		if dir == "" {
			return
		}
		var data []byte
		switch v := value.(type) {
		case []byte:
			data = v
		case string:
			data = []byte(v)
		default:
			data, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	save("source.nanz", source)
	save("training-workload.json", train)
	save("heldout-workload.json", heldout)
	independentOracle := func(w hir.ProfileWorkload) []map[string]any {
		host := newHashmapHost()
		out := []map[string]any{}
		for _, call := range w.Calls {
			result, snapshot, access := host.run(call)
			out = append(out, map[string]any{"results": result, "snapshot": snapshot, "access": access})
		}
		return out
	}
	save("training-independent-oracle.json", independentOracle(train))
	save("heldout-independent-oracle.json", independentOracle(heldout))
	module, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	trainingRaw, err := hir.RunLoopProfile(module, train)
	if err != nil {
		t.Fatal(err)
	}
	checkHashmapIndependentRun(t, train, trainingRaw, nil)
	profile, err := json.Marshal(trainingRaw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	save("training-profile.json", trainingRaw.Profile)
	save("training-raw.json", trainingRaw)
	// Heldout is never supplied to the selector or the training-profile loader.
	for _, size := range []bool{false, true} {
		result, err := MeasureOrderedByteRWFunctionVariantSets(factory, profile, train, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Evidence.Candidates) != 4 || len(result.Evidence.EligibleSites) != 2 {
			t.Fatal("expected rd16/wr16 four-candidate selection")
		}
		goal := "speed"
		if size {
			goal = "size"
		}
		save(goal+"-selection.json", result.Evidence)
		save(goal+"-selected.a80", result.Steps.Assembly)
		save(goal+"-selected.bin", result.Binary)
		save(goal+"-selected.mir2", result.Steps.MIR2Opt)
		save(goal+"-selected-allocation.json", result.Steps.Allocation)
		for _, candidate := range result.Evidence.Candidates {
			checkHashmapIndependentRun(t, train, candidate.RawRun, candidate.Run)
		}
		original, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		heldoutRaw, err := hir.RunLoopProfile(original, heldout)
		if err != nil {
			t.Fatal(err)
		}
		checkHashmapIndependentRun(t, heldout, heldoutRaw, nil)
		save("heldout-raw.json", heldoutRaw)
		baselineModule, err := factory()
		if err != nil {
			t.Fatal(err)
		}
		// The measured candidates compile with an applied ordered-byte-rw policy,
		// which disables variant search; the baseline uses the same configuration so
		// its binary is the all-original candidate rather than an independently
		// searched program.
		baselineSteps, err := CompileHIRSteps(baselineModule, Options{AssertMode: "none", DisableVariantSearch: true})
		if err != nil {
			t.Fatal(err)
		}
		baseline, err := RunZ80ProfileWorkload(baselineSteps, original, heldout, *heldoutRaw)
		if err != nil {
			t.Fatal(err)
		}
		checkHashmapIndependentRun(t, heldout, nil, baseline)
		selected, err := RunZ80ProfileWorkload(result.Steps, original, heldout, *heldoutRaw)
		if err != nil {
			t.Fatal(err)
		}
		checkHashmapIndependentRun(t, heldout, nil, selected)
		if selected.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(result.Binary)) {
			t.Fatal("heldout used a different selected binary")
		}
		save(goal+"-heldout-baseline.json", baseline)
		save(goal+"-heldout-selected.json", selected)
		save(goal+"-baseline.a80", baselineSteps.Assembly)
		baselineBIN, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + baselineSteps.Assembly)
		if err != nil || len(baselineBIN.Errors) > 0 {
			t.Fatal("baseline assembly", err)
		}
		if baseline.BinarySHA256 != result.Evidence.Candidates[0].BinarySHA256 {
			t.Fatal("heldout baseline differs from all-original training binary")
		}
		save(goal+"-baseline.bin", baselineBIN.Binary)
		for i, candidate := range result.Evidence.Candidates {
			save(fmt.Sprintf("%s-candidate%d.bin", goal, i), candidate.Binary)
		}
		// Frozen 2026-10-27 after O9 (fe3f327fb): byte-pair assembly made the
		// original rd16/wr16 code as fast as the ordered-byte variants on this
		// program, so the measured selection is the all-original candidate
		// (heldout all-original 245106 -> 204318 T, both variants 228348/226638 ->
		// 204318/215262 T) and a strict win over the baseline no longer exists.
		// Kept, and re-aimed at the unseen workload: the selected program must be
		// at least as good as the all-original baseline in both objectives, and no
		// measured alternative may beat it on the selection's own goal when that
		// alternative is itself measured on the heldout workload.
		if selected.TotalTStates > baseline.TotalTStates || len(result.Binary) > len(baselineBIN.Binary) {
			t.Fatal("frozen heldout program regressed against the all-original baseline")
		}
		for i, candidate := range result.Evidence.Candidates {
			if i == result.Evidence.Selected {
				continue
			}
			alternativeModule, err := factory()
			if err != nil {
				t.Fatal(err)
			}
			if err := hir.ApplyOrderedByteRWChoices(alternativeModule, result.Evidence.Compatibility, candidate.RequestedSites); err != nil {
				t.Fatal(err)
			}
			alternativeSteps, err := CompileHIRSteps(alternativeModule, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size, DisableVariantSearch: true})
			if err != nil {
				t.Fatal(err)
			}
			alternativeSource, err := factory()
			if err != nil {
				t.Fatal(err)
			}
			alternative, err := RunZ80ProfileWorkload(alternativeSteps, alternativeSource, heldout, *heldoutRaw)
			if err != nil {
				t.Fatal(err)
			}
			if alternative.BinarySHA256 != candidate.BinarySHA256 {
				t.Fatalf("heldout candidate%d is not the measured binary", i)
			}
			// Encoded size is a property of the binary, not of the workload.
			better := alternative.TotalTStates < selected.TotalTStates
			if goal == "size" {
				better = candidate.ModuleBytes < len(result.Binary)
			}
			if better {
				t.Fatalf("heldout contradicts the measured %s selection: candidate%d is better", goal, i)
			}
		}
		save(goal+"-summary.json", map[string]any{"selected": result.Evidence.Selected, "training_calls": len(train.Calls), "heldout_calls": len(heldout.Calls), "baseline_bytes": len(baselineBIN.Binary), "selected_bytes": len(result.Binary), "training_baseline_t": result.Evidence.Candidates[0].TotalTStates, "training_selected_t": result.Evidence.Candidates[result.Evidence.Selected].TotalTStates, "heldout_baseline_t": baseline.TotalTStates, "heldout_selected_t": selected.TotalTStates, "selected_binary_sha256": selected.BinarySHA256})
		t.Logf("%s selected%d training %d→%dT %d→%dB; heldout %d→%dT calls%d", goal, result.Evidence.Selected, result.Evidence.Candidates[0].TotalTStates, result.Evidence.Candidates[result.Evidence.Selected].TotalTStates, result.Evidence.Candidates[0].ModuleBytes, len(result.Binary), baseline.TotalTStates, selected.TotalTStates, len(heldout.Calls))
	}
}
