package mir2

import "github.com/minz/minzc/pkg/z80spec"

// Single-use zero-extension fusion.
//
// `ext u8 -> u16` whose only reader is the next emitted instruction does not
// need its word: the consumer can read the byte where it already lives and
// treat the missing high byte as the immediate 0.  tryFuseExt decides at the
// extension, emits nothing, and records the byte's location in g.fusedExt; the
// consumer then reads that location instead of the extension's home.
//
// The decision is made at the extension with the consumer's full emission
// conditions, so a consumer never finds a fused operand it cannot serve.  Only
// suppressed constants (which emit nothing and write no home) may stand between
// the two, so the byte's register is still intact when the consumer runs; the
// location is captured at the extension because a scratch relocation of a dead
// source is dropped by the consumer's prologue.
//
// Consumers:
//   - an unsigned ordering compare consumed only by its branch
//     (genFusedExtCmp16);
//   - `shl ext(c), 8`: the byte becomes the high half, the low half is 0;
//   - `or w, ext(b)` where w's low byte is known to be zero: the byte becomes
//     the low half.
// `add acc, ext(b)` is handled by genWideningByteAdd.

// fusedExtSourceByte reports whether loc is a byte register an A-ALU operand or
// an 8-bit LD can name directly.
func fusedExtSourceByte(loc string) bool {
	switch loc {
	case "A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL":
		return true
	}
	return false
}

// fusedExtWordPair reports whether loc is a 16-bit register pair with
// addressable halves.
func fusedExtWordPair(loc string) bool {
	switch loc {
	case "BC", "DE", "HL", "IX", "IY":
		return true
	}
	return false
}

// tryFuseExt suppresses a single-use zero-extension whose consumer reads the
// source byte directly.  It reports whether the extension was absorbed.
func (g *z80cg) tryFuseExt(ext *Inst) bool {
	if ext.Op != OpExt || ext.SrcTy != TyU8 || ext.Ty == nil || ext.Ty.Width() != 16 || ext.ClsHard ||
		g.fn == nil || g.curBlock == nil || ext.Src[0] == g.pendingFlagReg {
		return false
	}
	src := g.loc(ext.Src[0])
	if !fusedExtSourceByte(src) {
		return false
	}
	insts := g.curBlock.Insts
	at := -1
	for i, inst := range insts {
		if inst == ext {
			at = i
			break
		}
	}
	if at < 0 {
		return false
	}
	var use *Inst
	for _, inst := range insts[at+1:] {
		if inst.Op == OpConst && g.deadConsts[inst.Dst] && g.loc(inst.Dst) != "F" ||
			inst.Op == OpAddrOf && g.deadConsts[inst.Dst] {
			continue // emits nothing and writes no home
		}
		use = inst
		break
	}
	if use == nil || countRegUses(g.fn)[ext.Dst] != 1 || g.fusedExtSkipped(use) {
		return false
	}
	ok := false
	switch use.Op {
	case OpCmp:
		ok = g.fusedExtCmpReady(use, ext.Dst, src)
	case OpShl:
		ok = use.Src[0] == ext.Dst && g.fusedExtShlReady(use)
	case OpOr:
		ok = g.fusedExtOrReady(use, ext.Dst)
	}
	if !ok {
		return false
	}
	if g.fusedExt == nil {
		g.fusedExt = map[Reg]string{}
	}
	g.fusedExt[ext.Dst] = src
	g.comment("zero-extension fused into its consumer")
	return true
}

// fusedExtSkipped reports whether another fusion already owns inst's emission.
func (g *z80cg) fusedExtSkipped(inst *Inst) bool {
	return g.bitAccumSkip[inst] || g.widenedByteAdds[inst] || g.lutSkip[inst.Dst] ||
		g.bitStoreSkip[inst.Dst] || g.bitCmpSkip[inst.Dst] || g.globalFieldSkip[inst.Dst] ||
		inst == g.shiftCarryCmp || inst == g.addCarryInst
}

// fusedExtCmpReady reports whether genFusedExtCmp16 can lower cmp with the
// fused extension ext (source byte in src) as one operand.
func (g *z80cg) fusedExtCmpReady(cmp *Inst, ext Reg, src string) bool {
	switch cmp.Cond {
	case CmpUlt, CmpUge, CmpUgt, CmpUle:
	default:
		return false
	}
	if cmp.SrcTy != nil && cmp.SrcTy.Width() != 16 || cmp.Src[0] == cmp.Src[1] {
		return false
	}
	other := cmp.Src[0]
	if other == ext {
		other = cmp.Src[1]
	} else if cmp.Src[1] != ext {
		return false
	}
	if _, _, ok := g.bytewiseCmpOperand(other); !ok {
		return false
	}
	if _, bit := g.bitCmpPat[cmp.Dst]; bit || cmp.Dst == NoReg || !comparisonBranchOnly(g.fn, g.curBlock, cmp) {
		return false
	}
	// The sequence writes A and F.  A source byte in A is consumed first; any
	// other live accumulator or pending predicate keeps the generic path.
	live := g.scratchLiveAfter(cmp)
	for r := range live {
		if r == cmp.Dst || g.deadConsts[r] {
			continue
		}
		if loc := g.loc(r); loc == "A" || loc == "AF" || loc == "F" {
			return false
		}
	}
	if g.pendingAccReg != NoReg && live[g.pendingAccReg] {
		return false
	}
	if g.pendingFlagReg != NoReg && live[g.pendingFlagReg] {
		return false
	}
	return true
}

// fusedExtShlReady accepts `shl ext(c), k` with 8 <= k < 16 into a register
// pair: the result is c<<(k-8) in the high half and 0 in the low half.  Only
// k = 8 is lowered (two LDs, no flags); other counts keep the generic path.
func (g *z80cg) fusedExtShlReady(shl *Inst) bool {
	if shl.Ty == nil || shl.Ty.Width() != 16 || shl.ClsHard {
		return false
	}
	if k, ok := g.wordConstant(shl.Src[1]); !ok || k != 8 {
		return false
	}
	return fusedExtWordPair(g.loc(shl.Dst))
}

// fusedExtOrReady accepts `or w, ext(b)` (either operand order) whose word w
// has a zero low byte by construction (`shl x, k` with 8 <= k < 16): the OR is
// then the byte move lo(dst) = b, hi(dst) = hi(w).  The ALU flags of the OR
// are not produced, so they must be unobserved.
func (g *z80cg) fusedExtOrReady(or *Inst, ext Reg) bool {
	if or.Ty == nil || or.Ty.Width() != 16 || or.ClsHard || or.Src[0] == or.Src[1] {
		return false
	}
	w := or.Src[0]
	if w == ext {
		w = or.Src[1]
	} else if or.Src[1] != ext {
		return false
	}
	if !fusedExtWordPair(g.loc(or.Dst)) || !fusedExtWordPair(g.loc(w)) || !g.lowByteZeroWord(w) {
		return false
	}
	return wordMaskFlagsUnobserved(g.fn, or.Dst)
}

// lowByteZeroWord reports whether w is defined exactly once, by a 16-bit left
// shift by a constant in [8, 16), so its low byte is zero on every path.
func (g *z80cg) lowByteZeroWord(w Reg) bool {
	if g.blockParamRegs[w] {
		return false
	}
	var def *Inst
	for _, b := range g.fn.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst == w {
				if def != nil {
					return false
				}
				def = inst
			}
		}
	}
	if def == nil || def.Op != OpShl || def.Ty == nil || def.Ty.Width() != 16 {
		return false
	}
	k, ok := g.wordConstant(def.Src[1])
	return ok && k >= 8 && k < 16
}

// wordConstant returns r's value when r is a non-parameter vreg defined only by
// OpConst.  Unlike constVals it does not depend on emission order, so it also
// answers for a constant defined between an extension and its consumer.
func (g *z80cg) wordConstant(r Reg) (int64, bool) {
	if v, ok := g.constVals[r]; ok {
		return v, true
	}
	if g.blockParamRegs[r] {
		return 0, false
	}
	var def *Inst
	for _, b := range g.fn.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst == r {
				if def != nil {
					return 0, false
				}
				def = inst
			}
		}
	}
	if def == nil || def.Op != OpConst {
		return 0, false
	}
	return def.Imm, true
}

// genFusedExtShl emits `shl ext(c), 8` as hi(dst) = c, lo(dst) = 0.  The high
// half is written first, so c may itself be either half of dst.
func (g *z80cg) genFusedExtShl(shl *Inst) bool {
	c, ok := g.fusedExt[shl.Src[0]]
	if !ok {
		return false
	}
	dst := g.loc(shl.Dst)
	g.emitLD8(highByte(dst), c)
	g.emitLD8(lowByte(dst), "0")
	g.invalidate(dst)
	return true
}

// genFusedExtOr emits `or w, ext(b)` with w's low byte zero as two byte moves.
// When b is dst's high half, the low half is written first so b is read
// before it is overwritten; hi(w) can never be dst's low half.
func (g *z80cg) genFusedExtOr(or *Inst) bool {
	w, ext := or.Src[0], or.Src[1]
	b, ok := g.fusedExt[ext]
	if !ok {
		w, ext = ext, w
		if b, ok = g.fusedExt[ext]; !ok {
			return false
		}
	}
	dst, wide := g.loc(or.Dst), g.loc(w)
	switch {
	case dst == wide:
		g.emitLD8(lowByte(dst), b)
	case z80spec.Overlaps(b, highByte(dst)):
		g.emitLD8(lowByte(dst), b)
		g.emitLD8(highByte(dst), highByte(wide))
	default:
		g.emitLD8(highByte(dst), highByte(wide))
		g.emitLD8(lowByte(dst), b)
	}
	g.invalidate(dst)
	return true
}

// genFusedExtCmp16 lowers an unsigned ordering between a word x and a fused
// zero-extended byte b through A, reading b where it lives:
//
//	x - b:  LD A, lo(x) / SUB b / LD A, hi(x) / SBC A, 0        carry = x < b
//	b - x:  LD A, b / SUB lo(x) / LD A, 0 / SBC A, hi(x)        carry = b < x
//	b-x-1:  LD A, b / SCF / SBC A, lo(x) / LD A, 0 / SBC A, hi(x)  carry = b <= x
//
// The final borrow of a byte-wise subtraction is exactly the 16-bit unsigned
// borrow.  When b is already in A only the b-first forms are used (A is
// overwritten by the high-byte step, after b has been consumed), with the
// predicate chosen as C or NC on the branch.
func (g *z80cg) genFusedExtCmp16(cmp *Inst) bool {
	lhsB, lhsFused := g.fusedExt[cmp.Src[0]]
	rhsB, rhsFused := g.fusedExt[cmp.Src[1]]
	if !lhsFused && !rhsFused {
		return false
	}
	// Normalise to P(x, b) with x the word operand.
	x, b, cond := cmp.Src[0], rhsB, cmp.Cond
	if lhsFused {
		x, b, cond = cmp.Src[1], lhsB, cmp.Cond.Swap()
	}
	xLo, xHi, _ := g.bytewiseCmpOperand(x)
	cc := "C"
	switch {
	case b != "A" && (cond == CmpUlt || cond == CmpUge):
		// x - b: carry = x < b.
		g.emitf("    LD A, %s", xLo)
		g.emitf("    SUB %s", b)
		g.emitf("    LD A, %s", xHi)
		g.emit("    SBC A, 0")
		if cond == CmpUge {
			cc = "NC"
		}
	case cond == CmpUgt || cond == CmpUle:
		// b - x: carry = b < x = x > b.
		g.emitLD8("A", b)
		g.emitf("    SUB %s", xLo)
		g.emit("    LD A, 0")
		g.emitf("    SBC A, %s", xHi)
		if cond == CmpUle {
			cc = "NC"
		}
	default:
		// b in A, x < b or x >= b: b - x - 1 borrows exactly when b <= x,
		// i.e. when x >= b.
		g.emit("    SCF")
		g.emitf("    SBC A, %s", xLo)
		g.emit("    LD A, 0")
		g.emitf("    SBC A, %s", xHi)
		if cond == CmpUlt {
			cc = "NC"
		}
	}
	g.invalidate("A")
	g.pendingAccReg = NoReg
	g.pendingFlagReg = cmp.Dst
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
	delete(g.cmpSwapped, cmp.Dst)
	delete(g.cmpNeedsTwo, cmp.Dst)
	g.setCmpBranchCC(cmp.Dst, cc)
	return true
}
