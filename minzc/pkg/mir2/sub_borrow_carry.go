package mir2

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/z80effects"
)

// CmpSubCarry(r, b) and CmpSubCarryNot(r, b) read the borrow of r = sub(x, b)
// from the carry flag instead of comparing again. That is only sound while C
// holds that borrow. Not every emitted subtraction leaves it there: with b in
// A, "NEG / ADD A, x" computes x - b but its carry is (b != 0 && x >= b), and
// a small constant step (DEC) leaves C untouched. The carry-reading predicate
// therefore requires carryFrom to name the subtraction, and the subtraction
// publishes itself only when its emitted text ends in a borrow-producing
// carry writer.

// subBorrowRead reports whether a later instruction in the current block reads
// the borrow of sub through CmpSubCarry / CmpSubCarryNot.
func (g *z80cg) subBorrowRead(sub *Inst) bool {
	if g.curBlock == nil {
		return false
	}
	seen := false
	for _, x := range g.curBlock.Insts {
		if x == sub {
			seen = true
			continue
		}
		if seen && x.Op == OpCmp && (x.Cond == CmpSubCarry || x.Cond == CmpSubCarryNot) &&
			x.Src[0] == sub.Dst && x.Src[1] == sub.Src[1] {
			return true
		}
	}
	return false
}

// publishSubBorrow names sub as the producer of C when the code emitted for
// it since start ends in a carry writer that leaves the unsigned borrow of
// Src[0] - Src[1]: SUB, the borrow-exact "SCF / ADC A, x / CCF" form of the
// rhs-in-A case, or "OR A / SBC HL, rr" for words.
func (g *z80cg) publishSubBorrow(sub *Inst, start int) {
	out := g.sb.String()
	if start > len(out) {
		return
	}
	var writers []string
	for _, line := range strings.Split(out[start:], "\n") {
		e := z80effects.Instruction(line)
		if !e.Known {
			return
		}
		if e.FlagsWrite&1 != 0 {
			writers = append(writers, strings.ToUpper(strings.Join(strings.Fields(strings.SplitN(line, ";", 2)[0]), " ")))
		}
	}
	n := len(writers)
	if n == 0 {
		return
	}
	last := writers[n-1]
	ok := false
	switch {
	case sub.Ty.Width() <= 8 && strings.HasPrefix(last, "SUB "):
		ok = true
	case sub.Ty.Width() <= 8 && last == "CCF" && n >= 3 &&
		strings.HasPrefix(writers[n-2], "ADC A,") && writers[n-3] == "SCF":
		ok = true
	case sub.Ty.Width() == 16 && strings.HasPrefix(last, "SBC HL,") && n >= 2 &&
		(writers[n-2] == "OR A" || writers[n-2] == "AND A"):
		ok = true
	}
	if ok {
		g.carryFrom = sub
	}
}

// genSubCarryCmp emits the carry-reading predicate. When C still holds the
// borrow of the defining subtraction nothing is emitted. Otherwise a byte
// borrow is recomputed from r and b: x - b borrows exactly when
// r + b >= 256, that is when ~r < b, so with r in A "CPL / CP b / CPL" sets C
// to the borrow and leaves A unchanged.
func (g *z80cg) genSubCarryCmp(inst *Inst) {
	if c := g.carryFrom; c != nil && c.Op == OpSub && c.Dst == inst.Src[0] && c.Src[1] == inst.Src[1] {
		g.pendingFlagReg = inst.Dst
		return
	}
	r, b := g.loc(inst.Src[0]), g.loc(inst.Src[1])
	cv, bConst := g.constVals[inst.Src[1]]
	byteCmp := inst.SrcTy == nil || inst.SrcTy.Width() <= 8
	if byteCmp && g.holdsValue("A", r) && (bConst || (isSimpleReg(b) && b != "A" && b != "F" && !isPairReg(b))) {
		g.materializePendingFlag(inst)
		g.lastFlagsLhs, g.lastFlagsRhs = "", ""
		g.emit("    CPL")
		if bConst {
			g.emitf("    CP %d", cv&0xFF)
		} else {
			g.emit8ALU("CP", b)
		}
		g.emit("    CPL")
		g.pendingFlagReg = inst.Dst
		return
	}
	if g.err == nil {
		g.err = fmt.Errorf("z80 codegen: %s: r%d = %s(r%d, r%d): carry no longer holds the borrow of the subtraction (r%d in %s, r%d in %s)",
			g.fn.Name, inst.Dst, inst.Cond, inst.Src[0], inst.Src[1], inst.Src[0], r, inst.Src[1], b)
	}
}
