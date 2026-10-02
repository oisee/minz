package mir2

import (
	"fmt"
	"strings"
)

// ── Multiply ──────────────────────────────────────────────────────────────────

// genMul emits 8-bit unsigned multiply via shift-and-add.
// Power-of-2 constants use N×(ADD A,A); small constants use LD tmp,A + adds.
// Variable or large constants use the __mul8 runtime.
func (g *z80cg) genMul(inst *Inst) {
	dst := g.loc(inst.Dst)
	lhs := g.loc(inst.Src[0])

	if inst.Ty.Width() >= 24 {
		g.genMul32(inst)
		return
	}
	if inst.Ty.Width() > 8 {
		g.genMul16(inst)
		return
	}

	// Preserve the accumulator value used after this multiply, including a
	// loop block parameter. Relocate before saving runtime scratch pairs.
	for r := range g.regsLiveAfterInst(inst) {
		if r == inst.Dst || g.loc(r) != "A" {
			continue
		}
		// regsLiveAfterInst includes terminator uses defined later in this
		// block; those values are not in A yet.
		definedLater := false
		for _, later := range g.curBlock.Insts[g.curInstIdx+1:] {
			if later.Dst == r {
				definedLater = true
				break
			}
		}
		if !definedLater {
			g.saveAccOperandIfLive(r, inst)
			break
		}
	}
	// Every byte multiply path must replace the old accumulator identity.
	defer func() {
		g.pendingAccReg = NoReg
		if dst == "A" {
			g.pendingAccReg = inst.Dst
		}
	}()
	lhs = g.loc(inst.Src[0])
	cv, isConst := g.constVals[inst.Src[1]]

	if isConst {
		switch cv {
		case 0:
			g.emit("    XOR A")
			g.invalidate("A")
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			}
			return
		case 1:
			if dst != lhs {
				g.emitMov(dst, lhs, 8)
			}
			return
		}
		// Power-of-2: log2(cv) × (ADD A,A).
		if cv > 0 && cv&(cv-1) == 0 {
			shifts := 0
			for n := cv; n > 1; n >>= 1 {
				shifts++
			}
			if !g.holdsValue("A", lhs) {
				g.emitLDA(lhs)
			}
			for i := 0; i < shifts; i++ {
				g.emit("    ADD A, A")
			}
			g.invalidate("A")
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			}
			return
		}
		// x*3  = x + x*2;  x*5  = x + x*4;   x*6 = x*2 + x*4
		// x*7  = x*8 - x;  x*9  = x + x*8;   x*10 = x*2 + x*8
		// x*12 = x*4 + x*8; x*15 = x*16 - x
		if cv == 3 || cv == 5 || cv == 6 || cv == 7 || cv == 9 || cv == 10 || cv == 12 || cv == 15 {
			if !g.holdsValue("A", lhs) {
				g.emitLDA(lhs)
			}
			tmp := g.pickScratch8(inst)
			switch cv {
			case 3: // x + x*2
				g.emitf("    LD %s, A", tmp)  // tmp = x
				g.emit("    ADD A, A")        // A  = x*2
				g.emitf("    ADD A, %s", tmp) // A  = x*3
			case 5: // x + x*4
				g.emitf("    LD %s, A", tmp)
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")        // A  = x*4
				g.emitf("    ADD A, %s", tmp) // A  = x*5
			case 6: // x*2 + x*4
				g.emit("    ADD A, A")        // A  = x*2
				g.emitf("    LD %s, A", tmp)  // tmp= x*2
				g.emit("    ADD A, A")        // A  = x*4
				g.emitf("    ADD A, %s", tmp) // A  = x*6
			case 7: // x*8 - x
				g.emitf("    LD %s, A", tmp) // tmp = x
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")     // A  = x*8
				g.emitf("    SUB %s", tmp) // A  = x*7
			case 9: // x + x*8
				g.emitf("    LD %s, A", tmp)
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")        // A  = x*8
				g.emitf("    ADD A, %s", tmp) // A  = x*9
			case 10: // x*2 + x*8
				g.emit("    ADD A, A")       // A  = x*2
				g.emitf("    LD %s, A", tmp) // tmp= x*2
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")        // A  = x*8
				g.emitf("    ADD A, %s", tmp) // A  = x*10
			case 12: // x*4 + x*8
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")        // A  = x*4
				g.emitf("    LD %s, A", tmp)  // tmp= x*4
				g.emit("    ADD A, A")        // A  = x*8
				g.emitf("    ADD A, %s", tmp) // A  = x*12
			case 15: // x*16 - x
				g.emitf("    LD %s, A", tmp) // tmp = x
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")     // A  = x*16
				g.emitf("    SUB %s", tmp) // A  = x*15
			}
			g.invalidate("A")
			if dst != "A" {
				g.emitLD8(dst, "A")
				g.setCopy(dst, "A")
			}
			return
		}
	}

	// General constant or variable multiply: use runtime __mul8 routine.
	// __mul8 clobbers A/B/C/D/E/F. Save pairs before ABI setup so
	// live operands (including a multiplier in B) survive the call.
	g.emit("    PUSH BC")
	g.emit("    PUSH DE")
	// Set up both runtime operands as a parallel copy: the multiplier may
	// currently be in A, and loading the multiplicand first would destroy it.
	if isConst {
		g.emitLDA(lhs)
		g.emitf("    LD B, %d", cv&0xFF)
	} else {
		g.emitParallelCopy([]parallelCopy{
			{srcName: lhs, dstName: "A", ty: TyU8},
			{srcName: g.loc(inst.Src[1]), dstName: "B", ty: TyU8},
		})
	}
	g.emit("    CALL __mul8")
	g.emit("    POP DE")
	g.emit("    POP BC")
	g.needsMul8 = true
	g.invalidate("A")
	g.invalidate("F")
	if dst != "A" {
		g.emitLD8(dst, "A")
		g.setCopy(dst, "A")
	}
}

// genMul16 emits 16-bit multiply.
//
// Constants:
//
//	0:       LD H,0 / LD L,0
//	1:       no-op
//	2^n:     n × ADD HL,HL  (7T per shift)
//	3,5,6,9: PUSH HL / POP BC + shift+add sequence
//
// Variable (software shift-and-add, Russian-peasant LSB-first, 16 iterations):
//
//	BC = multiplicand (lhs)
//	DE = multiplier   (rhs)
//	HL = result = 0
//	A  = 16 (iteration counter — DEC A / JR NZ preserves SRL carry)
//	loop:
//	  SRL D / RR E   → DE >>= 1, old bit0 → carry
//	  JR NC, no_add  → skip if bit was 0
//	  ADD HL, BC     → result += multiplicand (carry from ADD is ignored by next SRL)
//	no_add:
//	  SLA C / RL B   → BC <<= 1 (next power-of-2 multiplicand)
//	  DEC A / JR NZ loop
//
// SRL D is independent of incoming carry, so ADD HL,BC's carry does not corrupt
// the extraction.  Total: ~320T worst case.
func (g *z80cg) genMul16(inst *Inst) {
	dst := g.loc(inst.Dst)
	lhs := g.loc(inst.Src[0])

	cv, isConst := g.constVals[inst.Src[1]]
	regInfo := collectRegInfo(g.fn)
	// Widen byte operands according to their source type. Preserve A and its
	// flags: either may still hold a live value (including the other operand).
	extendByte := func(src Reg, low, high string) {
		if IsSigned(regInfo[src].Ty) {
			g.emit("    PUSH AF")
			g.emitLD8("A", low)
			g.emit("    RLA")
			g.emit("    SBC A, A")
			g.emitLD8(high, "A")
			g.emit("    POP AF")
		} else {
			g.emitf("    LD %s, 0", high)
		}
	}

	// 16-bit multiply needs HL for ADD HL,rr.
	// A live lhs must survive too when the result is allocated elsewhere.
	savedHL := false
	if dst != "HL" {
		// HL may hold a live value (e.g. a constant computed earlier).
		// Save it on stack; restore after multiply into dst.
		g.emit("    PUSH HL")
		savedHL = true
	}

	// Move lhs into HL for the multiply.
	if lhs != "HL" {
		if isIXY(lhs) {
			// IX/IY → HL: PUSH/POP (byte-copy invalid due to DD prefix).
			g.emitf("    PUSH %s", lhs)
			g.emit("    POP HL")
		} else if isIXYReg(lhs) {
			// IXH/IXL/IYH/IYL: widen the byte to HL.
			g.emitMovViaAltA("L", lhs)
			extendByte(inst.Src[0], "L", "H")
		} else if isSimpleReg(lhs) && !isPairReg(lhs) {
			// Widen a byte operand promoted by the expression.
			g.emitLD8("L", lhs)
			extendByte(inst.Src[0], "L", "H")
		} else if isSpill(lhs) {
			g.loadSpill16("HL", lhs)
		} else {
			g.emitf("    LD H, %s", highByte(lhs))
			g.emitLD8("L", lowByte(lhs))
		}
		g.invalidate("HL")
	}

	// mul16Epilogue moves the result from HL to dst and restores saved HL.
	mul16Epilogue := func() {
		if dst == "HL" {
			// already there
		} else if isIXY(dst) {
			g.emit("    PUSH HL")
			g.emitf("    POP %s", dst)
		} else if isSpill(dst) {
			g.storeSpill16(dst, "HL")
			g.invalidate("HL")
		} else {
			g.emitLD8(highByte(dst), "H")
			g.emitLD8(lowByte(dst), "L")
		}
		if savedHL {
			g.emit("    POP HL") // restore pre-mul HL
		}
		g.invalidate(dst)
		if savedHL {
			g.invalidate("HL")
		}
	}

	if isConst {
		switch cv {
		case 0:
			g.emit("    LD H, 0")
			g.emit("    LD L, 0")
			mul16Epilogue()
			return
		case 1:
			mul16Epilogue()
			return
		}
		// Fix A: byte-boundary shift — LD H,L / LD L,0 is cheaper than ADD HL,HL chains.
		if cv >= 256 && cv&(cv-1) == 0 {
			g.emit("    LD H, L")
			g.emit("    LD L, 0")
			for n := cv >> 8; n > 1; n >>= 1 {
				g.emit("    ADD HL, HL")
			}
			mul16Epilogue()
			return
		}
		if cv > 0 && cv&(cv-1) == 0 { // power-of-2: log2(cv) × ADD HL,HL
			for n := cv; n > 1; n >>= 1 {
				g.emit("    ADD HL, HL")
			}
			mul16Epilogue()
			return
		}
		// Small multiples via PUSH/POP BC save + shift sequences.
		if cv == 3 || cv == 5 || cv == 6 || cv == 9 {
			g.emit("    PUSH BC") // preserve values live across the scratch pair
			g.emit("    PUSH HL") // save x → BC
			g.emit("    POP BC")  // BC = x (17T)
			switch cv {
			case 3: // x*2 + x
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
			case 5: // x*4 + x
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
			case 6: // x*4 + x*2
				g.emit("    ADD HL, HL")
				g.emit("    PUSH HL")
				g.emit("    POP BC")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
			case 9: // x*8 + x
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
			}
			g.emit("    POP BC")
			mul16Epilogue()
			return
		}
	}

	// Variable or unsupported constant: software shift-and-add loop.
	// Register layout:
	//   BC = multiplicand (lhs, copied from HL)
	//   DE = multiplier   (rhs)
	//   HL = result = 0
	//   A  = iteration counter (16)
	rhs := g.loc(inst.Src[1])
	loopLbl := fmt.Sprintf(".%s_m16_%d_lp", sanitizeIdent(g.fn.Name), int(inst.Dst))
	skipLbl := fmt.Sprintf(".%s_m16_%d_sk", sanitizeIdent(g.fn.Name), int(inst.Dst))
	g.comment(fmt.Sprintf("mul16 %s * %s → HL  (~320T, 16-iter shift-and-add)", lhs, rhs))
	// Save AF and DE: the multiply uses A as a loop counter and clobbers DE by
	// shifting it 16 times.  Any live variable in A, D, or E would be corrupted.
	g.emit("    PUSH AF") // preserve A (= possible live variable) and F
	g.emit("    PUSH DE") // preserve D and E across multiply
	// Load multiplier into DE BEFORE overwriting BC with the multiplicand.
	// (rhs may be in C/B/BC which gets clobbered by LD B,H; LD C,L below.)
	if isConst {
		// OpMul constants have no physical load (deadConstsForFunc).
		// Strength reduction consumes cv directly; so must this fallback.
		g.emitf("    LD DE, %d", cv&0xFFFF)
		g.invalidate("DE")
	} else if rhs != "DE" {
		if isSpill(rhs) {
			g.loadSpill16("DE", rhs)
		} else if isIXY(rhs) {
			// IX/IY → DE: safe byte-copy (D/E not substituted by DD/FD).
			g.emitf("    LD D, %s", highByte(rhs))
			g.emitf("    LD E, %s", lowByte(rhs))
		} else if isPairReg(rhs) {
			g.emitf("    LD D, %s", highByte(rhs))
			g.emitf("    LD E, %s", lowByte(rhs))
		} else if isIXYReg(rhs) {
			// IXH/IXL half-reg: widen to DE.
			g.emitMovViaAltA("E", rhs)
			extendByte(inst.Src[1], "E", "D")
		} else {
			// 8-bit rhs: widen into DE.
			g.emitf("    LD E, %s", rhs)
			extendByte(inst.Src[1], "E", "D")
		}
		g.invalidate("DE")
	}
	g.emit("    PUSH BC") // save BC across multiply
	g.emit("    LD B, H")
	g.emit("    LD C, L") // BC = multiplicand (lhs)
	g.emit("    LD H, 0")
	g.emit("    LD L, 0")  // HL = result = 0
	g.emit("    LD A, 16") // A = bit counter
	g.emitf("%s:", loopLbl)
	g.emit("    SRL D") // DE >>= 1 (logical right shift, bit0 → carry)
	g.emit("    RR E")
	g.emitf("    JRS NC, %s", skipLbl)
	g.emit("    ADD HL, BC") // result += multiplicand (current bit weight)
	g.emitf("%s:", skipLbl)
	g.emit("    SLA C") // BC <<= 1 (multiplicand doubles)
	g.emit("    RL B")
	g.emit("    DEC A") // counter-- (does not affect carry)
	g.emitf("    JRS NZ, %s", loopLbl)
	g.emit("    POP BC") // restore BC
	g.emit("    POP DE") // restore old DE (live variables in D/E preserved)
	g.emit("    POP AF") // restore old A (live variables in A preserved)
	mul16Epilogue()
	g.invalidate("BC")
}

// genMul32 emits 24/32-bit multiply by a constant via SHL-and-add.
//
// Power-of-2 constants: log2(cv) × SHL32 (ADD HL,HL / EXX / ADC HL,HL / EXX per bit).
// Small constants (3,5,6,9): LD+saves using emitMov32 for the partial product.
// General constants: decompose into Σ 2^k terms (binary method).
// Variable multiplier: TODO (software loop).
func (g *z80cg) genMul32(inst *Inst) {
	dst := g.loc(inst.Dst)
	lhs := g.loc(inst.Src[0])

	// Emit one 32-bit left-shift-by-1 on the register pair p.
	// Z80 only has ADD HL,HL — route through HL if p != HL.
	shl1 := func(p string) {
		if p != "HL" {
			g.emitMov("HL", p, 16)
		}
		g.emit("    ADD HL, HL")
		g.emit("    EXX")
		g.emit("    ADC HL, HL")
		g.emit("    EXX")
		if p != "HL" {
			g.emitMov(p, "HL", 16)
			g.invalidate("HL")
		}
	}

	cv, isConst := g.constVals[inst.Src[1]]

	if !isConst {
		rhs := g.loc(inst.Src[1])
		g.comment(fmt.Sprintf("TODO: 32-bit variable mul %s * %s → %s", lhs, rhs, dst))
		return
	}

	// Ensure lhs is in dst before operating.
	if dst != lhs {
		g.emitMov32(dst, lhs)
	}

	switch cv {
	case 0:
		// dst = 0: load zero constant.
		if isSpill(dst) {
			g.emit("    LD HL, 0")
			g.storeSpill16(dst, "HL")
			g.invalidate("HL")
		} else {
			g.emitf("    LD %s, 0", dst)
			g.emit("    EXX")
			g.emitf("    LD %s, 0", dst)
			g.emit("    EXX")
		}
		g.invalidate(dst)
		return
	case 1:
		// dst = lhs — already done by move above.
		return
	}

	// Power-of-2: log2(cv) left shifts.
	if cv > 0 && cv&(cv-1) == 0 {
		shifts := 0
		for n := cv; n > 1; n >>= 1 {
			shifts++
		}
		for i := 0; i < shifts; i++ {
			shl1(dst)
		}
		g.invalidate(dst)
		return
	}

	// Binary decomposition: dst = Σ (dst_orig << k) for each set bit k in cv.
	// Uses a saved copy of the original value to accumulate.
	// Only valid when dst is HL (has ADD/ADC). For others, fall through to TODO.
	if dst == "HL" && cv > 0 {
		// Save original in DE (push to stack if DE is occupied).
		g.emit("    PUSH DE") // save main DE
		g.emit("    EXX")
		g.emit("    PUSH DE") // save shadow DE
		g.emit("    EXX")
		// Copy lhs (already in HL/H'L') to DE/D'E'.
		g.emitMov32("DE", "HL")
		// Accumulate: acc = 0, then for each set bit k add (orig << k).
		// Clear HL for accumulation.
		g.emitf("    LD HL, 0")
		g.emit("    EXX")
		g.emitf("    LD HL, 0")
		g.emit("    EXX")
		shift := 0
		for bit := int64(0); bit < 32; bit++ {
			if cv&(1<<bit) != 0 {
				// Shift DE/D'E' to the current bit position (relative to prev shift).
				for i := shift; i < int(bit); i++ {
					// SHL DE by 1: ADD HL,DE / ... — no, we shift DE itself.
					// Use: EXX / SLA E / RL D / EXX / SLA E / RL D
					// Wait, that shifts D'E'. Let's use ADD DE,DE — Z80 has no ADD DE,DE.
					// Instead: shift DE using SLA/RL pattern.
					g.emit("    EXX")
					g.emit("    SLA E")
					g.emit("    RL  D")
					g.emit("    EXX")
					g.emit("    SLA E")
					g.emit("    RL  D")
				}
				shift = int(bit)
				// acc (HL/H'L') += DE/D'E'.
				g.emitf("    ADD HL, DE")
				g.emit("    EXX")
				g.emitf("    ADC HL, DE")
				g.emit("    EXX")
			}
		}
		// Restore DE.
		g.emit("    EXX")
		g.emit("    POP DE")
		g.emit("    EXX")
		g.emit("    POP DE")
		g.invalidate(dst)
		return
	}

	// Fallback for non-HL dst or complex constants.
	g.comment(fmt.Sprintf("TODO: 32-bit mul by %d in %s", cv, dst))
}

// ── Division / Modulo ─────────────────────────────────────────────────────────
//
// Z80 has no hardware divide. We use:
//   u8:  repeated subtraction — A / C → B=quotient, A=remainder
//   u16: shift-and-subtract long division — HL / DE → HL=quotient, DE=remainder
//
// OpDiv/OpMod return quotient/remainder respectively.
// OpSDiv is signed: negate operands, unsigned div, fix sign.

func (g *z80cg) genDivMod(inst *Inst) {
	w := inst.Ty.Width()
	defer g.saveWrittenScratch(inst)()

	// Positive powers of two have cheap unsigned and signed implementations.
	if k, ok := g.constVals[inst.Src[1]]; ok && k > 0 && k&(k-1) == 0 && w <= 16 && ((inst.Op != OpSDiv && inst.Op != OpSMod) || k < int64(1)<<(w-1)) {
		signed := inst.Op == OpSDiv || inst.Op == OpSMod
		if signed && g.nonNegative(inst.Src[0], w, make(map[Reg]bool)) {
			signed = false
		}
		g.genPow2DivMod(inst, k, signed)
		return
	}

	if w <= 8 {
		g.genDivMod8(inst)
	} else if w <= 16 {
		g.genDivMod16(inst)
	} else {
		g.comment(fmt.Sprintf("TODO: %d-bit div/mod", w))
	}
}

// genDivMod8 — 8-bit unsigned division via shift-and-subtract (restoring).
//
// Based on DIVU111 from Dark / X-Trade, Spectrum Expert #01 (1997):
//
//	Input: B = dividend, C = divisor
//	Output: B = quotient, A = remainder
//	Cost: 236–244 T-states
//
// Algorithm: binary long division. Shift dividend left through carry
// into accumulator (A). Try subtracting divisor; if it fits, set the
// quotient bit via INC B (bit 0 is free after SLA B).
func (g *z80cg) genDivMod8(inst *Inst) {
	dst := g.loc(inst.Dst)
	lhs := g.loc(inst.Src[0])
	rhs := g.loc(inst.Src[1])
	idx := g.trampIdx
	g.trampIdx++

	isSigned := inst.Op == OpSDiv || inst.Op == OpSMod
	wantMod := inst.Op == OpMod || inst.Op == OpSMod

	if isSigned {
		// Stage both operands before assigning B/C, including swapped allocations.
		g.emit("    PUSH AF")
		g.emitLDA(lowByte(lhs))
		g.emit("    PUSH AF")
		if lowByte(rhs) == "A" {
			// Original A is the divisor; loading the dividend must not replace it.
			g.emit("    POP BC") // B = staged dividend
			g.emit("    POP AF") // A = original divisor
			g.emit("    LD C, A")
		} else {
			g.emitLDA(lowByte(rhs))
			g.emit("    LD C, A")
			g.emit("    POP AF")
			g.emit("    LD B, A")
			g.emit("    POP AF")
		}
	} else {
		// Setup: B = dividend, C = divisor.
		// Width guard: lhs/rhs might be a 16-bit pair (const allocated to HL/DE).
		if lhs == "B" {
			// already there
		} else if isPairReg(lhs) {
			g.emitf("    LD B, %s", lowByte(lhs))
		} else if isSpill(lhs) {
			g.loadSpill8("B", lhs)
		} else if lhs == "A" {
			g.emit("    LD B, A")
		} else {
			g.emitf("    LD B, %s", lhs)
		}
		if rhs == "C" {
			// already there
		} else if isPairReg(rhs) {
			g.emitf("    LD C, %s", lowByte(rhs))
		} else if isSpill(rhs) {
			g.loadSpill8("C", rhs)
		} else {
			g.emitf("    LD C, %s", rhs)
		}

	}
	if isSigned {
		g.emit("    LD A, B")
		if !wantMod {
			g.emit("    XOR C")
		}
		g.emit("    PUSH AF") // result sign, separate from division workspace
		g.emit("    BIT 7, B")
		g.emitf("    JR Z, .div8_abs_l_%d", idx)
		g.emit("    LD A, B")
		g.emit("    NEG")
		g.emit("    LD B, A")
		g.emitf(".div8_abs_l_%d:", idx)
		g.emit("    BIT 7, C")
		g.emitf("    JR Z, .div8_abs_r_%d", idx)
		g.emit("    LD A, C")
		g.emit("    NEG")
		g.emit("    LD C, A")
		g.emitf(".div8_abs_r_%d:", idx)
	}

	g.emit("    XOR A")   // clear accumulator (remainder workspace)
	g.emit("    LD D, 8") // 8 bits to process
	g.emitf(".div8_%d:", idx)
	g.emit("    SLA B")                     // shift dividend left — MSB → CF
	g.emit("    RLA")                       // shift CF into accumulator
	g.emit("    CP C")                      // try subtract divisor
	g.emitf("    JR C, .div8_skip_%d", idx) // A < C → skip
	g.emit("    SUB C")                     // subtract divisor
	g.emit("    INC B")                     // set quotient bit 0 (free after SLA)
	g.emitf(".div8_skip_%d:", idx)
	g.emit("    DEC D")
	g.emitf("    JR NZ, .div8_%d", idx)
	// Result: B = quotient, A = remainder.

	if isSigned {
		if wantMod {
			g.emit("    LD B, A")
		}
		g.emit("    POP AF")
		g.emit("    BIT 7, A")
		g.emit("    LD A, B")
		g.emitf("    JR Z, .div8_sign_%d", idx)
		g.emit("    NEG")
		g.emitf(".div8_sign_%d:", idx)
		g.emit("    LD B, A")
	}

	if wantMod {
		if dst != "A" {
			g.emitLD8(dst, "A")
			g.setCopy(dst, "A")
		}
	} else {
		if dst == "A" {
			g.emit("    LD A, B")
		} else if dst != "B" {
			g.emitf("    LD %s, B", dst)
		}
		g.setCopy(dst, "B")
	}
	g.invalidate("A")
	g.invalidate("B")
	g.invalidate("D")
	g.invalidate("F")
}

// genDivMod16 — 16-bit unsigned division via shift-and-subtract long division.
//
//	HL / DE → HL=quotient, BC=remainder
//
// Classic Z80 restoring division (16 iterations):
//
//	BC = 0 (remainder), A = 16 (counter)
//	loop:
//	  ADD HL, HL       ; shift dividend left, MSB → CF
//	  RL C; RL B       ; shift CF into remainder
//	  PUSH HL; LD H,B; LD L,C; SBC HL,DE  ; trial: remainder - divisor
//	  if no borrow: BC = HL (accept), POP HL, SET 0,L (quotient bit = 1)
//	  if borrow:    POP HL (quotient bit stays 0, BC unchanged)
//	  DEC A; JR NZ loop
func (g *z80cg) genDivMod16(inst *Inst) {
	dst := g.loc(inst.Dst)
	lhs := g.loc(inst.Src[0])
	rhs := g.loc(inst.Src[1])
	idx := g.trampIdx
	g.trampIdx++

	wantMod := inst.Op == OpMod || inst.Op == OpSMod
	isSigned := inst.Op == OpSDiv || inst.Op == OpSMod

	// Stage both inputs without changing any source register, including spills
	// and index halves. The unsigned path needs the same parallel-move safety.
	g.pushWord(lhs)
	g.pushWord(rhs)
	g.emit("    POP DE")
	g.emit("    POP HL")
	g.invalidate("DE")
	g.invalidate("HL")

	if isSigned {
		g.emit("    LD A, H")
		if !wantMod {
			g.emit("    XOR D")
		}
		g.emit("    PUSH AF")
		for _, pair := range []string{"HL", "DE"} {
			hi, lo := highByte(pair), lowByte(pair)
			g.emitf("    BIT 7, %s", hi)
			g.emitf("    JR Z, .div16_abs_%s_%d", pair, idx)
			g.emit("    XOR A")
			g.emitf("    SUB %s", lo)
			g.emitf("    LD %s, A", lo)
			g.emit("    SBC A, A")
			g.emitf("    SUB %s", hi)
			g.emitf("    LD %s, A", hi)
			g.emitf(".div16_abs_%s_%d:", pair, idx)
		}
	}

	// BC = 0 (remainder accumulator), A = 16 (iteration counter).
	g.emit("    LD BC, 0")
	g.emit("    LD A, 16")

	// Main loop.
	g.emitf(".div16_%d:", idx)
	g.emit("    ADD HL, HL") // shift dividend left; MSB → CF
	g.emit("    RL C")       // shift CF into remainder (BC)
	g.emit("    RL B")
	g.emit("    PUSH HL") // save quotient-in-progress
	g.emit("    LD H, B") // HL = BC (remainder)
	g.emit("    LD L, C")
	g.emit("    OR A")                       // clear CF for SBC
	g.emit("    SBC HL, DE")                 // trial subtract: remainder - divisor
	g.emitf("    JR C, .div16_skip_%d", idx) // borrow → remainder < divisor
	// Accept: remainder = HL (after subtract).
	g.emit("    LD B, H")
	g.emit("    LD C, L")
	g.emit("    POP HL")   // restore quotient
	g.emit("    SET 0, L") // this quotient bit = 1
	g.emit("    DEC A")
	g.emitf("    JR NZ, .div16_%d", idx)
	g.emitf("    JR .div16_done_%d", idx)
	// Skip: remainder stays, quotient bit stays 0.
	g.emitf(".div16_skip_%d:", idx)
	g.emit("    POP HL") // restore quotient (bit 0 = 0)
	g.emit("    DEC A")
	g.emitf("    JR NZ, .div16_%d", idx)

	g.emitf(".div16_done_%d:", idx)
	// Result: HL = quotient, BC = remainder.
	if isSigned {
		hi, lo := "H", "L"
		if wantMod {
			hi, lo = "B", "C"
		}
		g.emit("    POP AF")
		g.emit("    BIT 7, A")
		g.emitf("    JR Z, .div16_sign_%d", idx)
		g.emit("    XOR A")
		g.emitf("    SUB %s", lo)
		g.emitf("    LD %s, A", lo)
		g.emit("    SBC A, A")
		g.emitf("    SUB %s", hi)
		g.emitf("    LD %s, A", hi)
		g.emitf(".div16_sign_%d:", idx)
	}

	if wantMod {
		// Want remainder (BC) → move to dst.
		if dst == "HL" {
			g.emit("    LD H, B")
			g.emit("    LD L, C")
		} else if dst == "BC" {
			// already there
		} else if isSpill(dst) {
			g.emit("    LD H, B")
			g.emit("    LD L, C")
			g.storeSpill16(dst, "HL")
		} else if isIXY(dst) {
			g.emit("    LD H, B")
			g.emit("    LD L, C")
			g.emit("    PUSH HL")
			g.emitf("    POP %s", dst)
		} else {
			g.emitLD8(highByte(dst), "B")
			g.emitLD8(lowByte(dst), "C")
		}
	} else {
		// Want quotient (HL) → move to dst if needed.
		if dst == "HL" {
			// already there
		} else if isSpill(dst) {
			g.storeSpill16(dst, "HL")
		} else if isIXY(dst) {
			// HL→IX/IY: byte-copy invalid (DD NOP). Use PUSH/POP.
			g.emit("    PUSH HL")
			g.emitf("    POP %s", dst)
		} else {
			g.emitLD8(highByte(dst), "H")
			g.emitLD8(lowByte(dst), "L")
		}
	}
	g.invalidate("A")
	g.invalidate("BC")
	g.invalidate("DE")
	g.invalidate("HL")
	g.invalidate("F")
}

// Only use proofs that survive same-width signed casts: narrower unsigned parameters,
// zero extension from a narrower width, and copies of a proven value.
func (g *z80cg) nonNegative(r Reg, width int, seen map[Reg]bool) bool {
	if seen[r] {
		return false
	}
	seen[r] = true
	for _, p := range g.fn.Contract.Params {
		if p.Reg == r {
			return p.Ty.Width() < width && !IsSigned(p.Ty)
		}
	}
	for _, b := range g.fn.Blocks {
		for _, i := range b.Insts {
			if i.Dst != r {
				continue
			}
			switch i.Op {
			case OpExt:
				return i.SrcTy != nil && i.Ty != nil && i.SrcTy.Width() < i.Ty.Width() && i.SrcTy.Width() < width
			case OpConst:
				return i.Imm >= 0 && i.Imm < (int64(1)<<(width-1))
			case OpMove:
				return g.nonNegative(i.Src[0], width, seen)
			}
		}
	}
	return false
}

func (g *z80cg) genPow2DivMod(inst *Inst, k int64, signed bool) {
	dst, lhs := g.loc(inst.Dst), g.loc(inst.Src[0])
	w := inst.Ty.Width()
	wantMod := inst.Op == OpMod || inst.Op == OpSMod
	if !signed && !wantMod && w <= 8 && !isSpill(dst) && !isIXY(dst) {
		if isPairReg(lhs) {
			lhs = lowByte(lhs)
		}
		g.emitLDA(lhs)
		for v := k; v > 1; v >>= 1 {
			g.emit("    SRL A")
		}
		if isPairReg(dst) {
			g.emitLD8(lowByte(dst), "A")
			g.emitf("    LD %s, 0", highByte(dst))
		} else {
			g.emitLD8(dst, "A")
		}
		g.invalidate("A")
		g.invalidate(dst)
		return
	}

	if !signed && wantMod && k <= 256 && !isIXY(dst) && !isSpill(dst) {

		if isPairReg(lhs) {
			lhs = lowByte(lhs)
		}
		g.emitLDA(lhs)
		g.emitf("    AND %d", k-1)
		if isPairReg(dst) {
			g.emitLD8(lowByte(dst), "A")
			g.emitf("    LD %s, 0", highByte(dst))
		} else {
			g.emitLD8(dst, "A")
		}

		g.invalidate("A")
		g.invalidate(dst)
		return
	}
	// HL is temporary unless it is the result. AF holds the original sign.

	if lhs != "HL" {
		g.pushWord(lhs)
		g.emit("    POP HL")
	}
	idx := g.trampIdx
	g.trampIdx++
	negate := func() {
		g.emit("    XOR A")
		g.emit("    SUB L")
		g.emit("    LD L, A")
		if w == 16 {
			g.emit("    SBC A, A")
			g.emit("    SUB H")
			g.emit("    LD H, A")
		}
	}
	if signed {
		hi := "H"
		if w <= 8 {
			hi = "L"
		}
		g.emitf("    LD A, %s", hi)
		g.emit("    PUSH AF")
		g.emitf("    BIT 7, %s", hi)
		g.emitf("    JR Z, .pow2_abs_%d", idx)
		negate()
		g.emitf(".pow2_abs_%d:", idx)
	}
	if wantMod {
		mask := k - 1
		g.emit("    LD A, L")
		g.emitf("    AND %d", mask&255)
		g.emit("    LD L, A")
		if w == 16 {
			if mask < 256 {
				g.emit("    LD H, 0")
			} else {
				g.emit("    LD A, H")
				g.emitf("    AND %d", (mask>>8)&255)
				g.emit("    LD H, A")
			}
		}
	} else {
		for v := k; v > 1; v >>= 1 {
			if w == 16 {
				g.emit("    SRL H")
				g.emit("    RR L")
			} else {
				g.emit("    SRL L")
			}
		}
	}
	if signed {
		g.emit("    POP AF")
		g.emit("    OR A")
		g.emitf("    JP P, .pow2_done_%d", idx)
		negate()
		g.emitf(".pow2_done_%d:", idx)
	}
	if w <= 8 {
		g.emitMov(dst, "L", w)
	} else {
		g.emitMov(dst, "HL", w)
	}

	g.invalidate("A")
	g.invalidate(dst)
	g.invalidate("HL")
}

// Arithmetic helpers use fixed scratch registers beyond their allocated dst.
// Preserve any unrelated live values there, including byte halves of pairs.
func (g *z80cg) saveLiveScratch(inst *Inst, pairs ...string) func() {
	live := g.scratchLiveAfter(inst)

	dst := g.loc(inst.Dst)
	accPair := "BC"
	if dst == "BC" || dst == "B" || dst == "C" {
		accPair = "HL"
	}
	saved := []string{}
	for _, pair := range pairs {
		overlaps := func(loc string) bool {
			return loc == pair || loc == highByte(pair) || loc == lowByte(pair) || (pair == "AF" && (loc == "A"))
		}
		if overlaps(dst) {
			continue
		}
		needed := false
		for r := range live {
			if r != inst.Dst && overlaps(g.loc(r)) {
				needed = true
				break
			}
		}
		if needed {
			if pair == "AF" {
				g.emitf("    PUSH %s", accPair)
			}
			g.emitf("    PUSH %s", pair)
			saved = append(saved, pair)
		}
	}
	return func() {
		for i := len(saved) - 1; i >= 0; i-- {
			if saved[i] == "AF" {
				g.emitf("    POP %s", accPair)
				g.emitf("    LD A, %s", highByte(accPair))
				g.emitf("    POP %s", accPair)
				g.invalidate("A")
			} else {
				g.emitf("    POP %s", saved[i])
				g.invalidate(saved[i])
			}
		}
	}
}

// scratchLiveAfter walks backwards from live-out, killing later definitions.
// A value first defined after inst does not occupy its allocation yet.
func (g *z80cg) scratchLiveAfter(inst *Inst) map[Reg]bool {
	live := make(map[Reg]bool)
	if g.curBlock == nil {
		return live
	}
	if g.liveness != nil {
		out := g.liveness.LiveOutOf(g.fn, g.curBlock)
		for r := range g.ar.Locs {
			if out != nil && out.Has(r) {
				live[r] = true
			}
		}
	}
	if g.curBlock.Term != nil {
		for _, r := range g.curBlock.Term.termUses() {
			live[r] = true
		}
	}
	for i := len(g.curBlock.Insts) - 1; i >= 0; i-- {
		next := g.curBlock.Insts[i]
		if next == inst {
			break
		}
		delete(live, next.Dst)
		for _, r := range next.ExtraRets {
			delete(live, r)
		}
		for _, r := range next.Uses() {
			if r != NoReg {
				live[r] = true
			}
		}
	}
	return live
}

// Buffer the selected sequence so preservation follows its physical writes,
// rather than a worst-case list for the MIR opcode. Flags are intentionally
// excluded: comparison flags must survive restoration of the accumulator.
func (g *z80cg) saveWrittenScratch(inst *Inst) func() {
	outer := g.sb
	var body strings.Builder
	g.sb = &body
	live := g.scratchLiveAfter(inst)
	locs := make(map[Reg]string)
	for r := range live {
		locs[r] = g.loc(r)
	}
	return func() {
		g.sb = outer
		writes := scratchSequenceWrites(body.String())
		var pairs []string
		for _, pair := range []string{"AF", "HL", "DE", "BC", "IX", "IY"} {
			for r, loc := range locs {
				if r == inst.Dst {
					continue
				}
				if (loc == pair && (writes[highByte(pair)] || writes[lowByte(pair)])) ||
					(parentPair(loc) == pair && writes[loc]) {
					pairs = append(pairs, pair)
					break
				}
			}
		}
		restore := g.saveLiveScratch(inst, pairs...)
		g.sb.WriteString(body.String())
		restore()
	}
}

// These helpers emit only ordinary scalar Z80 instructions. Unknown operations
// conservatively write all registers, so adding a sequence cannot omit a save.
func scratchSequenceWrites(asm string) map[string]bool {
	writes := make(map[string]bool)
	mark := func(r string) {
		if r == "AF" {
			writes["A"] = true
		} else if isPairReg(r) {
			writes[highByte(r)], writes[lowByte(r)] = true, true
		} else if isSimpleReg(r) || isIXYReg(r) {
			if r != "F" {
				writes[r] = true
			}
		}
	}
	for _, line := range strings.Split(asm, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, ";", 2)[0])
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasSuffix(line, ":") {
			continue
		}
		op := fields[0]
		args := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, op)), ",")
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}
		switch op {
		case "LD", "POP", "INC", "DEC", "SLA", "SRA", "SRL", "RL", "RR", "RLC", "RRC":
			mark(args[0])
		case "SET", "RES":
			if len(args) > 1 {
				mark(args[1])
			}
		case "ADD", "ADC", "SBC":
			if len(args) > 1 {
				mark(args[0])
			} else {
				mark("A")
			}
		case "AND", "OR":
			if args[0] != "A" {
				mark("A")
			}
		case "SUB", "XOR", "NEG", "RLA", "RRA", "RLCA", "RRCA", "CPL":
			mark("A")
		case "EX":
			for _, r := range args {
				mark(r)
			}
		case "CP", "BIT", "PUSH", "JP", "JR", "SCF", "CCF", "NOP":
		case "DJNZ":
			mark("B")
		default:
			for _, r := range []string{"AF", "HL", "DE", "BC", "IX", "IY"} {
				mark(r)
			}
		}
	}
	return writes
}
