package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/pipeline"
	"path/filepath"
)

func validateContractStrategyCLI(source string) error {
	if err := pipeline.ValidateContractStrategy(pipeline.Options{ContractOpt: true, ContractStrategy: contractStrategy, Backend: backend}); err != nil {
		return err
	}
	if contractStrategy == "" {
		return nil
	}
	if memoryHostSearchCLIRequested() || memoryHostPolicyCLIRequested() || memoryHostCLI != (memoryHostCLIFlags{}) || hirProfileAutoDirectory != "" || hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || hirProfileJoint || hirProfile != (hirProfileFlags{}) || pgoProfile != "" || pgoDebug {
		return fmt.Errorf("explicit contract strategy cannot be combined with profile search, collection or replay")
	}
	switch filepath.Ext(source) {
	case ".nanz", ".minz", ".hir", ".lanz", ".lizp", ".pas", ".c", ".m", ".abap", ".frl", ".plm":
		return nil
	}
	return fmt.Errorf("contract strategy requires the HIR pipeline")
}
