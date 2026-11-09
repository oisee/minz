package mir2

import (
	"strings"

	"github.com/minz/minzc/pkg/z80effects"
)

// scanAddCarryPattern finds the unsigned-overflow test of a 16-bit add:
//
//	%s = add %x, %y : u16
//	%c = cmp.ult %s, %x      (or %y; also cmp.uge, and cmp.ugt/ule %x, %s)
//	br_if %c, ...
//
// with the add and the cmp as the block's last two instructions. The carry of
// ADD HL/IX/IY, rr is exactly that predicate. Nothing is suppressed here: the
// add is published only if its emitted sequence ends its carry writes with such
// an ADD, and the cmp falls back to an ordinary comparison otherwise.
func (g *z80cg) scanAddCarryPattern(b *Block) {
	g.addCarryInst, g.addCarryCmp = nil, nil
	term, ok := b.Term.(*TermBrIf)
	if !ok || len(b.Insts) < 2 {
		return
	}
	add, cmp := b.Insts[len(b.Insts)-2], b.Insts[len(b.Insts)-1]
	if cmp.Op != OpCmp || cmp.Dst != term.Cond || add.Op != OpAdd || add.Dst == NoReg ||
		add.Ty == nil || add.Ty.Width() != 16 || len(add.Src) != 2 || len(cmp.Src) != 2 {
		return
	}
	if _, ok := addCarryCond(cmp, add); !ok {
		return
	}
	g.addCarryInst, g.addCarryCmp = add, cmp
}

// addCarryCond returns the branch condition for cmp read from the add's carry.
func addCarryCond(cmp, add *Inst) (string, bool) {
	isAddend := func(r Reg) bool { return r == add.Src[0] || r == add.Src[1] }
	switch {
	case cmp.Src[0] == add.Dst && isAddend(cmp.Src[1]):
		switch cmp.Cond {
		case CmpUlt:
			return "C", true
		case CmpUge:
			return "NC", true
		}
	case cmp.Src[1] == add.Dst && isAddend(cmp.Src[0]):
		switch cmp.Cond {
		case CmpUgt:
			return "C", true
		case CmpUle:
			return "NC", true
		}
	}
	return "", false
}

// publishAddCarry records the add as the carry source when the last carry
// writer it emitted is a 16-bit ADD into HL/IX/IY.
func (g *z80cg) publishAddCarry(inst *Inst, start int) {
	out := g.sb.String()
	if start > len(out) {
		return
	}
	last := ""
	for _, line := range strings.Split(out[start:], "\n") {
		e := z80effects.Instruction(line)
		if !e.Known {
			return
		}
		if e.FlagsWrite&1 != 0 {
			last = strings.Join(strings.Fields(strings.SplitN(line, ";", 2)[0]), " ")
		}
	}
	for _, dst := range []string{"HL", "IX", "IY"} {
		if strings.HasPrefix(last, "ADD "+dst+",") || strings.HasPrefix(last, "ADD "+dst+" ,") {
			g.carryFrom = inst
			return
		}
	}
}

// tryAddCarryCmp branches on the carry left by the fused add, if it survived.
func (g *z80cg) tryAddCarryCmp(inst *Inst) bool {
	if inst != g.addCarryCmp || g.addCarryInst == nil || g.carryFrom != g.addCarryInst {
		return false
	}
	cc, ok := addCarryCond(inst, g.addCarryInst)
	if !ok {
		return false
	}
	g.cmpBranchCC[inst.Dst] = cc
	g.pendingFlagReg = inst.Dst
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
	delete(g.cmpSwapped, inst.Dst)
	delete(g.cmpNeedsTwo, inst.Dst)
	return true
}
