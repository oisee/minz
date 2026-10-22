package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
	"reflect"
	"runtime"
)

type MeasuredModeCandidate struct {
	Options          Options                     `json:"options"`
	CompilationID    string                      `json:"compilation_id"`
	Assembly         string                      `json:"assembly"`
	Binary           []byte                      `json:"binary"`
	FullMIRSHA256    string                      `json:"full_mir_sha256,omitempty"`
	AllocationSHA256 string                      `json:"allocation_sha256,omitempty"`
	FinalMIR         string                      `json:"final_mir"`
	Run              *Z80ProfileWorkloadEvidence `json:"run"`
}
type MeasuredModeEvidence struct {
	Schema        string                    `json:"schema"`
	Goal          string                    `json:"goal"`
	Compatibility *hir.ProfileCompatibility `json:"compatibility"`
	FunctionSites *hir.FunctionSiteMap      `json:"function_sites"`
	Oracle        *hir.ProfileRun           `json:"oracle"`
	Candidates    []MeasuredModeCandidate   `json:"candidates"`
	Selected      int                       `json:"selected"`
	SHA256        string                    `json:"sha256,omitempty"`
}
type MeasuredModeResult struct {
	Steps    Steps
	Binary   []byte
	Evidence *MeasuredModeEvidence
}

// MeasureCompilerModeVariants compares the four production ContractOpt/MIR
// Grace combinations, with all HIR transformations disabled. Every candidate
// must pass the controlled native oracle; any failure cancels the entire search.
// Speed and size are strict separate objectives, with earlier candidates winning ties.
func MeasureCompilerModeVariants(factory func() (*hir.Module, error), workload hir.ProfileWorkload, goal string, optSize bool) (*MeasuredModeResult, error) {
	fail := func(e error) (*MeasuredModeResult, error) { return nil, fmt.Errorf("measured compiler modes: %w", e) }
	if factory == nil || (goal != "speed" && goal != "size") {
		return fail(fmt.Errorf("invalid factory/goal"))
	}
	data, e := json.Marshal(workload)
	if e != nil {
		return fail(e)
	}
	var owned hir.ProfileWorkload
	if e = json.Unmarshal(data, &owned); e != nil {
		return fail(e)
	}
	workload = owned
	seen := map[uintptr]any{}
	retained := []*hir.Module{}
	defer func() { runtime.KeepAlive(retained) }()
	var key *hir.ProfileCompatibility
	var sites *hir.FunctionSiteMap
	clean := func(m *hir.Module) bool {
		return m != nil && m.OrderedByteRWPolicy == nil && m.OrderedBytePolicy == nil && m.RegionComparePolicy == nil && m.ForwardCopyPolicy == nil && m.PointerWalkPolicy == nil && m.GraceRulePolicy == nil && m.InvariantBasePolicy == nil && m.LoopLineage == nil
	}
	identity := func(m *hir.Module) error {
		if !clean(m) {
			return fmt.Errorf("factory must return unchanged HIR")
		}
		ctx, e := hir.BuildWorkloadProfileContext(m, workload)
		if e != nil {
			return e
		}
		k, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return e
		}
		s, e := hir.BuildFunctionSiteMap(m)
		if e != nil {
			return e
		}
		if key == nil {
			key = k
			sites = s
		} else if !jointEqual(key, k) || !jointEqual(sites, s) {
			return fmt.Errorf("source/HIR/ABI/context changed")
		}
		return nil
	}
	fresh := func() (*hir.Module, error) {
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if e = identity(m); e != nil {
			return nil, e
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		retained = append(retained, m)
		return m, nil
	}
	check := func() error {
		for _, m := range retained {
			if e := identity(m); e != nil {
				return e
			}
		}
		return nil
	}
	source, e := fresh()
	if e != nil {
		return fail(e)
	}
	modules := make([]*hir.Module, 4)
	for i := range modules {
		modules[i], e = fresh()
		if e != nil {
			return fail(e)
		}
	}
	if e = check(); e != nil {
		return fail(e)
	}
	oracle, e := hir.RunLoopProfile(source, workload)
	if e != nil {
		return fail(e)
	}
	compiled := []Steps{}
	allocations := [][]byte{}
	postHIR := []*hir.ProfileCompatibility{}
	fullMIR := [][]byte{}
	evidence := &MeasuredModeEvidence{Schema: "minz.measured-compiler-modes/v1", Goal: goal, Compatibility: key, FunctionSites: sites, Oracle: oracle}
	result := &MeasuredModeResult{Evidence: evidence}
	// Compiled copies may change during legitimate preparation; unchanged source
	// remains retained and rechecked after every compilation and fresh sentinel.
	retained = retained[:1]
	for i, m := range modules {
		opts := Options{DisableVariantSearch: true, ContractOpt: i&1 != 0, UseGrace: i&2 != 0, OptSize: optSize, AssertMode: "none"}
		id, e := ProfileCompilationID(opts, "default")
		if e != nil {
			return fail(e)
		}
		steps, e := CompileHIRSteps(m, opts)
		if e != nil {
			return fail(e)
		}
		if e = check(); e != nil {
			return fail(e)
		}
		run, e := RunZ80ProfileWorkload(steps, source, workload, *oracle)
		if e != nil {
			return fail(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil {
			return fail(e)
		}
		if len(bin.Errors) > 0 {
			return fail(fmt.Errorf("candidate assembly errors: %v", bin.Errors))
		}
		candidate := MeasuredModeCandidate{Options: opts, CompilationID: id, Assembly: steps.Assembly, Binary: bin.Binary, FinalMIR: steps.MIR2Module.Dump(), Run: run}
		evidence.Candidates = append(evidence.Candidates, candidate)
		allocation, e := json.Marshal(steps.Allocation)
		if e != nil {
			return fail(e)
		}
		compiled = append(compiled, steps)
		allocations = append(allocations, allocation)
		ctx, e := hir.BuildWorkloadProfileContext(m, workload)
		if e != nil {
			return fail(e)
		}
		key, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return fail(e)
		}
		postHIR = append(postHIR, key)
		mirBytes, e := json.Marshal(steps.MIR2Module)
		if e != nil {
			return fail(e)
		}
		fullMIR = append(fullMIR, mirBytes)
		evidence.Candidates[i].FullMIRSHA256 = fmt.Sprintf("%x", sha256.Sum256(mirBytes))
		evidence.Candidates[i].AllocationSHA256 = fmt.Sprintf("%x", sha256.Sum256(allocation))
		better := i == 0
		if i > 0 {
			best := evidence.Candidates[evidence.Selected]
			if goal == "speed" {
				better = run.TotalTStates < best.Run.TotalTStates
			} else {
				better = len(bin.Binary) < len(best.Binary)
			}
		}
		if better {
			evidence.Selected = i
			result.Steps = steps
			result.Binary = append([]byte(nil), bin.Binary...)
		}
		// Check uncompiled future factory outputs too, before allowing preparation.
		for _, next := range modules[i+1:] {
			if e = identity(next); e != nil {
				return fail(e)
			}
		}
	}
	if _, e = fresh(); e != nil {
		return fail(e)
	}
	if e = check(); e != nil {
		return fail(e)
	}
	// Factory callbacks may retain mutable semantic type objects shared with
	// the compiled graph. Revalidate every measured Steps after the sentinel;
	// comparing only the retained input HIR cannot protect the returned winner.
	for i, steps := range compiled {
		c := evidence.Candidates[i]
		ctx, e := hir.BuildWorkloadProfileContext(modules[i], workload)
		if e != nil {
			return fail(e)
		}
		key, e := hir.BuildProfileCompatibility(modules[i], ctx)
		if e != nil {
			return fail(e)
		}
		if !jointEqual(key, postHIR[i]) {
			return fail(fmt.Errorf("postcompile HIR semantics changed after execution"))
		}
		mirBytes, e := json.Marshal(steps.MIR2Module)
		if e != nil {
			return fail(e)
		}
		if !reflect.DeepEqual(mirBytes, fullMIR[i]) {
			return fail(fmt.Errorf("compiled MIR semantic metadata changed after execution"))
		}
		if steps.MIR2Module == nil || steps.MIR2Module.Dump() != c.FinalMIR || steps.Assembly != c.Assembly {
			return fail(fmt.Errorf("measured compiled MIR/ABI/assembly changed after execution"))
		}
		a, e := json.Marshal(steps.Allocation)
		if e != nil {
			return fail(e)
		}
		if !reflect.DeepEqual(a, allocations[i]) {
			return fail(fmt.Errorf("measured allocation changed after execution"))
		}
	}
	b, e := json.Marshal(evidence)
	if e != nil {
		return fail(e)
	}
	evidence.SHA256 = fmt.Sprintf("%x", sha256.Sum256(b))
	return result, nil
}
