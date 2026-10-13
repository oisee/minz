package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

func readLayoutModeArtifacts(directory string) (map[string][]byte, error) {
	path := filepath.Join(directory, "manifest.json")
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("layout replay: manifest must be regular")
	}
	var manifest map[string]string
	if e = decodeHIRProfileJSON(path, 4<<20, &manifest); e != nil {
		return nil, e
	}
	if len(manifest) > 1024 {
		return nil, fmt.Errorf("layout replay: too many artifacts")
	}
	root, e := canonicalMemoryHostCLIPath(directory)
	if e != nil {
		return nil, e
	}
	files := map[string][]byte{}
	for name, hash := range manifest {
		if filepath.IsAbs(name) || filepath.Clean(name) != name || name == "." || name == "manifest.json" {
			return nil, fmt.Errorf("layout replay: invalid artifact path")
		}
		path := filepath.Join(directory, name)
		canonical, e := canonicalMemoryHostCLIPath(path)
		if e != nil || !compilerModePathContains(root, canonical) {
			return nil, fmt.Errorf("layout replay: artifact escapes directory")
		}
		info, e := os.Lstat(path)
		if e != nil {
			return nil, e
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("layout replay: artifacts must be regular")
		}
		data, e := readHIRProfileBytes(path, 64<<20)
		if e != nil {
			return nil, e
		}
		if compilerModeReplayHash(data) != hash {
			return nil, fmt.Errorf("layout replay: artifact digest mismatch: %s", name)
		}
		files[name] = data
	}
	for _, name := range layoutModeOutputNames() {
		if name != "manifest.json" && files[name] == nil {
			return nil, fmt.Errorf("layout replay: missing artifact %s", name)
		}
	}
	data, e := readHIRProfileBytes(filepath.Join(directory, "manifest.json"), 4<<20)
	if e != nil {
		return nil, e
	}
	var current map[string]string
	if e = json.Unmarshal(data, &current); e != nil || !reflect.DeepEqual(current, manifest) {
		return nil, fmt.Errorf("layout replay: manifest changed")
	}
	files["manifest.json"] = data
	return files, nil
}

// Replay reproduces the complete saved training experiment for admission, then
// executes its exact saved winner on heldout. It does not select from heldout,
// and currently provides integrity/reproducibility rather than a faster replay.
func replayHIRLayoutModes(sourceFile, stdlibDir string, source []byte, factory func() (*hir.Module, error)) error {
	saved, e := readLayoutModeArtifacts(hirProfileLayoutReplay)
	if e != nil {
		return e
	}
	var receipt layoutModeReceipt
	if e = decodeHIRProfileJSONBytes("selection.json", saved["selection.json"], &receipt); e != nil {
		return e
	}
	if receipt.Schema != "minz.conditional-layout-cli/v1" || receipt.Function == "" || receipt.Limit < 1 || receipt.Limit > 4 || receipt.Evidence == nil {
		return fmt.Errorf("layout replay: invalid receipt")
	}
	var training hir.ProfileWorkload
	if e = decodeHIRProfileJSONBytes("workload.json", saved["workload.json"], &training); e != nil {
		return e
	}
	var mapping map[string]string
	if e = decodeHIRProfileJSONBytes("source-paths.json", saved["source-paths.json"], &mapping); e != nil {
		return e
	}
	sources, current, paths, e := layoutModeSources(sourceFile, stdlibDir, source, factory)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(mapping, current) || !bytes.Equal(source, saved["source.nanz"]) {
		return fmt.Errorf("layout replay: source inventory changed")
	}
	allowed := map[string]bool{}
	for _, name := range layoutModeOutputNames() {
		allowed[name] = true
	}
	for name, data := range sources {
		allowed[name] = true
		if !bytes.Equal(data, saved[name]) {
			return fmt.Errorf("layout replay: source dependency changed")
		}
	}
	for name := range saved {
		if !allowed[name] {
			return fmt.Errorf("layout replay: unexpected artifact")
		}
	}
	inputPaths := []string{hirProfileAutoWorkload}
	for name := range saved {
		inputPaths = append(inputPaths, filepath.Join(hirProfileLayoutReplay, name))
	}
	inputPaths = append(inputPaths, compilerModeSourcePaths(paths)...)
	names := []string{"workload.json", "run.json", "oracle.json", "selection.json", "selected.a80", "selected.bin", "manifest.json"}
	if e = validateCompilerModePaths(hirProfileAutoDirectory, inputPaths, names); e != nil {
		return e
	}
	goal := "speed"
	if receipt.SizePolicy {
		goal = "size"
	}
	if receipt.Evidence.Goal != goal {
		return fmt.Errorf("layout replay: objective mismatch")
	}
	// Complete reproduction checks costs, all candidates, MIR/allocation bindings,
	// current compiler executable, source/ABI and selected indexes. Hashes alone
	// cannot authorize hand-edited assembly or a forged alternative's timing.
	reproduced, e := pipeline.MeasureConditionalLayoutModes(factory, receipt.Function, training, layoutModeOptions(receipt.SizePolicy), goal, receipt.Limit)
	if e != nil {
		return e
	}
	if !compilerModeReplayEqual(reproduced.Evidence, receipt.Evidence) || reproduced.Steps.Assembly != string(saved["selected.a80"]) || !bytes.Equal(reproduced.Binary, saved["selected.bin"]) {
		return fmt.Errorf("layout replay: saved training selection cannot be reproduced")
	}
	var heldout hir.ProfileWorkload
	workloadBytes, e := readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if e != nil {
		return e
	}
	if e = decodeHIRProfileJSONBytes(hirProfileAutoWorkload, workloadBytes, &heldout); e != nil {
		return e
	}
	currentBytes, e := readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if e != nil || !bytes.Equal(workloadBytes, currentBytes) {
		return fmt.Errorf("layout replay: heldout changed during admission")
	}
	m, e := factory()
	if e != nil {
		return e
	}
	oracle, e := hir.RunLoopProfile(m, heldout)
	if e != nil {
		return e
	}
	// Execute the saved bytes after proving identity; avoid compiling -o again.
	reproduced.Steps.Assembly = string(saved["selected.a80"])
	run, e := pipeline.RunZ80ProfileWorkload(reproduced.Steps, m, heldout, *oracle)
	if e != nil {
		return e
	}
	currentSaved, e := readLayoutModeArtifacts(hirProfileLayoutReplay)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(saved, currentSaved) {
		return fmt.Errorf("layout replay: training artifacts changed during execution")
	}
	for unit, path := range paths {
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if !bytes.Equal(data, sources[mapping[unit]]) {
			return fmt.Errorf("layout replay: source file changed")
		}
	}
	currentBytes, e = readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if e != nil || !bytes.Equal(workloadBytes, currentBytes) {
		return fmt.Errorf("layout replay: heldout input changed")
	}
	files := map[string][]byte{"selected.a80": saved["selected.a80"], "selected.bin": saved["selected.bin"], "selection.json": saved["selection.json"]}
	for name, value := range map[string]any{"workload.json": heldout, "run.json": run, "oracle.json": oracle} {
		data, e := marshalCompilerModesJSON(value)
		if e != nil {
			return e
		}
		files[name] = data
	}
	manifest := map[string]string{}
	for name, data := range files {
		manifest[name] = compilerModeReplayHash(data)
	}
	encoded, e := marshalCompilerModesJSON(manifest)
	if e != nil {
		return e
	}
	files["manifest.json"] = encoded
	if e = validateCompilerModePaths(hirProfileAutoDirectory, inputPaths, names); e != nil {
		return e
	}
	chosen := saved["selected.a80"]
	if strings.EqualFold(filepath.Ext(outputFile), ".bin") {
		chosen = saved["selected.bin"]
	}
	if e = publishCompilerModeArtifacts(hirProfileAutoDirectory, files, outputFile, chosen); e != nil {
		return e
	}
	fmt.Fprintf(os.Stderr, "[layout-modes-replay] saved winner: %d heldout T / %d B; artifacts %s\n", run.TotalTStates, len(saved["selected.bin"]), hirProfileAutoDirectory)
	return nil
}
