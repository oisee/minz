package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/vir"
	"strings"
)

// ProfileSelection supplies observations, workload and independently measured
// models. Model and Models are mutually exclusive. Context is recomputed from
// current executable, HIR and workload.
type ProfileSelection struct {
	Observations  []byte
	Workload      hir.ProfileWorkload
	Models        []hir.LoopCostModel
	Model         hir.LoopCostModel
	AssemblerMode string
}

// ProfileCompilationID keys pipeline options and assembler mode. Trace sinks and
// the entire ProfileSelection request are excluded. Only measured default JRS
// policy is supported.
func ProfileCompilationID(options Options, mode string) (string, error) {
	if err := ValidateContractStrategy(options); err != nil {
		return "", err
	}
	if options.Runtime != "" || options.RuntimeStackBytes != 0 {
		return "", fmt.Errorf("PGO: application runtimes are unsupported by the measured workload model")
	}
	if options.CheckedLocalTablePath != "" {
		if options.CheckedLocalTableSHA256 == "" {
			return "", fmt.Errorf("PGO: external local tables require a content pin")
		}
		if !options.OptSize || options.SkipZ80Emission || options.Backend != "" && options.Backend != "z80" {
			return "", fmt.Errorf("PGO: checked local tables require emitted Z80 OptSize")
		}
		data, err := readCheckedLocalTableSnapshot(options)
		if err != nil {
			return "", err
		}
		var table vir.RegAllocTable
		table.Init()
		if err := table.LoadJSON(data); err != nil {
			return "", fmt.Errorf("PGO: checked local table: %w", err)
		}
		// Identity binds exact bytes, not a machine-specific filename. Each compile
		// verifies the pin again before decoding its one selection snapshot.
		options.CheckedLocalTablePath = "sha256:" + strings.ToLower(options.CheckedLocalTableSHA256)
		options.CheckedLocalTableSHA256 = strings.ToLower(options.CheckedLocalTableSHA256)
	} else if options.CheckedLocalTableSHA256 != "" {
		return "", fmt.Errorf("PGO: table SHA256 requires a path")
	}
	if mode != "default" {
		return "", fmt.Errorf("PGO: unsupported assembler configuration")
	}
	options.ProfileSelection = nil
	options.GraceStats = nil
	data, err := json.Marshal(struct {
		Options       Options
		AssemblerMode string
	}{options, mode})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}
func applyPipelineProfile(m *hir.Module, options Options) ([]hir.ProfileDecision, error) {
	request := options.ProfileSelection
	config, err := ProfileCompilationID(options, request.AssemblerMode)
	if err != nil {
		return nil, err
	}
	models := request.Models
	if models != nil && request.Model != (hir.LoopCostModel{}) {
		return nil, fmt.Errorf("PGO: Model and Models are mutually exclusive")
	}
	if models == nil {
		models = []hir.LoopCostModel{request.Model}
	}
	for _, model := range models {
		if config != model.CompilationID {
			return nil, fmt.Errorf("PGO: measured compilation configuration mismatch; remeasure with the requested options (legacy models require both Grace passes off; CLI: --no-grace)")
		}
	}
	context, err := hir.BuildWorkloadProfileContext(m, request.Workload)
	if err != nil {
		return nil, err
	}
	var decisions []hir.ProfileDecision
	if request.Models == nil {
		decisions, err = hir.SelectLoopVariants(bytes.NewReader(request.Observations), m, context, request.Model, options.OptSize)
	} else {
		decisions, err = hir.SelectLoopVariantsWithModels(bytes.NewReader(request.Observations), m, context, models, options.OptSize)
	}
	if err != nil {
		return nil, err
	}
	if err := hir.ApplyProfileDecisions(m, decisions); err != nil {
		return nil, err
	}
	return decisions, nil
}
