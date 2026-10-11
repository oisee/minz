package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"
)

type memoryHostSearchCLIFlags struct{ Directory, LegacyAssociation, PreserveAssociation string }

var memoryHostSearchCLI memoryHostSearchCLIFlags
var memoryHostSearchFlagSet *pflag.FlagSet
var memoryHostSearchFlagNames = []string{"memory-host-arena-append-search", "memory-host-search-legacy-association", "memory-host-search-preserve-association"}

func registerMemoryHostSearchCLIFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	memoryHostSearchFlagSet = f
	f.StringVar(&memoryHostSearchCLI.Directory, memoryHostSearchFlagNames[0], "", "measure eight closed policy/mask variants; save search artifacts")
	f.StringVar(&memoryHostSearchCLI.LegacyAssociation, memoryHostSearchFlagNames[1], "", "paired whole Legacy policy-v2 association input")
	f.StringVar(&memoryHostSearchCLI.PreserveAssociation, memoryHostSearchFlagNames[2], "", "paired whole PreserveForSpeed policy-v2 association input")
}
func memoryHostSearchCLIRequested() bool {
	if memoryHostSearchCLI != (memoryHostSearchCLIFlags{}) {
		return true
	}
	if memoryHostSearchFlagSet != nil {
		for _, n := range memoryHostSearchFlagNames {
			if f := memoryHostSearchFlagSet.Lookup(n); f != nil && f.Changed {
				return true
			}
		}
	}
	return false
}
func memoryHostSearchCLIOutputNames() []string {
	names := []string{"source.nanz", "workload.json", "association-legacy-v2.json", "association-preserve-v2.json", "search.json", "decisions.json", "selected.a80", "selected.bin", "selection-policy0-v2.json", "selection-policy1-v2.json"}
	for policy := 0; policy < 2; policy++ {
		for mask := 0; mask < 4; mask++ {
			for _, n := range []string{"baseline-v2.json", "target.a80", "target.bin", "final.mir2", "allocation.json"} {
				names = append(names, filepath.Join(fmt.Sprintf("policy%d-mask%d", policy, mask), n))
			}
		}
	}
	return names
}
func validateMemoryHostSearchCLI(source string) error {
	if memoryHostSearchCLI.Directory == "" || memoryHostCLI.Workload == "" {
		return fmt.Errorf("memory host search CLI: directory and workload required")
	}
	if memoryHostPolicyCLIRequested() || memoryHostCLI.Directory != "" || memoryHostCLI.Profile != "" {
		return fmt.Errorf("memory host search CLI: old/fixed policy modes are incompatible")
	}
	for _, n := range []string{"memory-host-arena-append", "memory-host-profile"} {
		if f := memoryHostSearchFlagSet.Lookup(n); f != nil && f.Changed {
			return fmt.Errorf("memory host search CLI: old flags are incompatible")
		}
	}
	for _, n := range memoryHostSearchFlagNames[1:] {
		if f := memoryHostSearchFlagSet.Lookup(n); f != nil && f.Changed && f.Value.String() == "" {
			return fmt.Errorf("memory host search CLI: explicitly empty association input")
		}
	}
	if (memoryHostSearchCLI.LegacyAssociation == "") != (memoryHostSearchCLI.PreserveAssociation == "") {
		return fmt.Errorf("memory host search CLI: both association inputs or neither required")
	}
	names := memoryHostSearchCLIOutputNames()
	if e := validateMemoryHostCLIPaths(source, memoryHostCLIFlags{Directory: memoryHostSearchCLI.Directory, Workload: memoryHostCLI.Workload}, names); e != nil {
		return e
	}
	inputs := []string{source, memoryHostCLI.Workload}
	if memoryHostSearchCLI.LegacyAssociation != "" {
		inputs = append(inputs, memoryHostSearchCLI.LegacyAssociation, memoryHostSearchCLI.PreserveAssociation)
	}
	return validateMemoryHostCLIArtifactAliases(memoryHostSearchCLI.Directory, inputs, names)
}
func runMemoryHostSearchCLI(sourceFile string) error {
	if e := validateMemoryHostSearchCLI(sourceFile); e != nil {
		return e
	}
	source, e := readHIRProfileBytes(sourceFile, 8<<20)
	if e != nil {
		return e
	}
	const sourceSHA = "0e92689c6bb4a92fd681c9bc70d5d50549631a0fdd669cd026f723cf4e197526"
	if fmt.Sprintf("%x", sha256.Sum256(source)) != sourceSHA {
		return fmt.Errorf("memory host search CLI: exact original SelfLanz source required")
	}
	factory := func() (*hir.Module, error) {
		m, e := nanz.Parse(string(source), sourceFile)
		if e != nil {
			return nil, e
		}
		if !m.SourceUnitsComplete || len(m.SourceUnits) != 1 || m.SourceUnits[sourceFile] != sourceSHA {
			return nil, fmt.Errorf("memory host search CLI: imports/incomplete source unsupported")
		}
		m.Target = hir.TargetCPM
		return m, nil
	}
	// Check the owned source before collecting any native association.
	if _, e = factory(); e != nil {
		return e
	}
	var w hir.MemoryHostWorkload
	if e = decodeMemoryHostCLIWorkload(memoryHostCLI.Workload, &w); e != nil {
		return e
	}
	input := pipeline.MemoryHostPolicySearchInputs{Source: source, SourceName: sourceFile}
	if memoryHostSearchCLI.LegacyAssociation != "" {
		input.Legacy, e = readHIRProfileBytes(memoryHostSearchCLI.LegacyAssociation, int64(pipeline.MaxMemoryHostPolicyAssociationBytes))
		if e != nil {
			return e
		}
		input.PreserveSpeed, e = readHIRProfileBytes(memoryHostSearchCLI.PreserveAssociation, int64(pipeline.MaxMemoryHostPolicyAssociationBytes))
		if e != nil {
			return e
		}
		if len(input.Legacy) > pipeline.MaxMemoryHostPolicySearchBytes-len(input.PreserveSpeed) {
			return fmt.Errorf("memory host search CLI: combined association size exceeded")
		}
	} else {
		for policy := 0; policy < 2; policy++ {
			a, e := pipeline.RunZ80MemoryHostLoopProfileWithOptions(factory, w, pipeline.MemoryHostBaselineOptions{UnsignedPairComparePolicy: mir2.UnsignedPairComparePolicy(policy)})
			if e != nil {
				return e
			}
			b, e := json.MarshalIndent(a, "", "  ")
			if e != nil {
				return e
			}
			if policy == 0 {
				input.Legacy = b
			} else {
				input.PreserveSpeed = b
			}
		}
	}
	goal := "speed"
	if optSize {
		goal = "size"
	}
	result, e := pipeline.MeasureMemoryHostArenaAppendPolicySearch(factory, w, input, pipeline.MemoryHostPolicySearchOptions{Goal: goal, MaxSites: 2})
	if e != nil {
		return e
	}
	encoded, e := json.MarshalIndent(result, "", "  ")
	if e != nil {
		return e
	}
	selected := result.Selections[int(result.Selected.Policy)].Candidates[result.Selected.Mask]
	targetEvidence := selected.Evidence.Baseline.Target
	decision := struct {
		SourceName        string                                `json:"source_name"`
		SourceSHA256      string                                `json:"source_sha256"`
		Schema            string                                `json:"schema"`
		Goal              string                                `json:"goal"`
		Selected          pipeline.MemoryHostPolicySearchChoice `json:"selected"`
		Choices           hir.MemoryHostArenaAppendChoices      `json:"choices"`
		SearchSHA256      string                                `json:"search_sha256"`
		AssociationSHA256 [2]string                             `json:"association_sha256"`
		BinarySHA256      string                                `json:"binary_sha256"`
		TStates           int64                                 `json:"t_states"`
		Bytes             int                                   `json:"bytes"`
	}{sourceFile, sourceSHA, "minz.memory-host-arena-append-policy-search-cli-decision/v1", goal, result.Selected, selected.Choices, fmt.Sprintf("%x", sha256.Sum256(encoded)), [2]string{fmt.Sprintf("%x", sha256.Sum256(input.Legacy)), fmt.Sprintf("%x", sha256.Sum256(input.PreserveSpeed))}, targetEvidence.BinarySHA256, targetEvidence.TotalTStates, len(targetEvidence.Binary)}
	files := map[string][]byte{"source.nanz": source, "association-legacy-v2.json": input.Legacy, "association-preserve-v2.json": input.PreserveSpeed, "search.json": encoded, "selected.a80": []byte(targetEvidence.Assembly), "selected.bin": targetEvidence.Binary}
	for n, v := range map[string]any{"workload.json": result.Selections[0].Candidates[0].Evidence.Baseline.Workload, "decisions.json": decision} {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			return e
		}
		files[n] = b
	}
	for policy, s := range result.Selections {
		b, e := json.MarshalIndent(s, "", "  ")
		if e != nil {
			return e
		}
		files[fmt.Sprintf("selection-policy%d-v2.json", policy)] = b
		for mask, c := range s.Candidates {
			b, e := json.MarshalIndent(c.Evidence, "", "  ")
			if e != nil {
				return e
			}
			t := c.Evidence.Baseline.Target
			for n, b := range map[string][]byte{"baseline-v2.json": b, "target.a80": []byte(t.Assembly), "target.bin": t.Binary, "final.mir2": []byte(t.FinalMIRDump), "allocation.json": t.AllocationJSON} {
				files[filepath.Join(fmt.Sprintf("policy%d-mask%d", policy, mask), n)] = b
			}
		}
	}
	// Marshal every semantic payload before any filesystem mutation.
	names := memoryHostSearchCLIOutputNames()
	if len(files) != len(names) {
		return fmt.Errorf("memory host search CLI: incomplete output inventory")
	}
	for _, name := range names {
		if _, ok := files[name]; !ok {
			return fmt.Errorf("memory host search CLI: missing output %s", name)
		}
	}
	if e = validateMemoryHostSearchCLI(sourceFile); e != nil {
		return e
	}
	for _, name := range names {
		path := filepath.Join(memoryHostSearchCLI.Directory, name)
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(path, files[name], 0644); e != nil {
			return e
		}
	}
	// IO errors may leave partial files; admission and marshal refusals do not.
	if outputFile != "" {
		if e = os.WriteFile(outputFile, []byte(targetEvidence.Assembly), 0644); e != nil {
			return e
		}
	}
	fmt.Printf("Closed memory-host search %s selected policy%d/mask%d: %dT / %dB; evidence %s\n", goal, result.Selected.Policy, result.Selected.Mask, targetEvidence.TotalTStates, len(targetEvidence.Binary), memoryHostSearchCLI.Directory)
	return nil
}
