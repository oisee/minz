package pipeline

import (
	"fmt"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/vir"
)

// GenerateCheckedLocalTables compiles the ordinary post-contract size pipeline,
// then solves eligible leaf problems offline. Rejected problems are reported and
// never saved as certificates. The caller owns the destination path/save step.
func GenerateCheckedLocalTables(h *hir.Module, options Options, solver vir.SolverOptions) (*vir.RegAllocTable, []string, error) {
	if h == nil || !options.OptSize || options.SkipZ80Emission || options.CheckedLocalTablePath != "" || options.ProfileSelection != nil || options.Backend != "" && options.Backend != "z80" {
		return nil, nil, fmt.Errorf("generation requires ordinary Z80 OptSize without profile/table")
	}
	options.SkipZ80Emission = false
	steps, err := CompileHIRSteps(h, options)
	if err != nil {
		return nil, nil, err
	}
	table := &vir.RegAllocTable{}
	table.Init()
	var notes []string
	for _, f := range steps.MIR2Module.Funcs {
		p, err := vir.PrepareLocalTableProblem(f, steps.Allocation)
		if err != nil {
			notes = append(notes, f.Name+": skipped: "+err.Error())
			continue
		}
		if err := p.Generate(table, solver); err != nil {
			notes = append(notes, f.Name+": rejected: "+err.Error())
			continue
		}
		notes = append(notes, f.Name+": admitted")
	}
	return table, notes, nil
}
