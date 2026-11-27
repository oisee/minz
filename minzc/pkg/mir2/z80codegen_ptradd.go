package mir2

import (
	"fmt"

	"github.com/minz/minzc/pkg/z80spec"
)

// Address-arithmetic forms for ptr_add that avoid the generic "base -> HL,
// offset -> DE/BC" funnel, which has to PUSH/POP HL and the offset pair
// whenever either holds a live value (≈ 97 T for an index kept in HL).
//
//   - Offset in HL, base in BC/DE, HL free afterwards: ADD HL, base.  The
//     addition is commutative.
//   - Constant base (addr_of a global), HL free afterwards, offset in BC/DE:
//     LD HL, sym / ADD HL, off; offset in HL: LD rr, sym / ADD HL, rr with a
//     pair rr that holds no live value.
//   - Sum into DE, one operand dying in DE, the other in BC or a busy HL:
//     EX DE, HL / ADD HL, rr / EX DE, HL.  HL is the same before and after.
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
//
// An addr_of whose every use is a ptr_add base is a dead address constant: it
// emits nothing (and may hold no home, LocRemat), and every ptr_add path,
// including the generic one, names the symbol as an immediate.  `add i, k`
// directly before such a ptr_add, with k in 1..255 and the sum used only
// there, is folded into the immediate as sym+k.

// ptrAddImmediateBase returns the symbol expression a ptr_add must use as an
// immediate base and the vreg holding its offset.  The symbol is non-empty when
// the base is a dead address constant (it has no materialised home); a folded
// `add i, k` turns the expression into sym+k and the offset into i.
func (g *z80cg) ptrAddImmediateBase(inst *Inst) (string, Reg) {
	off := inst.Src[1]
	if !g.deadConsts[inst.Src[0]] {
		return "", off
	}
	sym := g.ptrAddBaseSymbol(inst.Src[0])
	if fold, ok := g.ptrAddFolds[inst]; ok && sym != "" {
		sym, off = fmt.Sprintf("%s+%d", sym, fold.imm), fold.index
	}
	return sym, off
}

// instReads returns the vregs whose homes the code emitted for inst reads:
// inst.Uses(), plus the index of a folded add when inst is its ptr_add.  That
// index is not an operand of the ptr_add and may be dead after the add, so
// scratch selection and relocation cleanup at the ptr_add must keep it.
func (g *z80cg) instReads(inst *Inst) []Reg {
	uses := inst.Uses()
	if fold, ok := g.ptrAddFolds[inst]; ok {
		uses = append(uses[:len(uses):len(uses)], fold.index)
	}
	return uses
}

// instWrites returns the vregs whose homes the code emitted for inst writes:
// the destination, a call's extra results (a fallible call's failed bit and
// error code) and inline-asm outputs. Every question "does inst write this
// register" asked by eviction of relocated values and by scratch selection
// goes through here, so an extra result is never treated as free space.
func instWrites(inst *Inst) []Reg {
	var out []Reg
	if inst.Dst != NoReg {
		out = append(out, inst.Dst)
	}
	for _, r := range inst.ExtraRets {
		if r != NoReg {
			out = append(out, r)
		}
	}
	if inst.Asm != nil {
		for _, r := range inst.Asm.Outs {
			if r != NoReg {
				out = append(out, r)
			}
		}
	}
	return out
}

// functionConstants maps every OpConst destination of f to its value.
func functionConstants(f *Func) map[Reg]int64 {
	out := map[Reg]int64{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpConst {
				out[inst.Dst] = inst.Imm
			}
		}
	}
	return out
}

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
	sym, offReg := g.ptrAddImmediateBase(inst)
	dst, base, off := g.loc(inst.Dst), g.loc(inst.Src[0]), g.loc(offReg)
	if !fusedExtWordPair(dst) || inst.Src[0] == offReg {
		return false
	}
	if _, c := g.constVals[offReg]; c {
		return false
	}
	if sym != "" {
		base = "" // no materialised home: only the immediate is valid
	} else if !fusedExtWordPair(base) {
		if sym = g.ptrAddBaseSymbol(inst.Src[0]); sym == "" {
			return false
		}
	}
	byteOff := false
	if info, ok := g.regInfo[offReg]; ok && z80ValueWidth(info.Ty) <= 8 {
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

	// Constant base, offset in HL, HL free: LD rr, sym / ADD HL, rr with a
	// pair holding no live value (it may be the destination itself: ADD has
	// consumed it before the result is copied out).
	if sym != "" && !byteOff && off == "HL" && hlFree {
		if pair := g.ptrAddFreePair(inst); pair != "" {
			g.comment("ptr_add: constant base, offset in HL")
			g.emitf("    LD %s, %s", pair, sym)
			g.emit("    ADD HL, " + pair)
			g.invalidate(pair)
			g.finishPtrAddHL(dst)
			return true
		}
	}

	// Sum into DE with one operand dying in DE and the other in BC or HL:
	// EX DE, HL / ADD HL, rr / EX DE, HL forms it in place (3 bytes, 19 T) and
	// hands HL back intact, whatever it holds.  A dead HL operand with HL free
	// stays with the generic ADD HL, DE.
	if sym == "" && !byteOff && dst == "DE" && (base == "DE" || off == "DE") {
		other, inDE := off, inst.Src[0]
		if off == "DE" {
			other, inDE = base, offReg
		}
		if (other == "BC" || other == "HL" && !hlFree) && !g.regsLiveAfterInst(inst)[inDE] {
			g.comment("ptr_add: into DE through EX DE, HL")
			g.emit("    EX DE, HL")
			if other == "HL" {
				other = "DE" // the swap moved it
			}
			g.emit("    ADD HL, " + other)
			g.emit("    EX DE, HL")
			g.invalidate("HL")
			g.invalidate("DE")
			return true
		}
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

// ptrAddFreePair returns DE or BC when no value other than inst's result
// occupies it (or a half of it) after inst, and no scratch relocation uses it.
func (g *z80cg) ptrAddFreePair(inst *Inst) string {
	live := g.regsLiveAfterInst(inst)
	for _, pair := range []string{"DE", "BC"} {
		free := true
		for r := range live {
			if r != inst.Dst && !g.deadConsts[r] && z80spec.Overlaps(pair, g.loc(r)) {
				free = false
			}
		}
		for _, loc := range g.physOverride {
			if z80spec.Overlaps(pair, loc) {
				free = false
			}
		}
		if free {
			return pair
		}
	}
	return ""
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

// deadAddressConstants returns the addr_of vregs of f that no instruction
// reads out of a home: a single definition, not a block parameter, and every
// use is the base operand of a ptr_add (never its offset).  The ptr_add
// lowering then names the symbol as an immediate, so the address needs neither
// its LD nor, once rematerialised, a register pair.
func deadAddressConstants(f *Func) map[Reg]bool {
	defs := map[Reg]int{}
	addr := map[Reg]bool{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst] += 2 // never a single definition
		}
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				defs[inst.Dst]++
			}
			if inst.Op == OpAddrOf && inst.Dst != NoReg && inst.Sym != "" {
				addr[inst.Dst] = true
			}
			for _, r := range inst.ExtraRets {
				defs[r]++
			}
		}
	}
	if len(addr) == 0 {
		return nil
	}
	total, base := map[Reg]int{}, map[Reg]int{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for _, r := range inst.Uses() {
				if addr[r] {
					total[r]++
				}
			}
			if inst.Op == OpPtrAdd && addr[inst.Src[0]] && inst.Src[1] != inst.Src[0] {
				base[inst.Src[0]]++
			}
		}
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				if addr[r] {
					total[r]++
				}
			}
		}
	}
	var dead map[Reg]bool
	for r := range addr {
		if defs[r] != 1 || total[r] == 0 || total[r] != base[r] {
			continue
		}
		if dead == nil {
			dead = map[Reg]bool{}
		}
		dead[r] = true
	}
	return dead
}

// ptrAddFold describes `t = add i, k` immediately followed by
// `p = ptr_add @sym, t`, t's only use: the ptr_add is emitted as
// (sym+k) + i and the add emits nothing.
type ptrAddFold struct {
	add   *Inst
	index Reg   // i, read by the ptr_add in place of t
	k     Reg   // the constant operand
	imm   int64 // its value, 1..255
}

// ptrAddConstantFolds finds the folds above for f.  constVal maps OpConst
// vregs to their values.  The add must be the instruction right before the
// ptr_add, so i's location is still intact when the ptr_add reads it (nothing
// is emitted in between); its flags must be unobserved, the base a dead
// address constant (deadAddressConstants), and i a non-constant word.
func ptrAddConstantFolds(f *Func, constVal map[Reg]int64) map[*Inst]ptrAddFold {
	deadAddr := deadAddressConstants(f)
	if len(deadAddr) == 0 {
		return nil
	}
	info := collectRegInfo(f)
	var uses map[Reg]int
	constDefs := map[Reg]int{}
	blockParam := map[Reg]bool{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			blockParam[p.Dst] = true
		}
		for _, inst := range b.Insts {
			if inst.Op == OpConst {
				constDefs[inst.Dst]++
			}
		}
	}
	var folds map[*Inst]ptrAddFold
	for _, b := range f.Blocks {
		for idx := 1; idx < len(b.Insts); idx++ {
			p, add := b.Insts[idx], b.Insts[idx-1]
			if p.Op != OpPtrAdd || !deadAddr[p.Src[0]] || add.Op != OpAdd || add.Dst != p.Src[1] ||
				add.ClsHard || add.Ty == nil || add.Ty.Width() != 16 || add.Src[0] == add.Src[1] {
				continue
			}
			// A block parameter merged with a constant's vreg is not constant.
			isConst := func(r Reg) bool {
				_, c := constVal[r]
				return c && constDefs[r] == 1 && !blockParam[r]
			}
			i, k := add.Src[0], add.Src[1]
			if isConst(i) {
				i, k = k, i
			}
			if !isConst(k) || isConst(i) || i == p.Src[0] {
				continue
			}
			kv := constVal[k]
			if kv < 1 || kv > 255 {
				continue
			}
			if ri, ok := info[i]; !ok || ri.Ty == nil || ri.Ty.Width() != 16 {
				continue
			}
			if uses == nil {
				uses = countRegUses(f)
			}
			if uses[add.Dst] != 1 || !wordMaskFlagsUnobserved(f, add.Dst) {
				continue
			}
			if folds == nil {
				folds = map[*Inst]ptrAddFold{}
			}
			folds[p] = ptrAddFold{add: add, index: i, k: k, imm: kv}
		}
	}
	return folds
}
