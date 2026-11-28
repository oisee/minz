package mir2

// sinkableOps are pure register-to-register instructions: they read only
// their operands, write only Dst, and cannot trap or loop forever.
var sinkableOps = map[Op]bool{
	OpConst: true, OpMove: true,
	OpAdd: true, OpSub: true, OpAnd: true, OpOr: true, OpXor: true,
	OpNot: true, OpNeg: true,
	OpExt: true, OpSext: true, OpTrunc: true,
	OpCmp: true,
}

// SinkIntoSingleUseSuccessor moves a pure byte/bool instruction of block B into the
// start of B's successor S when every use of its result is in S itself
// (instructions or terminator) and S has B as its only predecessor. The other
// successor paths then no longer compute (or hold a register for) a value
// they never read.
//
// Execution count: S runs only after B, at most once per run of B, so the
// instruction never executes more often than before (a single-predecessor
// block cannot be a loop header entered from elsewhere; a self loop S == B is
// excluded). Operands remain available: they dominate B, and B dominates S.
// B's instructions are visited in reverse, so a chain whose last link sinks
// (const k; add x, k) follows it, and the moved instructions keep their order.
//
// Returns true if anything moved.
func SinkIntoSingleUseSuccessor(f *Func) bool {
	preds := map[string]int{}
	for _, b := range f.Blocks {
		for _, s := range b.Successors() {
			preds[s]++
		}
	}
	params := map[Reg]bool{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			params[p.Dst] = true
		}
	}
	// useBlocks[r] = blocks reading r (instructions or terminator).
	useBlocks := map[Reg]map[*Block]bool{}
	note := func(r Reg, b *Block) {
		if r == NoReg {
			return
		}
		if useBlocks[r] == nil {
			useBlocks[r] = map[*Block]bool{}
		}
		useBlocks[r][b] = true
	}
	defs := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for _, r := range inst.Uses() {
				note(r, b)
			}
			if inst.Dst != NoReg {
				defs[inst.Dst]++
			}
			for _, r := range inst.ExtraRets {
				defs[r]++
			}
		}
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				note(r, b)
			}
		}
	}

	changed := false
	for _, b := range f.Blocks {
		if b.Term == nil || len(b.Insts) == 0 {
			continue
		}
		succs := b.Term.Successors()
		if len(succs) < 2 {
			continue // nothing to gain: the value is needed on every path
		}
		var keep []*Inst
		moved := map[*Block][]*Inst{}
		for i := len(b.Insts) - 1; i >= 0; i-- {
			inst := b.Insts[i]
			target := sinkTarget(f, b, inst, succs, preds, params, defs, useBlocks)
			if target == nil {
				keep = append(keep, inst)
				continue
			}
			moved[target] = append([]*Inst{inst}, moved[target]...)
			// The operands are now read in target, not in b (unless b still
			// reads them elsewhere: the per-block sets are recomputed below).
			for _, r := range inst.Uses() {
				note(r, target)
			}
			delete(useBlocks[inst.Dst], b)
			for _, r := range inst.Uses() {
				if !blockReads(b, r, keep, inst) {
					delete(useBlocks[r], b)
				}
			}
			changed = true
		}
		if len(moved) == 0 {
			continue
		}
		for l, r := 0, len(keep)-1; l < r; l, r = l+1, r-1 {
			keep[l], keep[r] = keep[r], keep[l]
		}
		b.Insts = keep
		for target, insts := range moved {
			target.Insts = append(insts, target.Insts...)
		}
	}
	return changed
}

// blockReads reports whether b still reads r: in its terminator, in an
// instruction that stays (kept so far, in reverse order) or in an earlier
// instruction not yet visited.
func blockReads(b *Block, r Reg, keep []*Inst, sunk *Inst) bool {
	for _, u := range b.Term.termUses() {
		if u == r {
			return true
		}
	}
	for _, k := range keep {
		for _, u := range k.Uses() {
			if u == r {
				return true
			}
		}
	}
	for _, inst := range b.Insts {
		if inst == sunk {
			break
		}
		for _, u := range inst.Uses() {
			if u == r {
				return true
			}
		}
	}
	return false
}

// readsPrecedingFlags reports whether inst's meaning depends on the flags
// left by the instruction emitted before it rather than on its operands:
// CmpSubCarry / CmpSubCarryNot (FusionSubCmpInBlock, FuseAbsDiff,
// FuseByteAbsDiffCFG) emit nothing and read the carry of the SUB that
// immediately precedes them. Such an instruction must not be moved away from
// that SUB (HoistDiamondCommon, SinkIntoSingleUseSuccessor).
func readsPrecedingFlags(inst *Inst) bool {
	return inst.Op == OpCmp && (inst.Cond == CmpSubCarry || inst.Cond == CmpSubCarryNot)
}

func sinkTarget(f *Func, b *Block, inst *Inst, succs []string, preds map[string]int, params map[Reg]bool, defs map[Reg]int, useBlocks map[Reg]map[*Block]bool) *Block {
	// Byte and bool results only: sinking a word computation into a block
	// that is already short of pairs measurably added spills
	// (examples/mzv/test_sphere.minz, march).
	if !sinkableOps[inst.Op] || readsPrecedingFlags(inst) || inst.Dst == NoReg || inst.Ty == nil || inst.Ty.Width() > 8 || inst.Volatile || inst.ClsHard || inst.Sym != "" ||
		inst.Asm != nil || len(inst.Args) > 0 || len(inst.ExtraRets) > 0 || inst.SequentialBytes ||
		params[inst.Dst] || defs[inst.Dst] != 1 {
		return nil
	}
	// DJNZ decrements its counter as it branches: an instruction moved past
	// it would read the counter one step later.
	if _, djnz := b.Term.(*TermDJNZ); djnz {
		return nil
	}
	users := useBlocks[inst.Dst]
	if len(users) != 1 || users[b] {
		return nil
	}
	var target *Block
	for u := range users {
		target = u
	}
	isSucc := false
	for _, s := range succs {
		if s == target.Label {
			isSucc = true
		}
	}
	if !isSucc || target == b || preds[target.Label] != 1 || target == f.Blocks[0] {
		return nil
	}
	return target
}
