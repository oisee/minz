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
	pred := map[string][]*Block{}
	dom := reachableDominators(f)
	for _, b := range f.Blocks {
		if b.Term != nil {
			for _, succ := range b.Term.Successors() {
				pred[succ] = append(pred[succ], b)
			}
		}
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
	for _, head := range f.Blocks {
		var latches []*Block
		var pre *Block
		for _, p := range pred[head.Label] {
			if dom[p.Label][head.Label] {
				latches = append(latches, p)
			} else if pre == nil {
				pre = p
			} else {
				pre = nil
				break
			}
		}
		if pre == nil || len(latches) != 1 {
			continue
		}
		jump, ok := pre.Term.(*TermJmp)
		if !ok || jump.Target != head.Label || len(jump.Args) != len(head.Params) {
			continue
		}
		loop := map[string]bool{head.Label: true}
		stack := []*Block{latches[0]}
		for len(stack) > 0 {
			b := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if loop[b.Label] {
				continue
			}
			loop[b.Label] = true
			stack = append(stack, pred[b.Label]...)
		}
		legal := true
		for name := range loop {
			if !dom[name][head.Label] {
				legal = false
			}
			b := f.BlockByLabel(name)
			for _, i := range b.Insts {
				if i.Op == OpAsm || i.Op == OpCall || i.Op == OpCallIndirect || i.Op == OpCallCond || i.Op == OpIn8 || i.Op == OpIn16 || i.Op == OpOut8 || i.Op == OpOut16 {
					legal = false
				}
			}
		}
		if !legal {
			continue
		}
		invariant := map[Reg]Reg{}
		for _, p := range f.Contract.Params {
			invariant[p.Reg] = p.Reg
		}
		for r, b := range defs {
			if !loop[b.Label] && dom[pre.Label][b.Label] {
				invariant[r] = r
			}
		}
		// A header parameter is invariant only when every loop edge passes it back
		// unchanged and its entering value dominates the preheader.
		for i, p := range head.Params {
			entry, known := invariant[jump.Args[i]]
			if !known {
				continue
			}
			same := true
			for _, b := range pred[head.Label] {
				if !loop[b.Label] {
					continue
				}
				args, ok := invariantEdgeArgs(b.Term, head.Label)
				if !ok || len(args) != len(head.Params) || args[i] != p.Dst {
					same = false
				}
			}
			if same {
				invariant[p.Dst] = entry
			}
		}
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
		for r, i := range instDefs {
			if needed[r] && (i.Op == OpAdd || i.Op == OpPtrAdd) && canHoist(r) {
				enable(r)
			}
		}
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
				case OpConst, OpAdd, OpPtrAdd, OpMove, OpExt:
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
			}
			b.Insts = out
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
