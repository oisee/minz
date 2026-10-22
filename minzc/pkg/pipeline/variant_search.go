package pipeline

import (
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
	Counted  hir.CountedLoopPolicy
	MIRGrace bool
	HIRGrace bool
}

func (v FunctionVariant) String() string {
	return fmt.Sprintf("counted=%d,mir-grace=%t,hir-grace=%t", v.Counted, v.MIRGrace, v.HIRGrace)
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

const variantCostModel = "z80-loop-max-v1" // unknown trips: 16; unknown arms: max

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

func variantSearchEnabled(h *hir.Module, options []Options) bool {
	var o Options
	if len(options) > 0 {
		o = options[0]
	}
	// Preserve explicit policy/PGO/runtime/table experiments and intermediate-only
	// paths. Search has one supported native Z80 context in its first stage.
	return h != nil && !o.DisableVariantSearch && !o.variantCandidate && len(o.FunctionVariants) == 0 && !o.DisableCountedLoops && !o.SkipZ80Emission &&
		(o.Backend == "" || o.Backend == "z80") && o.AssertMode != "wasm" && o.AssertMode != "llvm" && o.ProfileSelection == nil && o.CheckedLocalTablePath == "" && o.Runtime == "" && !o.CPMApplicationEntry && !o.ZXApplicationEntry && !needsZXApplication(h)
}

func compileVariantSearch(source *hir.Module, options ...Options) (Steps, error) {
	var o Options
	if len(options) > 0 {
		o = options[0]
	}
	o.variantCandidate = true
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
		return CompileHIRSteps(hir.CloneModule(source), o)
	}
	best, err := compile()
	if err != nil {
		return best, err
	}
	bestStats := compiledStats
	image, err := assembleVariant(best)
	if err != nil { // unsupported assembly preserves the ordinary product
		return finishVariantSearch(source, best, mode)
	}
	names := make([]string, 0, len(source.Funcs))
	for _, f := range source.Funcs {
		if !f.IsExtern && f.Body != nil {
			names = append(names, f.Name)
		}
	}
	sort.Strings(names)
	var decisions []VariantDecision
	for _, name := range names {
		if best.MIR2Module.FuncByName(name) == nil {
			continue
		} // inlined or pruned
		current := variantFor(o, name)
		d := VariantDecision{Function: name, Recipe: current, Model: variantCostModel}
		oldCost, supported := image.score(best.MIR2Module.FuncByName(name))
		if o.OptSize {
			oldCost = int64(len(image.binary))
		}
		d.Before, d.After = oldCost, oldCost
		if !o.OptSize && !supported {
			d.Rejected = []string{"baseline cost unsupported"}
			decisions = append(decisions, d)
			continue
		}
		// One deterministic coordinate sweep: at most four extra complete module
		// compiles per input function. No goroutines: the compiler still has shared
		// backend caches, so candidate compilation is deliberately serial.
		for axis := 0; axis < 4; axis++ {
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
			}
			if candidate == current {
				continue
			}
			o.FunctionVariants[name] = candidate
			// These axes affect only HIR/lowering before MIR optimization. Exact
			// equality of the complete raw MIR means the native product must tie;
			// no candidate is created or accepted from this early deduplication.
			if axis != 2 {
				raw, e := variantRawModule(source, o)
				if e == nil && raw == best.MIR2Raw {
					o.FunctionVariants[name] = current
					continue
				}
			}
			trial, e := compile()
			d.Candidates++
			reason := ""
			var trialImage *variantImage
			if e != nil {
				reason = "compile: " + e.Error()
			} else {
				trialImage, e = assembleVariant(trial)
				if e != nil {
					reason = "assembly unsupported"
				} else if !image.sameContext(trialImage, name) {
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
					if !supported || !ok || !image.nonIncreasingPaths(trialImage, name) || !sameVariantLoops(best.MIR2Module.FuncByName(name), trial.MIR2Module.FuncByName(name)) {
						reason = "unsupported loop comparison"
					}
				}
			}
			// Speed selection also refuses module growth. Size has its explicit byte
			// objective; ties always retain the incumbent complete product.
			if reason == "" && len(trialImage.binary) <= len(image.binary) && cost < oldCost {
				best, image, current, oldCost = trial, trialImage, candidate, cost
				bestStats = compiledStats
				_, supported = image.score(best.MIR2Module.FuncByName(name))
				d.Recipe, d.After = current, cost
			} else if reason != "" {
				d.Rejected = append(d.Rejected, fmt.Sprintf("%d:%s", axis, reason))
			}
			o.FunctionVariants[name] = current
		}
		decisions = append(decisions, d)
	}
	best.VariantDecisions = decisions
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
	hir.OptimizeCAggregateABI(h)
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
	hir.OptimizeCAggregateABI(h)
	if e := hir.ResolveErrorABI(h); e != nil {
		return "", e
	}
	if e := hir.ResolveCallSignatures(h); e != nil {
		return "", e
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
	return hir.LowerModuleWithOptions(h, hir.LowerOptions{DisableCountedLoops: o.DisableCountedLoops, CountedLoopPolicy: variantCountedPolicies(o)}).Dump(), nil
}
