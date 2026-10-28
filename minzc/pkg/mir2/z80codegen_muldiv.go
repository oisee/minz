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
	// Inline and runtime multiplication both replace physical flags. Capture
	// a live comparison first; materialization preserves the current A operand.
	g.materializePendingFlag(inst)
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
	// Capture scratch preservation after relocating a live A operand. Otherwise
	// restoring the old scratch pair discards the new home recorded in
	// physOverride while later instructions still read that home.
	defer g.saveWrittenScratch(inst)()
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
		if cv == 3 || cv == 5 || cv == 6 || cv == 7 || cv == 9 || cv == 10 || cv == 11 || cv == 12 || cv == 15 {
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
			case 11: // ((x*2 + x)*2)*2 - x; one scratch, no incoming carry.
				g.emitf("    LD %s, A", tmp)
				g.emit("    ADD A, A")
				g.emitf("    ADD A, %s", tmp)
				g.emit("    ADD A, A")
				g.emit("    ADD A, A")
				g.emitf("    SUB %s", tmp)
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
//	3,5,6,9,10,12: LD B,H / LD C,L + shift+add sequence
//
// Variable (software shift-and-add, Russian-peasant LSB-first):
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
//
// When the multiplier is provably a zero-extended byte (provablyBelow256), its
// high byte is zero: iterations 9-16 always take the skip and contribute
// nothing, so the loop runs 8 iterations instead. That body also relocates the
// multiplier's low byte into A and the multiplicand into DE, freeing B as a
// DJNZ counter (A = multiplier, DE = multiplicand, HL = result, B = 8) — the
// layout of research/c89-benchmark/fusion/hand-asm/fusion_dot_product.a80.
func (g *z80cg) genMul16(inst *Inst) {
	// Query using the original instruction identity, before commutative staging
	// makes a local copy. Include pair aliases and live operands, but not dst.
	live := g.regsLiveAfterInst(inst)
	pairLive := func(pair string) bool {
		for r := range live {
			if r != inst.Dst && (g.loc(r) == pair || parentPair(g.loc(r)) == pair) {
				return true
			}
		}
		return false
	}
	// Multiplication is commutative. Keep a resident variable operand in HL:
	// moving the other operand there would destroy it (DE * HL -> DE * DE).
	// A constant RHS has no physical load: its allocated HL is only a nominal
	// home. Keep that constant on the RHS so strength reduction/fallback reads
	// its immediate value rather than treating stale HL as a variable operand.
	_, rhsConstant := g.constVals[inst.Src[1]]
	if !rhsConstant && g.loc(inst.Src[1]) == "HL" && g.loc(inst.Src[0]) != "HL" {
		copy := *inst
		copy.Src[0], copy.Src[1] = inst.Src[1], inst.Src[0]
		inst = &copy
	}
	// A proven byte multiplier halves the loop. Stage the proven operand as the
	// shifting multiplier (rhs/DE). Constants take their own count from cv
	// below. An HL-resident lhs must not become the rhs: staging the other
	// operand into HL would destroy it before the multiplier load reads it.
	byteMultiplier := false
	if _, lhsConst := g.constVals[inst.Src[0]]; !lhsConst {
		if _, rhsConst := g.constVals[inst.Src[1]]; !rhsConst {
			if g.provablyBelow256(inst.Src[1], inst.Ty.Width(), make(map[Reg]bool)) {
				byteMultiplier = true
			} else if loc := g.loc(inst.Src[0]); loc != "HL" && loc != "H" && loc != "L" &&
				g.provablyBelow256(inst.Src[0], inst.Ty.Width(), make(map[Reg]bool)) {
				copy := *inst
				copy.Src[0], copy.Src[1] = inst.Src[1], inst.Src[0]
				inst = &copy
				byteMultiplier = true
			}
		}
	}
	dst := g.loc(inst.Dst)
	lhs := g.loc(inst.Src[0])

	// Two proven bytes inside a loop, in speed mode, multiply through the
	// shared quarter-square table instead of the shift-and-add loop. Checked
	// before the operand staging below, which would otherwise move lhs into HL
	// for the loop body.
	if g.mul16QuarterSquareAdmissible(inst) {
		g.genMul16QuarterSquare(inst, pairLive)
		return
	}

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
	loadSpilledOperand := func(reg Reg, spill, pair string) {
		if info, ok := regInfo[reg]; ok && info.Ty != nil && info.Ty.Width() <= 8 {
			// Promoted byte operands retain one-byte spill slots. Loading a
			// word would accidentally include the neighbouring spill value.
			g.emit("    PUSH AF")
			g.emitNativeSpillf(spill, 0, 1, false, "    LD A, (%s)", spill)
			g.emitLD8(lowByte(pair), "A")
			g.emit("    POP AF")
			g.invalidate("A")
			extendByte(reg, lowByte(pair), highByte(pair))
		} else {
			g.loadSpill16(pair, spill)
		}
	}

	// 16-bit multiply needs HL for ADD HL,rr.
	// A live lhs must survive too when the result is allocated elsewhere.
	savedHL := false
	if dst != "HL" && pairLive("HL") {
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
			loadSpilledOperand(inst.Src[0], lhs, "HL")
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
		// Small multiples use BC as scratch for shift-and-add sequences.
		if cv == 3 || cv == 5 || cv == 6 || cv == 9 || cv == 10 || cv == 12 {
			savedBC := pairLive("BC")
			if savedBC {
				g.emit("    PUSH BC")
			}
			g.emit("    LD B, H")
			g.emit("    LD C, L") // BC = x, without a stack round trip
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
				g.emit("    LD B, H")
				g.emit("    LD C, L")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
			case 9: // x*8 + x
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
			case 10: // (x*4 + x)*2; ADD HL,rr is 11T on Z80.
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
				g.emit("    ADD HL, HL")
			case 12: // (x*2 + x)*4.
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, BC")
				g.emit("    ADD HL, HL")
				g.emit("    ADD HL, HL")
			}
			if savedBC {
				g.emit("    POP BC")
			}
			g.invalidate("BC")
			mul16Epilogue()
			return
		}
	}

	// Variable or unsupported constant: software shift-and-add loop.
	// Register layout:
	//   BC = multiplicand (lhs, copied from HL)
	//   DE = multiplier   (rhs)
	//   HL = result = 0
	//   A  = iteration counter (16, or 8 for a proven byte multiplier)
	rhs := g.loc(inst.Src[1])
	iterations := 16
	if isConst {
		// The constant's own bits above bit 7 decide, not the value's magnitude.
		if cv&0xFFFF < 256 {
			iterations = 8
		}
	} else if byteMultiplier {
		iterations = 8
	}
	loopLbl := fmt.Sprintf(".%s_m16_%d_lp", sanitizeIdent(g.fn.Name), int(inst.Dst))
	skipLbl := fmt.Sprintf(".%s_m16_%d_sk", sanitizeIdent(g.fn.Name), int(inst.Dst))
	g.comment(fmt.Sprintf("mul16 %s * %s → HL  (%d-iter shift-and-add)", lhs, rhs, iterations))
	// Eight-iteration multipliers fit the shifting operand's low byte in A,
	// freeing B as a DJNZ counter — the hand-asm/SDCC body. One SRL A replaces
	// SRL D/RR E and DJNZ replaces DEC A/JRS NZ, saving ~11T per iteration.
	// An operand homed in H or L would be destroyed by the multiplicand
	// staging into HL; those keep the DE-based body.
	fastByteBody := iterations == 8 && rhs != "H" && rhs != "L"
	// Save AF and DE: the multiply uses A (multiplier or counter) and clobbers
	// DE by shifting it.  Any live variable in A, D, or E would be corrupted.
	g.emit("    PUSH AF") // preserve A (= possible live variable) and F
	g.emit("    PUSH DE") // preserve D and E across multiply
	// Load the multiplier BEFORE overwriting BC with the multiplicand.
	// (rhs may be in C/B/BC which gets clobbered by LD B,H; LD C,L below.)
	if fastByteBody {
		if isConst {
			// OpMul constants have no physical load (deadConstsForFunc).
			g.emitf("    LD A, %d", cv&0xFF)
			g.invalidate("A")
		} else if isSpill(rhs) {
			// The multiplier's low byte: byte slots hold it at offset 0, and
			// word slots are little-endian.
			g.loadSpill8("A", rhs)
			g.invalidate("A")
		} else {
			g.emitLD8("A", rhs)
			if rhs != "A" {
				g.invalidate("A")
			}
		}
	} else if isConst {
		// OpMul constants have no physical load (deadConstsForFunc).
		// Strength reduction consumes cv directly; so must this fallback.
		g.emitf("    LD DE, %d", cv&0xFFFF)
		g.invalidate("DE")
	} else if rhs != "DE" {
		if isSpill(rhs) {
			loadSpilledOperand(inst.Src[1], rhs, "DE")
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
	savedBC := pairLive("BC")
	if savedBC {
		g.emit("    PUSH BC")
	}
	if fastByteBody {
		g.emit("    LD D, H")
		g.emit("    LD E, L") // DE = multiplicand (lhs)
	} else {
		g.emit("    LD B, H")
		g.emit("    LD C, L") // BC = multiplicand (lhs)
	}
	g.emit("    LD H, 0")
	g.emit("    LD L, 0") // HL = result = 0
	if fastByteBody {
		g.emit("    LD B, 8") // B = bit counter
		g.emitf("%s:", loopLbl)
		g.emit("    SRL A") // A >>= 1 (logical right shift, bit0 → carry)
		g.emitf("    JRS NC, %s", skipLbl)
		g.emit("    ADD HL, DE") // result += multiplicand (current bit weight)
		g.emitf("%s:", skipLbl)
		g.emit("    SLA E") // DE <<= 1 (multiplicand doubles)
		g.emit("    RL D")
		g.emitf("    DJNZ %s", loopLbl) // counter-- (does not affect carry)
	} else {
		g.emitf("    LD A, %d", iterations)
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
	}
	if savedBC {
		g.emit("    POP BC")
	}
	g.emit("    POP DE") // restore old DE (live variables in D/E preserved)
	g.emit("    POP AF") // restore old A (live variables in A preserved)
	mul16Epilogue()
	g.invalidate("BC")
}

// genMul32 handles constants and variables with the same complete,
// alias-safe kernel. Specialized constant recipes can be added after they
// prove equivalence and register preservation against this implementation.
func (g *z80cg) genMul32(inst *Inst) {
	g.genWideArithmetic(inst)
}

// ── Quarter-square table multiply ─────────────────────────────────────────────

// quarterSquareTableLabel names the module-wide quarter-square table.
const quarterSquareTableLabel = "__qsq"

// quarterSquareTableAsm emits the shared quarter-square table: 511 words,
// sq[n] = floor(n*n/4) for n = 0..510 (1022 bytes). a*b = sq[a+b] - sq[|a-b|]
// holds exactly because a+b and a-b have the same parity, so the two floors
// cancel; the largest entry, sq[510] = 65025, still fits a word.
func quarterSquareTableAsm() string {
	var sb strings.Builder
	sb.WriteString("; runtime: quarter-square table sq[n] = n*n/4, n = 0..510 (511 words)\n")
	fmt.Fprintf(&sb, "%s:\n", quarterSquareTableLabel)
	for n := 0; n <= 510; n += 8 {
		sb.WriteString("    DW ")
		for i := n; i <= n+7 && i <= 510; i++ {
			if i > n {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%d", (i*i)/4)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// mulLoopBodyLabels returns the labels of every block inside a natural loop,
// i.e. reachable from a back edge (an edge whose target dominates its source)
// without leaving the loop. AnalyzeNaturalLoops needs a unique preheader and a
// single latch; this only needs dominance, so no loop shape is missed.
func mulLoopBodyLabels(f *Func) map[string]bool {
	if f == nil || len(f.Blocks) == 0 {
		return nil
	}
	dom := reachableDominators(f)
	pred := map[string][]*Block{}
	for _, b := range f.Blocks {
		if b.Term == nil {
			continue
		}
		for _, s := range b.Term.Successors() {
			pred[s] = append(pred[s], b)
		}
	}
	inLoop := map[string]bool{}
	for _, b := range f.Blocks {
		if b.Term == nil {
			continue
		}
		for _, s := range b.Term.Successors() {
			if !dom[b.Label][s] { // not a back edge
				continue
			}
			seen := map[string]bool{s: true}
			stack := []*Block{b}
			for len(stack) > 0 {
				x := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if seen[x.Label] {
					continue
				}
				seen[x.Label] = true
				stack = append(stack, pred[x.Label]...)
			}
			for label := range seen {
				inLoop[label] = true
			}
		}
	}
	return inLoop
}

// blockInNaturalLoop reports whether the block being emitted sits inside a
// natural loop, so a multiply there executes at least twice and can amortize
// the module's shared quarter-square table.
func (g *z80cg) blockInNaturalLoop() bool {
	if g.curBlock == nil || g.fn == nil {
		return false
	}
	if g.mulLoopFn != g.fn || g.mulLoopBlocks == nil {
		g.mulLoopFn = g.fn
		g.mulLoopBlocks = mulLoopBodyLabels(g.fn)
	}
	return g.mulLoopBlocks[g.curBlock.Label]
}

// genMul16QuarterSquare lowers a 16-bit multiply of two proven bytes through
// the module's shared quarter-square table. Emits 185 T when lhs >= rhs and
// 188 T when lhs < rhs (the NEG branch), against ~423 T for the 8-iteration
// shift-and-add loop it replaces. The 1022-byte table is emitted once per
// module, so only speed-mode multiplies inside a loop take this path.
//
// Inputs A = a, C = b (both 0..255); clobbers A, F, BC, DE, HL; IX/IY survive.
// Output HL = a*b.
//
//	LD B, A        ; 4   B = a
//	ADD A, C       ; 4   A = (a+b) & 255, CF = bit 8 of s = a+b
//	LD L, A        ; 4
//	LD H, 0        ; 7
//	RL H           ; 8   H = bit 8 of s → HL = s
//	ADD HL, HL     ; 11  HL = 2s
//	LD DE, __qsq   ; 10
//	ADD HL, DE     ; 11  HL = &sq[s]
//	LD E, (HL)     ; 7
//	INC HL         ; 6
//	LD D, (HL)     ; 7   DE = sq[s]
//	LD A, B        ; 4
//	SUB C          ; 4   A = a-b, CF = borrow
//	JRS NC, .pos   ; 12 taken (a≥b) / 7 not
//	NEG            ; 8   A = |a-b|
//	.pos:
//	LD L, A        ; 4
//	LD H, 0        ; 7
//	ADD HL, HL     ; 11  HL = 2|a-b|
//	LD BC, __qsq   ; 10
//	ADD HL, BC     ; 11  HL = &sq[|a-b|]
//	LD C, (HL)     ; 7
//	INC HL         ; 6
//	LD B, (HL)     ; 7   BC = sq[|a-b|]
//	EX DE, HL      ; 4   HL = sq[s]
//	OR A           ; 4
//	SBC HL, BC     ; 15  HL = sq[s] - sq[|a-b|] = a*b
func (g *z80cg) genMul16QuarterSquare(inst *Inst, pairLive func(string) bool) {
	dst := g.loc(inst.Dst)
	g.comment(fmt.Sprintf("mul16 %s * %s → HL (quarter-square %s)", g.loc(inst.Src[0]), g.loc(inst.Src[1]), quarterSquareTableLabel))
	// AF and DE are saved like the shift-and-add path does: A may hold a
	// pending accumulator the live set does not describe. A pair carrying the
	// result is never saved — the restore would overwrite it.
	g.emit("    PUSH AF")
	savedDE := dst != "DE" && dst != "D" && dst != "E"
	if savedDE {
		g.emit("    PUSH DE")
	}
	savedHL := dst != "HL" && dst != "H" && dst != "L" && pairLive("HL")
	if savedHL {
		g.emit("    PUSH HL")
	}
	savedBC := dst != "BC" && dst != "B" && dst != "C" && pairLive("BC")
	if savedBC {
		g.emit("    PUSH BC")
	}

	// Stage the operand bytes into A and C. The core is symmetric in its
	// operands, so assignment only has to avoid a load destroying the other
	// operand: a spill takes A (LD A,(nn) is the only direct byte load, and a
	// spill staged into C would have to borrow A), and a value already in A
	// keeps it. After that, the order decides which load runs first: C first
	// when its load reads or borrows A, A first when A's load would read C.
	x, y := inst.Src[0], inst.Src[1]
	lx, ly := g.loc(x), g.loc(y)
	if isSpill(ly) != isSpill(lx) && isSpill(ly) {
		x, y = y, x
		lx, ly = ly, lx
	} else if !isSpill(lx) && !isSpill(ly) && ly == "A" && lx != "A" {
		x, y = y, x
		lx, ly = ly, lx
	}
	loadByte := func(dstReg string, src Reg) {
		if cv, ok := g.constVals[src]; ok {
			g.emitf("    LD %s, %d", dstReg, cv&0xFF)
			return
		}
		loc := g.loc(src)
		if isSpill(loc) {
			g.loadSpill8(dstReg, loc)
		} else {
			g.emitLD8(dstReg, loc)
		}
	}
	switch {
	case ly == "A" || (isSpill(ly) && lx != "A"):
		// C's load reads A (LD C, A) or borrows it (spill): run it before A
		// is staged.
		loadByte("C", y)
		loadByte("A", x)
	default:
		// A's load would read C when lx is C, so A runs first anyway.
		loadByte("A", x)
		loadByte("C", y)
	}

	posLbl := fmt.Sprintf(".%s_qsq_%d_pos", sanitizeIdent(g.fn.Name), int(inst.Dst))
	g.emit("    LD B, A") // B = a
	g.emit("    ADD A, C")
	g.emit("    LD L, A")
	g.emit("    LD H, 0")
	g.emit("    RL H") // H = bit 8 of a+b
	g.emit("    ADD HL, HL")
	g.emitf("    LD DE, %s", quarterSquareTableLabel)
	g.emit("    ADD HL, DE") // HL = &sq[a+b]
	g.emit("    LD E, (HL)")
	g.emit("    INC HL")
	g.emit("    LD D, (HL)") // DE = sq[a+b]
	g.emit("    LD A, B")
	g.emit("    SUB C")
	g.emitf("    JRS NC, %s", posLbl)
	g.emit("    NEG") // A = |a-b|
	g.emitf("%s:", posLbl)
	g.emit("    LD L, A")
	g.emit("    LD H, 0")
	g.emit("    ADD HL, HL") // HL = 2*|a-b|
	g.emitf("    LD BC, %s", quarterSquareTableLabel)
	g.emit("    ADD HL, BC") // HL = &sq[|a-b|]
	g.emit("    LD C, (HL)")
	g.emit("    INC HL")
	g.emit("    LD B, (HL)") // BC = sq[|a-b|]
	g.emit("    EX DE, HL")  // HL = sq[a+b]
	g.emit("    OR A")
	g.emit("    SBC HL, BC") // HL = a*b
	g.needsQuarterSquare = true

	// Move the product to dst, then restore the saved pairs.
	if dst == "HL" {
		// already there
	} else if isIXY(dst) {
		g.emit("    PUSH HL")
		g.emitf("    POP %s", dst)
	} else if isSpill(dst) {
		g.storeSpill16(dst, "HL")
	} else {
		g.emitLD8(highByte(dst), "H")
		g.emitLD8(lowByte(dst), "L")
	}
	if savedBC {
		g.emit("    POP BC")
	}
	if savedHL {
		g.emit("    POP HL")
	}
	if savedDE {
		g.emit("    POP DE")
	}
	g.emit("    POP AF")
	g.invalidate("A")
	g.invalidate("F")
	g.invalidate("BC")
	g.invalidate("DE")
	g.invalidate(dst)
	if savedHL {
		g.invalidate("HL")
	}
}

// mul16QuarterSquareAdmissible reports whether inst may take the quarter-square
// table path: speed mode (the 1022-byte table is a size cost), both operands
// runtime values proven below 256 by provenance, and the multiply inside a
// natural loop so the table is amortized. Constants stay on the strength
// reduction / loop paths: their cheap recipes (2^n, 3, 5, 6, 9, 10, 12) cost
// far less than a table lookup, and admitting the rest would mean ordering this
// gate after the constant recipes instead of before the operand staging.
func (g *z80cg) mul16QuarterSquareAdmissible(inst *Inst) bool {
	if g.optimizeForSize || !g.blockInNaturalLoop() {
		return false
	}
	for _, src := range inst.Src[:2] {
		if _, isConst := g.constVals[src]; isConst {
			return false
		}
		if !g.provablyBelow256(src, inst.Ty.Width(), make(map[Reg]bool)) {
			return false
		}
	}
	return true
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
	g.materializePendingFlag(inst)
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
		g.genWideArithmetic(inst)
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
		// Assign the division workspace simultaneously: the divisor may be
		// in B, so loading the dividend into B first would overwrite it.
		if isPairReg(lhs) {
			lhs = lowByte(lhs)
		}
		if isPairReg(rhs) {
			rhs = lowByte(rhs)
		}
		g.emitParallelCopy([]parallelCopy{
			{srcName: lhs, dstName: "B", ty: TyU8},
			{srcName: rhs, dstName: "C", ty: TyU8},
		})

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
		// The quotient can have a memory home under register pressure.
		// Use the same location-aware byte copy as the remainder path.
		g.emitLD8(dst, "B")
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

// provablyBelow256 reports whether r's bits above bit 7 are provably zero, so
// the shift-and-add multiply over r needs only eight of its sixteen
// iterations. Like nonNegative, only provenance qualifies — never a guess:
// zero extension from an at-most-byte source (OpExt fills with zeros whatever
// the source signedness), an unsigned parameter narrower than the multiply, a
// constant whose bit pattern stays under 256, a copy of a proven value, and a
// sign extension whose source is separately proven non-negative. A sign
// extension of an arbitrary byte keeps a possible 0xFF high byte and must keep
// all sixteen iterations.
func (g *z80cg) provablyBelow256(r Reg, width int, seen map[Reg]bool) bool {
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
				return i.SrcTy != nil && i.SrcTy.Width() <= 8
			case OpSext:
				return i.SrcTy != nil && i.SrcTy.Width() <= 8 && g.nonNegative(i.Src[0], width, seen)
			case OpConst:
				return i.Imm >= 0 && i.Imm < 256
			case OpMove:
				return g.provablyBelow256(i.Src[0], width, seen)
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
	} else if !signed && w == 16 && k == 256 {
		// Logical division by 256 extracts the original high byte.
		g.emit("    LD L, H")
		g.emit("    LD H, 0")
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
		// A byte result may share a pair with an unrelated live byte.
		// Preserve that neighbour rather than skipping the whole pair.
		if dst == pair || (pair == "AF" && dst == "A") {
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
				pair := saved[i]
				byteResult := dst == highByte(pair) || dst == lowByte(pair)
				if byteResult {
					// Stage the result in the alternate accumulator. Main A
					// may hold an unrelated live or pending value even when
					// the load itself never writes A.
					g.emit("    EX AF, AF'")
					g.emitf("    LD A, %s", dst)
				}
				g.emitf("    POP %s", pair)
				if byteResult {
					g.emitf("    LD %s, A", dst)
					g.emit("    EX AF, AF'")
					g.invalidate("A")
				}
				g.invalidate(pair)
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
		// A primary word store temporarily advances HL and restores it itself.
		// Counting both INC and DEC as lasting writes would add a redundant
		// PUSH/POP around every store through a loop-carried pointer.
		if inst.Op == OpStore && inst.Ty == TyU16 && restoredHLWordStore(body.String()) {
			delete(writes, "H")
			delete(writes, "L")
		}
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

// Admit only the straight-line, canonical four-instruction word store. No
// branch, aliasing byte write or unrelated pointer arithmetic can hide here.
func restoredHLWordStore(asm string) bool {
	var lines []string
	for _, line := range strings.Split(asm, "\n") {
		line = strings.Join(strings.Fields(strings.SplitN(line, ";", 2)[0]), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 4 || lines[1] != "INC HL" || lines[3] != "DEC HL" {
		return false
	}
	valid := func(line string) bool {
		switch line {
		case "LD (HL), B", "LD (HL), C", "LD (HL), D", "LD (HL), E", "LD (HL), A":
			return true
		}
		return false
	}
	return valid(lines[0]) && valid(lines[2])
}
