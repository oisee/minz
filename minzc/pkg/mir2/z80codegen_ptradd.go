package mir2

// Address-arithmetic forms for ptr_add that avoid the generic "base -> HL,
// offset -> DE/BC" funnel, which has to PUSH/POP HL and the offset pair
// whenever either holds a live value (≈ 97 T for an index kept in HL).
//
//   - Offset in HL, base in BC/DE, HL free afterwards: ADD HL, base.  The
//     addition is commutative.
//   - Constant base (addr_of a global), HL free afterwards, offset in BC/DE:
//     LD HL, sym / ADD HL, off.
//   - Otherwise, when HL is busy or an index register is involved, the sum is
//     formed byte-wise through A, writing the destination pair directly:
//
//     word offset:  LD A, lo(off) / ADD A, lo(base) / LD lo(dst), A /
//     LD A, hi(off) / ADC A, hi(base) / LD hi(dst), A
//     byte offset:  LD A, off / ADD A, lo(base) / LD lo(dst), A /
//     ADC A, hi(base) / SUB lo(dst) / LD hi(dst), A
//
//     lo(base)/hi(base) are register halves or the assembler's sym^L / sym^H
//     immediates.  The carry of the low ADD feeds ADC, so the result is the
//     exact 16-bit modular sum.  The byte form adds the carry to hi(base)
//     through A (lo(dst) + hi(base) + C - lo(dst)).  Every operand byte is read
//     before the only destination byte that could alias it: a low half is never
//     the high half of another pair, so writing lo(dst) cannot disturb hi(off)
//     or hi(base).  A and F are written; a live accumulator is moved to a free
//     scratch byte first, or the form is not used.

// ptrAddBaseSymbol returns the global symbol r was taken the address of when
// r has exactly one definition, an OpAddrOf, and is not a block parameter.
func (g *z80cg) ptrAddBaseSymbol(r Reg) string {
	if g.blockParamRegs[r] {
		return ""
	}
	var def *Inst
	for _, b := range g.fn.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst == r {
				if def != nil {
					return ""
				}
				def = inst
			}
		}
	}
	if def == nil || def.Op != OpAddrOf {
		return ""
	}
	return sanitizeIdent(def.Sym)
}

// ptrAddHLFree reports whether no value other than inst's result occupies HL
// or one of its halves after inst.
func (g *z80cg) ptrAddHLFree(inst *Inst) bool {
	for r := range g.regsLiveAfterInst(inst) {
		if r == inst.Dst || g.deadConsts[r] {
			continue
		}
		if loc := g.loc(r); loc == "HL" || loc == "H" || loc == "L" {
			return false
		}
	}
	return true
}

// tryPtrAddForms emits one of the forms above and reports whether it did.
func (g *z80cg) tryPtrAddForms(inst *Inst) bool {
	if g.ptrAddPushHL == inst.Dst {
		return false // the following load consumes the result from HL
	}
	dst, base, off := g.loc(inst.Dst), g.loc(inst.Src[0]), g.loc(inst.Src[1])
	if !fusedExtWordPair(dst) || inst.Src[0] == inst.Src[1] {
		return false
	}
	if _, c := g.constVals[inst.Src[1]]; c {
		return false
	}
	sym := ""
	if !fusedExtWordPair(base) {
		if sym = g.ptrAddBaseSymbol(inst.Src[0]); sym == "" {
			return false
		}
	}
	byteOff := false
	if info, ok := g.regInfo[inst.Src[1]]; ok && z80ValueWidth(info.Ty) <= 8 {
		byteOff = true
		if isPairReg(off) {
			off = lowByte(off)
		}
		if !fusedExtSourceByte(off) {
			return false
		}
	} else if !fusedExtWordPair(off) {
		return false
	}
	hlFree := g.ptrAddHLFree(inst)

	// Commutative ADD HL: offset already in HL, base in BC/DE.
	if !byteOff && off == "HL" && (base == "BC" || base == "DE") && hlFree {
		g.comment("ptr_add: offset in HL, ADD HL, base")
		g.emit("    ADD HL, " + base)
		g.finishPtrAddHL(dst)
		return true
	}
	// Constant base with HL free: LD HL, sym / ADD HL, off.
	if sym != "" && !byteOff && (off == "BC" || off == "DE") && hlFree {
		g.comment("ptr_add: constant base")
		g.emitf("    LD HL, %s", sym)
		g.emit("    ADD HL, " + off)
		g.finishPtrAddHL(dst)
		return true
	}

	// The byte-wise form pays off when the generic path would have to save
	// HL, involves an index register, or has no register for the base.
	involvesIndex := isIXY(dst) || isIXY(base) || isIXY(off) || isIXYReg(off)
	if hlFree && !involvesIndex && sym == "" {
		return false
	}
	if !g.ptrAddFreeA(inst) {
		return false
	}
	lo, hi := lowByte(base), highByte(base)
	if sym != "" {
		lo, hi = sym+"^L", sym+"^H"
	}
	g.comment("ptr_add: byte-wise through A")
	if byteOff {
		g.emitLD8("A", off)
		g.emitf("    ADD A, %s", lo)
		g.emitf("    LD %s, A", lowByte(dst))
		g.emitf("    ADC A, %s", hi)
		g.emitf("    SUB %s", lowByte(dst))
		g.emitf("    LD %s, A", highByte(dst))
	} else {
		g.emitf("    LD A, %s", lowByte(off))
		g.emitf("    ADD A, %s", lo)
		g.emitf("    LD %s, A", lowByte(dst))
		g.emitf("    LD A, %s", highByte(off))
		g.emitf("    ADC A, %s", hi)
		g.emitf("    LD %s, A", highByte(dst))
	}
	g.invalidate("A")
	g.invalidate(dst)
	return true
}

// finishPtrAddHL moves a sum formed in HL to its destination pair.
func (g *z80cg) finishPtrAddHL(dst string) {
	if dst != "HL" {
		g.emitMov(dst, "HL", 16)
		g.invalidate("HL")
	}
	g.invalidate(dst)
}

// ptrAddFreeA makes A available for the byte-wise form: a live accumulator
// (other than the result) is moved to a free scratch byte through
// physOverride.  It reports false, emitting nothing, when no scratch byte is
// free.
func (g *z80cg) ptrAddFreeA(inst *Inst) bool {
	var held []Reg
	for r := range g.regsLiveAfterInst(inst) {
		if r == inst.Dst || g.deadConsts[r] {
			continue
		}
		switch g.loc(r) {
		case "A":
			held = append(held, r)
		case "AF", "F":
			return false
		}
	}
	if len(held) > 1 {
		return false
	}
	if len(held) == 1 {
		scratch := g.freeScratch8(inst)
		if scratch == "" {
			return false
		}
		g.emitf("    LD %s, A    ; preserve live r%d across ptr_add", scratch, held[0])
		g.physOverride[held[0]] = scratch
		if g.pendingAccReg == held[0] {
			g.pendingAccReg = NoReg
		}
	}
	return true
}
