package mir2

import (
	"fmt"
)

// emitFlagPredicateByte writes a canonical 0/1 bool to A from the current
// condition flags, including two-test predicates (C or Z, NC and NZ).
func (g *z80cg) emitFlagPredicateByte(cc string) {
	idx := g.trampIdx
	g.trampIdx++
	trueLabel := fmt.Sprintf(".bool_true_%d", idx)
	falseLabel := fmt.Sprintf(".bool_false_%d", idx)
	doneLabel := fmt.Sprintf(".bool_done_%d", idx)
	switch cc {
	case "CLE":
		g.emitf("    JP C, %s", trueLabel)
		g.emitf("    JP Z, %s", trueLabel)
	case "CGT":
		g.emitf("    JP Z, %s", falseLabel)
		g.emitf("    JP C, %s", falseLabel)
		g.emitf("    JP %s", trueLabel)
	default:
		g.emitf("    JP %s, %s", cc, trueLabel)
	}
	g.emitf("%s:", falseLabel)
	g.emit("    LD A, 0")
	g.emitf("    JP %s", doneLabel)
	g.emitf("%s:", trueLabel)
	g.emit("    LD A, 1")
	g.emitf("%s:", doneLabel)
	g.invalidate("A")
}

// ── Compare ───────────────────────────────────────────────────────────────────

// flagStillNeeded reports whether pendingFlagReg is used as a source
// in any instruction AFTER upcomingInst in the current block.
func (g *z80cg) flagStillNeeded(upcomingInst *Inst) bool {
	if g.curBlock == nil {
		return false
	}
	seen := false
	for _, inst := range g.curBlock.Insts {
		if inst == upcomingInst {
			seen = true
			continue
		}
		if !seen {
			continue
		}
		for _, s := range inst.Src {
			if s == g.pendingFlagReg {
				return true
			}
		}
	}
	return false
}

// accStillNeeded reports whether pendingAccReg is used as a source
// in any instruction AFTER upcomingInst in the current block, or by
// the block's terminal (e.g. passed as a block arg to a successor).
func (g *z80cg) accStillNeeded(upcomingInst *Inst) bool {
	if g.curBlock == nil {
		return false
	}
	seen := false
	for _, inst := range g.curBlock.Insts {
		if inst == upcomingInst {
			seen = true
			continue
		}
		if !seen {
			continue
		}
		for _, s := range inst.Src {
			if s == g.pendingAccReg {
				return true
			}
		}
	}
	// Also check the block terminal — block params are passed via term args
	// (e.g. TermBrIf passes acc through to the loop body block).
	if g.curBlock.Term != nil {
		for _, r := range g.curBlock.Term.termUses() {
			if r == g.pendingAccReg {
				return true
			}
		}
	}
	return false
}

// pickScratch8 returns an 8-bit scratch register that is not currently
// occupied by an operand, destination or value live after upcomingInst.
// Prefers E, H, L (unlikely to hold params) before D, B, C.
// Always excludes "A" and "F".
func (g *z80cg) pickScratch8(upcomingInst *Inst) string {
	used := map[string]bool{"A": true, "F": true}
	mark := func(loc string) {
		used[loc] = true
		if isPairReg(loc) || isIXY(loc) {
			used[highByte(loc)] = true
			used[lowByte(loc)] = true
		}
	}
	for r := range g.regsLiveAfterInst(upcomingInst) {
		mark(g.loc(r))
	}
	for _, r := range upcomingInst.Uses() {
		mark(g.loc(r))
	}
	mark(g.loc(upcomingInst.Dst))
	for _, loc := range g.physOverride {
		mark(loc)
	}
	for _, r := range []string{"E", "H", "L", "D", "B", "C", "IYH", "IYL", "IXH", "IXL"} {
		if !used[r] {
			return r
		}
	}
	return "D" // existing fallback for register-exhausted blocks
}

// materializePendingFlag checks whether the most recent ClassFlag result
// (pendingFlagReg) is still needed after upcomingInst. If so, it emits:
//
//	SBC A, A      ; A = 0xFF if condition-true, 0x00 if condition-false
//	LD  <scratch>, A
//
// where <scratch> is a register that is not currently holding any live value,
// and reroutes pendingFlagReg via physOverride so subsequent genBinOp reads
// it from there rather than from the (now-clobbered) carry flag.
func (g *z80cg) materializePendingFlag(upcomingInst *Inst) {
	if g.pendingFlagReg == NoReg {
		return
	}
	if !g.flagStillNeeded(upcomingInst) {
		g.pendingFlagReg = NoReg
		return
	}
	scratch := g.pickScratch8(upcomingInst)
	// Materialize carry → 0xFF (true) or 0x00 (false) via SBC A, A.
	g.emit("    SBC A, A")
	g.invalidate("A")
	g.emitf("    LD %s, A", scratch)
	g.physOverride[g.pendingFlagReg] = scratch
	g.pendingFlagReg = NoReg
}

// materializePendingAcc checks whether pendingAccReg (a ClassAcc value
// currently in A) is still needed after upcomingInst. If so, it saves A
// to a scratch register that is not currently holding any live value,
// and reroutes pendingAccReg via physOverride so the upcoming "LD A, imm"
// in genCmp does not clobber the live value.
func (g *z80cg) materializePendingAcc(upcomingInst *Inst) {
	if g.pendingAccReg == NoReg {
		return
	}
	if !g.accStillNeeded(upcomingInst) {
		g.pendingAccReg = NoReg
		return
	}
	scratch := g.pickScratch8(upcomingInst)
	g.emitf("    LD %s, A    ; materialize r%d (pendingAcc)", scratch, g.pendingAccReg)
	g.physOverride[g.pendingAccReg] = scratch
	g.pendingAccReg = NoReg
}

// saveAccForCondRet saves A to a scratch register if a CMP is about to load a
// different vreg into A and there is a live vreg in A that the terminator or a
// successor block still needs.
//
// Originally this only covered TermCondRet (abs_val fix). Expanded to also
// cover TermBrIf: gcd(a,b) has CmpEq(b,0) that loads b into A, clobbering a
// which the then-branch returns.
func (g *z80cg) saveAccForCondRet(cmpInst *Inst) {
	if g.fn == nil || g.curBlock == nil {
		return
	}
	term := g.curBlock.Term
	if term == nil {
		return
	}

	// Check if CMP will load a different vreg into A (clobbering current A value).
	lhs := g.loc(cmpInst.Src[0])
	if lhs == "A" {
		return // lhs already in A, no clobber
	}

	// Collect all vregs that the terminator and successor blocks need.
	// If any of them is currently in A, save A before the CMP clobbers it.
	liveInA := g.findVregInA()
	if liveInA == NoReg {
		return // nothing in A worth saving
	}

	needed := false
	switch t := term.(type) {
	case *TermCondRet:
		for _, v := range t.Vals {
			if v == liveInA {
				needed = true
				break
			}
		}
		if !needed {
			// Check if the Then successor needs it
			needed = g.vregUsedInBlock(liveInA, t.Then)
		}
	case *TermBrIf:
		// Check if either successor uses the vreg in A
		needed = g.vregUsedInBlock(liveInA, t.Then) || g.vregUsedInBlock(liveInA, t.Else)
	case *TermBrIf2:
		needed = g.vregUsedInBlock(liveInA, t.Eq) ||
			g.vregUsedInBlock(liveInA, t.Lt) ||
			g.vregUsedInBlock(liveInA, t.Gt)
	}

	if needed {
		scratch := g.pickScratch8(cmpInst)
		g.emitf("    LD %s, A    ; save ret val before CMP", scratch)
		g.physOverride[liveInA] = scratch
	}
}

// findVregInA returns the vreg currently mapped to A, or NoReg if none.
func (g *z80cg) findVregInA() Reg {
	if g.fn == nil {
		return NoReg
	}
	// Check block params first (they're live-through the block)
	for _, bp := range g.curBlock.Params {
		if g.loc(bp.Dst) == "A" {
			return bp.Dst
		}
	}
	// Check function params that might still be in A
	for _, cp := range g.fn.Contract.Params {
		if g.loc(cp.Reg) == "A" {
			return cp.Reg
		}
	}
	return NoReg
}

// vregUsedInBlock checks if vreg is used in the instructions or terminator
// of the block with the given label.
func (g *z80cg) vregUsedInBlock(vreg Reg, label string) bool {
	blk := g.fn.BlockByLabel(label)
	if blk == nil {
		return false
	}
	for _, inst := range blk.Insts {
		for _, r := range inst.Uses() {
			if r == vreg {
				return true
			}
		}
	}
	if blk.Term != nil {
		for _, r := range blk.Term.termUses() {
			if r == vreg {
				return true
			}
		}
	}
	return false
}

func (g *z80cg) genCmp(inst *Inst) {
	// CmpSubCarry / CmpSubCarryNot: carry flag already set by the immediately
	// preceding SUB.  No instruction needed — carry encodes a < b (C) or a >= b (NC).
	if inst.Cond == CmpSubCarry || inst.Cond == CmpSubCarryNot {
		return
	}

	// Before emitting any flag-clobbering comparison instruction:
	// 1. Materialize any live ClassFlag register whose carry would be overwritten.
	// 2. Save any live ClassAcc register whose A value would be overwritten by
	//    the "LD A, imm" used to set up the CP operand.
	// 3. Save A if the block ends with TermCondRet and A holds a return value
	//    that would be clobbered by loading the CMP operand (ADR-0041 fix).
	// Both are saved to "D" (scratch) via physOverride; the OR-from-flag handler
	// in genBinOp will then read them from "D" instead of from "F"/"A".
	g.materializePendingFlag(inst)
	g.materializePendingAcc(inst)
	g.saveAccForCondRet(inst)

	lhs := g.loc(inst.Src[0])
	rhs := g.loc(inst.Src[1])

	// 32-bit DWord comparison: must dispatch before isPairReg (DWord locs also
	// have pair names like "HL", so they'd otherwise fall into genCmp16).
	if g.isDWord(inst.Src[0]) || g.isDWord(inst.Src[1]) {
		g.genCmp32(inst)
		return
	}

	if pat, ok := g.bitCmpPat[inst.Dst]; ok {
		if pat.useMem {
			ptr := g.loc(pat.ptrReg)
			if isSpill(ptr) {
				g.emitf("    LD HL, (%s)   ; reload spilled ptr", ptr)
				g.invalidate("HL")
				ptr = "HL"
			}
			if g.emitBitMemOp("BIT", pat.bit, ptr, pat.byteOffset) {
				g.pendingFlagReg = inst.Dst
				return
			}
		} else {
			src := g.loc(pat.srcReg)
			src = selectBitReg(src, pat.byteOffset)
			if isSimpleReg(src) && !isSpill(src) && !isIXYReg(src) {
				g.emitf("    BIT %d, %s", pat.bit, src)
				g.pendingFlagReg = inst.Dst
				return
			}
		}
	}

	// 16-bit comparison: one or both operands are register pairs (HL/DE/BC/…).
	// Z80 only supports SBC HL, rr for 16-bit flag-setting subtraction.
	cv, immediate := g.constVals[inst.Src[1]]
	byteImmediate := immediate && cv >= 0 && cv <= 255 && !isPairReg(lhs) && (isSimpleReg(lhs) || isIXYReg(lhs)) && lhs != "F" && (inst.SrcTy == nil || inst.SrcTy.Width() <= 8)
	if isPairReg(lhs) || (isPairReg(rhs) && !byteImmediate) {
		g.genCmp16(inst)
		return
	}

	// CmpGt / CmpUgt and CmpLe / CmpUle are not directly expressible as a
	// single Z80 flag after a normal CP (which would need NC+NZ or C+Z).
	// Fix: force-swap the operands.
	//   CmpGt(a,b): a>b ↔ b<a → put b in A, CP a → C flag single-condition ✓
	//   CmpLe(a,b): a≤b ↔ b≥a → put b in A, CP a → NC flag single-condition ✓
	// Exception: rhs==0 is handled by the AND A path below (cmpAndZero mechanism
	// already returns NZ for Gt and Z for Le in condCode, so no swap needed).
	isGtOrLe := inst.Cond == CmpGt || inst.Cond == CmpUgt ||
		inst.Cond == CmpLe || inst.Cond == CmpUle
	if isGtOrLe {
		rhsIsZero := false
		if cv, ok := g.constVals[inst.Src[1]]; ok && cv == 0 && !isSignedOrdering(inst) {
			rhsIsZero = true
		}
		if !rhsIsZero {
			if lhs != "A" {
				// Force-swap: put rhs in A, emit CP lhs → single condition.
				//   CmpGt(a,b) → b<a → C flag ✓
				//   CmpLe(a,b) → b≥a → NC flag ✓
				if cv, ok := g.constVals[inst.Src[1]]; ok {
					g.emitf("    LD A, %d", cv)
					g.invalidate("A")
				} else if !g.holdsValue("A", rhs) {
					g.emitLDA(rhs)
				}
				g.emit8ALU("CP", lhs)
				g.normalizeSignedCmp(inst)
				g.cmpSwapped[inst.Dst] = true
				g.pendingFlagReg = inst.Dst
				return
			}
			// lhs IS "A": force-swap would clobber the live value in A.
			// Emit normally (A=lhs already, CP rhs); TermBrIf will use two JPs.
			g.cmpNeedsTwo[inst.Dst] = true
			// Fall through to normal CP code (which will find A=lhs and emit CP rhs).
		}
		// rhsIsZero: fall through to AND A path (cmpAndZero handles NZ/Z).
	}

	// Z80 CP instruction: computes A − operand and sets flags.
	// We need A = lhs, then CP rhs.
	//
	// Peephole: if rhs is a known constant, use CP imm8 directly.
	// This avoids an extra register load (e.g. LD C,0 / LD A,B / CP C
	// → LD A,B / CP 0), and the constant no longer needs to be
	// materialised into a register (eliminating a dead store).
	if cv, ok := g.constVals[inst.Src[1]]; ok {
		if !g.holdsValue("A", lhs) {
			g.emitLDA(lhs)
		}
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		if cv == 0 && !isSignedOrdering(inst) {
			g.emit("    AND A") // AND A ≡ CP 0 for all flags; 1B/4T vs 2B/7T
			g.cmpAndZero[inst.Dst] = true
		} else {
			g.emitf("    CP %d", cv&0xFF)
		}
		g.normalizeSignedCmp(inst)
		// CP/AND A does not modify A; aliases remain valid.
		g.pendingFlagReg = inst.Dst
		return
	}

	// Two-address conflict: if rhs is already in A and lhs is elsewhere,
	// loading lhs into A would destroy rhs.
	// Fix: swap operands (emit CP lhs with A=rhs) and mark as swapped.
	// condCode() will invert the CmpCond to compensate.
	if rhs == "A" && lhs != "A" {
		// A already holds rhs. CP lhs computes A−lhs = rhs−lhs.
		// This is the swapped comparison; condCode will invert it.
		// For CmpLt/CmpGe (and unsigned variants), the equality case (A==lhs)
		// is not captured by a single carry flag after the reversal, so we need
		// two jumps (CGT/CLE) in TermBrIf.
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.emit8ALU("CP", lhs)
		g.normalizeSignedCmp(inst)
		g.cmpSwapped[inst.Dst] = true
		swappedCond := inst.Cond.Swap()
		if swappedCond == CmpGt || swappedCond == CmpUgt ||
			swappedCond == CmpLe || swappedCond == CmpUle {
			g.cmpNeedsTwo[inst.Dst] = true
		}
		// CP does not modify A; aliases remain valid.
		g.pendingFlagReg = inst.Dst
		return
	}

	// Sub+Cmp flag fusion: if the immediately preceding instruction was
	// SUB with the same operands, the carry flag is already set correctly.
	//   CmpLt/CmpUlt  → C  (borrow from SUB)
	//   CmpGe/CmpUge  → NC (no borrow from SUB)
	// CRITICAL: check BEFORE loading lhs into A, because if r = Sub(a,b)
	// lives in A and lhs is somewhere else (e.g. B), emitLDA would overwrite
	// r.  The IAR abs_diff pattern depends on this: r must survive to the
	// return path after the comparison.
	subCmpFused := ((inst.Cond == CmpUlt || inst.Cond == CmpUge) ||
		((inst.Cond == CmpLt || inst.Cond == CmpGe) && !isSignedOrdering(inst))) &&
		g.lastFlagsLhs == lhs && g.lastFlagsRhs == rhs
	if subCmpFused {
		// Flags valid from preceding SUB — skip both LD A and CP.
		return
	}

	// Special case: rhs is currently in A but lhs is not.
	// We cannot emit "LD A, lhs; CP A" (overwrites rhs before compare).
	// Instead keep A=rhs and emit "CP lhs" — the comparison is reversed.
	// Record the swap so condCode inverts the condition.  The swapped condition
	// may require two jumps (CmpGt/CmpLe) to handle the equality case.
	if rhs == "A" && !g.holdsValue("A", lhs) {
		g.emit8ALU("CP", lhs)
		g.normalizeSignedCmp(inst)
		g.cmpSwapped[inst.Dst] = true
		swappedCond := inst.Cond.Swap()
		if swappedCond == CmpGt || swappedCond == CmpUgt ||
			swappedCond == CmpLe || swappedCond == CmpUle {
			g.cmpNeedsTwo[inst.Dst] = true
		}
		g.pendingFlagReg = inst.Dst
		return
	}

	// Coalescing: A already holds lhs — skip LD A, lhs.
	if !g.holdsValue("A", lhs) {
		g.emitLDA(lhs)
	}
	g.lastFlagsLhs = ""
	g.lastFlagsRhs = ""
	g.emit8ALU("CP", rhs)
	g.normalizeSignedCmp(inst)
	// CP does not modify A; aliases remain valid.
	g.pendingFlagReg = inst.Dst
}

func isSignedOrdering(inst *Inst) bool {
	if inst.SrcTy == nil || !IsSigned(inst.SrcTy) {
		return false
	}
	c := inst.Cond
	return c == CmpLt || c == CmpLe || c == CmpGt || c == CmpGe
}

// Z80 CP/SBC sets S and P/V for signed subtraction, but its carry is an
// unsigned borrow. Convert S xor V to carry while preserving Z (needed for
// <= and >). SCF/CCF preserve S, Z and P/V and do not change A or HL.
func (g *z80cg) normalizeSignedCmp(inst *Inst) {
	if !isSignedOrdering(inst) {
		return
	}
	idx := g.trampIdx
	g.trampIdx++
	g.emitf("    JP M, .scmp_neg_%d", idx)
	g.emitf("    JP PE, .scmp_true_%d", idx)
	g.emitf("    JP .scmp_false_%d", idx)
	g.emitf(".scmp_neg_%d:", idx)
	g.emitf("    JP PO, .scmp_true_%d", idx)
	g.emitf(".scmp_false_%d:", idx)
	g.emit("    SCF")
	g.emit("    CCF")
	g.emitf("    JP .scmp_done_%d", idx)
	g.emitf(".scmp_true_%d:", idx)
	g.emit("    SCF")
	g.emitf(".scmp_done_%d:", idx)
}

// condCode returns the Z80 condition code for the virtual register holding
// the comparison result.  If genCmp swapped the operands, the condition
// is automatically inverted via CmpCond.Swap().
//
// For ClassFlag returns from CALL instructions, callFlags[cond] holds the
// agreed FlagCond directly — no AND A / CP 0 is needed at the call site.
func (g *z80cg) condCode(f *Func, cond Reg) string {
	// Fast path: call result with ClassFlag return — condition already in F.
	if fc, ok := g.callFlags[cond]; ok {
		return cmpCondCode(fc)
	}

	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst == cond && inst.Op == OpCmp {
				c := inst.Cond
				if g.cmpSwapped[cond] {
					c = c.Swap() // operands were swapped → invert condition
				}
				// When genCmp emitted AND A (rhs=0), carry is always 0.
				// Replace NC/C-based conditions with NZ/Z-based ones:
				//   CmpGt(n, 0) → n != 0 → NZ
				//   CmpGe(n, 0) → n >= 0 → always true → handled as NC (fine)
				//   CmpLt(n, 0) → never for unsigned → C (never fires)
				//   CmpLe(n, 0) → n == 0 → Z
				if g.cmpAndZero[cond] {
					switch c {
					case CmpGt, CmpUgt:
						return "NZ"
					case CmpLe, CmpUle:
						return "Z"
					}
				}
				if g.cmpNeedsTwo[cond] {
					switch c {
					case CmpGt, CmpUgt:
						return "CGT" // NC && NZ: JP Z,else; JP NC,then
					case CmpLe, CmpUle:
						return "CLE" // C || Z: JP C,then; JP Z,then
					}
				}
				return cmpCondCode(c)
			}
		}
	}
	return "NZ" // fallback
}

// invertCC returns the Z80 condition code that is the logical negation of cc.
func invertCC(cc string) string {
	switch cc {
	case "Z":
		return "NZ"
	case "NZ":
		return "Z"
	case "C":
		return "NC"
	case "NC":
		return "C"
	case "PE":
		return "PO"
	case "PO":
		return "PE"
	case "M":
		return "P"
	case "P":
		return "M"
	}
	return cc
}

// cmpCondCode maps CmpCond → Z80 condition string.
func cmpCondCode(c CmpCond) string {
	switch c {
	case CmpEq:
		return "Z"
	case CmpNe:
		return "NZ"
	case CmpLt, CmpUlt, CmpSubCarry:
		return "C"
	case CmpGe, CmpUge, CmpSubCarryNot:
		return "NC"
	case CmpLe, CmpUle:
		// LE: JP Z / JP C — requires two jumps; emit Z for now (backend TODO).
		return "Z"
	case CmpGt, CmpUgt:
		return "NC"
	}
	return "NZ"
}

// genCmp16 emits a 16-bit comparison using SBC HL, rr.
//
// Z80 only provides SBC HL, rr for 16-bit arithmetic that sets flags.
// The sequence is:
//
//	OR A           ; clear carry (preserves A; sets S/Z/H/PV from A)
//	SBC HL, rr     ; HL = lhs − rhs − 0; sets C (borrow) and Z (equal)
//
// SBC is destructive (clobbers HL).  We save/restore HL with PUSH/POP so
// that lhs remains available for subsequent uses (e.g. the "return a" branch
// of min16).
//
// Conditions after SBC HL, rr (unsigned):
//
//	C flag set   → lhs < rhs (CmpUlt / CmpUge after negation)
//	Z flag set   → lhs == rhs
//
// Signed comparisons with an explicit signed SrcTy normalize S⊕V to carry
// after restoring HL, so the branch/materialization paths share one contract.
func (g *z80cg) genCmp16(inst *Inst) {
	lhs := g.loc(inst.Src[0])
	rhs := g.loc(inst.Src[1])
	defer g.saveWrittenScratch(inst)()

	// CmpGt/CmpLe traditionally swap operands so SBC HL,rr gives the C flag
	// directly.  However, when lhs=HL and rhs=DE (common case), the swap
	// forces EX DE,HL before+after SBC — 2 extra bytes, 16 extra T-states.
	//
	// Optimisation: when lhs is already in HL, do NOT swap; compute lhs-rhs
	// directly and use a two-jump pattern (CGT/CLE) at the branch site.
	// Cost: same bytes in branch, but saves the two EX DE,HL around the SBC.
	isGtOrLe := inst.Cond == CmpGt || inst.Cond == CmpUgt ||
		inst.Cond == CmpLe || inst.Cond == CmpUle
	if isGtOrLe {
		if lhs == "HL" && isPairReg(rhs) {
			// lhs already in HL — skip swap, use CGT/CLE two-jump pattern.
			g.cmpNeedsTwo[inst.Dst] = true
			// Do NOT set cmpSwapped — we keep original operand order.
		} else {
			// Fallback: swap operands as before.
			lhs, rhs = rhs, lhs
			g.cmpSwapped[inst.Dst] = true
		}
	}

	// A byte lhs must not be extended into HL while rhs still lives there.
	// Stage both values before changing either register (also covers D/E).
	if rhs == "HL" && (isIXYReg(lhs) || (isSimpleReg(lhs) && !isPairReg(lhs) && lhs != "F")) {
		g.pushWord(lhs)
		g.pushWord(rhs)
		g.emit("    POP DE")
		g.emit("    POP HL")
		g.invalidate("DE")
		g.invalidate("HL")
		lhs, rhs = "HL", "DE"
	}
	// Guard: materialise F or 8-bit operands to proper 16-bit regs.
	if lhs == "F" {
		g.emit("    SBC A, A") // materialise flag to A
		g.emit("    LD L, A")
		g.emit("    LD H, 0")
		g.invalidate("A")
		g.invalidate("HL")
		lhs = "HL"
	} else if isIXYReg(lhs) {
		// IXH/IXL/IYH/IYL → zero-extend to HL via shadow A.
		g.emitMovViaAltA("L", lhs)
		g.emit("    LD H, 0")
		g.invalidate("HL")
		lhs = "HL"
	} else if isSimpleReg(lhs) && !isPairReg(lhs) {
		// 8-bit reg → zero-extend to HL
		g.emitLD8("L", lhs)
		g.emit("    LD H, 0")
		g.invalidate("HL")
		lhs = "HL"
	}
	if rhs == "F" || isIXYReg(rhs) || (isSimpleReg(rhs) && !isPairReg(rhs)) {
		// Already handled below in the SBC guard, but mark for safety.
	}

	// SBC HL, rr requires lhs in HL.  If lhs is already HL we proceed.
	// If rhs is HL and lhs is DE, EX DE,HL is the cheapest fix.
	// Otherwise move lhs byte-by-byte into HL.
	origLhs, origRhs := lhs, rhs
	if lhs != "HL" {
		if lhs == "DE" && rhs == "HL" {
			// Swap via EX DE,HL: after EX, HL=old_DE=lhs, DE=old_HL=rhs.
			g.emit("    EX DE, HL")
			g.invalidate("HL")
			g.invalidate("DE")
			lhs, rhs = "HL", "DE"
		} else if rhs == "HL" {
			// rhs is HL, lhs is BC or IX/IY.  Swap: save rhs(HL)→DE, load lhs→HL.
			g.emitf("    LD D, H") // save rhs high into D
			g.emitf("    LD E, L") // save rhs low  into E
			if isIXY(lhs) {
				// IX→HL: byte-copy invalid. Use PUSH/POP.
				g.emitf("    PUSH %s", lhs)
				g.emit("    POP HL")
			} else if isIXYReg(lhs) || (isSimpleReg(lhs) && !isPairReg(lhs)) {
				// 8-bit register: zero-extend to HL.
				g.emitMovViaAltA("L", lhs)
				g.emit("    LD H, 0")
			} else if isSpill(lhs) {
				g.loadSpill16("HL", lhs)
			} else {
				g.emitf("    LD H, %s", highByte(lhs))
				g.emitLD8("L", lowByte(lhs))
			}
			g.invalidate("HL")
			g.invalidate("DE")
			lhs, rhs = "HL", "DE"
		} else if isIXY(lhs) {
			// IX/IY → HL: byte-copy invalid (DD prefix conflict). Use PUSH/POP.
			g.emitf("    PUSH %s", lhs)
			g.emit("    POP HL")
			g.invalidate("HL")
			lhs = "HL"
		} else if isIXYReg(lhs) || (isSimpleReg(lhs) && !isPairReg(lhs)) {
			// 8-bit register (including IXH/IXL): zero-extend into HL.
			g.emitMovViaAltA("L", lhs)
			g.emit("    LD H, 0")
			g.invalidate("HL")
		} else if isSpill(lhs) {
			g.loadSpill16("HL", lhs)
			g.invalidate("HL")
		} else {
			// Neither operand is HL; move lhs into HL byte-by-byte.
			g.emitf("    LD H, %s", highByte(lhs))
			g.emitLD8("L", lowByte(lhs))
			g.invalidate("HL")
			lhs = "HL"
		}
	}

	// Restore the original operand orientation after SBC. The wrapper
	// restores any live values overwritten by staging or subtraction.

	swappedDE := origRhs == "HL" && origLhs != "HL"

	// SBC HL, rr requires rhs to be a 16-bit register pair (BC/DE/HL/SP).
	// Guard against invalid operands: F, single 8-bit regs, IXY halves.
	if rhs == "F" {
		// Flag result used as 16-bit comparison operand — materialise to BC.
		g.emit("    SBC A, A") // A = 0xFF if carry, 0x00 if not
		g.emit("    LD C, A")
		g.emit("    LD B, 0")
		g.invalidate("A")
		rhs = "BC"
	} else if isIXYReg(rhs) {
		// IXH/IXL/IYH/IYL: load into BC via A.
		g.emitf("    LD A, %s", rhs)
		g.emit("    LD C, A")
		g.emit("    LD B, 0")
		g.invalidate("A")
		rhs = "BC"
	} else if isSimpleReg(rhs) && !isPairReg(rhs) {
		// 8-bit register: promote to pair (handles A→BC, others→parentPair).
		rhs = g.promote8toPair(rhs)
	} else if isIXY(rhs) {
		// IX/IY: push to HL would conflict. Push rhs → stack, pop to BC.
		g.emitf("    PUSH %s", rhs)
		g.emit("    POP BC")
		rhs = "BC"
	} else if isSpill(rhs) {
		// LocMem spill: SBC HL,$F0xx is not valid. Load to BC first.
		g.loadSpill16("BC", rhs)
		rhs = "BC"
	}

	// The selected-sequence wrapper preserves HL only when its original
	// value is live. POP restores it without disturbing comparison flags.
	g.emit("    OR A")
	g.emitSBCHL(rhs)

	if swappedDE {
		// EX leaves the comparison flags intact; live originals are restored
		// by the wrapper after signed-flag normalization.

		g.emit("    EX DE, HL")
		g.invalidate("HL")
		g.invalidate("DE")
	} else {
		g.invalidate("HL")
	}
	g.normalizeSignedCmp(inst)
	g.pendingFlagReg = inst.Dst
}

// genCmp32 emits a non-destructive 32-bit unsigned comparison (lhs vs rhs)
// via a PUSH/SBC-chain/POP pattern; carry is set if lhs < rhs, NC if lhs >= rhs.
//
// Strategy (lhs=HL+H'L', rhs=DE+D'E'):
//
//	PUSH HL          ; save a_lo  (11T)
//	EXX
//	PUSH HL          ; save a_hi  (11T, HL = H'L' after EXX)
//	EXX
//	AND A            ; clear carry (4T)
//	SBC HL, DE       ; a_lo - b_lo; carry = lo borrow (15T)
//	EXX
//	SBC HL, DE       ; a_hi - b_hi - lo_carry; carry = 32-bit borrow (15T)
//	EXX
//	EXX              ; switch to restore a_hi
//	POP HL           ; restore H'L' (10T); carry preserved by POP rr
//	EXX
//	POP HL           ; restore HL  (10T); carry preserved
//
// Total: 91T.  For the typical abs_diff pattern this is followed immediately by
// BrIf which branches on carry — no intermediate store of the flag needed.
func (g *z80cg) genCmp32(inst *Inst) {
	lhs := g.loc(inst.Src[0])
	rhs := g.loc(inst.Src[1])

	// CmpGt/CmpLe: swap operands so the carry encodes the right condition.
	//   CmpUgt(a,b): a>b ↔ b<a → swap, then carry = (b<a) ✓
	//   CmpUle(a,b): a≤b ↔ b≥a → swap, then NC = (b≥a) ✓
	isGtOrLe := inst.Cond == CmpGt || inst.Cond == CmpUgt ||
		inst.Cond == CmpLe || inst.Cond == CmpUle
	if isGtOrLe {
		lhs, rhs = rhs, lhs
		g.cmpSwapped[inst.Dst] = true
	}

	// Z80 SBC requires lhs in HL.  The allocator biases ClassDWord toward HL (cost 0),
	// so lhs=HL is the common case.
	if lhs != "HL" {
		g.comment(fmt.Sprintf("TODO: genCmp32 lhs=%s not HL (rhs=%s)", lhs, rhs))
		return
	}

	// Non-destructive 32-bit comparison (lhs preserved by PUSH/POP):
	g.emit("    PUSH HL") // save a_lo (main HL)
	g.emit("    EXX")
	g.emit("    PUSH HL") // save a_hi (shadow HL = H'L')
	g.emit("    EXX")
	g.emit("    AND A") // clear carry
	g.emitSBCHL(rhs)    // a_lo - b_lo; carry = lo borrow
	g.emit("    EXX")
	g.emitSBCHL(rhs) // a_hi - b_hi - lo_carry; carry = 32-bit borrow
	g.emit("    EXX")
	// Restore a (carry preserved by POP rr — POP does not affect flags):
	g.emit("    EXX")
	g.emit("    POP HL") // restore H'L' (a_hi)
	g.emit("    EXX")
	g.emit("    POP HL") // restore HL   (a_lo)
	g.invalidate("HL")   // HL was clobbered during SBC, now restored; flush cache
}

// Comparisons can encode their result in Z, carry, or a two-flag predicate.
// A scalar copy must use that predicate, and must preserve a live A when
// writing another register (e.g. a short-circuit condition's block argument).
func (g *z80cg) emitKnownFlagCopy(dst string, width int) bool {
	if g.pendingFlagReg == NoReg {
		return false
	}
	pair := "BC"
	if dst == "BC" || dst == "B" || dst == "C" {
		pair = "HL"
	}
	if dst != "A" {
		g.emitf("    PUSH %s", pair)
		g.emit("    PUSH AF")
	}
	g.emitFlagPredicateByte(g.condCode(g.fn, g.pendingFlagReg))
	g.emitMov(dst, "A", width)
	if dst != "A" {
		g.emitf("    POP %s", pair)
		g.emitf("    LD A, %s", highByte(pair))
		g.emitf("    POP %s", pair)
		g.invalidate(pair)
	}
	g.invalidate("A")
	g.invalidate(dst)
	return true
}
