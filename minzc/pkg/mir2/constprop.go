package mir2

// collectKnownConsts returns a map of Reg → int64 for every OpConst-defined
// register in f.  This is the seed used by PropagateConstants, FoldConstants,
// and ConstantCallElim.
func collectKnownConsts(f *Func) map[Reg]int64 {
	m := make(map[Reg]int64)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpConst {
				m[inst.Dst] = inst.Imm
			}
		}
	}
	return m
}

// FoldConstants folds pure instructions whose operands are all compile-time
// constants into a single OpConst.
//
// Processes each block's instructions in forward order, updating the known-
// constant map after each fold so that later instructions in the same block
// can chain (e.g. x=Const(3); y=Mul(x,2) → y=Const(6); z=Add(y,1) → z=Const(7)).
//
// Supports:
//   - Unary:   OpNeg, OpNot, OpMove, OpExt, OpTrunc, OpSext
//   - Binary:  OpAdd, OpSub, OpMul, OpDiv, OpSDiv, OpMod
//              OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar
//   - Compare: OpCmp (all conditions, signed and unsigned)
//
// Returns true if any instruction was folded.  Callers should follow with
// DeadStoreElim to remove the now-unused original operand definitions.
func FoldConstants(f *Func) bool {
	consts := collectKnownConsts(f)
	changed := false

	for _, b := range f.Blocks {
		for i, inst := range b.Insts {
			if inst.Dst == NoReg {
				continue
			}
			folded, ok := tryFoldInst(inst, consts)
			if !ok {
				continue
			}
			b.Insts[i] = &Inst{
				Op:  OpConst,
				Dst: inst.Dst,
				Imm: folded,
				Ty:  inst.Ty,
				Cls: inst.Cls,
			}
			consts[inst.Dst] = folded
			changed = true
		}
	}
	return changed
}

// tryFoldInst attempts to evaluate inst at compile time using the known
// constant values in consts.  Returns (result, true) on success.
func tryFoldInst(inst *Inst, consts map[Reg]int64) (int64, bool) {
	src0, ok0 := consts[inst.Src[0]]
	src1, ok1 := consts[inst.Src[1]]

	switch inst.Op {

	// ── Unary ────────────────────────────────────────────────────────────────
	case OpMove:
		if !ok0 {
			return 0, false
		}
		return maskToWidth(src0, inst.Ty), true
	case OpNeg:
		if !ok0 {
			return 0, false
		}
		return maskToWidth(-src0, inst.Ty), true
	case OpNot:
		if !ok0 {
			return 0, false
		}
		return maskToWidth(^src0, inst.Ty), true
	case OpExt, OpTrunc:
		if !ok0 {
			return 0, false
		}
		return maskToWidth(src0, inst.Ty), true
	case OpSext:
		if !ok0 {
			return 0, false
		}
		// Sign-extend from SrcTy width to inst.Ty width.
		srcBits := 8
		if inst.SrcTy != nil {
			srcBits = inst.SrcTy.Width()
		}
		v := src0
		if srcBits < 64 {
			shift := 64 - srcBits
			v = (v << shift) >> shift // arithmetic right shift
		}
		return maskToWidth(v, inst.Ty), true

	// ── Binary arithmetic ─────────────────────────────────────────────────────
	case OpAdd:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0+src1, inst.Ty), true
	case OpSub:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0-src1, inst.Ty), true
	case OpMul:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0*src1, inst.Ty), true
	case OpDiv:
		if !ok0 || !ok1 || src1 == 0 {
			return 0, false
		}
		// Unsigned division.
		return maskToWidth(int64(uint64(src0)/uint64(src1)), inst.Ty), true
	case OpSDiv:
		if !ok0 || !ok1 || src1 == 0 {
			return 0, false
		}
		return maskToWidth(src0/src1, inst.Ty), true
	case OpMod:
		if !ok0 || !ok1 || src1 == 0 {
			return 0, false
		}
		return maskToWidth(int64(uint64(src0)%uint64(src1)), inst.Ty), true

	// ── Bitwise ───────────────────────────────────────────────────────────────
	case OpAnd:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0&src1, inst.Ty), true
	case OpOr:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0|src1, inst.Ty), true
	case OpXor:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0^src1, inst.Ty), true
	case OpShl:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0<<uint(src1), inst.Ty), true
	case OpShr:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(int64(uint64(src0)>>uint(src1)), inst.Ty), true
	case OpSar:
		if !ok0 || !ok1 {
			return 0, false
		}
		return maskToWidth(src0>>uint(src1), inst.Ty), true

	// ── Comparison ────────────────────────────────────────────────────────────
	case OpCmp:
		if !ok0 || !ok1 {
			return 0, false
		}
		return foldCmpCond(inst.Cond, src0, src1), true
	}
	return 0, false
}

// maskToWidth masks v to the bit-width of ty (e.g. u8 → &0xFF).
// Returns v unchanged for wide or unknown types.
func maskToWidth(v int64, ty Ty) int64 {
	if ty == nil {
		return v
	}
	w := ty.Width()
	if w <= 0 || w >= 64 {
		return v
	}
	return v & int64((1<<w)-1)
}

// foldCmpCond evaluates a comparison at compile time.
func foldCmpCond(cond CmpCond, lhs, rhs int64) int64 {
	var result bool
	switch cond {
	case CmpEq:
		result = lhs == rhs
	case CmpNe:
		result = lhs != rhs
	case CmpLt:
		result = lhs < rhs // signed
	case CmpLe:
		result = lhs <= rhs
	case CmpGt:
		result = lhs > rhs
	case CmpGe:
		result = lhs >= rhs
	case CmpUlt:
		result = uint64(lhs) < uint64(rhs)
	case CmpUle:
		result = uint64(lhs) <= uint64(rhs)
	case CmpUgt:
		result = uint64(lhs) > uint64(rhs)
	case CmpUge:
		result = uint64(lhs) >= uint64(rhs)
	}
	if result {
		return 1
	}
	return 0
}

// PropagateConstants performs forward constant propagation over block params.
//
// A block param at position i is folded when every predecessor edge that passes
// an argument for position i supplies the same compile-time constant.
//
// When a param is folded:
//  1. A fresh OpConst instruction is prepended to the block body.
//  2. Every use of the original param Dst inside the block (instructions and
//     terminator args) is rewritten to the fresh reg.
//  3. The original block param is kept structurally (predecessors still pass
//     the arg), but its Dst now has zero uses and DeadStoreElim will not clean
//     it up (block params are not instructions).  The wasted block param does
//     not affect correctness — it is invisible to the register allocator because
//     only instruction Dsts are coloured.
//
// Returns true if at least one param was folded.  Callers may iterate until
// false to reach a fixpoint (needed when one block's params feed another's).
//
// Integration: run after EliminateDeadBlocks (fewer predecessors to track) and
// before Allocate.  DeadStoreElim should run after to clean up newly-dead consts.
func PropagateConstants(f *Func) bool {
	if len(f.Blocks) == 0 {
		return false
	}

	// ── Step 1: seed const map from all OpConst instructions ─────────────────

	consts := collectKnownConsts(f) // reg → compile-time value
	constTy := make(map[Reg]Ty)
	constCls := make(map[Reg]RegClass)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpConst {
				constTy[inst.Dst] = inst.Ty
				constCls[inst.Dst] = inst.Cls
			}
		}
	}

	// ── Step 2: collect incoming args for each (blockLabel, paramIndex) ──────

	type paramKey struct {
		label string
		idx   int
	}
	incoming := make(map[paramKey][]Reg) // → arg regs from each predecessor

	addEdgeArgs := func(target string, args []Reg) {
		for i, a := range args {
			k := paramKey{target, i}
			incoming[k] = append(incoming[k], a)
		}
	}

	for _, b := range f.Blocks {
		switch t := b.Term.(type) {
		case *TermJmp:
			addEdgeArgs(t.Target, t.Args)
		case *TermBrIf:
			addEdgeArgs(t.Then, t.ThenArgs)
			addEdgeArgs(t.Else, t.ElseArgs)
		case *TermBrIf2:
			addEdgeArgs(t.Eq, t.EqArgs)
			addEdgeArgs(t.Lt, t.LtArgs)
			addEdgeArgs(t.Gt, t.GtArgs)
		case *TermDJNZ:
			addEdgeArgs(t.Body, t.BodyArgs)
			addEdgeArgs(t.Exit, t.ExitArgs)
		}
	}

	// ── Step 3: fold params where all incoming args are the same constant ─────

	changed := false
	for _, b := range f.Blocks {
		for pi, param := range b.Params {
			srcs := incoming[paramKey{b.Label, pi}]
			if len(srcs) == 0 {
				continue // entry-block params (function args) — no predecessors
			}

			// All sources must be the same known constant.
			cv, ok := consts[srcs[0]]
			if !ok {
				continue
			}
			allSame := true
			for _, s := range srcs[1:] {
				if v, ok2 := consts[s]; !ok2 || v != cv {
					allSame = false
					break
				}
			}
			if !allSame {
				continue
			}

			// Param is provably constant — record it for step 4.
			consts[param.Dst] = cv
			constTy[param.Dst] = param.Ty
			constCls[param.Dst] = param.Class
			changed = true
		}
	}

	if !changed {
		return false
	}

	// ── Step 4: materialise OpConst + rewrite uses for each folded param ─────
	//
	// For each block B, find which of its params are now in `consts` (were folded
	// in step 3, not just the original OpConst seeds — block param Dsts are never
	// the same reg as an OpConst Dst because AllocReg is monotonically increasing).

	// Build a set of regs that are OpConst-originated (step 1 seeds) so we
	// don't accidentally double-fold them in step 4.
	opConstRegs := make(map[Reg]bool)
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpConst {
				opConstRegs[inst.Dst] = true
			}
		}
	}

	for _, b := range f.Blocks {
		var prepend []*Inst
		for _, param := range b.Params {
			if opConstRegs[param.Dst] {
				continue // not a block param; shouldn't happen, but be safe
			}
			cv, ok := consts[param.Dst]
			if !ok {
				continue
			}

			// Emit a fresh OpConst for this param's constant value.
			nr := f.AllocReg()
			prepend = append(prepend, &Inst{
				Op:  OpConst,
				Dst: nr,
				Imm: cv,
				Ty:  constTy[param.Dst],
				Cls: constCls[param.Dst],
			})

			// Rewrite all uses of param.Dst in this block to nr.
			rewriteBlockUses(b, param.Dst, nr)

			// Make the new reg visible to further propagation.
			consts[nr] = cv
			opConstRegs[nr] = true
		}

		if len(prepend) > 0 {
			b.Insts = append(prepend, b.Insts...)
		}
	}

	return true
}

// rewriteBlockUses replaces every *use* of `old` within block b (instructions
// and the terminator) with `new_`.  Instruction Dst fields are definitions —
// never rewritten.
func rewriteBlockUses(b *Block, old, new_ Reg) {
	for _, inst := range b.Insts {
		if inst.Src[0] == old {
			inst.Src[0] = new_
		}
		if inst.Src[1] == old {
			inst.Src[1] = new_
		}
		for i, a := range inst.Args {
			if a == old {
				inst.Args[i] = new_
			}
		}
		if inst.Asm != nil {
			for i, r := range inst.Asm.Ins {
				if r == old {
					inst.Asm.Ins[i] = new_
				}
			}
		}
	}
	rewriteTermUses(b.Term, old, new_)
}

// rewriteTermUses replaces every use of `old` in terminator t with `new_`.
func rewriteTermUses(t Term, old, new_ Reg) {
	rewriteSlice := func(s []Reg) {
		for i, r := range s {
			if r == old {
				s[i] = new_
			}
		}
	}
	switch t := t.(type) {
	case *TermJmp:
		rewriteSlice(t.Args)
	case *TermBrIf:
		if t.Cond == old {
			t.Cond = new_
		}
		rewriteSlice(t.ThenArgs)
		rewriteSlice(t.ElseArgs)
	case *TermBrIf2:
		if t.Lhs == old {
			t.Lhs = new_
		}
		if t.Rhs == old {
			t.Rhs = new_
		}
		rewriteSlice(t.EqArgs)
		rewriteSlice(t.LtArgs)
		rewriteSlice(t.GtArgs)
	case *TermDJNZ:
		if t.Counter == old {
			t.Counter = new_
		}
		rewriteSlice(t.BodyArgs)
		rewriteSlice(t.ExitArgs)
	case *TermRet:
		rewriteSlice(t.Vals)
	}
}
