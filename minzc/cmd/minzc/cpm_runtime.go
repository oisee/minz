package main

import (
	"fmt"
	"path/filepath"

	"github.com/minz/minzc/pkg/mir2"
)

var runtimeMode string
var runtimeStackBytes int

func validateCLIRuntime(source string) error {
	if e := mir2.ValidateCPMRuntimeStack(mir2.Z80Runtime(runtimeMode), runtimeStackBytes); e != nil {
		return e
	}
	if runtimeMode == "" {
		return nil
	}
	if runtimeMode != string(mir2.Z80RuntimeCPM) || target != "cpm" || backend != "z80" || (filepath.Ext(source) != ".nanz" && filepath.Ext(source) != ".pas" && filepath.Ext(source) != ".abap") {
		return fmt.Errorf("--runtime cpm requires ordinary Nanz, Pascal or ABAP, -t cpm and Z80 backend")
	}
	if memoryHostCLI != (memoryHostCLIFlags{}) || memoryHostPolicyCLIRequested() || memoryHostSearchCLIRequested() || hirProfile != (hirProfileFlags{}) || hirProfileAutoDirectory != "" || hirProfileAutoWorkload != "" || hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || pgoProfile != "" || pgoDebug || checkedLocalTablePath != "" || checkedLocalTableSHA256 != "" || generateCheckedLocalTablePath != "" || emitFormat != "" || gpuRun != "" || listAsserts || outputFormat != "" {
		return fmt.Errorf("--runtime cpm cannot combine with measured/profile or alternate output modes")
	}
	if outputFile != "" && filepath.Ext(outputFile) != ".com" && filepath.Ext(outputFile) != ".a80" {
		return fmt.Errorf("--runtime cpm output must be .com or .a80")
	}
	return nil
}
