package main

import (
	"fmt"
	"path/filepath"

	"github.com/minz/minzc/pkg/mir2"
)

var runtimeMode string

func validateCLIRuntime(source string) error {
	if runtimeMode == "" {
		return nil
	}
	if runtimeMode != string(mir2.Z80RuntimeCPM) || target != "cpm" || backend != "z80" || filepath.Ext(source) != ".nanz" {
		return fmt.Errorf("--runtime cpm requires ordinary Nanz, -t cpm and Z80 backend")
	}
	if memoryHostCLI != (memoryHostCLIFlags{}) || memoryHostPolicyCLIRequested() || memoryHostSearchCLIRequested() || hirProfile != (hirProfileFlags{}) || hirProfileAutoDirectory != "" || hirProfileAutoWorkload != "" || hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || pgoProfile != "" || pgoDebug || checkedLocalTablePath != "" || generateCheckedLocalTablePath != "" || emitFormat != "" || gpuRun != "" || listAsserts || outputFormat != "" {
		return fmt.Errorf("--runtime cpm cannot combine with measured/profile or alternate output modes")
	}
	if outputFile != "" && filepath.Ext(outputFile) != ".com" && filepath.Ext(outputFile) != ".a80" {
		return fmt.Errorf("--runtime cpm output must be .com or .a80")
	}
	return nil
}
