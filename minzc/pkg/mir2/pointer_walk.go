// pointer_walk.go — Local CSE for repeated ptr_add terms in loop bodies.
//
// When a loop body block contains multiple memory accesses (load/store) that
// compute the same ptr_add(base, offset) independently, this pass replaces
// the duplicates with a single shared pointer register.
//
// This is NOT a true pointer-walk rewrite (that requires threading a walking
// pointer through block args). It is a preparatory CSE-like reshape that
// reduces repeated address construction — a foundation for later pointer-walk
// and row-helper transforms.
//
// Called from RunGracePasses after declarative Grace rules.
package mir2

// PointerWalkStats tracks how many rewrites fired.
type PointerWalkStats struct {
	FuncsVisited int
	BlocksEdited int
	PtrAddsDeduped int
}

// ApplyPointerWalkDedup scans f for loop-body blocks with repeated
// ptr_add(base, offset) terms and deduplicates them. Returns true if
// any rewrite fired.
func ApplyPointerWalkDedup(f *Func, stats *PointerWalkStats) bool {
	if f == nil || len(f.Blocks) == 0 {
		return false
	}

	facts := CollectSolverFriendlyShapeFacts(f)
	if stats != nil {
		stats.FuncsVisited++
	}

	// Build set of blocks that have repeated same_term facts inside a loop.
	type dedupTarget struct {
		blockLabel string
		ptrKey     string
	}
	var targets []dedupTarget
	for _, rt := range facts.RepeatedAddrTerms {
		if rt.InLoop && rt.Kind == "same_term" && rt.Count >= 2 {
			targets = append(targets, dedupTarget{
				blockLabel: rt.BlockLabel,
				ptrKey:     rt.Key,
			})
		}
	}

	if len(targets) == 0 {
		return false
	}

	changed := false
	for _, tgt := range targets {
		b := f.BlockByLabel(tgt.blockLabel)
		if b == nil {
			continue
		}
		n := deduplicatePtrAddsInBlock(f, b, tgt.ptrKey)
		if n > 0 {
			changed = true
			if stats != nil {
				stats.BlocksEdited++
				stats.PtrAddsDeduped += n
			}
		}
	}
	return changed
}

// deduplicatePtrAddsInBlock finds all OpPtrAdd instructions in b that produce
// the same (base, offset) pair identified by ptrKey, keeps the first one, and
// rewrites all later uses of the duplicate results to use the first result.
// deduplicatePtrAddsInBlock returns the number of duplicate ptr_adds removed (0 if none).
func deduplicatePtrAddsInBlock(f *Func, b *Block, ptrKey string) int {
	// Find all ptr_add insts in this block and group by matching (base, offset).
	type ptrAddInfo struct {
		inst  *Inst
		index int
	}
	var matching []ptrAddInfo

	for i, inst := range b.Insts {
		if inst.Op != OpPtrAdd {
			continue
		}
		key := valueKey(inst.Dst, f)
		if key == ptrKey {
			matching = append(matching, ptrAddInfo{inst: inst, index: i})
		}
	}

	if len(matching) < 2 {
		return 0
	}

	// Keep the first, replace uses of all later duplicates with the first's Dst.
	keepReg := matching[0].inst.Dst
	replaced := 0

	for _, dup := range matching[1:] {
		oldReg := dup.inst.Dst
		// Replace all uses of oldReg in subsequent instructions and terminator.
		for _, inst := range b.Insts {
			replaceSrc(inst, oldReg, keepReg)
		}
		if b.Term != nil {
			replaceTermReg(b.Term, oldReg, keepReg)
		}
		// Mark the duplicate ptr_add as dead (OpNop-like: replace with a copy
		// that DSE can clean up, or just rewrite to point at keepReg).
		dup.inst.Op = OpMove
		dup.inst.Src = [2]Reg{keepReg, NoReg}
		replaced++
	}

	return replaced
}

// replaceSrc replaces occurrences of old with new in inst's Src slots.
func replaceSrc(inst *Inst, old, new Reg) {
	if inst.Src[0] == old {
		inst.Src[0] = new
	}
	if inst.Src[1] == old {
		inst.Src[1] = new
	}
}

// replaceTermReg replaces occurrences of old with new in a terminator's
// registers (branch args, condition, return values).
func replaceTermReg(t Term, old, new Reg) {
	switch tt := t.(type) {
	case *TermJmp:
		for i, r := range tt.Args {
			if r == old {
				tt.Args[i] = new
			}
		}
	case *TermBrIf:
		if tt.Cond == old {
			tt.Cond = new
		}
		for i, r := range tt.ThenArgs {
			if r == old {
				tt.ThenArgs[i] = new
			}
		}
		for i, r := range tt.ElseArgs {
			if r == old {
				tt.ElseArgs[i] = new
			}
		}
	case *TermRet:
		for i, r := range tt.Vals {
			if r == old {
				tt.Vals[i] = new
			}
		}
	}
}
