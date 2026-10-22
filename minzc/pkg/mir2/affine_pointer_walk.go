package mir2

// strengthReduceAffinePointers handles single-body natural loops. The index
// proof is separate from address matching: byte expressions must not wrap
// before widening, whereas word address arithmetic is modular throughout.
// Only pure address DAGs are cloned into the preheader; loads/stores never move.
func strengthReduceAffinePointers(f *Func) int {
	if f == nil || verifyFunc(f) != nil || f.Attrs.HasAsm || f.Attrs.FallibleCY {
		return 0
	}
	defs := map[Reg]*Inst{}
	types := map[Reg]Ty{}
	for _, p := range f.Contract.Params {
		if p.ClassHard {
			return 0
		}
		types[p.Reg] = p.Ty
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			types[p.Dst] = p.Ty
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst] = x
				types[x.Dst] = x.Ty
			}
		}
	}
	changed := 0
	for _, l := range AnalyzeNaturalLoops(f) {
		if l.Opaque || len(l.Blocks) > 2 || countPredecessors(f, l.Header.Label) != 2 {
			continue
		}
		// On a conditional entering edge, keep the guard comparison last.
		// Requiring it to be last already ensures moving it below cloned address
		// arithmetic cannot move its definition past an existing boolean use.
		switch t := l.Preheader.Term.(type) {
		case *TermJmp:
		case *TermBrIf:
			n := len(l.Preheader.Insts)
			if n == 0 || l.Preheader.Insts[n-1].Op != OpCmp || l.Preheader.Insts[n-1].Dst != t.Cond {
				continue
			}
		default:
			continue
		}
		// A recurrence update must execute exactly once on every backedge.
		// Admit a self-loop or a header whose sole continuing edge enters the latch.
		if l.Header != l.Latch {
			if l.Back == nil {
				continue
			}
			target := ""
			switch t := l.Header.Term.(type) {
			case *TermCondRet:
				target = t.Then
			case *TermBrIf:
				if !l.Blocks[t.Else] {
					target = t.Then
				}
			}
			if target != l.Latch.Label || countPredecessors(f, l.Latch.Label) != 1 {
				continue
			}
		}
		for _, iv := range l.Inductions {
			initialTy := types[iv.Initial]
			// Signed/unsigned word edges preserve all bits; byte-to-word edges do not.
			sameBits := initialTy == iv.Param.Ty || initialTy != nil && initialTy.Width() == 16 && iv.Param.Ty.Width() == 16
			if (iv.Step != 1 && iv.Step != -1) || !sameBits {
				continue
			}
			low, high, ok := affinePointerIndexRange(l, iv, defs, types)
			if !ok {
				continue
			}
			type affine struct{ scale, offset, low, high int64 }
			var match func(Reg, int) (affine, bool)
			match = func(r Reg, depth int) (affine, bool) {
				if depth > 16 {
					return affine{}, false
				}
				if r == iv.Param.Dst {
					return affine{1, 0, low, high}, true
				}
				x := defs[r]
				if x == nil || x.ClsHard || x.Ty == nil {
					return affine{}, false
				}
				switch x.Op {
				case OpConst:
					return affine{0, x.Imm, x.Imm, x.Imm}, true
				case OpMove, OpExt:
					a, ok := match(x.Src[0], depth+1)
					if x.Op == OpExt && (x.SrcTy != TyU8 || x.Ty.Width() != 16 || a.low < 0 || a.high > 255) {
						return affine{}, false
					}
					if x.Op == OpMove && (x.Ty.Width() != 16 && (x.Ty != TyU8 || a.low < 0 || a.high > 255)) {
						return affine{}, false
					}
					return a, ok
				case OpAdd, OpSub, OpMul:
					a, ok := match(x.Src[0], depth+1)
					if !ok {
						return affine{}, false
					}
					b, ok := match(x.Src[1], depth+1)
					if !ok {
						return affine{}, false
					}
					var c affine
					switch x.Op {
					case OpAdd:
						c = affine{a.scale + b.scale, a.offset + b.offset, a.low + b.low, a.high + b.high}
					case OpSub:
						c = affine{a.scale - b.scale, a.offset - b.offset, a.low - b.high, a.high - b.low}
					case OpMul:
						if b.scale != 0 || b.low != b.high || b.low < 1 || b.low > 2 {
							return affine{}, false
						}
						c = affine{a.scale * b.low, a.offset * b.low, a.low * b.low, a.high * b.low}
					}
					if x.Ty.Width() == 8 {
						if x.Ty != TyU8 || c.low < 0 || c.high > 255 {
							return affine{}, false
						}
					} else if x.Ty.Width() != 16 {
						return affine{}, false
					}
					return c, true
				}
				return affine{}, false
			}
			// Validate and later clone the exact original address expression. Keeping
			// all casts and widths also preserves initial arithmetic on zero-trip paths.
			var canInit func(Reg, int) bool
			canInit = func(r Reg, depth int) bool {
				if depth > 20 {
					return false
				}
				if r == iv.Param.Dst {
					return true
				}
				if _, ok := l.Invariant[r]; ok {
					return true
				}
				x := defs[r]
				if x == nil || x.ClsHard {
					return false
				}
				switch x.Op {
				case OpAddrOf:
					y := *x
					y.Sym = ""
					return plainCountedPointerInst(&y) && f.mod != nil && invariantGlobalSymbol(f.mod, x.Sym)
				case OpConst:
					return plainAffineConstant(x)
				case OpMove, OpExt, OpAdd, OpSub, OpMul, OpPtrAdd:
					if !plainCountedPointerInstExceptCast(x) {
						return false
					}
					for _, s := range x.Uses() {
						if !canInit(s, depth+1) {
							return false
						}
					}
					return true
				}
				return false
			}
			var candidates []*Inst
			for _, x := range l.Latch.Insts {
				if x.Op != OpPtrAdd || x.Ty != TyPtr || !plainCountedPointerInst(x) {
					continue
				}
				a, ok := match(x.Src[1], 0)
				if !ok || (a.scale != 1 && a.scale != 2) || !canInit(x.Dst, 0) {
					continue
				}
				// The base must be invariant, including a known normal global symbol.
				if _, ok := l.Invariant[x.Src[0]]; !ok {
					b := defs[x.Src[0]]
					if b == nil || b.Op != OpAddrOf || b.ClsHard || f.mod == nil || !invariantGlobalSymbol(f.mod, b.Sym) {
						continue
					}
				}
				candidates = append(candidates, x)
			}
			if len(candidates) == 0 {
				continue
			}
			// Once addressing has its own pointer, a word IV no longer needs
			// HL's memory addressing preference. Its remaining compare/store and
			// unit step can use DE directly; leave the actual pointer preferring HL.
			if iv.Param.Ty.Width() == 16 {
				for i := range l.Header.Params {
					if l.Header.Params[i].Dst == iv.Param.Dst {
						l.Header.Params[i].Class = ClassIndex
					}
				}
				if next := defs[iv.Next]; next != nil && !next.ClsHard {
					next.Cls = ClassIndex
				}
			}
			cache := map[Reg]Reg{iv.Param.Dst: iv.Initial}
			var init func(Reg) Reg
			init = func(r Reg) Reg {
				if v, ok := cache[r]; ok {
					return v
				}
				if v, ok := l.Invariant[r]; ok {
					return v
				}
				x := *defs[r]
				for i, s := range x.Src {
					if s != NoReg {
						x.Src[i] = init(s)
					}
				}
				x.Dst = f.AllocReg()
				cache[r] = x.Dst
				l.Preheader.Insts = append(l.Preheader.Insts, &x)
				return x.Dst
			}
			for _, x := range candidates {
				a, _ := match(x.Src[1], 0)
				initial := init(x.Dst)
				walk, bump := f.AllocReg(), f.AllocReg()
				l.Header.Params = append(l.Header.Params, BlockParam{Dst: walk, Ty: TyPtr, Class: ClassPointer})
				appendAffinePointerEdge(l.Preheader.Term, l.Header.Label, initial)
				// OpPtrBump(-1) is the SoA column sentinel, not subtraction.
				// Use ordinary word subtraction for a descending modular pointer.
				advance := &Inst{Op: OpPtrBump, Dst: bump, Src: [2]Reg{walk}, Imm: a.scale, Ty: TyPtr, Cls: ClassPointer, SrcFile: x.SrcFile, SrcLine: x.SrcLine}
				if iv.Step < 0 {
					stride := f.AllocReg()
					l.Latch.Insts = append(l.Latch.Insts, &Inst{Op: OpConst, Dst: stride, Imm: a.scale, Ty: TyPtr, Cls: ClassPointer})
					advance.Op = OpSub
					advance.Imm = 0
					advance.Src[1] = stride
				}
				l.Latch.Insts = append(l.Latch.Insts, advance)
				appendAffinePointerEdge(l.Latch.Term, l.Header.Label, bump)
				x.Op = OpMove
				x.Src = [2]Reg{walk}
				changed++
			}
			// Keep a preheader guard after the cloned arithmetic. Besides reducing
			// predicate live ranges, this avoids relying on flags across address ALU.
			if t, ok := l.Preheader.Term.(*TermBrIf); ok {
				for i, x := range l.Preheader.Insts {
					if x.Dst == t.Cond && x.Op == OpCmp {
						l.Preheader.Insts = append(append(l.Preheader.Insts[:i:i], l.Preheader.Insts[i+1:]...), x)
						break
					}
				}
			}
			// One induction per loop is enough; analysis edge positions are now stale.
			break
		}
	}
	if changed > 0 {
		PropagateCopies(f)
		DeadStoreElim(f)
		DeadBlockArgElim(f)
	}
	return changed
}

func plainAffineConstant(x *Inst) bool {
	y := *x
	y.Imm = 0
	return plainCountedPointerInst(&y)
}
func plainCountedPointerInstExceptCast(x *Inst) bool {
	y := *x
	y.SrcTy = nil
	return plainCountedPointerInst(&y)
}
func appendAffinePointerEdge(t Term, target string, r Reg) {
	switch t := t.(type) {
	case *TermJmp:
		t.Args = append(t.Args, r)
	case *TermBrIf:
		if t.Then == target {
			t.ThenArgs = append(t.ThenArgs, r)
		} else {
			t.ElseArgs = append(t.ElseArgs, r)
		}
	case *TermCondRet:
		t.ThenArgs = append(t.ThenArgs, r)
	case *TermDJNZ:
		t.BodyArgs = append(t.BodyArgs, r)
	}
}

// Executed index bounds, not just bounds on the entering SSA value. In
// particular, DJNZ decrements zero to 255: reject an unguarded zero entry.
func affinePointerIndexRange(l *NaturalLoop, iv LoopInduction, defs map[Reg]*Inst, types map[Reg]Ty) (int64, int64, bool) {
	constant := func(r Reg, n int64) bool {
		x := defs[r]
		return x != nil && x.Op == OpConst && !x.ClsHard && x.Imm == n
	}
	if t, ok := l.Latch.Term.(*TermDJNZ); ok && l.Header == l.Latch && t.Counter == iv.Param.Dst && iv.Param.Ty == TyU8 {
		if x := defs[iv.Initial]; x != nil && x.Op == OpConst && x.Imm >= 1 && x.Imm <= 255 {
			return 1, x.Imm, true
		}
		if t, ok := l.Preheader.Term.(*TermBrIf); ok && t.Then == l.Header.Label {
			c := defs[t.Cond]
			if c != nil && c.Op == OpCmp && c.Cond == CmpNe && c.Src[0] == iv.Initial && constant(c.Src[1], 0) {
				return 1, 255, true
			}
		}
		return 0, 0, false
	}
	if l.Header == l.Latch {
		return 0, 0, false
	}
	var cond Reg
	switch t := l.Header.Term.(type) {
	case *TermCondRet:
		cond = t.Cond
	case *TermBrIf:
		cond = t.Cond
	}
	c := defs[cond]
	if c == nil || c.Op != OpCmp || c.Src[0] != iv.Param.Dst || c.SrcTy != nil && c.SrcTy.Width() != iv.Param.Ty.Width() {
		return 0, 0, false
	}
	if iv.Step == -1 && c.Cond == CmpUgt && (iv.Param.Ty == TyU8 || iv.Param.Ty == TyU16) && constant(c.Src[1], 0) {
		high := int64(65535)
		if iv.Param.Ty == TyU8 {
			high = 255
		}
		return 1, high, true
	}
	if iv.Step == -1 && iv.Param.Ty == TyI16 && c.Cond == CmpGe && constant(c.Src[1], 0) {
		return 0, 32767, true
	}
	if iv.Step == 1 && ((c.Cond == CmpUlt && (iv.Param.Ty == TyU8 || iv.Param.Ty == TyU16)) || (c.Cond == CmpLt && iv.Param.Ty == TyI16)) && (types[c.Src[1]] == iv.Param.Ty || c.SrcTy == iv.Param.Ty) {
		x := defs[iv.Initial]
		if x == nil || x.Op != OpConst || x.ClsHard || x.Imm < 0 {
			return 0, 0, false
		}
		max := int64(65534)
		if iv.Param.Ty == TyI16 {
			max = 32766
		}
		if iv.Param.Ty == TyU8 {
			max = 254
		}
		if x.Imm > max {
			return 0, 0, false
		}
		return x.Imm, max, true
	}
	return 0, 0, false
}
