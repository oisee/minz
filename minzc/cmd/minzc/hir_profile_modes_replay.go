package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

var hirProfileCompilerModesReplay string

func validateHIRCompilerModesReplay(source string) error {
	if hirProfileCompilerModes || optSize {
		return fmt.Errorf("compiler modes replay: search and objective flags cannot change the saved winner")
	}
	if err := validateHIRCompilerModes(source); err != nil {
		return err
	}
	train, err := canonicalMemoryHostCLIPath(hirProfileCompilerModesReplay)
	if err != nil {
		return err
	}
	out, err := canonicalMemoryHostCLIPath(hirProfileAutoDirectory)
	if err != nil {
		return err
	}
	if train == out || compilerModePathContains(train, out) || compilerModePathContains(out, train) {
		return fmt.Errorf("compiler modes replay: training and output directories must be separate")
	}
	if outputFile != "" {
		p, err := canonicalMemoryHostCLIPath(outputFile)
		if err != nil {
			return err
		}
		if p == train || compilerModePathContains(train, p) || compilerModePathContains(p, train) {
			return fmt.Errorf("compiler modes replay: -o must be outside training artifacts")
		}
	}
	return nil
}

func compilerModeReplayHash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func compilerModeReplayEqual(a, b any) bool {
	x, e := json.Marshal(a)
	y, f := json.Marshal(b)
	return e == nil && f == nil && bytes.Equal(x, y)
}

type compilerModeReplayInput struct {
	files    map[string][]byte
	evidence pipeline.MeasuredModeEvidence
	workload hir.ProfileWorkload
	sources  map[string]string
}

func loadCompilerModeReplay(directory string) (*compilerModeReplayInput, error) {
	r := &compilerModeReplayInput{files: map[string][]byte{}}
	manifestInfo, err := os.Lstat(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return nil, err
	}
	if !manifestInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("compiler modes replay: manifest must be a regular file")
	}
	var manifest map[string]string
	if err := decodeHIRProfileJSON(filepath.Join(directory, "manifest.json"), 4<<20, &manifest); err != nil {
		return nil, err
	}
	canonicalDir, err := canonicalMemoryHostCLIPath(directory)
	if err != nil {
		return nil, err
	}
	for name, hash := range manifest {
		if filepath.IsAbs(name) || filepath.Clean(name) != name || name == "." || !compilerModePathContains(directory, filepath.Join(directory, name)) {
			return nil, fmt.Errorf("compiler modes replay: invalid manifest path")
		}
		path := filepath.Join(directory, name)
		canonical, err := canonicalMemoryHostCLIPath(path)
		if err != nil || !compilerModePathContains(canonicalDir, canonical) {
			return nil, fmt.Errorf("compiler modes replay: artifact escapes training directory")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("compiler modes replay: artifact must be a regular file")
		}
		data, err := readHIRProfileBytes(path, 64<<20)
		if err != nil {
			return nil, err
		}
		if compilerModeReplayHash(data) != hash {
			return nil, fmt.Errorf("compiler modes replay: artifact hash mismatch: %s", name)
		}
		r.files[name] = data
	}
	for name, out := range map[string]any{"selection.json": &r.evidence, "workload.json": &r.workload, "source-paths.json": &r.sources} {
		if _, ok := r.files[name]; !ok {
			return nil, fmt.Errorf("compiler modes replay: manifest omits %s", name)
		}
		if err := decodeHIRProfileJSON(filepath.Join(directory, name), 64<<20, out); err != nil {
			return nil, err
		}
		// Strict decoding reads disk; require the decoded input's exact bytes
		// still match the already-owned, hashed payload.
		again, err := readHIRProfileBytes(filepath.Join(directory, name), 64<<20)
		if err != nil || !bytes.Equal(again, r.files[name]) {
			return nil, fmt.Errorf("compiler modes replay: artifact changed during admission")
		}

	}
	required := append(compilerModeOutputNames(), "source-paths.json")
	for _, name := range required {
		if name == "manifest.json" {
			continue
		}
		if _, ok := r.files[name]; !ok {
			return nil, fmt.Errorf("compiler modes replay: manifest omits %s", name)
		}
	}
	for _, name := range r.sources {
		if _, ok := r.files[name]; !ok {
			return nil, fmt.Errorf("compiler modes replay: source manifest omission")
		}
	}
	if len(manifest) != len(required)-1+len(r.sources) {
		return nil, fmt.Errorf("compiler modes replay: unexpected artifact manifest entries")
	}
	e := &r.evidence
	digest := e.SHA256
	e.SHA256 = ""
	raw, err := json.Marshal(e)
	e.SHA256 = digest
	if err != nil || compilerModeReplayHash(raw) != digest {
		return nil, fmt.Errorf("compiler modes replay: selection digest mismatch")
	}
	if e.Schema != "minz.measured-compiler-modes/v1" || e.Compatibility == nil || len(e.Candidates) != 4 || e.Selected < 0 || e.Selected >= 4 || (e.Goal != "speed" && e.Goal != "size") {
		return nil, fmt.Errorf("compiler modes replay: invalid selection")
	}
	if err := validateCompilerModeTrainingRuns(r); err != nil {
		return nil, err
	}
	best := 0
	for i, c := range e.Candidates {
		options := pipeline.Options{ContractOpt: i&1 != 0, UseGrace: i&2 != 0, AssertMode: "none"}
		id, err := pipeline.ProfileCompilationID(options, "default")
		if err != nil || !reflect.DeepEqual(c.Options, options) || c.CompilationID != id || c.Run == nil {
			return nil, fmt.Errorf("compiler modes replay: invalid recorded mode %d", i)
		}
		prefix := fmt.Sprintf("mode-%d/", i)
		if !bytes.Equal(c.Binary, r.files[prefix+"target.bin"]) || c.Assembly != string(r.files[prefix+"target.a80"]) || c.FinalMIR != string(r.files[prefix+"final.mir2"]) || compilerModeReplayHash(c.Binary) != c.Run.BinarySHA256 {
			return nil, fmt.Errorf("compiler modes replay: candidate artifact mismatch")
		}
		if (e.Goal == "speed" && c.Run.TotalTStates < e.Candidates[best].Run.TotalTStates) || (e.Goal == "size" && len(c.Binary) < len(e.Candidates[best].Binary)) {
			best = i
		}
	}
	winner := e.Candidates[e.Selected]
	if best != e.Selected || !bytes.Equal(winner.Binary, r.files["selected.bin"]) || winner.Assembly != string(r.files["selected.a80"]) {
		return nil, fmt.Errorf("compiler modes replay: saved winner mismatch")
	}
	manifestBytes, err := readHIRProfileBytes(filepath.Join(directory, "manifest.json"), 4<<20)
	if err != nil {
		return nil, err
	}
	var savedManifest map[string]string
	if err = json.Unmarshal(manifestBytes, &savedManifest); err != nil || !reflect.DeepEqual(savedManifest, manifest) {
		return nil, fmt.Errorf("compiler modes replay: manifest changed during admission")
	}
	r.files["manifest.json"] = manifestBytes
	return r, nil
}

func runHIRCompilerModesReplay(sourceFile, stdlibDir string, source []byte, factory func() (*hir.Module, error)) error {
	saved, err := loadCompilerModeReplay(hirProfileCompilerModesReplay)
	if err != nil {
		return err
	}
	heldBytes, err := readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if err != nil {
		return err
	}
	var held hir.ProfileWorkload
	if err = decodeHIRProfileJSON(hirProfileAutoWorkload, hirWorkloadJSONLimit, &held); err != nil {
		return err
	}
	decodedBytes, err := readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if err != nil || !bytes.Equal(decodedBytes, heldBytes) {
		return fmt.Errorf("compiler modes replay: heldout workload changed during admission")
	}
	m, err := factory()
	if err != nil {
		return err
	}
	paths, err := resolveCompilerModeSources(sourceFile, stdlibDir, m.SourceUnits)
	if err != nil {
		return err
	}
	if len(saved.sources) != len(m.SourceUnits) || !bytes.Equal(source, saved.files["source.nanz"]) {
		return fmt.Errorf("compiler modes replay: source set changed")
	}
	for unit, hash := range m.SourceUnits {
		name, ok := saved.sources[unit]
		if !ok || compilerModeReplayHash(saved.files[name]) != hash {
			return fmt.Errorf("compiler modes replay: source dependency changed: %s", unit)
		}
	}
	artifactPaths := []string{}
	for name := range saved.files {
		artifactPaths = append(artifactPaths, filepath.Join(hirProfileCompilerModesReplay, name))
	}
	names := []string{"workload.json", "run.json", "oracle.json", "replay.json", "selected.bin", "selected.a80", "manifest.json"}
	if err = validateCompilerModePaths(hirProfileAutoDirectory, append(append(compilerModeSourcePaths(paths), artifactPaths...), hirProfileAutoWorkload), names); err != nil {
		return err
	}
	checkModule := func(fresh *hir.Module) error {
		ctx, err := hir.BuildWorkloadProfileContext(fresh, saved.workload)
		if err != nil {
			return err
		}
		key, err := hir.BuildProfileCompatibility(fresh, ctx)
		if err != nil {
			return err
		}
		if !compilerModeReplayEqual(key, saved.evidence.Compatibility) {
			return fmt.Errorf("compiler modes replay: requires same executable, source paths, HIR/ABI and training context")
		}
		for unit, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if compilerModeReplayHash(data) != fresh.SourceUnits[unit] {
				return fmt.Errorf("compiler modes replay: source file changed")
			}
		}
		return nil
	}
	checkSource := func() error {
		fresh, err := factory()
		if err != nil {
			return err
		}
		return checkModule(fresh)
	}
	if err = checkSource(); err != nil {
		return err
	}
	compiled, err := factory()
	if err != nil {
		return err
	}
	if err = checkModule(compiled); err != nil {
		return err
	}
	winner := saved.evidence.Candidates[saved.evidence.Selected]
	steps, err := pipeline.CompileHIRSteps(compiled, winner.Options)
	if err != nil {
		return err
	}
	if steps.Assembly != winner.Assembly || steps.MIR2Module == nil || steps.MIR2Module.Dump() != winner.FinalMIR {
		return fmt.Errorf("compiler modes replay: recovered compiler output differs from saved winner")
	}
	binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + string(saved.files["selected.a80"]))
	if err != nil {
		return err
	}
	if len(binary.Errors) != 0 || !bytes.Equal(binary.Binary, saved.files["selected.bin"]) {
		return fmt.Errorf("compiler modes replay: saved ASM does not assemble to saved BIN")
	}
	// Recovered metadata serves only the call ABI. Execute the saved assembly.
	steps.Assembly = string(saved.files["selected.a80"])
	snapshot, err := json.Marshal(steps.MIR2Module)
	if err != nil {
		return err
	}
	allocation, err := json.Marshal(steps.Allocation)
	if err != nil {
		return err
	}
	// Reproduce the recorded selected mode on training, not a new search.
	trainingSource, err := factory()
	if err != nil {
		return err
	}
	if err = checkModule(trainingSource); err != nil {
		return err
	}
	trainingOracle, err := hir.RunLoopProfile(trainingSource, saved.workload)
	if err != nil {
		return err
	}
	if !compilerModeReplayEqual(trainingOracle, saved.evidence.Oracle) {
		return fmt.Errorf("compiler modes replay: recorded training oracle was not reproduced")
	}
	trainingRun, err := pipeline.RunZ80ProfileWorkload(steps, trainingSource, saved.workload, *trainingOracle)
	if err != nil {
		return err
	}
	if err = checkModule(trainingSource); err != nil {
		return err
	}
	if !compilerModeReplayEqual(trainingRun, winner.Run) {
		return fmt.Errorf("compiler modes replay: recorded winner training execution was not reproduced")
	}
	oracleSource, err := factory()
	if err != nil {
		return err
	}
	if err = checkModule(oracleSource); err != nil {
		return err
	}
	oracle, err := hir.RunLoopProfile(oracleSource, held)
	if err != nil {
		return err
	}
	run, err := pipeline.RunZ80ProfileWorkload(steps, oracleSource, held, *oracle)
	if err != nil {
		return err
	}
	if run.BinarySHA256 != compilerModeReplayHash(saved.files["selected.bin"]) {
		return fmt.Errorf("compiler modes replay: executed binary differs from saved winner")
	}
	after, _ := json.Marshal(steps.MIR2Module)
	afterAllocation, _ := json.Marshal(steps.Allocation)
	if !bytes.Equal(snapshot, after) || !bytes.Equal(allocation, afterAllocation) {
		return fmt.Errorf("compiler modes replay: compiled semantics changed during replay")
	}
	if err = checkModule(oracleSource); err != nil {
		return err
	}
	if err = checkSource(); err != nil {
		return err
	}
	reread, err := loadCompilerModeReplay(hirProfileCompilerModesReplay)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(reread.files, saved.files) {
		return fmt.Errorf("compiler modes replay: training artifacts changed during replay")
	}
	finalHeld, err := readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if err != nil || !bytes.Equal(finalHeld, heldBytes) {
		return fmt.Errorf("compiler modes replay: heldout workload changed during replay")
	}
	payloads := map[string][]byte{"selected.bin": saved.files["selected.bin"], "selected.a80": saved.files["selected.a80"]}
	record := struct {
		Schema             string `json:"schema"`
		Selected           int    `json:"selected"`
		SelectionSHA256    string `json:"training_selection_sha256"`
		BinarySHA256       string `json:"binary_sha256"`
		TrainingReproduced bool   `json:"training_reproduced"`
	}{"minz.compiler-modes-replay/v1", saved.evidence.Selected, saved.evidence.SHA256, run.BinarySHA256, true}
	for name, value := range map[string]any{"workload.json": held, "oracle.json": oracle, "run.json": run, "replay.json": record} {
		data, err := marshalCompilerModesJSON(value)
		if err != nil {
			return err
		}
		payloads[name] = data
	}
	manifest := map[string]string{}
	for name, data := range payloads {
		manifest[name] = compilerModeReplayHash(data)
	}
	data, err := marshalCompilerModesJSON(manifest)
	if err != nil {
		return err
	}
	payloads["manifest.json"] = data
	chosen := payloads["selected.a80"]
	if strings.EqualFold(filepath.Ext(outputFile), ".bin") {
		chosen = payloads["selected.bin"]
	}
	if err = publishCompilerModeArtifacts(hirProfileAutoDirectory, payloads, outputFile, chosen); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[compiler-modes replay] fixed mode %d: %d T / %d B; saved BIN %s\n", saved.evidence.Selected, run.TotalTStates, len(saved.files["selected.bin"]), run.BinarySHA256)
	return nil
}

func validateCompilerModeTrainingRuns(saved *compilerModeReplayInput) error {
	fail := func() error { return fmt.Errorf("compiler modes replay: inconsistent training execution evidence") }
	e, w := &saved.evidence, saved.workload
	oracle := e.Oracle
	n := len(w.Calls)
	if oracle == nil || n == 0 || oracle.Profile.FailedCalls != 0 || oracle.Profile.CompletedCalls != uint64(n) || !compilerModeReplayEqual(oracle.Profile.Compatibility, e.Compatibility) || len(oracle.Results) != n || len(oracle.Snapshots) != n || (w.ExactDataAccess && len(oracle.Accesses) != n) {
		return fail()
	}
	for _, c := range e.Candidates {
		r := c.Run
		if r == nil || !compilerModeReplayEqual(r.Compatibility, e.Compatibility) || len(r.Invocations) != n || len(r.BootstrapSHA256) != n || r.TotalTStates < 0 || r.BootstrapTStates < 0 || r.ExecutedSteps <= 0 || r.ExecutedSteps > w.MaxSteps {
			return fail()
		}
		sum := int64(0)
		for i, row := range r.Invocations {
			hash, err := hex.DecodeString(r.BootstrapSHA256[i])
			if err != nil || len(hash) != 32 || r.BootstrapSHA256[i] != hex.EncodeToString(hash) {
				return fail()
			}
			if row.Function != w.Calls[i].Function || row.TStates <= 0 || sum > math.MaxInt64-row.TStates || !compilerModeReplayEqual(row.Results, oracle.Results[i]) || !compilerModeReplayEqual(row.Snapshot, oracle.Snapshots[i]) {
				return fail()
			}
			if w.ExactDataAccess && (row.Access == nil || !compilerModeReplayEqual(row.Access, oracle.Accesses[i])) {
				return fail()
			}
			sum += row.TStates
		}
		if sum != r.TotalTStates || sum > math.MaxInt64-r.BootstrapTStates || !compilerModeReplayEqual(r.Globals, oracle.Globals) || !compilerModeReplayEqual(r.Resources, oracle.Resources) {
			return fail()
		}
	}
	return nil
}
