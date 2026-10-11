package mir2

import "fmt"

// tryExperimentalUnsignedPairCmp is deliberately narrower than genCmp16.
// It changes no memory operation and preserves both word operands.
func (g *z80cg) tryExperimentalUnsignedPairCmp(inst *Inst) bool {
	if !g.experimentalUnsignedPreservePair || g.fn == nil || g.curBlock == nil ||
		(inst.Cond != CmpUlt && inst.Cond != CmpUge) || inst.Ty != TyBool ||
		inst.Dst == NoReg || g.loc(inst.Dst) != "F" || inst.FlagsOnly ||
		(inst.SrcTy != nil && inst.SrcTy != TyU16) {
		return false
	}
	if g.regInfo[inst.Src[0]].Ty != TyU16 || g.regInfo[inst.Src[1]].Ty != TyU16 {
		return false
	}
	if _, constant := g.constVals[inst.Src[0]]; constant {
		return false
	}
	if _, constant := g.constVals[inst.Src[1]]; constant {
		return false
	}
	lhs, rhs := g.loc(inst.Src[0]), g.loc(inst.Src[1])
	ordinary := func(s string) bool { return s == "HL" || s == "DE" || s == "BC" }
	if !ordinary(lhs) || !ordinary(rhs) || lhs == rhs {
		return false
	}
	live := g.scratchLiveAfter(inst)
	for r := range live {
		loc := g.loc(r)
		if loc == "A" || loc == "AF" {
			return false
		}
	}
	// Do not consume or relocate a pending scalar condition/accumulator.
	// Restricting even a pending flag allocated elsewhere is conservative.
	if g.pendingAccReg != NoReg && live[g.pendingAccReg] {
		return false
	}
	if g.pendingFlagReg != NoReg && live[g.pendingFlagReg] {
		return false
	}
	// Original SBC setup necessarily overwrites HL, and overwrites DE when
	// staging either operand out of HL. Only use this experiment when at least
	// one of those pairs holds a future-live value and needs preservation.
	written := map[string]bool{"HL": true}
	if lhs != "HL" && rhs == "HL" {
		written["DE"] = true
	}
	needsSave := false
	for r := range live {
		if r == inst.Dst {
			continue
		}
		loc := g.loc(r)
		for pair := range written {
			if loc == pair || loc == highByte(pair) || loc == lowByte(pair) {
				needsSave = true
			}
		}
	}
	if !needsSave {
		return false
	}
	idx := g.trampIdx
	g.trampIdx++
	done := fmt.Sprintf(".ucmp_pair_done_%d", idx)
	g.emitf("    LD A, %s", highByte(lhs))
	g.emitf("    CP %s", highByte(rhs))
	g.emitf("    JR NZ, %s", done)
	g.emitf("    LD A, %s", lowByte(lhs))
	g.emitf("    CP %s", lowByte(rhs))
	g.emitf("%s:", done)
	g.invalidate("A")
	g.pendingAccReg = NoReg
	g.pendingFlagReg = inst.Dst
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
	delete(g.cmpSwapped, inst.Dst)
	delete(g.cmpNeedsTwo, inst.Dst)
	return true
}
