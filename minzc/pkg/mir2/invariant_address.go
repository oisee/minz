package mir2

// HoistInvariantAddresses moves modular, non-trapping address arithmetic and —
// under the stricter proof in hoistInvariantGlobalLoads — loads of invariant
// static global addresses from a natural loop into its unique unconditional
// preheader. Other memory/host operations remain at their original locations,
// including on zero-trip paths. Calls, inline assembly and ports reject the
// loop; longer address lifetimes across them would introduce unmeasured saves
// and hard-register hazards.
func HoistInvariantAddresses(f *Func) bool {
	if len(f.Blocks) < 3 {
		return false
	}
	defs := map[Reg]*Block{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst] = b
		}
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				defs[i.Dst] = b
			}
		}
	}
	changed := false
	for _, facts := range AnalyzeNaturalLoops(f) {
		if facts.Entry == nil {
			continue
		} // hoisting requires an unconditional preheader
		pre, loop := facts.Preheader, facts.Blocks
		legal := true
		for _, b := range f.Blocks {
			if !loop[b.Label] {
				continue
			}
			for _, i := range b.Insts {
				if i.Op == OpAsm || i.Op == OpCall || i.Op == OpCallIndirect || i.Op == OpCallCond || i.Op == OpIn8 || i.Op == OpIn16 || i.Op == OpOut8 || i.Op == OpOut16 {
					legal = false
				}
			}
		}
		if !legal {
			continue
		}
		invariant := facts.Invariant
		// Restrict arithmetic to DAGs that actually feed memory address operands.
		needed := map[Reg]bool{}
		var mark func(Reg)
		instDefs := map[Reg]*Inst{}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				instDefs[i.Dst] = i
			}
		}
		mark = func(r Reg) {
			if needed[r] {
				return
			}
			needed[r] = true
			if i := instDefs[r]; i != nil && (i.Op == OpAdd || i.Op == OpPtrAdd || i.Op == OpMove || i.Op == OpExt) {
				for _, s := range i.Uses() {
					mark(s)
				}
			}
		}
		for _, b := range f.Blocks {
			if !loop[b.Label] {
				continue
			}
			for _, i := range b.Insts {
				if i.Op == OpLoad || i.Op == OpStore {
					mark(i.Src[0])
				}
				if i.Op == OpCall && (i.Sym == "peek" || i.Sym == "@peek" || i.Sym == "poke" || i.Sym == "@poke") && len(i.Args) > 0 {
					mark(i.Args[0])
				}
			}
		}
		// Only hoist a constant when it supports a proven invariant addition;
		// moving isolated loop constants otherwise increases register pressure.
		eligible := map[Reg]bool{}
		visiting := map[Reg]bool{}
		var canHoist func(Reg) bool
		canHoist = func(r Reg) bool {
			if _, ok := invariant[r]; ok {
				return true
			}
			if visiting[r] {
				return false
			}
			visiting[r] = true
			defer delete(visiting, r)
			i := instDefs[r]
			if i == nil || defs[r] == nil || !loop[defs[r].Label] || i.ClsHard || i.Ty == nil || i.Ty.Width() != 16 {
				return false
			}
			switch i.Op {
			case OpConst:
				return true
			case OpAddrOf:
				// Unknown/string/dynamic symbols may fail resolution in VM;
				// only an existing module global is a total address constant.
				return f.mod != nil && invariantGlobalSymbol(f.mod, i.Sym)
			case OpAdd, OpPtrAdd, OpMove, OpExt:
			default:
				return false
			}
			for _, src := range i.Uses() {
				if !canHoist(src) {
					return false
				}
			}
			return true
		}
		var enable func(Reg)
		enable = func(r Reg) {
			if _, ok := invariant[r]; ok {
				return
			}
			if eligible[r] {
				return
			}
			eligible[r] = true
			for _, src := range instDefs[r].Uses() {
				enable(src)
			}
		}
		// Modular address addition: (invariant + index) + invariantBase
		// becomes (invariantBase + invariant) + index. Only rewrite an
		// address DAG whose original sum has one exclusive use.
		uses := countRegUses(f)
		for _, b := range f.Blocks {
			if !loop[b.Label] {
				continue
			}
			for _, i := range b.Insts {
				if !needed[i.Dst] || i.Op != OpPtrAdd || i.ClsHard || i.Ty == nil || i.Ty.Width() != 16 {
					continue
				}
				base, indexSum := i.Src[0], instDefs[i.Src[1]]
				if !canHoist(base) || indexSum == nil || defs[indexSum.Dst] == nil || !loop[defs[indexSum.Dst].Label] || indexSum.Op != OpAdd || indexSum.ClsHard || indexSum.Ty == nil || indexSum.Ty.Width() != 16 || uses[indexSum.Dst] != 1 {
					continue
				}
				invariantPart, variable := indexSum.Src[0], indexSum.Src[1]
				if !canHoist(invariantPart) || canHoist(variable) {
					continue
				}
				indexSum.Op = OpPtrAdd
				indexSum.Ty = i.Ty
				indexSum.Src = [2]Reg{base, invariantPart}
				i.Src = [2]Reg{indexSum.Dst, variable}
				i.InBounds = false // the base is no longer the declared object
				changed = true
			}
		}
		for r, i := range instDefs {
			if needed[r] && (i.Op == OpAdd || i.Op == OpPtrAdd) && canHoist(r) {
				enable(r)
			}
		}
		for progress := true; progress; {
			progress = false
			for _, b := range f.Blocks {
				if !loop[b.Label] {
					continue
				}
				out := b.Insts[:0]
				for _, i := range b.Insts {
					if !eligible[i.Dst] || i.ClsHard || i.Ty == nil || i.Ty.Width() != 16 {
						out = append(out, i)
						continue
					}
					switch i.Op {
					case OpConst, OpAddrOf, OpAdd, OpPtrAdd, OpMove, OpExt:
					default:
						out = append(out, i)
						continue
					}
					pure := true
					for _, r := range i.Uses() {
						if _, ok := invariant[r]; !ok {
							pure = false
						}
					}
					if !pure {
						out = append(out, i)
						continue
					}
					copyInst := *i
					for n, r := range copyInst.Src {
						if r != NoReg {
							copyInst.Src[n] = invariant[r]
						}
					}
					pre.Insts = append(pre.Insts, &copyInst)
					invariant[i.Dst] = i.Dst
					defs[i.Dst] = pre
					changed = true
					progress = true
				}
				b.Insts = out
			}
		}
		if hoistInvariantGlobalLoads(f, facts) {
			changed = true
		}
	}
	return changed
}

// hoistInvariantGlobalLoads moves loads of loop-invariant static global
// addresses into the loop preheader. A hoisted load executes once per loop
// entry instead of once per iteration, and also on a zero-trip entry, so every
// condition below is a proof, not a heuristic:
//
//   - nothing in the loop can write the loaded global or has unknown effects
//     (loopMayWriteGlobal: every store provably stays inside a different
//     ordinary global), so the loaded location cannot change between the
//     preheader and any iteration that reads it;
//   - the address is a byte range inside one ordinary module global — not an
//     absolute (At) placement, volatile storage or a special storage class —
//     so the read has no observable effect and the storage always exists,
//     which is what makes executing it before the loop safe;
//   - the address is built from addr_of and constants only, so it is
//     invariant by construction. A pointer parameter, a heap address or a
//     loop-variant offset is never admitted.
//
// Three further gates are profitability, not safety, and all are measured: the
// result must be one byte wide, and the loop must read that byte range exactly
// once. Both exclude loops that already rematerialise the value in one direct
// instruction, where the long live range costs more than the reload. Third,
// the loop must contain no store at all, although loopMayWriteGlobal can prove
// a store harmless: a storing loop is what RecognizeByteLoops later turns into
// a fill/copy intrinsic, which reads a count or value global directly and is
// cheaper than a hoisted register argument (admitting stores here measured
// memset_fill 3588390T -> 3598695T, and no benchmark case faster). A loop
// bound in such a loop is moved by affineCounterPlan instead.
//
// The value is re-read on every call: only the loop is escaped, never the
// function.
func hoistInvariantGlobalLoads(f *Func, facts *NaturalLoop) bool {
	if f.mod == nil || facts.Preheader == nil {
		return false
	}
	loop := facts.Blocks
	for _, b := range f.Blocks {
		if loop[b.Label] {
			for _, i := range b.Insts {
				if i.Op == OpStore {
					return false
				}
			}
		}
	}
	instDefs := map[Reg]*Inst{}
	home := map[*Inst]*Block{}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				instDefs[i.Dst] = i
			}
			home[i] = b
		}
	}
	// Address definitions move with their load, in dependency order.
	var move []*Inst
	moving := map[*Inst]bool{}
	var emit func(i *Inst)
	emit = func(i *Inst) {
		if moving[i] {
			return
		}
		moving[i] = true
		for _, u := range i.Uses() {
			if d := instDefs[u]; d != nil && loop[home[d].Label] && !moving[d] {
				emit(d)
			}
		}
		move = append(move, i)
	}
	// Collect the candidates first, keyed by the byte range they read, so a
	// location the loop reads more than once can be refused as a whole.
	type byteRange struct {
		sym    string
		offset int
	}
	candidates := map[byteRange][]*Inst{}
	var order []byteRange
	for _, b := range f.Blocks {
		if !loop[b.Label] {
			continue
		}
		for _, i := range b.Insts {
			// Only byte-wide results are hoisted. A wider value has to hold a
			// full pointer pair for the whole loop, while the direct-addressing
			// form (LD rr,(nn)) already re-reads it in one instruction with no
			// address register: hoisting the loop bound `n` measured 13-46%
			// slower on the C fusion cases because the pair it occupied
			// displaced the array base pointers into IX/IY.
			if i.Op != OpLoad || i.Ty == nil || ByteWidth(i.Ty) != 1 {
				continue
			}
			g, base, ok := staticGlobalLoad(f, i, instDefs)
			if !ok || loopMayWriteGlobal(f, loop, g, instDefs) {
				continue
			}
			key := byteRange{g.Name, base}
			if _, seen := candidates[key]; !seen {
				order = append(order, key)
			}
			candidates[key] = append(candidates[key], i)
		}
	}
	hoisted := false
	for _, key := range order {
		list := candidates[key]
		// A loop that re-reads the same location several times is
		// rematerialising it: LD A,(nn) costs 13T and no register, while
		// several hoisted copies would each have to survive the whole loop
		// (measured +28% on checksum16, whose loop bound is read in the head
		// and again in the body). Leave such locations alone.
		if len(list) != 1 {
			continue
		}
		emit(list[0])
		hoisted = true
	}
	if !hoisted {
		return false
	}
	for _, i := range move {
		b := home[i]
		out := b.Insts[:0]
		for _, x := range b.Insts {
			if x != i {
				out = append(out, x)
			}
		}
		b.Insts = out
	}
	facts.Preheader.Insts = append(facts.Preheader.Insts, move...)
	return true
}

// staticGlobalLoad proves that load reads a byte range inside one ordinary
// module global through an address built from addr_of and constants only, and
// returns that global and the offset. Such a read has no observable effect and
// its storage always exists, so it may execute earlier than written; whether
// the location can change in between is the caller's proof
// (loopMayWriteGlobal for a loop).
func staticGlobalLoad(f *Func, load *Inst, instDefs map[Reg]*Inst) (*Global, int, bool) {
	if f.mod == nil || load.Op != OpLoad || load.Dst == NoReg || load.Volatile || load.SequentialBytes ||
		len(load.SequentialByteOrigins) != 0 || len(load.SequentialAddressDefs) != 0 ||
		load.Ty == nil || load.Src[0] == NoReg {
		return nil, 0, false
	}
	word := f.mod.PointerType().Width()
	// prove resolves r to the global it addresses and the byte offset within
	// it, admitting only constants, addr_of and pure pointer arithmetic.
	var prove func(r Reg, seen map[Reg]bool) (*Global, int, bool)
	prove = func(r Reg, seen map[Reg]bool) (*Global, int, bool) {
		if seen[r] {
			return nil, 0, false
		}
		i := instDefs[r]
		if i == nil || i.ClsHard || i.Ty == nil || i.Ty.Width() != word {
			return nil, 0, false
		}
		seen[r] = true
		defer delete(seen, r)
		switch i.Op {
		case OpAddrOf:
			for idx := range f.mod.Globals {
				g := &f.mod.Globals[idx]
				if g.Name != i.Sym || g.Ty == nil {
					continue
				}
				if g.At != nil || g.Storage != StorageNormal || g.Volatile || HasVolatileStorage(g.Ty) {
					return nil, 0, false
				}
				return g, 0, true
			}
			return nil, 0, false
		case OpMove, OpExt:
			if i.Src[0] == NoReg {
				return nil, 0, false
			}
			g, base, ok := prove(i.Src[0], seen)
			return g, base, ok
		case OpField, OpPtrBump:
			if i.Src[0] == NoReg || i.Imm < 0 {
				return nil, 0, false
			}
			g, base, ok := prove(i.Src[0], seen)
			return g, base + int(i.Imm), ok
		case OpAdd, OpPtrAdd:
			for _, side := range [2]int{0, 1} {
				other := 1 - side
				if i.Src[side] == NoReg || i.Src[other] == NoReg {
					continue
				}
				g, base, ok := prove(i.Src[side], seen)
				if !ok {
					continue
				}
				c := instDefs[i.Src[other]]
				if c == nil || c.Op != OpConst || c.ClsHard || c.Ty == nil || c.Ty.Width() != word || c.Imm < 0 {
					continue
				}
				return g, base + int(c.Imm), true
			}
		}
		return nil, 0, false
	}
	g, base, ok := prove(load.Src[0], map[Reg]bool{})
	if !ok || g == nil || g.Ty == nil || base < 0 || base+ByteWidth(load.Ty) > ByteWidth(g.Ty) {
		return nil, 0, false
	}
	return g, base, true
}

func invariantEdgeArgs(t Term, target string) ([]Reg, bool) {
	switch t := t.(type) {
	case *TermJmp:
		if t.Target == target {
			return t.Args, true
		}
	case *TermBrIf:
		if t.Then == target && t.Else != target {
			return t.ThenArgs, true
		}
		if t.Else == target && t.Then != target {
			return t.ElseArgs, true
		}
	case *TermCondRet:
		if t.Then == target {
			return t.ThenArgs, true
		}
	case *TermDJNZ:
		if t.Body == target && t.Exit != target {
			return t.BodyArgs, true
		}
		if t.Exit == target && t.Body != target {
			return t.ExitArgs, true
		}
	}
	return nil, false
}

func invariantGlobalSymbol(m *Module, name string) bool {
	for _, g := range m.Globals {
		if g.Name == name && g.Storage == StorageNormal {
			return true
		}
	}
	return false
}
