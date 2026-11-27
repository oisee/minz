package pipeline

import (
	"sort"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// unrollEnabled reports whether lowering may fully unroll short constant-trip
// loops. Only the speed profile of emitted native Z80 unrolls: the byte budget
// is a measured comparison of assembled products, and the size profile must
// stay byte-identical. Measured/replayed loop configurations keep their loops.
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

// functionByteSizes measures each named function as the distance from its
// label to the next function label (the last one runs to the image end).
// Trailing data is identical in both compared products, so differences of
// these sizes are the functions' code growth.
func functionByteSizes(asm string, m *mir2.Module) (map[string]int, bool) {
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if err != nil || len(a.Errors) != 0 {
		return nil, false
	}
	type at struct {
		name string
		addr int
	}
	var labels []at
	for _, f := range m.Funcs {
		if addr, ok := a.Symbols[f.Name]; ok && len(f.Blocks) != 0 {
			labels = append(labels, at{f.Name, addr})
		}
	}
	sort.Slice(labels, func(i, j int) bool {
		if labels[i].addr != labels[j].addr {
			return labels[i].addr < labels[j].addr
		}
		return labels[i].name < labels[j].name
	})
	end := 0x8000 + len(a.Binary)
	sizes := map[string]int{}
	for i, l := range labels {
		next := end
		if i+1 < len(labels) {
			next = labels[i+1].addr
		}
		sizes[l.name] = next - l.addr
	}
	return sizes, true
}

// gateUnrolledProduct keeps the unrolled product only when every function that
// unrolled grew by at most UnrollMaxGrowthPerLoop per unrolled loop and at most
// UnrollMaxGrowthPerFunction, measured against a complete product compiled
// without unrolling. Offending functions are denied and the module is
// recompiled; a product that cannot be measured falls back to the baseline.
func gateUnrolledProduct(hm *hir.Module, opt Options, initial, s Steps, splitInfo map[string]hir.SplitResult, unrolled map[string]int) (Steps, error) {
	if len(unrolled) == 0 || s.Assembly == "" || opt.unrollUngated {
		return s, nil
	}
	baseOpt := opt
	baseOpt.DisableLoopUnroll = true
	baseOpt.GraceStats = nil
	// The mandatory comparison must produce complete allocated code even
	// when a search trial's MIR was seen before.
	baseOpt.variantMIRReady = nil
	baseModule := hir.LowerModuleWithOptions(hm, lowerOptions(hm, baseOpt, nil))
	base, err := compileMIRProductSteps(hm, baseOpt, initial, splitInfo, baseModule)
	if err != nil {
		return s, err
	}
	baseSizes, ok := functionByteSizes(base.Assembly, base.MIR2Module)
	if !ok {
		return base, nil
	}
	baseImage, err := assembleVariant(base)
	if err != nil {
		return base, nil
	}
	deny := map[string]bool{}
	for name, v := range opt.unrollDeny {
		deny[name] = v
	}
	for {
		sizes, ok := functionByteSizes(s.Assembly, s.MIR2Module)
		if !ok {
			return base, nil
		}
		image, err := assembleVariant(s)
		if err != nil {
			return base, nil
		}
		admitted := true
		names := make([]string, 0, len(unrolled))
		for name := range unrolled {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			after, inAfter := sizes[name]
			before, inBefore := baseSizes[name]
			// An unrolled function inlined or pruned in either product has no
			// separate size to measure: refuse rather than guess.
			budget := unrolled[name] * hir.UnrollMaxGrowthPerLoop
			if budget > hir.UnrollMaxGrowthPerFunction {
				budget = hir.UnrollMaxGrowthPerFunction
			}
			// Speed: the variant search's static model (loops weighted by
			// their proved trips, unknown loops by 16, branch arms by their
			// maximum) must strictly improve; an unscorable shape refuses.
			slow, scored := baseImage.score(base.MIR2Module.FuncByName(name))
			fast, scoredAfter := image.score(s.MIR2Module.FuncByName(name))
			if !scored || !scoredAfter || fast >= slow {
				deny[name] = true
				admitted = false
				continue
			}
			if !inAfter || !inBefore || after-before > budget {
				deny[name] = true
				admitted = false
			}
		}
		if admitted {
			return s, nil
		}
		retry := opt
		retry.unrollDeny = deny
		unrolled = map[string]int{}
		m := hir.LowerModuleWithOptions(hm, lowerOptions(hm, retry, unrolled))
		if len(unrolled) == 0 {
			return base, nil
		}
		retry.variantMIRReady = nil
		retry.GraceStats = nil
		if s, err = compileMIRProductSteps(hm, retry, initial, splitInfo, m); err != nil {
			return s, err
		}
	}
}
