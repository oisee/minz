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

	addEdgesAt := func(target string, args []Reg, paramOffset int) {
		tb := blockByLabel[target]
		if tb == nil || len(args) == 0 {
			return
		}
		for i, arg := range args {
			paramIdx := i + paramOffset
			if paramIdx >= len(tb.Params) {
				break
			}
			edges = append(edges, affEdge{dst: tb.Params[paramIdx].Dst, src: arg})
		}
	}
	addEdges := func(target string, args []Reg) { addEdgesAt(target, args, 0) }

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
			// BodyArgs does NOT include the counter (Params[0]); offset by 1.
			addEdgesAt(t.Body, t.BodyArgs, 1)
			addEdges(t.Exit, t.ExitArgs)
		}
	}

	if len(edges) == 0 {
		return
	}

	// Build a register-class index so we can guard recoloring below.
	// We must not recolor a register to a physical location that is
	// incompatible with its class (e.g. ClassCounter must stay in B).
	regClass := make(map[Reg]RegClass, len(result.Locs))
	for _, cp := range f.Contract.Params {
		regClass[cp.Reg] = cp.Class
	}
	for _, b := range f.Blocks {
		for _, bp := range b.Params {
			if bp.Dst != NoReg {
				regClass[bp.Dst] = bp.Class
			}
		}
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				regClass[inst.Dst] = inst.Cls
			}
		}
	}

	// ct is needed to check whether a physical location is finite-cost for
	// a given register class.  We use Z80CostTable (the only cost table in
	// use) to determine compatibility.
	ct := Z80CostTable{}

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

	// ── ISA output-constraint recoloring ──────────────────────────────────────
	// Instructions that always write to A on Z80 (ADD, SUB, AND, OR, XOR, NEG
	// with 8-bit operands) should live in A.  Recolor their Dst to A if safe,
	// so that subsequent block-arg affinity edges can chain from A to the
	// block params that receive the value.
	locA := PhysLoc{Kind: LocReg, Name: "A"}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst == NoReg || recolored[inst.Dst] {
				continue
			}
			if !isaAlwaysWritesA(inst) {
				continue
			}
			if result.Locs[inst.Dst] == locA {
				continue // already optimal
			}
			safe := true
			ig.Neighbors(inst.Dst).Each(func(n Reg) {
				if physicallyConflicts(result.Locs[n], locA) {
					safe = false
				}
			})
			if safe {
				result.Locs[inst.Dst] = locA
				recolored[inst.Dst] = true
			}
		}
	}

	for _, e := range edges {
		if recolored[e.dst] {
			continue // already committed this reg's location
		}
		srcLoc, srcOk := result.Locs[e.src]
		_, dstOk := result.Locs[e.dst]
		if !srcOk || !dstOk || result.Locs[e.dst] == srcLoc {
			continue // already coalesced or one reg is unallocated (spill)
		}
		// Class-compatibility guard: do not recolor dst to srcLoc if srcLoc
		// is not a legal home for dst's register class.  For example, a
		// ClassCounter register must stay in B; recoloring it to A would
		// break DJNZ.  Use InfCost as the "illegal" sentinel.
		if dstCls, ok := regClass[e.dst]; ok {
			if ct.Cost(dstCls, srcLoc) >= InfCost {
				continue
			}
		}
		// Recolor dst → srcLoc if no IG neighbour of dst uses srcLoc or any
		// of its physical aliases (e.g. C and BC alias — coalescing to C is
		// unsafe if a neighbour is in BC, and vice versa).
		safe := true
		ig.Neighbors(e.dst).Each(func(n Reg) {
			if physicallyConflicts(result.Locs[n], srcLoc) {
				safe = false
			}
		})
		if safe {
			result.Locs[e.dst] = srcLoc
			recolored[e.dst] = true
		}
	}
}

// isaAlwaysWritesA reports whether inst always writes its result to the A
// register on Z80 (8-bit ALU ops: ADD, SUB, AND, OR, XOR, NEG).
// 16-bit variants (ADD HL,rr etc.) write to HL, not A.
func isaAlwaysWritesA(inst *Inst) bool {
	if inst.Ty.Width() > 8 {
		return false
	}
	switch inst.Op {
	case OpAdd, OpSub, OpAnd, OpOr, OpXor, OpNeg:
		return true
	}
	return false
}
