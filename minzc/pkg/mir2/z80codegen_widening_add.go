package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80spec"
)

// genWideningByteAdd selects adjacent add16(acc,ext_u8(x)) without ever
// materializing the word temporary. The MIR keeps its ordinary value semantics.
func (g *z80cg) genWideningByteAdd(ext *Inst) bool {
	if ext.Op != OpExt || ext.SrcTy != TyU8 || ext.Ty == nil || ext.Ty.Width() != 16 || ext.ClsHard || g.curBlock == nil || g.pendingFlagReg != NoReg {
		return false
	}
	i := g.curInstIdx + 1
	if i >= len(g.curBlock.Insts) || countRegUses(g.fn)[ext.Dst] != 1 {
		return false
	}
	add := g.curBlock.Insts[i]
	if add.Op != OpAdd || add.Ty != ext.Ty || add.ClsHard || !wordMaskFlagsUnobserved(g.fn, add.Dst) {
		return false
	}
	acc := NoReg
	if add.Src[0] == ext.Dst {
		acc = add.Src[1]
	} else if add.Src[1] == ext.Dst {
		acc = add.Src[0]
	}
	if acc == NoReg {
		return false
	}
	dst, wide, src, accLoc := g.loc(add.Dst), g.loc(ext.Dst), g.loc(ext.Src[0]), g.loc(acc)
	// In place (acc == dst) the carry is propagated with JR NC / INC hi.  Into
	// a different pair, the high byte is hi(acc) + carry computed through A.
	inPlace := accLoc == dst
	if inPlace && (dst != "BC" && dst != "DE" && dst != "HL" || z80spec.Overlaps(src, dst)) {
		return false
	}
	if !inPlace && (!fusedExtWordPair(dst) || !fusedExtWordPair(accLoc)) {
		return false
	}
	switch src {
	case "A", "B", "C", "D", "E", "H", "L":
	case "IXH", "IXL", "IYH", "IYL":
		if inPlace {
			return false
		}
	default:
		return false
	}
	costlyWide := wide == "IX" || wide == "IY" || isSpill(wide)
	// ADD HL,rr with a primary zero-extended pair is already smaller/faster.
	if inPlace && dst == "HL" && !costlyWide {
		return false
	}
	// The same holds into HL from another pair when the extension's own home
	// is HL: LD L, x / LD H, 0 / ADD HL, acc is 4 bytes, 22 T against 6 bytes,
	// 24 T through A.  With x already in A the fused form is 2 T faster.
	if !inPlace && dst == "HL" && wide == "HL" && src != "A" {
		return false
	}
	live := g.regsLiveAfterInst(add)
	preserveA := false
	for r := range live {
		if r != add.Dst && !g.deadConsts[r] {
			if g.loc(r) == "F" {
				return false
			}
			if g.loc(r) == "A" {
				preserveA = true
			}
		}
	}
	// Preserving A adds a stack round trip. It is only profitable when the
	// old extension/add also required an index or memory word temporary.
	if preserveA && !costlyWide {
		return false
	}
	if preserveA {
		g.emit("    PUSH AF")
	}
	g.comment("widening byte add")
	if src != "A" {
		g.emitLD8("A", src)
	}
	if inPlace {
		g.emitf("    ADD A, %s", lowByte(dst))
		g.emitLD8(lowByte(dst), "A")
		done := fmt.Sprintf(".%s_byte_add_%d", sanitizeIdent(g.fn.Name), add.Dst)
		g.emitf("    JR NC, %s", done)
		g.emitf("    INC %s", highByte(dst))
		g.emitf("%s:", done)
	} else {
		// A = x + lo(acc) = lo(dst); ADC A, hi(acc) gives lo(dst) + hi(acc) +
		// carry, and subtracting lo(dst) again leaves hi(acc) + carry mod 256.
		// x is read before dst is written; lo(dst) is never a half of acc's
		// high byte, so hi(acc) is still intact when ADC reads it.
		g.emitf("    ADD A, %s", lowByte(accLoc))
		g.emitf("    LD %s, A", lowByte(dst))
		g.emitf("    ADC A, %s", highByte(accLoc))
		g.emitf("    SUB %s", lowByte(dst))
		g.emitf("    LD %s, A", highByte(dst))
	}
	if preserveA {
		g.emit("    POP AF")
	}
	g.lastFlagsLhs, g.lastFlagsRhs = "", ""
	g.invalidate("A")
	g.invalidate(dst)
	if g.widenedByteAdds == nil {
		g.widenedByteAdds = map[*Inst]bool{}
	}
	g.widenedByteAdds[add] = true
	return true
}
