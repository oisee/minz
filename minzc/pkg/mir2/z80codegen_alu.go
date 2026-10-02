package mir2

import (
	"fmt"
	"maps"
	"math/bits"
	"slices"
)

// emitADDHL emits ADD HL, rr with IX/IY guard.
// Z80 ADD HL only accepts BC/DE/HL/SP — NOT IX/IY.
func (g *z80cg) emitADDHL(rhs string) {
	if rhs == "IX" || rhs == "IY" {
		g.emit("    PUSH DE")
		if rhs == "IX" {
			g.emit("    LD D, IXH")
			g.emit("    LD E, IXL")
		} else {
			g.emit("    LD D, IYH")
			g.emit("    LD E, IYL")
		}
		g.emit("    ADD HL, DE")
		g.emit("    POP DE")
	} else if isSpill(rhs) {
		g.emitf("    LD BC, (%s)", rhs)
		g.emit("    ADD HL, BC")
	} else {
		g.emitf("    ADD HL, %s", rhs)
	}
}

// emitSBCHL emits SBC HL, rr with IX/IY/spill/8-bit guard.
// Z80 SBC HL only accepts BC/DE/HL/SP — NOT IX/IY or 8-bit.
func (g *z80cg) emitSBCHL(rhs string) {
	if rhs == "IX" || rhs == "IY" {
		g.emit("    PUSH DE")
		if rhs == "IX" {
			g.emit("    LD D, IXH")
			g.emit("    LD E, IXL")
		} else {
			g.emit("    LD D, IYH")
			g.emit("    LD E, IYL")
		}
		g.emit("    SBC HL, DE")
		g.emit("    POP DE")
	} else if isSpill(rhs) {
		g.emitf("    LD BC, (%s)", rhs)
		g.emit("    SBC HL, BC")
	} else if isSimpleReg(rhs) && !isPairReg(rhs) {
		// 8-bit operand: promote to pair via promote8toPair.
		pair := g.promote8toPair(rhs)
		g.emitf("    SBC HL, %s", pair)
	} else {
		g.emitf("    SBC HL, %s", rhs)
	}
}

// ── Binary operations ─────────────────────────────────────────────────────────

func (g *z80cg) genBinOp(mnem string, inst *Inst) {
	dst := g.loc(inst.Dst)
	lhs := g.loc(inst.Src[0])
	rhs := g.loc(inst.Src[1])
	rhsReg := inst.Src[1]
	w := inst.Ty.Width()

	if w == 16 {
		defer g.saveWrittenScratch(inst)()
	}

	if w >= 24 {
		g.genBinOp32(mnem, dst, lhs, rhs)
		return
	}

	// Word operations also use HL as a working destination. Preserve a rhs
	// there before copying lhs, regardless of the allocated result location.
	if w == 16 {
		rhsValue, rhsConstant := g.constVals[rhsReg]
		inPlaceUnit := rhsConstant && rhsValue == 1 && lhs == dst
		if (mnem == "ADD" && rhs == "HL" && !inPlaceUnit) || ((mnem == "AND" || mnem == "OR" || mnem == "XOR") && rhs == dst && !rhsConstant) {
			lhs, rhs = rhs, lhs
			rhsReg = inst.Src[0]
		}
		if mnem == "SUB" && ((rhs == dst && lhs != dst) || (rhs == "HL" && lhs != "HL")) {
			g.pushWord(lhs)
			g.pushWord(rhs)
			g.emit("    POP DE")
			g.emit("    POP HL")
			g.emit("    OR A")
			g.emit("    SBC HL, DE")
			g.emitMov(dst, "HL", 16)
			g.invalidate(dst)
			return
		}
	}

	if w <= 8 {
		if lhs == dst && dst != "A" && dst != "F" && !isSpill(dst) {
			if cv, ok := g.constVals[inst.Src[1]]; ok {
				switch mnem {
				case "OR":
					mask := uint16(cv)
					if bits.OnesCount16(mask) == 1 {
						bit := bits.TrailingZeros16(mask)
						if bit < 8 && g.emitBitRegOp("SET", bit, dst) {
							g.invalidate(dst)
							return
						}
					}
				case "AND":
					mask := uint16(cv)
					if bits.OnesCount16(^mask) == 1 {
						bit := bits.TrailingZeros16(^mask)
						if bit < 8 && g.emitBitRegOp("RES", bit, dst) {
							g.invalidate(dst)
							return
						}
					}
				}
			}
		}

		// Special case: boolean OR with a ClassFlag operand — Z80 cannot LD A, F
		// or OR F.  Materialize the flag condition as 0/1 via a conditional skip:
		//   (ensure A holds the non-flag operand)
		//   JR Ncc, skip    ; if condition false, skip setting bit 0
		//   OR 1            ; A |= 1 (condition true)
		// skip:
		// Also treat a source as a flag if it's the pending carry-flag result
		// (g.loc() may return "A" because PBQP assigned it there, but the
		// actual value is still live in the carry flag via pendingFlagReg).
		isPendingFlag0 := len(inst.Src) > 0 && inst.Src[0] == g.pendingFlagReg
		isPendingFlag1 := len(inst.Src) > 1 && inst.Src[1] == g.pendingFlagReg
		if mnem == "OR" && (lhs == "F" || rhs == "F" || isPendingFlag0 || isPendingFlag1) {
			var flagSrc Reg
			var otherLoc string
			if lhs == "F" || isPendingFlag0 {
				flagSrc = inst.Src[0]
				otherLoc = rhs
			} else {
				flagSrc = inst.Src[1]
				otherLoc = lhs
			}
			// Consume the pending flag tracking for flagSrc.
			if flagSrc == g.pendingFlagReg {
				g.pendingFlagReg = NoReg
			}
			if otherLoc != "A" && !g.holdsValue("A", otherLoc) {
				g.emitLDA(otherLoc)
			}
			g.invalidate("A")
			cc := g.condCode(g.fn, flagSrc)
			skipLbl := fmt.Sprintf(".%s_orf_%d_sk", sanitizeIdent(g.fn.Name), int(inst.Dst))
			g.emitf("    JR %s, %s", invertCC(cc), skipLbl)
			g.emit("    OR 1")
			g.emitf("%s:", skipLbl)
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			}
			// Result is in A (or dst). Track it as a pending acc value in case
			// a subsequent genCmp overwrites A with "LD A, imm".
			if dst == "A" {
				g.pendingAccReg = inst.Dst
			}
			return
		}

		// If there is a pending ClassAcc value in A that is still live after this
		// instruction, save it to a scratch register before we overwrite A.
		// This fires when two successive 8-bit ALU ops both need A: the first
		// result (r33=acc) must survive while the second (r35=i+1) runs.
		// Skip when the peephole below will use INC/DEC (does not touch A).
		// INC/DEC doesn't clobber A — only fires when rhs is a small constant.
		_, rhsIsConst := g.constVals[inst.Src[1]]
		willUseINCDEC := (mnem == "ADD" || mnem == "SUB") && lhs == dst && rhsIsConst
		if !willUseINCDEC {
			g.materializePendingAcc(inst)
		}

		// Peephole: ADD/SUB dst, N where dst == lhs and N ≤ 3 → INC/DEC dst × N.
		// N=1: 1B/4T vs 4B/15T; N=2: 2B/8T vs 4B/15T; N=3: 3B/12T vs 4B/15T.
		// N=4 would be 16T vs 15T — worse in T-states, skip and fall through to ALU.
		if mnem == "ADD" && lhs == dst && !isSpill(dst) && dst != "F" {
			if cv, ok := g.constVals[inst.Src[1]]; ok && cv >= 1 && cv <= 3 {
				for range cv {
					g.emitf("    INC %s", dst)
				}
				g.invalidate(dst)
				return
			}
		}
		if mnem == "SUB" && lhs == dst && !isSpill(dst) && dst != "F" {
			if cv, ok := g.constVals[inst.Src[1]]; ok && cv >= 1 && cv <= 3 {
				for range cv {
					g.emitf("    DEC %s", dst)
				}
				g.invalidate(dst)
				return
			}
		}
		// 8-bit: accumulator must hold lhs.  Emit LD A,lhs if needed.
		//
		// Special case: rhs is already A and lhs is elsewhere.
		// Loading lhs into A would destroy the rhs value.
		//   ADD: commutative → just ADD A, lhs.
		//   SUB: A = lhs - A → NEG first (A = -A), then ADD A, lhs.
		if rhs == "A" && lhs != "A" {
			// The result overwrites A, which holds rhs: save rhs if it is
			// still live (gcd's b = b - a keeps a live in A for the loop).
			g.saveAccOperandIfLive(inst.Src[1], inst)
			g.invalidate("A") // A about to hold a new result
			switch mnem {
			case "ADD":
				g.emitf("    ADD A, %s", lhs)
			case "AND", "OR", "XOR":
				g.emit8ALU(mnem, lhs)
			case "SUB":
				g.emit("    NEG")
				g.emitf("    ADD A, %s", lhs)
			default:
				g.comment(fmt.Sprintf("TODO: 8-bit %s %s, A → %s with A as rhs", mnem, lhs, dst))
				g.emitLDA(lhs)
				g.emit8ALU(mnem, rhs)
			}
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			} else {
				g.pendingAccReg = inst.Dst
			}
			return
		}
		// Coalescing: if A already holds lhs or (commutative) rhs, skip LD A, reg.
		isCommutative := mnem == "ADD" || mnem == "AND" || mnem == "OR" || mnem == "XOR"
		if g.holdsValue("A", lhs) {
			// Save-before-overwrite: if the lhs vreg in A is still live after this
			// instruction (used by a later inst or terminator), save it to a scratch
			// register. The ALU op will overwrite A with the result.
			// Example: fib's r6=sub(r0,1) where r0 is still needed for r9=sub(r0,2).
			g.saveAccOperandIfLive(inst.Src[0], inst)
			// A == lhs already — skip LD A, lhs.
			g.invalidate("A")
			// Use immediate form if rhs is a known constant.
			if cv, ok := g.constVals[inst.Src[1]]; ok {
				g.emit8ALUImm(mnem, cv)
			} else {
				g.emit8ALU(mnem, rhs)
			}
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			} else {
				g.pendingAccReg = inst.Dst
			}
			return
		}
		if isCommutative && g.holdsValue("A", rhs) {
			// A == rhs, commutative swap: emit OP A, lhs instead.
			g.saveAccOperandIfLive(inst.Src[1], inst)
			g.invalidate("A")
			// Use immediate form if lhs is a known constant.
			if cv, ok := g.constVals[inst.Src[0]]; ok {
				g.emit8ALUImm(mnem, cv)
			} else {
				g.emit8ALU(mnem, lhs)
			}
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			} else {
				g.pendingAccReg = inst.Dst
			}
			return
		}

		// Immediate peephole: AND/OR/XOR/ADD/SUB n — no register needed for rhs.
		if cv, ok := g.constVals[inst.Src[1]]; ok {
			if lhs != "A" {
				// Before loading lhs into A, save any live vreg currently in A.
				// Example: fib — after first CALL, A=fib(n-1) is needed later but
				// computing n-2 requires loading n (from scratch) into A.
				g.saveABeforeOverwrite(inst)
				g.emitLDA(lhs)
			}
			switch mnem {
			case "AND", "OR", "XOR", "SUB":
				g.emitf("    %s %d", mnem, cv)
			case "ADD":
				g.emitf("    ADD A, %d", cv)
			default:
				g.emit8ALU(mnem, rhs)
			}
			g.invalidate("A") // ALU result is a new value — break old lhs alias
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			} else {
				g.pendingAccReg = inst.Dst
			}
			return
		}
		// If A holds a live value from a different virtual reg (e.g. a function
		// param allocated to A), and this ALU op will clobber A as scratch,
		// we need to save it first. The pendingAccReg mechanism handles the
		// common case, but there's also the case where a param (ClassAcc) lives
		// in A and is used AFTER this instruction. In that case, relocate it
		// to a scratch register before clobbering A.
		if lhs != "A" {
			g.saveABeforeOverwrite(inst)
			// A will be clobbered (LD A,lhs then ALU). If A holds a value
			// that's used later, save it.
			g.saveAccIfLive(inst)
		}
		if lhs != "A" {
			g.emitLDA(lhs)
		}
		g.emit8ALU(mnem, rhs)
		g.invalidate("A") // ALU result is a new value — break old lhs alias
		if dst != "A" {
			g.emitLD8(dst, "A")
			g.setCopy(dst, "A")
		} else {
			g.pendingAccReg = inst.Dst
		}
	} else {
		if lhs == dst && dst != "A" && !isSpill(dst) {
			if cv, ok := g.constVals[inst.Src[1]]; ok {
				switch mnem {
				case "OR":
					mask := uint16(cv)
					if bits.OnesCount16(mask) == 1 {
						totalBit := bits.TrailingZeros16(mask)
						target := selectBitReg(dst, totalBit/8)
						if g.emitBitRegOp("SET", totalBit%8, target) {
							g.invalidate(dst)
							g.invalidate(target)
							return
						}
					}
				case "AND":
					mask := uint16(cv)
					if bits.OnesCount16(^mask) == 1 {
						totalBit := bits.TrailingZeros16(^mask)
						target := selectBitReg(dst, totalBit/8)
						if g.emitBitRegOp("RES", totalBit%8, target) {
							g.invalidate(dst)
							g.invalidate(target)
							return
						}
					}
				}
			}
		}

		// 16-bit peephole: INC/DEC rr when adding/subtracting 1 in-place.
		if rhsReg == inst.Src[1] && lhs == dst && !isSpill(dst) && dst != "F" {
			if cv, ok := g.constVals[inst.Src[1]]; ok {
				if mnem == "ADD" && cv == 1 {
					g.emitf("    INC %s", dst)
					g.invalidate(dst) // pair + its bytes all change
					return
				}
				if mnem == "SUB" && cv == 1 {
					g.emitf("    DEC %s", dst)
					g.invalidate(dst)
					return
				}
			}
		}
		// 16-bit ADD: ADD HL,rr  (Z80 only has ADD HL,rr natively)
		// SUB/AND/OR/XOR on 16-bit require workarounds; emit comment for now.
		switch mnem {
		case "ADD":
			// Z80 only has ADD HL, rr.  If dst is not HL we must route
			// through HL.  The common case is dst=DE, which can be handled
			// cheaply with EX DE,HL:
			//   EX DE, HL          ; HL ← lhs (was in DE), DE ← old HL
			//   ADD HL, <rhs'>     ; rhs' adjusted for any EX swap
			//   EX DE, HL          ; DE ← result, HL ← restored old value
			if dst != "HL" {
				// After EX DE,HL: the register formerly in DE is now in HL,
				// and what was in HL is now in DE.
				adjustedRhs := rhs
				if lhs == "DE" && dst == "DE" {
					g.emit("    EX DE, HL")
					if rhs == "HL" {
						adjustedRhs = "DE" // what was HL is now in DE after EX
					} else if rhs == "DE" {
						adjustedRhs = "HL"
					}
					if adjustedRhs == "IX" || adjustedRhs == "IY" {
						g.emit("    PUSH BC")
						g.emitMov("BC", adjustedRhs, w)
						g.emit("    ADD HL, BC")
						g.emit("    POP BC")
					} else if isSimpleReg(adjustedRhs) && !isPairReg(adjustedRhs) {
						pair := g.promote8toPair(adjustedRhs)
						g.emitADDHL(pair)
					} else if isSpill(adjustedRhs) {
						g.loadSpill16("BC", adjustedRhs)
						g.emit("    ADD HL, BC")
					} else if adjustedRhs == "IX" || adjustedRhs == "IY" {
						g.emit("    PUSH DE")
						g.emitMov("DE", adjustedRhs, 16)
						g.emit("    ADD HL, DE")
						g.emit("    POP DE")
					} else {
						g.emitADDHL(adjustedRhs)
					}
					g.emit("    EX DE, HL")
					g.invalidate("HL")
					g.invalidate("DE")
					break
				}
				// General fallback: move lhs to HL, add rhs, move result to dst.
				// Track rhs renaming if lhs is moved from HL/DE.
				if rhs == "HL" && lhs == "DE" {
					adjustedRhs = "DE" // lhs/dst swap via EX
				}
				g.emitMov("HL", lhs, w)
				if adjustedRhs == "IX" || adjustedRhs == "IY" {
					g.emit("    PUSH DE")
					g.emitMov("DE", adjustedRhs, w)
					g.emit("    ADD HL, DE")
					g.emit("    POP DE")
				} else if isSimpleReg(adjustedRhs) && !isPairReg(adjustedRhs) {
					pair := g.promote8toPair(adjustedRhs)
					g.emitADDHL(pair)
				} else if isSpill(adjustedRhs) {
					g.loadSpill16("BC", adjustedRhs)
					g.emit("    ADD HL, BC")
				} else {
					g.emitADDHL(adjustedRhs)
				}
				g.emitMov(dst, "HL", w)
				g.invalidate(dst)
				break
			}
			// Standard path: dst == HL.
			if lhs != dst {
				g.emitMov(dst, lhs, w)
			}
			// ADD HL, rr only accepts BC/DE/HL/SP — not IX/IY or 8-bit regs.
			if rhs == "IX" || rhs == "IY" {
				g.emit("    PUSH DE")
				g.emitMov("DE", rhs, w)
				g.emit("    ADD HL, DE")
				g.emit("    POP DE")
			} else if isSimpleReg(rhs) && !isPairReg(rhs) {
				// 8-bit rhs: zero-extend to parent pair, then ADD HL, pair.
				pair := g.promote8toPair(rhs)
				g.emitADDHL(pair)
			} else if isSpill(rhs) {
				// LocMem spill: load to BC, then ADD HL, BC.
				g.loadSpill16("BC", rhs)
				g.emit("    ADD HL, BC")
			} else {
				// dst must be HL here; ADD HL, rr is the only valid 16-bit ADD.
				g.emitADDHL(rhs)
			}
			g.invalidate(dst)
		case "SUB":
			// Z80 only has SBC HL, rr for 16-bit subtraction.
			// When dst != "HL" we must route through HL; the only legal instruction is
			// SBC HL, rr, so the general strategy is:
			//   EX DE,HL  (put lhs in HL, save rhs in DE — only when both are DE/HL)
			//   OR A
			//   SBC HL, rhs'
			//   EX DE,HL  (restore HL=rhs, put result in DE=dst)
			if dst != "HL" && lhs == dst && rhs == "HL" && dst == "DE" {
				// dst=DE, lhs=DE (0 or anything), rhs=HL.
				// EX DE,HL swaps: HL=lhs, DE=rhs.  SBC HL,DE = lhs-rhs.
				// Second EX DE,HL: DE=result, HL=rhs restored.
				g.emit("    EX DE, HL")
				g.emit("    OR A")
				g.emit("    SBC HL, DE")
				g.emit("    EX DE, HL")
				g.invalidate("HL")
				g.invalidate("DE")
				return
			}
			if lhs != dst {
				// When lhs and dst are the HL/DE pair AND the rhs is the other member,
				// use EX DE,HL (a true swap, 4T) to move lhs→dst while simultaneously
				// placing the old dst contents into lhs's old location.
				// This keeps rhs accessible in the swapped location.
				// Note: emitMov(HL,DE) does byte-by-byte (LD H,D; LD L,E) — it preserves
				// DE but does NOT produce the swap that the rhs update below relies on.
				if (dst == "HL" && lhs == "DE") || (dst == "DE" && lhs == "HL") {
					g.emit("    EX DE, HL")
					// After EX DE,HL: whatever was in HL is now in DE, and vice versa.
					switch rhs {
					case "HL":
						rhs = "DE"
					case "DE":
						rhs = "HL"
					}
				} else {
					g.emitMov(dst, lhs, w)
				}
			}
			g.emit("    OR A") // clear carry (1b/4T; SCF+CCF would be 2b/8T)
			if dst == "DE" && (rhs == "HL" || rhs == "BC" || rhs == "DE") {
				// Optimal path: EX DE,HL; SBC HL,rhs'; EX DE,HL (23T)
				// After EX: HL=old_DE(lhs), DE=old_HL
				adjustedRhs := rhs
				if rhs == "HL" {
					adjustedRhs = "DE" // old HL is now in DE
				} else if rhs == "DE" {
					adjustedRhs = "HL" // self-subtraction after EX: lhs is now HL
				}
				g.emit("    EX DE, HL")
				g.emitSBCHL(adjustedRhs)
				g.emit("    EX DE, HL") // result back to DE, HL restored
				g.invalidate("HL")
				g.invalidate("DE")
			} else if dst != "HL" {
				// General non-HL: route through HL via emitMov.
				g.emitMov("HL", dst, w)
				if rhs == "HL" {
					rhs = dst
				}
				if isSpill(rhs) {
					g.loadSpill16("BC", rhs)
					rhs = "BC"
				}
				g.emitSBCHL(rhs)
				g.emitMov(dst, "HL", w)
				g.invalidate("HL")
			} else {
				if isSpill(rhs) {
					g.loadSpill16("BC", rhs)
					rhs = "BC"
				}
				g.emitSBCHL(rhs)
			}
			g.invalidate(dst)
		case "OR", "AND", "XOR":
			if lhs != dst {
				g.emitMov(dst, lhs, w)
			}
			hi, lo := highByte(dst), lowByte(dst)
			imm, constant := g.constVals[rhsReg]
			for _, part := range []struct {
				dst, src string
				shift    uint
			}{{hi, highByte(rhs), 8}, {lo, lowByte(rhs), 0}} {
				g.emitLDA(part.dst)
				if constant {
					g.emit8ALUImm(mnem, imm>>part.shift)
				} else {
					g.emit8ALU(mnem, part.src)
				}
				g.invalidate("A")
				g.emitLD8(part.dst, "A")
			}
			g.invalidate(dst)

		default:
			g.comment(fmt.Sprintf("TODO: 16-bit %s %s, %s → %s", mnem, lhs, rhs, dst))
		}
	}
}

// genBinOp32 emits 32-bit ADD or SUB using the Z80 EXX / shadow-pair technique.
//
// The 32-bit value is split across main and shadow register banks:
//
//	LocDWord{"HL"}: main HL = lo16, shadow H'L' = hi16 (via EXX)
//
// For ADD (dst=HL, rhs=DE):
//
//	ADD HL, DE    ; lo ← lo(lhs) + lo(rhs), carry set if overflow  (11T)
//	EXX           ; switch to shadow bank: HL←H'L', DE←D'E'        (4T)
//	ADC HL, DE    ; hi ← hi(lhs) + hi(rhs) + carry                 (15T)
//	EXX           ; switch back to main bank                         (4T)
//
// For SUB (dst=HL, rhs=DE):
//
//	AND A         ; clear carry                                      (4T)
//	SBC HL, DE   ; lo ← lo(lhs) - lo(rhs), borrow in carry         (15T)
//	EXX           ; switch to shadow bank                            (4T)
//	SBC HL, DE   ; hi ← hi(lhs) - hi(rhs) - borrow                 (15T)
//	EXX           ; switch back                                       (4T)
//
// EXX simultaneously swaps BC↔BC', DE↔DE', HL↔HL', so after EXX:
//   - "HL" refers to the old H'L' (hi of lhs/dst)
//   - "DE" refers to the old D'E' (hi of rhs)
//
// This works because both dst and rhs are LocDWord with the same name in both
// banks.  The rhs DWord is restored to its original state after the final EXX.
//
// Limitation: Z80 only provides ADD/ADC/SBC with HL as destination.
// When dst≠HL, we temporarily use HL via emitMov32 (TODO: optimise).
func (g *z80cg) genBinOp32(mnem, dst, lhs, rhs string) {
	// ADD is commutative: if rhs == dst and lhs != dst, swap so we don't clobber rhs.
	if mnem == "ADD" && rhs == dst && lhs != dst {
		lhs, rhs = rhs, lhs
	}

	// SUB when lhs != dst and rhs == dst: moving lhs into dst would clobber rhs.
	// Special case lhs=DE, dst=HL: use EX DE,HL to swap lo/hi words in place,
	// then SBC computes original_lhs - original_rhs in HL.
	if mnem == "SUB" && lhs != dst && rhs == dst {
		if lhs == "DE" && dst == "HL" {
			// EX DE,HL swaps main words; EXX + EX DE,HL + EXX swaps shadow words.
			// After: HL = old DE = lhs_lo, DE = old HL = rhs_lo  (and same in shadow).
			g.emit("    EX DE, HL")
			g.emit("    EXX")
			g.emit("    EX DE, HL")
			g.emit("    EXX")
			g.emit("    AND A")
			g.emit("    SBC HL, DE") // lhs_lo - rhs_lo
			g.emit("    EXX")
			g.emit("    SBC HL, DE") // lhs_hi - rhs_hi - borrow
			g.emit("    EXX")
			return
		}
		// General fallback: save rhs on stack, move lhs to dst, reload rhs to DE.
		// Only HL is supported as dst for now.
		g.comment(fmt.Sprintf("TODO: 32-bit SUB rhs==dst non-EX case: %s - %s → %s", lhs, rhs, dst))
		return
	}

	// Ensure lhs is in dst before the operation.
	if lhs != dst {
		g.emitMov32(dst, lhs)
	}

	switch mnem {
	case "ADD":
		if dst == "HL" {
			// Native 32-bit add: ADD HL,rr / EXX / ADC HL,rr / EXX
			g.emitADDHL(rhs)
			g.emit("    EXX")
			g.emitf("    ADC HL, %s", rhs)
			g.emit("    EXX")
		} else {
			// Non-HL dst: save HL, use it as scratch, then move result.
			g.comment(fmt.Sprintf("32-bit ADD via HL scratch: %s += %s", dst, rhs))
			g.emit("    PUSH HL") // save main HL
			g.emit("    EXX")
			g.emit("    PUSH HL") // save shadow HL
			g.emit("    EXX")
			// Transfer dst to HL.
			g.emitMov32("HL", dst)
			// Now add rhs (rhs was LocDWord; after emitMov32 we haven't clobbered it
			// because emitMov32 uses PUSH/POP+EXX which preserves all non-HL regs).
			g.emitADDHL(rhs)
			g.emit("    EXX")
			g.emitf("    ADC HL, %s", rhs)
			g.emit("    EXX")
			// Move result from HL to dst, then restore HL.
			g.emitMov32(dst, "HL")
			g.emit("    EXX")
			g.emit("    POP HL") // restore shadow HL
			g.emit("    EXX")
			g.emit("    POP HL") // restore main HL
		}
	case "SUB":
		if dst == "HL" {
			g.emit("    AND A") // clear carry
			g.emitSBCHL(rhs)
			g.emit("    EXX")
			g.emitSBCHL(rhs)
			g.emit("    EXX")
		} else {
			g.comment(fmt.Sprintf("32-bit SUB via HL scratch: %s -= %s", dst, rhs))
			g.emit("    PUSH HL")
			g.emit("    EXX")
			g.emit("    PUSH HL")
			g.emit("    EXX")
			g.emitMov32("HL", dst)
			g.emit("    AND A")
			g.emitSBCHL(rhs)
			g.emit("    EXX")
			g.emitSBCHL(rhs)
			g.emit("    EXX")
			g.emitMov32(dst, "HL")
			g.emit("    EXX")
			g.emit("    POP HL")
			g.emit("    EXX")
			g.emit("    POP HL")
		}
	default:
		g.comment(fmt.Sprintf("TODO: 32-bit %s %s, %s → %s", mnem, lhs, rhs, dst))
	}
}

// saveAccIfLive checks whether A contains a live value from a virtual register
// that will be needed AFTER the current instruction.  If so, it relocates
// A's value to a scratch register (via physOverride) so the ALU can use A freely.
func (g *z80cg) saveAccIfLive(inst *Inst) {
	// Find which virtual reg currently lives in A. Prefer the lowest ID
	// on ties, matching definition order and the allocator tie-break.
	var accReg Reg
	for _, r := range slices.Sorted(maps.Keys(g.ar.Locs)) {
		loc := g.ar.Locs[r]
		if loc.Kind == LocReg && loc.Name == "A" {
			accReg = r
			break
		}
	}
	if accReg == NoReg {
		return
	}
	// Check if physOverride already moved it away.
	if p, ok := g.physOverride[accReg]; ok && p != "A" {
		return
	}
	// Skip if this instruction defines or uses accReg (it'll be handled normally).
	if accReg == inst.Dst || accReg == inst.Src[0] || accReg == inst.Src[1] {
		return
	}
	// Check if accReg is used after this instruction in the block.
	live := g.regsLiveAfterInst(inst)
	if !live[accReg] {
		return
	}
	// accReg is live in A and will be clobbered — save to scratch.
	// Pick a scratch register that isn't used by this instruction.
	scratch := g.pickScratch8(inst)
	g.emitf("    LD %s, A", scratch)
	g.physOverride[accReg] = scratch
}

// reorderAccMoves swaps consecutive instruction pairs where a move→A is followed
// by an 8-bit ALU→nonA that will use A as scratch. By putting the ALU first,
// A is free for scratch use and the move loads A last (preserving it for the call).
func reorderAccMoves(insts []*Inst, ar *AllocResult) []*Inst {
	result := make([]*Inst, len(insts))
	copy(result, insts)

	for i := 0; i < len(result)-1; i++ {
		a, b := result[i], result[i+1]

		// Pattern: a = OpMove, dst in A; b = 8-bit ALU, dst NOT in A.
		if a.Op != OpMove {
			continue
		}
		aLoc := ar.Locs[a.Dst]
		if aLoc.Name != "A" {
			continue
		}

		// b must be an 8-bit ALU op that needs A as scratch.
		is8bitALU := (b.Op == OpAdd || b.Op == OpSub || b.Op == OpAnd ||
			b.Op == OpOr || b.Op == OpXor) && b.Ty != nil && b.Ty.Width() <= 8
		if !is8bitALU {
			continue
		}
		bLoc := ar.Locs[b.Dst]
		if bLoc.Name == "A" {
			continue // b also targets A — no help from swapping
		}

		// Safety: b must not READ a.Dst, and a must not READ b.Dst.
		bReadsA := b.Src[0] == a.Dst || b.Src[1] == a.Dst
		aReadsB := a.Src[0] == b.Dst
		if bReadsA || aReadsB {
			continue // data dependency — cannot swap
		}

		// Swap.
		result[i], result[i+1] = b, a
	}
	return result
}

// emit8ALU emits an 8-bit ALU instruction with A as the implicit destination.
// MZA requires "ADD A, src" (two operands) but "SUB/AND/OR/XOR src" (one operand).
func (g *z80cg) emit8ALU(mnem, src string) {
	// LocMem spill: Z80 ALU ops can't use absolute addresses directly.
	// Load spill value into a scratch register first.
	if isSpill(src) {
		// Load from memory into A first, but A is the implicit LHS for 8-bit ALU.
		// Must save A, load spill into scratch, restore A, then ALU.
		// Simpler: use (HL) indirect — save HL, point to spill, ALU (HL), restore.
		// Simplest: load spill into a scratch 8-bit reg.
		// For now, load into A via memory, which works if LHS is already in A.
		g.emit("    PUSH HL")
		g.emitf("    LD HL, %s", src)
		g.emitf("    LD H, (HL)") // H = value at spill address
		switch mnem {
		case "ADD", "ADC":
			g.emitf("    %s A, H", mnem)
		default:
			g.emitf("    %s H", mnem)
		}
		g.emit("    POP HL")
		g.invalidate("H")
		return
	}
	// Width mismatch: pair reg as 8-bit ALU operand — use low byte.
	if isPairReg(src) {
		src = lowByte(src)
	}
	switch mnem {
	case "ADD", "ADC":
		g.emitf("    %s A, %s", mnem, src)
	default:
		g.emitf("    %s %s", mnem, src)
	}
}

// emit8ALUImm emits an 8-bit ALU instruction with an immediate constant operand.
func (g *z80cg) emit8ALUImm(mnem string, imm int64) {
	imm8 := imm & 0xFF // Z80 8-bit ALU: truncate to byte
	switch mnem {
	case "ADD", "ADC":
		g.emitf("    %s A, %d", mnem, imm8)
	default:
		g.emitf("    %s %d", mnem, imm8)
	}
}

// ── Shifts ────────────────────────────────────────────────────────────────────

func (g *z80cg) genShift(mnem string, inst *Inst) {
	if _, constant := g.constVals[inst.Src[1]]; !constant {
		g.genVariableShift(mnem, inst)
		return
	}

	w := inst.Ty.Width()
	if w >= 24 {
		g.genShift32(mnem, inst)
		return
	}
	dst := g.loc(inst.Dst)
	src := g.loc(inst.Src[0])
	if dst != src {
		g.emitMov(dst, src, w)
	}
	// Shift count in Src[1] — Z80 shifts are always by 1 per instruction.
	// Constant counts emit N copies; variable counts use genVariableShift.
	// IMPORTANT: count=0 must emit zero shifts (identity), not default to 1.
	count := int64(1)
	if cv, ok := g.constVals[inst.Src[1]]; ok {
		count = cv // cv=0 → no shifts emitted (identity)
	}
	if w == 16 {
		// Z80 has no SLA/SRL/SRA on register pairs.
		// SHL/SHR/SAR on IX/IY halves are also invalid (DD CB prefix
		// always addresses (IX+d), never IXH/IXL directly).
		// Route IX/IY shifts through HL.
		shiftDst := dst
		if isIXY(dst) {
			g.emitMov("HL", dst, 16)
			shiftDst = "HL"
		}
		hi := highByte(shiftDst)
		lo := lowByte(shiftDst)
		if mnem == "SLA" && count == 8 {
			g.emitf("    LD %s, %s", hi, lo)
			g.emitf("    LD %s, 0", lo)
		} else {
			for i := int64(0); i < count; i++ {
				switch mnem {
				case "SLA":
					if shiftDst == "HL" {
						g.emitf("    ADD HL, HL")
					} else {
						g.emitf("    SLA %s", lo)
						g.emitf("    RL  %s", hi)
					}
				case "SRL":
					g.emitf("    SRL %s", hi)
					g.emitf("    RR  %s", lo)
				case "SRA":
					g.emitf("    SRA %s", hi)
					g.emitf("    RR  %s", lo)
				}
			}
		}
		if isIXY(dst) {
			g.emitMov(dst, "HL", 16)
		}
		g.invalidate(dst)
		if isIXY(dst) {
			g.invalidate("HL")
		}
		return
	}
	for i := int64(0); i < count; i++ {
		g.emitf("    %s %s", mnem, dst)
	}
	g.invalidate(dst) // shift modifies dst
}

// genVariableShift uses a saved scratch pair for the counter. Loading it
// before moving the value also handles count registers overlapping the result.
func (g *z80cg) genVariableShift(mnem string, inst *Inst) {
	dst, src := g.loc(inst.Dst), g.loc(inst.Src[0])
	w := inst.Ty.Width()
	pair, counter := "", ""
	for _, p := range []string{"BC", "DE", "HL"} {
		overlaps := func(loc string) bool { return loc == p || loc == highByte(p) || loc == lowByte(p) }
		if !overlaps(dst) && !overlaps(src) && !(isIXY(dst) && p == "HL") {
			pair = p
			counter = highByte(p)
			break
		}
	}
	if pair == "" {
		panic("no scratch pair for variable shift")
	}
	g.emitf("    PUSH %s", pair)
	g.emitLD8(counter, lowByte(g.loc(inst.Src[1])))
	if dst != src {
		g.emitMov(dst, src, w)
	}
	shiftDst := dst
	if isIXY(dst) {
		g.emit("    PUSH HL")
		g.emitMov("HL", dst, 16)
		shiftDst = "HL"
	}
	if isIXYReg(dst) {
		g.emit("    PUSH AF")
		g.emitLDA(dst)
		shiftDst = "A"
	}
	idx := g.trampIdx
	g.trampIdx++
	// INC/DEC test zero without touching the shifted value or accumulator.
	g.emitf("    INC %s", counter)
	g.emitf("    DEC %s", counter)
	g.emitf("    JR Z, .shift_done_%d", idx)
	g.emitf(".shift_loop_%d:", idx)
	if w >= 24 {
		hi, lo := highByte(shiftDst), lowByte(shiftDst)
		if mnem == "SLA" {
			g.emitf("    SLA %s", lo)
			g.emitf("    RL %s", hi)
			g.emit("    EXX")
			g.emitf("    RL %s", lo)
			g.emitf("    RL %s", hi)
			g.emit("    EXX")
		} else {
			g.emit("    EXX")
			if w == 24 {
				g.emitf("    %s %s", mnem, lo)
			} else {
				g.emitf("    %s %s", mnem, hi)
				g.emitf("    RR %s", lo)
			}
			g.emit("    EXX")
			g.emitf("    RR %s", hi)
			g.emitf("    RR %s", lo)
		}
	} else if w == 16 {
		hi, lo := highByte(shiftDst), lowByte(shiftDst)
		if mnem == "SLA" {
			g.emitf("    SLA %s", lo)
			g.emitf("    RL %s", hi)
		} else {
			g.emitf("    %s %s", mnem, hi)
			g.emitf("    RR %s", lo)
		}
	} else {
		g.emitf("    %s %s", mnem, shiftDst)
	}
	g.emitf("    DEC %s", counter)
	g.emitf("    JR NZ, .shift_loop_%d", idx)
	g.emitf(".shift_done_%d:", idx)
	if isIXY(dst) {
		g.emitMov(dst, "HL", 16)
		g.emit("    POP HL")
	}
	if isIXYReg(dst) {
		g.emitLD8(dst, "A")
		g.emit("    POP AF")
	}
	g.emitf("    POP %s", pair)
	g.invalidate(dst)
}

// genShift32 emits 24/32-bit shifts via the EXX shadow-pair technique.
//
// SHL by 1 (logical left):
//
//	ADD HL, HL    ; lo16 <<= 1, carry = old bit15  (11T)
//	EXX           ; switch to shadow                (4T)
//	ADC HL, HL    ; hi16 <<= 1, carry in from lo   (15T)
//	EXX           ; switch back                      (4T)  = 34T per bit
//
// SHR by 1 (logical right):
//
//	EXX           ; switch to shadow
//	SRL H         ; hi16: H >>= 1 (zero-extend), carry = old bit0  (8T)
//	RR  L         ; hi16: L = carry<<7 | L>>1                      (8T)
//	EXX           ; back to main
//	RR  H         ; lo16: H = carry<<7 | H>>1                      (8T)
//	RR  L         ; lo16: L = carry<<7 | L>>1                      (8T)  = 40T per bit
//
// SAR by 1 (arithmetic right, sign-extend):
//
//	EXX / SRA H / RR L / EXX / RR H / RR L  (same as SHR but SRA preserves sign)
func (g *z80cg) genShift32(mnem string, inst *Inst) {
	dst := g.loc(inst.Dst)
	src := g.loc(inst.Src[0])
	if dst != src {
		g.emitMov32(dst, src)
	}

	count := int64(1)
	if cv, ok := g.constVals[inst.Src[1]]; ok {
		count = cv
	}

	hi := highByte(dst) // e.g. "H" for dst="HL"
	lo := lowByte(dst)  // e.g. "L"

	switch mnem {
	case "SLA":
		// Left shift: ADD HL,HL / EXX / ADC HL,HL / EXX  (× count)
		// Z80 only supports ADD HL,HL — not ADD DE,DE or ADD BC,BC.
		// Route through HL if needed.
		if dst != "HL" {
			g.emitMov("HL", dst, 16)
		}
		for i := int64(0); i < count; i++ {
			g.emit("    ADD HL, HL")
			g.emit("    EXX")
			g.emit("    ADC HL, HL")
			g.emit("    EXX")
		}
		if dst != "HL" {
			g.emitMov(dst, "HL", 16)
			g.invalidate("HL")
		}
	case "SRL":
		// Logical right shift: EXX / SRL H / RR L / EXX / RR H / RR L  (× count)
		for i := int64(0); i < count; i++ {
			g.emit("    EXX")
			if inst.Ty.Width() == 24 {
				g.emitf("    SRL %s", lo)
			} else {
				g.emitf("    SRL %s", hi)
				g.emitf("    RR  %s", lo)
			}
			g.emit("    EXX")
			g.emitf("    RR  %s", hi)
			g.emitf("    RR  %s", lo)
		}
	case "SRA":
		// Arithmetic right shift: sign bit preserved via SRA on high byte.
		for i := int64(0); i < count; i++ {
			g.emit("    EXX")
			if inst.Ty.Width() == 24 {
				g.emitf("    SRA %s", lo)
			} else {
				g.emitf("    SRA %s", hi)
				g.emitf("    RR  %s", lo)
			}
			g.emit("    EXX")
			g.emitf("    RR  %s", hi)
			g.emitf("    RR  %s", lo)
		}
	default:
		g.comment(fmt.Sprintf("TODO: 32-bit shift %s", mnem))
	}
	g.invalidate(dst)
}
