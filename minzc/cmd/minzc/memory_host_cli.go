package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type memoryHostCLIFlags struct{ Directory, Workload, Profile string }

var memoryHostCLI memoryHostCLIFlags

func registerMemoryHostCLIFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&memoryHostCLI.Directory, "memory-host-arena-append", "", "measure four closed memory-host variants; save bounded-workload native artifacts in directory")
	f.StringVar(&memoryHostCLI.Workload, "memory-host-workload", "", "strict closed byte-host workload JSON")
	f.StringVar(&memoryHostCLI.Profile, "memory-host-profile", "", "optional same-executable closed source profile JSON; otherwise collect before replay")
}
func memoryHostCLIOutputNames() []string {
	names := []string{"source.nanz", "workload.json", "source-profile.json", "selection.json", "decisions.json", "selected.a80", "selected.bin"}
	for _, variant := range []string{"original", "arena", "append", "both"} {
		for _, name := range []string{"factory-baseline.json", "target.a80", "target.bin", "final.mir2", "allocation.json"} {
			names = append(names, filepath.Join(variant, name))
		}
	}
	return names
}
func canonicalMemoryHostCLIPath(path string) (string, error) {
	a, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	suffix := []string{}
	for {
		resolved, e := filepath.EvalSymlinks(a)
		if e == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(e) {
			return "", e
		}
		parent := filepath.Dir(a)
		if parent == a {
			return "", e
		}
		suffix = append(suffix, filepath.Base(a))
		a = parent
	}
}
func validateMemoryHostCLI(source string) error {
	if memoryHostPolicyCLIRequested() || memoryHostSearchCLIRequested() {
		return fmt.Errorf("memory host CLI: policy mode cannot combine with legacy mode")
	}
	return validateMemoryHostCLIPaths(source, memoryHostCLI, memoryHostCLIOutputNames())
}
func validateMemoryHostCLIPaths(source string, p memoryHostCLIFlags, outputNames []string) error {
	if p == (memoryHostCLIFlags{}) {
		return nil
	}
	fail := func(s string) error { return fmt.Errorf("memory host CLI: %s", s) }
	if e := validateMemoryHostSelectedOutput(outputFile); e != nil {
		return e
	}
	if p.Directory == "" || p.Workload == "" {
		return fail("--memory-host-arena-append and --memory-host-workload are required")
	}
	if filepath.Ext(source) != ".nanz" || backend != "z80" || target != "cpm" || (outputFormat != "" && outputFormat != "code") {
		return fail("requires original Nanz, z80 backend, CPM target and code output")
	}
	if hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || hirProfile != (hirProfileFlags{}) || hirProfileAutoDirectory != "" || hirProfileAutoWorkload != "" || hirProfileAutoFunction != "" || hirProfileJoint || hirProfileIndependentRules || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileForwardCopySites || hirProfileMixedCopyWeightedSites || hirProfileRegionCompareSites || hirProfileOrderedByteFunctions || hirProfileOrderedByteRWFunctions || hirProfileMixedCopyRegionSites || pgoProfile != "" || pgoDebug {
		return fail("cannot combine with ordinary HIR or legacy profile modes")
	}
	// These measured modes choose their own passes; accept ordinary defaults.
	if useHIRGrace || !useGrace || noGrace || disableOptimize || disableIROpt || disableReroll || disableAsmOpt || disableCodegenOpt || disableSMC || disableCTIE || ctieDebug || emitFormat != "" || gpuRun != "" || enableTAS || tasFile != "" || tasReplay != "" || dumpAST || dumpMIR || compileTrace || superoptRules != "" || visualizeMIR != "" || emitSLD || annotateTStates {
		return fail("unsupported compilation or execution options")
	}
	if listAsserts || assertLines != "" || assertLinesSet || assertControlSet || assertReceipt || assertForce != "" || (assertMode != "" && assertMode != "all") {
		return fail("assert filtering or alternate assert execution is unsupported")
	}
	if info, e := os.Stat(p.Directory); e == nil && !info.IsDir() {
		return fail("artifact directory is not a directory")
	}
	inputs := []string{source, p.Workload}
	if p.Profile != "" {
		inputs = append(inputs, p.Profile)
	}
	return validateMemoryHostCLIArtifactAliases(p.Directory, inputs, outputNames)
}
func validateMemoryHostCLIArtifactAliases(directory string, inputs, outputNames []string) error {
	fail := func(s string) error { return fmt.Errorf("memory host CLI: %s", s) }
	outputs := []string{}
	for _, name := range outputNames {
		outputs = append(outputs, filepath.Join(directory, name))
	}
	if outputFile != "" {
		outputs = append(outputs, outputFile)
	}
	paths := append(append([]string{}, inputs...), outputs...)
	for i, path := range paths {
		a, e := canonicalMemoryHostCLIPath(path)
		if e != nil {
			return e
		}
		for j := 0; j < i; j++ {
			b, e := canonicalMemoryHostCLIPath(paths[j])
			if e != nil {
				return e
			}
			x, xe := os.Stat(path)
			y, ye := os.Stat(paths[j])
			if a == b || (xe == nil && ye == nil && os.SameFile(x, y)) {
				return fail("input, output and artifact paths must be distinct")
			}
		}
	}
	return nil
}
func runMemoryHostCLI(sourceFile string) error {
	if e := validateMemoryHostCLI(sourceFile); e != nil {
		return e
	}
	source, e := readHIRProfileBytes(sourceFile, 8<<20)
	if e != nil {
		return e
	}
	var w hir.MemoryHostWorkload
	if e = decodeMemoryHostCLIWorkload(memoryHostCLI.Workload, &w); e != nil {
		return e
	}
	var profile []byte
	if memoryHostCLI.Profile != "" {
		profile, e = readHIRProfileBytes(memoryHostCLI.Profile, int64(hir.MaxMemoryHostLoopProfileBytes))
		if e != nil {
			return e
		}
	}
	factory := func() (*hir.Module, error) {
		// First admission supports one immutable source unit; imports need a later
		// explicit owned dependency and output-alias contract.
		m, e := nanz.Parse(string(source), sourceFile)
		if e != nil {
			return nil, e
		}
		expected := fmt.Sprintf("%x", sha256.Sum256(source))
		if !m.SourceUnitsComplete || len(m.SourceUnits) != 1 || m.SourceUnits[sourceFile] != expected {
			return nil, fmt.Errorf("memory host CLI: imports or incomplete source units are unsupported")
		}
		m.Target = hir.TargetCPM
		return m, nil
	}
	if profile == nil {
		m, e := factory()
		if e != nil {
			return e
		}
		run, e := hir.RunMemoryHostLoopProfile(m, w)
		if e != nil {
			return e
		}
		profile, e = json.Marshal(run)
		if e != nil {
			return e
		}
	}
	goal := "speed"
	if optSize {
		goal = "size"
	}
	selection, e := pipeline.MeasureMemoryHostArenaAppend(factory, w, profile, pipeline.MemoryHostArenaAppendSelectorOptions{Goal: goal, MaxSites: 2})
	if e != nil {
		return e
	}
	// Recheck aliases after all execution and immediately before filesystem writes.
	if e = validateMemoryHostCLI(sourceFile); e != nil {
		return e
	}
	encoded, e := json.MarshalIndent(selection, "", "  ")
	if e != nil {
		return e
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(encoded))
	candidate := selection.Candidates[selection.Selected]
	targetEvidence := candidate.Evidence.Target
	decision := struct {
		Schema          string                           `json:"schema"`
		Goal            string                           `json:"goal"`
		Selected        int                              `json:"selected"`
		Choices         hir.MemoryHostArenaAppendChoices `json:"choices"`
		SelectionSHA256 string                           `json:"selection_sha256"`
		BinarySHA256    string                           `json:"binary_sha256"`
		TStates         int64                            `json:"t_states"`
		Bytes           int                              `json:"bytes"`
	}{"minz.memory-host-arena-append-cli-decision/v1", goal, selection.Selected, candidate.Choices, digest, targetEvidence.BinarySHA256, targetEvidence.TotalTStates, len(targetEvidence.Binary)}
	if e = os.MkdirAll(memoryHostCLI.Directory, 0755); e != nil {
		return e
	}
	write := func(name string, b []byte) error {
		path := filepath.Join(memoryHostCLI.Directory, name)
		if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return e
		}
		return os.WriteFile(path, b, 0644)
	}
	for name, b := range map[string][]byte{"source.nanz": source, "source-profile.json": profile, "selected.a80": []byte(targetEvidence.Assembly), "selected.bin": targetEvidence.Binary, "selection.json": encoded} {
		if e = write(name, b); e != nil {
			return e
		}
	}
	for name, value := range map[string]any{"workload.json": selection.Association.Baseline.Workload, "decisions.json": decision} {
		b, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			return e
		}
		if e = write(name, b); e != nil {
			return e
		}
	}
	for index, c := range selection.Candidates {
		name := []string{"original", "arena", "append", "both"}[index]
		b, e := json.MarshalIndent(c.Evidence, "", "  ")
		if e != nil {
			return e
		}
		for file, b := range map[string][]byte{"factory-baseline.json": b, "target.a80": []byte(c.Evidence.Target.Assembly), "target.bin": c.Evidence.Target.Binary, "final.mir2": []byte(c.Evidence.Target.FinalMIRDump), "allocation.json": c.Evidence.Target.AllocationJSON} {
			if e = write(filepath.Join(name, file), b); e != nil {
				return e
			}
		}
	}
	if outputFile != "" {
		if e = writeMemoryHostSelectedOutput(outputFile, targetEvidence.Assembly, targetEvidence.Binary); e != nil {
			return e
		}
	}
	fmt.Printf("Closed memory-host %s selected mask%d: %dT / %dB; evidence %s\n", goal, selection.Selected, targetEvidence.TotalTStates, len(targetEvidence.Binary), strings.TrimSpace(memoryHostCLI.Directory))
	return nil
}

// JSON field names are case-exact, including recursively nested typed records.
// Maps of named hosts/regions/globals retain arbitrary user identifiers.
func checkMemoryHostCLIKeys(value any, ty reflect.Type) error {
	for ty.Kind() == reflect.Pointer {
		ty = ty.Elem()
	}
	switch ty.Kind() {
	case reflect.Struct:
		obj, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" {
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			fields[tag] = f.Type
		}
		for key, v := range obj {
			field, ok := fields[key]
			if !ok {
				return fmt.Errorf("memory host CLI: unknown or noncanonical field %q", key)
			}
			if e := checkMemoryHostCLIKeys(v, field); e != nil {
				return e
			}
		}
	case reflect.Map:
		if obj, ok := value.(map[string]any); ok {
			for _, v := range obj {
				if e := checkMemoryHostCLIKeys(v, ty.Elem()); e != nil {
					return e
				}
			}
		}
	case reflect.Slice:
		if list, ok := value.([]any); ok {
			for _, v := range list {
				if e := checkMemoryHostCLIKeys(v, ty.Elem()); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func decodeMemoryHostCLIWorkload(path string, out *hir.MemoryHostWorkload) error {
	b, e := readHIRProfileBytes(path, hirWorkloadJSONLimit)
	if e != nil {
		return e
	}
	duplicate := json.NewDecoder(bytes.NewReader(b))
	if e = checkHIRProfileJSON(duplicate, 0); e != nil {
		return e
	}
	if _, e = duplicate.Token(); e != io.EOF {
		return fmt.Errorf("memory host CLI: trailing workload JSON")
	}
	typed := json.NewDecoder(bytes.NewReader(b))
	typed.DisallowUnknownFields()
	if e = typed.Decode(out); e != nil {
		return e
	}
	var value any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if e = d.Decode(&value); e != nil {
		return e
	}
	if _, ok := value.(map[string]any); !ok {
		return fmt.Errorf("memory host CLI: workload must be a JSON object")
	}
	return checkMemoryHostCLIKeys(value, reflect.TypeOf(*out))
}
