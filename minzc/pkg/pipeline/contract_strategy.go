package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

// ValidateContractStrategy is shared by compilation and evidence identity.
// Explicit strategies cannot be silently ignored when contracts are disabled.
func ValidateContractStrategy(opts Options) error {
	switch opts.ContractStrategy {
	case "":
		return nil
	case "greedy", "pbqp":
	default:
		return fmt.Errorf("unsupported contract strategy %q", opts.ContractStrategy)
	}
	if !opts.ContractOpt {
		return fmt.Errorf("contract strategy requires ContractOpt")
	}
	if opts.ContractStrategy == "pbqp" && opts.Backend != "" && opts.Backend != "z80" {
		return fmt.Errorf("PBQP contract strategy is verified only for Z80")
	}
	return nil
}
func optimizePipelineContracts(m *mir2.Module, ct mir2.CostTable, strategy string) mir2.ContractSet {
	if strategy == "pbqp" {
		return mir2.OptimizeContractsPBQP(m, ct)
	}
	return mir2.OptimizeContracts(m, ct)
}
