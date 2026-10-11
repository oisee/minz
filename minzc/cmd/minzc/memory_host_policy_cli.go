package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type memoryHostPolicyCLIFlags struct{ Directory, Policy, Association string }

var memoryHostPolicyCLI memoryHostPolicyCLIFlags
var memoryHostPolicyFlagSet *pflag.FlagSet

func registerMemoryHostPolicyCLIFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	memoryHostPolicyFlagSet = f
	f.StringVar(&memoryHostPolicyCLI.Directory, "memory-host-arena-append-policy", "", "measure four closed variants at one explicit comparison policy; save v2 artifacts")
	f.StringVar(&memoryHostPolicyCLI.Policy, "memory-host-compare-policy", "", "required policy: legacy or preserve-speed")
	f.StringVar(&memoryHostPolicyCLI.Association, "memory-host-association", "", "optional whole same-executable policy-aware association v2 JSON")
}
func memoryHostPolicyCLIRequested() bool {
	if memoryHostPolicyCLI != (memoryHostPolicyCLIFlags{}) {
		return true
	}
	if memoryHostPolicyFlagSet == nil {
		return false
	}
	for _, name := range []string{"memory-host-arena-append-policy", "memory-host-compare-policy", "memory-host-association"} {
		if flag := memoryHostPolicyFlagSet.Lookup(name); flag != nil && flag.Changed {
			return true
		}
	}
	return false
}
func resolveMemoryHostPolicyCLI() (pipeline.MemoryHostBaselineOptions, error) {
	switch memoryHostPolicyCLI.Policy {
	case "legacy":
		return pipeline.MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairCompareLegacy}, nil
	case "preserve-speed":
		return pipeline.MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairComparePreserveForSpeed}, nil
	default:
		return pipeline.MemoryHostBaselineOptions{}, fmt.Errorf("memory host policy CLI: explicit policy legacy or preserve-speed is required")
	}
}
func memoryHostPolicyCLIOutputNames() []string {
	names := []string{"source.nanz", "workload.json", "association-v2.json", "selection-v2.json", "decisions-v2.json", "selected.a80", "selected.bin"}
	for _, variant := range []string{"original", "arena", "append", "both"} {
		for _, name := range []string{"baseline-v2.json", "target.a80", "target.bin", "final.mir2", "allocation.json"} {
			names = append(names, filepath.Join(variant, name))
		}
	}
	return names
}
func validateMemoryHostPolicyCLI(source string) error {
	if memoryHostSearchCLIRequested() {
		return fmt.Errorf("memory host policy CLI: search mode is incompatible")
	}
	if memoryHostPolicyCLI.Directory == "" || memoryHostCLI.Workload == "" {
		return fmt.Errorf("memory host policy CLI: policy directory and workload are required")
	}
	legacyFlagsPresent := memoryHostCLI.Directory != "" || memoryHostCLI.Profile != ""
	for _, name := range []string{"memory-host-arena-append", "memory-host-profile"} {
		if f := memoryHostPolicyFlagSet.Lookup(name); f != nil && f.Changed {
			legacyFlagsPresent = true
		}
	}
	if legacyFlagsPresent {
		return fmt.Errorf("memory host policy CLI: legacy directory/profile flags are incompatible")
	}
	if _, e := resolveMemoryHostPolicyCLI(); e != nil {
		return e
	}
	return validateMemoryHostCLIPaths(source, memoryHostCLIFlags{Directory: memoryHostPolicyCLI.Directory, Workload: memoryHostCLI.Workload, Profile: memoryHostPolicyCLI.Association}, memoryHostPolicyCLIOutputNames())
}

func runMemoryHostPolicyCLI(sourceFile string) error {
	if e := validateMemoryHostPolicyCLI(sourceFile); e != nil {
		return e
	}
	options, e := resolveMemoryHostPolicyCLI()
	if e != nil {
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
	var association []byte
	if memoryHostPolicyCLI.Association != "" {
		association, e = readHIRProfileBytes(memoryHostPolicyCLI.Association, int64(pipeline.MaxMemoryHostPolicyAssociationBytes))
		if e != nil {
			return e
		}
	}
	factory := func() (*hir.Module, error) {
		m, e := nanz.Parse(string(source), sourceFile)
		if e != nil {
			return nil, e
		}
		expected := fmt.Sprintf("%x", sha256.Sum256(source))
		if !m.SourceUnitsComplete || len(m.SourceUnits) != 1 || m.SourceUnits[sourceFile] != expected {
			return nil, fmt.Errorf("memory host policy CLI: imports or incomplete source units are unsupported")
		}
		m.Target = hir.TargetCPM
		return m, nil
	}
	if association == nil {
		run, e := pipeline.RunZ80MemoryHostLoopProfileWithOptions(factory, w, options)
		if e != nil {
			return e
		}
		association, e = json.MarshalIndent(run, "", "  ")
		if e != nil {
			return e
		}
	}
	goal := "speed"
	if optSize {
		goal = "size"
	}
	selection, e := pipeline.MeasureMemoryHostArenaAppendWithOptions(factory, w, association, pipeline.MemoryHostArenaAppendSelectorOptions{Goal: goal, MaxSites: 2}, options)
	if e != nil {
		return e
	}
	encoded, e := json.MarshalIndent(selection, "", "  ")
	if e != nil {
		return e
	}
	candidate := selection.Candidates[selection.Selected]
	targetEvidence := candidate.Evidence.Baseline.Target
	decision := struct {
		Schema            string                             `json:"schema"`
		CompileOptions    pipeline.MemoryHostBaselineOptions `json:"compile_options"`
		Goal              string                             `json:"goal"`
		Selected          int                                `json:"selected"`
		Choices           hir.MemoryHostArenaAppendChoices   `json:"choices"`
		SelectionSHA256   string                             `json:"selection_sha256"`
		AssociationSHA256 string                             `json:"association_sha256"`
		BinarySHA256      string                             `json:"binary_sha256"`
		TStates           int64                              `json:"t_states"`
		Bytes             int                                `json:"bytes"`
	}{"minz.memory-host-arena-append-cli-decision/v2", options, goal, selection.Selected, candidate.Choices, fmt.Sprintf("%x", sha256.Sum256(encoded)), fmt.Sprintf("%x", sha256.Sum256(association)), targetEvidence.BinarySHA256, targetEvidence.TotalTStates, len(targetEvidence.Binary)}
	// All admission and execution refusals happen before any artifact writes.
	// Recheck aliases immediately before writing. IO failures may leave partial files.
	if e = validateMemoryHostPolicyCLI(sourceFile); e != nil {
		return e
	}
	if e = os.MkdirAll(memoryHostPolicyCLI.Directory, 0755); e != nil {
		return e
	}
	write := func(name string, b []byte) error {
		p := filepath.Join(memoryHostPolicyCLI.Directory, name)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return e
		}
		return os.WriteFile(p, b, 0644)
	}
	for name, b := range map[string][]byte{"source.nanz": source, "association-v2.json": association, "selection-v2.json": encoded, "selected.a80": []byte(targetEvidence.Assembly), "selected.bin": targetEvidence.Binary} {
		if e = write(name, b); e != nil {
			return e
		}
	}
	for name, v := range map[string]any{"workload.json": selection.Association.Baseline.Baseline.Workload, "decisions-v2.json": decision} {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			return e
		}
		if e = write(name, b); e != nil {
			return e
		}
	}
	for index, c := range selection.Candidates {
		name := []string{"original", "arena", "append", "both"}[index]
		payload, e := json.MarshalIndent(c.Evidence, "", "  ")
		if e != nil {
			return e
		}
		target := c.Evidence.Baseline.Target
		for file, b := range map[string][]byte{"baseline-v2.json": payload, "target.a80": []byte(target.Assembly), "target.bin": target.Binary, "final.mir2": []byte(target.FinalMIRDump), "allocation.json": target.AllocationJSON} {
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
	fmt.Printf("Closed memory-host policy %s %s selected mask%d: %dT / %dB; evidence %s\n", memoryHostPolicyCLI.Policy, goal, selection.Selected, targetEvidence.TotalTStates, len(targetEvidence.Binary), memoryHostPolicyCLI.Directory)
	return nil
}
