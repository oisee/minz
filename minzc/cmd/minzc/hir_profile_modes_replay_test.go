package main

import (
	"bytes"
	"encoding/json"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func compilerModeReplayTestState(t *testing.T) {
	compilerModeTestState(t)
	old := hirProfileCompilerModesReplay
	t.Cleanup(func() { hirProfileCompilerModesReplay = old })
	hirProfileCompilerModesReplay = ""
}

func TestCompilerModeReplayActualFrozenHeldout(t *testing.T) {
	compilerModeReplayTestState(t)
	source := "../../../examples/nanz/pfcco_weighted_packet.nanz"
	dir := t.TempDir()
	train := filepath.Join(dir, "training")
	hirProfileAutoDirectory = train
	hirProfileAutoWorkload = "../../../docs/grace-pfcco-prospective-heldout/artifacts/training-workload.json"
	outputFile = filepath.Join(dir, "training.bin")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	hirProfileCompilerModes = false
	hirProfileCompilerModesReplay = train
	hirProfileAutoDirectory = filepath.Join(dir, "heldout")
	hirProfileAutoWorkload = "../../../docs/grace-pfcco-prospective-heldout/artifacts/heldout-workload.json"
	outputFile = filepath.Join(dir, "heldout.bin")
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	replay, err := os.ReadFile(outputFile)
	if err != nil || !bytes.Equal(replay, saved) {
		t.Fatal("heldout changed saved winner", err)
	}
	raw, err := os.ReadFile(filepath.Join(hirProfileAutoDirectory, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var run pipeline.Z80ProfileWorkloadEvidence
	if err = json.Unmarshal(raw, &run); err != nil {
		t.Fatal(err)
	}
	var workload hir.ProfileWorkload
	if err = decodeHIRProfileJSON(hirProfileAutoWorkload, hirWorkloadJSONLimit, &workload); err != nil {
		t.Fatal(err)
	}
	if len(run.Invocations) != 12 || run.TotalTStates != 5172 {
		t.Fatal("frozen heldout metrics", run.TotalTStates)
	}
	trace := int64(65532)
	for i, call := range workload.Calls {
		a, b := *call.Arguments[0].Value, *call.Arguments[1].Value
		want := ((b-a)&65535 + 3*((a-b)&65535)) & 65535
		row := run.Invocations[i]
		trace = (trace + 2) & 65535
		if len(row.Results) != 1 || row.Results[0].I != want {
			t.Fatal("independent packet oracle", i, row.Results, want)
		}
		if !bytes.Equal(row.Snapshot.Globals["packet_trace"], []byte{byte(trace), byte(trace >> 8)}) {
			t.Fatal("independent trace state", i)
		}
	}
}

func TestCompilerModeReplayRejectsMutatedTraining(t *testing.T) {
	compilerModeReplayTestState(t)
	source := "../../../examples/nanz/pfcco_weighted_packet.nanz"
	dir := t.TempDir()
	train := filepath.Join(dir, "training")
	hirProfileAutoDirectory = train
	hirProfileAutoWorkload = "../../../docs/grace-pfcco-prospective-heldout/artifacts/training-workload.json"
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	original, err := loadCompilerModeReplay(train)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"binary", "missing-manifest-entry", "selected-index", "selected-flags", "source", "negative-total", "forged-positive-total", "forged-result", "wrong-run-context", "corrupt-manifest"} {
		t.Run(kind, func(t *testing.T) {
			mutated := filepath.Join(dir, kind)
			os.Mkdir(mutated, 0755)
			files := map[string][]byte{}
			for name, data := range original.files {
				files[name] = append([]byte(nil), data...)
			}
			switch kind {
			case "binary":
				files["selected.bin"][0] ^= 1
			case "missing-manifest-entry":
				var m map[string]string
				json.Unmarshal(files["manifest.json"], &m)
				delete(m, "mode-3/final.mir2")
				files["manifest.json"], _ = json.Marshal(m)
			case "corrupt-manifest":
				files["manifest.json"] = []byte("{broken")
			case "source":
				files["source.nanz"] = []byte("fun changed()->u8{return 7}")
			default:
				var e pipeline.MeasuredModeEvidence
				json.Unmarshal(files["selection.json"], &e)
				switch kind {
				case "selected-index":
					e.Selected = 99
				case "selected-flags":
					e.Candidates[e.Selected].Options.UseHIRGrace = true
				case "negative-total":
					e.Candidates[e.Selected].Run.TotalTStates = -1
				case "forged-positive-total":
					e.Candidates[e.Selected].Run.Invocations[0].TStates--
					e.Candidates[e.Selected].Run.TotalTStates--
				case "forged-result":
					e.Candidates[e.Selected].Run.Invocations[0].Results[0].I++
				case "wrong-run-context":
					e.Candidates[e.Selected].Run.Compatibility.Context.TargetID = "other"
				}
				e.SHA256 = ""
				raw, _ := json.Marshal(e)
				e.SHA256 = compilerModeReplayHash(raw)
				files["selection.json"], _ = json.Marshal(e)
				var m map[string]string
				json.Unmarshal(files["manifest.json"], &m)
				m["selection.json"] = compilerModeReplayHash(files["selection.json"])
				files["manifest.json"], _ = json.Marshal(m)
			}
			for name, data := range files {
				path := filepath.Join(mutated, name)
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, data, 0644)
			}
			hirProfileCompilerModes = false
			hirProfileCompilerModesReplay = mutated
			hirProfileAutoDirectory = filepath.Join(dir, kind+"-out")
			hirProfileAutoWorkload = "../../../docs/grace-pfcco-prospective-heldout/artifacts/heldout-workload.json"
			if err := compile(source); err == nil {
				t.Fatal("mutated training accepted", kind)
			} else if kind == "forged-positive-total" && !strings.Contains(err.Error(), "training execution was not reproduced") {
				t.Fatal("consistent forged timing did not reach independent reproduction", err)
			}
			if _, err := os.Lstat(hirProfileAutoDirectory); !os.IsNotExist(err) {
				t.Fatal("failure published output", err)
			}
		})
	}
}

func TestCompilerModeReplayChangedImportedSource(t *testing.T) {
	compilerModeReplayTestState(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "caller.nanz")
	library := filepath.Join(dir, "library.nanz")
	os.WriteFile(source, []byte("import library { advance }\nfun arithmetic(n:u8)->u8{return advance(n)}"), 0644)
	os.WriteFile(library, []byte("fun advance(n:u8)->u8{return n+7}"), 0644)
	n := int64(250)
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}, Calls: []hir.ProfileInvocation{{Function: "arithmetic", Arguments: []hir.ProfileArgument{{Value: &n}}}}}
	workload := filepath.Join(dir, "workload.json")
	writeHIRProfileJSON(workload, w)
	hirProfileAutoDirectory = filepath.Join(dir, "training")
	hirProfileAutoWorkload = workload
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	hirProfileCompilerModes = false
	hirProfileCompilerModesReplay = hirProfileAutoDirectory
	hirProfileAutoDirectory = filepath.Join(dir, "heldout")
	os.WriteFile(library, []byte("fun advance(n:u8)->u8{return n+8}"), 0644)
	if err := compile(source); err == nil {
		t.Fatal("changed import admitted")
	}
	if _, err := os.Stat(hirProfileAutoDirectory); !os.IsNotExist(err) {
		t.Fatal("failed import check emitted", err)
	}
	os.WriteFile(library, []byte("fun advance(n:u8)->u8{return n+7}"), 0644)
	os.WriteFile(source, []byte("import library { advance }\nfun arithmetic(n:u8)->u8{return advance(n)+1}"), 0644)
	if err := compile(source); err == nil {
		t.Fatal("changed primary admitted")
	}
}

func TestCompilerModeReplayFlagAndPathAdmission(t *testing.T) {
	compilerModeReplayTestState(t)
	source := filepath.Join(t.TempDir(), "source.nanz")
	os.WriteFile(source, []byte("fun id(n:u8)->u8{return n}"), 0644)
	hirProfileCompilerModes = false
	hirProfileCompilerModesReplay = filepath.Join(t.TempDir(), "training")
	if err := validateHIRProfileAuto(source); err != nil {
		t.Fatal(err)
	}
	hirProfileCompilerModes = true
	if err := validateHIRProfileAuto(source); err == nil {
		t.Fatal("search and replay accepted")
	}
	hirProfileCompilerModes = false
	optSize = true
	if err := validateHIRProfileAuto(source); err == nil {
		t.Fatal("objective change accepted")
	}
	optSize = false
	outputFile = filepath.Join(hirProfileCompilerModesReplay, "custom.bin")
	if err := validateHIRProfileAuto(source); err == nil {
		t.Fatal("training output overwrite accepted")
	}
}

func TestCompilerModeReplayRejectsHeldoutDiskMutation(t *testing.T) {
	compilerModeReplayTestState(t)
	source := "../../../examples/nanz/pfcco_weighted_packet.nanz"
	dir := t.TempDir()
	train := filepath.Join(dir, "training")
	hirProfileAutoDirectory = train
	hirProfileAutoWorkload = "../../../docs/grace-pfcco-prospective-heldout/artifacts/training-workload.json"
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	hirProfileCompilerModes = false
	hirProfileCompilerModesReplay = train
	hirProfileAutoDirectory = filepath.Join(dir, "heldout")
	held, err := os.ReadFile("../../../docs/grace-pfcco-prospective-heldout/artifacts/heldout-workload.json")
	if err != nil {
		t.Fatal(err)
	}
	hirProfileAutoWorkload = filepath.Join(dir, "heldout.json")
	os.WriteFile(hirProfileAutoWorkload, held, 0644)
	sourceBytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	factory := func() (*hir.Module, error) {
		calls++
		if calls == 3 {
			os.WriteFile(hirProfileAutoWorkload, append(held, ' '), 0644)
		}
		m, e := nanz.ParseWithOpts(string(sourceBytes), source, nanz.ParseOpts{BaseDir: filepath.Dir(source)})
		if e == nil {
			m.Target = hir.TargetCPM
		}
		return m, e
	}
	if err := runHIRCompilerModesReplay(source, "", sourceBytes, factory); err == nil || !strings.Contains(err.Error(), "heldout workload changed") {
		t.Fatal("heldout disk mutation not rejected", err)
	}
	if _, err := os.Stat(hirProfileAutoDirectory); !os.IsNotExist(err) {
		t.Fatal("mutation published", err)
	}
}
