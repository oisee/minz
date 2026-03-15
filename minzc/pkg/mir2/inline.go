package mir2

// MIR2-level trivial function inlining.
//
// InlineTrivial replaces call sites of "trivial" functions with the callee
// body.  After inlining, PropagateCopies removes the resulting move chains,
// and DeadStoreElim cleans up unused instructions.
//
// A function is trivial if all of the following hold:
//   - Exactly one block (the entry block — no control flow).
//   - At most maxSize instructions in that block.
//   - No OpCall or OpCallIndirect (leaf only — no call-stack growth).
//   - No OpPatch / OpPatchSlot / OpLoadPatched (SMC address anchors must
//     remain at their original site).
//   - No OpAsm (inline asm may reference specific registers by position).
//   - Not marked IsExtern (no body to inline).
//   - Not marked IsRecursive (cycle-safe — trivial implies ≤1 block, so in
//     practice recursive trivial functions cannot exist, but guard anyway).
//
// Inlining mechanics
// ──────────────────
//
//  Given:
//    callInst: %d0, %d1... = CALL callee(%a0, %a1, ...)
//    callee entry block:
//      param %p0, %p1, ...
//      %r0 = Op(...)
//      ...
//      ret %v0, %v1, ...
//
//  Step 1: build regMap
//    %p_i → %a_i            (param → caller arg — direct, no copy needed)
//    %r_k → fresh caller reg (body instruction dsts)
//
//  Step 2: splice remapped copies of callee body instructions before callInst.
//
//  Step 3: emit OpMove for each return value:
//    %d0 = OpMove(regMap[%v0])
//    %d1 = OpMove(regMap[%v1])
//    (if %d_i == NoReg, the return slot is unused — skip it.)
//
//  Step 4: remove the original callInst.
//
// After inlining, run PropagateCopies + DeadStoreElim:
//   %p0 = %a0 (via regMap),  %v0 = %p_j (chain) → %d0 = Move(%a_j)
//   PropagCopies folds: any use of %d0 → %a_j directly.
//   DSE removes now-unused Moves.
//
// Example (swap):
//   Before:  %3, %4 = CALL swap(%1, %2)
//   Inline:  %3 = Move(%2);  %4 = Move(%1)
//   CopyProp: all uses of %3 → %2,  %4 → %1
//   DSE:     delete Move instructions (both unused)
//   Net:     CALL+EX+RET = 35T → 0T

// InlineTrivial inlines all call sites of trivial callees in the module.
// maxSize is the maximum number of instructions in the callee entry block.
// Returns true if any inlining was performed.
//
// Run PropagateCopies + DeadStoreElim on each caller after this pass.
func InlineTrivial(mod *Module, maxSize int) bool {
	trivial := findTrivialFuncs(mod, maxSize)
	if len(trivial) == 0 {
		return false
	}

	anyChanged := false
	for _, caller := range mod.Funcs {
		if inlineCallsInFunc(caller, mod, trivial) {
			anyChanged = true
		}
	}
	return anyChanged
}

// findTrivialFuncs returns the set of trivial functions in mod.
func findTrivialFuncs(mod *Module, maxSize int) map[string]*Func {
	result := make(map[string]*Func)
	for _, f := range mod.Funcs {
		if isTrivialFunc(f, maxSize) {
			result[f.Name] = f
		}
	}
	return result
}

// isTrivialFunc reports whether f is safe and profitable to inline.
func isTrivialFunc(f *Func, maxSize int) bool {
	if f.Attrs.IsExtern || f.Attrs.IsRecursive {
		return false
	}
	if len(f.Blocks) != 1 {
		return false
	}
	entry := f.Blocks[0]
	if len(entry.Insts) > maxSize {
		return false
	}
	for _, inst := range entry.Insts {
		switch inst.Op {
		case OpCall, OpCallIndirect,
			OpPatch, OpPatchSlot, OpLoadPatched,
			OpAsm:
			return false
		case OpAddrOf, OpPtrAdd:
			// Functions with pointer operations need 16-bit pair registers.
			// Inlining them into register-starved callers can force pointers
			// into 8-bit regs → invalid Z80 instructions.
			return false
		}
	}
	return true
}

// inlineCallsInFunc rewrites all OpCall sites in caller that target a trivial
// function.  Returns true if at least one call was inlined.
func inlineCallsInFunc(caller *Func, mod *Module, trivial map[string]*Func) bool {
	changed := false
	for _, b := range caller.Blocks {
		b.Insts = inlineCallsInBlock(caller, mod, b, trivial, &changed)
	}
	return changed
}

// inlineCallsInBlock processes one block, replacing qualifying OpCall
// instructions with the inlined body.  Returns the new instruction slice.
func inlineCallsInBlock(
	caller *Func,
	mod *Module,
	b *Block,
	trivial map[string]*Func,
	changed *bool,
) []*Inst {
	out := make([]*Inst, 0, len(b.Insts))
	for _, inst := range b.Insts {
		if inst.Op != OpCall {
			out = append(out, inst)
			continue
		}
		callee, ok := trivial[inst.Sym]
		if !ok {
			out = append(out, inst)
			continue
		}
		// Inline this call site.
		inlined := inlineOneCall(caller, callee, inst)
		out = append(out, inlined...)
		*changed = true
	}
	return out
}

// inlineOneCall inlines a single call to callee at inst, returning the slice of
// replacement instructions.  inst itself is NOT included in the result.
func inlineOneCall(caller, callee *Func, inst *Inst) []*Inst {
	entry := callee.Entry()
	if entry == nil {
		return nil // extern with no body — should have been excluded by isTrivialFunc
	}

	// ── Step 1: build register map ────────────────────────────────────────────

	regMap := make(map[Reg]Reg, len(callee.Contract.Params)+len(entry.Insts))

	// Map callee params → caller args.
	for i, p := range callee.Contract.Params {
		if i < len(inst.Args) {
			regMap[p.Reg] = inst.Args[i]
		}
	}

	// Allocate fresh caller regs for callee body instruction dsts.
	for _, ci := range entry.Insts {
		if ci.Dst != NoReg {
			if _, already := regMap[ci.Dst]; !already {
				nr := caller.AllocReg()
				regMap[ci.Dst] = nr
			}
		}
	}

	// ── Step 2: splice remapped body instructions ─────────────────────────────

	splice := make([]*Inst, 0, len(entry.Insts)+len(inst.ExtraRets)+1)
	for _, ci := range entry.Insts {
		ni := remapInst(ci, regMap)
		splice = append(splice, ni)
	}

	// ── Step 3: emit OpMove for each return value ─────────────────────────────

	if ret, ok := entry.Term.(*TermRet); ok {
		for i, retReg := range ret.Vals {
			var dst Reg
			switch i {
			case 0:
				dst = inst.Dst
			default:
				idx := i - 1
				if idx < len(inst.ExtraRets) {
					dst = inst.ExtraRets[idx]
				}
			}
			if dst == NoReg {
				continue // unused return slot
			}
			src := remapReg(retReg, regMap)
			if src == NoReg {
				continue // shouldn't happen; guard for safety
			}
			if src == dst {
				continue // already in place — no move needed
			}
			// Determine the return type for the OpMove.
			ty := retTy(callee, i)
			cls := retCls(callee, i)
			splice = append(splice, &Inst{
				Op:  OpMove,
				Dst: dst,
				Src: [2]Reg{src},
				Ty:  ty,
				Cls: cls,
			})
		}
	}

	// Step 4: the original callInst is dropped (not appended to splice).
	return splice
}

// remapInst returns a shallow copy of inst with all register references
// substituted via regMap.
func remapInst(inst *Inst, regMap map[Reg]Reg) *Inst {
	ni := *inst // shallow copy — safe for fields without pointer ownership
	ni.Dst = remapReg(inst.Dst, regMap)
	ni.Src[0] = remapReg(inst.Src[0], regMap)
	ni.Src[1] = remapReg(inst.Src[1], regMap)
	if len(inst.Args) > 0 {
		ni.Args = make([]Reg, len(inst.Args))
		for i, a := range inst.Args {
			ni.Args[i] = remapReg(a, regMap)
		}
	}
	// ExtraRets, Asm etc. should not appear in trivial callee bodies.
	return &ni
}

// remapReg returns the mapped register, or r itself if not in regMap.
// NoReg is always returned as-is.
func remapReg(r Reg, regMap map[Reg]Reg) Reg {
	if r == NoReg {
		return NoReg
	}
	if mapped, ok := regMap[r]; ok {
		return mapped
	}
	return r
}

// retTy returns the type of the i-th return value of f.
func retTy(f *Func, i int) Ty {
	if i < len(f.Contract.Returns) {
		return f.Contract.Returns[i].Ty
	}
	return TyU8 // fallback
}

// retCls returns the register class of the i-th return value of f.
func retCls(f *Func, i int) RegClass {
	if i < len(f.Contract.Returns) {
		return f.Contract.Returns[i].Class
	}
	return ClassGeneral
}

// ── PropagateCopies ───────────────────────────────────────────────────────────

// PropagateCopies replaces all uses of OpMove destinations with their ultimate
// source register (following chains).  After copy propagation, DeadStoreElim
// removes the now-unused OpMove instructions.
//
// This is the clean-up pass for InlineTrivial output.  It is also useful after
// any other transformation that introduces redundant moves.
//
// Returns true if any substitution was made.
func PropagateCopies(f *Func) bool {
	// Collect all move pairs: dst → src (direct, not yet chained).
	moves := make(map[Reg]Reg)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpMove && inst.Dst != NoReg && inst.Src[0] != NoReg {
				moves[inst.Dst] = inst.Src[0]
			}
		}
	}
	if len(moves) == 0 {
		return false
	}

	// resolve follows the chain A → B → C → D until no further mapping exists.
	// Cycle-safe: if the chain is longer than len(moves), something is wrong;
	// bail after len(moves)+1 steps.
	resolve := func(r Reg) Reg {
		for steps := 0; steps <= len(moves); steps++ {
			src, ok := moves[r]
			if !ok {
				break
			}
			r = src
		}
		return r
	}

	changed := false

	// Rewrite instruction operands.
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for i, s := range inst.Src {
				if s == NoReg {
					continue
				}
				if r := resolve(s); r != s {
					inst.Src[i] = r
					changed = true
				}
			}
			for i, a := range inst.Args {
				if a == NoReg {
					continue
				}
				if r := resolve(a); r != a {
					inst.Args[i] = r
					changed = true
				}
			}
		}
		// Rewrite terminator operands.
		if b.Term != nil {
			for _, u := range b.Term.termUses() {
				if u == NoReg {
					continue
				}
				if r := resolve(u); r != u {
					rewriteTermUses(b.Term, u, r)
					changed = true
				}
			}
		}
	}

	return changed
}
