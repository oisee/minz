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
	// ── Pass 0: demote ClassFlag vregs used in ALU ops to ClassAcc ────────────
	// F register cannot be INC'd/DEC'd/ADD'd. If a vreg is ClassFlag but also
	// used as src/dst in arithmetic, it must live in a GPR, not F.
	aluRegs := make(map[Reg]bool)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			switch inst.Op {
			case OpAdd, OpSub, OpMul, OpDiv, OpMod, OpShl, OpShr, OpSar,
				OpExt, OpSext, OpTrunc:
				if inst.Dst != NoReg {
					aluRegs[inst.Dst] = true
				}
				for _, s := range inst.Src {
					if s != NoReg {
						aluRegs[s] = true
					}
				}
			}
		}
		for _, p := range b.Params {
			aluRegs[p.Dst] = true // block params = loop counters
		}
	}
	for r, ri := range info {
		if ri.Cls == ClassFlag && aluRegs[r] {
			ri.Cls = ClassAcc // demote: F can't do ALU, A can do both CMP and ALU
			info[r] = ri
		}
	}

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
			propagateArgsAt := func(target string, args []Reg, paramOffset int) {
				tb := blockByLabel[target]
				if tb == nil {
					return
				}
				for i, arg := range args {
					paramIdx := i + paramOffset
					if paramIdx >= len(tb.Params) || arg == NoReg {
						break
					}
					paramInfo, ok := info[tb.Params[paramIdx].Dst]
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
			b.Term.ForEachEdge(func(target string, args []Reg, paramOffset int) {
				propagateArgsAt(target, args, paramOffset)
			})
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
	// classOf returns the register class for a virtual register by checking
	// function contract params first (they are not stored as block params),
	// then block params, then instruction dsts.
	classOf := func(r Reg) RegClass {
		// Function parameters live in the contract, not in block params.
		for _, cp := range f.Contract.Params {
			if cp.Reg == r {
				return cp.Class
			}
		}
		for _, b := range f.Blocks {
			for _, bp := range b.Params {
				if bp.Dst == r {
					return bp.Class
				}
			}
			for _, inst := range b.Insts {
				if inst.Dst == r {
					return inst.Cls
				}
			}
		}
		return ClassGeneral
	}

	// contractParamRegs holds function parameter registers whose physical
	// location is determined by OptimizeContracts (runs before us).  These
	// must NOT be coalesced with internal block params: doing so would let
	// PBQP override the ABI decision and produce a calling-convention mismatch.
	contractParamRegs := make(map[Reg]bool, len(f.Contract.Params))
	for _, cp := range f.Contract.Params {
		contractParamRegs[cp.Reg] = true
	}

	// Only merge when both sides have the same class (or an allowed upgrade).
	// Merging across class boundaries forces the argument into a specific
	// physical register, which can break the function's calling convention.
	// Merging a function contract param with an internal block param is also
	// prohibited: it allows PBQP to reassign the param to a different physreg
	// than what OptimizeContracts chose, causing an ABI mismatch for callers.
	mergeEdgeAt := func(target string, args []Reg, paramOffset int) {
		tb := blockByLabel[target]
		if tb == nil {
			return
		}
		for i, arg := range args {
			paramIdx := i + paramOffset
			if arg == NoReg || paramIdx >= len(tb.Params) {
				continue
			}
			// Don't coalesce function contract params — their physreg is
			// fixed by OptimizeContracts; coalescing would override that.
			if contractParamRegs[arg] {
				continue
			}
			argCls := classOf(arg)
			paramCls := tb.Params[paramIdx].Class
			// Allow: equal classes, or the well-known ClassGeneral→ClassAcc upgrade.
			if argCls != paramCls && !canUpgradeClass(argCls, paramCls) {
				continue // incompatible classes — would force wrong physreg
			}
			union(arg, tb.Params[paramIdx].Dst)
		}
	}
	mergeEdge := func(target string, args []Reg) { mergeEdgeAt(target, args, 0) }

	// Build block-index map for back-edge detection.
	// In the block list (RPO), a back-edge targets a block with a lower index.
	blockIdx := make(map[string]int, len(f.Blocks))
	for i, b := range f.Blocks {
		blockIdx[b.Label] = i
	}

	for bi, b := range f.Blocks {
		switch t := b.Term.(type) {
		case *TermJmp:
			// Skip back-edges entirely: merging across a loop back-edge can
			// union swap-pattern variables (a←b, b←a+b) into the same vreg,
			// preventing the allocator from assigning them to different
			// physical registers.  The post-allocation coalescer handles
			// these safely.
			if targetIdx, ok := blockIdx[t.Target]; ok && targetIdx <= bi {
				break // back-edge — skip
			}
			mergeEdge(t.Target, t.Args)
		case *TermBrIf:
			// Only merge unconditional-like paths (Else when ThenArgs empty,
			// or only the args that cannot conflict with the condition reg).
			// Merging across conditional branches risks promoting a merged
			// register to ClassAcc (A) which conflicts with the condition
			// computation (AND/CP etc.) also needing A in the source block.
			// For safety, we skip BrIf merges here; coalesceAllocResult's
			// affinity pass handles them post-allocation without class issues.
		case *TermBrIf2:
			// Same concern as BrIf: skip to avoid ClassAcc conflicts.
		case *TermDJNZ:
			// Do NOT merge BodyArgs: body→body is a back-edge. Merging
			// creates a register defined both as a block param and as an
			// instruction Dst in the same block, breaking liveness analysis.
			// Only merge the exit edge (an unconditional forward jump).
			mergeEdge(t.Exit, t.ExitArgs)
		case *TermCondRet:
			// CondRet has a condition — same ClassAcc risk as BrIf. Skip.
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
