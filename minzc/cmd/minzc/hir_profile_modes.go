package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

var hirProfileCompilerModes bool

func compilerModeOutputNames() []string {
	names := []string{"source.nanz", "workload.json", "selection.json", "selected.a80", "selected.bin", "manifest.json"}
	for i := 0; i < 4; i++ {
		for _, n := range []string{"target.a80", "target.bin", "final.mir2"} {
			names = append(names, filepath.Join(fmt.Sprintf("mode-%d", i), n))
		}
	}
	return names
}
func validateHIRCompilerModes(source string) error {
	if hirProfileCompilerModesReplay != "" && hirProfileCompilerModes {
		return fmt.Errorf("compiler modes: search and replay cannot be combined")
	}
	if hirProfileAutoDirectory == "" || hirProfileAutoWorkload == "" {
		return fmt.Errorf("compiler modes: --hir-profile-auto directory and --hir-profile-auto-workload are required")
	}
	if hirProfileAutoFunction != "" || hirProfileJoint || hirProfileIndependentRules || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileForwardCopySites || hirProfileMixedCopyWeightedSites || hirProfileRegionCompareSites || hirProfileOrderedByteFunctions || hirProfileOrderedByteRWFunctions || hirProfileMixedCopyRegionSites || hirProfile != (hirProfileFlags{}) || memoryHostCLI != (memoryHostCLIFlags{}) || memoryHostPolicyCLIRequested() || memoryHostSearchCLIRequested() {
		return fmt.Errorf("compiler modes: cannot combine with other profile/function/site modes")
	}
	if filepath.Ext(source) != ".nanz" || backend != "z80" || target != "cpm" || emitFormat != "" || outputFormat != "" || gpuRun != "" || pgoProfile != "" || pgoDebug || listAsserts || generateCheckedLocalTablePath != "" || checkedLocalTablePath != "" || checkedLocalTableSHA256 != "" {
		return fmt.Errorf("compiler modes: requires Nanz, CPM, default Z80 output")
	}
	// These measured modes choose their own passes; accept ordinary defaults.
	if !useGrace || useHIRGrace || noGrace || disableOptimize || disableIROpt || disableReroll || disableAsmOpt || disableCodegenOpt || disableSMC || disableCTIE || ctieDebug || emitSLD || annotateTStates || assertForce != "" || assertLines != "" || assertLinesSet || assertControlSet || assertReceipt || enableTAS || tasFile != "" || tasReplay != "" || dumpAST || dumpMIR || compileTrace || superoptRules != "" || visualizeMIR != "" {
		return fmt.Errorf("compiler modes: unsupported options; the four modes are selected by measurement")
	}
	if assertMode != "" && assertMode != "none" && assertMode != "all" {
		return fmt.Errorf("compiler modes: alternate assertion backends unsupported")
	}
	if outputFile != "" && !strings.EqualFold(filepath.Ext(outputFile), ".a80") && !strings.EqualFold(filepath.Ext(outputFile), ".bin") {
		return fmt.Errorf("compiler modes: -o must end in .a80 or .bin")
	}
	if e := validateCompilerModeDestination(hirProfileAutoDirectory); e != nil {
		return e
	}
	return validateCompilerModePaths(hirProfileAutoDirectory, []string{source, hirProfileAutoWorkload}, compilerModeOutputNames())
}
func runHIRCompilerModes(sourceFile, stdlibDir string, source []byte, factory func() (*hir.Module, error)) error {
	var workload hir.ProfileWorkload
	if e := decodeHIRProfileJSON(hirProfileAutoWorkload, hirWorkloadJSONLimit, &workload); e != nil {
		return e
	}
	// Capture every source dependency, including imports, before measuring.
	module, e := factory()
	if e != nil {
		return e
	}
	units := make([]string, 0, len(module.SourceUnits))
	for unit := range module.SourceUnits {
		units = append(units, unit)
	}
	sort.Strings(units)
	paths, e := resolveCompilerModeSources(sourceFile, stdlibDir, module.SourceUnits)
	if e != nil {
		return e
	}
	dependencies := map[string][]byte{}
	sourcePaths := map[string]string{}
	names := append(compilerModeOutputNames(), "source-paths.json")
	for i, unit := range units {
		data, err := os.ReadFile(paths[unit])
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != module.SourceUnits[unit] {
			return fmt.Errorf("compiler modes: source dependency changed: %s", unit)
		}
		name := fmt.Sprintf("sources/unit-%d.nanz", i)
		dependencies[name] = data
		sourcePaths[unit] = name
		names = append(names, name)
	}
	if e := validateCompilerModePaths(hirProfileAutoDirectory, append(compilerModeSourcePaths(paths), hirProfileAutoWorkload), names); e != nil {
		return e
	}
	if !bytes.Equal(source, dependencies[sourcePaths[sourceFile]]) {
		return fmt.Errorf("compiler modes: primary source changed before measurement")
	}
	goal := "speed"
	if optSize {
		goal = "size"
	}
	// Osize selects the objective here; all candidate compiler policies remain
	// identical OptSize=false, so the benchmark measures exactly four modes.
	result, e := pipeline.MeasureCompilerModeVariants(factory, workload, goal, false)
	if e != nil {
		return e
	}
	for unit, name := range sourcePaths {
		if result.Evidence.Compatibility.Context.Dependencies[unit] != fmt.Sprintf("%x", sha256.Sum256(dependencies[name])) {
			return fmt.Errorf("compiler modes: measured dependency differs: %s", unit)
		}
	}
	payloads := dependencies
	payloads["source.nanz"] = source
	payloads["selected.a80"] = []byte(result.Steps.Assembly)
	payloads["selected.bin"] = result.Binary
	encoded, e := marshalCompilerModesJSON(sourcePaths)
	if e != nil {
		return e
	}
	payloads["source-paths.json"] = encoded
	encoded, e = marshalCompilerModesJSON(workload)
	if e != nil {
		return e
	}
	payloads["workload.json"] = encoded
	encoded, e = marshalCompilerModesJSON(result.Evidence)
	if e != nil {
		return e
	}
	payloads["selection.json"] = encoded
	for i, c := range result.Evidence.Candidates {
		prefix := fmt.Sprintf("mode-%d/", i)
		payloads[prefix+"target.a80"] = []byte(c.Assembly)
		payloads[prefix+"target.bin"] = c.Binary
		payloads[prefix+"final.mir2"] = []byte(c.FinalMIR)
	}
	manifest := map[string]string{}
	for name, data := range payloads {
		manifest[name] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	encoded, e = marshalCompilerModesJSON(manifest)
	if e != nil {
		return e
	}
	payloads["manifest.json"] = encoded
	chosen := []byte(result.Steps.Assembly)
	if strings.EqualFold(filepath.Ext(outputFile), ".bin") {
		chosen = result.Binary
	}
	if e := publishCompilerModeArtifacts(hirProfileAutoDirectory, payloads, outputFile, chosen); e != nil {
		return e
	}
	best := result.Evidence.Candidates[result.Evidence.Selected]
	fmt.Fprintf(os.Stderr, "[compiler-modes] %s selected mode %d: %d T / %d B; artifacts %s\n", goal, result.Evidence.Selected, best.Run.TotalTStates, len(best.Binary), hirProfileAutoDirectory)
	return nil
}

func marshalCompilerModesJSON(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

// Frontend source units use logical module names for imports. Resolve those
// names conservatively and verify the actual bytes against the frontend hash.
func resolveCompilerModeSources(primary, stdlibDir string, units map[string]string) (map[string]string, error) {
	paths := map[string]string{primary: primary}
	dirs := []string{filepath.Dir(primary)}
	if stdlibDir != "" {
		dirs = append(dirs, stdlibDir)
	}
	for len(paths) < len(units) {
		progress := false
		for unit, hash := range units {
			if _, ok := paths[unit]; ok {
				continue
			}
			matches := map[string]bool{}
			for _, dir := range dirs {
				base := strings.ReplaceAll(unit, ".", string(filepath.Separator))
				if dir == stdlibDir {
					base = strings.TrimPrefix(base, "stdlib"+string(filepath.Separator))
				}
				for _, ext := range []string{".nanz", ".minz", ".lanz", ".lizp", ".plm", ".pas"} {
					path := filepath.Join(dir, base+ext)
					data, e := os.ReadFile(path)
					if e != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != hash {
						continue
					}
					abs, e := filepath.Abs(path)
					if e != nil {
						return nil, e
					}
					matches[abs] = true
				}
			}
			if len(matches) > 1 {
				return nil, fmt.Errorf("compiler modes: ambiguous source unit %s", unit)
			}
			for path := range matches {
				paths[unit] = path
				dirs = append(dirs, filepath.Dir(path))
				progress = true
			}
		}
		if !progress {
			return nil, fmt.Errorf("compiler modes: cannot locate all source dependencies")
		}
	}
	return paths, nil
}
func compilerModeSourcePaths(paths map[string]string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		result = append(result, path)
	}
	return result
}
