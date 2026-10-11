package pipeline

import (
	"fmt"
	"reflect"
	"runtime"

	"github.com/minz/minzc/pkg/hir"
)

// MemoryHostLoopProfileEvidence associates original-source loop observations
// with an independently executed, internally compiled native baseline. It is
// neither an optimization decision nor a source-to-target loop lineage proof.
type MemoryHostLoopProfileEvidence struct {
	Schema   string                        `json:"schema"`
	Baseline *MemoryHostBaselineEvidence   `json:"baseline"`
	Profile  *hir.MemoryHostLoopProfileRun `json:"profile"`
}

// RunZ80MemoryHostLoopProfile owns inputs before any factory code. Counting
// hooks are confined to a separate VM run; native costs exclude instrumentation.
func RunZ80MemoryHostLoopProfile(factory func() (*hir.Module, error), workload hir.MemoryHostWorkload) (*MemoryHostLoopProfileEvidence, error) {
	fail := func(err error) (*MemoryHostLoopProfileEvidence, error) {
		return nil, fmt.Errorf("memory host loop association: %w", err)
	}
	if factory == nil {
		return fail(fmt.Errorf("missing factory"))
	}
	owned, err := ownMemoryHostWorkload(workload)
	if err != nil {
		return fail(err)
	}
	seen := map[uintptr]bool{}
	retained := []*hir.Module{}
	fresh := func() (*hir.Module, error) {
		m, err := factory()
		if err != nil {
			return nil, err
		}
		if m == nil {
			return nil, fmt.Errorf("nil input HIR")
		}
		if err = jointCheckFreshHIR(m, seen); err != nil {
			return nil, err
		}
		retained = append(retained, m)
		return m, nil
	}
	baseline, err := RunZ80MemoryHostBaseline(fresh, owned)
	if err != nil {
		return fail(err)
	}
	// The baseline owns three factory calls: original raw source, a private
	// compilation candidate, and a fresh sentinel. Only the candidate is allowed
	// to change during compiler preparation. Keep every other source for later
	// identity rechecks when the outer factory is invoked again.
	if len(retained) != 3 {
		return fail(fmt.Errorf("unexpected baseline factory lifecycle"))
	}
	source, err := fresh()
	if err != nil {
		return fail(err)
	}
	identity := func(m *hir.Module) error {
		_, ctx, err := hir.PrepareMemoryHostWorkload(m, owned)
		if err != nil {
			return err
		}
		key, err := hir.BuildProfileCompatibility(m, ctx)
		if err != nil {
			return err
		}
		sites, err := hir.BuildFunctionSiteMap(m)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(key, baseline.Compatibility) || !reflect.DeepEqual(sites, baseline.FunctionSites) {
			return fmt.Errorf("factory changed baseline source or function origins")
		}
		return nil
	}
	checkRetained := func() error {
		for i, m := range retained {
			if i == 1 { // Privately compiled candidate, captured by baseline.
				continue
			}
			if err := identity(m); err != nil {
				return err
			}
		}
		return nil
	}
	if err = checkRetained(); err != nil {
		return fail(err)
	}
	if err = identity(source); err != nil {
		return fail(err)
	}
	if err = checkRetained(); err != nil {
		return fail(err)
	}
	loops, err := hir.BuildLoopSiteMap(source)
	if err != nil {
		return fail(err)
	}
	profile, profileErr := hir.RunMemoryHostLoopProfile(source, owned)
	if profile == nil {
		return fail(profileErr)
	}
	if profileErr != nil {
		if len(profile.Invocations) == 0 || profile.Invocations[len(profile.Invocations)-1].Observation.Failure == nil {
			return fail(profileErr)
		}
	}
	if err = identity(source); err != nil {
		return fail(err)
	}
	if !reflect.DeepEqual(loops, profile.Sites) {
		return fail(fmt.Errorf("profile changed input loop origins"))
	}
	// Instrumentation has its own semantics key and gas accounting. Match it
	// against this exact source, rather than replacing its key with the baseline.
	profileKey, err := hir.BuildProfileCompatibility(source, profile.Context)
	if err != nil {
		return fail(err)
	}
	if !reflect.DeepEqual(profileKey, profile.Compatibility) {
		return fail(fmt.Errorf("profile compatibility mismatch"))
	}
	if len(profile.Invocations) != len(baseline.Raw.Invocations) {
		return fail(fmt.Errorf("instrumented invocation boundary differs"))
	}
	for i, invocation := range profile.Invocations {
		actual, expected := invocation.Observation, baseline.Raw.Invocations[i]
		actual.Steps, expected.Steps = 0, 0
		if !reflect.DeepEqual(actual, expected) || invocation.Completed != (expected.Error == "") {
			return fail(fmt.Errorf("instrumentation changed observation at call %d", i))
		}
	}
	sentinel, err := fresh()
	if err != nil {
		return fail(err)
	}
	if err = identity(sentinel); err != nil {
		return fail(err)
	}
	if err = checkRetained(); err != nil {
		return fail(err)
	}
	if err = identity(source); err != nil {
		return fail(err)
	}
	finalLoops, err := hir.BuildLoopSiteMap(source)
	if err != nil {
		return fail(err)
	}
	if !reflect.DeepEqual(loops, finalLoops) {
		return fail(fmt.Errorf("factory mutated retained loop origins"))
	}
	runtime.KeepAlive(retained)
	return &MemoryHostLoopProfileEvidence{Schema: "minz.memory-host-loop-baseline-association/v1", Baseline: baseline, Profile: profile}, nil
}
