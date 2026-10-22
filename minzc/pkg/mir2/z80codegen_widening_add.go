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
	dst, wide, src := g.loc(add.Dst), g.loc(ext.Dst), g.loc(ext.Src[0])
	if (dst != "BC" && dst != "DE" && dst != "HL") || g.loc(acc) != dst || z80spec.Overlaps(src, dst) {
		return false
	}
	switch src {
	case "A", "B", "C", "D", "E", "H", "L":
	default:
		return false
	}
	costlyWide := wide == "IX" || wide == "IY" || isSpill(wide)
	// ADD HL,rr with a primary zero-extended pair is already smaller/faster.
	if dst == "HL" && !costlyWide {
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
	g.emitf("    ADD A, %s", lowByte(dst))
	g.emitLD8(lowByte(dst), "A")
	done := fmt.Sprintf(".%s_byte_add_%d", sanitizeIdent(g.fn.Name), add.Dst)
	g.emitf("    JR NC, %s", done)
	g.emitf("    INC %s", highByte(dst))
	g.emitf("%s:", done)
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
