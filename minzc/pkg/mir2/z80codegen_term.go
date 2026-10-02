package mir2

import (
	"fmt"
	"sort"
)

// trampolineBlock is emitted after all regular blocks of a function.
// It resolves block-argument copies for one outgoing BrIf edge.
// If isRet is true, the block ends with RET instead of JRS target.
type trampolineBlock struct {
	label  string // local label for this trampoline
	copies []parallelCopy
	target string // real destination label (already formatted)
	isRet  bool   // true → emit RET instead of JRS target
}

// restoreLiveOutOverrides moves every relocated vreg that is live out of the
// current block, or read by its terminator, back to its allocated register
// before the terminator. A
// relocation (physOverride) is block-local: successors, and blocks laid out
// after this one, read the allocator's location. Without this, a value saved
// to a scratch register in one branch (gcd's else arm) was read from that
// scratch on paths that never wrote it.
func (g *z80cg) restoreLiveOutOverrides(f *Func, t Term) {
	if g.liveness == nil || len(g.physOverride) == 0 {
		return
	}
	liveOut := g.liveness.LiveOutOf(f, g.curBlock)
	if liveOut == nil {
		return
	}
	// Terminator operands count too: jmp @head(%r3, …) passes r3 to the param
	// that is r3 itself after coalescing, and the parallel copy skips it as an
	// identity move, so r3 must already sit in its allocated register.
	usedByTerm := map[Reg]bool{}
	for _, r := range t.termUses() {
		usedByTerm[r] = true
	}
	regs := make([]Reg, 0, len(g.physOverride))
	for r := range g.physOverride {
		if liveOut.Has(r) || usedByTerm[r] {
			regs = append(regs, r)
		}
	}
	sort.Slice(regs, func(i, j int) bool { return regs[i] < regs[j] })
	for _, r := range regs {
		scratch := g.physOverride[r]
		canonLoc := g.ar.Loc(r)
		delete(g.physOverride, r)
		if canonLoc.Kind != LocReg && canonLoc.Kind != LocIXY8 {
			continue
		}
		if canon := canonLoc.Name; canon != "" && canon != scratch {
			width := 8
			if canon == "HL" || canon == "DE" || canon == "BC" {
				width = 16
			}
			g.emitMov(canon, scratch, width)
			g.comment(fmt.Sprintf("restore live-out r%d from scratch", r))
		}
	}
}

// ── Terminators ───────────────────────────────────────────────────────────────

func (g *z80cg) genTerm(f *Func, t Term) {
	// If any block param was saved to a scratch register (via materializePendingAcc),
	// restore it to its canonical physical location before any terminal jump or ret.
	// This ensures live-through values (used in successor blocks without explicit
	// block params) are in their allocator-assigned locations when those blocks run.
	// physOverride[r] was valid within this block; canonical location = ar.Loc(r).
	//
	// EXCEPTION: For TermDJNZ, do NOT restore block params that are explicitly
	// passed as body-edge args (BodyArgs).  Those params receive their new values
	// from the parallel copy emitted by the DJNZ handler.  Restoring them from
	// scratch would overwrite the computed result with the stale initial value.
	djnzBodyArgs := map[Reg]bool{}
	if td, ok := t.(*TermDJNZ); ok {
		bodyBlock := f.BlockByLabel(td.Body)
		if bodyBlock != nil {
			for i, arg := range td.BodyArgs {
				if i+1 < len(bodyBlock.Params) {
					djnzBodyArgs[bodyBlock.Params[i+1].Dst] = true
					_ = arg // only need the param dst
				}
			}
		}
	}
	for _, bp := range g.curBlock.Params {
		if djnzBodyArgs[bp.Dst] {
			delete(g.physOverride, bp.Dst)
			continue
		}
		if scratch, ok := g.physOverride[bp.Dst]; ok {
			// Use ar.Loc (static allocation) not g.loc (which returns the override).
			canonLoc := g.ar.Loc(bp.Dst)
			delete(g.physOverride, bp.Dst)
			// Only restore register-backed block params — spilled params
			// (LocMem) are already in memory and weren't saved to a scratch GPR.
			if canonLoc.Kind == LocReg || canonLoc.Kind == LocIXY8 {
				canon := canonLoc.Name
				if canon != "" && canon != scratch {
					g.emitMov(canon, scratch, bp.Ty.Width())
					g.comment("restore block param from scratch")
				}
			}
		}
	}
	g.restoreLiveOutOverrides(f, t)

	switch t := t.(type) {
	case *TermRet:
		// Tail call: JP was already emitted by genCall; skip all moves and RET.
		if g.tailCallInst != nil {
			g.tailCallInst = nil
			return
		}
		// Move return values to their calling-convention physical registers using
		// parallel copy resolution.  Sequential moves are incorrect when two return
		// values share the same physical register after constant folding/PBQP
		// (e.g. both [acc]=A), causing write-after-write clobber.
		if len(t.Vals) == 1 && len(f.Contract.Returns) == 1 &&
			(f.Contract.Returns[0].Ty == TyBool || (f.Contract.Returns[0].Ty.Width() <= 8 && collectRegInfo(f)[t.Vals[0]].Ty == TyBool)) &&
			f.Contract.Returns[0].Class != ClassFlag && g.loc(t.Vals[0]) == "F" {
			g.emitFlagPredicateByte(g.condCode(f, t.Vals[0]))
			if dst := canonicalReturnLoc(f.Contract.Returns[0].Class, TyBool); dst != "A" {
				g.emitLD8(dst, "A")
			}
		} else {
			g.emitParallelCopy(g.buildReturnCopies(t.Vals))
		}
		// main() is the program entry point: use XOR A / DI / HALT instead of
		// RET so mze/mzx can detect program termination cleanly.  XOR A zeroes
		// A for a clean exit code 0.
		if f.Name == "main" && len(t.Vals) == 0 {
			g.emit("    XOR A")
			g.emit("    DI")
			g.emit("    HALT")
		} else {
			g.emit("    RET")
		}

	case *TermJmp:
		// Resolve block-argument parallel copies before the jump.
		copies := g.buildBlockCopies(f, t.Target, t.Args)
		g.emitParallelCopy(copies)
		// Fall-through elimination: if the target is the very next physical block,
		// the JR is redundant — execution falls through naturally.
		if !g.isFallThrough(f, t.Target) {
			g.emitf("    JRS %s", blockLabel(f.Name, t.Target))
		}

	case *TermBrIf:
		cc := g.condCode(f, t.Cond)

		// Build block-arg copies for each edge.
		thenCopies := g.buildBlockCopies(f, t.Then, t.ThenArgs)
		elseCopies := g.buildBlockCopies(f, t.Else, t.ElseArgs)

		// If a branch has non-trivial copies, it cannot be a fall-through:
		// redirect it through a trampoline that performs the copies then JR.
		thenLbl := g.branchLabel(f, t.Then, thenCopies)
		elseLbl := g.branchLabel(f, t.Else, elseCopies)

		// Fall-through is only possible when there are no copies for that edge.
		thenFT := g.isFallThrough(f, t.Then) && len(thenCopies) == 0
		elseFT := g.isFallThrough(f, t.Else) && len(elseCopies) == 0

		// Two-condition sentinels: CmpGt/CmpLe with lhs in A (no operand swap).
		// CGT = NC && NZ (a > b); CLE = C || Z (a ≤ b).
		// Note: CLE/CGT need two jumps — first uses JR, second uses JR too.
		// MZA auto-promotes any JR to JP when out of range.
		if cc == "CLE" {
			// a ≤ b: branch to then if C (a<b) or Z (a==b).
			g.emitf("    JRS C, %s", thenLbl)
			g.emitf("    JRS Z, %s", thenLbl)
			if !elseFT {
				g.emitf("    JRS %s", elseLbl)
			}
		} else if cc == "CGT" {
			// a > b: branch to else if Z (a==b) or C (a<b), else fall/JR to then.
			g.emitf("    JRS Z, %s", elseLbl)
			g.emitf("    JRS C, %s", elseLbl)
			if !thenFT {
				g.emitf("    JRS %s", thenLbl)
			}
		} else if elseFT {
			g.emitf("    JRS %s, %s", cc, thenLbl)
			// else falls through naturally
		} else if thenFT {
			g.emitf("    JRS %s, %s", invertCC(cc), elseLbl)
			// then falls through naturally
		} else {
			g.emitf("    JRS %s, %s", cc, thenLbl)
			g.emitf("    JRS %s", elseLbl)
		}

	case *TermBrIf2:
		// Three-way unsigned comparison: one CP, three outcomes (==, <, >).
		// Emit: CP rhs (with A=lhs), then JR Z eq, JR C lt, fall-through/JR gt.
		lhs := g.loc(t.Lhs)
		rhs := g.loc(t.Rhs)

		// Emit the CP (same logic as genCmp).
		if cv, ok := g.constVals[t.Rhs]; ok {
			if !g.holdsValue("A", lhs) {
				g.emitLDA(lhs)
			}
			g.emitf("    CP %d", cv&0xFF)
		} else if rhs == "A" && lhs != "A" {
			// A already holds rhs; swap: CP lhs computes rhs-lhs.
			// Eq is still Z (rhs-lhs==0 ↔ rhs==lhs), but C now means rhs<lhs.
			// So we swap Lt↔Gt below.
			g.emit8ALU("CP", lhs)
			// Build copies for eq, swapped lt/gt.
			eqCopies := g.buildBlockCopies(f, t.Eq, t.EqArgs)
			ltCopies := g.buildBlockCopies(f, t.Gt, t.GtArgs) // swapped: C→gt
			gtCopies := g.buildBlockCopies(f, t.Lt, t.LtArgs) // swapped: NC,NZ→lt
			eqLbl := g.branchLabel(f, t.Eq, eqCopies)
			ltLbl := g.branchLabel(f, t.Gt, ltCopies)
			gtLbl := g.branchLabel(f, t.Lt, gtCopies)
			g.emitf("    JRS Z, %s", eqLbl)
			g.emitf("    JRS C, %s", ltLbl)
			if !g.isFallThrough(f, t.Lt) || len(gtCopies) != 0 {
				g.emitf("    JRS %s", gtLbl)
			}
			return
		} else {
			if !g.holdsValue("A", lhs) {
				g.emitLDA(lhs)
			}
			g.emit8ALU("CP", rhs)
		}

		// Build copies for each of the three edges.
		eqCopies := g.buildBlockCopies(f, t.Eq, t.EqArgs)
		ltCopies := g.buildBlockCopies(f, t.Lt, t.LtArgs)
		gtCopies := g.buildBlockCopies(f, t.Gt, t.GtArgs)
		eqLbl := g.branchLabel(f, t.Eq, eqCopies)
		ltLbl := g.branchLabel(f, t.Lt, ltCopies)
		gtLbl := g.branchLabel(f, t.Gt, gtCopies)

		g.emitf("    JRS Z, %s", eqLbl)
		g.emitf("    JRS C, %s", ltLbl)
		if !g.isFallThrough(f, t.Gt) || len(gtCopies) != 0 {
			g.emitf("    JRS %s", gtLbl)
		}

	case *TermDJNZ:
		// Emit body-edge copies for non-counter args.
		// The counter (B) is handled implicitly by DJNZ.
		// body.Params[0] = counter (gets decremented B); BodyArgs = params[1:].
		bodyBlock := f.BlockByLabel(t.Body)
		var bodyCopies []parallelCopy
		if bodyBlock != nil {
			for i, arg := range t.BodyArgs {
				if i+1 >= len(bodyBlock.Params) {
					break
				}
				param := bodyBlock.Params[i+1] // skip counter param[0]
				src := g.loc(arg)
				dst := g.loc(param.Dst)
				if src != dst {
					bodyCopies = append(bodyCopies, parallelCopy{srcName: src, dstName: dst, ty: param.Ty})
				}
			}
		}
		g.emitParallelCopy(bodyCopies)
		g.emitf("    DJNZ %s", blockLabel(f.Name, t.Body))
		// Exit path.
		exitCopies := g.buildBlockCopies(f, t.Exit, t.ExitArgs)
		g.emitParallelCopy(exitCopies)
		if !g.isFallThrough(f, t.Exit) || len(exitCopies) != 0 {
			g.emitf("    JP %s", blockLabel(f.Name, t.Exit))
		}

	case *TermCondRet:
		// Strategy: jump to Then FIRST (before moving return values),
		// because moving return values may emit instructions that clobber
		// the flag register the condition depends on (e.g. OR A before SBC).
		cc := g.condCode(f, t.Cond)
		copies := g.buildBlockCopies(f, t.Then, t.ThenArgs)
		thenLbl := g.branchLabel(f, t.Then, copies)
		retCopies := g.buildReturnCopies(t.Vals)
		thenFT := g.isFallThrough(f, t.Then) && len(copies) == 0

		switch cc {
		case "CLE":
			// a ≤ b: take Then if C (a<b) or Z (a==b).
			// Jump to Then with two conditional JRs; fall through to RET.
			g.emitf("    JRS C, %s", thenLbl)
			g.emitf("    JRS Z, %s", thenLbl)
			g.emitParallelCopy(retCopies)
			g.emit("    RET")
		case "CGT":
			// a > b: take Then if NC && NZ.
			// Invert: skip Then (jump to ret path) if C or Z.
			retLbl := fmt.Sprintf(".%s_cret%d", sanitizeIdent(f.Name), g.trampIdx)
			g.trampIdx++
			g.emitf("    JRS Z, %s", retLbl)
			g.emitf("    JRS C, %s", retLbl)
			if !thenFT {
				g.emitf("    JRS %s", thenLbl)
			}
			// Register ret path as trampoline so it's emitted AFTER the fallthrough
			// then-block (otherwise the inline label would block the fallthrough path).
			g.trampolines = append(g.trampolines, trampolineBlock{
				label:  retLbl,
				copies: retCopies,
				isRet:  true,
			})
		default:
			if thenFT && len(retCopies) == 0 {
				// Then-block is the fallthrough AND return path is empty:
				// use RET invertCC (1 byte, optimal). Only safe when retCopies is
				// empty — otherwise retCopies would corrupt registers that @then
				// reads, since they execute before the branch is resolved.
				g.emitf("    RET %s", invertCC(cc))
			} else if thenFT {
				// Then-block is the fallthrough but retCopies are non-trivial.
				// Jump to @then when condition is true (skipping the return path),
				// then fall through to @then naturally.
				g.emitf("    JRS %s, %s", cc, thenLbl)
				g.emitParallelCopy(retCopies)
				g.emit("    RET")
			} else {
				// General case: emit conditional jump to Then first, then return path.
				g.emitf("    JRS %s, %s", cc, thenLbl)
				g.emitParallelCopy(copies)
				g.emitParallelCopy(retCopies)
				g.emit("    RET")
			}
		}

	case *TermUnreachable:
		g.comment("unreachable")
	}
}

// branchLabel returns the label to jump to for a BrIf edge.
// If the edge has parallel copies, a trampoline block is registered and its
// label is returned instead of the real target label.
func (g *z80cg) branchLabel(f *Func, target string, copies []parallelCopy) string {
	realLbl := blockLabel(f.Name, target)
	if len(copies) == 0 {
		return realLbl
	}
	tramLbl := fmt.Sprintf(".%s_trmp%d", sanitizeIdent(f.Name), g.trampIdx)
	g.trampIdx++
	g.trampolines = append(g.trampolines, trampolineBlock{
		label:  tramLbl,
		copies: copies,
		target: realLbl,
	})
	return tramLbl
}

// isFallThrough reports whether targetLabel is the next physical block after
// the current one (g.blockIdx), making a JP redundant.
func (g *z80cg) isFallThrough(f *Func, targetLabel string) bool {
	next := g.blockIdx + 1
	if next >= len(f.Blocks) {
		return false
	}
	return f.Blocks[next].Label == targetLabel
}

// ── DJNZ peephole ─────────────────────────────────────────────────────────────

// findConstInFunc finds the Imm value of an OpConst instruction defining r.
func findConstInFunc(f *Func, r Reg) (int64, bool) {
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpConst && inst.Dst == r {
				return inst.Imm, true
			}
		}
	}
	return 0, false
}

// djnzPeepholeResult holds the result of DJNZ peephole detection.
type djnzPeepholeResult struct {
	skipLast  bool      // skip the last OpSub instruction (DJNZ handles DEC B)
	bodyLbl   string    // DJNZ target: the loop body label (brif.Then)
	checkBrif *TermBrIf // the check block's brif (for exit-edge copies)
}

// detectDJNZPeephole checks if block b ends with the DJNZ-optimizable pattern:
//
//	last_inst: OpSub(counter→B, 1)
//	term:      TermJmp to check_block
//	check_block: [OpCmp(CmpNe, B, 0)] + TermBrIf(Then=body, Else=exit)
//	all args trivially in-place (no non-trivial copies needed)
func (g *z80cg) detectDJNZPeephole(f *Func, b *Block) djnzPeepholeResult {
	if len(b.Insts) == 0 {
		return djnzPeepholeResult{}
	}
	jmp, ok := b.Term.(*TermJmp)
	if !ok {
		return djnzPeepholeResult{}
	}
	// Last instruction must be OpSub(counter, 1) with counter→B.
	last := b.Insts[len(b.Insts)-1]
	if last.Op != OpSub {
		return djnzPeepholeResult{}
	}
	if g.ar.Loc(last.Src[0]).Name != "B" || g.ar.Loc(last.Dst).Name != "B" {
		return djnzPeepholeResult{}
	}
	// RHS must be constant 1.
	rhs, found := findConstInFunc(f, last.Src[1])
	if !found || rhs != 1 {
		return djnzPeepholeResult{}
	}
	// Target must be the check block: only CmpNe(B, 0) + optional Const(0) + TermBrIf.
	check := f.BlockByLabel(jmp.Target)
	if check == nil || len(check.Insts) == 0 || len(check.Insts) > 2 {
		return djnzPeepholeResult{}
	}
	// Find the OpCmp instruction (last if there are two).
	var cmp *Inst
	for i := len(check.Insts) - 1; i >= 0; i-- {
		if check.Insts[i].Op == OpCmp {
			cmp = check.Insts[i]
			break
		}
	}
	if cmp == nil || cmp.Cond != CmpNe {
		return djnzPeepholeResult{}
	}
	if g.ar.Loc(cmp.Src[0]).Name != "B" {
		return djnzPeepholeResult{}
	}
	zero, found2 := findConstInFunc(f, cmp.Src[1])
	if !found2 || zero != 0 {
		return djnzPeepholeResult{}
	}
	// Verify all instructions in check block are CmpNe + optional Const(0) only.
	for _, inst := range check.Insts {
		if inst.Op != OpCmp && inst.Op != OpConst {
			return djnzPeepholeResult{}
		}
	}
	brif, ok2 := check.Term.(*TermBrIf)
	if !ok2 || brif.Cond != cmp.Dst {
		return djnzPeepholeResult{}
	}
	// Verify all TermJmp args are trivially in-place (excluding counter B→B).
	for i, arg := range jmp.Args {
		if i >= len(check.Params) {
			break
		}
		srcLoc := g.loc(arg)
		dstLoc := g.loc(check.Params[i].Dst)
		if srcLoc == "B" && dstLoc == "B" {
			continue // counter handled by DJNZ
		}
		if srcLoc != dstLoc {
			return djnzPeepholeResult{} // non-trivial copy, fall back
		}
	}
	// Verify exit-edge copies are trivial.
	exitBlock := f.BlockByLabel(brif.Else)
	if exitBlock != nil {
		for i, arg := range brif.ElseArgs {
			if i >= len(exitBlock.Params) {
				break
			}
			if g.ar.Loc(arg).Name != g.ar.Loc(exitBlock.Params[i].Dst).Name {
				return djnzPeepholeResult{} // non-trivial exit copy, fall back
			}
		}
	}
	return djnzPeepholeResult{
		skipLast:  true,
		bodyLbl:   blockLabel(f.Name, brif.Then),
		checkBrif: brif,
	}
}
