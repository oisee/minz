package mir2

import "reflect"

// HoistDiamondCommon moves the first common total, pure computation from the
// two exclusive successors of a branch to its head. Leading constants may
// differ in SSA identity, but must have identical typed values. No memory,
// trapping operations, hard constraints or successor parameters are moved.
func HoistDiamondCommon(f *Func) bool {
	if f == nil || len(f.Blocks) == 0 {
		return false
	}
	dom := reachableDominators(f)
	defs := map[Reg]*Block{}
	counts := map[Reg]int{}
	define := func(r Reg, b *Block) {
		if r != NoReg {
			counts[r]++
			defs[r] = b
		}
	}
	for _, p := range f.Contract.Params {
		define(p.Reg, nil)
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			define(p.Dst, b)
		}
		for _, i := range b.Insts {
			define(i.Dst, b)
			for _, r := range i.ExtraRets {
				define(r, b)
			}
			if i.Asm != nil {
				for _, r := range i.Asm.Outs {
					define(r, b)
				}
			}
		}
	}
	available := func(r Reg, head *Block) bool {
		return counts[r] == 1 && (defs[r] == nil || dom[head.Label][defs[r].Label])
	}
	// Only integer value operations whose effects are wholly represented by Dst.
	pure := func(i *Inst) bool {
		if i.Dst == NoReg || counts[i.Dst] != 1 || i.ClsHard || i.Volatile || i.Ty == nil {
			return false
		}
		switch i.Op {
		case OpMove, OpAdd, OpSub, OpMul, OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar, OpNeg, OpNot, OpExt, OpSext, OpTrunc:
			return true
		}
		return false
	}
	prefix := func(b *Block) (int, map[Reg]*Inst) {
		cs := map[Reg]*Inst{}
		n := 0
		for n < len(b.Insts) && b.Insts[n].Op == OpConst {
			i := b.Insts[n]
			if i.ClsHard || i.Volatile || counts[i.Dst] != 1 {
				break
			}
			cs[i.Dst] = i
			n++
		}
		return n, cs
	}
	changed := false
	for _, head := range f.Blocks {
		br, ok := head.Term.(*TermBrIf)
		if !ok || !dom[head.Label][head.Label] || br.Then == br.Else || len(head.Insts) == 0 {
			continue
		}
		cmp := head.Insts[len(head.Insts)-1]
		if cmp.Op != OpCmp || cmp.Dst != br.Cond || cmp.Cond == CmpSubCarry || cmp.Cond == CmpSubCarryNot {
			continue
		}
		a, b := f.BlockByLabel(br.Then), f.BlockByLabel(br.Else)
		if a == nil || b == nil || a == head || b == head || a == f.Entry() || b == f.Entry() || len(a.Params) != 0 || len(b.Params) != 0 || countPredecessors(f, a.Label) != 1 || countPredecessors(f, b.Label) != 1 {
			continue
		}
		ai, ac := prefix(a)
		bi, bc := prefix(b)
		if ai == len(a.Insts) || bi == len(b.Insts) {
			continue
		}
		x, y := a.Insts[ai], b.Insts[bi]
		if !pure(x) || !pure(y) {
			continue
		}
		// Compare all semantic/constraint metadata, excluding operand identity and
		// source locations. Keep the first arm's source location on the shared op.
		xx, yy := *x, *y
		xx.Dst, yy.Dst = NoReg, NoReg
		xx.Src, yy.Src = [2]Reg{}, [2]Reg{}
		xx.SrcFile, yy.SrcFile = "", ""
		xx.SrcLine, yy.SrcLine = 0, 0
		if !reflect.DeepEqual(xx, yy) {
			continue
		}
		needed := map[Reg]bool{}
		legal := true
		for k, r := range x.Src {
			s := y.Src[k]
			if r == s && (r == NoReg || available(r, head) && r != cmp.Dst) {
				continue
			}
			c, d := ac[r], bc[s]
			if c == nil || d == nil || c.Ty != d.Ty || c.Imm != d.Imm || c.Cls != d.Cls {
				legal = false
				break
			}
			needed[r] = true
		}
		if !legal {
			continue
		}
		// Keep the branch predicate last: a hoisted flag writer must not
		// overwrite an already computed ClassFlag condition.
		head.Insts = head.Insts[:len(head.Insts)-1]
		kept := a.Insts[:0]
		for n, i := range a.Insts {
			if n == ai {
				continue
			}
			if needed[i.Dst] {
				head.Insts = append(head.Insts, i)
				defs[i.Dst] = head
			} else {
				kept = append(kept, i)
			}
		}
		a.Insts = kept
		head.Insts = append(head.Insts, x, cmp)
		defs[x.Dst] = head
		b.Insts = append(b.Insts[:bi], b.Insts[bi+1:]...)
		for _, block := range f.Blocks {
			rewriteBlockUses(block, y.Dst, x.Dst)
			renameTermReg(block.Term, y.Dst, x.Dst)
		}
		changed = true
	}
	if changed {
		DeadStoreElim(f)
	}
	return changed
}
