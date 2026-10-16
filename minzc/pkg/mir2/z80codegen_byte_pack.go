package mir2

// genDisjointByteOr only packs already-computed words. It never moves or
// removes byte accesses, extends operand lifetime, or relies on memory effects.
func (g *z80cg) genDisjointByteOr(inst *Inst) bool {
	if inst.Op != OpOr || inst.Ty != TyU16 || g.curBlock == nil || g.pendingFlagReg != NoReg || bytePackImplicitFlagUse(g.fn, inst.Dst) {
		return false
	}
	defs := map[Reg]*Inst{}
	repeated := map[Reg]bool{}
	for _, i := range g.curBlock.Insts {
		if i == inst {
			break
		}
		if i.Dst != NoReg {
			if _, exists := defs[i.Dst]; exists {
				repeated[i.Dst] = true
			}
			defs[i.Dst] = i
		}
	}
	lowWord := func(r Reg) bool {
		i := defs[r]
		return !repeated[r] && i != nil && i.Op == OpExt && i.SrcTy == TyU8 && i.Ty == TyU16
	}
	highWord := func(r Reg) bool {
		i := defs[r]
		if repeated[r] || i == nil || i.Op != OpShl || i.Ty != TyU16 || !lowWord(i.Src[0]) {
			return false
		}
		k := defs[i.Src[1]]
		return !repeated[i.Src[1]] && k != nil && k.Op == OpConst && k.Imm == 8
	}
	hi, lo := inst.Src[0], inst.Src[1]
	if !highWord(hi) || !lowWord(lo) {
		hi, lo = lo, hi
		if !highWord(hi) || !lowWord(lo) {
			return false
		}
	}
	dst, high, low := g.loc(inst.Dst), g.loc(hi), g.loc(lo)
	primary := func(s string) bool { return s == "HL" || s == "DE" || s == "BC" }
	if !primary(dst) || high != dst || !primary(low) || low == dst {
		return false
	}
	// Do not preserve a predicate whose MIR definition is this OR by accidentally
	// retaining earlier flags. Any live flag boundary takes the ordinary emitter.
	for r := range g.regsLiveAfterInst(inst) {
		if g.loc(r) == "F" || r != inst.Dst && (g.loc(r) == dst || parentPair(g.loc(r)) == dst) {
			return false
		}
	}
	g.emitf("    LD %s, %s    ; pack disjoint byte lanes", lowByte(dst), lowByte(low))
	g.invalidate(dst)
	return true
}

// CondRet/BrIf can consume the NZ flags of an integer definition without any
// vreg allocated to F. Follow flag-preserving copies and block argument edges,
// so this gate is based on MIR control flow rather than physical storage alone.
func bytePackImplicitFlagUse(f *Func, result Reg) bool {
	forwarded := map[Reg]bool{result: true}
	changed := true
	for changed {
		changed = false
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Op == OpMove && forwarded[i.Src[0]] && !forwarded[i.Dst] {
					forwarded[i.Dst] = true
					changed = true
				}
			}
			if b.Term != nil {
				b.Term.ForEachEdge(func(label string, args []Reg, _ int) {
					dest := f.BlockByLabel(label)
					if dest == nil {
						return
					}
					for j, a := range args {
						if j < len(dest.Params) && forwarded[a] && !forwarded[dest.Params[j].Dst] {
							forwarded[dest.Params[j].Dst] = true
							changed = true
						}
					}
				})
			}
		}
	}
	for _, b := range f.Blocks {
		switch t := b.Term.(type) {
		case *TermBrIf:
			if forwarded[t.Cond] {
				return true
			}
		case *TermCondRet:
			if forwarded[t.Cond] {
				return true
			}
		case *TermRet:
			for i, r := range t.Vals {
				if i < len(f.Contract.Returns) && f.Contract.Returns[i].Class == ClassFlag && forwarded[r] {
					return true
				}
			}
		}
	}
	return false
}

// Word mask lane identities may change incidental ALU flags. Admit only
// value-based consumers, excluding physical flags/assembly and error ABIs.
func wordMaskFlagsUnobserved(f *Func, result Reg) bool {
	if f == nil || f.Attrs.HasAsm || f.Attrs.FallibleCY || bytePackImplicitFlagUse(f, result) {
		return false
	}
	for _, p := range f.Contract.Params {
		if p.Class == ClassFlag {
			return false
		}
	}
	for _, r := range f.Contract.Returns {
		if r.Class == ClassFlag {
			return false
		}
	}
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.Op == OpAsm || x.Op == OpCallCond || x.FallibleCY || x.Op == OpCmp && (x.Cond == CmpSubCarry || x.Cond == CmpSubCarryNot) {
				return false
			}
		}
	}
	return true
}
