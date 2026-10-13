package pipeline

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/vir"
)

// ComposeCheckedLocalTables prepares exact current certificates from fragments
// without hand-written cuts or an external solver. Compilation and the ordinary
// smaller-BIN selector remain separate, making the candidate table reviewable.
func ComposeCheckedLocalTables(h *hir.Module, options Options, source *vir.RegAllocTable, limit int) (*vir.RegAllocTable, []string, error) {
	if h == nil || source == nil || !options.OptSize || options.SkipZ80Emission || options.CheckedLocalTablePath != "" || options.CheckedLocalTableSHA256 != "" || options.ProfileSelection != nil || options.Backend != "" && options.Backend != "z80" || limit < 1 || limit > vir.MaxLegacyCandidates {
		return nil, nil, fmt.Errorf("composition requires ordinary Z80 OptSize, a source pool and bounded limit")
	}
	if source.Size() > limit {
		return nil, nil, fmt.Errorf("composition candidate limit exceeded")
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
		count, err := source.ComposeCheckedAutomatically(p, target, limit)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		notes = append(notes, fmt.Sprintf("%s: %d admitted compositions", f.Name, count))
	}
	return target, notes, nil
}
