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
// re-solved for — the function spills somewhere, or the constant is live
// across a call and so pays PUSH/POP for that home (fold_max's two constants
// cost 50 T per element exactly that way).  Where neither holds, the home is a
// free register for the whole function and rematerialising it cannot relieve
// anything; the re-solve then only re-orders unrelated tie-breaks.  That is
// not a win we can claim, and it measurably is not one: p1.nanz's `hu`
// (straight-line, no spills, no calls) kept its constants' LDs suppressed on
// main but re-tied its parameters when they stopped competing for HL, paying
// 21 T of PUSH/POP per call for a compare operand that must survive, plus a
// 42 T stack swap in the subtract arm — 5203 -> 9074 T over 63 calls.
func constRematCandidates(f *Func, ar *AllocResult, lr *LivenessResult) map[Reg]bool {
	dead := computeDeadConsts(f, ar)
	if len(dead) == 0 {
		return nil
	}
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
	spilled := len(ar.Spilled) > 0
	liveAcrossCall := liveAcrossCallRegs(f, lr)
	var candidates map[Reg]bool
	for r, isDead := range dead {
		if !isDead || constDefs[r] != 1 || blockParam[r] {
			continue
		}
		if loc, ok := ar.Locs[r]; !ok || loc.Kind == LocFlag || loc.Kind == LocDWord {
			continue
		}
		if !spilled && !liveAcrossCall[r] {
			continue
		}
		if candidates == nil {
			candidates = make(map[Reg]bool, len(dead))
		}
		candidates[r] = true
	}
	return candidates
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
