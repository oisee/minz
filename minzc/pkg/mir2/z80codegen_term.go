package mir2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/z80spec"
)

// trampolineBlock is emitted after all regular blocks of a function.
// It resolves block-argument copies for one outgoing BrIf edge.
// If isRet is true, the block ends with RET instead of JRS target.
type trampolineBlock struct {
	label      string // local label for this trampoline
	copies     []parallelCopy
	target     string // real destination label (already formatted)
	isRet      bool   // true → emit RET instead of JRS target
	frameBytes int
}

// restoreLiveOutOverrides moves every relocated vreg that is live out of the
// current block, or read by its terminator, back to its allocated register
// before the terminator. A
// relocation (physOverride) is block-local: successors, and blocks laid out
// after this one, read the allocator's location. Without this, a value saved
// to a scratch register in one branch (gcd's else arm) was read from that
// scratch on paths that never wrote it.
func (g *z80cg) restoreLiveOutOverrides(f *Func, t Term) {
	if len(g.physOverride) == 0 {
		return
	}
	needed := map[Reg]bool{}
	if g.liveness != nil {
		if liveOut := g.liveness.LiveOutOf(f, g.curBlock); liveOut != nil {
			for r := range g.physOverride {
				needed[r] = liveOut.Has(r)
			}
		}
	}
	// Block params and other live values participate in the same shuffle.
	// DJNZ body params instead receive their next values from its edge copies.
	skipped := map[Reg]bool{}
	if td, ok := t.(*TermDJNZ); ok {
		if body := f.BlockByLabel(td.Body); body != nil {
			for i := range td.BodyArgs {
				if i+1 < len(body.Params) {
					skipped[body.Params[i+1].Dst] = true
				}
			}
		}
	}
	// A loop parameter overwritten on the next iteration may still be a
	// source of another back-edge argument or the exit value (parallel state
	// updates such as Fibonacci). Preserve its old value until those reads.
	usedByTerm := map[Reg]bool{}
	for _, r := range t.termUses() {
		usedByTerm[r] = true
	}
	for _, bp := range g.curBlock.Params {
		if skipped[bp.Dst] && !needed[bp.Dst] && !usedByTerm[bp.Dst] {
			delete(g.physOverride, bp.Dst)
		} else {
			needed[bp.Dst] = true
		}
	}
	// Terminator operands count too: jmp @head(%r3, …) passes r3 to the param
	// that is r3 itself after coalescing, and the parallel copy skips it as an
	// identity move, so r3 must already sit in its allocated register.
	regs := make([]Reg, 0, len(g.physOverride))
	for r := range g.physOverride {
		if needed[r] || usedByTerm[r] {
			regs = append(regs, r)
		}
	}
	sort.Slice(regs, func(i, j int) bool { return regs[i] < regs[j] })
	var copies []parallelCopy
	for _, r := range regs {
		scratch := g.physOverride[r]
		canonLoc := g.ar.Loc(r)
		// A predicate materialized before a flag writer has a real byte
		// home. Keep it until this terminator tests it; F cannot receive an
		// ordinary copy. The identity reserves its scratch during shuffles.
		if canonLoc.Kind == LocFlag && usedByTerm[r] {
			copies = append(copies, parallelCopy{srcName: scratch, dstName: scratch, ty: TyU8})
			continue
		}
		delete(g.physOverride, r)
		if canonLoc.Kind != LocReg && canonLoc.Kind != LocIXY8 {
			continue
		}
		if canon := canonLoc.Name; canon != "" && canon != scratch {
			ty := TyU8
			if isPairReg(canon) {
				ty = TyU16
			}
			copies = append(copies, parallelCopy{srcName: scratch, dstName: canon, ty: ty})
			g.comment(fmt.Sprintf("restore live-out r%d from scratch", r))
		}
	}
	// Restores are simultaneous: a canonical destination may still contain
	// another value's scratch source (including a byte half of a pair).
	g.emitParallelCopy(copies)
}

// ── Terminators ───────────────────────────────────────────────────────────────

func (g *z80cg) genTerm(f *Func, t Term) {
	g.restoreLiveOutOverrides(f, t)
	if g.shiftCarryInst != nil && g.carryFrom != g.shiftCarryInst {
		g.err = fmt.Errorf("z80 codegen: shift carry lost before fused branch in %s", g.fn.Name)
		return
	}

	switch t := t.(type) {
	case *TermErrorRet:
		g.emitLD8("A", g.loc(t.Code))
		g.emitLocalFrameCleanup(g.localFrameDepth[g.curBlock.Label])
		g.emit("    SCF")
		g.emit("    RET")
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
			(f.Contract.Returns[0].Ty == TyBool || (z80ValueWidth(f.Contract.Returns[0].Ty) <= 8 && collectRegInfo(f)[t.Vals[0]].Ty == TyBool)) &&
			f.Contract.Returns[0].Class != ClassFlag && g.loc(t.Vals[0]) == "F" {
			g.emitFlagPredicateByte(g.condCode(f, t.Vals[0]))
			if dst := canonicalReturnLoc(f.Contract.Returns[0].Class, TyBool); dst != "A" {
				g.emitLD8(dst, "A")
			}
		} else {
			g.emitParallelCopy(g.buildReturnCopies(t.Vals))
		}
		g.emitLocalFrameCleanup(g.localFrameDepth[g.curBlock.Label])
		if f.Attrs.FallibleCY {
			g.emit("    OR A")
		}
		// main() is the program entry point: use XOR A / DI / HALT instead of
		// RET so mze/mzx can detect program termination cleanly.  XOR A zeroes
		// A for a clean exit code 0.
		if f.Name == "main" && len(t.Vals) == 0 && g.runtime != Z80RuntimeCPM && !g.calledMain && !f.Attrs.FallibleCY {
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
		thenFT := g.isFallThrough(f, t.Then) && !hasEffectiveBlockCopies(thenCopies)
		elseFT := g.isFallThrough(f, t.Else) && !hasEffectiveBlockCopies(elseCopies)

		// Two-condition sentinels: CmpGt/CmpLe with lhs in A (no operand swap).
		// CGT = NC && NZ (a > b); CLE = C || Z (a ≤ b).
		// Note: CLE/CGT need two jumps — first uses JR, second uses JR too.
		// MZA auto-promotes any JR to JP when out of range.
		if g.cmpBranchCC[t.Cond] == "signed" {
			g.emitSignedBranch(f, t.Cond, thenLbl, elseLbl)
		} else if cc == "CLE" {
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
				if g.brIf2ClobbersLiveA(f, t) {
					g.genBrIf2SavingA(f, t, lhs, "")
					return
				}
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
				if g.brIf2ClobbersLiveA(f, t) {
					g.genBrIf2SavingA(f, t, lhs, rhs)
					return
				}
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
		// ClassCounter prefers B but is not a hard pin. Coalescing and
		// register pressure can put it elsewhere; decrement its actual home.
		// The implicit body parameter receives that decremented value.
		bodyArgs := append([]Reg{t.Counter}, t.BodyArgs...)
		bodyCopies := g.buildBlockCopies(f, t.Body, bodyArgs)
		exitCopies := g.buildBlockCopies(f, t.Exit, t.ExitArgs)
		counter := g.loc(t.Counter)
		// Explicit edge arguments read the pre-decrement SSA value. Preserve
		// it separately from the implicit first parameter's decremented value.
		oldCounter := ""
		for _, args := range [][]Reg{t.BodyArgs, t.ExitArgs} {
			for _, arg := range args {
				if arg == t.Counter {
					oldCounter = fmt.Sprintf("._call_result_%s_djnz_old%d", sanitizeIdent(f.Name), g.trampIdx)
				}
			}
		}
		if oldCounter != "" {
			g.trampIdx++
			g.emit("    PUSH HL")
			g.emit("    PUSH AF")
			g.emitLD8("A", counter)
			g.emitf("    LD (%s), A", oldCounter)
			g.emit("    POP AF")
			g.emit("    POP HL")
			// buildBlockCopies retains identity copies, so even a coalesced
			// explicit counter argument can be redirected to the snapshot.
			for i, arg := range t.BodyArgs {
				if arg == t.Counter && i+1 < len(bodyCopies) {
					bodyCopies[i+1].srcName = oldCounter
					bodyCopies[i+1].isImm = false
				}
			}
			for i, arg := range t.ExitArgs {
				if arg == t.Counter && i < len(exitCopies) {
					exitCopies[i].srcName = oldCounter
					exitCopies[i].isImm = false
				}
			}
		}
		needsBodyCopy := false
		for _, c := range bodyCopies {
			if c.isImm || c.srcName != c.dstName {
				needsBodyCopy = true
			}
		}
		// Hoist the back-edge shuffle only after proving the exit can still
		// read its original operands. Rewrite exit sources to their new homes.
		bodyLabel := blockLabel(f.Name, t.Body)
		if counter == "B" && needsBodyCopy && !g.functionOptions.DJNZTrampoline {
			// The hoist runs the shuffle on every iteration including the last,
			// and the exit reads the shuffled homes. The trampoline instead pays
			// a JRS on every iteration (12T). With one iteration, the worst case,
			// the hoist must cost no more than the trampoline's exit path.
			if adjusted, ok := g.hoistedDJNZExitCopies(f, t, bodyCopies, exitCopies); ok {
				newExit, restoreNew := g.trialCopies(adjusted)
				restoreNew()
				oldExit, restoreOld := g.trialCopies(exitCopies)
				restoreOld()
				hoist, restore := g.trialCopies(bodyCopies)
				pays := asmTStates(hoist)+asmTStates(newExit) <= asmTStates(oldExit)+12
				if pays && g.djnzTargetWithinRangeAfter(bodyLabel, hoist) {
					g.sb.WriteString(hoist)
					exitCopies = adjusted
					bodyCopies = nil
					needsBodyCopy = false
				} else {
					restore()
				}
			}
		}
		if counter == "B" && (needsBodyCopy || !g.djnzTargetWithinRange(bodyLabel)) {
			// DJNZ cannot be promoted to a long branch. Keep its taken
			// label adjacent, even when edge copies need a distant target.
			taken := fmt.Sprintf(".%s_djnz_taken%d", sanitizeIdent(f.Name), g.trampIdx)
			fall := fmt.Sprintf(".%s_djnz_fall%d", sanitizeIdent(f.Name), g.trampIdx)
			g.trampIdx++
			g.emitf("    DJNZ %s", taken)
			g.emitf("    JRS %s", fall)
			g.emit(taken + ":")
			g.emitParallelCopy(bodyCopies)
			g.emitf("    JRS %s", bodyLabel)
			g.emit(fall + ":")
		} else if counter == "B" {
			g.emitf("    DJNZ %s", bodyLabel)
		} else {
			if needsBodyCopy {
				bodyLabel = g.branchLabel(f, t.Body, bodyCopies)
			}
			if isSpill(counter) {
				// Preserve both A and HL while retaining DEC's branch flags.
				g.emit("    PUSH HL")
				g.emit("    PUSH AF")
				g.emitLD8("A", counter)
				g.emit("    DEC A")
				g.emitLD8(counter, "A")
				g.emit("    POP HL")
				g.emit("    LD A, H")
				g.emit("    POP HL")
			} else {
				g.emitf("    DEC %s", counter)
			}
			g.emitf("    JRS NZ, %s", bodyLabel)
		}
		// Exit copies use the adjusted homes after a proven hoist; otherwise
		// body copies remain on the taken edge to preserve exit operands.
		g.emitParallelCopy(exitCopies)
		if oldCounter != "" || !g.isFallThrough(f, t.Exit) || len(exitCopies) != 0 {
			g.emitf("    JP %s", blockLabel(f.Name, t.Exit))
		}

		if oldCounter != "" {
			g.emit(oldCounter + ": DB 0")
		}

	case *TermCondRet:
		// Strategy: jump to Then FIRST (before moving return values),
		// because moving return values may emit instructions that clobber
		// the flag register the condition depends on (e.g. OR A before SBC).
		cc := g.condCode(f, t.Cond)
		copies := g.buildBlockCopies(f, t.Then, t.ThenArgs)
		thenLbl := g.branchLabel(f, t.Then, copies)
		retCopies := g.buildReturnCopies(t.Vals)
		thenFT := g.isFallThrough(f, t.Then) && !hasEffectiveBlockCopies(copies)
		frameBytes := g.localFrameDepth[g.curBlock.Label]

		switch cc {
		case "CLE":
			// a ≤ b: take Then if C (a<b) or Z (a==b).
			// Jump to Then with two conditional JRs; fall through to RET.
			g.emitf("    JRS C, %s", thenLbl)
			g.emitf("    JRS Z, %s", thenLbl)
			g.emitParallelCopy(retCopies)
			g.emitLocalFrameCleanup(frameBytes)
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
				label:      retLbl,
				copies:     retCopies,
				isRet:      true,
				frameBytes: frameBytes,
			})
		default:
			if thenFT && len(retCopies) == 0 && frameBytes == 0 {
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
				g.emitLocalFrameCleanup(frameBytes)
				g.emit("    RET")
			} else {
				// General case: emit conditional jump to Then first, then return path.
				g.emitf("    JRS %s, %s", cc, thenLbl)
				// Then copies belong to its branch trampoline, never the return path.
				g.emitParallelCopy(retCopies)
				g.emitLocalFrameCleanup(frameBytes)
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
	if !hasEffectiveBlockCopies(copies) {
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

// Coalesced copies reserve physical locations for scratch selection when
// other copies execute, but an edge consisting only of them emits no moves.
func hasEffectiveBlockCopies(copies []parallelCopy) bool {
	for _, c := range copies {
		if c.isImm || c.srcName != c.dstName {
			return true
		}
	}
	return false
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

// A Z80 instruction occupies at most four bytes. This conservative bound
// keeps small backwards DJNZ loops unchanged; uncertain or forward spans
// use a nearby taken label and a relaxable JRS to the real body.
func (g *z80cg) djnzTargetWithinRange(label string) bool {
	return g.djnzTargetWithinRangeAfter(label, "")
}

// djnzTargetWithinRangeAfter applies the same bound as if pending were
// already emitted before the DJNZ.
func (g *z80cg) djnzTargetWithinRangeAfter(label, pending string) bool {
	text := g.sb.String() + pending
	at := strings.LastIndex(text, "\n"+label+":")
	if at < 0 {
		return false
	}
	bound := 2 // DJNZ itself
	for _, line := range strings.Split(text[at+1:], "\n") {
		line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		if strings.Contains(line, ":") {
			return false
		}
		// Inline assembly may contain data directives or macros; their
		// expansion has no four-byte bound, so don't assume a short span.
		op := strings.ToUpper(strings.Fields(line)[0])
		switch op {
		case "LD", "PUSH", "POP", "EX", "EXX", "INC", "DEC", "ADD", "ADC", "SUB", "SBC", "AND", "OR", "XOR", "CP", "NOP", "NEG", "CPL", "SCF", "CCF", "DAA", "JP", "JR", "JRS", "DJNZ", "CALL", "RET", "RETI", "RETN", "RST", "DI", "EI", "HALT", "IN", "OUT", "BIT", "SET", "RES", "RL", "RLC", "RR", "RRC", "RLA", "RLCA", "RRA", "RRCA", "SLA", "SRA", "SRL", "RLD", "RRD", "LDI", "LDIR", "LDD", "LDDR", "CPI", "CPIR", "CPD", "CPDR":
		default:
			return false
		}
		bound += 4
		if bound > 128 {
			return false
		}
	}
	return true
}

// Branch on S xor V directly. Both paths leave flags and registers untouched;
// edge copies still run through the ordinary branch trampolines.
func (g *z80cg) emitSignedBranch(f *Func, cond Reg, thenLbl, elseLbl string) {
	var c CmpCond
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op == OpCmp && i.Dst == cond {
				c = i.Cond
			}
		}
	}
	if g.cmpSwapped[cond] {
		c = c.Swap()
	}
	lt, ge := thenLbl, elseLbl
	if c == CmpGe || c == CmpGt {
		lt, ge = elseLbl, thenLbl
	}
	if c == CmpLe {
		g.emitf("    JRS Z, %s", thenLbl)
	}
	if c == CmpGt {
		g.emitf("    JRS Z, %s", elseLbl)
	}
	idx := g.trampIdx
	g.trampIdx++
	g.emitf("    JP M, .sbranch_neg_%d", idx)
	g.emitf("    JP PE, %s", lt)
	g.emitf("    JRS %s", ge)
	g.emitf(".sbranch_neg_%d:", idx)
	g.emitf("    JP PO, %s", lt)
	g.emitf("    JRS %s", ge)
}

// hoistedDJNZExitCopies refuses partial aliases, wide/shadow moves, and writes
// to B or reads of its post-decrement value. For a destroyed exit source it requires an exact, same-width copy of
// that source in a surviving destination. Other successor live-ins must remain
// at their allocated homes. A failed proof leaves the edge-local trampoline.
func (g *z80cg) hoistedDJNZExitCopies(f *Func, t *TermDJNZ, body, exit []parallelCopy) ([]parallelCopy, bool) {
	changed := func(c parallelCopy) bool { return c.isImm || c.srcName != c.dstName }
	overlaps := func(name string) bool {
		for _, c := range body {
			if changed(c) && (name == c.dstName || z80spec.Overlaps(name, c.dstName)) {
				return true
			}
		}
		return false
	}
	// Copy expansion may borrow shadow A (index halves, spills and cycles).
	// Its implicit scratch writes must not destroy exit values or operands.
	for _, copies := range [][]parallelCopy{body, exit} {
		for _, c := range copies {
			if z80ValueWidth(c.ty) > 16 || strings.Contains(c.srcName, "'") || strings.Contains(c.dstName, "'") {
				return nil, false
			}
		}
	}
	for _, c := range body {
		if changed(c) && (z80ValueWidth(c.ty) > 16 || z80spec.Overlaps(c.dstName, "B") || (!c.isImm && z80spec.Overlaps(c.srcName, "B")) || c.dstName == "F" || c.srcName == "F" || isSpill(c.dstName)) {
			return nil, false
		}
	}
	if g.liveness == nil {
		return nil, false
	}
	if live := g.liveness.LiveInOf(f, f.BlockByLabel(t.Exit)); live != nil {
		for _, r := range live.Slice() {
			loc := g.ar.Loc(r)
			if loc.Kind == LocShadow || loc.Kind == LocDWord || overlaps(g.loc(r)) {
				return nil, false
			}
		}
	}
	adjusted := append([]parallelCopy(nil), exit...)
	for i, c := range adjusted {
		if c.isImm || !overlaps(c.srcName) {
			continue
		}
		found := false
		for _, b := range body {
			if !b.isImm && b.srcName == c.srcName && b.ty == c.ty {
				// All destinations describe the simultaneous result of the shuffle.
				// Refuse a second destination that partially overwrites this new home.
				intact := true
				for _, other := range body {
					if changed(other) && other.dstName != b.dstName && z80spec.Overlaps(other.dstName, b.dstName) {
						intact = false
					}
				}
				if intact {
					adjusted[i].srcName = b.dstName
					found = true
					break
				}
			}
		}
		if !found {
			return nil, false
		}
	}
	return adjusted, true
}

// trialCopies emits a parallel copy into a scratch buffer. The caller either
// writes the text back or calls restore to drop the bookkeeping it recorded.
func (g *z80cg) trialCopies(copies []parallelCopy) (string, func()) {
	outer := g.sb
	var buf strings.Builder
	g.sb = &buf
	restore := g.trialEmission()
	g.emitParallelCopy(copies)
	g.sb = outer
	return buf.String(), restore
}

// asmTStates sums the primary T-states of emitted instruction lines.
func asmTStates(text string) int {
	total := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "    ;") {
			ts, _ := z80TStates(line)
			total += ts
		}
	}
	return total
}

// trialEmission snapshots the bookkeeping that emission records outside the
// text buffer. The returned function discards what a rejected trial added,
// so native memory markers and T-state annotations match the final text.
func (g *z80cg) trialEmission() func() {
	sinkLen := -1
	if g.nativeMemorySink != nil {
		sinkLen = len(*g.nativeMemorySink)
	}
	spills, blockT := g.nativeSpillCount, g.blockT
	return func() {
		if sinkLen >= 0 {
			for _, r := range (*g.nativeMemorySink)[sinkLen:] {
				delete(g.nativeOrphanLabels, r.ID)
			}
			*g.nativeMemorySink = (*g.nativeMemorySink)[:sinkLen]
		}
		g.nativeSpillCount, g.blockT = spills, blockT
	}
}

// brIf2ClobbersLiveA reports whether loading the three-way comparison's lhs
// into A would destroy a value that one of its edges still needs: a value
// live out of the block or passed as an edge argument, other than lhs itself.
func (g *z80cg) brIf2ClobbersLiveA(f *Func, t *TermBrIf2) bool {
	inA := func(r Reg) bool { return r != t.Lhs && g.loc(r) == "A" }
	for _, args := range [][]Reg{t.EqArgs, t.LtArgs, t.GtArgs} {
		for _, r := range args {
			if inA(r) {
				return true
			}
		}
	}
	if g.liveness != nil {
		if out := g.liveness.LiveOutOf(f, g.curBlock); out != nil {
			for _, r := range out.Slice() {
				if inA(r) {
					return true
				}
			}
		}
	}
	return false
}

// genBrIf2SavingA emits the three-way comparison while preserving a live A:
// PUSH AF before loading lhs, and POP AF on every edge before its copies.
// The flags restored by POP AF are no longer needed once the edge is taken.
// rhs == "" means the right operand is a constant.
func (g *z80cg) genBrIf2SavingA(f *Func, t *TermBrIf2, lhs, rhs string) {
	g.emit("    PUSH AF")
	g.emitLDA(lhs)
	if rhs == "" {
		g.emitf("    CP %d", g.constVals[t.Rhs]&0xFF)
	} else {
		g.emit8ALU("CP", rhs)
	}
	base := fmt.Sprintf(".%s_brif2_%d", sanitizeIdent(f.Name), g.trampIdx)
	g.trampIdx++
	g.emitf("    JRS Z, %s_eq", base)
	g.emitf("    JRS C, %s_lt", base)
	for _, e := range []struct {
		suffix string
		target string
		args   []Reg
	}{{"gt", t.Gt, t.GtArgs}, {"eq", t.Eq, t.EqArgs}, {"lt", t.Lt, t.LtArgs}} {
		copies := g.buildBlockCopies(f, e.target, e.args)
		g.emit(base + "_" + e.suffix + ":")
		g.emit("    POP AF")
		g.emitParallelCopy(copies)
		g.emitf("    JP %s", blockLabel(f.Name, e.target))
	}
	g.invalidate("A")
}
