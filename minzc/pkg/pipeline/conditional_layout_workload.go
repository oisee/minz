package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

// ConditionalLayoutCandidate retains a complete executable and its measured
// ordered workload. A nil order denotes the ordinary production baseline.
type ConditionalLayoutCandidate struct {
	Order    []int                       `json:"order"`
	Assembly string                      `json:"assembly"`
	Binary   []byte                      `json:"binary"`
	Run      *Z80ProfileWorkloadEvidence `json:"run"`
}
type ConditionalLayoutEvidence struct {
	Schema           string                       `json:"schema"`
	Function         string                       `json:"function"`
	Goal             string                       `json:"goal"`
	CompilationID    string                       `json:"compilation_id"`
	Binding          string                       `json:"binding"`
	FullMIRSHA256    string                       `json:"full_mir_sha256"`
	AllocationSHA256 string                       `json:"allocation_sha256"`
	Candidates       []ConditionalLayoutCandidate `json:"candidates"`
	Selected         int                          `json:"selected"`
	SHA256           string                       `json:"sha256,omitempty"`
}
type ConditionalLayoutResult struct {
	Steps    Steps
	Binary   []byte
	Evidence *ConditionalLayoutEvidence
}

// conditionalFunctionSpan includes every return, but excludes following
// functions, global/string storage and runtime helpers. Only canonical codegen
// output is supplied by the fresh compilation below; this is not an ASM importer.
func conditionalFunctionSpan(assembly, name string) (int, int, error) {
	header := "; fun " + name + "("
	start := strings.Index(assembly, header)
	if start < 0 || strings.Count(assembly, header) != 1 {
		return 0, 0, fmt.Errorf("missing or ambiguous function boundary")
	}
	end := len(assembly)
	for _, marker := range []string{"\n; fun ", "\n; globals\n", "\n; strings\n", "\n; runtime:"} {
		if offset := strings.Index(assembly[start+len(header):], marker); offset >= 0 {
			boundary := start + len(header) + offset + 1
			if boundary < end {
				end = boundary
			}
		}
	}
	return start, end, nil
}

// MeasureConditionalLayoutWorkload selects one validated layout in a complete
// production module. Costs include actual calls and caller preservation, not
// isolated classifier histograms. Every candidate must agree with the input HIR
// VM on the ordered workload; any failure cancels the whole search. Pure speed
// ignores bytes, size ignores timing, and ties retain the earlier candidate.
// Heldout workloads must be replayed on the returned exact Steps separately.
func MeasureConditionalLayoutWorkload(factory func() (*hir.Module, error), function string, workload hir.ProfileWorkload, options Options, goal string, limit int) (*ConditionalLayoutResult, error) {
	// Layout enumeration requires the exact requested compiler configuration.
	options.DisableVariantSearch = true
	fail := func(err error) (*ConditionalLayoutResult, error) {
		return nil, fmt.Errorf("conditional layout workload: %w", err)
	}
	if factory == nil || function == "" || (goal != "speed" && goal != "size") || limit < 1 || limit > 4 || options.SkipZ80Emission || options.ProfileSelection != nil || options.Runtime != "" || options.Backend != "" && options.Backend != "z80" {
		return fail(fmt.Errorf("unsupported factory/function/goal/configuration"))
	}
	config, err := ProfileCompilationID(options, "default")
	if err != nil {
		return fail(err)
	}
	payload, err := json.Marshal(workload)
	if err != nil {
		return fail(err)
	}
	var owned hir.ProfileWorkload
	if err = json.Unmarshal(payload, &owned); err != nil {
		return fail(err)
	}
	workload = owned
	seen := map[uintptr]any{}
	retained := []*hir.Module{}
	defer func() { runtime.KeepAlive(retained) }()
	var identity *hir.ProfileCompatibility
	clean := func(m *hir.Module) bool {
		return m != nil && m.OrderedByteRWPolicy == nil && m.OrderedBytePolicy == nil && m.RegionComparePolicy == nil && m.ForwardCopyPolicy == nil && m.PointerWalkPolicy == nil && m.GraceRulePolicy == nil && m.InvariantBasePolicy == nil && m.LoopLineage == nil
	}
	fresh := func() (*hir.Module, error) {
		m, e := factory()
		if e != nil {
			return nil, e
		}
		if !clean(m) {
			return nil, fmt.Errorf("factory must return unchanged HIR without policy/lineage")
		}
		ctx, e := hir.BuildWorkloadProfileContext(m, workload)
		if e != nil {
			return nil, e
		}
		key, e := hir.BuildProfileCompatibility(m, ctx)
		if e != nil {
			return nil, e
		}
		if identity == nil {
			identity = key
		} else if !jointEqual(identity, key) {
			return nil, fmt.Errorf("factory HIR/source/ABI changed")
		}
		if e = jointCheckFreshHIR(m, seen); e != nil {
			return nil, e
		}
		retained = append(retained, m)
		return m, nil
	}
	snapshot := func(m *hir.Module) ([]byte, error) {
		return json.Marshal(struct {
			Module   *hir.Module
			Units    map[string]string
			Complete bool
		}{m, m.SourceUnits, m.SourceUnitsComplete})
	}
	source, err := fresh()
	if err != nil {
		return fail(err)
	}
	sourceBytes, err := snapshot(source)
	if err != nil {
		return fail(err)
	}
	input, err := fresh()
	if err != nil {
		return fail(err)
	}
	checkSource := func() error {
		if !clean(source) {
			return fmt.Errorf("retained source policy/lineage changed")
		}
		current, e := snapshot(source)
		if e != nil {
			return e
		}
		if !bytes.Equal(sourceBytes, current) {
			return fmt.Errorf("retained source changed")
		}
		return nil
	}
	if err = checkSource(); err != nil {
		return fail(err)
	}
	oracle, err := hir.RunLoopProfile(source, workload)
	if err != nil {
		return fail(err)
	}
	steps, err := CompileHIRSteps(input, options)
	if err != nil {
		return fail(err)
	}
	if err = checkSource(); err != nil {
		return fail(err)
	}
	problem, err := vir.PrepareConditionalTableProblem(steps.MIR2Module.FuncByName(function), steps.Allocation)
	if err != nil {
		return fail(err)
	}
	orders, err := problem.Layouts(limit)
	if err != nil {
		return fail(err)
	}
	start, end, err := conditionalFunctionSpan(steps.Assembly, function)
	if err != nil {
		return fail(err)
	}
	assemblies := []string{steps.Assembly}
	for _, order := range orders {
		candidate, e := problem.EmitLayout(order)
		if e != nil {
			return fail(e)
		}
		// Preserve production ABI comments and all following module storage/helpers.
		headerEnd := strings.IndexByte(steps.Assembly[start:end], '\n')
		candidateEnd := strings.IndexByte(candidate, '\n')
		if headerEnd < 0 || candidateEnd < 0 {
			return fail(fmt.Errorf("missing ABI header"))
		}
		candidate = steps.Assembly[start:start+headerEnd+1] + candidate[candidateEnd+1:]
		assemblies = append(assemblies, steps.Assembly[:start]+candidate+steps.Assembly[end:])
	}
	compiledBytes, err := snapshot(input)
	if err != nil {
		return fail(err)
	}
	mirBytes, err := json.Marshal(steps.MIR2Module)
	if err != nil {
		return fail(err)
	}
	allocBytes, err := json.Marshal(steps.Allocation)
	if err != nil {
		return fail(err)
	}
	evidence := &ConditionalLayoutEvidence{Schema: "minz.measured-conditional-layout/v1", Function: function, Goal: goal, CompilationID: config, Binding: problem.Binding, FullMIRSHA256: fmt.Sprintf("%x", sha256.Sum256(mirBytes)), AllocationSHA256: fmt.Sprintf("%x", sha256.Sum256(allocBytes))}
	for i, assembly := range assemblies {
		variant := steps
		variant.Assembly = assembly
		run, e := RunZ80ProfileWorkload(variant, source, workload, *oracle)
		if e != nil {
			return fail(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
		if e != nil {
			return fail(e)
		}
		if len(bin.Errors) > 0 {
			return fail(fmt.Errorf("candidate assembly failed: %v", bin.Errors))
		}
		candidate := ConditionalLayoutCandidate{Assembly: assembly, Binary: bin.Binary, Run: run}
		if i > 0 {
			candidate.Order = append([]int(nil), orders[i-1]...)
		}
		evidence.Candidates = append(evidence.Candidates, candidate)
		best := evidence.Candidates[evidence.Selected]
		if goal == "speed" && run.TotalTStates < best.Run.TotalTStates || goal == "size" && len(bin.Binary) < len(best.Binary) {
			evidence.Selected = i
		}
	}
	if _, err = fresh(); err != nil {
		return fail(err)
	}
	if err = checkSource(); err != nil {
		return fail(err)
	}
	current, err := snapshot(input)
	if err != nil || !bytes.Equal(current, compiledBytes) {
		return fail(fmt.Errorf("compiled HIR changed after measurement"))
	}
	current, err = json.Marshal(steps.MIR2Module)
	if err != nil || !bytes.Equal(current, mirBytes) {
		return fail(fmt.Errorf("compiled MIR changed after measurement"))
	}
	current, err = json.Marshal(steps.Allocation)
	if err != nil || !bytes.Equal(current, allocBytes) {
		return fail(fmt.Errorf("allocation changed after measurement"))
	}
	currentConfig, err := ProfileCompilationID(options, "default")
	if err != nil {
		return fail(err)
	}
	if currentConfig != config {
		return fail(fmt.Errorf("compilation configuration changed after measurement"))
	}
	if err = problem.Validate(); err != nil {
		return fail(err)
	}
	payload, err = json.Marshal(evidence)
	if err != nil {
		return fail(err)
	}
	evidence.SHA256 = fmt.Sprintf("%x", sha256.Sum256(payload))
	chosen := evidence.Candidates[evidence.Selected]
	steps.Assembly = chosen.Assembly
	if evidence.Selected != 0 {
		traces := make(map[string]*FuncTrace, len(steps.Traces))
		for name, trace := range steps.Traces {
			if trace == nil {
				traces[name] = nil
				continue
			}
			copy := *trace
			if name == function {
				copy.Backend = "checked-conditional-layout"
			}
			traces[name] = &copy
		}
		steps.Traces = traces
	}
	return &ConditionalLayoutResult{Steps: steps, Binary: append([]byte(nil), chosen.Binary...), Evidence: evidence}, nil
}
