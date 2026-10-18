package main

import (
	"path/filepath"

	"github.com/minz/minzc/pkg/hir"
)

// Select ordinary application entry only at the CLI native ABAP boundary.
// Library compilations (including ABAP's default CPM target) keep their existing
// bare/called entry contracts. Measured and assertion workloads retain theirs.
func ordinaryABAPCPMEntry(source string, hm *hir.Module, skipZ80 bool, assertions string) bool {
	if filepath.Ext(source) != ".abap" || target != "cpm" || (backend != "" && backend != "z80") || runtimeMode != "" || skipZ80 || emitFormat != "" || gpuRun != "" || listAsserts || (outputFormat != "" && outputFormat != "com") {
		return false
	}
	if assertions != "none" && (len(hm.Asserts) > 0 || len(hm.Sandboxes) > 0) {
		return false
	}
	if memoryHostCLI != (memoryHostCLIFlags{}) || memoryHostPolicyCLIRequested() || memoryHostSearchCLIRequested() || hirProfile != (hirProfileFlags{}) || hirProfileAutoDirectory != "" || hirProfileAutoWorkload != "" || hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || pgoProfile != "" || pgoDebug || generateCheckedLocalTablePath != "" || composeCheckedLocalTablePath != "" {
		return false
	}
	return true
}
