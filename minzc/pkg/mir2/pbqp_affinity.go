package mir2

// Phase 6e: PBQP affinity nudges for correlated-allocation patterns.
//
// This file implements pre-PBQP node cost adjustments that model correlated
// register assignment decisions — cases where the optimal allocation of one
// virtual register depends on the allocation of another.
//
// Unlike full PBQP edge costs (which require modifying the R1 reduction rule),
// these nudges are applied directly to the node cost vectors before the solver
// runs.  Each nudge lowers the cost for a specific (reg, loc) pair by a fixed
// reward.  The effect is equivalent to a PBQP edge cost in the common case
// where the influenced register has low degree (R0/R1 resolved).
//
// Current patterns
// ────────────────
//
//  BC★ / DE★ — page-aligned LUT index
//    Detected: OpExt(src8, u8→u16) → idx16 → OpPtrAdd(base, idx16) → OpLoad
//    Reward:   lower src8's cost for C by LUTBCStarReward (4T)
//              lower src8's cost for E by LUTBCStarReward (4T)
//    Rationale: if src8 ends up in C, codegen emits LD B, sym^H + LD A,(BC) = 14T
//               instead of LD H,sym^H + LD L,src8 + LD A,(HL) = 18T.
//
//  mul16 rhs → DE
//    Detected: OpMul with 16-bit type; Src[1] is the multiplier (rhs)
//    Reward:   lower rhs's cost for DE by Mul16DEReward (8T)
//    Rationale: genMul16 fast path skips LD D,high(rhs); LD E,low(rhs) = 8T
//               when rhs is already in DE.
//
//  DJNZ counter → B
//    Detected: TermDJNZ.Counter
//    Reward:   lower counter's cost for B by DJNZCounterReward (4T)
//    Rationale: DJNZ requires the counter in B.  If the counter is in another
//               register (e.g. C from ClassGeneral), the codegen emits LD B,r.
//               The nudge biases allocation toward B, eliminating that move.

// Mul16DEReward is the T-state savings when the mul16 rhs is already in DE:
//
//	rhs NOT in DE: LD D, high(rhs) (4T) + LD E, low(rhs) (4T) = 8T overhead
//	rhs in DE:     no setup needed = 0T overhead
//	Savings: 8T
const Mul16DEReward = 8

// DJNZCounterReward is the T-state savings when the DJNZ counter is already in B:
//
//	counter NOT in B: LD B, counter (4T)
//	counter in B:     no move needed = 0T
//	Savings: 4T
const DJNZCounterReward = 4

// LUTBCStarReward is the T-state savings from the BC★/DE★ optimisation:
//
//	HL path:  LD H,sym^H (7T) + LD L,idx8 (4T) + LD A,(HL) (7T) = 18T
//	BC★ path: LD B,sym^H (7T) +                  LD A,(BC) (7T) = 14T
//	Savings:  4T
const LUTBCStarReward = 4

// applyLUTAffinityNudge scans f for page-aligned LUT access patterns and
// lowers the node cost for the 8-bit index register toward C and E by
// LUTBCStarReward.  This biases the allocator toward the BC★/DE★ fast path.
//
// The scan does NOT check whether the symbol is actually page-aligned; that
// check is deferred to codegen (scanLUTPatterns / isGlobalPageAligned).  A
// false-positive nudge toward C/E is harmless: C and E are valid general-
// purpose registers and the nudge is small (4T ≪ spill cost).
func applyLUTAffinityNudge(f *Func, states map[Reg]*regState, allLocs []PhysLoc) {
	// Find the index into allLocs for "C" and "E".
	idxC, idxE := -1, -1
	for i, loc := range allLocs {
		if loc.Kind == LocReg {
			switch loc.Name {
			case "C":
				idxC = i
			case "E":
				idxE = i
			}
		}
	}
	if idxC < 0 && idxE < 0 {
		return // no C or E location available (shouldn't happen on Z80)
	}

	// Collect the set of src8Reg candidates from LUT patterns.
	lut8Regs := collectLUTSrc8Regs(f)

	// Apply reward: lower cost for C and E.
	for src8 := range lut8Regs {
		rs, ok := states[src8]
		if !ok {
			continue
		}
		if idxC >= 0 && rs.costs[idxC] < InfCost {
			if rs.costs[idxC] >= LUTBCStarReward {
				rs.costs[idxC] -= LUTBCStarReward
			} else {
				rs.costs[idxC] = 0
			}
		}
		if idxE >= 0 && rs.costs[idxE] < InfCost {
			if rs.costs[idxE] >= LUTBCStarReward {
				rs.costs[idxE] -= LUTBCStarReward
			} else {
				rs.costs[idxE] = 0
			}
		}
	}
}

// collectLUTSrc8Regs returns the set of 8-bit index virtual registers that
// participate in a LUT access pattern within f.
//
// Pattern (mirrors scanLUTPatterns in z80codegen.go):
//
//	src8   → (OpExt u8→u16) → idx16
//	base   = (OpAddrOf sym)
//	ptr    = (OpPtrAdd base, idx16)
//	dst    = (OpLoad ptr, u8)
func collectLUTSrc8Regs(f *Func) map[Reg]struct{} {
	result := make(map[Reg]struct{})

	for _, b := range f.Blocks {
		// Build reg→def map for this block.
		defInst := make(map[Reg]*Inst, len(b.Insts))
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				defInst[inst.Dst] = inst
			}
		}

		for _, inst := range b.Insts {
			// Need: OpLoad of an 8-bit value.
			if inst.Op != OpLoad || inst.Ty.Width() != 8 || inst.Dst == NoReg {
				continue
			}
			ptrReg := inst.Src[0]

			// ptr = PtrAdd(base, idx16)
			ptrInst, ok := defInst[ptrReg]
			if !ok || ptrInst.Op != OpPtrAdd {
				continue
			}
			idx16Reg := ptrInst.Src[1]

			// idx16 = Ext(src8, u8→u16)
			extInst, ok := defInst[idx16Reg]
			if !ok || extInst.Op != OpExt {
				continue
			}
			if extInst.SrcTy == nil || extInst.SrcTy.Width() != 8 || extInst.Ty.Width() != 16 {
				continue
			}
			result[extInst.Src[0]] = struct{}{}
		}
	}
	return result
}

// applyMul16DEAffinityNudge scans f for 16-bit multiply instructions and
// lowers the node cost for the rhs (multiplier) register toward DE by
// Mul16DEReward.  This biases the allocator to place the multiplier in DE
// so genMul16 can skip the LD D,h; LD E,l setup.
func applyMul16DEAffinityNudge(f *Func, states map[Reg]*regState, allLocs []PhysLoc) {
	idxDE := -1
	for i, loc := range allLocs {
		if loc.Kind == LocReg && loc.Name == "DE" {
			idxDE = i
			break
		}
	}
	if idxDE < 0 {
		return
	}

	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op != OpMul || inst.Ty.Width() != 16 || len(inst.Src) < 2 {
				continue
			}
			rhs := inst.Src[1]
			rs, ok := states[rhs]
			if !ok {
				continue
			}
			if rs.costs[idxDE] < InfCost {
				if rs.costs[idxDE] >= Mul16DEReward {
					rs.costs[idxDE] -= Mul16DEReward
				} else {
					rs.costs[idxDE] = 0
				}
			}
		}
	}
}

// applyDJNZCounterAffinityNudge scans f for TermDJNZ terminators and lowers
// the node cost for the Counter register toward B by DJNZCounterReward.
// This biases the allocator to place the counter in B so the DJNZ codegen
// can skip the LD B, counter move.
func applyDJNZCounterAffinityNudge(f *Func, states map[Reg]*regState, allLocs []PhysLoc) {
	idxB := -1
	for i, loc := range allLocs {
		if loc.Kind == LocReg && loc.Name == "B" {
			idxB = i
			break
		}
	}
	if idxB < 0 {
		return
	}

	for _, b := range f.Blocks {
		t, ok := b.Term.(*TermDJNZ)
		if !ok {
			continue
		}
		rs, ok := states[t.Counter]
		if !ok {
			continue
		}
		if rs.costs[idxB] < InfCost {
			if rs.costs[idxB] >= DJNZCounterReward {
				rs.costs[idxB] -= DJNZCounterReward
			} else {
				rs.costs[idxB] = 0
			}
		}
	}
}

