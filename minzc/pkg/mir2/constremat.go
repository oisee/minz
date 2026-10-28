package mir2

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
func constRematCandidates(f *Func, ar *AllocResult) map[Reg]bool {
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
	var candidates map[Reg]bool
	for r, isDead := range dead {
		if !isDead || constDefs[r] != 1 || blockParam[r] {
			continue
		}
		if loc, ok := ar.Locs[r]; !ok || loc.Kind == LocFlag || loc.Kind == LocDWord {
			continue
		}
		if candidates == nil {
			candidates = make(map[Reg]bool, len(dead))
		}
		candidates[r] = true
	}
	return candidates
}
