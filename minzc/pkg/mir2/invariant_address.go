package mir2

// HoistInvariantAddresses moves only modular, non-trapping address arithmetic
// from a natural loop into its unique unconditional preheader. Memory/host
// operations remain at their original locations, including on zero-trip paths.
// Calls, inline assembly and ports reject the loop; longer address lifetimes
// across them would introduce unmeasured saves and hard-register hazards.
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
	}
	return changed
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
