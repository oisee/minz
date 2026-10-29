package mir2

import "fmt"

// InlineCompareSelect inlines calls to a callee whose whole body is a
// compare-and-select:
//
//	fun @f(%p0: T, %p1: T) -> T
//	  block @entry:
//	    %c = cmp.<cond> %pa, %pb : bool [flag]
//	    cond_ret %c, [%pv], @sel()
//	  block @sel:
//	    ret %pw
//
// (or the equivalent plain form — `br_if %c, @sel(), @retblk()` with two bare
// return blocks).  %pa/%pb are the two parameters in either order, and the two
// arms return one of the parameters unchanged.  `larger`/`smaller` from the
// fusion suite are exactly this shape.
//
// InlineTrivial refuses these because the body is multi-block, so every element
// of a fold pays PUSH/PUSH/LD/CALL/POP/POP (≈63 T) around a 15-23 T body.  The
// rewrite replaces
//
//	%d = call @f(%acc, %x)      … tail …   jmp @loop(%p, %i, %d)
//
// with
//
//	%c = cmp.<cond> %x, %acc : bool [flag]
//	… tail …
//	br_if %c, @loop(%p, %i, %x), @loop(%p, %i, %acc)
//
// which the backend lowers to `CP r / JR cc / [LD r,A]` — the hand-asm shape.
//
// # Exclusions (hard)
//
//   - recursive, ReferencedByName, address-taken, extern, comptime, fallible-CY
//     and fixed-indirect-ABI callees; the callee body is left in place.
//   - any memory op, call, inline asm or port I/O in the callee.  This is
//     enforced by the instruction budget, not by an opcode list: a matched
//     callee has exactly one instruction, and that instruction must be the
//     compare.
//   - more than the matched single instruction in the callee.
//   - mismatched widths or signedness between the two parameters, between the
//     parameters and the compare's operand type, or between the parameters and
//     the return type.
//
// The callee is never deleted here: if it becomes unreferenced,
// PruneUnreachableFuncs already handles it.
//
// keepCallers names callers whose calls must be preserved (variant measurement
// pins their call counts); inlinedCallers, when non-nil, records the callers
// that were rewritten, mirroring InlineTrivial's out-parameter.
func InlineCompareSelect(m *Module, keepCallers map[string]bool, inlinedCallers map[string]bool) bool {
	if m == nil {
		return false
	}
	symbolic := cmpSelectSymbolicRefs(m)
	shapes := make(map[string]*cmpSelectShape)
	for _, f := range m.Funcs {
		if f == nil || symbolic[f.Name] {
			continue
		}
		if shape := matchCmpSelect(f); shape != nil {
			shapes[f.Name] = shape
		}
	}
	if len(shapes) == 0 {
		return false
	}
	changed := false
	for _, caller := range m.Funcs {
		if caller == nil || keepCallers[caller.Name] {
			continue
		}
		if inlineCompareSelectInFunc(caller, shapes) {
			if inlinedCallers != nil {
				inlinedCallers[caller.Name] = true
			}
			changed = true
		}
	}
	return changed
}

// cmpSelectShape is the matched callee, expressed as parameter indices so the
// rewrite can map it onto the caller's argument registers.
type cmpSelectShape struct {
	cond     CmpCond // comparison predicate, unchanged
	lhs      int     // parameter index compared as Src[0]
	rhs      int     // parameter index compared as Src[1]
	selTrue  int     // parameter index returned when the condition holds
	selFalse int     // parameter index returned when it does not
	srcTy    Ty      // compare operand type (nil for the unsigned default)
	cls      RegClass
}

// cmpSelectMaxInsts is the exact instruction count a matched callee may have:
// the single compare.  Anything more is excluded — which is also what rules out
// memory ops, calls, inline asm and port I/O in the callee.
const cmpSelectMaxInsts = 1

// cmpSelectSymbolicRefs returns every function reachable by symbolic reference
// instead of a direct OpCall: the address-taken set the contract optimiser
// already computes, plus global relocations and vtable entries.
func cmpSelectSymbolicRefs(m *Module) map[string]bool {
	symbolic := addrTakenFuncs(m)
	for _, g := range m.Globals {
		for _, r := range g.Relocs {
			symbolic[r.Symbol] = true
		}
		for _, s := range g.VtableSyms {
			symbolic[s] = true
		}
	}
	return symbolic
}

// matchCmpSelect reports whether f is exactly a compare-and-select, returning
// the matched shape or nil.
func matchCmpSelect(f *Func) *cmpSelectShape {
	if f.Attrs.ReferencedByName || f.Attrs.FixedIndirectABI || f.Attrs.IsExtern ||
		f.Attrs.IsRecursive || f.Attrs.IsComptime || f.Attrs.FallibleCY || f.Attrs.HasAsm {
		return nil
	}
	if len(f.Contract.StructReturnBuffers) > 0 {
		return nil
	}
	if len(f.Contract.Params) != 2 || len(f.Contract.Returns) != 1 {
		return nil
	}
	p0, p1 := f.Contract.Params[0], f.Contract.Params[1]
	if p0.Ty != p1.Ty || f.Contract.Returns[0].Ty != p0.Ty {
		return nil // mismatched widths or signedness
	}
	if p0.Class == ClassFlag || p0.ClassHard || p1.ClassHard || f.Contract.Returns[0].Class == ClassFlag {
		return nil
	}
	if len(f.Blocks) < 2 {
		return nil
	}
	entry := f.Entry()
	if len(entry.Params) != 0 || len(entry.Insts) != cmpSelectMaxInsts {
		return nil
	}
	cmpInst := entry.Insts[0]
	if cmpInst.Op != OpCmp || cmpInst.Dst == NoReg || cmpInst.ClsHard {
		return nil
	}
	if !cmpSelectCond(cmpInst.Cond) {
		return nil
	}
	if cmpInst.SrcTy != nil && cmpInst.SrcTy != p0.Ty {
		return nil // compare width/signedness differs from the parameters
	}
	lhs, lhsOK := cmpSelectParamIndex(f, cmpInst.Src[0])
	rhs, rhsOK := cmpSelectParamIndex(f, cmpInst.Src[1])
	if !lhsOK || !rhsOK || lhs == rhs {
		return nil
	}

	// Bare return block helper: no instructions, no params, one returned value.
	bareRet := func(label string) (Reg, bool) {
		b := f.BlockByLabel(label)
		if b == nil || b == entry || len(b.Insts) != 0 || len(b.Params) != 0 {
			return NoReg, false
		}
		ret, ok := b.Term.(*TermRet)
		if !ok || len(ret.Vals) != 1 {
			return NoReg, false
		}
		return ret.Vals[0], true
	}

	var selTrue, selFalse Reg
	switch t := entry.Term.(type) {
	case *TermCondRet:
		// cond_ret %c, [%vFalse], @then()  — cond false returns Vals,
		// cond true jumps to Then, which returns its own value.
		if len(f.Blocks) != 2 || len(t.Vals) != 1 || len(t.ThenArgs) != 0 {
			return nil
		}
		after, ok := bareRet(t.Then)
		if !ok {
			return nil
		}
		selTrue, selFalse = after, t.Vals[0]
	case *TermBrIf:
		// br_if %c, @then(), @else() with two bare return blocks.
		if len(f.Blocks) != 3 || len(t.ThenArgs) != 0 || len(t.ElseArgs) != 0 {
			return nil
		}
		after, okThen := bareRet(t.Then)
		before, okElse := bareRet(t.Else)
		if !okThen || !okElse {
			return nil
		}
		selTrue, selFalse = after, before
	default:
		return nil
	}

	st, okT := cmpSelectParamIndex(f, selTrue)
	sf, okF := cmpSelectParamIndex(f, selFalse)
	if !okT || !okF || st == sf {
		return nil
	}
	return &cmpSelectShape{
		cond:     cmpInst.Cond,
		lhs:      lhs,
		rhs:      rhs,
		selTrue:  st,
		selFalse: sf,
		srcTy:    cmpInst.SrcTy,
		cls:      cmpInst.Cls,
	}
}

// cmpSelectParamIndex returns which contract parameter r is, if any.
func cmpSelectParamIndex(f *Func, r Reg) (int, bool) {
	for i, p := range f.Contract.Params {
		if p.Reg == r {
			return i, true
		}
	}
	return 0, false
}

// cmpSelectCond reports whether cond is a plain value comparison.  The
// synthetic carry predicates assume a preceding subtraction in the callee and
// cannot be replayed at the call site.
func cmpSelectCond(c CmpCond) bool {
	switch c {
	case CmpEq, CmpNe, CmpLt, CmpLe, CmpGt, CmpGe,
		CmpUlt, CmpUle, CmpUgt, CmpUge:
		return true
	}
	return false
}

// cmpSelectOutcome says what a rewrite did to the caller's block, so the scan
// loop knows where to continue.
type cmpSelectOutcome int

const (
	cmpSelectRefused cmpSelectOutcome = iota
	cmpSelectDropped                  // dead call removed; slot i now holds the next instruction
	cmpSelectInlined                  // slot i holds the inline compare; the tail follows it
	cmpSelectSplit                    // tail moved to a new block; this block now ends at i
)

// inlineCompareSelectInFunc rewrites every qualifying call site in the blocks
// caller has on entry.  Blocks created by the split path are not rescanned, so
// a nested call in a moved tail keeps its CALL — conservative, and it keeps the
// rewrite acyclic.
func inlineCompareSelectInFunc(caller *Func, shapes map[string]*cmpSelectShape) bool {
	changed := false
	blocks := append([]*Block(nil), caller.Blocks...)
	for _, b := range blocks {
		for i := 0; i < len(b.Insts); {
			inst := b.Insts[i]
			shape, ok := shapes[inst.Sym]
			if inst.Op != OpCall || !ok {
				i++
				continue
			}
			if len(inst.Args) != 2 || len(inst.ExtraRets) != 0 ||
				inst.FallibleCY || inst.ByteLoop != nil ||
				inst.CallAttr.IsComptime || inst.CallAttr.IsHost || inst.CallAttr.IsTailCall ||
				inst.SequentialBytes || inst.StructReturnBuffers != nil {
				i++
				continue
			}
			outcome := inlineCompareSelectSite(caller, b, i, shape)
			if outcome != cmpSelectRefused {
				changed = true
			}
			switch outcome {
			case cmpSelectRefused, cmpSelectInlined:
				i++
			case cmpSelectDropped:
				// Stay at i: the instruction after the deleted call has
				// shifted into it, and it may itself be a call.
			case cmpSelectSplit:
				// The tail moved into the new join block, which is not in the
				// snapshot; nothing is left to scan here.
				i = len(b.Insts)
			}
		}
	}
	return changed
}

// inlineCompareSelectSite rewrites caller block b at call index i.
func inlineCompareSelectSite(caller *Func, b *Block, i int, shape *cmpSelectShape) cmpSelectOutcome {
	call := b.Insts[i]
	dst := call.Dst

	// The selected value must be consumed inside this block: the rewrite
	// substitutes it into the tail and the terminator only.
	total, local := cmpSelectUseCount(caller, dst, b, i)
	if total != local {
		return cmpSelectRefused
	}
	if total == 0 {
		// Result unused: the callee is exactly a compare with no side effects,
		// so the call is dead.  Drop it.
		b.Insts = append(b.Insts[:i], b.Insts[i+1:]...)
		return cmpSelectDropped
	}

	tail := b.Insts[i+1:]
	tailUses := 0
	for _, inst := range tail {
		for _, u := range inst.Uses() {
			if u == dst {
				tailUses++
			}
		}
	}

	cond := caller.AllocReg()
	cmpI := &Inst{
		Op:      OpCmp,
		Dst:     cond,
		Src:     [2]Reg{call.Args[shape.lhs], call.Args[shape.rhs]},
		Cond:    shape.cond,
		Ty:      TyBool,
		SrcTy:   shape.srcTy,
		Cls:     shape.cls,
		SrcLine: call.SrcLine,
		SrcFile: call.SrcFile,
	}
	selTrue, selFalse := call.Args[shape.selTrue], call.Args[shape.selFalse]

	if tailUses == 0 {
		if jmp, ok := b.Term.(*TermJmp); ok {
			// Fast path: one target, two argument sets.  The backend emits
			// `CP r / JR cc` with the selected value moved on the taken edge.
			regMap := map[Reg]Reg{dst: selTrue}
			thenArgs := remapRegs(jmp.Args, regMap)
			elseArgs := remapRegs(jmp.Args, map[Reg]Reg{dst: selFalse})
			// Emit the compare last when the tail cannot redefine its operands:
			// a tail `sub` (loop counter decrement) clobbers F, which would
			// otherwise force the predicate out of the flag and into a byte.
			head := append([]*Inst(nil), b.Insts[:i]...)
			tailInsts := append([]*Inst(nil), b.Insts[i+1:]...)
			if tailRedefines(tailInsts, cmpI.Src[0], cmpI.Src[1]) {
				b.Insts = append(append(head, cmpI), tailInsts...)
			} else {
				b.Insts = append(append(head, tailInsts...), cmpI)
			}
			b.Term = &TermBrIf{
				Cond:     cond,
				Then:     jmp.Target,
				ThenArgs: thenArgs,
				Else:     jmp.Target,
				ElseArgs: elseArgs,
			}
			return cmpSelectInlined
		}
	}

	// General path: route both arms through one continuation block that carries
	// the selected value as a block parameter.  The tail is emitted once, so
	// code size does not grow with the size of the call site's block.
	//
	//	b:       … cmp … ; br_if %c, @selT(), @cont(selFalse)
	//	selT:    jmp @cont(selTrue)
	//	cont(v): … tail with %dst → v … ; original terminator
	//
	// remapInst substitutes every register operand of the tail, including an
	// inline-asm template's Ins/Outs lists.
	prefix := freshCmpSelectLabel(caller)
	join := &Block{Label: prefix + "c", Params: []BlockParam{{
		Dst:   caller.AllocReg(),
		Ty:    call.Ty,
		Class: call.Cls,
	}}}
	regMap := map[Reg]Reg{dst: join.Params[0].Dst}

	join.Insts = make([]*Inst, 0, len(tail))
	for _, inst := range tail {
		join.Insts = append(join.Insts, remapInst(inst, regMap))
	}
	join.Term = cloneTermRegMap(b.Term, regMap)

	selBlock := &Block{Label: prefix + "t", Term: &TermJmp{Target: join.Label, Args: []Reg{selTrue}}}
	b.Insts = append(b.Insts[:i:i], cmpI)
	b.Term = &TermBrIf{
		Cond:     cond,
		Then:     selBlock.Label,
		Else:     join.Label,
		ElseArgs: []Reg{selFalse},
	}
	caller.Blocks = append(caller.Blocks, selBlock, join)
	return cmpSelectSplit
}

// freshCmpSelectLabel returns a label prefix unique within f.  It deliberately
// does not consume a virtual-register number, which would perturb register
// numbering — and MIR dumps — for unrelated reasons.
func freshCmpSelectLabel(f *Func) string {
	for n := len(f.Blocks); ; n++ {
		prefix := fmt.Sprintf("cmpsel%d_", n)
		if f.BlockByLabel(prefix+"t") == nil && f.BlockByLabel(prefix+"c") == nil {
			return prefix
		}
	}
}

// tailRedefines reports whether any instruction in tail defines a or b,
// including an inline-asm template's output list.
func tailRedefines(tail []*Inst, a, b Reg) bool {
	for _, inst := range tail {
		if inst.Dst == a || inst.Dst == b {
			return true
		}
		for _, r := range inst.ExtraRets {
			if r == a || r == b {
				return true
			}
		}
		if inst.Asm != nil {
			for _, r := range inst.Asm.Outs {
				if r == a || r == b {
					return true
				}
			}
		}
	}
	return false
}

// cmpSelectUseCount returns the number of uses of r in the whole function and
// the number of those that sit after index i of block b (tail or terminator).
func cmpSelectUseCount(f *Func, r Reg, b *Block, i int) (total, local int) {
	if r == NoReg {
		return 0, 0
	}
	for _, blk := range f.Blocks {
		for j, inst := range blk.Insts {
			for _, u := range inst.Uses() {
				if u == r {
					total++
					if blk == b && j > i {
						local++
					}
				}
			}
		}
		if blk.Term != nil {
			for _, u := range blk.Term.termUses() {
				if u == r {
					total++
					if blk == b {
						local++
					}
				}
			}
		}
	}
	return total, local
}

// cloneTermRegMap deep-copies term with all register references substituted via
// regMap.
func cloneTermRegMap(t Term, regMap map[Reg]Reg) Term {
	switch t := t.(type) {
	case *TermJmp:
		return &TermJmp{Target: t.Target, Args: remapRegs(t.Args, regMap)}
	case *TermBrIf:
		return &TermBrIf{Cond: remapReg(t.Cond, regMap), Then: t.Then, ThenArgs: remapRegs(t.ThenArgs, regMap), Else: t.Else, ElseArgs: remapRegs(t.ElseArgs, regMap)}
	case *TermBrIf2:
		return &TermBrIf2{Lhs: remapReg(t.Lhs, regMap), Rhs: remapReg(t.Rhs, regMap), Eq: t.Eq, EqArgs: remapRegs(t.EqArgs, regMap), Lt: t.Lt, LtArgs: remapRegs(t.LtArgs, regMap), Gt: t.Gt, GtArgs: remapRegs(t.GtArgs, regMap)}
	case *TermDJNZ:
		return &TermDJNZ{Counter: remapReg(t.Counter, regMap), Body: t.Body, BodyArgs: remapRegs(t.BodyArgs, regMap), Exit: t.Exit, ExitArgs: remapRegs(t.ExitArgs, regMap)}
	case *TermCondRet:
		return &TermCondRet{Cond: remapReg(t.Cond, regMap), Vals: remapRegs(t.Vals, regMap), Then: t.Then, ThenArgs: remapRegs(t.ThenArgs, regMap)}
	case *TermRet:
		return &TermRet{Vals: remapRegs(t.Vals, regMap)}
	case *TermErrorRet:
		return &TermErrorRet{Code: remapReg(t.Code, regMap)}
	case *TermUnreachable:
		return &TermUnreachable{}
	}
	// A nil here would seal the cloned block with no terminator, which Verify
	// rejects far away from the cause.
	panic(fmt.Sprintf("cloneTermRegMap: unhandled terminator %T", t))
}
