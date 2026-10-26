package mir2

// DeadBlockArgElim removes block parameters that are never used anywhere in
// the function, including closed cycles used only by block-arg forwarding.
// When a parameter is removed, the corresponding argument is dropped from
// every incoming edge.
//
// This eliminates spurious parallel copies in trampolines — e.g. a loop exit
// block that returns only one of its three block args no longer generates
// copies for the two unused ones.
//
// Must run after DeadStoreElim (which may expose dead block params).
func DeadBlockArgElim(f *Func) bool {
	changed := false
	// Removing an incoming argument can make a predecessor parameter dead.
	// Recount uses after every round until no more parameters can be removed.
	for deadBlockArgElimRound(f) {
		changed = true
	}
	return changed
}

func deadBlockArgElimRound(f *Func) bool {
	// Solve semantic liveness across the whole forwarding graph.
	needed := liveBlockArguments(f)

	changed := false
	for _, b := range f.Blocks {
		if len(b.Params) == 0 {
			continue
		}
		// Never touch the entry block's params — they are function parameters.
		if b == f.Blocks[0] {
			continue
		}

		// Keep only parameters connected to a semantic use or ABI root.
		live := make([]bool, len(b.Params))
		allLive := true
		for i, p := range b.Params {
			live[i] = needed[p.Dst]
			if !live[i] {
				allLive = false
			}
		}
		if allLive {
			continue
		}

		// Compact the block's params.
		newParams := make([]BlockParam, 0, len(b.Params))
		for i, p := range b.Params {
			if live[i] {
				newParams = append(newParams, p)
			}
		}
		b.Params = newParams
		changed = true

		// Patch every incoming edge: remove args at dead positions.
		label := b.Label
		for _, pred := range f.Blocks {
			if pred.Term == nil {
				continue
			}
			compactEdgeArgs(pred.Term, label, live)
		}
	}
	return changed
}

// liveBlockArguments distinguishes semantic register uses from forwarding.
// An edge operand is needed only if its destination parameter is needed. Seed
// real instruction/control/return uses, then propagate backwards over all CFG
// edges. A forwarding-only cycle has no seed and therefore remains dead.
// This deliberately treats all instruction inputs as roots: removing dead
// instruction cycles is a separate DSE problem.
func liveBlockArguments(f *Func) map[Reg]bool {
	live := map[Reg]bool{}
	byLabel := map[string]*Block{}
	for _, b := range f.Blocks {
		byLabel[b.Label] = b
	}
	incoming := map[Reg][]Reg{}
	var work []Reg
	mark := func(r Reg) {
		if r != NoReg && !live[r] {
			live[r] = true
			work = append(work, r)
		}
	}
	if len(f.Blocks) == 0 {
		return live
	}
	// The entry signature and implicit DJNZ counters must keep their positions.
	for _, p := range f.Blocks[0].Params {
		mark(p.Dst)
	}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			for _, r := range i.Uses() {
				mark(r)
			}
		}
		if b.Term == nil {
			continue
		}
		known := true
		switch t := b.Term.(type) {
		case *TermJmp:
		case *TermBrIf:
			mark(t.Cond)
		case *TermBrIf2:
			mark(t.Lhs)
			mark(t.Rhs)
		case *TermDJNZ:
			mark(t.Counter)
		case *TermCondRet:
			mark(t.Cond)
			for _, r := range t.Vals {
				mark(r)
			}
		case *TermRet:
			for _, r := range t.Vals {
				mark(r)
			}
		case *TermErrorRet:
			mark(t.Code)
		case *TermUnreachable:
		default:
			// Future terminators retain their edge signatures until compaction knows
			// how to rewrite them; their reported semantic uses are also preserved.
			known = false
			for _, r := range termRegUses(b.Term) {
				mark(r)
			}
		}
		b.Term.ForEachEdge(func(label string, args []Reg, offset int) {
			target := byLabel[label]
			if target == nil {
				for _, r := range args {
					mark(r)
				}
				return
			}
			for j, p := range target.Params {
				if !known || j < offset {
					mark(p.Dst)
				}
			}
			for j, r := range args {
				at := j + offset
				if at >= len(target.Params) {
					mark(r)
					continue
				}
				dst := target.Params[at].Dst
				incoming[dst] = append(incoming[dst], r)
			}
		})
	}
	for len(work) > 0 {
		r := work[len(work)-1]
		work = work[:len(work)-1]
		for _, source := range incoming[r] {
			mark(source)
		}
	}
	return live
}

// compactEdgeArgs removes args at dead positions for edges to target.
func compactEdgeArgs(t Term, target string, live []bool) {
	compact := func(args []Reg, mask []bool) []Reg {
		out := make([]Reg, 0, len(args))
		for i, a := range args {
			if i < len(mask) && mask[i] {
				out = append(out, a)
			}
		}
		return out
	}

	switch t := t.(type) {
	case *TermJmp:
		if t.Target == target {
			t.Args = compact(t.Args, live)
		}
	case *TermBrIf:
		if t.Then == target {
			t.ThenArgs = compact(t.ThenArgs, live)
		}
		if t.Else == target {
			t.ElseArgs = compact(t.ElseArgs, live)
		}
	case *TermBrIf2:
		if t.Eq == target {
			t.EqArgs = compact(t.EqArgs, live)
		}
		if t.Lt == target {
			t.LtArgs = compact(t.LtArgs, live)
		}
		if t.Gt == target {
			t.GtArgs = compact(t.GtArgs, live)
		}
	case *TermDJNZ:
		if t.Body == target && len(live) > 0 {
			// Body's first parameter is supplied implicitly by the decremented
			// counter; explicit body arguments start at parameter index one.
			t.BodyArgs = compact(t.BodyArgs, live[1:])
		}
		if t.Exit == target {
			t.ExitArgs = compact(t.ExitArgs, live)
		}
	case *TermCondRet:
		if t.Then == target {
			t.ThenArgs = compact(t.ThenArgs, live)
		}
	}
}
