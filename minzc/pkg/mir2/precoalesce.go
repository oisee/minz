package mir2

// PropagateClassHintsForTest exports propagateClassHints for external test packages.
func PropagateClassHintsForTest(f *Func, info map[Reg]RegInfo) {
	propagateClassHints(f, info)
}

// propagateClassHints propagates register class preferences through phi-web
// edges before PBQP. It runs two sub-passes:
//
//  1. Return-value promotion: registers used directly in TermRet/TermCondRet
//     are upgraded to ClassAcc (they must be in A on Z80).
//
//  2. Backward phi-web propagation: if a block parameter has a more-specific
//     class than an arg feeding it, the arg's class is upgraded.
//     Repeats to fixpoint so chains of length >1 are handled.
//
// This ensures that when an ALU op's result (ClassAcc) is threaded through
// block arguments to a return, PBQP sees ClassAcc on every node in the chain
// and assigns them all to A — avoiding LD C,A / LD A,C round-trips.
func propagateClassHints(f *Func, info map[Reg]RegInfo) {
	// ── Pass 1: mark return values as ClassAcc ────────────────────────────────
	for _, b := range f.Blocks {
		promoteRetVals := func(vals []Reg) {
			for _, v := range vals {
				if v == NoReg {
					continue
				}
				ri, ok := info[v]
				if !ok {
					continue
				}
				// Only upgrade scalar ints; bool/ptr/u16 have their own classes.
				if ri.Ty == TyBool || ri.Ty == TyPtr ||
					ri.Ty == TyU16 || ri.Ty == TyI16 {
					continue
				}
				if canUpgradeClass(ri.Cls, ClassAcc) {
					ri.Cls = ClassAcc
					info[v] = ri
				}
			}
		}
		switch t := b.Term.(type) {
		case *TermRet:
			promoteRetVals(t.Vals)
		case *TermCondRet:
			promoteRetVals(t.Vals)
		}
	}

	// ── Pass 2: backward phi-web propagation ─────────────────────────────────
	blockByLabel := make(map[string]*Block, len(f.Blocks))
	for _, b := range f.Blocks {
		blockByLabel[b.Label] = b
	}

	changed := true
	for changed {
		changed = false
		for _, b := range f.Blocks {
			propagateArgs := func(target string, args []Reg) {
				tb := blockByLabel[target]
				if tb == nil {
					return
				}
				for i, arg := range args {
					if i >= len(tb.Params) || arg == NoReg {
						break
					}
					paramInfo, ok := info[tb.Params[i].Dst]
					if !ok {
						continue
					}
					argInfo, ok := info[arg]
					if !ok {
						continue
					}
					// Upgrade arg's class if the param requires a more-specific class.
					if canUpgradeClass(argInfo.Cls, paramInfo.Cls) {
						argInfo.Cls = paramInfo.Cls
						info[arg] = argInfo
						changed = true
					}
				}
			}

			switch t := b.Term.(type) {
			case *TermJmp:
				propagateArgs(t.Target, t.Args)
			case *TermBrIf:
				propagateArgs(t.Then, t.ThenArgs)
				propagateArgs(t.Else, t.ElseArgs)
			case *TermBrIf2:
				propagateArgs(t.Eq, t.EqArgs)
				propagateArgs(t.Lt, t.LtArgs)
				propagateArgs(t.Gt, t.GtArgs)
			case *TermDJNZ:
				propagateArgs(t.Body, t.BodyArgs)
				propagateArgs(t.Exit, t.ExitArgs)
			case *TermCondRet:
				propagateArgs(t.Then, t.ThenArgs)
			}
		}
	}
}

// canUpgradeClass reports whether a register currently assigned `from` can be
// safely upgraded to `to` without restricting its use.  The only upgrade we
// perform is ClassGeneral → ClassAcc: ClassAcc is a subset of ClassGeneral
// (both map to A as the optimal location), so narrowing to ClassAcc never
// causes a new spill.
func canUpgradeClass(from, to RegClass) bool {
	return from == ClassGeneral && to == ClassAcc
}

// ── Union-Find pre-coalescing ──────────────────────────────────────────────

// PreallocCoalesce merges block-argument and block-parameter virtual registers
// across every CFG edge before the PBQP allocator runs.
//
// # Why this is safe
//
// By SSA construction, a block argument (defined somewhere in the source block)
// and the corresponding block parameter (live-in at the target block) have
// DISJOINT live ranges: the argument's last use is the branch itself, and the
// parameter's first use is the first instruction of the target block.  Because
// they never overlap, they can never interfere, so merging them into one
// virtual register cannot introduce a new conflict in the interference graph.
//
// # What it buys
//
// Without pre-coalescing, PBQP assigns each virtual register independently.
// A block parameter and its feeding argument often end up in different physical
// registers (e.g. param→A, arg→E), forcing the parallel-copy resolver to emit
// a swap or multiple LD instructions on the back-edge of every loop iteration.
// After pre-coalescing, the merged register gets a single physical location
// that satisfies all uses, and the parallel copy becomes a no-op.
//
// # Limitation
//
// Pre-coalescing does not remap registers that appear in hard-class
// instructions (ClsHard) where the classes of the two endpoints conflict.
// Such pairs are left unmerged to avoid forcing an impossible allocation.
// PreallocCoalesceMap returns a canonical-register map after pre-coalescing.
// Keys are original virtual registers; values are the union-find roots they
// were merged into.  Callers can use this to look up allocation results for
// registers that were remapped (e.g. block params that became args).
type PreallocCoalesceMap = map[Reg]Reg

// PreallocCoalesce merges block-argument and block-parameter virtual registers
// across every CFG edge before the PBQP allocator runs.  It returns a map
// from original register → canonical root so callers can resolve remapped regs.
//
// # Why this is safe
//
// By SSA construction, a block argument (defined somewhere in the source block)
// and the corresponding block parameter (live-in at the target block) have
// DISJOINT live ranges: the argument's last use is the branch itself, and the
// parameter's first use is the first instruction of the target block.  Because
// they never overlap, they can never interfere, so merging them into one
// virtual register cannot introduce a new conflict in the interference graph.
//
// # What it buys
//
// Without pre-coalescing, PBQP assigns each virtual register independently.
// A block parameter and its feeding argument often end up in different physical
// registers (e.g. param→A, arg→E), forcing the parallel-copy resolver to emit
// a swap or multiple LD instructions on the back-edge of every loop iteration.
// After pre-coalescing, the merged register gets a single physical location
// that satisfies all uses, and the parallel copy becomes a no-op.
func PreallocCoalesce(f *Func) PreallocCoalesceMap {
	if len(f.Blocks) == 0 {
		return nil
	}

	// ── Union-Find ────────────────────────────────────────────────────────────
	parent := make(map[Reg]Reg)
	rank := make(map[Reg]int)

	var find func(Reg) Reg
	find = func(r Reg) Reg {
		if r == NoReg {
			return NoReg
		}
		if p, ok := parent[r]; ok && p != r {
			root := find(p)
			parent[r] = root // path compression
			return root
		}
		return r
	}

	union := func(a, b Reg) {
		if a == NoReg || b == NoReg {
			return
		}
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		// Union by rank.
		if rank[ra] < rank[rb] {
			ra, rb = rb, ra
		}
		parent[rb] = ra
		if rank[ra] == rank[rb] {
			rank[ra]++
		}
	}

	// Initialise each register as its own root.
	for _, b := range f.Blocks {
		for _, bp := range b.Params {
			if bp.Dst != NoReg {
				parent[bp.Dst] = bp.Dst
			}
		}
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				parent[inst.Dst] = inst.Dst
			}
		}
	}

	// Build block-label index.
	blockByLabel := make(map[string]*Block, len(f.Blocks))
	for _, b := range f.Blocks {
		blockByLabel[b.Label] = b
	}

	// For each CFG edge, union arg[i] with target.Params[i].
	// Skip pairs where either side has a hard class constraint that conflicts
	// with the other — forcing them together would produce an impossible class.
	unionsafe := func(argReg, paramDst Reg, argCls, paramCls RegClass, argHard, paramHard bool) {
		if argHard && paramHard && argCls != paramCls {
			return // conflicting hard constraints — leave separate
		}
		union(argReg, paramDst)
	}

	mergeEdge := func(target string, args []Reg) {
		tb := blockByLabel[target]
		if tb == nil {
			return
		}
		for i, arg := range args {
			if arg == NoReg || i >= len(tb.Params) {
				continue
			}
			// Determine class constraints for the arg (from defining instruction).
			var argCls RegClass
			var argHard bool
			for _, b := range f.Blocks {
				for _, inst := range b.Insts {
					if inst.Dst == arg {
						argCls = inst.Cls
						argHard = inst.ClsHard
					}
				}
			}
			unionsafe(arg, tb.Params[i].Dst, argCls, tb.Params[i].Class, argHard, false)
		}
	}

	for _, b := range f.Blocks {
		switch t := b.Term.(type) {
		case *TermJmp:
			mergeEdge(t.Target, t.Args)
		case *TermBrIf:
			mergeEdge(t.Then, t.ThenArgs)
			mergeEdge(t.Else, t.ElseArgs)
		case *TermBrIf2:
			mergeEdge(t.Eq, t.EqArgs)
			mergeEdge(t.Lt, t.LtArgs)
			mergeEdge(t.Gt, t.GtArgs)
		case *TermDJNZ:
			mergeEdge(t.Body, t.BodyArgs)
			mergeEdge(t.Exit, t.ExitArgs)
		case *TermCondRet:
			mergeEdge(t.Then, t.ThenArgs)
		}
	}

	// ── Remap ─────────────────────────────────────────────────────────────────
	// Replace every virtual register reference with its union-find root.

	mapReg := func(r Reg) Reg {
		if r == NoReg {
			return NoReg
		}
		return find(r)
	}

	mapSlice := func(rs []Reg) {
		for i, r := range rs {
			rs[i] = mapReg(r)
		}
	}

	for _, b := range f.Blocks {
		// Remap block params.
		for i := range b.Params {
			b.Params[i].Dst = mapReg(b.Params[i].Dst)
		}
		// Remap instruction operands.
		for _, inst := range b.Insts {
			inst.Dst = mapReg(inst.Dst)
			inst.Src[0] = mapReg(inst.Src[0])
			inst.Src[1] = mapReg(inst.Src[1])
			mapSlice(inst.Args)
			mapSlice(inst.ExtraRets)
		}
		// Remap terminators.
		if b.Term != nil {
			switch t := b.Term.(type) {
			case *TermJmp:
				mapSlice(t.Args)
			case *TermBrIf:
				t.Cond = mapReg(t.Cond)
				mapSlice(t.ThenArgs)
				mapSlice(t.ElseArgs)
			case *TermBrIf2:
				t.Lhs = mapReg(t.Lhs)
				t.Rhs = mapReg(t.Rhs)
				mapSlice(t.EqArgs)
				mapSlice(t.LtArgs)
				mapSlice(t.GtArgs)
			case *TermDJNZ:
				t.Counter = mapReg(t.Counter)
				mapSlice(t.BodyArgs)
				mapSlice(t.ExitArgs)
			case *TermCondRet:
				t.Cond = mapReg(t.Cond)
				mapSlice(t.Vals)
				mapSlice(t.ThenArgs)
			case *TermRet:
				mapSlice(t.Vals)
			}
		}
	}

	// Remap contract params (entry block params bound to the ABI).
	for i := range f.Contract.Params {
		f.Contract.Params[i].Reg = mapReg(f.Contract.Params[i].Reg)
	}

	// Build and return the canonical map: original reg → root reg.
	// This lets callers resolve allocation results for remapped registers.
	canonical := make(PreallocCoalesceMap, len(parent))
	for r := range parent {
		canonical[r] = find(r)
	}
	return canonical
}
