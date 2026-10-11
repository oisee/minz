package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"reflect"
	"runtime"
)

// MemoryHostBaselineEvidence binds the owned workload, original source identity,
// verified raw VM observations and privately compiled target. It is a measured
// baseline, not a rewrite certificate or a caller-supplied oracle attestation.
type MemoryHostBaselineEvidence struct {
	Schema        string                    `json:"schema"`
	Workload      hir.MemoryHostWorkload    `json:"workload"`
	Compatibility *hir.ProfileCompatibility `json:"compatibility"`
	FunctionSites *hir.FunctionSiteMap      `json:"function_sites"`
	Raw           *hir.MemoryHostRun        `json:"raw"`
	Target        *MemoryHostTargetEvidence `json:"target"`
	OptimizedMIR  string                    `json:"optimized_mir_diagnostic"`
}

// RunZ80MemoryHostBaseline owns the workload before calling the factory. Every
// returned HIR must be independently allocated and have identical complete
// source and origin identity. The assembly and raw oracle are created privately.
// Failed-call evidence is limited to matching typed denials from the closed host
// builtins. Direct native memory permission traps and execution limits refuse
// attestation as infrastructure errors.
func RunZ80MemoryHostBaseline(factory func() (*hir.Module, error), workload hir.MemoryHostWorkload) (*MemoryHostBaselineEvidence, error) {
	return runZ80MemoryHostBaseline(factory, workload, MemoryHostBaselineOptions{})
}

// RunZ80MemoryHostBaselineWithOptions owns and validates the typed compiler
// choices before invoking the source factory. The published v1 wrapper remains
// Legacy; this v2 entry makes the measured backend policy explicit.
func RunZ80MemoryHostBaselineWithOptions(factory func() (*hir.Module, error), workload hir.MemoryHostWorkload, options MemoryHostBaselineOptions) (*MemoryHostBaselineEvidenceV2, error) {
	owned := options
	if _, err := resolveUnsignedPairComparePolicy(Options{UnsignedPairComparePolicy: owned.UnsignedPairComparePolicy}); err != nil {
		return nil, err
	}
	baseline, err := runZ80MemoryHostBaseline(factory, workload, owned)
	if err != nil {
		return nil, err
	}
	return &MemoryHostBaselineEvidenceV2{Schema: "minz.memory-host-baseline/v2", CompileOptions: owned, Baseline: baseline}, nil
}

func runZ80MemoryHostBaseline(factory func() (*hir.Module, error), workload hir.MemoryHostWorkload, compileOptions MemoryHostBaselineOptions) (*MemoryHostBaselineEvidence, error) {
	fail := func(e error) (*MemoryHostBaselineEvidence, error) {
		return nil, fmt.Errorf("memory host baseline: %w", e)
	}
	if factory == nil {
		return fail(fmt.Errorf("missing factory"))
	}
	// Serialization ownership precedes all user factory code.
	owned, e := ownMemoryHostWorkload(workload)
	if e != nil {
		return fail(e)
	}
	seen := map[uintptr]bool{}
	retained := []*hir.Module{}
	var compatibility *hir.ProfileCompatibility
	var sites *hir.FunctionSiteMap
	identity := func(m *hir.Module) (*hir.ProfileCompatibility, *hir.FunctionSiteMap, error) {
		_, ctx, e := hir.PrepareMemoryHostWorkload(m, owned)
		if e != nil {
			return nil, nil, e
		}
		key, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return nil, nil, e
		}
		origins, e := hir.BuildFunctionSiteMap(m)
		return key, origins, e
	}
	fresh := func() (*hir.Module, error) {
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if m == nil {
			return nil, fmt.Errorf("nil input HIR")
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		key, origins, e := identity(m)
		if e != nil {
			return nil, e
		}
		if compatibility == nil {
			compatibility = key
			sites = origins
		} else if !reflect.DeepEqual(key, compatibility) || !reflect.DeepEqual(origins, sites) {
			return nil, fmt.Errorf("factory changed source or function/path origin identity")
		}
		retained = append(retained, m)
		return m, nil
	}
	checkRetained := func() error {
		for _, m := range retained {
			key, origins, e := identity(m)
			if e != nil {
				return e
			}
			if !reflect.DeepEqual(key, compatibility) || !reflect.DeepEqual(origins, sites) {
				return fmt.Errorf("factory or compilation mutated retained source/origins")
			}
		}
		return nil
	}
	source, e := fresh()
	if e != nil {
		return fail(e)
	}
	raw, e := hir.RunMemoryHostBaseline(source, owned)
	if raw == nil {
		return fail(e)
	}
	// A failed raw invocation is still a bounded execution prefix to replay.
	candidate, e := fresh()
	if e != nil {
		return fail(e)
	}
	if e = checkRetained(); e != nil {
		return fail(e)
	}
	// Compile the candidate only after capturing its immutable input identity.
	// The source remains retained and must stay unchanged through code generation.
	steps, e := CompileHIRSteps(candidate, Options{AssertMode: "none", UnsignedPairComparePolicy: compileOptions.UnsignedPairComparePolicy})
	if e != nil {
		return fail(e)
	}
	// Candidate HIR may be structurally rewritten by normal compiler preparation;
	// source factory outputs kept for immutable review exclude that private copy.
	retained = retained[:len(retained)-1]
	if e = checkRetained(); e != nil {
		return fail(e)
	}
	target, e := runZ80MemoryHostWorkload(steps, source, owned, *raw)
	if e != nil {
		return fail(e)
	}
	if e = checkRetained(); e != nil {
		return fail(e)
	}
	// A final fresh sentinel detects delayed mutation of earlier inputs. The
	// synchronously compiled private candidate is excluded from source rechecks.
	if _, e = fresh(); e != nil {
		return fail(e)
	}
	if e = checkRetained(); e != nil {
		return fail(e)
	}
	runtime.KeepAlive(candidate)
	runtime.KeepAlive(retained)
	return &MemoryHostBaselineEvidence{Schema: "minz.memory-host-baseline/v1", Workload: owned, Compatibility: compatibility, FunctionSites: sites, Raw: raw, Target: target, OptimizedMIR: steps.MIR2Opt}, nil
}

func ownMemoryHostWorkload(w hir.MemoryHostWorkload) (hir.MemoryHostWorkload, error) {
	var owned hir.MemoryHostWorkload
	b, e := json.Marshal(w)
	if e == nil {
		e = json.Unmarshal(b, &owned)
	}
	return owned, e
}
