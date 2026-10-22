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
var hirProfileIndependentRules bool
var hirProfileInvariantSites bool
var hirProfileSiteRules bool
var hirProfileJointSites bool
var hirProfileWeightedSites bool
var hirProfileForwardCopySites bool
var hirProfileMixedCopyWeightedSites bool
var hirProfileRegionCompareSites bool
var hirProfileOrderedByteFunctions bool
var hirProfileOrderedByteRWFunctions bool
var hirProfileMixedCopyRegionSites bool

func validateHIRProfileAuto(source string) error {
	if hirProfileLayoutFunction != "" || hirProfileLayoutReplay != "" {
		return validateHIRLayoutModes(source)
	}
	if hirProfileCompilerModesReplay != "" {
		return validateHIRCompilerModesReplay(source)
	}
	if hirProfileCompilerModes {
		return validateHIRCompilerModes(source)
	}
	if hirProfileOrderedByteRWFunctions && (!hirProfileJoint || hirProfileOrderedByteFunctions || hirProfileForwardCopySites || hirProfileMixedCopyWeightedSites || hirProfileRegionCompareSites || hirProfileMixedCopyRegionSites || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileIndependentRules) {
		return fmt.Errorf("HIR ordered byte read/write functions: requires --hir-profile-joint and cannot combine with other function, site or independent rule modes")
	}

	if hirProfileOrderedByteFunctions && (!hirProfileJoint || hirProfileOrderedByteRWFunctions || hirProfileForwardCopySites || hirProfileMixedCopyWeightedSites || hirProfileRegionCompareSites || hirProfileMixedCopyRegionSites || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileIndependentRules) {
		return fmt.Errorf("HIR ordered byte functions: requires --hir-profile-joint and cannot combine with other site or independent rule modes")
	}

	if hirProfileMixedCopyRegionSites && (!hirProfileJoint || hirProfileRegionCompareSites || hirProfileForwardCopySites || hirProfileMixedCopyWeightedSites || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileIndependentRules) {
		return fmt.Errorf("HIR mixed copy region sites: requires --hir-profile-joint and cannot combine with other site or independent rule modes")
	}

	if hirProfileRegionCompareSites && (!hirProfileJoint || hirProfileMixedCopyRegionSites || hirProfileForwardCopySites || hirProfileMixedCopyWeightedSites || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileIndependentRules) {
		return fmt.Errorf("HIR region compare sites: requires --hir-profile-joint and cannot combine with other site or independent rule modes")
	}

	if hirProfileMixedCopyWeightedSites && (!hirProfileJoint || hirProfileForwardCopySites || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileIndependentRules) {
		return fmt.Errorf("HIR mixed copy weighted sites: requires --hir-profile-joint and cannot combine with other site or independent rule modes")
	}

	if hirProfileForwardCopySites && (!hirProfileJoint || hirProfileInvariantSites || hirProfileSiteRules || hirProfileJointSites || hirProfileWeightedSites || hirProfileIndependentRules) {
		return fmt.Errorf("HIR forward copy sites: requires --hir-profile-joint and cannot combine with invariant, site, weighted, or independent rule modes")
	}

	if hirProfileWeightedSites && !hirProfileJointSites {
		return fmt.Errorf("HIR weighted sites: requires --hir-profile-joint-sites")
	}
	if hirProfileJointSites && (!hirProfileInvariantSites || hirProfileSiteRules) {
		return fmt.Errorf("HIR joint sites: requires --hir-profile-invariant-sites and cannot combine with --hir-profile-site-rules")
	}
	if hirProfileSiteRules && !hirProfileInvariantSites {
		return fmt.Errorf("HIR site rules: requires --hir-profile-invariant-sites")
	}
	if hirProfileInvariantSites && (!hirProfileJoint || hirProfileIndependentRules) {
		return fmt.Errorf("HIR invariant sites: requires --hir-profile-joint and cannot combine with --hir-profile-independent-rules")
	}
	if hirProfileIndependentRules && !hirProfileJoint {
		return fmt.Errorf("HIR profile rules: --hir-profile-independent-rules requires --hir-profile-joint")
	}
	if hirProfileAutoDirectory == "" {
		if hirProfileJoint || hirProfileAutoFunction != "" || hirProfileAutoWorkload != "" {
			return fmt.Errorf("HIR profile auto: --hir-profile-auto directory is required")
		}
		return nil
	}
	if err := validateHIRProfileGracePolicy(hirProfileJoint); err != nil {
		return err
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
		for _, input := range []string{source, hirProfileAutoWorkload, checkedLocalTablePath} {
			if input == "" {
				continue
			}
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
		for _, input := range []string{source, hirProfileAutoWorkload, outputFile, checkedLocalTablePath} {
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
	measure := pipeline.MeasureGraceVariantSets
	if hirProfileIndependentRules {
		measure = pipeline.MeasureGraceRuleVariantSets
	}
	var result *pipeline.ProfileVariantSetResult
	var evidence any
	var decisions any
	if hirProfileOrderedByteRWFunctions {
		ordered, err := pipeline.MeasureOrderedByteRWFunctionVariantSets(factory, observations, workload, options, 4)
		if err != nil {
			return nil, err
		}
		result = &pipeline.ProfileVariantSetResult{Steps: ordered.Steps, Binary: ordered.Binary}
		evidence = ordered.Evidence
		best := ordered.Evidence.Candidates[ordered.Evidence.Selected]
		decisions = map[string]any{"schema": "minz.ordered-byte-rw-function-decisions/v1", "compatibility": ordered.Evidence.Compatibility, "function_sites": ordered.Evidence.FunctionSites, "selected": ordered.Evidence.Selected, "requested_functions": best.RequestedSites, "proofs": best.Proofs, "postcompile_proofs": best.FinalProofs, "binary_sha256": best.BinarySHA256, "evidence_sha256": ordered.Evidence.SHA256}
	} else if hirProfileOrderedByteFunctions {
		ordered, err := pipeline.MeasureOrderedByteFunctionVariantSets(factory, observations, workload, options, 4)
		if err != nil {
			return nil, err
		}
		result = &pipeline.ProfileVariantSetResult{Steps: ordered.Steps, Binary: ordered.Binary}
		evidence = ordered.Evidence
		best := ordered.Evidence.Candidates[ordered.Evidence.Selected]
		decisions = map[string]any{"schema": "minz.ordered-byte-function-decisions/v1", "compatibility": ordered.Evidence.Compatibility, "function_sites": ordered.Evidence.FunctionSites, "selected": ordered.Evidence.Selected, "requested_functions": best.RequestedSites, "proofs": best.Proofs, "postcompile_proofs": best.FinalProofs, "binary_sha256": best.BinarySHA256, "evidence_sha256": ordered.Evidence.SHA256}
	} else if hirProfileMixedCopyRegionSites {
		mixed, err := pipeline.MeasureMixedCopyRegionSiteVariantSets(factory, observations, workload, options, 4)
		if err != nil {
			return nil, err
		}
		result = &pipeline.ProfileVariantSetResult{Steps: mixed.Steps, Binary: mixed.Binary}
		evidence = mixed.Evidence
		best := mixed.Evidence.Candidates[mixed.Evidence.Selected]
		decisions = map[string]any{"schema": "minz.mixed-copy-region-site-decisions/v1", "compatibility": mixed.Evidence.Compatibility, "selected": mixed.Evidence.Selected, "requested_choices": best.RequestedSites, "proofs": best.Proofs, "postcompile_proofs": best.FinalProofs, "binary_sha256": best.BinarySHA256, "evidence_sha256": mixed.Evidence.SHA256}
	} else if hirProfileRegionCompareSites {
		region, err := pipeline.MeasureRegionCompareSiteVariantSets(factory, observations, workload, options, 4)
		if err != nil {
			return nil, err
		}
		result = &pipeline.ProfileVariantSetResult{Steps: region.Steps, Binary: region.Binary}
		evidence = region.Evidence
		best := region.Evidence.Candidates[region.Evidence.Selected]
		decisions = map[string]any{"schema": "minz.region-compare-site-decisions/v1", "compatibility": region.Evidence.Compatibility, "selected": region.Evidence.Selected, "requested_region_compare_sites": best.RequestedSites, "proofs": best.Proofs, "postcompile_proofs": best.FinalProofs, "binary_sha256": best.BinarySHA256, "evidence_sha256": region.Evidence.SHA256}
	} else if hirProfileMixedCopyWeightedSites {
		mixed, err := pipeline.MeasureMixedCopyWeightedVariantSets(factory, observations, workload, options, 4)
		if err != nil {
			return nil, err
		}
		result = &pipeline.ProfileVariantSetResult{Steps: mixed.Steps, Binary: mixed.Binary}
		evidence = mixed.Evidence
		best := mixed.Evidence.Candidates[mixed.Evidence.Selected]
		decisions = map[string]any{"schema": "minz.mixed-copy-weighted-site-decisions/v1", "compatibility": mixed.Evidence.Compatibility, "selected": mixed.Evidence.Selected, "requested_choices": best.RequestedSites, "proofs": best.Proofs, "postcompile_proofs": best.FinalProofs, "binary_sha256": best.BinarySHA256, "evidence_sha256": mixed.Evidence.SHA256}
	} else if hirProfileForwardCopySites {
		copyResult, copyErr := pipeline.MeasureForwardCopySiteVariantSets(factory, observations, workload, options, 4)
		if copyErr != nil {
			return nil, copyErr
		}
		result = &pipeline.ProfileVariantSetResult{Steps: copyResult.Steps, Binary: copyResult.Binary}
		evidence = copyResult.Evidence
		best := copyResult.Evidence.Candidates[copyResult.Evidence.Selected]
		decisions = map[string]any{"schema": "minz.forward-copy-site-decisions/v1", "compatibility": copyResult.Evidence.Compatibility, "selected": copyResult.Evidence.Selected, "requested_copy_sites": best.RequestedSites, "proofs": best.FinalProofs, "binary_sha256": best.BinarySHA256}
	} else if hirProfileInvariantSites {
		measureSites := pipeline.MeasureInvariantSiteVariantSets
		if hirProfileSiteRules {
			measureSites = pipeline.MeasureInvariantSiteRuleVariantSets
		}
		if hirProfileJointSites {
			measureSites = pipeline.MeasureJointSourceSiteVariantSets
		}
		if hirProfileWeightedSites {
			measureSites = pipeline.MeasureWeightedJointSourceSiteVariantSets
		}
		sites, siteErr := measureSites(factory, observations, workload, options, 4)
		if siteErr != nil {
			return nil, siteErr
		}
		result = &pipeline.ProfileVariantSetResult{Steps: sites.Steps, Decisions: sites.Decisions, Binary: sites.Binary}
		evidence = sites.Evidence
	} else {
		result, err = measure(factory, observations, workload, options, 4)
		if err != nil {
			return nil, err
		}
		evidence = result.Evidence
	}
	if decisions == nil {
		decisions = result.Decisions
	}
	if err := os.MkdirAll(hirProfileAutoDirectory, 0755); err != nil {
		return nil, err
	}
	for name, value := range map[string]any{"workload.json": workload, "profile.json": run.Profile, "run.json": run, "joint-evidence.json": evidence, "decisions.json": decisions} {
		if err := writeHIRProfileJSON(filepath.Join(hirProfileAutoDirectory, name), value); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(filepath.Join(hirProfileAutoDirectory, "selected.bin"), result.Binary, 0644); err != nil {
		return nil, err
	}
	return result, nil
}
