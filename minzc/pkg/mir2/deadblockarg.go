package mir2

// DeadBlockArgElim removes block parameters that are never used anywhere in
// the function (not by any instruction, terminator, or block-arg forwarding).
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
	// Compute global use counts for all registers across the whole function.
	uses := countRegUses(f)

	changed := false
	for _, b := range f.Blocks {
		if len(b.Params) == 0 {
			continue
		}
		// Never touch the entry block's params — they are function parameters.
		if b == f.Blocks[0] {
			continue
		}

		// Build the keep mask: a param is live if its register has any use.
		live := make([]bool, len(b.Params))
		allLive := true
		for i, p := range b.Params {
			live[i] = uses[p.Dst] > 0
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
