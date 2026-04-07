package mir2

// FoldConditionalCalls rewrites BrIf → single-call-block → join patterns
// into OpCallCond instructions, eliminating the branch overhead.
//
// Before:
//
//	block_a:
//	    %cond = cmp ...
//	    br_if %cond, @then, @join
//
//	then:
//	    %r = call @target(%args...)
//	    jmp @join
//
//	join:
//	    ...
//
// After:
//
//	block_a:
//	    %cond = cmp ...
//	    %r = call_cc @target(%args...) if cond   [Src[0] = cond reg]
//	    jmp @join
//
//	then:  (dead — will be removed by EliminateDeadBlocks)
//
// The condition is INVERTED from the BrIf: if BrIf branches to @then when
// cond!=0, then call_cc fires when cond!=0 (i.e. the call happens on the
// same condition). The Inst.Cond field carries the comparison condition
// from the OpCmp that produced the cond register, if available.
//
// Constraints:
//   - then-block must have exactly one OpCall instruction (no other side effects)
//   - then-block must terminate with TermJmp to the else-block (no block args)
//   - the call must not return a value used by subsequent instructions
//     (Dst == NoReg, or only used after the join point — simplified: Dst == NoReg for now)
func FoldConditionalCalls(f *Func) int {
	blockMap := make(map[string]*Block, len(f.Blocks))
	for _, b := range f.Blocks {
		blockMap[b.Label] = b
	}

	// Count how many predecessors each block has, for safe dead-block removal.
	predCount := make(map[string]int)
	for _, b := range f.Blocks {
		if b.Term == nil {
			continue
		}
		b.Term.ForEachEdge(func(target string, _ []Reg, _ int) {
			predCount[target]++
		})
	}

	folded := 0
	for _, b := range f.Blocks {
		brif, ok := b.Term.(*TermBrIf)
		if !ok {
			continue
		}

		// Try both branches: then-block or else-block could be the single-call block.
		// The other branch is the join block.
		for _, flip := range []bool{false, true} {
			var callBlockLabel, joinBlockLabel string
			if !flip {
				callBlockLabel = brif.Then
				joinBlockLabel = brif.Else
			} else {
				callBlockLabel = brif.Else
				joinBlockLabel = brif.Then
			}

			callBlock := blockMap[callBlockLabel]
			if callBlock == nil {
				continue
			}

			// Call block must have exactly one instruction: OpCall with no return value.
			if len(callBlock.Insts) != 1 {
				continue
			}
			callInst := callBlock.Insts[0]
			if callInst.Op != OpCall {
				continue
			}
			// For now, only fold void calls (no return value needed conditionally).
			if callInst.Dst != NoReg {
				continue
			}

			// Call block must terminate with TermJmp to the join block (no block args).
			jmp, isJmp := callBlock.Term.(*TermJmp)
			if !isJmp {
				continue
			}
			if jmp.Target != joinBlockLabel {
				continue
			}
			if len(jmp.Args) > 0 {
				continue
			}

			// No block args on the BrIf edges.
			if !flip && len(brif.ThenArgs) > 0 {
				continue
			}
			if flip && len(brif.ElseArgs) > 0 {
				continue
			}

			// Call block must have only one predecessor (this BrIf).
			if predCount[callBlockLabel] != 1 {
				continue
			}

			// Match! Create OpCallCond and rewrite.
			condCall := &Inst{
				Op:   OpCallCond,
				Dst:  NoReg,
				Src:  [2]Reg{brif.Cond, 0},
				Sym:  callInst.Sym,
				Args: callInst.Args,
				Ty:   callInst.Ty,
				Cls:  callInst.Cls,
			}

			// Find the CmpCond from the OpCmp that produced brif.Cond.
			condCall.Cond = findCondForReg(b, brif.Cond)

			// If we're folding the else-branch (flip=true), the call fires
			// when cond==0, so we invert the condition.
			if flip {
				condCall.Cond = invertCmpCond(condCall.Cond)
			}

			b.Insts = append(b.Insts, condCall)

			// Replace BrIf with Jmp to join block.
			var joinArgs []Reg
			if !flip {
				joinArgs = brif.ElseArgs
			} else {
				joinArgs = brif.ThenArgs
			}
			b.Term = &TermJmp{Target: joinBlockLabel, Args: joinArgs}

			// Clear call block (will be removed by EliminateDeadBlocks).
			callBlock.Insts = nil
			callBlock.Term = &TermJmp{Target: joinBlockLabel}

			folded++
			break // don't try the other flip direction
		}
	}

	if folded > 0 {
		EliminateDeadBlocks(f)
	}
	return folded
}

// findCondForReg finds the CmpCond from the OpCmp that defined reg in block b.
func findCondForReg(b *Block, reg Reg) CmpCond {
	for i := len(b.Insts) - 1; i >= 0; i-- {
		inst := b.Insts[i]
		if inst.Op == OpCmp && inst.Dst == reg {
			return inst.Cond
		}
	}
	return CmpNe // default: call when cond != 0
}

// invertCmpCond returns the inverse condition.
func invertCmpCond(c CmpCond) CmpCond {
	switch c {
	case CmpEq:
		return CmpNe
	case CmpNe:
		return CmpEq
	case CmpLt:
		return CmpGe
	case CmpGe:
		return CmpLt
	case CmpGt:
		return CmpLe
	case CmpLe:
		return CmpGt
	case CmpUlt:
		return CmpUge
	case CmpUge:
		return CmpUlt
	case CmpUgt:
		return CmpUle
	case CmpUle:
		return CmpUgt
	}
	return c
}
