package mir2

import "fmt"

// bitAccum is one fused "count += v & 1; v >>= 1" step:
//
//	%e = ext %v : u8 -> u16      (optional)
//	%m = and %e, 1               (or Nanz's %m = ext(and %v, 1))
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
			// Nanz masks the byte before widening: ext(and(v, 1)). Unwrap
			// that ext the same way as the C form's and(ext(v), 1).
			var widen *Inst
			if and != nil && and.Op == OpExt && uses[m] == 1 && and.SrcTy != nil && and.SrcTy.Width() == 8 {
				widen, m = and, and.Src[0]
				and = def(m)
			}
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
				(pl != "BC" && pl != "DE" && pl != "HL" && pl != "IX" && pl != "IY") ||
				parentPair(vl) == pl {
				continue
			}
			g.bitAccumAt[add] = bitAccum{v: v, acc: acc, shr: shr}
			g.bitAccumSkip[and] = true
			g.bitAccumSkip[shr] = true
			if ext != nil {
				g.bitAccumSkip[ext] = true
			}
			if widen != nil {
				g.bitAccumSkip[widen] = true
			}
			break
		}
	}
}

// genBitAccum emits the fused step; it reports false when the add is not one.
//
// The SRL produces the shift's result at the add, but the MIR still defines
// it at the (suppressed) shr. Until then every codegen structure — liveness,
// pendingAccReg, physOverride — knows the value only under the name of the
// shifted operand v, which stays live up to the shr. So v is the carrier:
// relocating v moves the shifted byte, and finishBitAccumShift hands that
// byte over to the shr's destination at the shr's position.
func (g *z80cg) genBitAccum(inst *Inst) bool {
	p, ok := g.bitAccumAt[inst]
	if !ok {
		return false
	}
	// The scan matched allocated homes. A value relocated since then (a
	// saved accumulator, a dynamic spill) returns home first: SRL/INC need
	// registers, and the add's result is expected in its own home.
	g.materializePendingFlag(inst)
	v, pair := g.ar.Loc(p.shr.Dst).Name, g.ar.Loc(inst.Dst).Name
	if at := g.loc(p.v); at != v {
		g.emitLD8(v, at)
		delete(g.physOverride, p.v)
		g.invalidate(v)
	}
	if at := g.loc(p.acc); at != pair {
		g.emitMov(pair, at, 16)
		delete(g.physOverride, p.acc)
		g.invalidate(pair)
	}
	idx := g.trampIdx
	g.trampIdx++
	skip := fmt.Sprintf(".bit_accum_%d", idx)
	g.emitf("    SRL %s", v)
	g.emitf("    JRS NC, %s", skip)
	g.emitf("    INC %s", pair)
	g.emitf("%s:", skip)
	g.invalidate(v)
	g.invalidate(pair)
	return true
}

// finishBitAccumShift runs at a shr suppressed by a fused step: the shifted
// byte, carried so far under the operand's name, becomes the shr's result in
// the shr's allocated home (where an unfused shr would have written it).
func (g *z80cg) finishBitAccumShift(shr *Inst) {
	v, w := shr.Src[0], shr.Dst
	home := g.ar.Loc(w).Name
	if at := g.loc(v); at != home {
		g.emitLD8(home, at)
		g.comment(fmt.Sprintf("r%d (shifted r%d) back to its home", w, v))
		g.invalidate(home)
	}
	delete(g.physOverride, v)
	delete(g.physOverride, w)
	if g.pendingAccReg == v {
		g.pendingAccReg = NoReg
		if home == "A" {
			g.pendingAccReg = w
		}
	}
}
