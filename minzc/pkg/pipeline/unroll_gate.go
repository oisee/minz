package pipeline

import (
	"sort"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
)

// unrollEnabled reports whether lowering may fully unroll short constant-trip
// loops. Only the speed profile of emitted native Z80 unrolls: the budget is a
// measured comparison of assembled products, and the size profile must stay
// byte-identical. Measured/replayed loop configurations keep their loops.
func unrollEnabled(hm *hir.Module, o Options) bool {
	return !o.OptSize && !o.DisableLoopUnroll && !o.SkipZ80Emission && (o.Backend == "" || o.Backend == "z80") &&
		!o.FuseGenerators && !measuredLoopConfiguration(hm, o)
}

// lowerOptions is the one place pipeline lowering options are formed, so the
// product, its size baselines and the variant search's raw replay agree.
func lowerOptions(hm *hir.Module, o Options, unrolled map[string]int) hir.LowerOptions {
	return hir.LowerOptions{
		FuseGenerators:      o.FuseGenerators,
		DisableCountedLoops: o.DisableCountedLoops,
		CountedLoopPolicy:   variantCountedPolicies(o),
		UnrollLoops:         unrollEnabled(hm, o),
		UnrollDeny:          o.unrollDeny,
		Unrolled:            unrolled,
	}
}

// unrollAdmissions replays the lowering prefix on a clone (no MIR
// optimisation, allocation or codegen) and reports the loops lowering would
// unroll per function. A module without any admitted loop costs nothing more.
func unrollAdmissions(source *hir.Module, o Options) (map[string]int, error) {
	h := hir.CloneModule(source)
	hir.OptimizeAggregateABI(h)
	if err := hir.ResolveErrorABI(h); err != nil {
		return nil, err
	}
	if err := hir.ResolveCallSignatures(h); err != nil {
		return nil, err
	}
	if variantHasHIRGrace(h, o) {
		if _, err := hir.RunGraceWithOptions(h, hir.GraceOptions{OptimizeForSize: o.OptSize, Functions: variantHIRFunctions(h, o)}); err != nil {
			return nil, err
		}
	}
	hir.SplitHighPressure(h)
	if err := hir.ResolveCallSignatures(h); err != nil {
		return nil, err
	}
	unrolled := map[string]int{}
	hir.LowerModuleWithOptions(h, lowerOptions(h, o, unrolled))
	return unrolled, nil
}

// unrollRoot names the source function a lowered function belongs to: a
// high-pressure split helper f$split_N (f_split_N in assembly) is charged to f.
func unrollRoot(name string) string {
	for _, sep := range []string{"$split_", "_split_"} {
		if i := strings.Index(name, sep); i > 0 {
			return name[:i]
		}
	}
	return name
}

// ownCodeBytes measures every function's own emitted bytes from the
// assembled image: the listing lines between its "; fun" header and the next
// header or module section (globals, strings, runtime helpers and tables
// belong to no function; assembleVariant's owners). Split helpers are added
// to their source function. It also returns the whole image size.
func ownCodeBytes(im *variantImage) (map[string]int, int) {
	sizes := map[string]int{}
	for owner, lines := range im.context {
		if owner == "" {
			continue
		}
		for _, line := range lines {
			// Each entry is "<first opcode byte>/<length>:<source>".
			_, rest, _ := strings.Cut(line, "/")
			n, _, _ := strings.Cut(rest, ":")
			if k, err := strconv.Atoi(n); err == nil {
				sizes[unrollRoot(owner)] += k
			}
		}
	}
	return sizes, len(im.binary)
}

// UnrollImageSlack is the image growth tolerated beyond the admitted
// functions' own code: none. Everything else in the image (globals,
// strings, runtime helpers and tables, padding) is unchanged unless
// unrolling changed it, and then the change is growth unrolling caused.
const UnrollImageSlack = 0

// unrollImageAdmits is the module-level rule: the whole image grows by no
// more than the admitted functions' own-code growth plus UnrollImageSlack.
func unrollImageAdmits(imageGrowth, ownGrowth int) bool {
	return imageGrowth <= ownGrowth+UnrollImageSlack
}

// unrollVerdict is the per-function rule: "" admits; otherwise the reason.
// A function must be measurable in both products, grow its own code by at
// most budget, be scored by the static model in BOTH products (a candidate
// the model cannot score is never assumed faster, whatever the base scored)
// and score strictly faster.
func unrollVerdict(measured bool, growth, budget int, slow int64, scored bool, fast int64, scoredAfter bool) string {
	switch {
	case !measured:
		return "unmeasured"
	case growth > budget:
		return "bytes"
	case !scored || !scoredAfter:
		return "unscored"
	case fast >= slow:
		return "slower"
	}
	return ""
}

// decideUnroll runs once per top-level compilation, before the variant
// search and outside its trials, and fixes which functions may unroll
// (Options.unrollDeny). When the lowering prefix admits no loop it returns at
// once. Otherwise it compiles two complete products without the variant
// search, without and with unrolling, and refuses a function unless
//   - its own emitted code (split helpers included; runtime helpers and
//     tables are not charged to it) grew by at most UnrollMaxGrowthPerLoop
//     per unrolled loop and at most UnrollMaxGrowthPerFunction, and
//   - the variant search's static T model scores it strictly faster.
//
// Second, module-level condition: the whole image may grow by at most the
// measured own-code growth of the admitted functions plus UnrollImageSlack.
// Any other growth (a runtime helper or table unrolling links in, alignment
// padding it shifts) is charged; a helper or table it drops buys no budget,
// because it only lowers the image side.
// Refused functions are denied and the candidate recompiled (at most once per
// function). The final product is compiled normally, with the variant search,
// under that decision; the decision is measured on the default recipes.
//
// A candidate that fails to compile (for example the pre-existing MIR verify
// failure on a block-scoped declaration after a diamond, see
// TestUnrollCandidateErrorKeepsLoops) disables unrolling for the module.
func decideUnroll(hm *hir.Module, opt Options) (Options, error) {
	opt.unrollDecided = true
	if !unrollEnabled(hm, opt) || opt.unrollUngated {
		return opt, nil
	}
	admitted, err := unrollAdmissions(hm, opt)
	if err != nil || len(admitted) == 0 {
		return opt, nil // the real compilation reports any error
	}
	trial := opt
	trial.AssertMode = "none"
	trial.GraceStats = nil
	trial.DisableVariantSearch = true
	// Trials print nothing: the emitted product reports the module's
	// diagnostics once ([HIR-SPLIT], [label-audit], Z80-VALIDATE).
	trial.quietDiagnostics = true
	trial.deferZ80Validation = true
	trial.unrollUngated = true // the candidate keeps what lowering admits
	baseOpt := trial
	baseOpt.DisableLoopUnroll = true
	base, err := CompileHIRSteps(hir.CloneModule(hm), baseOpt)
	if err != nil {
		opt.DisableLoopUnroll = true
		return opt, nil // the real compilation reports it
	}
	baseImage, imageErr := assembleVariant(base)
	if imageErr != nil {
		opt.DisableLoopUnroll = true
		return opt, nil
	}
	baseSizes, baseImageBytes := ownCodeBytes(baseImage)
	deny := map[string]bool{}
	for name, v := range opt.unrollDeny {
		deny[name] = v
	}
	for round := 0; round <= len(admitted); round++ {
		trial.unrollDeny = deny
		// A candidate the rest of the pipeline cannot compile keeps the
		// loops: unrolling must never turn a compiling module into an error.
		cand, err := CompileHIRSteps(hir.CloneModule(hm), trial)
		if err != nil {
			break
		}
		if len(cand.unrolled) == 0 {
			break
		}
		image, imageErr := assembleVariant(cand)
		if imageErr != nil {
			break
		}
		sizes, imageBytes := ownCodeBytes(image)
		loops := map[string]int{}
		var names []string
		for name, n := range cand.unrolled {
			root := unrollRoot(name)
			if loops[root] == 0 {
				names = append(names, root)
			}
			loops[root] += n
		}
		sort.Strings(names)
		changed, allowance := false, 0
		for _, name := range names {
			budget := loops[name] * hir.UnrollMaxGrowthPerLoop
			if budget > hir.UnrollMaxGrowthPerFunction {
				budget = hir.UnrollMaxGrowthPerFunction
			}
			after, inAfter := sizes[name]
			before, inBefore := baseSizes[name]
			slow, scored := baseImage.score(base.MIR2Module.FuncByName(name))
			fast, scoredAfter := image.score(cand.MIR2Module.FuncByName(name))
			reason := unrollVerdict(inBefore && inAfter, after-before, budget, slow, scored, fast, scoredAfter)
			if reason != "" {
				for lowered := range cand.unrolled {
					if unrollRoot(lowered) == name {
						deny[lowered] = true
					}
				}
				changed = true
				continue
			}
			allowance += after - before
		}
		if changed {
			// Nothing left to measure once every admitted function is denied.
			open := false
			for name := range admitted {
				open = open || !deny[name]
			}
			if !open {
				break
			}
			continue
		}
		if !unrollImageAdmits(imageBytes-baseImageBytes, allowance) {
			break
		}
		opt.unrollDeny = deny
		return opt, nil
	}
	opt.DisableLoopUnroll = true
	return opt, nil
}
