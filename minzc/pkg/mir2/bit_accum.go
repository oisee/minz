package mir2

import "fmt"

// bitAccum is one fused "count += v & 1; v >>= 1" step:
//
//	%e = ext %v : u8 -> u16      (optional)
//	%m = and %e, 1
//	%s = add %acc, %m : u16
//	%w = shr %v, 1 : u8
//
// emitted at the add as SRL v / JR NC, skip / INC acc — the shifted-out bit
// is exactly v & 1. The ext/and/shr are suppressed; v and w must share a
// byte register and acc and s a register pair.
type bitAccum struct {
	v, acc Reg
	shr    *Inst
}

func (g *z80cg) scanBitAccumPatterns(b *Block) {
	clear(g.bitAccumAt)
	clear(g.bitAccumSkip)
	if g.fn == nil || len(b.Insts) < 3 {
		return
	}
	for _, i := range b.Insts {
		if i.Op == OpCmp {
			return // keep pending comparison flags out of the picture
		}
	}
	// Registers are reused across blocks (block parameters), so count reads in
	// this block only and require the value to be dead on exit.
	uses := make(map[Reg]int)
	for _, i := range b.Insts {
		for _, r := range i.Uses() {
			uses[r]++
		}
	}
	if b.Term != nil {
		for _, r := range termRegUses(b.Term) {
			uses[r] += 100
		}
	}
	if g.liveness != nil {
		if out := g.liveness.LiveOutOf(g.fn, b); out != nil {
			for _, r := range out.Slice() {
				uses[r] += 100
			}
		}
	}
	def := func(r Reg) *Inst {
		for _, i := range b.Insts {
			if i.Dst == r {
				return i
			}
		}
		return nil
	}
	isOne := func(r Reg) bool {
		d := def(r)
		return d != nil && d.Op == OpConst && d.Imm == 1
	}
	byteReg := func(s string) bool {
		switch s {
		case "A", "B", "C", "D", "E", "H", "L":
			return true
		}
		return false
	}
	for ai, add := range b.Insts {
		if add.Op != OpAdd || add.Ty == nil || add.Ty.Width() != 16 || len(add.Src) != 2 || add.Dst == NoReg {
			continue
		}
		for k := 0; k < 2; k++ {
			acc, m := add.Src[k], add.Src[1-k]
			and := def(m)
			if and == nil || and.Op != OpAnd || uses[m] != 1 || len(and.Src) != 2 {
				continue
			}
			src, one := and.Src[0], and.Src[1]
			if !isOne(one) {
				src, one = and.Src[1], and.Src[0]
				if !isOne(one) {
					continue
				}
			}
			v, ext := src, def(src)
			if ext != nil && ext.Op == OpExt {
				if uses[src] != 1 || ext.SrcTy == nil || ext.SrcTy.Width() != 8 {
					continue
				}
				v = ext.Src[0]
			} else {
				ext = nil
			}
			if info, ok := g.regInfo[v]; !ok || info.Ty == nil || info.Ty.Width() != 8 {
				continue
			}
			// The single shift of v must come after the add, and nothing
			// else may read v: the fused SRL replaces v early.
			var shr *Inst
			for _, i := range b.Insts[ai+1:] {
				if i.Op == OpShr && len(i.Src) == 2 && i.Src[0] == v && isOne(i.Src[1]) && i.Ty != nil && i.Ty.Width() == 8 {
					shr = i
					break
				}
			}
			want := 2 // ext (or and) + shr
			if shr == nil || uses[v] != want {
				continue
			}
			vl, pl := g.loc(v), g.loc(acc)
			if !byteReg(vl) || g.loc(shr.Dst) != vl || g.loc(add.Dst) != pl ||
				!(pl == "BC" || pl == "DE" || pl == "HL" || pl == "IX" || pl == "IY") ||
				parentPair(vl) == pl {
				continue
			}
			g.bitAccumAt[add] = bitAccum{v: v, acc: acc, shr: shr}
			g.bitAccumSkip[and] = true
			g.bitAccumSkip[shr] = true
			if ext != nil {
				g.bitAccumSkip[ext] = true
			}
			break
		}
	}
}

// genBitAccum emits the fused step; it reports false when the add is not one.
func (g *z80cg) genBitAccum(inst *Inst) bool {
	p, ok := g.bitAccumAt[inst]
	if !ok {
		return false
	}
	v, pair := g.loc(p.v), g.loc(p.acc)
	idx := g.trampIdx
	g.trampIdx++
	skip := fmt.Sprintf(".bit_accum_%d", idx)
	g.materializePendingFlag(inst)
	g.emitf("    SRL %s", v)
	g.emitf("    JRS NC, %s", skip)
	g.emitf("    INC %s", pair)
	g.emitf("%s:", skip)
	g.invalidate(v)
	g.invalidate(pair)
	return true
}
