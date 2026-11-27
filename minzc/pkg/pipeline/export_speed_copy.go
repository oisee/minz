package pipeline

import "github.com/minz/minzc/pkg/z80timing"

// exportSpeedCopyMaxGrowth bounds the module growth (bytes) a speed-profile
// export copy may cost; the copied MIR body is bounded separately
// (mir2.ExportSpeedCopyMaxInsts / MaxBlocks).
const exportSpeedCopyMaxGrowth = 24

// exportSpeedCopiesFaster is the measured decision for speed-only export
// copies (mir2.SingleCallInlineStats.SpeedCopies, callee -> caller). It uses
// the variant search's static worst-path model on both complete products:
// the caller with the copy inlined must cost strictly less than the
// baseline caller with its CALL charged as CALL plus the callee's own
// worst path (including its RET). Any function the model cannot score
// refuses the copy.
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
		for pc, node := range base.nodes {
			at := pc - 0x8000
			if node.owner != caller || len(node.edges) != 0 || at < 0 || at+3 > len(base.binary) ||
				base.binary[at] != 0xcd || int(base.binary[at+1])|int(base.binary[at+2])<<8 != entry {
				continue
			}
			node.edges = []variantEdge{{to: pc + 3, cost: int64(callT) + calleeCost}}
			base.nodes[pc] = node
		}
		before, ok := base.score(baseline.MIR2Module.FuncByName(caller))
		if !ok || after >= before {
			return false
		}
	}
	return true
}
