package mir2

import "github.com/minz/minzc/pkg/z80spec"

// preferByteLoadStepHome elides a byte copy when the load is consumed only by
// an adjacent small add/sub. No access is moved or retired. The constant must
// already be suppressed, and no value may need the new home in between.
func (g *z80cg) preferByteLoadStepHome(load *Inst) {
	if load.Op != OpLoad || load.Ty != TyU8 || load.ClsHard || g.curBlock == nil || g.nativeMemorySink != nil || g.nativeCallFrameSink != nil || g.nativeCallSpillSink != nil || countRegUses(g.fn)[load.Dst] != 1 {
		return
	}
	n := g.curInstIdx + 1
	if n >= len(g.curBlock.Insts) {
		return
	}
	var constant *Inst
	if x := g.curBlock.Insts[n]; x.Op == OpConst {
		constant = x
		n++
	}
	if n >= len(g.curBlock.Insts) {
		return
	}
	add := g.curBlock.Insts[n]
	if add.Src[0] != load.Dst || add.ClsHard || !g.deadConsts[add.Src[1]] {
		return
	}
	k, ok := g.constVals[add.Src[1]]
	if constant != nil {
		if constant.Dst != add.Src[1] {
			return
		}
		k, ok = constant.Imm, true
	}
	dst := g.loc(add.Dst)
	switch dst {
	case "A", "B", "C", "D", "E", "H", "L":
	default:
		return
	}
	if !ok || !byteSmallStep(g.fn, add, k, dst, dst) || dst == g.loc(load.Dst) {
		return
	}
	for r := range g.regsLiveAfterInst(load) {
		if r != load.Dst && r != add.Dst && !g.deadConsts[r] && z80spec.Overlaps(g.loc(r), dst) {
			return
		}
	}
	g.physOverride[load.Dst] = dst
}
