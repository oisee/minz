package mir2

import "fmt"

// parallelCopy is a single register-to-register move in a parallel copy sequence.
type parallelCopy struct {
	srcName string
	dstName string
	ty      Ty
	// isImm marks a constant rematerialisation: no source register, just emit
	// LD dstName, immVal after all register-to-register copies are resolved.
	isImm  bool
	immVal int64
}

// buildReturnCopies computes the parallel copies needed to move return values
// into their calling-convention physical registers.
// Using parallel copy resolution (instead of sequential emitMov) is essential
// when two return values are allocated to the same physical register after
// constant folding/PBQP — sequential moves would clobber one value.
func (g *z80cg) buildReturnCopies(vals []Reg) []parallelCopy {
	var copies []parallelCopy
	for i, rv := range vals {
		if i >= len(g.fn.Contract.Returns) {
			break
		}
		if rv == NoReg {
			continue // void call result; return register already correct
		}
		ret := g.fn.Contract.Returns[i]
		retLoc := canonicalReturnLoc(ret.Class, ret.Ty)
		src := g.loc(rv)
		if src != retLoc && !g.holdsValue(retLoc, src) {
			copies = append(copies, parallelCopy{srcName: src, dstName: retLoc, ty: ret.Ty})
		}
	}
	return copies
}

// buildBlockCopies computes the parallel copies needed to pass args to a
// target block's formal parameters.  Returns only moves where src ≠ dst.
func (g *z80cg) buildBlockCopies(f *Func, targetName string, args []Reg) []parallelCopy {
	if len(args) == 0 {
		return nil
	}
	var target *Block
	for _, b := range f.Blocks {
		if b.Label == targetName {
			target = b
			break
		}
	}
	if target == nil {
		return nil
	}
	var copies []parallelCopy
	for i, arg := range args {
		if i >= len(target.Params) {
			break
		}
		dst := g.loc(target.Params[i].Dst)
		ty := target.Params[i].Ty
		// Use g.loc (which respects physOverride) so that values saved to scratch
		// registers by materializePendingAcc/materializePendingFlag are correctly
		// moved to their canonical destination in the target block.
		src := g.loc(arg)
		if src == dst {
			continue // pre-coalescing or same-location allocation — no copy needed
		}
		// If this argument is a compile-time constant, mark it for deferred
		// rematerialisation AFTER all register-to-register copies are resolved.
		// Emitting LD dst, imm inline would clobber live source registers that
		// other copies still need (BUG-002).
		// Guard: only rematerialise if src != dst (handled above) to avoid
		// overwriting a live value when a const reg was pre-coalesced with its
		// loop param (they share a root; the const check would fire spuriously).
		if cv, isConst := g.constVals[arg]; isConst && dst != "" {
			copies = append(copies, parallelCopy{dstName: dst, ty: ty, isImm: true, immVal: cv})
			continue
		}
		if src != "" {
			copies = append(copies, parallelCopy{srcName: src, dstName: dst, ty: ty})
		}
	}
	return copies
}

// emitParallelCopy resolves and emits a set of parallel register moves,
// correctly handling cycles (e.g. HL↔DE → EX DE,HL).
// Constant rematerialisations (isImm=true) are emitted AFTER all register-to-
// register moves so they don't clobber live source registers.
func (g *z80cg) emitParallelCopy(copies []parallelCopy) {
	if len(copies) == 0 {
		return
	}
	type move struct {
		src, dst string
		ty       Ty
		done     bool
	}
	// Separate constants from register moves; emit register moves first.
	var immCopies []parallelCopy
	var regCopies []parallelCopy
	for _, c := range copies {
		if c.isImm {
			immCopies = append(immCopies, c)
		} else {
			regCopies = append(regCopies, c)
		}
	}
	moves := make([]move, len(regCopies))
	for i, c := range regCopies {
		moves[i] = move{src: c.srcName, dst: c.dstName, ty: c.ty}
	}

	for {
		// Emit any move whose dst is not a source of another pending move.
		progress := false
		for i := range moves {
			if moves[i].done {
				continue
			}
			dstNeeded := false
			for j := range moves {
				if j != i && !moves[j].done && moves[j].src == moves[i].dst {
					dstNeeded = true
					break
				}
			}
			if !dstNeeded {
				g.emitSingleCopy(moves[i].src, moves[i].dst, moves[i].ty)
				moves[i].done = true
				progress = true
				break
			}
		}
		if progress {
			continue
		}

		// All remaining moves form cycles.  Find first undone.
		first := -1
		undone := 0
		for i := range moves {
			if !moves[i].done {
				undone++
				if first == -1 {
					first = i
				}
			}
		}
		if first == -1 {
			break
		}

		// Special case: HL↔DE two-node cycle → EX DE, HL.
		if undone == 2 {
			a, b := -1, -1
			for i := range moves {
				if !moves[i].done {
					if a == -1 {
						a = i
					} else {
						b = i
					}
				}
			}
			as, ad := moves[a].src, moves[a].dst
			bs, bd := moves[b].src, moves[b].dst
			if (as == "HL" && ad == "DE" && bs == "DE" && bd == "HL") ||
				(as == "DE" && ad == "HL" && bs == "HL" && bd == "DE") {
				g.emit("    EX DE, HL")
				moves[a].done = true
				moves[b].done = true
				continue
			}
		}

		// General cycle: break with a temporary.
		m := &moves[first]
		if m.ty.Width() <= 8 {
			// u8 cycle: break by saving the first node's source to a scratch register.
			//
			// Walk BACKWARD through the cycle so we always read old values.
			// Example 3-cycle D→C, C→E, E→D (scratch=A):
			//   LD A, D     (save D into scratch)
			//   LD D, E     (backward: who writes TO D? E→D. E still original.)
			//   LD E, C     (backward: who writes TO E? C→E. C still original.)
			//   LD C, A     (put D_old into C via scratch)
			//
			// When A is part of the cycle, using A as scratch would overwrite it
			// during the walk before the final restore.  Pick the first 8-bit
			// register not involved in any pending move instead.
			cycleRegs := map[string]bool{}
			for i := range moves {
				if !moves[i].done {
					cycleRegs[moves[i].src] = true
					cycleRegs[moves[i].dst] = true
				}
				// Also mark done-move destinations as live (they hold values).
				if moves[i].done {
					cycleRegs[moves[i].dst] = true
				}
			}
			scratch := "A"
			if cycleRegs["A"] {
				for _, r := range []string{"C", "E", "H", "L", "D", "B"} {
					if !cycleRegs[r] {
						scratch = r
						break
					}
				}
			}

			if cycleRegs[scratch] {
				// All seven byte registers are occupied. Snapshot the pending
				// sources on the stack; shadow AF protects the main accumulator.
				var pending []int
				for i := range moves {
					if moves[i].done {
						continue
					}
					pending = append(pending, i)
					if moves[i].src != "A" {
						g.emit("    EX AF, AF'")
						g.emitLD8("A", moves[i].src)
					}
					g.emit("    PUSH AF")
					if moves[i].src != "A" {
						g.emit("    EX AF, AF'")
					}
				}
				for j := len(pending) - 1; j >= 0; j-- {
					i := pending[j]
					if moves[i].dst != "A" {
						g.emit("    EX AF, AF'")
					}
					g.emit("    POP AF")
					if moves[i].dst != "A" {
						g.emitLD8(moves[i].dst, "A")
						g.emit("    EX AF, AF'")
					}
					moves[i].done = true
				}
				continue
			}

			firstDst := m.dst
			if m.src != scratch {
				g.emitSingleCopy(m.src, scratch, m.ty)
			}
			m.done = true
			cur := m.src // start at the freed slot (m.src saved to scratch)
			for {
				found := false
				for i := range moves {
					if !moves[i].done && moves[i].dst == cur {
						g.emitSingleCopy(moves[i].src, moves[i].dst, moves[i].ty)
						cur = moves[i].src // the source is the new freed slot
						moves[i].done = true
						found = true
						break
					}
				}
				if !found {
					break
				}
			}
			if firstDst != scratch {
				g.emitSingleCopy(scratch, firstDst, m.ty)
			}
		} else {
			// Walk backwards around a word cycle, just as for byte cycles.
			// Saving src frees src, not dst: BC->HL, HL->DE, DE->BC
			// becomes PUSH BC; BC=DE; DE=HL; POP HL.
			if isSpill(m.src) {
				g.emit("    PUSH HL")
				g.loadSpill16("HL", m.src)
				g.emit("    EX (SP), HL")
			} else {
				g.emitf("    PUSH %s", m.src)
			}
			m.done = true
			cur := m.src
			for {
				found := false
				for i := range moves {
					if !moves[i].done && moves[i].dst == cur {
						g.emitSingleCopy(moves[i].src, moves[i].dst, moves[i].ty)
						cur = moves[i].src
						moves[i].done = true
						found = true
						break
					}
				}
				if !found {
					break
				}
			}
			if isSpill(m.dst) {
				g.emit("    EX (SP), HL")
				g.storeSpill16(m.dst, "HL")
				g.emit("    POP HL")
			} else {
				g.emitf("    POP %s", m.dst)
			}
		}
	}
	// After all register-to-register moves are resolved, emit constant assignments.
	// Done last so they cannot clobber live source registers.
	for _, c := range immCopies {
		if c.dstName == "F" {
			// F register: use AND A (clear=0) or SCF (set=1).
			if c.immVal == 0 {
				g.emit("    AND A") // clears C, sets Z
			} else {
				g.emit("    SCF") // sets C
			}
		} else if isSpill(c.dstName) {
			g.emit("    EX AF, AF'")
			for i := 0; i < z80SpillBytes(c.ty); i++ {
				g.emitf("    LD A, %d", (c.immVal>>(8*i))&0xFF)
				addr := c.dstName
				if i > 0 {
					addr += fmt.Sprintf("+%d", i)
				}
				g.emitf("    LD (%s), A", addr)
			}
			g.emit("    EX AF, AF'")
		} else if c.ty.Width() <= 8 {
			g.emitf("    LD %s, %d", c.dstName, c.immVal&0xFF)
		} else {
			g.emitf("    LD %s, %d", c.dstName, c.immVal&0xFFFF)
		}
	}
}

// emitSingleCopy emits a non-destructive register copy (src → dst).
// For 16-bit, this emits two LD instructions (not EX DE,HL which is a swap).
func (g *z80cg) emitSingleCopy(src, dst string, ty Ty) {
	if src == dst {
		return
	}
	if isZ80WideInt(ty) {
		g.emitMov32(dst, src)
		if ty.Width() == 24 && isPairReg(dst) {
			g.emit("    EXX")
			g.emitLD8(highByte(dst), "0")
			g.emit("    EXX")
		}
		return
	}
	// F register: cannot be accessed directly. Materialise flag→register or
	// register→flag via the same logic as emitMov.
	if src == "F" {
		if g.emitKnownFlagCopy(dst, ty.Width()) {
			return
		}
		// Flag → register: SBC A,A materialises carry into A (0xFF/0x00).
		if dst == "A" {
			g.emit("    SBC A, A")
		} else if isPairReg(dst) {
			// Flag → 16-bit pair: materialise + zero-extend.
			g.emit("    SBC A, A")
			g.emitf("    LD %s, A", lowByte(dst))
			g.emitf("    LD %s, 0", highByte(dst))
		} else {
			g.emit("    SBC A, A")
			g.emitLD8(dst, "A")
		}
		g.invalidate("A")
		return
	}
	if dst == "F" {
		// Register → flag: set Z/NZ from value via AND A.
		if src != "A" {
			g.emitf("    LD A, %s", src)
		}
		g.emit("    AND A")
		g.invalidate("A")
		return
	}
	if ty.Width() <= 8 {
		switch {
		case isSpill(src) && dst == "A":
			g.emitf("    LD A, (%s)", src)
		case isSpill(src):
			g.emitf("    LD A, (%s)", src)
			g.emitLD8(dst, "A")
			g.invalidate("A")
		case isSpill(dst) && src == "A":
			g.emitf("    LD (%s), A", dst)
		case isSpill(dst):
			g.emitLDA(src)
			g.emitf("    LD (%s), A", dst)
		case (isIXYReg(src) && (dst == "H" || dst == "L")) ||
			(isIXYReg(dst) && (src == "H" || src == "L")):
			// DD/FD prefix conflict: LD H,IXH encodes as LD IXH,IXH (NOP).
			// Route through shadow A to preserve main A.
			g.emitMovViaAltA(dst, src)
		case isPairReg(src) && !isPairReg(dst):
			// Width mismatch: 16-bit source → 8-bit dest. Truncate (take low byte).
			lo := lowByte(src)
			if (isIXYReg(lo) && (dst == "H" || dst == "L")) ||
				((lo == "H" || lo == "L") && isIXYReg(dst)) {
				g.emitMovViaAltA(dst, lo)
			} else {
				g.emitLD8(dst, lo)
			}
		case !isPairReg(src) && isPairReg(dst):
			// Width mismatch: 8-bit source → 16-bit dest. Zero-extend.
			lo := lowByte(dst)
			if (isIXYReg(src) && (lo == "H" || lo == "L")) ||
				((src == "H" || src == "L") && isIXYReg(lo)) {
				g.emitMovViaAltA(lo, src)
			} else {
				g.emitLD8(lo, src)
			}
			g.emitf("    LD %s, 0", highByte(dst))
		default:
			g.emitLD8(dst, src)
		}
	} else if isSpill(src) && isSpill(dst) {
		// Spill-to-spill: route through HL.
		g.loadSpill16("HL", src)
		g.storeSpill16(dst, "HL")
		g.invalidate("HL")
	} else if isSpill(src) {
		// LocMem spill → register: use loadSpill16 (handles TSMC).
		if isPairReg(dst) {
			g.loadSpill16(dst, src)
		} else {
			// Spill → 8-bit: load low byte only
			g.loadSpill8(dst, src)
		}
	} else if isSpill(dst) {
		// Register → LocMem spill: use storeSpill16 (handles TSMC).
		if isPairReg(src) {
			g.storeSpill16(dst, src)
		} else {
			// 8-bit → spill
			g.storeSpill8(dst, src)
		}
	} else if isPairReg(src) && !isPairReg(dst) {
		// Width mismatch in 16-bit context: pair→8bit truncation.
		lo := lowByte(src)
		if (isIXYReg(lo) && (dst == "H" || dst == "L")) ||
			((lo == "H" || lo == "L") && isIXYReg(dst)) {
			g.emitMovViaAltA(dst, lo)
		} else {
			g.emitLD8(dst, lo)
		}
	} else if !isPairReg(src) && isPairReg(dst) {
		// Width mismatch in 16-bit context: 8bit→pair zero-extension.
		lo := lowByte(dst)
		if (isIXYReg(src) && (lo == "H" || lo == "L")) ||
			((src == "H" || src == "L") && isIXYReg(lo)) {
			g.emitMovViaAltA(lo, src)
		} else {
			g.emitLD8(lo, src)
		}
		g.emitf("    LD %s, 0", highByte(dst))
	} else {
		// 16-bit non-destructive copy: two 8-bit LDs.
		hi := highByte(dst)
		lo := lowByte(dst)
		hiS := highByte(src)
		loS := lowByte(src)
		// DD/FD prefix conflict: LD H,IXH / LD L,IXL encode as NOPs.
		// IX↔HL copies must use PUSH/POP or route through A.
		if (isIXY(src) && dst == "HL") || (isIXY(dst) && src == "HL") {
			g.emitf("    PUSH %s", src)
			g.emitf("    POP %s", dst)
		} else if (isIXYReg(hiS) && (hi == "H" || hi == "L")) ||
			(isIXYReg(loS) && (lo == "H" || lo == "L")) ||
			((hiS == "H" || hiS == "L") && isIXYReg(hi)) ||
			((loS == "H" || loS == "L") && isIXYReg(lo)) {
			// Route through shadow A to avoid clobbering main A.
			g.emit("    EX AF, AF'")
			g.emitf("    LD A, %s", hiS)
			g.emitLD8(hi, "A")
			g.emitf("    LD A, %s", loS)
			g.emitLD8(lo, "A")
			g.emit("    EX AF, AF'")
		} else {
			g.emitLD8(hi, hiS)
			g.emitLD8(lo, loS)
		}
	}
}
