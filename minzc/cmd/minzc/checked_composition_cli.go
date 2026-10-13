package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/vir"
)

func validateCheckedCompositionCLI(source string) error {
	if composeCheckedLocalTablePool == "" && composeCheckedLocalTablePath == "" {
		return nil
	}
	if composeCheckedLocalTablePool == "" || composeCheckedLocalTablePath == "" {
		return fmt.Errorf("checked composition requires --checked-local-fragment-pool and --compose-checked-local-table")
	}
	if !optSize || backend != "z80" || filepath.Ext(source) != ".nanz" || composeCheckedLocalTableLimit < 1 || composeCheckedLocalTableLimit > vir.MaxLegacyCandidates {
		return fmt.Errorf("checked composition requires Nanz, Z80, --Osize and limit 1..1024")
	}
	if outputFile != "" || outputFormat != "" || emitFormat != "" || gpuRun != "" || listAsserts || generateCheckedLocalTablePath != "" || checkedLocalTablePath != "" || checkedLocalTableSHA256 != "" || hirProfile != (hirProfileFlags{}) || hirProfileAutoDirectory != "" || hirProfileCompilerModes || hirProfileCompilerModesReplay != "" || pgoProfile != "" || pgoDebug || runtimeMode != "" || memoryHostCLI != (memoryHostCLIFlags{}) || memoryHostPolicyCLIRequested() || memoryHostSearchCLIRequested() {
		return fmt.Errorf("checked composition cannot combine with compilation output, table generation/replay, runtime or measured profile modes")
	}
	paths := []string{source, composeCheckedLocalTablePool, composeCheckedLocalTablePath}
	for i, path := range paths {
		absolute, err := canonicalMemoryHostCLIPath(path)
		if err != nil {
			return err
		}
		for _, other := range paths[:i] {
			previous, err := canonicalMemoryHostCLIPath(other)
			if err != nil {
				return err
			}
			a, ae := os.Stat(path)
			b, be := os.Stat(other)
			if absolute == previous || ae == nil && be == nil && os.SameFile(a, b) {
				return fmt.Errorf("checked composition input/output paths alias")
			}
		}
	}
	return nil
}

func runCheckedCompositionCLI(h *hir.Module, opts pipeline.Options) error {
	var pool vir.RegAllocTable
	pool.Init()
	if err := pool.Load(composeCheckedLocalTablePool); err != nil {
		return fmt.Errorf("checked fragment pool: %w", err)
	}
	table, notes, err := pipeline.ComposeCheckedLocalTables(h, opts, &pool, composeCheckedLocalTableLimit)
	if err != nil {
		return err
	}
	for _, note := range notes {
		fmt.Fprintln(os.Stderr, "[checked-composition] "+note)
	}
	if table.Size() == 0 {
		return fmt.Errorf("no checked compositions admitted")
	}
	return table.Save(composeCheckedLocalTablePath)
}
