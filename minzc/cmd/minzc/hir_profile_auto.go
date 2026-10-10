package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
)

var hirProfileAutoDirectory, hirProfileAutoFunction, hirProfileAutoWorkload string
var hirProfileJoint bool

func validateHIRProfileAuto(source string) error {
	if hirProfileAutoDirectory == "" {
		if hirProfileJoint || hirProfileAutoFunction != "" || hirProfileAutoWorkload != "" {
			return fmt.Errorf("HIR profile auto: --hir-profile-auto directory is required")
		}
		return nil
	}
	if hirProfile != (hirProfileFlags{}) {
		return fmt.Errorf("HIR profile auto: separate collection/application flags cannot be combined")
	}
	if (!hirProfileJoint && hirProfileAutoFunction == "") || hirProfileAutoWorkload == "" {
		return fmt.Errorf("HIR profile auto: --hir-profile-function and --hir-profile-auto-workload are required")
	}
	if hirProfileJoint && hirProfileAutoFunction != "" {
		return fmt.Errorf("HIR profile joint: function restriction is incompatible with joint search")
	}
	if filepath.Ext(source) != ".nanz" || backend != "z80" || emitFormat != "" || gpuRun != "" || listAsserts || pgoProfile != "" || pgoDebug {
		return fmt.Errorf("HIR profile auto: requires Nanz source and default Z80 output")
	}
	if disableOptimize || disableIROpt || disableReroll || disableAsmOpt || disableCodegenOpt || disableSMC {
		return fmt.Errorf("HIR profile auto: optimization-disable flags are unsupported")
	}
	if outputFile != "" {
		out, err := filepath.Abs(outputFile)
		if err != nil {
			return err
		}
		for _, input := range []string{source, hirProfileAutoWorkload} {
			path, err := filepath.Abs(input)
			if err != nil {
				return err
			}
			a, ae := os.Stat(out)
			b, be := os.Stat(path)
			if out == path || (ae == nil && be == nil && os.SameFile(a, b)) {
				return fmt.Errorf("HIR profile auto: output must differ from inputs")
			}
		}
	}
	// Never let an artifact overwrite its own source, workload or compiled product.
	for _, name := range []string{"workload.json", "profile.json", "run.json", "model.json", "evidence.json", "decisions.json", "input.hir", "raw.mir", "optimized.mir", "selected.a80", "joint-evidence.json", "selected.bin"} {
		dest, _ := filepath.Abs(filepath.Join(hirProfileAutoDirectory, name))
		for _, input := range []string{source, hirProfileAutoWorkload, outputFile} {
			if input == "" {
				continue
			}
			abs, _ := filepath.Abs(input)
			a, ae := os.Stat(dest)
			b, be := os.Stat(abs)
			if dest == abs || (ae == nil && be == nil && os.SameFile(a, b)) {
				return fmt.Errorf("HIR profile auto: artifact paths must differ from inputs/output")
			}
		}
	}
	return nil
}

func prepareHIRProfileAuto(m *hir.Module, factory func() (*hir.Module, error), options pipeline.Options) (*pipeline.ProfileSelection, error) {
	var workload hir.ProfileWorkload
	if err := decodeHIRProfileJSON(hirProfileAutoWorkload, hirWorkloadJSONLimit, &workload); err != nil {
		return nil, err
	}
	run, err := hir.RunLoopProfile(m, workload)
	if err != nil {
		return nil, fmt.Errorf("HIR profile auto collection: %w", err)
	}
	model, evidence, err := pipeline.MeasureLoopCostModel(factory, hirProfileAutoFunction, workload, options)
	if err != nil {
		return nil, err
	}
	observations, err := json.Marshal(run.Profile)
	if err != nil {
		return nil, err
	}
	// Verify the measured model against the actual input before writing artifacts.
	context, err := hir.BuildWorkloadProfileContext(m, workload)
	if err != nil {
		return nil, err
	}
	if model.HIRSHA256 != run.Profile.Compatibility.HIRSHA256 || model.CompilerID != context.CompilerID || model.TargetID != context.TargetID {
		return nil, fmt.Errorf("HIR profile auto: measured input/context changed")
	}
	if err := os.MkdirAll(hirProfileAutoDirectory, 0755); err != nil {
		return nil, err
	}
	for name, value := range map[string]any{"workload.json": workload, "profile.json": run.Profile, "run.json": run, "model.json": model, "evidence.json": evidence} {
		if err := writeHIRProfileJSON(filepath.Join(hirProfileAutoDirectory, name), value); err != nil {
			return nil, err
		}
	}
	return &pipeline.ProfileSelection{Observations: observations, Workload: workload, Model: model, AssemblerMode: "default"}, nil
}

func prepareHIRProfileJoint(m *hir.Module, factory func() (*hir.Module, error), options pipeline.Options) (*pipeline.ProfileVariantSetResult, error) {
	var workload hir.ProfileWorkload
	if err := decodeHIRProfileJSON(hirProfileAutoWorkload, hirWorkloadJSONLimit, &workload); err != nil {
		return nil, err
	}
	run, err := hir.RunLoopProfile(m, workload)
	if err != nil {
		return nil, err
	}
	observations, err := json.Marshal(run.Profile)
	if err != nil {
		return nil, err
	}
	result, err := pipeline.MeasureGraceVariantSets(factory, observations, workload, options, 4)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(hirProfileAutoDirectory, 0755); err != nil {
		return nil, err
	}
	for name, value := range map[string]any{"workload.json": workload, "profile.json": run.Profile, "run.json": run, "joint-evidence.json": result.Evidence, "decisions.json": result.Decisions} {
		if err := writeHIRProfileJSON(filepath.Join(hirProfileAutoDirectory, name), value); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"), result.Binary, 0644); err != nil {
		return nil, err
	}
	return result, nil
}
