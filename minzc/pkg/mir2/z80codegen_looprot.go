package mir2

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/minz/minzc/pkg/z80asm"
)

// Loop rotation by tail duplication.
//
// A loop whose header H is a small test block is laid out "test at the top":
// the latch L ends with JRS H, and H tests and branches into the body or out
// of the loop. When L's back edge reaches such an H, the generator emits H's
// test and branch again in place of the JRS, so each iteration pays one taken
// conditional branch instead of an unconditional jump plus a not-taken one.
//
// This works on the emitted code, not on the MIR CFG: the allocation is
// unchanged. After the parallel copy of L→H every parameter of H is in its
// allocated home, and every other value H reads is live out of L and has been
// returned to its home by restoreLiveOutOverrides, so a second emission of H at
// that point sees exactly the machine state H's own label sees. The emission
// state that genBlock establishes at H's label (enterBlockState) is
// established again for the copy, and the copy is generated into a scratch
// buffer first so it can be refused without trace.

// loopRotationMaxTestBytes bounds the duplicated test code (excluding the
// branches that end it).
const loopRotationMaxTestBytes = 6

// loopRotationMaxBytes bounds the whole copy, including its branches and an
// inline return path of a conditional-return header.
const loopRotationMaxBytes = 12

// loopRotationOps are the instructions a rotated header may contain: they
// read and write registers only, and their emission defines no labels. Memory
// loads are excluded until a proof that the loop does not write the location.
var loopRotationOps = map[Op]bool{
	OpConst: true, OpCmp: true, OpMove: true,
	OpExt: true, OpSext: true, OpTrunc: true,
	OpAdd: true, OpSub: true, OpAnd: true, OpOr: true, OpXor: true,
	OpNot: true, OpNeg: true,
}

// isLoopBackEdge reports whether from→to is a back edge: to dominates from.
func (g *z80cg) isLoopBackEdge(f *Func, from, to *Block) bool {
	if g.domFn != f {
		g.domFn = f
		g.dom = reachableDominators(f)
	}
	return g.dom[from.Label][to.Label]
}

// rotatableHeader reports whether h may be emitted a second time at the end
// of a latch. It checks the MIR shape only: every instruction is one of
// loopRotationOps without a symbol or volatile access; the terminator is a
// TermBrIf or TermCondRet whose condition an instruction of h defines; and
// every parameter, defined value, operand and terminator use (except
// suppressed constants) has a register, IX/IY-half or flag home that is not a
// spill slot. The emitted copy is checked separately by tryRotateLoop.
func (g *z80cg) rotatableHeader(h *Block) bool {
	var cond Reg
	switch t := h.Term.(type) {
	case *TermBrIf:
		cond = t.Cond
	case *TermCondRet:
		cond = t.Cond
	default:
		return false
	}
	inReg := func(r Reg) bool {
		if r == NoReg {
			return true
		}
		loc := g.ar.Loc(r)
		switch loc.Kind {
		case LocReg, LocIXY8, LocFlag:
		default:
			return false
		}
		return loc.Name != "" && !isSpill(g.loc(r))
	}
	condDefined := false
	for _, x := range h.Insts {
		if !loopRotationOps[x.Op] || x.Sym != "" || x.Volatile {
			return false
		}
		if x.Dst == cond {
			condDefined = true
		}
		// A suppressed constant has no home; its value is an immediate.
		if (x.Op != OpConst || !g.deadConsts[x.Dst]) && !inReg(x.Dst) {
			return false
		}
		for _, r := range x.Uses() {
			if g.deadConsts[r] {
				continue
			}
			if !inReg(r) {
				return false
			}
		}
	}
	if !condDefined {
		return false
	}
	for _, p := range h.Params {
		if !inReg(p.Dst) {
			return false
		}
	}
	for _, r := range h.Term.termUses() {
		if !inReg(r) {
			return false
		}
	}
	return true
}

// tryRotateLoop emits the header test of a back edge in place of the jump to
// it. It runs after the edge's parallel copy and reports whether it emitted
// the replacement; on false nothing was emitted and the caller emits the JRS.
func (g *z80cg) tryRotateLoop(f *Func, t *TermJmp) bool {
	if g.disableLoopRotation {
		return false
	}
	latch := g.curBlock
	head := f.BlockByLabel(t.Target)
	if latch == nil || head == nil || head == latch || !g.isLoopBackEdge(f, latch, head) || !g.rotatableHeader(head) {
		return false
	}

	outer := g.sb
	var buf strings.Builder
	g.sb = &buf
	restoreBook := g.trialEmission()
	nTramp, trampIdx, errBefore := len(g.trampolines), g.trampIdx, g.err
	needs := [...]bool{g.needsCallStack, g.needsCallIX, g.needsMul8, g.needsQuarterSquare, g.needsRotate}

	g.enterBlockState(head)
	g.tailCallInst = nil
	for i, x := range head.Insts {
		g.curInstIdx = i
		g.genInst(x)
	}
	testLen := buf.Len()
	g.genTerm(f, head.Term)
	g.sb = outer
	g.curBlock = latch

	text := g.shareRotationTrampolines(buf.String(), nTramp)
	ok := g.err == nil
	testBytes, total := 0, 0
	if ok {
		testBytes, ok = asmByteSize(text[:testLen])
	}
	if ok {
		total, ok = asmByteSize(text)
	}
	ok = ok && testBytes <= loopRotationMaxTestBytes && total <= loopRotationMaxBytes && !definesLabel(text) && !g.jumpsUnconditionallyIntoLoop(f, latch, head, text)
	// Size mode: the copy replaces a two-byte JR and may not cost more,
	// including any edge trampolines it registered.
	if ok && g.optimizeForSize && (total > 2 || len(g.trampolines) != nTramp) {
		ok = false
	}
	if !ok {
		restoreBook()
		g.trampolines = g.trampolines[:nTramp]
		g.trampIdx = trampIdx
		g.err = errBefore
		g.needsCallStack, g.needsCallIX, g.needsMul8, g.needsQuarterSquare, g.needsRotate = needs[0], needs[1], needs[2], needs[3], needs[4]
		return false
	}
	fmt.Fprintf(g.sb, "    ; loop rotation: test of %s repeated at the back edge\n", head.Label)
	g.sb.WriteString(text)
	return true
}

// jumpsUnconditionallyIntoLoop reports whether the copy reaches the loop body
// by an unconditional jump (directly or through an edge trampoline), as a
// two-jump condition does when the body is its fall-through arm (CGT/CLE).
// Such a copy costs the same JR per iteration that it was meant to remove.
// The in-loop successors are those that dominate the latch.
func (g *z80cg) jumpsUnconditionallyIntoLoop(f *Func, latch, head *Block, text string) bool {
	into := map[string]bool{}
	for _, s := range head.Term.Successors() {
		if g.dom[latch.Label][s] {
			into[blockLabel(f.Name, s)] = true
		}
	}
	for _, t := range g.trampolines {
		if into[t.target] {
			into[t.label] = true
		}
	}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(strings.SplitN(line, ";", 2)[0])
		if len(fields) == 2 && into[fields[1]] {
			switch strings.ToUpper(fields[0]) {
			case "JR", "JRS", "JP":
				return true
			}
		}
	}
	return false
}

// shareRotationTrampolines points the copy's edges at an identical trampoline
// registered before it (normally the header's own exit or body trampoline)
// and drops the new duplicate. Trampolines from index first on are the copy's.
func (g *z80cg) shareRotationTrampolines(text string, first int) string {
	kept := g.trampolines[:first]
	for _, t := range g.trampolines[first:] {
		shared := ""
		for _, old := range g.trampolines[:first] {
			if old.target == t.target && old.isRet == t.isRet && old.frameBytes == t.frameBytes && reflect.DeepEqual(old.copies, t.copies) {
				shared = old.label
				break
			}
		}
		if shared == "" {
			kept = append(kept, t)
			continue
		}
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			code, comment, hasComment := strings.Cut(line, ";")
			trimmed := strings.TrimRight(code, " ")
			if prefix, found := strings.CutSuffix(trimmed, t.label); found && (strings.HasSuffix(prefix, " ") || strings.HasSuffix(prefix, ",")) {
				lines[i] = prefix + shared + code[len(trimmed):]
				if hasComment {
					lines[i] += ";" + comment
				}
			}
		}
		text = strings.Join(lines, "\n")
	}
	g.trampolines = kept
	return text
}

// definesLabel reports whether emitted text contains a label definition. A
// duplicated label would be a second definition of the same symbol.
func definesLabel(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		code := strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if code == "" {
			continue
		}
		if !strings.HasPrefix(line, "    ") || strings.HasSuffix(code, ":") || strings.Contains(strings.Fields(code)[0], ":") {
			return true
		}
	}
	return false
}

var asmLineSizeCache sync.Map // instruction text → encoded size

// asmByteSize returns the encoded size of the instruction lines in text.
// Relative branches count two bytes and absolute jumps three; every other
// line is assembled on its own. It fails on anything it cannot assemble.
func asmByteSize(text string) (int, bool) {
	total := 0
	for _, line := range strings.Split(text, "\n") {
		code := strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		if code == "" {
			continue
		}
		switch strings.ToUpper(strings.Fields(code)[0]) {
		case "JR", "JRS":
			total += 2
			continue
		case "JP":
			if !strings.Contains(code, "(") {
				total += 3
				continue
			}
		}
		if n, ok := asmLineSizeCache.Load(code); ok {
			if n.(int) < 0 {
				return 0, false
			}
			total += n.(int)
			continue
		}
		n := -1
		if r, err := z80asm.NewAssembler().AssembleString("    ORG 0\n    " + code + "\n"); err == nil && r != nil && len(r.Errors) == 0 {
			n = len(r.Binary)
		}
		asmLineSizeCache.Store(code, n)
		if n < 0 {
			return 0, false
		}
		total += n
	}
	return total, true
}
