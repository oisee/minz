package mir2

import "strconv"

// tryBytewiseUnsignedCmp16 lowers a branch-only unsigned 16-bit ordering
// through A instead of SBC HL, rr:
//
//	LD A, lo(lhs) / SUB lo(rhs) / LD A, hi(lhs) / SBC A, hi(rhs)   ; 16T for registers
//
// The final borrow is exactly lhs < rhs. Neither operand pair is written, so
// the PUSH HL / OR A / SBC HL, rr / POP HL save (40T) disappears when lhs is
// live after the comparison. Z is not the equality flag here, so only
// predicates read from carry are accepted; Ugt/Ule swap their operands.
func (g *z80cg) tryBytewiseUnsignedCmp16(inst *Inst) bool {
	switch inst.Cond {
	case CmpUlt, CmpUge, CmpUgt, CmpUle:
	default:
		return false
	}
	if inst.SrcTy != nil && inst.SrcTy.Width() != 16 {
		return false
	}
	if inst.Dst == NoReg || !g.branchOnlyCmp(inst) {
		return false
	}
	lhsReg, rhsReg := inst.Src[0], inst.Src[1]
	swapped := inst.Cond == CmpUgt || inst.Cond == CmpUle
	if swapped {
		lhsReg, rhsReg = rhsReg, lhsReg
	}
	lhsLo, lhsHi, ok := g.bytewiseCmpOperand(lhsReg)
	if !ok {
		return false
	}
	rhsLo, rhsHi, ok := g.bytewiseCmpOperand(rhsReg)
	if !ok {
		return false
	}
	if _, c := g.constVals[lhsReg]; c {
		if _, c2 := g.constVals[rhsReg]; c2 {
			return false
		}
	}
	live := g.scratchLiveAfter(inst)
	for r := range live {
		if loc := g.loc(r); loc == "A" || loc == "AF" {
			return false
		}
	}
	if g.pendingAccReg != NoReg && live[g.pendingAccReg] {
		return false
	}
	if g.pendingFlagReg != NoReg && live[g.pendingFlagReg] {
		return false
	}
	g.emitf("    LD A, %s", lhsLo)
	g.emitf("    SUB %s", rhsLo)
	g.emitf("    LD A, %s", lhsHi)
	g.emitf("    SBC A, %s", rhsHi)
	g.invalidate("A")
	g.pendingAccReg = NoReg
	g.pendingFlagReg = inst.Dst
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
	if swapped {
		g.cmpSwapped[inst.Dst] = true
	} else {
		delete(g.cmpSwapped, inst.Dst)
	}
	delete(g.cmpNeedsTwo, inst.Dst)
	return true
}

// bytewiseCmpOperand names the low and high byte of a word operand as A-ALU
// operands: an immediate, a primary pair's halves, or IX/IY halves.
func (g *z80cg) bytewiseCmpOperand(r Reg) (lo, hi string, ok bool) {
	if cv, c := g.constVals[r]; c {
		v := uint16(cv)
		return strconv.Itoa(int(v & 255)), strconv.Itoa(int(v >> 8)), true
	}
	if info, known := g.regInfo[r]; !known || info.Ty == nil || info.Ty.Width() != 16 {
		return "", "", false
	}
	switch loc := g.loc(r); loc {
	case "BC", "DE", "HL", "IX", "IY":
		return lowByte(loc), highByte(loc), true
	}
	return "", "", false
}
