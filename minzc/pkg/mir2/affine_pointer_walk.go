package mir2

// strengthReduceAffinePointers handles innermost natural loops with one latch.
// The body below the header test may branch and may leave the loop early. The
// index proof is separate from address matching: byte expressions must not
// wrap before widening, whereas word address arithmetic is modular throughout.
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
		if l.Opaque || countPredecessors(f, l.Header.Label) != 2 {
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
		// Admit a self-loop, or a loop whose only backedge is the latch's
		// unconditional jump: the advance is appended to the latch, so it runs
		// once per completed iteration and never on an exit path. The analysis
		// already refuses a second backedge (a `continue` that re-enters the
		// header from another block).
		//
		// The header must end in the exit test with exactly one successor in the
		// loop. Every other loop block is then reached only through the
		// continuing edge, where the test held for this iteration's index; the
		// index range proof below depends on that. Header instructions run
		// before the test and are not candidates.
		//
		// The body must be acyclic below the header (no inner loop): the index
		// is fixed for one iteration, but an inner loop would make "once per
		// iteration" unprovable from the CFG shape alone.
		body := []*Block{l.Latch}
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
			if target == "" || target == l.Header.Label || !l.Blocks[target] || !affineLoopBodyAcyclic(f, l, target) {
				continue
			}
			body = body[:0]
			for _, b := range f.Blocks {
				if l.Blocks[b.Label] && b != l.Header {
					body = append(body, b)
				}
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
			// Candidates with the same base value and the same affine index
			// (scale, offset) compute the same address in every iteration, so
			// they share one walk. The match admits only non-wrapping byte steps
			// and modular word arithmetic, so equal (scale, offset) means equal
			// 16-bit values. The base is keyed by its invariant root or symbol.
			type walkKey struct {
				base          Reg
				sym           string
				scale, offset int64
			}
			var candidates []*Inst
			var walkOf []int
			walks := map[walkKey]int{}
			for _, x := range affineBodyInsts(body) {
				if x.Op != OpPtrAdd || x.Ty != TyPtr || !plainCountedPointerInst(x) {
					continue
				}
				a, ok := match(x.Src[1], 0)
				if !ok || (a.scale != 1 && a.scale != 2) || !canInit(x.Dst, 0) {
					continue
				}
				// The base must be invariant, including a known normal global symbol.
				key := walkKey{scale: a.scale, offset: a.offset}
				if root, ok := l.Invariant[x.Src[0]]; ok {
					key.base = root
				} else {
					b := defs[x.Src[0]]
					if b == nil || b.Op != OpAddrOf || b.ClsHard || f.mod == nil || !invariantGlobalSymbol(f.mod, b.Sym) {
						continue
					}
					key.sym = b.Sym
				}
				w, ok := walks[key]
				if !ok {
					w = len(walks)
					walks[key] = w
				}
				candidates = append(candidates, x)
				walkOf = append(walkOf, w)
			}
			if len(candidates) == 0 {
				continue
			}
			replaced := map[Reg]bool{}
			for _, x := range candidates {
				replaced[x.Dst] = true
			}
			// The down-counter is part of this rewrite: DisableAffine (and the
			// IndexedPointers variant) keeps the indexed shape for both.
			counter := affineCounterPlan(f, l, iv, replaced, defs, types)
			if !affinePointerProfitable(f, l, iv, candidates, len(walks), counter, defs, types) {
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
			walkRegs := make([]Reg, len(walks))
			for n, x := range candidates {
				if walk := walkRegs[walkOf[n]]; walk != NoReg {
					x.Op = OpMove
					x.Src = [2]Reg{walk}
					changed++
					continue
				}
				a, _ := match(x.Src[1], 0)
				initial := init(x.Dst)
				walk, bump := f.AllocReg(), f.AllocReg()
				walkRegs[walkOf[n]] = walk
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
			if counter != nil {
				counter.apply(f, l)
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

// affineNeeded models address-DAG DCE over the loop without modifying the
// function. A replacement pointer is a leaf: its old base/index operands no
// longer stay live. Invariant header parameters forwarded unchanged by the
// backedge are not uses. With a counter plan, the header exit test and the
// index's backedge argument are not roots either: both are replaced.
func affineNeeded(f *Func, l *NaturalLoop, replaced map[Reg]bool, defs map[Reg]*Inst, counter *affineCounter) map[Reg]bool {
	needed := map[Reg]bool{}
	var need func(Reg)
	need = func(r Reg) {
		if r == NoReg || needed[r] {
			return
		}
		needed[r] = true
		if replaced[r] {
			return
		}
		if x := defs[r]; x != nil {
			for _, s := range x.Uses() {
				need(s)
			}
		}
	}
	for _, b := range f.Blocks {
		if !l.Blocks[b.Label] {
			continue
		}
		for _, x := range b.Insts {
			if !isDSEPure(x.Op) {
				for _, r := range x.Uses() {
					need(r)
				}
			}
		}
		if jump, ok := b.Term.(*TermJmp); ok && jump.Target == l.Header.Label {
			for i, r := range jump.Args {
				if _, invariant := l.Invariant[l.Header.Params[i].Dst]; invariant {
					continue
				}
				if counter != nil && l.Header.Params[i].Dst == counter.iv.Param.Dst {
					continue
				}
				need(r)
			}
		} else {
			for _, r := range termRegUses(b.Term) {
				if counter != nil && b == l.Header && r == counter.cmp.Dst {
					continue
				}
				need(r)
			}
		}
	}
	return needed
}

// affineWordTemporary reports a needed 16-bit loop value that must live in a
// register pair of its own: not a replaced address, not a constant or symbol
// address (rematerialised), and not loop-carried state itself, which already
// has its pair. A state value is one passed on an edge; an `ext` of a byte
// whose only uses are adds/subs forming such state is folded into the add.
func affineWordTemporary(f *Func, l *NaturalLoop, replaced, needed map[Reg]bool) bool {
	edge := map[Reg]bool{}
	users := map[Reg][]*Inst{}
	for _, b := range f.Blocks {
		if !l.Blocks[b.Label] {
			continue
		}
		b.Term.ForEachEdge(func(_ string, args []Reg, _ int) {
			for _, r := range args {
				edge[r] = true
			}
		})
		for _, x := range b.Insts {
			for _, r := range x.Uses() {
				users[r] = append(users[r], x)
			}
		}
	}
	for _, b := range f.Blocks {
		if !l.Blocks[b.Label] {
			continue
		}
		for _, x := range b.Insts {
			if x.Dst == NoReg || !needed[x.Dst] || replaced[x.Dst] || x.Ty == nil || x.Ty.Width() != 16 || edge[x.Dst] {
				continue
			}
			switch x.Op {
			case OpConst, OpAddrOf:
				continue
			case OpExt:
				folded := x.SrcTy != nil && x.SrcTy.Width() == 8 && len(users[x.Dst]) > 0
				for _, u := range users[x.Dst] {
					folded = folded && (u.Op == OpAdd || u.Op == OpSub) && edge[u.Dst]
				}
				if folded {
					continue
				}
			}
			return true
		}
	}
	return false
}

func affineBodyInsts(body []*Block) []*Inst {
	var out []*Inst
	for _, b := range body {
		out = append(out, b.Insts...)
	}
	return out
}

// affineLoopBodyAcyclic reports whether the loop blocks reachable from the
// header's continuing successor, ignoring edges back to the header, form a
// DAG. Each block then executes at most once per iteration.
func affineLoopBodyAcyclic(f *Func, l *NaturalLoop, start string) bool {
	blocks := map[string]*Block{}
	for _, b := range f.Blocks {
		blocks[b.Label] = b
	}
	const (
		active = 1
		done   = 2
	)
	state := map[string]int{}
	var visit func(string) bool
	visit = func(label string) bool {
		if label == l.Header.Label || !l.Blocks[label] {
			return true
		}
		switch state[label] {
		case active:
			return false
		case done:
			return true
		}
		state[label] = active
		b := blocks[label]
		if b == nil {
			return false
		}
		for _, s := range b.Successors() {
			if !visit(s) {
				return false
			}
		}
		state[label] = done
		return true
	}
	return visit(start)
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

// Reject partial walks before making any edits. A surviving index address
// needs HL as scratch while the new loop-carried pointers occupy the pairs.
//
// An index that affineCounterPlan converts is not state: the model drops its
// exit test and its backedge argument, so neither the index nor a bound used
// only by that test is counted. The counter replacing them takes BC, which
// this budget reserves for counters.
func affinePointerProfitable(f *Func, l *NaturalLoop, iv LoopInduction, candidates []*Inst, walks int, counter *affineCounter, defs map[Reg]*Inst, types map[Reg]Ty) bool {
	replaced := map[Reg]bool{}
	for _, x := range candidates {
		replaced[x.Dst] = true
	}
	var depends func(Reg, map[Reg]bool) bool
	depends = func(r Reg, seen map[Reg]bool) bool {
		if r == iv.Param.Dst {
			return true
		}
		if replaced[r] {
			return false
		}
		if seen[r] {
			return false
		}
		seen[r] = true
		if x := defs[r]; x != nil {
			for _, s := range x.Uses() {
				if depends(s, seen) {
					return true
				}
			}
		}
		return false
	}
	for _, b := range f.Blocks {
		if !l.Blocks[b.Label] {
			continue
		}
		for _, x := range b.Insts {
			if (x.Op == OpPtrAdd && !replaced[x.Dst] && depends(x.Dst, map[Reg]bool{})) || ((x.Op == OpLoad || x.Op == OpStore) && depends(x.Src[0], map[Reg]bool{})) {
				return false
			}
		}
	}
	needed := affineNeeded(f, l, replaced, defs, counter)
	// Persistent word state includes surviving loop parameters and invariant
	// words needed on subsequent iterations. Rematerialised constants/addresses
	// are excluded. Reserve BC for counters; only HL and DE budget walks/state.
	words := walks
	state := map[Reg]bool{}
	for _, p := range l.Header.Params {
		if needed[p.Dst] && p.Ty.Width() == 16 {
			r := p.Dst
			if root, ok := l.Invariant[r]; ok {
				r = root
			}
			state[r] = true
		}
	}
	for r := range needed {
		if root, ok := l.Invariant[r]; ok && types[r] != nil && types[r].Width() == 16 {
			state[root] = true
		}
	}
	for r := range state {
		if x := defs[r]; x != nil && (x.Op == OpConst || x.Op == OpAddrOf) {
			continue
		}
		words++
	}
	if words > 2 {
		return false
	}
	// A counter occupies BC. With HL and DE also loop-carried, a word
	// temporary in the body has no pair left and is spilled to IX/IY on every
	// iteration, which costs more than the indexed loop it replaces. The
	// widening byte add `state + ext(byte)` needs no pair and is not counted.
	if counter != nil && words == 2 && affineWordTemporary(f, l, replaced, needed) {
		return false
	}
	// Setup is not amortised for a statically tiny loop. Strict ascending and
	// descending guards are the only shapes admitted by the range proof.
	initial := defs[iv.Initial]
	if initial != nil && initial.Op == OpConst {
		start := maskToWidth(initial.Imm, iv.Param.Ty)
		if iv.Param.Ty == TyI16 {
			start = signExtend(Value{I: start}, 16).I
		}
		trips := int64(-1)
		if _, ok := l.Latch.Term.(*TermDJNZ); ok {
			trips = start
		} else {
			var cond Reg
			switch t := l.Header.Term.(type) {
			case *TermBrIf:
				cond = t.Cond
			case *TermCondRet:
				cond = t.Cond
			}
			if c := defs[cond]; c != nil {
				if bound := defs[c.Src[1]]; bound != nil && bound.Op == OpConst {
					limit := maskToWidth(bound.Imm, iv.Param.Ty)
					if iv.Param.Ty == TyI16 {
						limit = signExtend(Value{I: limit}, 16).I
					}
					if iv.Step > 0 {
						trips = limit - start
					} else {
						trips = start - limit
						if c.Cond == CmpGe {
							trips++
						}
					}
					if trips < 0 {
						trips = 0
					}
				}
			}
		}
		if trips >= 0 && trips < 4 {
			return false
		}
	}
	return true
}
