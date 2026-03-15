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
	compact := func(args []Reg) []Reg {
		out := make([]Reg, 0, len(args))
		for i, a := range args {
			if i < len(live) && live[i] {
				out = append(out, a)
			}
		}
		return out
	}

	switch t := t.(type) {
	case *TermJmp:
		if t.Target == target {
			t.Args = compact(t.Args)
		}
	case *TermBrIf:
		if t.Then == target {
			t.ThenArgs = compact(t.ThenArgs)
		}
		if t.Else == target {
			t.ElseArgs = compact(t.ElseArgs)
		}
	case *TermBrIf2:
		if t.Eq == target {
			t.EqArgs = compact(t.EqArgs)
		}
		if t.Lt == target {
			t.LtArgs = compact(t.LtArgs)
		}
		if t.Gt == target {
			t.GtArgs = compact(t.GtArgs)
		}
	case *TermCondRet:
		if t.Then == target {
			t.ThenArgs = compact(t.ThenArgs)
		}
	}
}
