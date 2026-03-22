package mir2

// Phase 5b — Function Contract Optimisation (interprocedural PBQP).
//
// Problem:
//   Each function's parameter and return register classes are currently fixed
//   at IR build time.  Two independently-optimal choices can create needless
//   register moves at call sites:
//
//     double_sum calls double twice.
//     double_sum spills 'b' to ClassGeneral (C) to survive the first CALL.
//     double's contract demands ClassAcc (A).
//     → LD A, C emitted before the second CALL (4T overhead).
//
//   With interprocedural contract optimisation we ask:
//     "Which class for each param/return of each function minimises the total
//      cost across all callers and callees?"
//
// Algorithm (greedy DP on topo-sorted call graph):
//
//  1. Build call graph.
//  2. Topological sort (leaves first).
//  3. For each function in topo order:
//     a. For each candidate class vector (all combinations of plausible classes
//        for each param/return slot):
//        - Unary cost  = internal adapter cost inside the function body
//        - Edge cost   = sum over call sites to already-decided callees of
//                        moveCost(candidateClass, calleeParamClass) × callCount
//     b. Pick the minimum-cost candidate.
//  4. Apply the chosen classes to Contract.Params[i].Class in place.
//
// For call graph sizes typical in our examples (≤20 functions, ≤4 params each)
// the candidate space is tiny and exhaustive enumeration is instantaneous.
//
// Note on Z80 gains:
//   On Z80 all primary reg-to-reg moves cost 4T (LD r,r').  Contract optimisation
//   rarely eliminates moves entirely; it mostly confirms the current manual choices
//   are optimal, and enables automatic class selection when lowering from HIR
//   (no manual ClassAcc/ClassGeneral annotation needed).
//   Bigger gains appear for ClassFlag returns (already implemented separately) and
//   for 16-bit params where HL vs DE can matter for subsequent ops.

// ContractSet maps function name → the optimised class choices for its contract.
type ContractSet map[string]*ContractChoice

// ContractChoice records the chosen register class for each param/return slot.
type ContractChoice struct {
	ParamClasses  []RegClass // one per Contract.Params[i]
	ReturnClasses []RegClass // one per Contract.Returns[i]
}

// OptimizeContracts finds the globally-optimal register classes for all
// function contracts in m, using a greedy DP on the call graph with
// multi-pass convergence.
//
// Pass 0: bottom-up topo order (callees before callers) — same as before.
//         No incoming cost (callers not yet decided).
// Pass 1+: re-evaluate every function with incomingEdgeCost included so that
//          callees with multiple callers can "pull back" an EX DE,HL from
//          N call sites into the callee body when N×moveCost > bodyMoveCost.
//
// Callers should call ApplyContracts(m, cs) before running Allocate.
func OptimizeContracts(m *Module, ct CostTable) ContractSet {
	cg := BuildCallGraph(m)
	cs := make(ContractSet, len(m.Funcs))

	// Functions whose address is taken (OpAddrOf) must keep standard ABI
	// because indirect calls use a synthetic standard-ABI contract.
	addrTaken := addrTakenFuncs(m)

	const maxPasses = 4
	for pass := 0; pass < maxPasses; pass++ {
		changed := false
		for _, name := range cg.Order() {
			f := m.FuncByName(name)
			if f == nil {
				continue
			}
			// Skip extern/no-body/address-taken/asm functions — their contract is fixed by the ABI.
			// Functions with inline asm have hard register constraints that must not be changed.
			if f.Attrs.IsExtern || len(f.Blocks) == 0 || addrTaken[name] || funcHasAsm(f) {
				if cs[name] == nil {
					cs[name] = currentChoice(f)
					changed = true
				}
				continue
			}

			// Start with the current (manually-set) contract as the reference.
			// Only replace it when a candidate is STRICTLY cheaper (stable: ties keep current).
			current := currentChoice(f)
			var inc int
			if pass > 0 {
				inc = incomingEdgeCost(f, current, m, cs, ct)
			}
			bestChoice := current
			bestCost := unaryCostWithMod(f, current, ct, m) + edgeCost(f, current, cg, cs, ct) + inc

			for _, choice := range candidateChoices(f, ct) {
				var choiceInc int
				if pass > 0 {
					choiceInc = incomingEdgeCost(f, choice, m, cs, ct)
				}
				cost := unaryCostWithMod(f, choice, ct, m) + edgeCost(f, choice, cg, cs, ct) + choiceInc
				if cost < bestCost {
					bestCost = cost
					bestChoice = choice
				}
			}

			if cs[name] == nil || !choiceEqual(cs[name], bestChoice) {
				changed = true
			}
			cs[name] = bestChoice
		}
		if !changed {
			break
		}
	}
	return cs
}

// addrTakenFuncs returns the set of function names whose address is taken
// via OpAddrOf anywhere in the module.  These functions must keep standard ABI
// because indirect calls (OpCallIndirect) use a synthetic standard-ABI contract.
func addrTakenFuncs(m *Module) map[string]bool {
	funcNames := make(map[string]bool, len(m.Funcs))
	for _, f := range m.Funcs {
		funcNames[f.Name] = true
	}
	taken := make(map[string]bool)
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op == OpAddrOf && funcNames[inst.Sym] {
					taken[inst.Sym] = true
				}
			}
		}
	}
	return taken
}

// funcHasAsm reports whether f contains any OpAsm instruction.
// Functions with inline asm have hard-coded register expectations that
// OptimizeContracts must not change.
func funcHasAsm(f *Func) bool {
	if f.Attrs.HasAsm {
		return true
	}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpAsm {
				f.Attrs.HasAsm = true // cache for next check
				return true
			}
		}
	}
	// Also check: any param with hard-pin class (@z80_c, @z80_d, etc.)
	for _, p := range f.Contract.Params {
		switch p.Class {
		case ClassRegC, ClassRegD, ClassRegE, ClassRegH, ClassRegL:
			return true
		}
	}
	return false
}

// ApplyContracts updates every function's Contract in place with the
// classes chosen by OptimizeContracts.  Call this before Allocate.
func ApplyContracts(m *Module, cs ContractSet) {
	for _, f := range m.Funcs {
		ch, ok := cs[f.Name]
		if !ok {
			continue
		}
		for i := range f.Contract.Params {
			if i < len(ch.ParamClasses) {
				f.Contract.Params[i].Class = ch.ParamClasses[i]
			}
		}
		// Return classes are NOT overridden here.
		// The lowerer sets them via classForRetPos (coerced at ReturnStmt via Move).
		// Updating return classes without also patching the producing instruction's Cls
		// causes PBQP to allocate to the original class while TermRet codegen generates
		// copies to the (now wrong) contract location — the multi-return clobber bug.
	}
}

// ── Internals ─────────────────────────────────────────────────────────────────

// currentChoice extracts the existing (manually-set) classes from a contract.
func currentChoice(f *Func) *ContractChoice {
	ch := &ContractChoice{
		ParamClasses:  make([]RegClass, len(f.Contract.Params)),
		ReturnClasses: make([]RegClass, len(f.Contract.Returns)),
	}
	for i, p := range f.Contract.Params {
		ch.ParamClasses[i] = p.Class
	}
	for i, r := range f.Contract.Returns {
		ch.ReturnClasses[i] = r.Class
	}
	return ch
}

// candidateChoices returns all plausible, non-conflicting class vectors for f's contract.
//
// We only enumerate classes physically meaningful for the parameter type, and we
// filter out vectors where two params compete for the same unique physical location
// (e.g. both ClassAcc → both want A, impossible without spilling one).
func candidateChoices(f *Func, ct CostTable) []*ContractChoice {
	paramCandidates := make([][]RegClass, len(f.Contract.Params))
	for i, p := range f.Contract.Params {
		paramCandidates[i] = plausibleClasses(p.Ty, p.Class)
	}
	retCandidates := make([][]RegClass, len(f.Contract.Returns))
	for i, r := range f.Contract.Returns {
		retCandidates[i] = plausibleClasses(r.Ty, r.Class)
	}

	// Cartesian product of all param × return candidate vectors, filtered for conflicts.
	var results []*ContractChoice
	var recurse func(pi, ri int, ch *ContractChoice)
	recurse = func(pi, ri int, ch *ContractChoice) {
		if pi < len(paramCandidates) {
			for _, cls := range paramCandidates[pi] {
				next := ch.clone()
				next.ParamClasses[pi] = cls
				// Reject if this class uniquely forces a physical location
				// already claimed by a preceding param.
				if choiceHasParamConflict(next.ParamClasses[:pi+1], ct) {
					continue
				}
				recurse(pi+1, ri, next)
			}
			return
		}
		if ri < len(retCandidates) {
			for _, cls := range retCandidates[ri] {
				next := ch.clone()
				next.ReturnClasses[ri] = cls
				recurse(pi, ri+1, next)
			}
			return
		}
		results = append(results, ch)
	}
	base := &ContractChoice{
		ParamClasses:  make([]RegClass, len(paramCandidates)),
		ReturnClasses: make([]RegClass, len(retCandidates)),
	}
	if len(paramCandidates) == 0 && len(retCandidates) == 0 {
		return []*ContractChoice{base}
	}
	recurse(0, 0, base)
	return results
}

// choiceHasParamConflict reports whether any two classes in the prefix share
// a uniquely-forced physical location (e.g. two ClassAcc both want A).
// Classes with multiple cost-0 locations (ClassGeneral: C/D/E/H/L) never conflict.
func choiceHasParamConflict(classes []RegClass, ct CostTable) bool {
	used := make(map[string]bool, len(classes))
	for _, cls := range classes {
		loc, forced := uniqueForcedLoc(cls, ct)
		if !forced {
			continue // multiple options — allocator will pick different regs
		}
		if used[loc] {
			return true
		}
		used[loc] = true
	}
	return false
}

// uniqueForcedLoc returns (locName, true) if cls has exactly one cost-0 physical
// location (meaning the allocator is forced to use that specific register).
// Returns ("", false) if cls has multiple cost-0 options (e.g. ClassGeneral).
func uniqueForcedLoc(cls RegClass, ct CostTable) (string, bool) {
	var zeros []string
	for _, loc := range ct.Locs() {
		if ct.Cost(cls, loc) == 0 {
			zeros = append(zeros, loc.Name)
		}
	}
	if len(zeros) == 1 {
		return zeros[0], true
	}
	return "", false
}

func (ch *ContractChoice) clone() *ContractChoice {
	p := make([]RegClass, len(ch.ParamClasses))
	r := make([]RegClass, len(ch.ReturnClasses))
	copy(p, ch.ParamClasses)
	copy(r, ch.ReturnClasses)
	return &ContractChoice{ParamClasses: p, ReturnClasses: r}
}

// plausibleClasses returns the candidate register classes to try for a slot
// of the given type.  The current class is always included.
// Flag-return slots are kept fixed (not enumerated) because the flag-return
// ABI is a semantic choice, not a cost-optimisation choice.
func plausibleClasses(ty Ty, current RegClass) []RegClass {
	if current == ClassFlag {
		return []RegClass{ClassFlag} // flag-return is fixed
	}
	switch ty {
	case TyU8, TyI8, TyBool:
		return []RegClass{ClassAcc, ClassCounter, ClassGeneral}
	case TyU16, TyI16:
		return []RegClass{ClassPointer, ClassIndex, ClassPair}
	case TyPtr:
		return []RegClass{ClassPointer, ClassIndex}
	default:
		return []RegClass{current}
	}
}

// unaryCost is the adapter cost a function pays internally when its param is
// in class `choice` rather than the class that the function body "naturally"
// wants (inferred from how the param register is first used).
//
// For example: if a param is used as the left operand of ADD A,x the body
// wants ClassAcc.  If we assign ClassGeneral (C), the body must emit LD A,C
// before the ADD — a 4T adapter.
func unaryCost(f *Func, choice *ContractChoice, ct CostTable) int {
	return unaryCostWithMod(f, choice, ct, f.mod)
}

func unaryCostWithMod(f *Func, choice *ContractChoice, ct CostTable, m *Module) int {
	total := 0
	for i, p := range f.Contract.Params {
		if i >= len(choice.ParamClasses) {
			break
		}
		natural := inferNaturalClass(f, p.Reg, m)
		chosen := choice.ParamClasses[i]
		if natural != chosen {
			total += classMoveCost(chosen, natural, ct)
		}
	}
	// Return values: if the body computes a value in one class but the return
	// contract asks for another, there's an adapter move before every RET.
	// We approximate: if the chosen return class differs from ClassAcc (the
	// default ALU output), add one move cost per return site.
	retSites := countRetSites(f)
	for i, r := range f.Contract.Returns {
		if i >= len(choice.ReturnClasses) {
			break
		}
		chosen := choice.ReturnClasses[i]
		natural := inferNaturalRetClass(f, r)
		if natural != chosen {
			total += classMoveCost(natural, chosen, ct) * retSites
		}
	}
	return total
}

// incomingEdgeCost sums the move costs that all callers of f in m would pay
// if f adopted the given contract choice.
//
// This is the "reverse" of edgeCost: edgeCost asks "how much does f pay to call
// its callees?"; incomingEdgeCost asks "how much do all of f's callers pay to
// call f?"
//
// Used in OptimizeContracts pass 1+ so that a callee with N callers can pull
// an adapter move back into its body when N×moveCost > bodyMoveCost.
func incomingEdgeCost(f *Func, choice *ContractChoice, m *Module, cs ContractSet, ct CostTable) int {
	total := 0
	for _, caller := range m.Funcs {
		if caller == f {
			continue
		}
		callerChoice, hasCaller := cs[caller.Name]
		if !hasCaller {
			callerChoice = currentChoice(caller)
		}
		ri := collectRegInfo(caller)

		// For caller's own params, use the caller's chosen class (not the
		// raw contract which hasn't been Apply'd yet).
		paramOverride := make(map[Reg]RegClass, len(caller.Contract.Params))
		for i, p := range caller.Contract.Params {
			if i < len(callerChoice.ParamClasses) {
				paramOverride[p.Reg] = callerChoice.ParamClasses[i]
			}
		}

		for _, b := range caller.Blocks {
			for _, inst := range b.Insts {
				if inst.Op != OpCall || inst.Sym != f.Name {
					continue
				}
				for ai, arg := range inst.Args {
					if ai >= len(choice.ParamClasses) {
						break
					}
					var argClass RegClass
					if cls, isParam := paramOverride[arg]; isParam {
						argClass = cls
					} else if argInfo, haveInfo := ri[arg]; haveInfo {
						argClass = argInfo.Cls
					} else {
						argClass = ClassGeneral
					}
					wantClass := choice.ParamClasses[ai]
					if argClass != wantClass {
						total += classMoveCost(argClass, wantClass, ct)
					}
				}
			}
		}
	}
	return total
}

// choiceEqual reports whether two ContractChoices are identical.
func choiceEqual(a, b *ContractChoice) bool {
	if len(a.ParamClasses) != len(b.ParamClasses) || len(a.ReturnClasses) != len(b.ReturnClasses) {
		return false
	}
	for i := range a.ParamClasses {
		if a.ParamClasses[i] != b.ParamClasses[i] {
			return false
		}
	}
	for i := range a.ReturnClasses {
		if a.ReturnClasses[i] != b.ReturnClasses[i] {
			return false
		}
	}
	return true
}

// edgeCost is the sum of move costs at all call sites from f to its callees,
// given that the callees' contracts are already in cs (DP order).
//
// When an arg is one of f's own param regs, we use the candidate class from
// `choice` (not the regInfo class), since we're evaluating what it would cost
// if f adopted this candidate contract.
func edgeCost(f *Func, choice *ContractChoice, cg *CallGraph, cs ContractSet, ct CostTable) int {
	total := 0
	ri := collectRegInfo(f)

	// For f's own param regs, the candidate class overrides the regInfo class.
	paramOverride := make(map[Reg]RegClass, len(f.Contract.Params))
	for i, p := range f.Contract.Params {
		if i < len(choice.ParamClasses) {
			paramOverride[p.Reg] = choice.ParamClasses[i]
		}
	}

	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op != OpCall || inst.Sym == "" || inst.Sym[0] == '@' {
				continue
			}
			calleeChoice, ok := cs[inst.Sym]
			if !ok {
				continue // callee not yet decided (or external)
			}
			for ai, arg := range inst.Args {
				if ai >= len(calleeChoice.ParamClasses) {
					break
				}
				// What class does the caller provide for this arg?
				var argClass RegClass
				if cls, isParam := paramOverride[arg]; isParam {
					argClass = cls // use candidate class for f's own params
				} else if argInfo, haveInfo := ri[arg]; haveInfo {
					argClass = argInfo.Cls
				} else {
					argClass = ClassGeneral
				}
				calleeClass := calleeChoice.ParamClasses[ai]
				if argClass != calleeClass {
					total += classMoveCost(argClass, calleeClass, ct)
				}
			}
		}
	}
	return total
}

// ── Class inference helpers ───────────────────────────────────────────────────

// inferNaturalClass infers the register class that a function body "wants"
// for virtual register r, based on its first use in the function body.
//
// Heuristic (scan blocks in order):
//   - First src operand of ALU op (ADD/SUB/AND/OR/XOR/CP) → ClassAcc
//   - Pointer operand of Load/Store/Field/PtrBump → ClassPointer
//   - Loop counter operand of TermDJNZ → ClassCounter
//   - Arg[i] of OpCall to a known callee → callee.Contract.Params[i].Class
//   - Otherwise → ClassGeneral
func inferNaturalClass(f *Func, r Reg, m *Module) RegClass {
	return inferNaturalClassSeen(f, r, m, map[Reg]bool{})
}

// inferNaturalClassSeen is the cycle-safe implementation of inferNaturalClass.
// The seen map prevents infinite recursion in degenerate (non-SSA) graphs.
func inferNaturalClassSeen(f *Func, r Reg, m *Module, seen map[Reg]bool) RegClass {
	if seen[r] {
		return ClassGeneral
	}
	seen[r] = true
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			src := inst.Src
			switch inst.Op {
			case OpAdd, OpSub, OpAnd, OpOr, OpXor, OpCmp, OpMul:
				if len(src) > 0 && src[0] == r {
					return ClassAcc
				}
				// Second ALU operand: 16-bit → ClassIndex (DE in SBC HL,DE);
				// 8-bit → ClassGeneral (any register allowed as rhs).
				// Checking src[1] before the OpMove case prevents a spurious
				// ClassPointer inference when r is both a comparison rhs AND
				// the source of a move that feeds a TermCondRet return slot.
				if len(src) > 1 && src[1] == r {
					if inst.Ty == TyU16 || inst.Ty == TyI16 || inst.Ty == TyPtr {
						return ClassIndex
					}
					return ClassGeneral
				}
			case OpLoad:
				if len(src) > 0 && src[0] == r {
					return ClassPointer
				}
			case OpStore:
				if len(src) > 0 && src[0] == r {
					return ClassPointer
				}
			case OpField, OpPtrBump, OpPtrAdd:
				if len(src) > 0 && src[0] == r {
					return ClassPointer
				}
			case OpMove:
				// Follow the move chain: the natural class of r is whatever
				// class the move destination naturally wants.  This handles
				// swap-style functions where a param is returned in a
				// different slot (e.g. %r4 = move %r1; ret %r3, %r4 →
				// %r1 naturally wants the class of the second return slot).
				if len(src) > 0 && src[0] == r {
					return inferNaturalClassSeen(f, inst.Dst, m, seen)
				}
			case OpCall:
				// If r is passed as arg[i] to a known callee, the natural
				// class is whatever the callee expects at that position.
				if m != nil && inst.Sym != "" && inst.Sym[0] != '@' {
					callee := m.FuncByName(inst.Sym)
					if callee != nil {
						for ai, arg := range inst.Args {
							if arg == r && ai < len(callee.Contract.Params) {
								return callee.Contract.Params[ai].Class
							}
						}
					}
				}
			}
		}
		// Check terminators for natural-class inference.
		switch t := b.Term.(type) {
		case *TermDJNZ:
			if t.Counter == r {
				return ClassCounter
			}
		case *TermRet:
			// If r is returned in slot i, it naturally wants the class of
			// f.Contract.Returns[i] (e.g. slot 0 = HL = ClassPointer).
			for i, v := range t.Vals {
				if v == r && i < len(f.Contract.Returns) {
					return f.Contract.Returns[i].Class
				}
			}
		case *TermCondRet:
			for i, v := range t.Vals {
				if v == r && i < len(f.Contract.Returns) {
					return f.Contract.Returns[i].Class
				}
			}
		}
	}
	return ClassGeneral
}

// inferNaturalRetClass infers the class that the function body naturally
// produces for return value r (i.e., what the allocator would choose without
// a contract constraint).  Most ALU results land in ClassAcc.
func inferNaturalRetClass(f *Func, ret Return) RegClass {
	// For flag-return, it's always ClassFlag.
	if ret.Class == ClassFlag {
		return ClassFlag
	}
	// For 16-bit returns (pointers), it's typically ClassPointer.
	if ret.Ty == TyPtr || ret.Ty == TyU16 || ret.Ty == TyI16 {
		return ClassPointer
	}
	// 8-bit: ALU results land in A → ClassAcc.
	return ClassAcc
}

// countRetSites counts the number of TermRet blocks in f.
func countRetSites(f *Func) int {
	n := 0
	for _, b := range f.Blocks {
		if _, ok := b.Term.(*TermRet); ok {
			n++
		}
	}
	if n == 0 {
		return 1 // at least 1 to avoid multiplying by 0
	}
	return n
}

// ── Move cost helpers ─────────────────────────────────────────────────────────

// ClassMoveCost returns the cost of moving a value from a physical location
// best-suited for class `from` into the physical location expected by class `to`.
//
// This is the fundamental "edge cost" in the PBQP formulation.
// It uses the CostTable to stay target-neutral.
// Exported for testing.
func ClassMoveCost(from, to RegClass, ct CostTable) int {
	return classMoveCost(from, to, ct)
}

func classMoveCost(from, to RegClass, ct CostTable) int {
	if from == to {
		return 0
	}
	fromLoc := bestLocForClass(from, ct)
	if fromLoc == (PhysLoc{}) {
		return InfCost
	}
	return ct.Cost(to, fromLoc)
}

// BestLocForClass returns the cheapest physical location for the given class.
// Exported for testing.
func BestLocForClass(cls RegClass, ct CostTable) PhysLoc { return bestLocForClass(cls, ct) }

// bestLocForClass returns the cheapest physical location for the given class.
func bestLocForClass(cls RegClass, ct CostTable) PhysLoc {
	best, bestCost := PhysLoc{}, InfCost
	for _, loc := range ct.Locs() {
		if c := ct.Cost(cls, loc); c < bestCost {
			bestCost, best = c, loc
		}
	}
	return best
}
