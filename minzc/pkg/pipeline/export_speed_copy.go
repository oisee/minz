package pipeline

import (
	"sort"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80timing"
)

// Speed-profile export copies. An exported (C external linkage, or Nanz
// root) small loop-free leaf with one call site may be copied into its
// caller; its callable definition is retained, so the copy always grows the
// module. mir2.InlineSingleCallSites reports the candidates that pass its
// pre-filter (mir2.SingleCallInlineStats.SpeedCandidates) without copying
// them. The reference product R is the ordinary compile, which makes no
// speed copy: the product main ships. Only when R has a candidate is a
// candidate product C compiled, from R's own options plus a speedCopyPlan
// (selectSpeedCopies); C is kept only if every rule holds:
//
//  1. Soundness: every function other than the callers that received a
//     copy, the retained callees included, and every data, string and
//     runtime record, is identical between R and C (the variant search's
//     context comparison) and declares the identical contract line
//     (params, returns, clobbers). The plan pins the parameter classes of
//     each retained callee and of each caller to R's choice (PFCCO would
//     otherwise re-choose them: the callee lost its in-module caller, the
//     caller's body changed), and declares for both the clobber set R
//     declared, when they write no more (mir2.Z80CodegenOptions
//     ExactClobberEntries, DeclaredClobbers): the callee keeps its contract
//     line although no call site asks for it, and the caller's own callers
//     save as in R.
//  2. Timing: each caller that received a copy still exists in C and is
//     strictly cheaper under the static worst-path model than R's caller
//     with its CALL charged as CALL plus the callee's own worst path. A
//     caller or callee the model cannot score, or one with a loop whose
//     trip count is not proved, refuses: an unknown-trip loop is charged a
//     guessed 16 trips, and a copy lets LICM move work between the loop and
//     its preheader, so the guess would decide the comparison.
//  3. Growth: each caller grows by at most exportSpeedCopyMaxGrowth bytes
//     per copy it received, the whole image by at most
//     exportSpeedCopyModuleGrowth.
//
// A refused copy is dropped and C recompiled with the rest (at most
// exportSpeedCopyMaxCompiles compiles in all); when nothing is left, R
// ships. Under the module cap the copies kept are chosen by caller in order
// of growth (smallest first), then caller name.
//
// The decision is made once per module compile, on its final product: by
// compileMIRSteps for a plain compile, and by compileVariantSearch after the
// recipe search and the rotation recompile, with the selected recipes and
// rotation setting. Variant trials, decideUnroll's measurements and the
// internal size baselines never select (speedCopySelectionEnabled).

// exportSpeedCopyMaxGrowth bounds the growth (bytes) of a caller per speed
// copy it receives; the copied MIR body is bounded separately
// (mir2.ExportSpeedCopyMaxInsts / MaxBlocks).
const exportSpeedCopyMaxGrowth = 24

// exportSpeedCopyModuleGrowth bounds the image growth of all speed copies of
// one module.
const exportSpeedCopyModuleGrowth = 96

// exportSpeedCopyMaxCompiles bounds the candidate compiles of one module.
const exportSpeedCopyMaxCompiles = 3

// speedCopyPlan is a decided set of speed copies. A compile with a plan
// makes exactly these copies (when still eligible), pins the parameter
// classes and clobber declarations of their callees and callers to the
// reference's, and reproduces the reference's narrow-copy decision; it is
// never gated again.
type speedCopyPlan struct {
	allow  map[string]bool
	pins   map[string][]mir2.RegClass // callees and callers: R's parameter classes
	narrow bool                       // the reference kept its parameterless narrow copies
	// clobbers holds, per callee and caller, the exact clobber set R
	// declared (nil: R declared the full ABI).
	clobbers map[string]map[string]bool
}

// speedCopyCompileHook, when set (tests), is called once per candidate
// compile.
var speedCopyCompileHook func()

// declaredClobbers is the DeclaredClobbers codegen option of the plan's
// callees and callers (nil without a plan).
func (p *speedCopyPlan) declaredClobbers() map[string]map[string]bool {
	if p == nil {
		return nil
	}
	return p.clobbers
}

// exactEntries names the copied callees whose exact clobber set R declared:
// with no call site left, codegen proves it only on request
// (mir2.Z80CodegenOptions.ExactClobberEntries).
func (p *speedCopyPlan) exactEntries(copies map[string]string) map[string]bool {
	if p == nil {
		return nil
	}
	out := map[string]bool{}
	for callee := range copies {
		if p.clobbers[callee] != nil {
			out[callee] = true
		}
	}
	return out
}

// speedCopySelectionEnabled reports whether a compile decides speed copies
// for its product: the default speed profile on the ordinary Z80 product,
// outside variant trials, internal measurement compiles and explicit
// policy/profile/runtime configurations.
func speedCopySelectionEnabled(h *hir.Module, o Options) bool {
	if h == nil || o.OptSize || o.disableSpeedCopies || o.speedCopyPlan != nil || o.disableExportCopies ||
		o.SkipZ80Emission || o.Backend != "" && o.Backend != "z80" {
		return false
	}
	if h.OrderedByteRWPolicy != nil || h.OrderedBytePolicy != nil || h.RegionComparePolicy != nil || h.InvariantBasePolicy != nil ||
		h.GraceRulePolicy != nil || h.ForwardCopyPolicy != nil || h.PointerWalkPolicy != nil || h.LoopLineage != nil {
		return false
	}
	return !o.FuseGenerators && o.ProfileSelection == nil && !o.CollectHIRSites && o.CheckedLocalTablePath == "" && o.CheckedLocalTableSHA256 == "" &&
		o.Runtime == "" && !o.CPMApplicationEntry && !o.ZXApplicationEntry && o.AssertMode != "wasm" && o.AssertMode != "llvm"
}

// selectSpeedCopies decides the speed copies of reference product ref
// (rules above). compile builds the candidate for a plan from ref's own
// options. No compile when ref has no candidate.
func selectSpeedCopies(ref Steps, compile func(*speedCopyPlan) (Steps, error)) Steps {
	if len(ref.speedCandidates) == 0 || ref.MIR2Module == nil || ref.Assembly == "" {
		return ref
	}
	refImage, err := assembleVariant(ref)
	if err != nil {
		return ref
	}
	// The plan for a set of callees: R's parameter classes of each callee
	// and its caller, R's clobber declaration of each caller.
	planFor := func(callees map[string]bool) *speedCopyPlan {
		plan := &speedCopyPlan{allow: map[string]bool{}, pins: map[string][]mir2.RegClass{}, narrow: ref.narrowCopiesKept, clobbers: map[string]map[string]bool{}}
		for callee := range callees {
			caller := ref.speedCandidates[callee]
			f, c := ref.MIR2Module.FuncByName(callee), ref.MIR2Module.FuncByName(caller)
			if f == nil || c == nil {
				continue
			}
			plan.allow[callee] = true
			for _, g := range []*mir2.Func{f, c} {
				classes := []mir2.RegClass{}
				for _, p := range g.Contract.Params {
					classes = append(classes, p.Class)
				}
				plan.pins[g.Name] = classes
			}
			plan.clobbers[callee] = ref.clobberSets[callee]
			plan.clobbers[caller] = ref.clobberSets[caller]
		}
		return plan
	}
	all := map[string]bool{}
	for callee := range ref.speedCandidates {
		all[callee] = true
	}
	plan := planFor(all)
	for round := 0; round < exportSpeedCopyMaxCompiles && len(plan.allow) > 0; round++ {
		if speedCopyCompileHook != nil {
			speedCopyCompileHook()
		}
		cand, err := compile(plan)
		if err != nil {
			return ref
		}
		keep := judgeSpeedCopies(ref, refImage, cand, plan.allow)
		if len(keep) == len(plan.allow) {
			return cand
		}
		next := map[string]bool{}
		for _, callee := range keep {
			next[callee] = true
		}
		plan = planFor(next)
	}
	return ref
}

// judgeSpeedCopies returns the callees of allow whose copies in cand pass
// the rules against ref, sorted. All of allow means cand is accepted.
func judgeSpeedCopies(ref Steps, refImage *variantImage, cand Steps, allow map[string]bool) []string {
	if cand.MIR2Module == nil || cand.Assembly == "" {
		return nil
	}
	candImage, err := assembleVariant(cand)
	if err != nil {
		return nil
	}
	byCaller := map[string][]string{}
	for callee := range allow {
		caller := cand.speedCopies[callee]
		if caller == "" || ref.speedCandidates[callee] != caller {
			continue // not made, or made elsewhere: dropped
		}
		byCaller[caller] = append(byCaller[caller], callee)
	}
	// Rule 1. A differing retained callee drops its own copy; any other
	// differing function or record cannot be attributed and refuses all.
	owners := map[string]bool{}
	for owner := range refImage.context {
		owners[owner] = true
	}
	for owner := range candImage.context {
		owners[owner] = true
	}
	dropped := map[string]bool{}
	for owner := range owners {
		if byCaller[owner] != nil {
			continue
		}
		a, inRef := refImage.context[owner]
		b, inCand := candImage.context[owner]
		if inRef && inCand && refImage.header[owner] == candImage.header[owner] && strings.Join(a, "\n") == strings.Join(b, "\n") {
			continue
		}
		if allow[owner] && inRef && inCand {
			dropped[owner] = true
			continue
		}
		return nil
	}
	refSizes, candSizes := ownerCodeBytes(refImage), ownerCodeBytes(candImage)
	type group struct {
		caller  string
		callees []string
		growth  int
	}
	callers := make([]string, 0, len(byCaller))
	for caller := range byCaller {
		callers = append(callers, caller)
	}
	sort.Strings(callers)
	var groups []group
	for _, caller := range callers {
		callees := byCaller[caller]
		sort.Strings(callees)
		ok := true
		for _, callee := range callees {
			ok = ok && !dropped[callee]
		}
		growth := candSizes[caller] - refSizes[caller]
		// Rules 3 (per copy) and 2.
		if !ok || growth > exportSpeedCopyMaxGrowth*len(callees) || !speedCopyCallerFaster(refImage, candImage, ref, cand, caller, callees) {
			continue
		}
		groups = append(groups, group{caller, callees, growth})
	}
	// Rule 3, module cap: smallest growth first, then caller name.
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].growth < groups[j].growth })
	var keep []string
	total := 0
	for _, g := range groups {
		if total+g.growth > exportSpeedCopyModuleGrowth {
			continue
		}
		total += g.growth
		keep = append(keep, g.callees...)
	}
	sort.Strings(keep)
	// Rule 1 leaves the callers' own growth as all the image may gain; an
	// image beyond the cap anyway refuses this product.
	if len(keep) == len(allow) && len(candImage.binary)-len(refImage.binary) > exportSpeedCopyModuleGrowth {
		return nil
	}
	return keep
}

// ownerCodeBytes is the encoded size of each owner's lines.
func ownerCodeBytes(im *variantImage) map[string]int {
	sizes := map[string]int{}
	for owner, lines := range im.context {
		for _, line := range lines {
			// Each entry is "<first opcode byte>/<length>:<source>".
			_, rest, _ := strings.Cut(line, "/")
			n, _, _ := strings.Cut(rest, ":")
			if k, err := strconv.Atoi(n); err == nil {
				sizes[owner] += k
			}
		}
	}
	return sizes
}

// boundedScore is the static worst-path cost of f, refused (rule 2) when f
// has a loop without a proved trip count.
func boundedScore(im *variantImage, f *mir2.Func) (int64, bool) {
	cost, ok := im.score(f)
	if !ok {
		return 0, false
	}
	if r := im.regions[f.Name]; r != nil {
		for _, l := range r.loops {
			if l.fact.Trips < 0 {
				return 0, false
			}
		}
	}
	return cost, true
}

// speedCopyCallerFaster is rule 2 for one caller and the callees copied
// into it: the caller in cand must cost strictly less than ref's caller
// with each CALL callee charged as CALL plus the callee's own worst path
// (including its RET), and a tail call JP callee as JP plus the callee.
func speedCopyCallerFaster(refImage, candImage *variantImage, ref, cand Steps, caller string, callees []string) bool {
	after, ok := boundedScore(candImage, cand.MIR2Module.FuncByName(caller))
	if !ok {
		return false
	}
	// Patch a private copy of ref's nodes: ref's image is shared by every
	// caller's judgement.
	base := *refImage
	base.nodes = make(map[int]variantInstruction, len(refImage.nodes))
	for pc, node := range refImage.nodes {
		base.nodes[pc] = node
	}
	base.regions = map[string]*variantRegions{}
	callT, _ := z80timing.Lookup(0, 0xcd)
	jpT, _ := z80timing.Lookup(0, 0xc3)
	for _, callee := range callees {
		calleeCost, ok := boundedScore(refImage, ref.MIR2Module.FuncByName(callee))
		if !ok {
			return false
		}
		entry, ok := refImage.symbols[callee]
		if !ok {
			return false
		}
		sites := 0
		for pc, node := range refImage.nodes {
			at := pc - 0x8000
			if node.owner != caller || at < 0 || at+3 > len(refImage.binary) ||
				int(refImage.binary[at+1])|int(refImage.binary[at+2])<<8 != entry {
				continue
			}
			switch refImage.binary[at] {
			case 0xcd: // CALL callee: returns to the next instruction
				node.edges = []variantEdge{{to: pc + 3, cost: int64(callT) + calleeCost}}
			case 0xc3: // tail call JP callee: the callee's RET leaves the caller
				node.edges = []variantEdge{{to: -1, cost: int64(jpT) + calleeCost}}
			default:
				continue
			}
			sites++
			base.nodes[pc] = node
		}
		if sites != 1 {
			return false
		}
	}
	before, ok := boundedScore(&base, ref.MIR2Module.FuncByName(caller))
	return ok && after < before
}
