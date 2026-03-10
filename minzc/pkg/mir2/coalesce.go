package mir2

// coalesceAllocResult performs post-allocation copy coalescing to eliminate
// parallel copies at block boundaries and within-block OpMove instructions.
//
// Strategy
// ────────
// 1. Collect affinity edges — pairs (dst, src) where assigning dst the same
//    physical location as src would eliminate a copy:
//      a. OpMove instructions:        dst affine with src
//      b. Block boundary args:        block_param affine with its arg
//         (one edge per predecessor per param position)
//
// 2. Build a fresh interference graph from liveness data.
//    (The PBQP solver may have removed nodes during R1 reduction, so we
//    cannot reuse the graph from PBQPAllocate.)
//
// 3. Iteratively: for each affinity edge (dst, src) where Locs differ,
//    recolor dst to src's location if no IG neighbour of dst currently
//    occupies src's location.  Repeat until fixpoint.
//
// Correctness
// ───────────
//   • Block params and their args have disjoint live ranges by SSA
//     construction, so they never appear as IG neighbours — coalescing
//     is always safe unless a *third* reg already occupies the target loc.
//   • OpMove coalescing is gated on the IG: if src and dst interfere,
//     src is a neighbour of dst ⟹ the check fails ⟹ no coalescing.
func coalesceAllocResult(f *Func, result *AllocResult, lr *LivenessResult) {
	if len(result.Locs) == 0 {
		return
	}

	// Fresh interference graph (independent of PBQP's internal modifications).
	ig := BuildInterferenceGraph(f, lr)

	// Block label → block for fast lookup.
	blockByLabel := make(map[string]*Block, len(f.Blocks))
	for _, b := range f.Blocks {
		blockByLabel[b.Label] = b
	}

	// ── Collect affinity edges ─────────────────────────────────────────────

	type affEdge struct{ dst, src Reg }
	var edges []affEdge

	addEdges := func(target string, args []Reg) {
		tb := blockByLabel[target]
		if tb == nil || len(args) == 0 {
			return
		}
		for i, arg := range args {
			if i >= len(tb.Params) {
				break
			}
			edges = append(edges, affEdge{dst: tb.Params[i].Dst, src: arg})
		}
	}

	for _, b := range f.Blocks {
		// Within-block: OpMove affinities.
		for _, inst := range b.Insts {
			if inst.Op == OpMove && inst.Src[0] != NoReg && inst.Dst != NoReg {
				edges = append(edges, affEdge{dst: inst.Dst, src: inst.Src[0]})
			}
		}
		// Inter-block: block param/arg affinities per outgoing edge.
		switch t := b.Term.(type) {
		case *TermJmp:
			addEdges(t.Target, t.Args)
		case *TermBrIf:
			addEdges(t.Then, t.ThenArgs)
			addEdges(t.Else, t.ElseArgs)
		case *TermBrIf2:
			addEdges(t.Eq, t.EqArgs)
			addEdges(t.Lt, t.LtArgs)
			addEdges(t.Gt, t.GtArgs)
		case *TermDJNZ:
			addEdges(t.Body, t.BodyArgs)
			addEdges(t.Exit, t.ExitArgs)
		}
	}

	if len(edges) == 0 {
		return
	}

	// ── Single-pass coalescing ─────────────────────────────────────────────
	//
	// We do ONE scan over the affinity edges (not a fixpoint loop).
	//
	// Why not iterate to fixpoint?
	// Loop back-edges introduce affinity cycles, e.g.
	//   loop_head(a,b) → loop_body(a',b') → loop_head(b',a'+b')
	// creates a cycle  a↔a'↔b'↔a  in the affinity graph.  Since none of
	// these pairs interfere (disjoint live ranges), a fixpoint loop would
	// rotate their physical locations forever (A→DE,DE→HL,HL→A … repeat).
	//
	// One pass is sufficient for the common cases:
	//   • Direct block boundary: arg and param adjacent → coalesced in pass 1.
	//   • Direct OpMove: src and dst adjacent → coalesced in pass 1.
	// Transitive chains (A→B→C) may require ordering-dependent passes, but
	// such multi-hop coalescing is a bonus; correctness does not require it.
	//
	// Regs that have already been recolored in this pass are not moved again:
	// once a reg has been given its affinity partner's location we lock it so
	// that a later edge in the same scan cannot undo or rotate it.
	recolored := make(map[Reg]bool, len(result.Locs))
	for _, e := range edges {
		if recolored[e.dst] {
			continue // already committed this reg's location
		}
		srcLoc, srcOk := result.Locs[e.src]
		_, dstOk := result.Locs[e.dst]
		if !srcOk || !dstOk || result.Locs[e.dst] == srcLoc {
			continue // already coalesced or one reg is unallocated (spill)
		}
		// Recolor dst → srcLoc if no IG neighbour of dst uses srcLoc.
		safe := true
		ig.Neighbors(e.dst).Each(func(n Reg) {
			if result.Locs[n] == srcLoc {
				safe = false
			}
		})
		if safe {
			result.Locs[e.dst] = srcLoc
			recolored[e.dst] = true
		}
	}
}
