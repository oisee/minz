package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// FunctionVariant is a recipe, not a replacement assembly fragment. Each
// recipe is applied before compiling a fresh complete module, including PFCCO.
type FunctionVariant struct {
	Counted          hir.CountedLoopPolicy
	MIRGrace         bool
	HIRGrace         bool
	CompactByteLoops bool
	KeepCalls        bool
	IndexedPointers  bool
	BitTests         bool
	DJNZTrampoline   bool
	CompactShifts    bool
	SizeAllocation   bool
}

func (v FunctionVariant) String() string {
	s := fmt.Sprintf("counted=%d,mir-grace=%t,hir-grace=%t", v.Counted, v.MIRGrace, v.HIRGrace)
	if v.CompactByteLoops || v.KeepCalls || v.IndexedPointers || v.BitTests || v.DJNZTrampoline || v.CompactShifts || v.SizeAllocation {
		s += fmt.Sprintf(",compact-bytes=%t,calls=%t,indexed=%t,bit=%t,trampoline=%t,compact-shifts=%t,size-alloc=%t", v.CompactByteLoops, v.KeepCalls, v.IndexedPointers, v.BitTests, v.DJNZTrampoline, v.CompactShifts, v.SizeAllocation)
	}
	return s
}

// VariantDecision records the versioned static objective and strict selection.
// Static T is a model cost, not a measured runtime or an all-input speed proof.
type VariantDecision struct {
	Function      string
	Recipe        FunctionVariant
	Model         string
	Before, After int64
	Candidates    int
	Rejected      []string
}

// VariantSearchStats counts complete-module trials, including trials refused at
// the MIR checkpoint. The baseline and one MIR-only discovery pass are excluded.
// Limit is deterministic and independent of elapsed time or host load.
type VariantSearchStats struct {
	CandidateCompilations int
	Limit                 int
	DeduplicatedMIR       int
	DeduplicatedAssembly  int
	Exhausted             bool
}

// Bound module-wide work even when a large runtime has many rewritable helpers.
// Loop-free modules get four trials; loops add four each, up to sixteen.
func variantCompileLimit(loopFunctions int) int {
	if loopFunctions > 3 {
		return 16
	}
	return 4 + 4*loopFunctions
}

// This is a refusing cache, never a source of replacement code. JSON includes
// instruction/ABI metadata omitted by Dump; Dump includes private string-pool
// contents. Hash ties can only suppress a trial, never accept a partial product.
func variantMIRKey(m *mir2.Module) ([32]byte, bool) {
	b, err := json.Marshal(m)
	if err != nil {
		return [32]byte{}, false // recursive semantic types: disable this cache
	}
	for i := 0; i < m.Strings.Len(); i++ {
		b = append(b, byte(m.Strings.Kind(i)))
	}
	return sha256.Sum256(append(b, []byte(m.Dump())...)), true
}

// BranchEquiv admits only pure, call-free byte functions: its result depends
// on f alone. Cache only refusals, which leave f untouched. Each search owns its
// cache; no mutable MIR, accepted rewrite, VM state or backend cache is reused.
func runVariantBranchEquiv(m *mir2.Module, f *mir2.Func, misses map[[32]byte]bool) bool {
	if misses == nil {
		return mir2.BranchEquiv(m, f)
	}
	b, err := json.Marshal(f)
	if err != nil {
		return mir2.BranchEquiv(m, f)
	}
	key := sha256.Sum256(b)
	if misses[key] {
		return false
	}
	changed := mir2.BranchEquiv(m, f)
	if !changed {
		misses[key] = true
	}
	return changed
}

func variantFunctionKeys(m *mir2.Module) map[string]string {
	out := map[string]string{}
	for _, f := range m.Funcs {
		b, err := json.Marshal(f)
		if err != nil {
			out[f.Name] = ""
			continue
		}
		out[f.Name] = string(b)
	}
	return out
}

func variantAssemblyKey(asm string) [32]byte {
	var b strings.Builder
	for _, line := range strings.Split(asm, "\n") {
		// Strip only whole-line diagnostics: semicolons inside literals are data.
		if strings.HasPrefix(strings.TrimSpace(line), ";") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return sha256.Sum256([]byte(b.String()))
}

const variantCostModel = "z80-loop-max-v2" // unknown trips: 16; unknown arms: max

func variantFor(o Options, name string) FunctionVariant {
	if v, ok := o.FunctionVariants[name]; ok {
		return v
	}
	return FunctionVariant{MIRGrace: o.UseGrace, HIRGrace: o.UseHIRGrace}
}

func variantCountedPolicies(o Options) map[string]hir.CountedLoopPolicy {
	p := map[string]hir.CountedLoopPolicy{}
	for name, v := range o.FunctionVariants {
		p[name] = v.Counted
	}
	return p
}

func variantHasHIRGrace(m *hir.Module, o Options) bool {
	for _, f := range m.Funcs {
		if !f.IsExtern && f.Body != nil && variantFor(o, f.Name).HIRGrace {
			return true
		}
	}
	return false
}

func variantHIRFunctions(m *hir.Module, o Options) map[string]bool {
	p := map[string]bool{}
	for _, f := range m.Funcs {
		p[f.Name] = variantFor(o, f.Name).HIRGrace
	}
	return p
}

// variantSearchMaxFuncs bounds the module size searched by default.
const variantSearchMaxFuncs = 32

func variantSearchEnabled(h *hir.Module, options []Options) bool {
	o := DefaultOptions()
	if len(options) > 0 {
		o = options[0]
	}
	// Non-nil policies (including empty deny-all maps) and seeded lineage belong
	// to an exact measured/replay configuration. Search must neither change its
	// recipes nor compile a clone in place of the caller's attributed module.
	if h == nil || h.OrderedByteRWPolicy != nil || h.OrderedBytePolicy != nil ||
		h.RegionComparePolicy != nil || h.InvariantBasePolicy != nil ||
		h.GraceRulePolicy != nil || h.ForwardCopyPolicy != nil ||
		h.PointerWalkPolicy != nil || h.LoopLineage != nil {
		return false
	}
	// Preserve explicit policy/PGO/runtime/table experiments and intermediate-only
	// paths. Search has one supported native Z80 context in its first stage.
	// Every trial recompiles the whole module, so large programs pay many full
	// compiles for little gain; they search only on request.
	if h != nil && len(h.Funcs) > variantSearchMaxFuncs && !o.ForceVariantSearch {
		return false
	}
	return h != nil && !o.FuseGenerators && !o.DisableVariantSearch && !o.variantCandidate && len(o.FunctionVariants) == 0 && !o.DisableCountedLoops && !o.SkipZ80Emission &&
		(o.Backend == "" || o.Backend == "z80") && o.AssertMode != "wasm" && o.AssertMode != "llvm" && o.ProfileSelection == nil && !o.CollectHIRSites && o.CheckedLocalTablePath == "" && o.CheckedLocalTableSHA256 == "" && o.Runtime == "" && !o.CPMApplicationEntry && !o.ZXApplicationEntry && !needsZXApplication(h)
}

func compileVariantSearch(source *hir.Module, options ...Options) (Steps, error) {
	o := DefaultOptions()
	if len(options) > 0 {
		o = options[0]
	}
	// The frontend ran once. Resolve the common ABI/signature prefix once too;
	// every candidate still owns a deep HIR clone before Grace/splitting/lowering.
	prepared := hir.CloneModule(source)
	hir.OptimizeAggregateABI(prepared)
	if err := hir.ResolveErrorABI(prepared); err != nil {
		return Steps{}, err
	}
	if err := hir.ResolveCallSignatures(prepared); err != nil {
		return Steps{}, err
	}
	o.variantPrepared = true
	o.variantCandidate = true
	o.variantBranchEquivMisses = map[[32]byte]bool{}
	// Candidates do not run assertions or publish shared Grace counters. Validate
	// only the retained exact product, with the caller's original assertion mode.
	mode, stats := o.AssertMode, o.GraceStats
	o.AssertMode = "none"
	o.GraceStats = nil
	o.FunctionVariants = map[string]FunctionVariant{}
	var compiledStats *mir2.GraceStats
	compile := func() (Steps, error) {
		compiledStats = mir2.NewGraceStats()
		o.GraceStats = compiledStats
		return CompileHIRSteps(hir.CloneModule(prepared), o)
	}
	var searchStats VariantSearchStats
	seenMIR := map[[32]byte]bool{}
	var baselineFunctions map[string]string
	rawCallers := map[string]bool{}
	o.variantRawCallers = rawCallers
	inlinedCallers := map[string]bool{}
	o.variantInlinedCallers = inlinedCallers
	o.variantMIRReady = func(m *mir2.Module, sizeAlternative bool) bool {
		if key, ok := variantMIRKey(m); ok && !sizeAlternative {
			seenMIR[key] = true
		}
		baselineFunctions = variantFunctionKeys(m)
		return true
	}
	best, err := compile()
	if err != nil {
		return best, err
	}
	bestStats := compiledStats
	o.variantRawCallers = nil
	o.variantInlinedCallers = nil
	image, err := assembleVariant(best)
	if err != nil { // Unsupported assembly cannot admit any scored candidate.
		mergeGraceStats(stats, bestStats)
		return finishVariantSearch(source, best, mode)
	}
	// Probe MIR Grace once for the entire module, before allocation/PFCCO/LUTs.
	// Call-free functions are independent of the Grace selection of peers.
	// Callers remain eligible: folding a call can depend on a peer's VM step cost.
	mirChanges := map[string]bool{}
	discovery := o
	discovery.GraceStats = nil
	discovery.FunctionVariants = map[string]FunctionVariant{}
	loopFunctions := 0
	for _, f := range source.Funcs {
		v := variantFor(o, f.Name)
		v.MIRGrace = !v.MIRGrace
		discovery.FunctionVariants[f.Name] = v
		if f := best.MIR2Module.FuncByName(f.Name); f != nil && len(f.StaticLoops) > 0 {
			loopFunctions++
		}
	}
	discovery.variantMIRReady = func(m *mir2.Module, _ bool) bool {
		for name, key := range variantFunctionKeys(m) {
			mirChanges[name] = o.OptSize && len(m.FuncByName(name).StaticLoops) > 0 || rawCallers[name] || baselineFunctions[name] == "" || key == "" || baselineFunctions[name] != key
		}
		return false
	}
	if _, e := CompileHIRSteps(hir.CloneModule(prepared), discovery); e != nil {
		// Discovery failure is a refusal, not permission to compile unknown trials.
		mirChanges = map[string]bool{}
	}
	searchStats.Limit = variantCompileLimit(loopFunctions)
	o.variantMIRReady = func(m *mir2.Module, sizeAlternative bool) bool {
		// An affine size trial also compares a separately optimized no-affine MIR.
		// Its first checkpoint alone does not identify that complete product.
		if sizeAlternative {
			return true
		}
		key, ok := variantMIRKey(m)
		if !ok {
			return true
		}
		if seenMIR[key] {
			searchStats.DeduplicatedMIR++
			return false
		}
		seenMIR[key] = true
		return true
	}
	seenAssembly := map[[32]byte]bool{variantAssemblyKey(best.Assembly): true}
	names := make([]string, 0, len(source.Funcs))
	for _, f := range source.Funcs {
		if !f.IsExtern && f.Body != nil {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	var decisions []VariantDecision
	for _, name := range names {
		f := best.MIR2Module.FuncByName(name)
		if f == nil {
			continue // inlined or pruned
		}
		cost, supported := image.score(f)
		if o.OptSize {
			cost = int64(len(image.binary))
		}
		d := VariantDecision{Function: name, Recipe: variantFor(o, name), Model: variantObjective(o), Before: cost, After: cost}
		if !o.OptSize && !supported {
			d.Rejected = []string{"baseline cost unsupported"}
		}
		decisions = append(decisions, d)
	}
	recipesChanged := false
	// Sweep each lowering axis across the module before optional Grace toggles.
	// Otherwise cheap early Grace trials can exhaust the cap before a later
	// function gets even its ordinary-loop alternative. Ordering and cap remain
	// deterministic; every accepted recipe is still a fresh complete product.
	axes := []int{0, 1, 2, 3, 10}
	if o.OptSize {
		axes = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	}
	for _, axis := range axes {
		for _, i := range variantAxisOrder(axis, best.MIR2Module, decisions) {
			d := &decisions[i]
			name := d.Function
			current := variantFor(o, name)
			f := best.MIR2Module.FuncByName(name)
			if f == nil {
				continue // an earlier complete-module choice may have pruned it
			}
			// Keep loop-free speed search and its compilation budget unchanged.
			if !o.OptSize && axis == 10 && len(f.StaticLoops) == 0 {
				continue
			}
			oldCost, supported := image.score(f)
			if o.OptSize {
				oldCost = int64(len(image.binary))
			} else if !supported {
				continue
			}
			candidate := current
			switch axis {
			case 0:
				candidate.Counted = hir.CountedOrdinary
			case 1:
				candidate.Counted = hir.CountedPermit
			case 2:
				candidate.MIRGrace = !candidate.MIRGrace
			case 3:
				candidate.HIRGrace = !candidate.HIRGrace
			case 4:
				candidate.CompactByteLoops = !candidate.CompactByteLoops
			case 5:
				candidate.KeepCalls = !candidate.KeepCalls
			case 6:
				candidate.IndexedPointers = !candidate.IndexedPointers
			case 7:
				candidate.BitTests = !candidate.BitTests
			case 8:
				candidate.DJNZTrampoline = !candidate.DJNZTrampoline
			case 9:
				candidate.CompactShifts = !candidate.CompactShifts
			case 10:
				candidate.SizeAllocation = !candidate.SizeAllocation
			}
			if axis >= 4 && !sizeAxisRelevant(axis, best, name, inlinedCallers) {
				continue
			}
			if candidate == current {
				continue
			}
			if axis < 2 && len(f.StaticLoops) == 0 || axis == 2 && !mirChanges[name] && !recipesChanged {
				continue
			}
			if searchStats.CandidateCompilations >= searchStats.Limit {
				searchStats.Exhausted = true
				continue
			}
			o.FunctionVariants[name] = candidate
			// These axes affect only HIR/lowering before MIR optimization. Exact
			// equality of the complete raw MIR means the native product must tie;
			// no candidate is created or accepted from this early deduplication.
			if axis < 4 && axis != 2 {
				raw, e := variantRawModule(prepared, o)
				if e == nil && raw == best.MIR2Raw {
					o.FunctionVariants[name] = current
					continue
				}
			}
			o.variantNativeAlternative = axis == 4 || axis >= 7
			trial, e := compile()
			o.variantNativeAlternative = false
			searchStats.CandidateCompilations++
			d.Candidates++
			if e == nil && trial.Assembly == "" {
				o.FunctionVariants[name] = current
				continue // identical optimized MIR, refused before native compilation
			}
			if e == nil {
				key := variantAssemblyKey(trial.Assembly)
				if seenAssembly[key] {
					searchStats.DeduplicatedAssembly++
					o.FunctionVariants[name] = current
					continue
				}
				seenAssembly[key] = true
			}
			reason := ""
			var trialImage *variantImage
			if e != nil {
				reason = "compile: " + e.Error()
			} else {
				trialImage, e = assembleVariant(trial)
				if e != nil {
					reason = "assembly unsupported"
				} else if !o.OptSize && !image.sameContext(trialImage, name, sameVariantEntryABI(best.MIR2Module, trial.MIR2Module, name)) {
					reason = "changed boundary/caller/context"
				}
			}
			var cost int64
			if reason == "" {
				if o.OptSize {
					cost = int64(len(trialImage.binary))
				} else {
					var ok bool
					cost, ok = trialImage.score(trial.MIR2Module.FuncByName(name))
					// Allocation-only trials may trade bounded path regressions for
					// a lower loop-weighted maximum. Identical complete MIR keeps
					// source branch meaning aligned for the labeled CFG comparison.
					allocationOnly := axis == 10 && best.MIR2Opt == trial.MIR2Opt
					pathsOK := image.nonIncreasingPaths(trialImage, name)
					if !pathsOK && allocationOnly {
						pathsOK = image.boundedAllocationPaths(trialImage, name)
					}
					if !supported || !ok || !pathsOK || !sameVariantLoops(best.MIR2Module.FuncByName(name), trial.MIR2Module.FuncByName(name)) {
						reason = "unsupported loop comparison"
					}
				}
			}
			// Speed selection also refuses module growth. Size has its explicit byte
			// objective; only equal-byte size trials consult the static T model.
			better := cost < oldCost
			if o.OptSize && reason == "" && cost == oldCost {
				better = sizeVariantFaster(image, trialImage, best.MIR2Module, trial.MIR2Module)
			}
			if reason == "" && len(trialImage.binary) <= len(image.binary) && better {
				best, image, current, oldCost = trial, trialImage, candidate, cost
				recipesChanged = true
				bestStats = compiledStats
				_, supported = image.score(best.MIR2Module.FuncByName(name))
				d.Recipe, d.After = current, cost
			} else if reason != "" {
				d.Rejected = append(d.Rejected, fmt.Sprintf("%d:%s", axis, reason))
			}
			o.FunctionVariants[name] = current
		}
	}
	best.VariantDecisions = decisions
	best.VariantSearch = searchStats
	for _, d := range decisions {
		if tr := best.Traces[d.Function]; tr != nil {
			tr.Variant = fmt.Sprintf("%s/%s/%d->%d", d.Model, d.Recipe, d.Before, d.After)
		}
	}
	// Comments only: preserve every byte/instruction of the selected full module.
	lines := strings.Split(best.Assembly, "\n")
	owner := ""
	for i, line := range lines {
		if strings.HasPrefix(line, "; fun ") {
			owner, _, _ = strings.Cut(strings.TrimPrefix(line, "; fun "), "(")
		}
		if strings.HasPrefix(line, "; [trace] ") {
			if tr := best.Traces[owner]; tr != nil {
				lines[i] = formatTrace(tr)
			}
		}
	}
	best.Assembly = strings.Join(lines, "\n")
	mergeGraceStats(stats, bestStats)
	return finishVariantSearch(source, best, mode)
}

func finishVariantSearch(h *hir.Module, s Steps, mode string) (Steps, error) {
	// Resolve signatures in an owned copy just as the ordinary steps pipeline does.
	assertStats := h.AssertStats
	h = hir.CloneModule(h)
	h.AssertStats = assertStats
	hir.OptimizeAggregateABI(h)
	if err := hir.ResolveErrorABI(h); err != nil {
		return s, err
	}
	if err := hir.ResolveCallSignatures(h); err != nil {
		return s, err
	}
	if mode != "z80" && mode != "none" {
		if err := RunAssertsMIR2(h, s.MIR2Module); err != nil {
			return s, err
		}
	}
	if mode != "mir2" && mode != "none" && mode != "wasm" && mode != "llvm" {
		if err := RunAssertsZ80(h, s.MIR2Module, s.Allocation, s.Assembly); err != nil {
			return s, err
		}
	}
	return s, nil
}

// Only an unreferenced entry with the same logical signature can change its
// allocated entry registers. Fixed/explicit conventions retain the strict gate.
func sameVariantEntryABI(before, after *mir2.Module, name string) bool {
	for _, m := range []*mir2.Module{before, after} {
		for _, f := range m.Funcs {
			if f.Attrs.HasAsm {
				return false // opaque assembly may reference the entry through aliases
			}
			for _, block := range f.Blocks {
				for _, inst := range block.Insts {
					if inst.Op == mir2.OpAsm || inst.Sym == name {
						return false
					}
				}
			}
		}
	}
	a, b := before.FuncByName(name), after.FuncByName(name)
	if a == nil || b == nil || a.Attrs.HasAsm || b.Attrs.HasAsm || a.Attrs.FixedIndirectABI || b.Attrs.FixedIndirectABI || a.Attrs.IsExtern || b.Attrs.IsExtern || a.Attrs.FallibleCY || b.Attrs.FallibleCY {
		return false
	}
	if len(a.Contract.StructReturnBuffers) != 0 || len(b.Contract.StructReturnBuffers) != 0 || len(a.Contract.Params) != len(b.Contract.Params) || !reflect.DeepEqual(a.Contract.Returns, b.Contract.Returns) {
		return false
	}
	for i, p := range a.Contract.Params {
		q := b.Contract.Params[i]
		if p.ClassHard || q.ClassHard || !reflect.DeepEqual(p.Ty, q.Ty) {
			return false
		}
	}
	return true
}

func sameVariantLoops(a, b *mir2.Func) bool {
	if a == nil || b == nil {
		return false
	}
	// Stable pre-order identities and unchanged conditions are required for all
	// unknown loops. HIR versioning/peeling that changes their count is refused.
	unknown := func(f *mir2.Func) map[int]string {
		m := map[int]string{}
		for _, l := range f.StaticLoops {
			if l.Trips < 0 {
				m[l.ID] = l.Condition
			}
		}
		return m
	}
	return reflect.DeepEqual(unknown(a), unknown(b))
}

// variantRawModule replays the ordinary complete-module preparation prefix.
// No allocation, code fragments, or partially compiled product is reused.
func variantRawModule(source *hir.Module, o Options) (string, error) {
	h := hir.CloneModule(source)
	if !o.variantPrepared {
		hir.OptimizeAggregateABI(h)
		if e := hir.ResolveErrorABI(h); e != nil {
			return "", e
		}
		if e := hir.ResolveCallSignatures(h); e != nil {
			return "", e
		}
	}
	if variantHasHIRGrace(h, o) {
		if _, e := hir.RunGraceWithOptions(h, hir.GraceOptions{OptimizeForSize: o.OptSize, Functions: variantHIRFunctions(h, o)}); e != nil {
			return "", e
		}
	}
	hir.SplitHighPressure(h)
	if e := hir.ResolveCallSignatures(h); e != nil {
		return "", e
	}
	return hir.LowerModuleWithOptions(h, hir.LowerOptions{FuseGenerators: o.FuseGenerators, DisableCountedLoops: o.DisableCountedLoops, CountedLoopPolicy: variantCountedPolicies(o)}).Dump(), nil
}
