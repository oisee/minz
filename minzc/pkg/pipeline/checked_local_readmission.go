package pipeline

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/vir"
)

// ReadmitCheckedLocalTables prepares a current module's certificates from a
// bounded assignment pool, without invoking an external solver. Output tables
// remain exact certificates and must pass the ordinary smaller-BIN selector.
func ReadmitCheckedLocalTables(h *hir.Module, options Options, source *vir.RegAllocTable, limit int) (*vir.RegAllocTable, []string, error) {
	if h == nil || source == nil || !options.OptSize || options.SkipZ80Emission || options.CheckedLocalTablePath != "" || options.CheckedLocalTableSHA256 != "" || options.ProfileSelection != nil || options.Backend != "" && options.Backend != "z80" || limit < 1 || limit > vir.MaxLegacyCandidates {
		return nil, nil, fmt.Errorf("readmission requires ordinary Z80 OptSize, a source pool and bounded limit")
	}
	if source.Size() > limit {
		return nil, nil, fmt.Errorf("readmission candidate limit exceeded")
	}
	steps, err := CompileHIRSteps(h, options)
	if err != nil {
		return nil, nil, err
	}
	target := &vir.RegAllocTable{}
	target.Init()
	var notes []string
	for _, f := range steps.MIR2Module.Funcs {
		p, err := vir.PrepareLocalTableProblem(f, steps.Allocation)
		if err != nil {
			notes = append(notes, f.Name+": skipped: "+err.Error())
			continue
		}
		count, err := source.ReadmitCheckedCandidates(p, target, limit)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		notes = append(notes, fmt.Sprintf("%s: %d admitted proposals", f.Name, count))
	}
	return target, notes, nil
}
