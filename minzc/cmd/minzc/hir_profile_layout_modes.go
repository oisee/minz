package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

var hirProfileLayoutFunction, hirProfileLayoutReplay string
var hirProfileLayoutLimit = 4

type layoutModeReceipt struct {
	Schema     string                                   `json:"schema"`
	Function   string                                   `json:"function"`
	Limit      int                                      `json:"limit"`
	SizePolicy bool                                     `json:"size_policy"`
	Evidence   *pipeline.ConditionalLayoutModesEvidence `json:"evidence"`
}

func layoutModeOutputNames() []string {
	return []string{"source.nanz", "source-paths.json", "workload.json", "selection.json", "selected.a80", "selected.bin", "manifest.json"}
}
func validateHIRLayoutModes(source string) error {
	if hirProfileLayoutFunction != "" && hirProfileLayoutReplay != "" {
		return fmt.Errorf("layout modes: collect and replay cannot combine")
	}
	if hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || hirProfileLayoutLimit < 1 || hirProfileLayoutLimit > 4 || runtimeMode != "" || runtimeStackBytes != 0 || contractStrategy != "" || fuseByteWords {
		return fmt.Errorf("layout modes: incompatible mode/runtime/contract options or layout limit")
	}
	// Reuse the established profile/output/alias validation without borrowing its
	// four-mode algorithm or changing any process-global flag state.
	if e := validateHIRCompilerModes(source); e != nil {
		return e
	}
	if hirProfileLayoutReplay != "" {
		if optSize {
			return fmt.Errorf("layout modes replay: objective is fixed by saved selection")
		}
		input, e := canonicalMemoryHostCLIPath(hirProfileLayoutReplay)
		if e != nil {
			return e
		}
		output, e := canonicalMemoryHostCLIPath(hirProfileAutoDirectory)
		if e != nil {
			return e
		}
		if input == output || compilerModePathContains(input, output) || compilerModePathContains(output, input) {
			return fmt.Errorf("layout modes replay: separate training/output directories required")
		}
		if outputFile != "" {
			out, e := canonicalMemoryHostCLIPath(outputFile)
			if e != nil {
				return e
			}
			if out == input || compilerModePathContains(input, out) || compilerModePathContains(out, input) {
				return fmt.Errorf("layout modes replay: -o overlaps training artifacts")
			}
		}
	}
	return validateCompilerModePaths(hirProfileAutoDirectory, []string{source, hirProfileAutoWorkload}, layoutModeOutputNames())
}
func layoutModeOptions(size bool) []pipeline.Options {
	modes := []pipeline.Options{}
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, mir := range []bool{false, true} {
			for _, hirGrace := range []bool{false, true} {
				modes = append(modes, pipeline.Options{OptSize: size, ContractOpt: true, ContractStrategy: strategy, UseGrace: mir, UseHIRGrace: hirGrace, AssertMode: "none"})
			}
		}
	}
	return modes
}
func layoutModeSources(sourceFile, stdlibDir string, source []byte, factory func() (*hir.Module, error)) (map[string][]byte, map[string]string, map[string]string, error) {
	m, e := factory()
	if e != nil {
		return nil, nil, nil, e
	}
	if !m.SourceUnitsComplete {
		return nil, nil, nil, fmt.Errorf("layout modes: incomplete source inventory")
	}
	paths, e := resolveCompilerModeSources(sourceFile, stdlibDir, m.SourceUnits)
	if e != nil {
		return nil, nil, nil, e
	}
	units := []string{}
	for unit := range paths {
		units = append(units, unit)
	}
	sort.Strings(units)
	files := map[string][]byte{}
	mapping := map[string]string{}
	for i, unit := range units {
		data, e := os.ReadFile(paths[unit])
		if e != nil {
			return nil, nil, nil, e
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != m.SourceUnits[unit] {
			return nil, nil, nil, fmt.Errorf("layout modes: source changed before measurement")
		}
		name := fmt.Sprintf("sources/unit-%d.nanz", i)
		files[name] = data
		mapping[unit] = name
	}
	if !bytes.Equal(source, files[mapping[sourceFile]]) {
		return nil, nil, nil, fmt.Errorf("layout modes: primary source changed")
	}
	return files, mapping, paths, nil
}
func runHIRLayoutModes(sourceFile, stdlibDir string, source []byte, factory func() (*hir.Module, error)) error {
	if hirProfileLayoutReplay != "" {
		return replayHIRLayoutModes(sourceFile, stdlibDir, source, factory)
	}
	var workload hir.ProfileWorkload
	workloadBytes, e := readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if e != nil {
		return e
	}
	if e := decodeHIRProfileJSONBytes(hirProfileAutoWorkload, workloadBytes, &workload); e != nil {
		return e
	}
	files, mapping, paths, e := layoutModeSources(sourceFile, stdlibDir, source, factory)
	if e != nil {
		return e
	}
	names := layoutModeOutputNames()
	for name := range files {
		names = append(names, name)
	}
	if e = validateCompilerModePaths(hirProfileAutoDirectory, append(compilerModeSourcePaths(paths), hirProfileAutoWorkload), names); e != nil {
		return e
	}
	goal := "speed"
	if optSize {
		goal = "size"
	}
	result, e := pipeline.MeasureConditionalLayoutModes(factory, hirProfileLayoutFunction, workload, layoutModeOptions(optSize), goal, hirProfileLayoutLimit)
	if e != nil {
		return e
	}
	// Factories can retain handles to source files; recheck those actual files
	// against the captured bytes before publishing any selection or requested -o.
	for unit, path := range paths {
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if !bytes.Equal(data, files[mapping[unit]]) {
			return fmt.Errorf("layout modes: source dependency changed after measurement")
		}
	}
	currentWorkload, e := readHIRProfileBytes(hirProfileAutoWorkload, hirWorkloadJSONLimit)
	if e != nil || !bytes.Equal(currentWorkload, workloadBytes) {
		return fmt.Errorf("layout modes: workload changed after measurement")
	}
	files["source.nanz"] = source
	files["selected.a80"] = []byte(result.Steps.Assembly)
	files["selected.bin"] = result.Binary
	for name, value := range map[string]any{"source-paths.json": mapping, "workload.json": workload, "selection.json": layoutModeReceipt{Schema: "minz.conditional-layout-cli/v1", Function: hirProfileLayoutFunction, Limit: hirProfileLayoutLimit, SizePolicy: optSize, Evidence: result.Evidence}} {
		data, e := marshalCompilerModesJSON(value)
		if e != nil {
			return e
		}
		if len(data) > 64<<20 {
			return fmt.Errorf("layout modes: artifact exceeds replay limit")
		}
		files[name] = data
	}
	return publishLayoutModeFiles(files, paths, result.Evidence)
}
func publishLayoutModeFiles(files map[string][]byte, paths map[string]string, evidence *pipeline.ConditionalLayoutModesEvidence) error {
	names := []string{"manifest.json"}
	for name := range files {
		names = append(names, name)
	}
	if e := validateCompilerModePaths(hirProfileAutoDirectory, append(compilerModeSourcePaths(paths), hirProfileAutoWorkload), names); e != nil {
		return e
	}
	manifest := map[string]string{}
	for name, data := range files {
		manifest[name] = compilerModeReplayHash(data)
	}
	data, e := marshalCompilerModesJSON(manifest)
	if e != nil {
		return e
	}
	files["manifest.json"] = data
	chosen := files["selected.a80"]
	if strings.EqualFold(filepath.Ext(outputFile), ".bin") {
		chosen = files["selected.bin"]
	}
	if e = publishCompilerModeArtifacts(hirProfileAutoDirectory, files, outputFile, chosen); e != nil {
		return e
	}
	mode := evidence.Runs[evidence.SelectedMode]
	candidate := mode.Candidates[mode.Selected]
	fmt.Fprintf(os.Stderr, "[layout-modes] %s mode%d layout%d: %d T / %d B; artifacts %s\n", evidence.Goal, evidence.SelectedMode, mode.Selected, candidate.Run.TotalTStates, len(candidate.Binary), hirProfileAutoDirectory)
	return nil
}
