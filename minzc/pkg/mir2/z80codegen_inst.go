package mir2

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ── Instructions ──────────────────────────────────────────────────────────────

func (g *z80cg) genInst(inst *Inst) {
	// Instructions merged into a page-aligned LUT load are skipped here;
	// their code is emitted by the Load case below.
	if g.lutSkip[inst.Dst] {
		return
	}

	if inst.Op != OpStore && g.bitStoreSkip[inst.Dst] {
		return
	}

	if inst.Op != OpCmp && g.bitCmpSkip[inst.Dst] {
		return
	}

	// Instructions merged into a global-field direct-addressing sequence are
	// skipped here; their code is emitted by the Load/Store case below.
	// (Stores have Dst==NoReg; check the addr-reg via the instruction's Src[0].)
	if inst.Op != OpStore && g.globalFieldSkip[inst.Dst] {
		return
	}

	// Scratch relocations of A are instruction-local. Return them to their
	// allocated home before another instruction can allocate that scratch. A dead
	// relocation must not overwrite a newer value that shares the allocated A.
	// Keep the current destination override: recursive spill staging owns it.
	liveOverrides := g.regsLiveAfterInst(inst)
	for _, r := range inst.Uses() {
		liveOverrides[r] = true
	}
	delete(liveOverrides, inst.Dst)
	for _, r := range slices.Sorted(maps.Keys(g.physOverride)) {
		if r != inst.Dst && !liveOverrides[r] {
			delete(g.physOverride, r)
			continue
		}
		loc := g.physOverride[r]
		if g.ar.Loc(r).Name == "A" && loc != "A" && loc != "F" {
			g.emitLD8("A", loc)
			delete(g.physOverride, r)
			g.invalidate("A")
		}
	}

	// Flag-clobbering ops clear lastFlagsLhs/Rhs explicitly in their cases.
	// Flag-preserving ops (LD, PUSH, POP, …) leave the tracker intact so that
	// the Sub+CmpLt flag-fusion peephole can fire across an intervening LD.

	dst := g.loc(inst.Dst)

	if g.genWideMemory(inst) {
		return
	}
	if g.genWideSpills(inst) {
		return
	}

	switch inst.Op {
	case OpConst:
		// Only record as constant if the Dst is NOT a block parameter.
		// After PreallocCoalesce, an OpConst may share a Reg with a block param
		// (loop counter). The initial value is constant but the param changes
		// each iteration — treating it as constant produces wrong code.
		if !g.blockParamRegs[inst.Dst] {
			g.constVals[inst.Dst] = inst.Imm
		}
		if dst == "F" {
			// Bool constant into flag register: SCF (true) or AND A (false, clears C+sets Z).
			g.lastFlagsLhs = ""
			g.lastFlagsRhs = ""
			if inst.Imm != 0 {
				g.emit("    SCF")
			} else {
				g.emit("    AND A") // A=A (no change), clears C, sets Z based on A
			}
			return
		}
		if !g.deadConsts[inst.Dst] {
			w := inst.Ty.Width()
			if isZ80WideInt(inst.Ty) {
				// 24/32-bit constant via shadow pair.
				//   LD rr, lo16   (10T)
				//   EXX           (4T)
				//   LD rr, hi     (10T) — hi8 for u24, hi16 for u32
				//   EXX           (4T)  = 28T total
				// For u24: hi is at most 0x00FF, so shadow H' is always 0.
				lo := uint32(inst.Imm) & 0xFFFF
				hi := uint32(inst.Imm) >> 16
				g.emitf("    LD %s, %d", dst, lo)
				g.emit("    EXX")
				g.emitf("    LD %s, %d", dst, hi)
				g.emit("    EXX")
			} else if isSpill(dst) {
				// LocMem spill destination.
				// Loading an immediate for a spill must preserve unrelated live
				// values in the staging register.
				if w <= 8 {
					g.emit("    PUSH AF")
				} else {
					g.emit("    PUSH HL")
				}
				// TSMC: if eligible, patch reload sites instead of memory store.
				if pair := g.tsmcSpillPairFor(inst.Dst); pair != nil {
					if w <= 8 {
						g.emitf("    LD A, %d", inst.Imm&0xFF)
						g.emitTSMCSpill(pair, "A")
						g.invalidate("A")
					} else {
						// Word TSMC stores use A to patch both immediate bytes.
						// OpConst does not define A or flags; preserve live inputs
						// even when orphaned reloads later become ordinary spills.
						g.emit("    PUSH AF")
						g.emitf("    LD HL, %d", inst.Imm&0xFFFF)
						g.emitTSMCSpill(pair, "HL")
						g.emit("    POP AF")
						g.invalidate("HL")
					}
				} else {
					// Traditional spill
					if w <= 8 {
						g.emitf("    LD A, %d", inst.Imm&0xFF)
						g.emitNativeSpillf(dst, 0, 1, true, "    LD (%s), A", dst)
						g.invalidate("A")
					} else {
						g.emitf("    LD HL, %d", inst.Imm&0xFFFF)
						g.emitNativeSpillf(dst, 0, 2, true, "    LD (%s), HL", dst)
						g.invalidate("HL")
					}
				}
				if w <= 8 {
					g.emit("    POP AF")
				} else {
					g.emit("    POP HL")
				}
			} else {
				g.emitf("    LD %s, %d", dst, inst.Imm)
			}
			// An immediate replaces the destination's value, including any
			// tracked copy in A. Otherwise a later ALU op can skip LD A,dst
			// using an alias established before this constant was loaded.
			g.invalidate(dst)
		}

	case OpMove:
		src := g.loc(inst.Src[0])
		if dst == src {
			return // no-op
		}
		// Absolute byte spill transfers use A as a scratch register, even
		// when neither logical operand lives there. Keep an unrelated live
		// accumulator (for example the first argument of a following CALL).
		var restore func()
		if z80SpillBytes(inst.Ty) == 1 && dst != "A" && src != "A" && (isSpill(dst) || isSpill(src)) {
			restore = g.saveAccIfLive(inst)
		}
		g.emitMov(dst, src, 8*z80SpillBytes(inst.Ty))
		if restore != nil {
			restore()
		}

	case OpAdd:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genBinOp("ADD", inst)

	case OpSub:
		g.lastFlagsLhs = "" // clear stale tracking before this sub
		g.lastFlagsRhs = ""
		g.genBinOp("SUB", inst)
		// Record that flags were set by SUB(lhs, rhs) for the Sub+CmpLt fusion.
		// Only track 8-bit subs; 16-bit uses SBC which sets different flags.
		if inst.Ty.Width() <= 8 {
			rhsP := g.loc(inst.Src[1])
			if cv, ok := g.constVals[inst.Src[1]]; ok {
				rhsP = fmt.Sprintf("%d", cv)
			}
			if rhsP != "A" { // skip NEG-trick case (rhs was in A)
				g.lastFlagsLhs = g.loc(inst.Src[0]) // logical lhs
				g.lastFlagsRhs = rhsP
			}
		}

	case OpMul:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genMul(inst)

	case OpDiv, OpSDiv, OpSMod, OpMod:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genDivMod(inst)

	case OpAnd:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genBinOp("AND", inst)

	case OpOr:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genBinOp("OR", inst)

	case OpXor:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genBinOp("XOR", inst)

	case OpBitGet:
		g.materializePendingFlag(inst)
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		src := g.loc(inst.Src[0])
		bit := int(inst.Imm)
		byteOffset := bit / 8
		bitInByte := bit % 8
		target := selectBitReg(src, byteOffset)
		if isSpill(src) {
			if inst.SrcTy != nil && inst.SrcTy.Width() > 8 {
				g.loadSpill16("HL", src)
				target = selectBitReg("HL", byteOffset)
			} else {
				g.emitLD8("A", src)
				target = "A"
			}
		}
		if target != "A" {
			g.emitLDA(target)
		}
		for i := 0; i < bitInByte; i++ {
			g.emit("    SRL A")
		}
		g.emit("    AND 1")
		g.invalidate("A")
		if dst != "A" {
			g.emitLD8(dst, "A")
		} else {
			g.pendingAccReg = inst.Dst
		}

	case OpBitSet, OpBitReset:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		src := g.loc(inst.Src[0])
		bit := int(inst.Imm)
		byteOffset := bit / 8
		bitInByte := bit % 8
		op := "SET"
		if inst.Op == OpBitReset {
			op = "RES"
		}
		if dst != src {
			g.emitMov(dst, src, inst.Ty.Width())
		}
		target := selectBitReg(dst, byteOffset)
		if g.emitBitRegOp(op, bitInByte, target) {
			g.invalidate(dst)
			g.invalidate(target)
			break
		}
		if isSpill(dst) && inst.Ty.Width() > 8 {
			g.loadSpill16("HL", dst)
			target = selectBitReg("HL", byteOffset)
			g.emitLDA(target)
			if op == "SET" {
				g.emitf("    OR %d", 1<<bitInByte)
			} else {
				g.emitf("    AND %d", ^(1<<bitInByte)&0xFF)
			}
			g.emitLD8(target, "A")
			g.storeSpill16(dst, "HL")
			g.invalidate("HL")
			g.invalidate("A")
			break
		}
		if target != "A" {
			g.emitLDA(target)
		}
		if op == "SET" {
			g.emitf("    OR %d", 1<<bitInByte)
		} else {
			g.emitf("    AND %d", ^(1<<bitInByte)&0xFF)
		}
		g.invalidate("A")
		if target != "A" {
			g.emitLD8(target, "A")
		}
		if dst != "A" && target == "A" {
			g.emitLD8(dst, "A")
		}
		g.invalidate(dst)

	case OpNeg:
		g.materializePendingFlag(inst)
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		if inst.Ty.Width() >= 24 {
			g.genWideArithmetic(inst)
		} else if inst.Ty.Width() == 16 {
			defer g.saveWrittenScratch(inst)()
			// The allocator may place either operand in a pair or a spill.
			// NEG's fixed HL sequence must consume the actual source and
			// publish its result at the allocated destination.
			src, dst := g.loc(inst.Src[0]), g.loc(inst.Dst)
			if src != "HL" {
				g.genUnsignedExtBytes("HL", src, 16, 16)
			}
			// 16-bit NEG HL using the classic demo-scene SBC A,A trick:
			//   XOR A      ; A=0, CF=0
			//   SUB L      ; A = 0-L, CF=1 if L≠0 (borrow)
			//   LD L, A    ; save negated low byte
			//   SBC A, A   ; A = 0-CF → 0x00 or 0xFF (propagate borrow)
			//   SUB H      ; A = -CF - H  (correct high byte with borrow)
			//   LD H, A    ; save negated high byte
			// 6 bytes, 24T — saves 2 bytes and 7T vs NEG-based sequence.
			g.emit("    XOR A")
			g.emit("    SUB L")
			g.emit("    LD L, A")
			g.emit("    SBC A, A")
			g.emit("    SUB H")
			g.emit("    LD H, A")
			if dst != "HL" {
				g.genUnsignedExtBytes(dst, "HL", 16, 16)
			}
			g.invalidate("A")
			g.invalidate("HL")
			g.invalidate(dst)
		} else {
			// Z80 NEG always operates on A; ensure source is in A first.
			srcLoc := g.loc(inst.Src[0])
			if srcLoc != "A" && !g.holdsValue("A", srcLoc) {
				g.emitLDA(srcLoc)
			}
			g.emit("    NEG")
			g.invalidate("A")
			// Record that inst.Dst is now in A so TermRet can skip the redundant move.
			if g.loc(inst.Dst) != "A" {
				g.physOverride[inst.Dst] = "A"
			}
		}

	case OpNot:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genNot(inst)
		g.invalidate("A")

	case OpShl:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genShift("SLA", inst)

	case OpShr:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genShift("SRL", inst)

	case OpSar:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genShift("SRA", inst)

	case OpExt:
		// Zero-extend srcTy → Ty.  Primary case: u8 → u16 into HL.
		// genExt may emit XOR (flag-clobbering) — clear to be safe.
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genExt(inst)

	case OpSext:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.genSext(inst)

	case OpTrunc:
		if inst.SrcTy != nil && isZ80WideInt(inst.SrcTy) && inst.Ty.Width() == 16 {
			// Wide integers carry their low word in the main pair. Preserve
			// both bytes, including overlapping allocated destinations.
			g.genUnsignedExtBytes(dst, g.loc(inst.Src[0]), 16, 16)
			break
		}
		if g.loc(inst.Src[0]) == "F" || inst.Src[0] == g.pendingFlagReg {
			// A predicate has no byte register to copy. Its truncation keeps
			// the canonical 0/1 value and uses the same live-safe materializer
			// as boolean extension (including allocated flag aliases).
			g.genExt(inst)
			break
		}
		src := g.loc(inst.Src[0])
		// Truncate 16-bit → 8-bit: take low byte.
		lo := lowByte(src)
		if dst == lo {
			// Already in place (e.g. dst=L, src=HL → L already has the value).
		} else {
			// emitLD8 handles DD/FD prefix conflicts through shadow A.
			// A is implicit scratch, not this truncation's destination; a
			// live accumulator (for example the original C argument) survives.
			g.emitLD8(dst, lo)
		}

	case OpCmp:
		g.genCmp(inst)

	case OpLoad:
		if isSpill(g.loc(inst.Src[0])) {
			defer g.saveWrittenScratch(inst)()
		}
		// Some byte loads need A as an encoding bridge, even though their
		// allocated destination is elsewhere. Preserve the accumulator and
		// flags, including live loop parameters, across that implicit scratch.
		// BC/DE byte loads use A even for a spilled destination. HL spills
		// preserve A in their own branch below and must not be saved twice.
		_, lutLoad := g.lutLoadPat[inst.Dst]
		loadPtr := g.loc(inst.Src[0])
		if inst.Ty.Width() <= 8 && dst != "A" &&
			((loadPtr == "BC" || loadPtr == "DE") || !isSpill(dst) && (lutLoad || isIXYReg(dst))) {
			g.emit("    EX AF, AF'      ; preserve byte-load scratch")
			defer func() {
				g.emit("    EX AF, AF'      ; restore byte-load scratch")
				g.invalidate("A") // restored A is not an alias of loaded dst
			}()
		}
		// Page-aligned LUT fast path (Phase 6e — BC★/DE★ optimization):
		//
		//   Baseline (HL path, 18T):
		//     LD H, sym^H  ;  7T  — only need the page (high byte)
		//     LD L, idx8   ;  4T  — L is overwritten anyway
		//     LD A, (HL)   ;  7T
		//
		//   BC★ (14T, saves 4T when idx is already in C):
		//     LD B, sym^H  ;  7T  — page base into B
		//                  ;  0T  — C already holds the index
		//     LD A, (BC)   ;  7T
		//
		//   DE★ (14T, saves 4T when idx is already in E):
		//     LD D, sym^H  ;  7T  — page base into D
		//                  ;  0T  — E already holds the index
		//     LD A, (DE)   ;  7T
		//
		// Note: BC★ and DE★ fire as a post-allocation codegen check.
		// Phase 6e PBQP nudge (see pbqp_affinity.go) biases the allocator
		// toward C/E for LUT indices so these paths fire more often.
		if pat, ok := g.lutLoadPat[inst.Dst]; ok {
			// Page/address scratch is not the MIR load's destination. A live
			// high byte (B/D) or HL value must survive the selected sequence.
			defer g.saveWrittenScratch(inst)()
			src8 := g.loc(pat.src8Reg)
			sym := sanitizeIdent(pat.sym)
			switch src8 {
			case "C": // BC★: index already in C — use B as page-high
				g.emitf("    LD B, %s^H    ; BC★ LUT (14T)", sym)
				g.invalidate("B")
				if dst != "A" {
					g.emitNativeMemoryf(inst, 0, 1, "    LD A, (BC)")
					g.invalidate("A")
					g.emitMov(dst, "A", 8)
				} else {
					g.emitNativeMemoryf(inst, 0, 1, "    LD A, (BC)")
					g.invalidate("A")
				}
			case "E": // DE★: index already in E — use D as page-high
				g.emitf("    LD D, %s^H    ; DE★ LUT (14T)", sym)
				g.invalidate("D")
				if dst != "A" {
					g.emitNativeMemoryf(inst, 0, 1, "    LD A, (DE)")
					g.invalidate("A")
					g.emitMov(dst, "A", 8)
				} else {
					g.emitNativeMemoryf(inst, 0, 1, "    LD A, (DE)")
					g.invalidate("A")
				}
			default: // HL path (18T)
				g.emitf("    LD H, %s^H", sym)
				g.invalidate("H")
				g.emitLD8("L", src8)
				g.invalidate("L")
				if dst != "A" {
					g.emitNativeMemoryf(inst, 0, 1, "    LD A, (HL)")
					g.invalidate("A")
					g.emitMov(dst, "A", 8)
				} else {
					g.emitNativeMemoryf(inst, 0, 1, "    LD A, (HL)")
					g.invalidate("A")
				}
			}
			break
		}
		// Global struct field direct-addressing fast path:
		//   LD A, (sym__field)   ; 13T, 3 bytes — direct memory addressing
		// Replaces: LD HL,sym + INC HL×N + LD r,(HL) + (LD A,r)  = 33–43T, 4–8 bytes.
		// Only u8 loads where dst == A are eligible without clobbering risk.
		// When dst != A the intermediates (AddrOf/Field) are NOT skipped in this path,
		// so the original HL-indirect code is emitted instead.
		if info, ok := g.globalFieldLoad[inst.Dst]; ok && dst == "A" {
			lbl := globalFieldLabel(info.sym, info.offset, info.fieldName)
			g.emitNativeMemoryf(inst, 0, 1, "    LD A, (%s)", lbl)
			g.invalidate("A")
			break
		}
		// A checked single-use literal displacement needs no address pair or A scratch.
		if inst.Ty.Width() == 16 && (dst == "HL" || dst == "DE" || dst == "BC") {
			if info, ok := g.globalFieldAddr[inst.Src[0]]; ok && g.globalFieldSkip[inst.Src[0]] {
				g.emitNativeMemoryf(inst, 0, 2, "    LD %s, (%s)    ; literal word load, no scratch clobber", dst, globalFieldLabel(info.sym, info.offset, info.fieldName))
				g.invalidate(dst)
				break
			}
		}
		// Direct global word loads preserve all registers except their destination.
		// The matcher accepts only a single-use AddrOf(global) and HL/DE/BC.
		// Avoids the INC/DEC trick (LD A,(HL)/INC HL/LD H,(HL)/LD L,A) that
		// uses A as scratch and corrupts loop counters stored in A.
		if inst.Ty.Width() == 16 {
			if sym, ok := g.globalU16LoadSym[inst.Src[0]]; ok && (dst == "HL" || dst == "DE" || dst == "BC") {
				if dst == "HL" {
					g.emitNativeMemoryf(inst, 0, 2, "    LD HL, (%s)    ; u16 direct load (20T, no A clobber)", sanitizeIdent(sym))
				} else {
					g.emitNativeMemoryf(inst, 0, 2, "    LD %s, (%s)    ; u16 direct load, no scratch clobber", dst, sanitizeIdent(sym))
				}
				g.invalidate(dst)
				break
			}
		}
		ptr := g.loc(inst.Src[0])
		w := inst.Ty.Width()
		// If the POINTER value itself is spilled to a memory slot ($Fxxx), reload it
		// into HL first. Z80 supports LD HL,(nn) for this (3 bytes, 16T).
		// After this, ptr=="HL" and the rest of the OpLoad path works unchanged.
		if isSpill(ptr) {
			g.emitNativeSpillf(ptr, 0, 2, false, "    LD HL, (%s)   ; reload spilled ptr", ptr)
			g.invalidate("HL")
			ptr = "HL"
		}
		if pat, ok := g.bitStorePat[inst.Src[1]]; ok {
			if g.emitBitMemOp(pat.op, pat.bit, ptr, pat.byteOffset) {
				break
			}
		}
		// Word loads may need A to bridge indirect/prefix restrictions. Keep
		// live accumulator values and flags intact without borrowing a pair.
		preserveWordAF := w == 16 && (dst == ptr || isSpill(dst) || ptr == "BC" || ptr == "DE" ||
			isIXY(dst) || (isIXY(ptr) && dst == "HL"))
		if preserveWordAF {
			g.emit("    PUSH AF          ; preserve word-load scratch")
		}
		if w == 24 {
			// 24-bit load (3 bytes little-endian) into DWord shadow pair.
			// Shadow H' (high byte of hi16) is always zero for u24 semantics.
			if isIXY(ptr) {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s     ; byte0 lo_lo", lowByte(dst), ptrIndirect(ptr, 0))
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, %s     ; byte1 lo_hi", highByte(dst), ptrIndirect(ptr, 1))
				g.emit("    EXX")
				g.emitNativeMemoryf(inst, 2, 1, "    LD %s, %s     ; byte2 hi_lo (shadow)", lowByte(dst), ptrIndirect(ptr, 2))
				g.emitf("    LD %s, 0        ; hi_hi always 0 for u24", highByte(dst))
				g.emit("    EXX")
			} else {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, (%s)     ; byte0 lo_lo", lowByte(dst), ptr)
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, (%s)     ; byte1 lo_hi", highByte(dst), ptr)
				g.emitf("    INC %s", ptr)
				g.emit("    EXX")
				g.emitNativeMemoryf(inst, 2, 1, "    LD %s, (%s)     ; byte2 hi8 (shadow lo)", lowByte(dst), ptr)
				g.emitf("    LD %s, 0        ; hi_hi always 0 for u24", highByte(dst))
				g.emit("    EXX")
				g.emitf("    DEC %s", ptr)
				g.emitf("    DEC %s", ptr)
			}
		} else if w == 32 {
			// 32-bit load via ptr (must not be same pair as dst DWord).
			if isIXY(ptr) {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s     ; byte0 lo_lo", lowByte(dst), ptrIndirect(ptr, 0))
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, %s     ; byte1 lo_hi", highByte(dst), ptrIndirect(ptr, 1))
				g.emit("    EXX")
				g.emitNativeMemoryf(inst, 2, 1, "    LD %s, %s     ; byte2 hi_lo (shadow)", lowByte(dst), ptrIndirect(ptr, 2))
				g.emitNativeMemoryf(inst, 3, 1, "    LD %s, %s     ; byte3 hi_hi (shadow)", highByte(dst), ptrIndirect(ptr, 3))
				g.emit("    EXX")
			} else {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, (%s)     ; byte0 lo_lo", lowByte(dst), ptr)
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, (%s)     ; byte1 lo_hi", highByte(dst), ptr)
				g.emitf("    INC %s", ptr)
				g.emit("    EXX")
				g.emitNativeMemoryf(inst, 2, 1, "    LD %s, (%s)     ; byte2 hi_lo (shadow)", lowByte(dst), ptr)
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 3, 1, "    LD %s, (%s)     ; byte3 hi_hi (shadow)", highByte(dst), ptr)
				g.emit("    EXX")
				g.emitf("    DEC %s", ptr)
				g.emitf("    DEC %s", ptr)
				g.emitf("    DEC %s", ptr)
			}
		} else if w <= 8 {
			// Z80 restriction: only LD A,(BC) and LD A,(DE) exist for BC/DE indirect.
			// Any other destination register must load through A first.
			if (ptr == "BC" || ptr == "DE") && dst != "A" {
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, %s", ptrIndirect(ptr, 0))
				g.invalidate("A")
				g.emitMov(dst, "A", 8)
			} else if isIXYReg(dst) && ptr == "HL" {
				// DD/FD prefix conflict: LD IXH,(HL) encodes as LD IXH,(IX+0) — invalid.
				// Route through A.
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, (HL)")
				g.invalidate("A")
				g.emitLD8(dst, "A")
			} else if isIXYReg(dst) && isIXY(ptr) {
				// BUG-008: LD IXL,(IX+d) impossible — DD prefix can't remap both.
				// Also self-clobber: writing IXL changes IX base for next read.
				// Route through A.
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, %s", ptrIndirect(ptr, 0))
				g.invalidate("A")
				g.emitLD8(dst, "A")
			} else if isSpill(dst) {
				g.emit("    EX AF, AF'")
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, %s", ptrIndirect(ptr, 0))
				g.emitNativeSpillf(dst, 0, 1, true, "    LD (%s), A", dst)
				g.emit("    EX AF, AF'")
			} else {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s", dst, ptrIndirect(ptr, 0))
			}
		} else if dst == ptr && isPairReg(ptr) {
			// Read both bytes before changing either byte of the address pair.
			// Staging the low byte on the stack works for HL/BC/DE and IX/IY,
			// including prefix restrictions, without clobbering another live pair.
			g.emitNativeMemoryf(inst, 0, 1, "    LD A, %s     ; lo", ptrIndirect(ptr, 0))
			g.emit("    PUSH AF          ; stage low byte before replacing address")
			if isIXY(ptr) {
				g.emitNativeMemoryf(inst, 1, 1, "    LD A, %s     ; hi", ptrIndirect(ptr, 1))
			} else {
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 1, 1, "    LD A, (%s)     ; hi", ptr)
			}
			g.emitLD8(highByte(dst), "A")
			g.emit("    POP AF")
			g.emitLD8(lowByte(dst), "A")
			g.invalidate(dst)
			g.invalidate("A")
		} else if isIXY(ptr) {
			// 16-bit load via IX/IY: use displacement addressing.
			lo := lowByte(dst)
			hi := highByte(dst)
			if isIXYReg(lo) || isIXYReg(hi) || lo == "H" || lo == "L" || hi == "H" || hi == "L" || isSpill(dst) {
				// BUG-008: dest overlaps IX prefix domain — route through A.
				// IXH/IXL: LD IXL,(IX+d) impossible (DD prefix conflict).
				// H/L: LD H,(IX+d) encodes as LD IXH,(IX+d) (prefix substitution).
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, %s     ; lo", ptrIndirect(ptr, 0))
				g.emitLD8(lo, "A")
				g.emitNativeMemoryf(inst, 1, 1, "    LD A, %s     ; hi", ptrIndirect(ptr, 1))
				g.emitLD8(hi, "A")
				g.invalidate("A")
			} else {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s     ; lo", lo, ptrIndirect(ptr, 0))
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, %s     ; hi", hi, ptrIndirect(ptr, 1))
			}
		} else if isSpill(dst) {
			// 16-bit load into LocMem spill slot: load through HL, then
			// use LD (nn), HL to store both bytes atomically.
			if ptr == "HL" {
				// Self-pointer: LD A,(HL)/INC/LD H,(HL)/LD L,A
				g.emit("    PUSH HL          ; save ptr for spill load")
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, (HL)     ; lo")
				g.emit("    INC HL")
				g.emitNativeMemory(inst, 1, 1, "    LD H, (HL)     ; hi")
				g.emit("    LD L, A        ; lo")
				g.emitNativeSpillf(dst, 0, 2, true, "    LD (%s), HL", dst)
				g.invalidate("HL")
				g.invalidate("A")
				g.emit("    POP HL           ; restore ptr")
			} else {
				// BC/DE indirect uses HL as scratch. A spilled result does not
				// own HL; retain any earlier live value before loading either byte.
				restoreHL := g.saveLiveScratch(inst, "HL")
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, (%s)     ; lo", ptr)
				g.emit("    LD L, A")
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 1, 1, "    LD A, (%s)     ; hi", ptr)
				g.emit("    LD H, A")
				g.emitf("    DEC %s", ptr)
				g.emitNativeSpillf(dst, 0, 2, true, "    LD (%s), HL", dst)
				g.invalidate("HL")
				g.invalidate("A")
				restoreHL()
			}
		} else {
			// 16-bit load via HL/DE/BC: INC/DEC trick — little-endian
			if ptr == "BC" || ptr == "DE" {
				// BC/DE indirect: only LD A,(rr) is valid — both bytes must go through A.
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, (%s)     ; lo", ptr)
				g.emitf("    LD %s, A", lowByte(dst))
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 1, 1, "    LD A, (%s)     ; hi", ptr)
				g.emitf("    LD %s, A", highByte(dst))
				g.emitf("    DEC %s", ptr)
				g.invalidate("A")
			} else if isIXYReg(lowByte(dst)) || isIXYReg(highByte(dst)) {
				// DD/FD prefix conflict: LD IXL,(HL) impossible.
				// Route through A.
				g.emitNativeMemoryf(inst, 0, 1, "    LD A, (%s)     ; lo", ptr)
				g.emitf("    LD %s, A", lowByte(dst))
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 1, 1, "    LD A, (%s)     ; hi", ptr)
				g.emitf("    LD %s, A", highByte(dst))
				g.emitf("    DEC %s", ptr)
				g.invalidate("A")
			} else {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, (%s)     ; lo", lowByte(dst), ptr)
				g.emitf("    INC %s", ptr)
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, (%s)     ; hi", highByte(dst), ptr)
				g.emitf("    DEC %s", ptr)
			}
		}

		if preserveWordAF {
			g.emit("    POP AF           ; restore word-load scratch")
		}

		// POP HL to restore the base ptr that was saved by OpPtrAdd (scanPtrAddBase).
		// The ptr_add emitted PUSH HL before ADD HL,rr; we restore it here, after the
		// load that consumed the ptr_add result.  This puts the original pointer back
		// in HL so the back-edge parallel copy can read it correctly.
		if g.ptrAddPushHL == inst.Src[0] {
			g.emit("    POP HL         ; restore base ptr (saved by ptr_add)")
			g.ptrAddPushHL = NoReg
			g.invalidate("HL")
		}

	case OpStore:
		if g.runtime == Z80RuntimeCPM {
			g.genCPMGlobalStoreGuard(inst)
		}
		if isSpill(g.loc(inst.Src[0])) || inst.Ty.Width() == 16 {
			defer g.saveWrittenScratch(inst)()
		}
		if g.ptrAddPushHL == inst.Src[0] {
			defer func() {
				g.emit("    POP HL         ; restore base ptr after store")
				g.ptrAddPushHL = NoReg
			}()
		}
		// Byte stores through BC/DE and index-register encodings use A
		// as an implicit scratch. A may still hold the next loop index or
		// another live value, even when it is not an operand of this store.
		if inst.Ty.Width() <= 8 {
			ptr, val := g.loc(inst.Src[0]), g.loc(inst.Src[1])
			_, constant := g.constVals[inst.Src[1]]
			indexedScratch := isIXY(ptr) && !constant && (val == "H" || val == "L" || isIXYReg(val) || isSpill(val) || isPairReg(val) && (lowByte(val) == "H" || lowByte(val) == "L" || isIXYReg(lowByte(val))))
			if val != "A" && (ptr == "BC" || ptr == "DE" || indexedScratch) {
				liveA := false
				for r := range g.regsLiveAfterInst(inst) {
					if g.loc(r) == "A" {
						liveA = true
						break
					}
				}
				if liveA {
					g.emit("    PUSH AF    ; preserve live A across byte store")
					defer func() { g.emit("    POP AF"); g.invalidate("A") }()
				}
			}
		}
		addrReg := inst.Src[0]
		if info, ok := g.globalFieldStore[addrReg]; ok && inst.Ty.Width() == 8 {
			if info.hlChain {
				// HL-chain: adjacent field stores share one address load.
				//   head:  LD HL, sym+offset   ; 10T
				//   each:  LD (HL), r   ; 7T  (r already in correct reg)
				//          INC HL       ; 6T  (omitted for last)
				if info.hlHead {
					g.emitf("    LD HL, %s", globalFieldLabel(info.sym, info.offset, ""))
				}
				if cv, ok := g.constVals[inst.Src[1]]; ok {
					g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), %d", cv)
				} else {
					val := g.loc(inst.Src[1])
					if isPairReg(val) && !isIXY(val) {
						// 16-bit value → store byte-by-byte: LD (HL),lo; INC HL; LD (HL),hi; DEC HL
						g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), %s", lowByte(val))
						g.emit("    INC HL")
						g.emitNativeMemoryf(inst, 1, 1, "    LD (HL), %s", highByte(val))
						g.emit("    DEC HL")
					} else if isIXYReg(val) || isIXY(val) || isSpill(val) {
						g.emit("    EX AF, AF'")
						if isSpill(val) {
							g.emitNativeSpillf(val, 0, 1, false, "    LD A, (%s)", val)
						} else {
							g.emitLDA(val)
						}
						g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), A")
						g.emit("    EX AF, AF'")
					} else {
						g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), %s", val)
					}
				}
				if !info.hlLast {
					g.emitf("    INC HL")
				}
				g.invalidate("HL")
			} else {
				// Global struct field direct-addressing fast path.
				lbl := globalFieldLabel(info.sym, info.offset, info.fieldName)
				if cv, ok := g.constVals[inst.Src[1]]; ok {
					// Constant store: LD HL, sym; LD (HL), imm — 10T+10T = 20T, no A clobber.
					// Avoids LD A, imm (4T) + LD (sym), A (13T) which would destroy A (= first param).
					preserveHL := false
					for r := range g.regsLiveAfterInst(inst) {
						switch g.loc(r) {
						case "HL", "H", "L":
							preserveHL = true
						}
					}
					if preserveHL {
						g.emit("    PUSH HL    ; preserve live HL across direct constant store")
					}
					g.emitf("    LD HL, %s", lbl)
					g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), %d", cv)
					if preserveHL {
						g.emit("    POP HL")
					}
					g.invalidate("HL")
				} else {
					//   LD A, val           ; 4T (omitted when val is already A)
					//   LD (sym__field), A  ; 13T
					val := g.loc(inst.Src[1])
					preserveA := false
					if val != "A" {
						for r := range g.regsLiveAfterInst(inst) {
							if r != inst.Src[1] && g.loc(r) == "A" {
								preserveA = true
								break
							}
						}
						if preserveA {
							g.emit("    PUSH AF    ; preserve live A across direct store")
						}
						g.emitLDA(val)
					}
					g.emitNativeMemoryf(inst, 0, 1, "    LD (%s), A", lbl)
					if preserveA {
						g.emit("    POP AF")
						g.invalidate("A")
					}
				}
			}
			break
		}
		// u16 direct-address store: LD (sym), HL — 16T, no A clobber.
		// Fires when the address register traces back to OpAddrOf(global) and the
		// value is allocated to HL, or an admitted DE/BC adjacent single-use pair.
		// Avoids byte-by-byte stores and their scratch registers.
		if inst.Ty.Width() == 16 {
			if sym, ok := g.globalU16StoreSym[inst.Src[0]]; ok {
				value := g.loc(inst.Src[1])
				if value == "HL" {
					g.emitNativeMemoryf(inst, 0, 2, "    LD (%s), HL    ; u16 direct store (16T, no A clobber)", sanitizeIdent(sym))
					break
				}
				if value == "DE" || value == "BC" {
					g.emitNativeMemoryf(inst, 0, 2, "    LD (%s), %s    ; u16 direct store, no scratch clobber", sanitizeIdent(sym), value)
					break
				}
			}
		}
		ptr := g.loc(inst.Src[0])
		val := g.loc(inst.Src[1])
		w := inst.Ty.Width()
		wordValue := isPairReg(val)
		if isSpill(val) {
			if info, ok := g.regInfo[inst.Src[1]]; ok && info.Ty != nil {
				wordValue = info.Ty.Width() >= 16
			}
		}
		// If ptr is spilled to a memory slot ($Fxxx), reload it into HL first.
		if isSpill(ptr) {
			// Loading the address into HL must not destroy the store value.
			// DE temporarily owns any H/L value; restore its previous contents
			// after the access even when no virtual register keeps DE live.
			if val == "HL" || val == "H" || val == "L" {
				g.emit("    PUSH DE")
				g.emit("    PUSH HL")
				g.emit("    POP DE")
				if val == "HL" {
					val = "DE"
				} else if val == "H" {
					val = "D"
				} else {
					val = "E"
				}
				defer func() { g.emit("    POP DE"); g.invalidate("DE") }()
			}
			g.emitNativeSpillf(ptr, 0, 2, false, "    LD HL, (%s)   ; reload spilled ptr", ptr)
			g.invalidate("HL")
			ptr = "HL"
		}
		if pat, ok := g.bitStorePat[inst.Src[1]]; ok {
			if g.emitBitMemOp(pat.op, pat.bit, ptr, pat.byteOffset) {
				break
			}
		}
		if w == 24 {
			// 24-bit store: write 3 bytes little-endian.
			if isIXY(ptr) {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s     ; byte0 lo_lo", ptrIndirect(ptr, 0), lowByte(val))
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, %s     ; byte1 lo_hi", ptrIndirect(ptr, 1), highByte(val))
				g.emit("    EXX")
				g.emitNativeMemoryf(inst, 2, 1, "    LD %s, %s     ; byte2 hi8 (shadow lo)", ptrIndirect(ptr, 2), lowByte(val))
				g.emit("    EXX")
			} else {
				g.emitf("    LD (%s), %s     ; byte0 lo_lo", ptr, lowByte(val))
				g.emitf("    INC %s", ptr)
				g.emitf("    LD (%s), %s     ; byte1 lo_hi", ptr, highByte(val))
				g.emitf("    INC %s", ptr)
				g.emit("    EXX")
				g.emitf("    LD (%s), %s     ; byte2 hi8 (shadow lo)", ptr, lowByte(val))
				g.emit("    EXX")
				g.emitf("    DEC %s", ptr)
				g.emitf("    DEC %s", ptr)
			}
		} else if w == 32 {
			// 32-bit store: write 4 bytes little-endian.
			if isIXY(ptr) {
				// Via IX/IY displacement: 4 × LD (IX+d),r = 4×19T = 76T.
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s     ; byte0 lo_lo", ptrIndirect(ptr, 0), lowByte(val))
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, %s     ; byte1 lo_hi", ptrIndirect(ptr, 1), highByte(val))
				g.emit("    EXX")
				g.emitNativeMemoryf(inst, 2, 1, "    LD %s, %s     ; byte2 hi_lo (shadow)", ptrIndirect(ptr, 2), lowByte(val))
				g.emitNativeMemoryf(inst, 3, 1, "    LD %s, %s     ; byte3 hi_hi (shadow)", ptrIndirect(ptr, 3), highByte(val))
				g.emit("    EXX")
			} else {
				// Via ptr INC trick (HL/DE/BC): ptr is advanced then restored.
				g.emitf("    LD (%s), %s     ; byte0 lo_lo", ptr, lowByte(val))
				g.emitf("    INC %s", ptr)
				g.emitf("    LD (%s), %s     ; byte1 lo_hi", ptr, highByte(val))
				g.emitf("    INC %s", ptr)
				g.emit("    EXX")
				g.emitf("    LD (%s), %s     ; byte2 hi_lo (shadow)", ptr, lowByte(val))
				g.emitf("    INC %s", ptr)
				g.emitf("    LD (%s), %s     ; byte3 hi_hi (shadow)", ptr, highByte(val))
				g.emit("    EXX")
				g.emitf("    DEC %s", ptr)
				g.emitf("    DEC %s", ptr)
				g.emitf("    DEC %s", ptr)
			}
		} else if w <= 8 {
			if cv, ok := g.constVals[inst.Src[1]]; ok {
				// LD (HL),n = 10T/2B  or  LD (IX+d),n = 19T/4B
				// Only HL and IX/IY support immediate indirect stores.
				// BC/DE: LD (DE),n is NOT valid — must go through A.
				if ptr == "HL" || isIXY(ptr) {
					g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %d", ptrIndirect(ptr, 0), cv)
				} else {
					g.emitf("    LD A, %d", cv)
					g.emitNativeMemoryf(inst, 0, 1, "    LD (%s), A", ptr)
					g.invalidate("A")
				}
			} else {
				// LD (BC),r and LD (DE),r: only r=A is valid.
				// LD (HL),IXH/IXL: DD prefix conflict — route through A.
				if (ptr == "BC" || ptr == "DE") && val != "A" {
					g.emitLDA(val)
					g.emitNativeMemoryf(inst, 0, 1, "    LD (%s), A", ptr)
				} else if ptr == "HL" && isPairReg(val) && !isIXY(val) {
					// Width mismatch: 8-bit store but val is pair. Use low byte.
					// Excludes IX/IY: lowByte("IX")="IXL" can't appear in LD (HL),r
					// (DD prefix conflict).
					g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), %s", lowByte(val))
				} else if ptr == "HL" && (isIXYReg(val) || isIXY(val) || isSpill(val)) {
					g.emit("    EX AF, AF'")
					if isSpill(val) {
						g.emitNativeSpillf(val, 0, 1, false, "    LD A, (%s)", val)
					} else {
						g.emitLDA(val)
					}
					g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), A")
					g.emit("    EX AF, AF'")
				} else if isIXY(ptr) && (val == "H" || val == "L" || isIXYReg(val)) {
					// BUG-008: LD (IX+d),H encodes as LD (IX+d),IXH (prefix substitution).
					// LD (IX+d),IXL impossible. Route through A.
					g.emitLDA(val)
					g.invalidate("A")
					g.emitNativeMemoryf(inst, 0, 1, "    LD %s, A", ptrIndirect(ptr, 0))
				} else if isPairReg(val) {
					// Width mismatch: 8-bit store but val is pair. Use low byte.
					lo := lowByte(val)
					if isIXY(ptr) && (lo == "H" || lo == "L" || isIXYReg(lo)) {
						// DD/FD prefix conflict: LD (IX+d),IXL or LD (IX+d),H impossible.
						g.emitLDA(lo)
						g.invalidate("A")
						g.emitNativeMemoryf(inst, 0, 1, "    LD %s, A", ptrIndirect(ptr, 0))
					} else if ptr == "HL" && isIXYReg(lo) {
						// DD prefix conflict: LD (HL),IXL impossible.
						g.emitLDA(lo)
						g.invalidate("A")
						g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), A")
					} else {
						g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s", ptrIndirect(ptr, 0), lo)
					}
				} else if isSpill(val) {
					g.emit("    EX AF, AF'")
					g.emitNativeSpillf(val, 0, 1, false, "    LD A, (%s)", val)
					g.emitNativeMemoryf(inst, 0, 1, "    LD %s, A", ptrIndirect(ptr, 0))
					g.emit("    EX AF, AF'")
				} else {
					g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s", ptrIndirect(ptr, 0), val)
				}
			}
		} else if isIXY(ptr) {
			// 16-bit store via IX/IY: use displacement addressing.
			lo := lowByte(val)
			hi := highByte(val)
			needA := lo == "H" || lo == "L" || isIXYReg(lo) || isSpill(lo)
			if needA {
				g.emitLDA(lo)
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, A     ; lo", ptrIndirect(ptr, 0))
			} else {
				g.emitNativeMemoryf(inst, 0, 1, "    LD %s, %s     ; lo", ptrIndirect(ptr, 0), lo)
			}
			if !wordValue {
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, 0     ; hi (zero-extend)", ptrIndirect(ptr, 1))
			} else if hi == "H" || hi == "L" || isIXYReg(hi) || isSpill(hi) {
				g.emitLDA(hi)
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, A     ; hi", ptrIndirect(ptr, 1))
				g.invalidate("A")
			} else {
				g.emitNativeMemoryf(inst, 1, 1, "    LD %s, %s     ; hi", ptrIndirect(ptr, 1), hi)
			}
		} else if ptr == "HL" {
			// LD (HL), r — valid for A,B,C,D,E,H,L but NOT IXH/IXL/IYH/IYL
			// (DD prefix substitutes (HL)→(IX+d), corrupting the encoding).
			lo := lowByte(val)
			hi := highByte(val)
			if isIXYReg(lo) || isSpill(lo) {
				g.emit("    EX AF, AF'")
				if isSpill(lo) {
					g.emitNativeSpillf(lo, 0, 1, false, "    LD A, (%s)", lo)
				} else {
					g.emitLDA(lo)
				}
				g.emitNativeMemory(inst, 0, 1, "    LD (HL), A     ; lo")
				g.emit("    EX AF, AF'")
			} else {
				g.emitNativeMemoryf(inst, 0, 1, "    LD (HL), %s     ; lo", lo)
			}
			g.emitf("    INC %s", ptr)
			if !wordValue {
				g.emitNativeMemoryf(inst, 1, 1, "    LD (%s), 0     ; hi (zero-extend u8→u16)", ptr)
			} else if isIXYReg(hi) || isSpill(hi) {
				g.emit("    EX AF, AF'")
				if isSpill(hi) {
					g.emitNativeSpillf(hi, 0, 1, false, "    LD A, (%s)", hi)
				} else {
					g.emitLDA(hi)
				}
				g.emitNativeMemory(inst, 1, 1, "    LD (HL), A     ; hi")
				g.emit("    EX AF, AF'")
			} else {
				g.emitNativeMemoryf(inst, 1, 1, "    LD (HL), %s     ; hi", hi)
			}
			g.emitf("    DEC %s", ptr)
		} else {
			// DE/BC: only LD (DE),A / LD (BC),A are valid — route both bytes through A.
			lo := lowByte(val)
			if lo != "A" {
				g.emitLDA(lo)
				g.invalidate("A")
			}
			g.emitNativeMemoryf(inst, 0, 1, "    LD (%s), A     ; lo", ptr)
			g.emitf("    INC %s", ptr)
			if !wordValue {
				g.emitf("    XOR A          ; hi = 0 (zero-extend u8→u16)")
				g.invalidate("A")
			} else {
				hi := highByte(val)
				if hi != "A" {
					g.emitLDA(hi)
					g.invalidate("A")
				}
			}
			g.emitNativeMemoryf(inst, 1, 1, "    LD (%s), A     ; hi", ptr)
			g.emitf("    DEC %s", ptr)
		}
	case OpCall, OpCallIndirect:
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		g.comment(fmt.Sprintf("genCall: %s dst=%v", inst.Sym, inst.Dst))
		if err := g.genCall(inst); err != nil && g.err == nil {
			g.err = err
		}

	case OpCallCond:
		// Conditional CALL: the condition flag is already set by a preceding CP/AND.
		// inst.Cond carries the CmpCond from the OpCmp that produced the condition.
		// Only applies to void calls (Dst == NoReg) where args are already in place.
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		// Resolve the producer's actual flags: comparison lowering may swap
		// operands, use AND A for zero, or require two flags for > / <=.
		// Hand-written raw MIR can request an explicit current-flags call.
		cc := cmpCondCode(inst.Cond)
		if inst.Src[0] != NoReg {
			cc = g.condCode(g.fn, inst.Src[0])
			original := CmpNe // a materialized boolean is tested for nonzero
			for _, block := range g.fn.Blocks {
				for _, producer := range block.Insts {
					if producer.Op == OpCmp && producer.Dst == inst.Src[0] {
						original = producer.Cond
					}
				}
			}
			if inst.Cond == invertCmpCond(original) {
				switch cc {
				case "CGT":
					cc = "CLE"
				case "CLE":
					cc = "CGT"
				default:
					cc = invertCC(cc)
				}
			} else if inst.Cond != original {
				if g.err == nil {
					g.err = fmt.Errorf("conditional call %s has unrelated predicate", inst.Sym)
				}
				return
			}
		}
		g.comment(fmt.Sprintf("genCallCond: CALL %s, %s", cc, inst.Sym))
		// Use the regular call lowering for intrinsics, fixed addresses, and
		// sanitized symbols. Intrinsics have no callable assembly label.
		skip := fmt.Sprintf(".%s_callcond%d", sanitizeIdent(g.fn.Name), g.trampIdx)
		g.trampIdx++
		switch cc {
		case "CGT":
			g.emitf("    JRS Z, %s", skip)
			g.emitf("    JRS C, %s", skip)
		case "CLE":
			take := skip + "_take"
			g.emitf("    JRS C, %s", take)
			g.emitf("    JRS NZ, %s", skip)
			g.emitf("%s:", take)
		default:
			g.emitf("    JRS %s, %s", invertCC(cc), skip)
		}
		if err := g.genCall(inst); err != nil && g.err == nil {
			g.err = err
		}
		g.emitf("%s:", skip)
		clear(g.holdsPhys) // calls clobber all volatile registers

	case OpIn8:
		// IN A, (port) — 8-bit port read
		port := inst.Imm & 0xFF
		g.emitf("    IN A, (0x%02X)", port)
		if dst != "A" {
			g.emitf("    LD %s, A", dst)
		}

	case OpOut8:
		// OUT (port), A — 8-bit port write
		port := inst.Imm & 0xFF
		src := g.loc(inst.Src[0])
		if src != "A" {
			g.emitf("    LD A, %s", src)
		}
		g.emitf("    OUT (0x%02X), A", port)

	case OpIn16:
		// 16-bit port address, 8-bit data: IN r,(C) with BC = port
		// Z80: LD BC, port / IN A,(C) — reads from full 16-bit address bus
		port := inst.Imm & 0xFFFF
		g.emitf("    LD BC, 0x%04X", port)
		g.emitf("    IN A, (C)")
		if dst != "A" {
			g.emitf("    LD %s, A", dst)
		}

	case OpOut16:
		// 16-bit port address, 8-bit data: OUT (C),r with BC = port
		port := inst.Imm & 0xFFFF
		src := g.loc(inst.Src[0])
		if src != "A" {
			g.emitf("    LD A, %s", src)
		}
		g.emitf("    LD BC, 0x%04X", port)
		g.emitf("    OUT (C), A")

	case OpAddrOf:
		sym := sanitizeIdent(inst.Sym)
		if isPairReg(dst) {
			g.emitf("    LD %s, %s", dst, sym)
		} else if isSpill(dst) {
			// A spilled address still occupies a complete word. Preserve HL,
			// which may hold an unrelated live pointer or a later call argument.
			g.emit("    PUSH HL")
			g.emitf("    LD HL, %s", sym)
			g.emitNativeSpillf(dst, 0, 2, true, "    LD (%s), HL", dst)
			g.emit("    POP HL")
		} else {
			// addr_of produces a 16-bit address but dst is 8-bit —
			// allocator spilled the pointer to an 8-bit reg.
			// Route through HL: LD HL, sym; LD dst, L (low byte only).
			// This is lossy but at least doesn't produce invalid asm.
			// TODO: proper fix is to ensure addr_of always gets a pair.
			g.emitf("    LD HL, %s", sym)
			g.emitLD8(dst, "L")
			g.invalidate("H")
			g.invalidate("L")
		}

	case OpPtrAdd:
		// Runtime pointer advance: dst = base_ptr + offset_u16.
		// Z80 ONLY has ADD HL, rr — there is no ADD DE,rr or ADD BC,rr.
		// Strategy: get base into HL, offset into DE (or BC), emit ADD HL,rr,
		// then move result from HL to dst if dst != HL.
		//
		// When the base is in HL and is live after this instruction (loop body
		// pattern: ptr survives to the next iteration), scanPtrAddBase sets
		// g.ptrAddPushHL = inst.Dst.  We emit PUSH HL before the ADD to save
		// the base; the subsequent OpLoad emits POP HL to restore it.
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		base := g.loc(inst.Src[0])
		off := g.loc(inst.Src[1])
		// ADD uses HL as scratch even when the base/result live elsewhere.
		// Preserve any unrelated live-through HL value (e.g. a loop offset).
		preserveHL := false
		if dst != "HL" && g.ptrAddPushHL != inst.Dst {
			for r := range g.regsLiveAfterInst(inst) {
				if r != inst.Dst && g.loc(r) == "HL" {
					preserveHL = true
					break
				}
			}
		}
		if preserveHL {
			g.emit("    PUSH HL")
		}

		// PUSH HL to preserve the base ptr before the ADD clobbers it.
		if g.ptrAddPushHL == inst.Dst {
			g.emit("    PUSH HL") // base is in HL; save it for POP HL after load
		}

		// Determine the offset pair: must be a 16-bit pair ≠ HL.
		// Choose DE; fall back to BC if DE is the base (moving base to HL would
		// clobber DE, so the offset must already be safe there) or same as dst.
		offPair := "DE"
		if base == "DE" {
			offPair = "BC"
		}

		preserveOffsetPair := false
		if off != offPair {
			for r := range g.regsLiveAfterInst(inst) {
				loc := g.loc(r)
				if r != inst.Dst && (loc == offPair || loc == lowByte(offPair) || loc == highByte(offPair)) {
					preserveOffsetPair = true
					break
				}
			}
		}
		if preserveOffsetPair {
			g.emitf("    PUSH %s", offPair)
		}

		// Place offset into offPair BEFORE moving the base (avoids clobbering
		// the offset when base and off share a register like DE).
		if off != offPair {
			if info, ok := g.regInfo[inst.Src[1]]; ok && z80ValueWidth(info.Ty) <= 8 {
				// Optimized ptr_add may consume a byte index directly. Its
				// spill home is one byte, so never reload an adjacent byte.
				g.genUnsignedExtBytes(offPair, off, 8, 16)
			} else if len(off) == 1 { // 8-bit: zero-extend into pair
				g.emitLD8(lowByte(offPair), off)
				g.emitf("    LD %s, 0", highByte(offPair))
			} else {
				g.emitMov(offPair, off, 16)
			}
		}

		// Get base into HL.
		if base != "HL" {
			g.emitMov("HL", base, 16)
		}

		// ADD HL, offPair — only BC/DE/HL/SP valid.
		if offPair == "IX" || offPair == "IY" {
			g.emit("    PUSH DE")
			g.emitMov("DE", offPair, 16)
			g.emit("    ADD HL, DE")
			g.emit("    POP DE")
		} else {
			g.emitADDHL(offPair)
		}

		// When the base was PUSHed to save it, keep the result in HL and
		// override the dst's physical location so the subsequent load uses
		// LD r,(HL) directly — avoiding the LD D,H;LD E,L byte-copy that
		// would force the load through A (LD A,(DE)) and clobber the accumulator.
		if g.ptrAddPushHL == inst.Dst {
			if dst != "HL" {
				g.physOverride[inst.Dst] = "HL"
			}
		} else {
			// Move result from HL to dst if needed.
			if dst != "HL" {
				g.emitMov(dst, "HL", 16)
				g.invalidate("HL")
			}
		}
		g.invalidate(dst)
		if preserveOffsetPair {
			g.emitf("    POP %s", offPair)
			g.invalidate(offPair)
		}
		if preserveHL {
			g.emit("    POP HL")
		}

	case OpField, OpPtrBump:
		// Advance pointer by compile-time byte offset.
		// OpField: struct field access; OpPtrBump: iterator advance.
		// SoA256 special strides: -1 → INC L (element advance), 256 → INC H (field switch).
		// INC and ADD clobber flags.
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		if info, ok := g.globalFieldAddr[inst.Dst]; ok {
			g.emitf("    LD %s, %s", dst, globalFieldLabel(info.sym, info.offset, ""))
			break
		}
		src := g.loc(inst.Src[0])
		offset := inst.Imm
		if dst != src {
			g.emitMov(dst, src, 16) // copy base ptr
		}
		switch {
		case offset == 0:
			// nothing — dst already at target
		case offset == -1:
			// SoA256: INC L — advance element index (H=column, L=index)
			g.emitf("    INC %s", lowByte(dst))
		case offset == 256:
			// SoA256: INC H — switch to next field column (H=column, L=index)
			g.emitf("    INC %s", highByte(dst))
		case offset <= 3 && !isSpill(dst):
			for range offset {
				g.emitf("    INC %s", dst)
			}
		default:
			// ADD HL,rr needs two scratch pairs even for a constant field
			// offset. Neither the unrelated HL base nor a live BC/DE value
			// may be destroyed merely because this result lives elsewhere.
			tmp := "BC"
			if dst == "BC" {
				tmp = "DE"
			}
			preserveHL, preserveTmp := false, false
			for r := range g.regsLiveAfterInst(inst) {
				if r == inst.Dst {
					continue
				}
				loc := g.loc(r)
				if dst != "HL" && (loc == "HL" || loc == "H" || loc == "L") {
					preserveHL = true
				}
				if loc == tmp || loc == lowByte(tmp) || loc == highByte(tmp) {
					preserveTmp = true
				}
			}
			if preserveHL {
				g.emit("    PUSH HL")
			}
			if preserveTmp {
				g.emitf("    PUSH %s", tmp)
			}
			if dst != "HL" {
				g.emitMov("HL", dst, 16)
			}
			g.emitf("    LD %s, %d", tmp, offset)
			g.emitADDHL(tmp)
			if dst != "HL" {
				g.emitMov(dst, "HL", 16)
				g.invalidate("HL")
			}
			if preserveTmp {
				g.emitf("    POP %s", tmp)
				g.invalidate(tmp)
			}
			if preserveHL {
				g.emit("    POP HL")
				g.invalidate("HL")
			}

		}
		g.invalidate(dst)

	case OpAlloca:
		// The object owns n bytes; temporary preservation lives below it.
		// ADD HL,SP changes carry, so preserve AF even when only flags are live.
		for range inst.Imm {
			g.emit("    DEC SP")
		}
		g.emit("    PUSH AF")
		bias := 2
		if dst != "HL" {
			g.emit("    PUSH HL")
			bias += 2
		}
		g.emitf("    LD HL, %d", bias)
		g.emit("    ADD HL, SP")
		if dst != "HL" {
			g.emitMov(dst, "HL", 16)
			g.emit("    POP HL")
			g.invalidate("HL")
		}
		g.emit("    POP AF")
		g.invalidate(dst)

	case OpPatchSlot:
		// Mutable immediate: emit initial value; the byte(s) after LD are patchable.
		w := inst.Ty.Width()
		var restore func()
		if inst.Sym != "" && isSpill(dst) {
			restore = g.saveLiveScratch(inst, "HL")
			dst = "HL"
		}
		if w <= 8 {
			g.emitf("    LD %s, %d          ; [patch_slot init=%d]", dst, inst.Imm, inst.Imm)
		} else {
			// 16-bit @smc slot (LD HL, imm16): emit EQU pointing at the imm16 bytes.
			g.emitf("    LD %s, %d          ; [patch_slot init=%d]", dst, inst.Imm, inst.Imm)
		}
		// Named slot (@smc parameter): record EQU label and register patcher.
		if inst.Sym != "" {
			eqLabel := SMCImmediateLabel(inst.Sym)
			g.sb.WriteString(eqLabel + "  EQU $-" + fmt.Sprintf("%d", w/8) + "\n")
			g.smcPatchers = append(g.smcPatchers, smcPatcherInfo{sym: inst.Sym, eqLabel: eqLabel})
		}
		if restore != nil {
			slotLoc := g.loc(inst.Dst)
			if g.spillSlotWidth(slotLoc) == 1 {
				// Byte-typed slot: a word store would overwrite the
				// neighbouring home. Keep the write inside the slot.
				g.emit("    EX AF, AF'")
				g.storeSpill8(slotLoc, "L")
				g.emit("    EX AF, AF'")
			} else {
				g.emitf("    LD (%s), HL", slotLoc)
			}
			restore()
		}

	case OpLoadPatched:
		// After patching, the slot register already holds the new value.
		// Emit a move; the codegen may elide this if dst == slot.
		slot := g.loc(inst.Src[0])
		if dst != slot {
			g.emitMov(dst, slot, inst.Ty.Width())
		}

	case OpPatch:
		if inst.Sym != "" {
			value := g.loc(inst.Src[1])
			restore := g.saveLiveScratch(inst, "HL")
			if value != "HL" {
				g.emitMov("HL", value, 16)
			}
			g.emitf("    LD (%s), HL", SMCImmediateLabel(inst.Sym))
			restore()
			break
		}
		// SMC patch: at runtime the code rewrites the immediate in OpPatchSlot.
		// In the pure-assembler path we emit a self-modifying write sequence.
		// slot = inst.Src[0], newVal = inst.Src[1]
		slotReg := g.loc(inst.Src[0])
		newVal := g.loc(inst.Src[1])
		g.comment(fmt.Sprintf("patch %s ← %s  (SMC: rewrite immediate in stream)", slotReg, newVal))
		// The patch_slot instruction is always LD r, n (2 bytes).
		// At runtime: caller computes address of the immediate byte and writes newVal.
		// We emit a placeholder comment; actual SMC requires knowing the patch address.
		// The Z80 backend will resolve this in a second pass.
		g.emitf("    ; TODO: LD (patch_addr_%s), %s", slotReg, newVal)

	case OpPush:
		src := g.loc(inst.Src[0])
		// PUSH requires a 16-bit pair; pad single registers to their pair.
		pair := toPair(src)
		g.emitf("    PUSH %s", pair)
		g.invalidate(src)

	case OpPop:
		pair := toPair(dst)
		g.emitf("    POP %s", pair)
		g.invalidate(pair)

	case OpAsm:
		// Inline asm may emit any instruction — conservatively clobber flags.
		g.lastFlagsLhs = ""
		g.lastFlagsRhs = ""
		if inst.Asm != nil {
			if g.runtime == Z80RuntimeCPM {
				for _, line := range strings.Split(inst.Asm.Template, "\n") {
					g.emit("    " + strings.TrimSpace(line))
				}
			} else {
				g.emitf("    %s", inst.Asm.Template)
			}
		}

	default:
		g.comment(fmt.Sprintf("TODO: %s", inst.Op))
	}
}
