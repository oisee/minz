package pipeline

import (
	"sort"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80timing"
)

// exportSpeedCopyMaxGrowth bounds the module growth (bytes) a speed-profile
// export copy may cost; the copied MIR body is bounded separately
// (mir2.ExportSpeedCopyMaxInsts / MaxBlocks).
const exportSpeedCopyMaxGrowth = 24

// exportSpeedCopyMaxTrials bounds the per-callee trials of one module: each
// costs one complete recompile. Candidates beyond it (in callee-name order)
// are refused.
const exportSpeedCopyMaxTrials = 4

// selectSpeedCopies decides the speed-profile export copies of candidate s
// (callee -> caller in copies). The reference is the product main builds:
// the same compile with speed copies off, whose narrow (parameterless
// one-block) copies pass main's byte gate on their own. A refused speed copy
// therefore leaves exactly that product.
//
//  1. All copies together are kept if each caller is faster than in the
//     reference and the module grows by at most exportSpeedCopyMaxGrowth per
//     copy. s is that product unless the reference refused narrow copies s
//     has; then a trial without them is compiled.
//  2. Otherwise, with more than one candidate, each callee (name order, at
//     most exportSpeedCopyMaxTrials) is tried on top of the copies accepted
//     so far: one recompile each, kept when its caller gets faster and the
//     module grows by at most exportSpeedCopyMaxGrowth.
//
// Compiles: 1 (reference) + up to 1 (joint trial) + up to
// exportSpeedCopyMaxTrials. The reference also pays main's own narrow gate.
// If the reference or a trial fails to compile, that is an internal
// inconsistency (they only disable copies of a product that compiled): a
// failed trial is skipped, and a failed reference ships s, which compiled.
func selectSpeedCopies(hm *hir.Module, opt Options, s, initial Steps, splitInfo map[string]hir.SplitResult, copies map[string]string) Steps {
	recompile := func(o Options) (Steps, bool) {
		o.GraceStats = nil
		o.variantMIRReady = nil
		m := hir.LowerModuleWithOptions(hm, lowerOptions(hm, opt, nil))
		r, err := compileMIRProductSteps(hm, o, initial, splitInfo, m)
		return r, err == nil
	}
	refOpt := opt
	refOpt.disableSpeedCopies = true
	ref, ok := recompile(refOpt)
	if !ok {
		return s
	}
	trialOpt := func(allow map[string]bool) Options {
		o := opt
		o.speedCopyTrial = true
		o.speedCopyAllow = allow
		o.disableNarrowCopies = !ref.narrowCopiesKept
		return o
	}
	accept := func(cand, base Steps, subset map[string]string) bool {
		for callee := range subset {
			if cand.speedCopies[callee] == "" {
				return false
			}
		}
		cb, ce := Assemble(cand.Assembly, "z80")
		bb, be := Assemble(base.Assembly, "z80")
		// A copy that does not grow the module is kept as main keeps narrow
		// copies; a larger one must be faster and bounded.
		return len(ce) == 0 && len(be) == 0 && (len(cb) <= len(bb) ||
			len(cb) <= len(bb)+exportSpeedCopyMaxGrowth*len(subset) && exportSpeedCopiesFaster(cand, base, subset))
	}
	joint := s
	if s.narrowCopies > 0 && !ref.narrowCopiesKept {
		all := map[string]bool{}
		for callee := range copies {
			all[callee] = true
		}
		if joint, ok = recompile(trialOpt(all)); !ok {
			joint = Steps{}
		}
	}
	if joint.MIR2Module != nil && accept(joint, ref, copies) {
		return joint
	}
	if len(copies) == 1 {
		return ref
	}
	names := make([]string, 0, len(copies))
	for callee := range copies {
		names = append(names, callee)
	}
	sort.Strings(names)
	best, allow := ref, map[string]bool{}
	for i, callee := range names {
		if i >= exportSpeedCopyMaxTrials {
			break
		}
		next := map[string]bool{callee: true}
		for c := range allow {
			next[c] = true
		}
		t, ok := recompile(trialOpt(next))
		if ok && accept(t, best, map[string]string{callee: copies[callee]}) {
			best, allow = t, next
		}
	}
	return best
}

// exportSpeedCopiesFaster is the measured decision for speed-only export
// copies (mir2.SingleCallInlineStats.SpeedCopies, callee -> caller). It uses
// the variant search's static worst-path model on both complete products:
// the caller with the copy inlined must cost strictly less than the
// baseline caller with its CALL charged as CALL plus the callee's own
// worst path (including its RET); a tail call JP callee is charged as JP
// plus the callee. Any function the model cannot score refuses the copy.
func exportSpeedCopiesFaster(candidate, baseline Steps, copies map[string]string) bool {
	if candidate.MIR2Module == nil || baseline.MIR2Module == nil {
		return false
	}
	cand, err := assembleVariant(candidate)
	if err != nil {
		return false
	}
	base, err := assembleVariant(baseline)
	if err != nil {
		return false
	}
	callT, _ := z80timing.Lookup(0, 0xcd)
	for callee, caller := range copies {
		after, ok := cand.score(candidate.MIR2Module.FuncByName(caller))
		if !ok {
			return false
		}
		calleeCost, ok := base.score(baseline.MIR2Module.FuncByName(callee))
		if !ok {
			return false
		}
		entry, ok := base.symbols[callee]
		if !ok {
			return false
		}
		jpT, _ := z80timing.Lookup(0, 0xc3)
		for pc, node := range base.nodes {
			at := pc - 0x8000
			if node.owner != caller || at < 0 || at+3 > len(base.binary) ||
				int(base.binary[at+1])|int(base.binary[at+2])<<8 != entry {
				continue
			}
			switch base.binary[at] {
			case 0xcd: // CALL callee: returns to the next instruction
				if len(node.edges) == 0 {
					node.edges = []variantEdge{{to: pc + 3, cost: int64(callT) + calleeCost}}
				}
			case 0xc3: // tail call JP callee: the callee's RET leaves the caller
				node.edges = []variantEdge{{to: -1, cost: int64(jpT) + calleeCost}}
			default:
				continue
			}
			base.nodes[pc] = node
		}
		before, ok := base.score(baseline.MIR2Module.FuncByName(caller))
		if !ok || after >= before {
			return false
		}
	}
	return true
}
