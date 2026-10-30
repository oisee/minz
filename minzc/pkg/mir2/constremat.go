package mir2

import (
	"maps"
	"slices"
)

// Constant rematerialisation (F6).
//
// An OpConst vreg whose every use the Z80 codegen serves from an immediate
// (CP n, INC/DEC rr, BIT n, repeated shifts, …) never has its home read.  The
// allocator nevertheless reserved a physical location for its whole live range,
// so a loop-invariant 0 or 1 occupied BC/IX/IY across a loop, was PUSHed and
// POPped around every call inside it (fold_max: PUSH BC + PUSH IX = 50 T per
// element for two constants nothing reads), and starved the loop of a pair
// (filter_count: the hoisted addr_of @k spilled to $F0xx because the two index
// registers were taken by unread constants).
//
// Rematerialisation gives such a constant LocRemat — no physical location at
// all.  It is only ever applied to constants whose LD computeDeadConsts already
// suppresses, because that analysis is the inventory of use sites the codegen
// serves without touching the constant's allocation.
//
// Soundness depends on the *final* allocation, not the trial one: a folding
// pattern may require its other operand to be in a particular kind of location
// (a pair or an 8-bit register), and re-solving with the constants removed can
// move that operand.  PBQPAllocate therefore re-runs the analysis on the
// rematerialised allocation and drops any constant that is no longer fully
// served from immediates, repeating until the set is stable.

// z80ConstantRematTable is implemented by cost tables that drive the Z80 code
// generator, where computeDeadConsts describes how constants are consumed.
type z80ConstantRematTable interface {
	ConstantRemat() bool
}

// constRematSupported reports whether ct's consumer can serve constant uses
// from immediates.  Other targets (m6502, …) keep the previous behaviour.
func constRematSupported(ct CostTable) bool {
	t, ok := ct.(z80ConstantRematTable)
	return ok && t.ConstantRemat()
}

// repairRematerialisedConstants re-validates every LocRemat entry of ar against
// the allocation ar actually describes and returns an allocation in which each
// broken promise holds again.
//
// LocRemat is a promise about the *rest* of the allocation: "no use of this
// constant reads a home".  The allocator verified it, but callers legitimately
// edit AllocResult.Locs afterwards (the pipeline's callers and several test
// fixtures force spill homes that way), and whether a use is served from an
// immediate can depend on where the *other* operand ended up — a compare
// against zero folds only while its left operand lives in a pair.  When the
// promise no longer holds, the constant gets a real memory home rather than a
// non-existent one, so the emitted code computes the right value instead of
// naming a location nothing backs.
func repairRematerialisedConstants(m *Module, ar *AllocResult) *AllocResult {
	if ar == nil {
		return ar
	}
	broken := make(map[Reg]bool)
	for _, f := range m.Funcs {
		if !funcHasRematLoc(f, ar) {
			continue
		}
		dead := computeDeadConsts(f, ar)
		for r := range collectRegInfo(f) {
			if ar.Locs[r].Kind == LocRemat && !dead[r] {
				broken[r] = true
			}
		}
	}
	if len(broken) == 0 {
		return ar
	}
	repaired := &AllocResult{Locs: maps.Clone(ar.Locs), Spilled: slices.Clone(ar.Spilled)}
	for r := range broken {
		repaired.Locs[r] = PhysLoc{Kind: LocMem, Name: "mem"}
		repaired.Spilled = append(repaired.Spilled, r)
	}
	return repaired
}

// funcHasRematLoc reports whether any vreg of f is currently rematerialised.
func funcHasRematLoc(f *Func, ar *AllocResult) bool {
	for r := range collectRegInfo(f) {
		if ar.Locs[r].Kind == LocRemat {
			return true
		}
	}
	return false
}

// funcHasConstDefs reports whether f defines any constant at all, so functions
// without constants do not pay the trial solve.
func funcHasConstDefs(f *Func) bool {
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpConst && inst.Dst != NoReg {
				return true
			}
		}
	}
	return false
}

// constRematCandidates returns the OpConst vregs of f that a trial allocation
// ar shows are never read out of a home: exactly one definition, not a block
// parameter (the codegen re-defines those per iteration and does not record
// them in constVals), and not in a location with its own materialisation path
// (LocFlag emits SCF/AND A, LocDWord drives an EXX pair).  A constant already
// spilled to LocMem is still a candidate: its store is suppressed too, so it
// only wastes a slot and an interference-graph node.
//
// Profitability: only constants whose reserved home is actually contested are
// re-solved for.  A reserved home is contested when one of three things holds:
//
//   - the function spills somewhere (the home is a register a live value needs);
//   - the constant is live across a call, so a callee clobber set forces
//     PUSH/POP for that home (fold_max's two constants cost 50 T per element
//     exactly that way);
//   - indexHalfBlockers found a vreg the reserved homes pushed into an index
//     half.  The constant itself is never read, so its own cost is zero, but a
//     pair blocks both of its bytes for the whole function (alloc.go
//     physicalAliases), and in fold_max that is the entire difference between a
//     byte loop element costing 0 (C) and 24 (IYH).
//
// Where none holds, the home is a free register for the whole function and
// rematerialising it cannot relieve anything; the re-solve then only re-orders
// unrelated tie-breaks, which is a coin flip with no basis behind it — and a
// measurably bad one: p1.nanz's `hu` (straight-line, no spills, no calls)
// re-tied its parameters when its constants stopped competing for HL, paying
// 21 T of PUSH/POP per call plus a 42 T stack swap, 5203 -> 9074 T over 63
// calls.
//
// The first two clauses are decided per constant, exactly as F6 decided them:
// a constant that is itself neither spilled-for nor live across a call keeps
// its home even when a sibling qualifies.  Only the third clause admits the
// whole eligible set, because which constants end up squatting on the pair the
// blocked byte needs is not known until the re-solve.
//
// The second return value names the vregs the third clause justified the
// re-solve with, so the caller can check the re-solve actually helped them.  It
// is nil when the first two clauses did the admitting.
func constRematCandidates(f *Func, ar *AllocResult, lr *LivenessResult, ct CostTable, costs map[Reg][]int, locs []PhysLoc) (map[Reg]bool, map[Reg]bool) {
	dead := computeDeadConsts(f, ar)
	if len(dead) == 0 {
		return nil, nil
	}
	eligible := eligibleRematConstants(f, ar, dead)
	spilled := len(ar.Spilled) > 0
	var blockers map[Reg]bool
	if !spilled {
		blockers = indexHalfBlockers(f, ar, lr, eligible, costs, locs)
	}
	liveAcrossCall := liveAcrossCallRegs(f, lr)
	var candidates map[Reg]bool
	for r := range eligible {
		if !spilled && !liveAcrossCall[r] && len(blockers) == 0 {
			continue
		}
		if candidates == nil {
			candidates = make(map[Reg]bool, len(eligible))
		}
		candidates[r] = true
	}
	return candidates, blockers
}

// eligibleRematConstants filters computeDeadConsts' output down to the
// constants that may hold no home: exactly one definition, not a block
// parameter, and not in a location with its own materialisation path.
func eligibleRematConstants(f *Func, ar *AllocResult, dead map[Reg]bool) map[Reg]bool {
	blockParam := make(map[Reg]bool, len(f.Blocks))
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			blockParam[p.Dst] = true
		}
	}
	constDefs := make(map[Reg]int)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpConst && inst.Dst != NoReg {
				constDefs[inst.Dst]++
			}
		}
	}
	eligible := make(map[Reg]bool, len(dead))
	for r, isDead := range dead {
		if !isDead || constDefs[r] != 1 || blockParam[r] {
			continue
		}
		if loc, ok := ar.Locs[r]; !ok || loc.Kind == LocFlag || loc.Kind == LocDWord {
			continue
		}
		eligible[r] = true
	}
	return eligible
}

// indexHalfBlockers returns the live vregs that the eligible dead constants'
// reserved homes pushed into an index half: the trial parked v in IXH/IXL/
// IYH/IYL although v's own cost vector prices a primary register strictly
// lower, that register is one of the constants' homes or its aliases, and it is
// free of every neighbour that is not itself a dead constant.
//
// Only index halves count.  An index half is a second-class home for a byte:
// no `LD IYH,(HL)` exists, so every load routes through A, and the emitter
// brackets it with an `EX AF,AF'` pair.  A displacement *within* the primary
// file (HL to BC, or BC to DE) does not count — that is a tie the model has no
// basis to re-open, and re-opening it is exactly the `hu` regression above.  A
// full index register does not count either: for a 16-bit value, and above all
// for a walking pointer that is dereferenced through `(IX+d)` and advanced with
// `INC IX`, IX and IY are first-class homes, and `costPointer`/`costGeneral`
// price them only a few T above a primary pair.  Treating such a displacement
// as a downgrade made the model prefer BC for a pointer the emitter still had
// to reach through IX, at 28 extra T per iteration
// (TestTwoWalkingPointerLoopRegisterChoice).
func indexHalfBlockers(f *Func, ar *AllocResult, lr *LivenessResult, eligible map[Reg]bool, costs map[Reg][]int, locs []PhysLoc) map[Reg]bool {
	if len(eligible) == 0 {
		return nil
	}
	if lr == nil {
		lr = ComputeLiveness(f)
	}
	idxOf := locIndex(locs)

	// Every location the eligible constants' homes make unavailable: the home
	// itself plus its pair/dword aliases.
	reserved := make(map[int]bool, len(eligible)*3)
	for r := range eligible {
		i, ok := idxOf[ar.Locs[r]]
		if !ok {
			continue
		}
		reserved[i] = true
		for _, alias := range physicalAliases(locs[i]) {
			if j, ok := idxOf[alias]; ok {
				reserved[j] = true
			}
		}
	}

	ig := BuildInterferenceGraph(f, lr)
	var blockers map[Reg]bool
	for v, cv := range costs {
		if eligible[v] {
			continue
		}
		homeIdx, ok := idxOf[ar.Locs[v]]
		if !ok || homeIdx >= len(cv) || cv[homeIdx] >= InfCost {
			continue // LocRemat, a spill offset, or no cost vector
		}
		if locs[homeIdx].Kind != LocIXY8 {
			continue // not pushed out of the primary register file
		}
		homeCost := cv[homeIdx]
		for i, c := range cv {
			if locs[i].Kind != LocReg || c >= homeCost {
				// Not a primary register, or not strictly cheaper than where
				// the trial actually put v.
				continue
			}
			if !reserved[i] {
				continue // nothing the constants hold back
			}
			if !locBlockedByNeighbors(ig, v, locs[i], ar, eligible) {
				if blockers == nil {
					blockers = make(map[Reg]bool, 4)
				}
				blockers[v] = true
				break
			}
		}
	}
	return blockers
}

// locBlockedByNeighbors reports whether any neighbour of v that is not in skip
// is allocated to a location overlapping loc.
func locBlockedByNeighbors(ig *InterferenceGraph, v Reg, loc PhysLoc, ar *AllocResult, skip map[Reg]bool) bool {
	blocked := false
	ig.Neighbors(v).Each(func(n Reg) {
		if blocked || skip[n] {
			return
		}
		// Both directions: physicalAliases is not symmetric (aliases(BC) names
		// B and C, aliases(B) names BC), so a pair and either of its bytes
		// overlap only when tested each way.
		if physicallyConflicts(loc, ar.Locs[n]) || physicallyConflicts(ar.Locs[n], loc) {
			blocked = true
		}
	})
	return blocked
}

// locIndex maps each of the cost table's locations to its position, so an
// allocation can be read back as a cost-vector index.
func locIndex(locs []PhysLoc) map[PhysLoc]int {
	idx := make(map[PhysLoc]int, len(locs))
	for i, l := range locs {
		idx[l] = i
	}
	return idx
}
// liveAcrossCallRegs returns the vregs whose live range covers a call, i.e. the
// ones a callee's clobber set forces the caller to PUSH/POP.
func liveAcrossCallRegs(f *Func, lr *LivenessResult) map[Reg]bool {
	out := map[Reg]bool{}
	for bi, b := range f.Blocks {
		callAt := -1
		for i, inst := range b.Insts {
			if inst.Op == OpCall || inst.Op == OpCallIndirect {
				callAt = i
				break
			}
		}
		if callAt < 0 {
			continue
		}
		if lr != nil && bi < len(lr.LiveOut) {
			lr.LiveOut[bi].Each(func(r Reg) { out[r] = true })
		}
		// Live-in or defined before the call, and read after it.
		before := map[Reg]bool{}
		if lr != nil && bi < len(lr.LiveIn) {
			lr.LiveIn[bi].Each(func(r Reg) { before[r] = true })
		}
		for _, inst := range b.Insts[:callAt+1] {
			if inst.Dst != NoReg {
				before[inst.Dst] = true
			}
		}
		for _, inst := range b.Insts[callAt+1:] {
			for _, r := range inst.Uses() {
				if before[r] {
					out[r] = true
				}
			}
		}
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				if before[r] {
					out[r] = true
				}
			}
		}
	}
	return out
}
