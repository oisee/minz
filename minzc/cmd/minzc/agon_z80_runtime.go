package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

func validateAgonZ80CLI(source string) error {
	if target != "agon" || backend != "z80" {
		return fmt.Errorf("agon-z80 runtime requires explicit -t agon -b z80")
	}
	if err := mir2.ValidateAgonZ80Stack(runtimeStackBytes); err != nil {
		return err
	}
	if ext := strings.ToLower(filepath.Ext(source)); ext != ".abap" && ext != ".nanz" {
		return fmt.Errorf("agon-z80 runtime currently requires ordinary ABAP or Nanz source")
	}
	if memoryHostCLI != (memoryHostCLIFlags{}) || memoryHostPolicyCLIRequested() || memoryHostSearchCLIRequested() || hirProfile != (hirProfileFlags{}) || hirProfileAutoDirectory != "" || hirProfileAutoWorkload != "" || hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || pgoProfile != "" || pgoDebug || checkedLocalTablePath != "" || checkedLocalTableSHA256 != "" || generateCheckedLocalTablePath != "" || composeCheckedLocalTablePath != "" || emitFormat != "" || gpuRun != "" || listAsserts {
		return fmt.Errorf("agon-z80 runtime cannot combine with measured/profile/table/alternate emission modes")
	}
	if outputFormat != "" && outputFormat != "agon-z80" {
		return fmt.Errorf("agon-z80 runtime requires its mode0 executable format")
	}
	if ext := strings.ToLower(filepath.Ext(outputFile)); ext != "" && ext != ".bin" && ext != ".asm" && ext != ".a80" {
		return fmt.Errorf("agon-z80 runtime output must be .bin or assembly")
	}
	return nil
}
