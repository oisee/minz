package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sort"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// ProfileVariantSetResult retains the exact compiled winner; no recompilation is
// needed to emit the binary whose execution was measured.
type ProfileVariantSetResult struct {
	Steps     Steps
	Decisions []hir.ProfileDecision
	Binary    []byte
	Evidence  *ProfileVariantSetEvidence
}

type ProfileVariantSetCandidate struct {
	HIR              string                      `json:"hir"`
	Lineage          []hir.LoopLineageRecord     `json:"lineage"`
	FinalHIR         string                      `json:"final_grace_hir,omitempty"`
	FinalLineage     []hir.LoopLineageRecord     `json:"postcompile_lineage,omitempty"`
	FinalRawRun      *hir.ProfileRun             `json:"postcompile_raw_run,omitempty"`
	RulePolicy       map[string]bool             `json:"rule_policy,omitempty"`
	CommonGraceRules bool                        `json:"common_grace_rules"`
	Decisions        []hir.ProfileDecision       `json:"decisions"`
	Binary           []byte                      `json:"binary"`
	BinarySHA256     string                      `json:"binary_sha256"`
	ModuleBytes      int                         `json:"module_bytes"`
	TotalTStates     int64                       `json:"total_t_states"`
	RawRun           *hir.ProfileRun             `json:"raw_run"`
	Run              *Z80ProfileWorkloadEvidence `json:"run"`
}

// SHA256 is the digest of JSON with that field omitted, binding every candidate,
// the automatic workload compatibility, compilation options and chosen index.
type ProfileVariantSetEvidence struct {
	Schema        string                       `json:"schema"`
	Compatibility *hir.ProfileCompatibility    `json:"compatibility"`
	CompilationID string                       `json:"compilation_id"`
	Options       Options                      `json:"options"`
	AssemblerMode string                       `json:"assembler_mode"`
	MaxSites      int                          `json:"max_sites"`
	EligibleSites []string                     `json:"eligible_sites"`
	Oracle        *hir.ProfileRun              `json:"oracle"`
	Candidates    []ProfileVariantSetCandidate `json:"candidates"`
	Selected      int                          `json:"selected"`
	SHA256        string                       `json:"sha256,omitempty"`
}

// MeasureProfileVariantSets exhaustively measures at most four observed eligible
// sites (16 sets), with common HIR rules enabled in every candidate.
// This is a pointer-only comparison; use MeasureGraceVariantSets for a baseline
// with all HIR Grace rules disabled. Independent linear models are neither required nor consulted.
// Strict profile loading and fresh equivalent factory inputs precede compilation.
// Ordinary ApplyProfileDecisions legality and sticky policy apply to every set.
// Every candidate must agree with the raw oracle and pass actual target execution
// before the best whole-workload speed (or whole-module size) is selected. Strict
// improvement is required: the all-original candidate wins ties with original.
func MeasureProfileVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*ProfileVariantSetResult, error) {
	return measureProfileVariantSets(factory, observations, workload, options, maxSites, 0)
}

// MeasureGraceVariantSets also compares structural constant-if/invariant-base
// rules disabled versus enabled. Every combination is measured; the baseline
// has all HIR Grace rules disabled. With four pointer sites, at most 32 sets.
func MeasureGraceVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*ProfileVariantSetResult, error) {
	return measureProfileVariantSets(factory, observations, workload, options, maxSites, 1)
}

// MeasureGraceRuleVariantSets measures independent module-wide constant-if and
// invariant-base policies crossed with pointer choices. It retains the exact
// measured winner, with all rules disabled first so the baseline wins ties.
// At most four pointer sites produce 64 bounded candidate combinations.
func MeasureGraceRuleVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int) (*ProfileVariantSetResult, error) {
	return measureProfileVariantSets(factory, observations, workload, options, maxSites, 2)
}

func measureProfileVariantSets(factory func() (*hir.Module, error), observations []byte, workload hir.ProfileWorkload, options Options, maxSites int, ruleMode int) (*ProfileVariantSetResult, error) {
	fail := func(err error) (*ProfileVariantSetResult, error) {
		return nil, fmt.Errorf("measure profile variant sets: %w", err)
	}
	if factory == nil || maxSites < 1 || maxSites > 4 || options.ProfileSelection != nil || options.SkipZ80Emission || (options.Backend != "" && options.Backend != "z80") {
		return fail(fmt.Errorf("unsupported factory/backend/configuration or site cap (must be 1..4)"))
	}
	config, err := ProfileCompilationID(options, "default")
	if err != nil {
		return fail(err)
	}
	// Own the request, so evidence hashes the workload actually executed.
	data, err := json.Marshal(workload)
	if err != nil {
		return fail(err)
	}
	var ownedWorkload hir.ProfileWorkload
	if err = json.Unmarshal(data, &ownedWorkload); err != nil {
		return fail(err)
	}
	workload = ownedWorkload
	seen := map[uintptr]any{}
	retained := []*hir.Module{}
	var key *hir.ProfileCompatibility
	fresh := func() (*hir.Module, error) {
		m, err := factory()
		if err != nil {
			return nil, err
		}
		if m == nil || m.OrderedByteRWPolicy != nil || m.OrderedBytePolicy != nil || m.RegionComparePolicy != nil || m.ForwardCopyPolicy != nil || m.PointerWalkPolicy != nil || m.LoopLineage != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil {
			return nil, fmt.Errorf("factory must return fresh input HIR without policy/lineage")
		}
		if err := jointCheckFreshHIR(m, seen); err != nil {
			return nil, err
		}
		c, err := hir.BuildWorkloadProfileContext(m, workload)
		if err != nil {
			return nil, err
		}
		k, err := hir.BuildProfileCompatibility(m, c)
		if err != nil {
			return nil, err
		}
		if key == nil {
			key = k
		} else if !jointEqual(key, k) {
			return nil, fmt.Errorf("factory returned non-equivalent HIR/source/context")
		}
		retained = append(retained, m) // Keep pointers alive while checking independence.
		return m, nil
	}
	source, err := fresh()
	if err != nil {
		return fail(err)
	}
	profile, err := hir.LoadLoopProfile(bytes.NewReader(observations), source, key.Context)
	if err != nil {
		return fail(err)
	}
	guards := map[string]bool{}
	for _, g := range profile.Guards {
		guards[g.SiteID] = true
	}
	observed := map[string]bool{}
	for _, c := range profile.Counts {
		observed[c.SiteID] = c.Entries > 0
	}
	eligible := []string{}
	for _, s := range profile.Sites.Sites {
		if observed[s.ID] && guards[s.ID] {
			eligible = append(eligible, s.ID)
		}
	}
	weighted, err := hir.EligibleWeightedCompositionSites(source)
	if err != nil {
		return fail(err)
	}
	weightedSet := map[string]bool{}
	for _, id := range weighted {
		weightedSet[id] = true
	}
	for _, id := range eligible {
		if weightedSet[id] {
			return fail(fmt.Errorf("weighted overlap requires explicit measured v4 composition contract"))
		}
	}
	sort.Strings(eligible)
	if len(eligible) > maxSites {
		return fail(fmt.Errorf("%d observed eligible sites exceed cap %d (maximum 4 sites/16 sets)", len(eligible), maxSites))
	}
	// Preflight all factory inputs before any compilation or execution.
	candidateCount := 1 << len(eligible)
	if ruleMode == 1 {
		candidateCount *= 2
	} else if ruleMode == 2 {
		candidateCount *= 4
	}
	modules := make([]*hir.Module, candidateCount)
	for i := range modules {
		modules[i], err = fresh()
		if err != nil {
			return fail(err)
		}
	}
	// A factory may mutate an earlier returned module while producing the next.
	for _, m := range retained {
		c, err := hir.BuildWorkloadProfileContext(m, workload)
		if err != nil {
			return fail(fmt.Errorf("factory mutated previously returned HIR/source: %w", err))
		}
		k, err := hir.BuildProfileCompatibility(m, c)
		if err != nil {
			return fail(fmt.Errorf("factory mutated previously returned HIR/source: %w", err))
		}
		if !jointEqual(k, key) || m.OrderedByteRWPolicy != nil || m.OrderedBytePolicy != nil || m.RegionComparePolicy != nil || m.ForwardCopyPolicy != nil || m.PointerWalkPolicy != nil || m.LoopLineage != nil || m.GraceRulePolicy != nil || m.InvariantBasePolicy != nil {
			return fail(fmt.Errorf("factory mutated previously returned HIR/source"))
		}
	}
	oracle, err := hir.RunLoopProfile(source, workload)
	if err != nil {
		return fail(err)
	}
	// A compatible but forged observation must not introduce unobserved choices.
	if !jointEqual(profile.Counts, jointSortedCounts(oracle.Profile.Counts)) || !jointEqual(jointSortedGuards(profile.Guards), jointSortedGuards(oracle.Profile.Guards)) || profile.CompletedCalls != oracle.Profile.CompletedCalls {
		return fail(fmt.Errorf("observations do not match actual controlled workload"))
	}
	e := &ProfileVariantSetEvidence{Schema: "minz.joint-profile-variant-sets/v1", Compatibility: key, CompilationID: config, Options: options, AssemblerMode: "default", MaxSites: maxSites, EligibleSites: eligible, Oracle: oracle, Candidates: []ProfileVariantSetCandidate{}}
	if ruleMode == 2 {
		e.Schema = "minz.joint-profile-rule-variant-sets/v2"
	}
	e.Options.GraceStats = nil
	compiled := make([]Steps, len(modules))
	for mask, m := range modules {
		commonEnabled := ruleMode == 0 || mask >= (1<<len(eligible))
		var rulePolicy map[string]bool
		if ruleMode == 1 {
			m.GraceRulePolicy = map[string]bool{"hir-constant-if": commonEnabled, "hir-invariant-base": commonEnabled}
		} else if ruleMode == 2 {
			ruleMask := mask >> len(eligible)
			rulePolicy = map[string]bool{"hir-constant-if": ruleMask&1 != 0, "hir-invariant-base": ruleMask&2 != 0}
			m.GraceRulePolicy = map[string]bool{"hir-constant-if": rulePolicy["hir-constant-if"], "hir-invariant-base": rulePolicy["hir-invariant-base"]}
			commonEnabled = ruleMask == 3
		}
		choices := map[string]bool{}
		for j, id := range eligible {
			choices[id] = mask&(1<<(len(eligible)-1-j)) != 0
		}
		decisions := make([]hir.ProfileDecision, 0, len(profile.Sites.Sites))
		for _, s := range profile.Sites.Sites {
			variant := "original"
			if choices[s.ID] {
				variant = "pointer-walk"
			}
			decisions = append(decisions, hir.ProfileDecision{Compatibility: key, SiteID: s.ID, Variant: variant, Reason: "jointly measured variant set"})
		}
		if err := hir.ApplyProfileDecisions(m, decisions); err != nil {
			return fail(err)
		}
		lineage := hir.CurrentLoopLineage(m)
		applied := map[string]bool{}
		for _, l := range lineage {
			if l.Lineage.Role == "fast" && l.Lineage.Rule == "hir-guarded-pointer-walk" {
				applied[l.Lineage.SiteID] = true
			}
		}
		for id, requested := range choices {
			if requested != applied[id] {
				return fail(fmt.Errorf("candidate %d requested pointer choice was not realized for %s", mask, id))
			}
		}
		hirDump := m.Dump()
		raw, err := jointRunRaw(m, workload)
		if err != nil {
			return fail(fmt.Errorf("candidate %d raw VM: %w", mask, err))
		}
		if !jointSameState(raw, oracle) {
			return fail(fmt.Errorf("candidate %d raw VM semantic mismatch", mask))
		}
		steps, err := CompileHIRSteps(m, options)
		if err != nil {
			return fail(err)
		}
		steps.ProfileDecisions = decisions
		var finalHIR string
		var finalLineage []hir.LoopLineageRecord
		var finalRaw *hir.ProfileRun
		if ruleMode == 2 {
			finalHIR = steps.HIR                     // After Grace, before function splitting.
			finalLineage = hir.CurrentLoopLineage(m) // Surviving postcompile HIR.
			finalRaw, err = jointRunRaw(m, workload)
			if err != nil || !jointSameState(finalRaw, oracle) {
				return fail(fmt.Errorf("candidate %d postcompile raw semantic mismatch: %v", mask, err))
			}
			finalApplied := map[string]bool{}
			for _, l := range finalLineage {
				if l.Lineage.Role == "fast" && l.Lineage.Rule == "hir-guarded-pointer-walk" {
					finalApplied[l.Lineage.SiteID] = true
				}
			}
			for id, requested := range choices {
				if requested != finalApplied[id] {
					return fail(fmt.Errorf("candidate %d postcompile pointer choice was not realized for %s", mask, id))
				}
			}
		}
		assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if err != nil {
			return fail(err)
		}
		if len(assembled.Errors) > 0 || len(assembled.Binary) == 0 {
			return fail(fmt.Errorf("candidate %d assembly failed: %v", mask, assembled.Errors))
		}
		run, err := RunZ80ProfileWorkload(steps, source, workload, *oracle)
		if err != nil {
			return fail(fmt.Errorf("candidate %d target workload: %w", mask, err))
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(assembled.Binary))
		if run == nil || run.BinarySHA256 != digest || run.TotalTStates < 0 {
			return fail(fmt.Errorf("candidate %d target evidence/binary mismatch", mask))
		}
		// Sum entry-through-RET invocation costs: the harness may additionally
		// charge bootstrap cycles in its cumulative execution evidence.
		var total int64
		if len(run.Invocations) != len(workload.Calls) {
			return fail(fmt.Errorf("candidate %d incomplete invocation evidence", mask))
		}
		for _, invocation := range run.Invocations {
			if invocation.TStates < 0 || invocation.TStates > int64(^uint64(0)>>1)-total {
				return fail(fmt.Errorf("candidate %d invalid invocation costs", mask))
			}
			total += invocation.TStates
		}
		e.Candidates = append(e.Candidates, ProfileVariantSetCandidate{HIR: hirDump, Lineage: lineage, FinalHIR: finalHIR, FinalLineage: finalLineage, FinalRawRun: finalRaw, RulePolicy: rulePolicy, CommonGraceRules: commonEnabled, TotalTStates: total, Decisions: decisions, Binary: assembled.Binary, BinarySHA256: digest, ModuleBytes: len(assembled.Binary), RawRun: raw, Run: run})
		compiled[mask] = steps
	}
	for i := 1; i < len(e.Candidates); i++ {
		candidate, best := e.Candidates[i], e.Candidates[e.Selected]
		better := candidate.TotalTStates < best.TotalTStates
		if options.OptSize {
			better = candidate.ModuleBytes < best.ModuleBytes
		}
		if better {
			e.Selected = i
		}
	}
	data, err = json.Marshal(e)
	if err != nil {
		return fail(err)
	}
	e.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	best := e.Candidates[e.Selected]
	// retained remains live through all mutations: pointer identity is not a GC key.
	runtime.KeepAlive(retained)
	return &ProfileVariantSetResult{Steps: compiled[e.Selected], Decisions: best.Decisions, Binary: best.Binary, Evidence: e}, nil
}

func jointEqual(a, b any) bool {
	x, ex := json.Marshal(a)
	y, ey := json.Marshal(b)
	return ex == nil && ey == nil && bytes.Equal(x, y)
}
func jointSameState(a, b *hir.ProfileRun) bool {
	return jointEqual(a.Accesses, b.Accesses) && jointEqual(a.Snapshots, b.Snapshots) && jointEqual(a.Results, b.Results) && jointEqual(a.Globals, b.Globals) && jointEqual(a.Resources, b.Resources)
}
func jointSortedCounts(in []hir.LoopCounts) []hir.LoopCounts {
	out := make([]hir.LoopCounts, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool { return out[i].SiteID < out[j].SiteID })
	return out
}

func jointSortedGuards(in []hir.GuardCounts) []hir.GuardCounts {
	out := make([]hir.GuardCounts, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool { return out[i].SiteID < out[j].SiteID })
	return out
}

// Reject shared AST nodes and shared mutable backing storage, including globals
// and source maps. Canonical immutable MIR type objects may be shared.
// Keep copied pointer/map/slice values alive for the whole operation: compilation
// can detach nodes from retained modules, and uintptr alone permits GC/address
// reuse to look like shared ownership. Reflect field values must not be retained
// directly because they can follow later changes to the original field.
func jointCheckFreshHIR(m *hir.Module, seen map[uintptr]any) error {
	current := map[uintptr]any{}
	traversed := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				return
			}
			// Zero-sized nodes (e.g. BreakStmt) have no mutable state and
			// Go may legally give separate allocations the same address.
			if v.Type().Elem().Size() == 0 {
				return
			}
			if _, ok := v.Interface().(mir2.Ty); ok {
				return
			}
			p := v.Pointer()
			if traversed[p] {
				return
			}
			traversed[p] = true
			current[p] = v.Interface()
			walk(v.Elem())
		case reflect.Map:
			if v.IsNil() {
				return
			}
			p := v.Pointer()
			current[p] = v.Interface()
			if traversed[p] {
				return
			}
			traversed[p] = true
			iter := v.MapRange()
			for iter.Next() {
				walk(iter.Key())
				walk(iter.Value())
			}
		case reflect.Slice:
			if !v.IsNil() && v.Cap() > 0 {
				current[v.Pointer()] = v.Interface()
			}
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Field(i).CanInterface() {
					walk(v.Field(i))
				}
			}
		}
	}
	walk(reflect.ValueOf(m))
	for p := range current {
		if _, exists := seen[p]; exists {
			return fmt.Errorf("factory returned shared HIR nodes/storage")
		}
		seen[p] = current[p]
	}
	return nil
}

// Execute unoptimised MIR2 independently of target code, preserving ordered
// calls, resource resolution, globals, memory limits and cumulative VM gas.
// The lowering keeps the memory idioms the target emits (LowerModuleForTraffic)
// so the exact byte-traffic oracle models fill_bytes16's destination reloads.
func jointRunRaw(m *hir.Module, w hir.ProfileWorkload) (*hir.ProfileRun, error) {
	raw := hir.LowerModuleForTraffic(m)
	if err := mir2.Verify(raw); err != nil {
		return nil, err
	}
	vm := mir2.NewVM(raw)
	vm.Hosts = map[string]mir2.HostFunc{}
	vm.MaxSteps = w.MaxSteps
	vm.MaxMemory = w.MaxMemory
	for _, f := range raw.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				switch i.Op {
				case mir2.OpAsm, mir2.OpIn8, mir2.OpIn16, mir2.OpOut8, mir2.OpOut16:
					return nil, fmt.Errorf("ASM/port I/O unsupported")
				}
			}
		}
	}
	used := vm.AllocHeap(nil).I
	if used > int64(w.MaxMemory) {
		return nil, fmt.Errorf("initial storage exceeds memory limit")
	}
	for name, b := range w.Globals {
		vm.WriteHeapBytes(vm.GlobalAddr(name), b)
	}
	names := []string{}
	for name := range w.Resources {
		names = append(names, name)
	}
	sort.Strings(names)
	addresses := map[string]int64{}
	for _, name := range names {
		b := w.Resources[name]
		if int64(len(b)) > int64(w.MaxMemory)-used {
			return nil, fmt.Errorf("resources exceed memory limit")
		}
		addresses[name] = vm.AllocHeap(b).I
		used += int64(len(b))
	}
	out := &hir.ProfileRun{Snapshots: []hir.ProfileMemorySnapshot{}, Results: [][]mir2.Value{}, Globals: map[string][]byte{}, Resources: map[string][]byte{}}
	var accesses *hir.ProfileAccessRecorder
	if w.ExactDataAccess {
		accesses = hir.NewProfileAccessRecorder(m, vm, w, addresses)
		vm.ObserveDataAccess = accesses.Observe
	}

	for _, call := range w.Calls {
		args := make([]mir2.Value, len(call.Arguments))
		for i, a := range call.Arguments {
			if a.Value != nil {
				args[i].I = *a.Value
			} else {
				args[i].I = addresses[a.Resource] + int64(a.Offset)
			}
		}
		if accesses != nil {
			accesses.Reset()
		}
		r, err := vm.Call(call.Function, args)
		if err != nil {
			return nil, err
		}
		out.Results = append(out.Results, r)
		snapshot := hir.ProfileMemorySnapshot{Globals: map[string][]byte{}, Resources: map[string][]byte{}}
		for _, g := range m.Globals {
			size := mir2.ByteWidth(g.Ty)
			if len(g.Init) > size {
				size = len(g.Init)
			}
			snapshot.Globals[g.Name] = append([]byte{}, vm.ReadHeap(vm.GlobalAddr(g.Name), size)...)
		}
		for _, name := range names {
			snapshot.Resources[name] = append([]byte{}, vm.ReadHeap(addresses[name], len(w.Resources[name]))...)
		}
		out.Snapshots = append(out.Snapshots, snapshot)
		if accesses != nil {
			a, err := accesses.Snapshot()
			if err != nil {
				return nil, err
			}
			out.Accesses = append(out.Accesses, a)
		}
	}
	for _, g := range m.Globals {
		size := mir2.ByteWidth(g.Ty)
		if len(g.Init) > size {
			size = len(g.Init)
		}
		out.Globals[g.Name] = vm.ReadHeap(vm.GlobalAddr(g.Name), size)
	}
	for _, name := range names {
		out.Resources[name] = vm.ReadHeap(addresses[name], len(w.Resources[name]))
	}
	return out, nil
}
