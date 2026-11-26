package mir2

// loopMayWriteGlobal reports whether any instruction of the loop can write a
// byte of the ordinary global g, or carries effects the pass cannot reason
// about: every call form (including memory intrinsics and conditional or
// indirect targets), inline assembly, ports, patch slots, stack pushes/pops,
// and sequential or volatile accesses. A store is admitted only when it
// provably stays inside one ordinary global other than g (storeConfinedGlobal).
// A loop rejected here may still hoist address arithmetic, which is pure.
//
// Callers have already proven that the load they want to move reads a byte
// range of g through staticGlobalLoad (so g is an ordinary, non-At,
// non-volatile module global).
func loopMayWriteGlobal(f *Func, loop map[string]bool, g *Global, instDefs map[Reg]*Inst) bool {
	if g == nil {
		return true
	}
	for _, b := range f.Blocks {
		if !loop[b.Label] {
			continue
		}
		for _, i := range b.Insts {
			// Sequential accesses preserve ordered partial observations; a
			// volatile access is externally observable. Neither may move.
			if i.Volatile || i.SequentialBytes || len(i.SequentialByteOrigins) != 0 || len(i.SequentialAddressDefs) != 0 {
				return true
			}
			switch i.Op {
			case OpPatch, OpPush, OpPop,
				OpCall, OpCallIndirect, OpCallCond,
				OpAsm, OpIn8, OpIn16, OpOut8, OpOut16:
				return true
			case OpStore:
				written, ok := storeConfinedGlobal(f, i, instDefs)
				if !ok || written == g {
					return true
				}
			}
		}
	}
	return false
}

// storeConfinedGlobal proves that every byte store writes lies inside one
// ordinary module global and returns it. Two proofs are accepted:
//
//  1. Range proof (any frontend): the address is addr_of @G plus constants
//     plus non-negative offsets whose value range (from constants, byte
//     zero-extensions, masks and small shifts/multiplies visible in the MIR)
//     keeps [offset, offset+width) inside G.
//  2. Declared-object proof (C only): the address is an InBounds ptr_add
//     whose base is addr_of @G. The C frontend sets InBounds only for a
//     subscript of a declared, complete, static-storage array object, where
//     an access outside the object is undefined (C89 3.3.6, C99 6.5.6p8). As
//     a guard against a later pass that widened the access, the store may be
//     no wider than the element stride visible in the offset, and G's size
//     must be a multiple of that stride.
//
// G must be ordinary storage that cannot overlap another global: no absolute
// (At) placement, no volatile or special storage class, and a unique name.
func storeConfinedGlobal(f *Func, store *Inst, instDefs map[Reg]*Inst) (*Global, bool) {
	if f.mod == nil || store.Op != OpStore || store.Volatile || store.SequentialBytes ||
		len(store.SequentialByteOrigins) != 0 || len(store.SequentialAddressDefs) != 0 ||
		store.Ty == nil || store.Src[0] == NoReg {
		return nil, false
	}
	width := int64(ByteWidth(store.Ty))
	if width <= 0 {
		return nil, false
	}
	word := f.mod.PointerType().Width()
	type span struct {
		g      *Global
		lo, hi int64 // byte offset range of the address within g
	}
	var walk func(r Reg, depth int) (span, bool)
	walk = func(r Reg, depth int) (span, bool) {
		i := instDefs[r]
		if depth > 16 || i == nil || i.ClsHard || i.Ty == nil || i.Ty.Width() != word {
			return span{}, false
		}
		switch i.Op {
		case OpAddrOf:
			g := ordinaryUniqueGlobal(f.mod, i.Sym)
			return span{g, 0, 0}, g != nil
		case OpMove:
			return walk(i.Src[0], depth+1)
		case OpField, OpPtrBump:
			if i.Imm < 0 || i.Src[0] == NoReg {
				return span{}, false
			}
			s, ok := walk(i.Src[0], depth+1)
			return span{s.g, s.lo + i.Imm, s.hi + i.Imm}, ok
		case OpAdd, OpPtrAdd:
			if i.Src[0] == NoReg || i.Src[1] == NoReg {
				return span{}, false
			}
			sides := [2]int{0, 1}
			if i.Op == OpPtrAdd {
				sides = [2]int{0, 0} // the base is Src[0]; the offset is never a base
			}
			for _, side := range sides {
				s, ok := walk(i.Src[side], depth+1)
				if !ok {
					continue
				}
				lo, hi, ok := valueRangeU16(i.Src[1-side], instDefs, 0)
				if !ok {
					return span{}, false
				}
				return span{s.g, s.lo + lo, s.hi + hi}, true
			}
		}
		return span{}, false
	}
	addr := store.Src[0]
	for {
		i := instDefs[addr]
		if i == nil || i.Op != OpMove || i.ClsHard || i.Ty == nil || i.Ty.Width() != word {
			break
		}
		addr = i.Src[0]
	}
	if s, ok := walk(addr, 0); ok {
		if size := int64(ByteWidth(s.g.Ty)); s.lo >= 0 && s.hi+width <= size {
			return s.g, true
		}
	}
	// Declared-object proof.
	p := instDefs[addr]
	if p == nil || p.Op != OpPtrAdd || !p.InBounds || p.ClsHard || p.Ty == nil || p.Ty.Width() != word {
		return nil, false
	}
	base := p.Src[0]
	for {
		i := instDefs[base]
		if i == nil || i.Op != OpMove || i.ClsHard || i.Ty == nil || i.Ty.Width() != word {
			break
		}
		base = i.Src[0]
	}
	b := instDefs[base]
	if b == nil || b.Op != OpAddrOf || b.ClsHard || b.Ty == nil || b.Ty.Width() != word {
		return nil, false
	}
	g := ordinaryUniqueGlobal(f.mod, b.Sym)
	if g == nil {
		return nil, false
	}
	stride := offsetStride(p.Src[1], instDefs)
	size := int64(ByteWidth(g.Ty))
	if width > stride || size < stride || size%stride != 0 {
		return nil, false
	}
	return g, true
}

// offsetStride returns the element stride a subscript offset visibly carries:
// 2^k for a word shift left by k, n for a word multiply by a power of two n,
// and 1 otherwise. Word arithmetic is modular, so only powers of two are
// trusted to keep the offset a multiple of the stride.
func offsetStride(r Reg, instDefs map[Reg]*Inst) int64 {
	i := instDefs[r]
	if i == nil || i.ClsHard || i.Ty == nil || i.Ty.Width() != 16 {
		return 1
	}
	constant := func(r Reg) (int64, bool) {
		c := instDefs[r]
		if c == nil || c.Op != OpConst || c.ClsHard {
			return 0, false
		}
		return maskToWidth(c.Imm, c.Ty), true
	}
	switch i.Op {
	case OpShl:
		if k, ok := constant(i.Src[1]); ok && k >= 1 && k <= 3 {
			return 1 << k
		}
	case OpMul:
		for side := 0; side < 2; side++ {
			if n, ok := constant(i.Src[side]); ok && (n == 2 || n == 4 || n == 8) {
				return n
			}
		}
	}
	return 1
}

// valueRangeU16 bounds an unsigned integer value from what the MIR shows:
// constants, zero-extension of a byte, a byte-typed value, AND with a
// constant mask, and left shift or multiply by a small constant. Anything
// else is unbounded (not ok). The range is exact integer arithmetic; a
// result that could exceed 16 bits is refused, so no wrap is hidden.
func valueRangeU16(r Reg, instDefs map[Reg]*Inst, depth int) (int64, int64, bool) {
	i := instDefs[r]
	if depth > 16 || i == nil || i.ClsHard || i.Ty == nil {
		return 0, 0, false
	}
	w := i.Ty.Width()
	if w != 8 && w != 16 {
		return 0, 0, false
	}
	full := int64(1)<<w - 1
	switch i.Op {
	case OpConst:
		v := maskToWidth(i.Imm, i.Ty)
		return v, v, true
	case OpExt:
		if i.SrcTy == nil || i.SrcTy.Width() != 8 {
			return 0, 0, false
		}
		lo, hi, ok := valueRangeU16(i.Src[0], instDefs, depth+1)
		if !ok {
			return 0, 255, true
		}
		return lo, hi, true
	case OpMove:
		if lo, hi, ok := valueRangeU16(i.Src[0], instDefs, depth+1); ok && hi <= full {
			return lo, hi, true
		}
	case OpAnd:
		for side := 0; side < 2; side++ {
			c := instDefs[i.Src[side]]
			if c != nil && c.Op == OpConst && !c.ClsHard {
				return 0, maskToWidth(c.Imm, i.Ty), true
			}
		}
	case OpShl, OpMul:
		c := instDefs[i.Src[1]]
		x := i.Src[0]
		if i.Op == OpMul && (c == nil || c.Op != OpConst) {
			c, x = instDefs[i.Src[0]], i.Src[1]
		}
		if c == nil || c.Op != OpConst || c.ClsHard {
			break
		}
		k := maskToWidth(c.Imm, c.Ty)
		factor := k
		if i.Op == OpShl {
			if k > 15 {
				break
			}
			factor = 1 << k
		}
		lo, hi, ok := valueRangeU16(x, instDefs, depth+1)
		if ok && hi*factor <= full {
			return lo * factor, hi * factor, true
		}
	}
	if w == 8 {
		return 0, 255, true
	}
	return 0, 0, false
}

// ordinaryUniqueGlobal returns the only module global named sym when it is
// ordinary data storage that no other global can overlap.
func ordinaryUniqueGlobal(m *Module, sym string) *Global {
	var found *Global
	for idx := range m.Globals {
		g := &m.Globals[idx]
		if g.Name != sym {
			continue
		}
		if found != nil {
			return nil
		}
		found = g
	}
	if found == nil || found.Ty == nil || found.At != nil || found.Storage != StorageNormal || found.Volatile || HasVolatileStorage(found.Ty) || ByteWidth(found.Ty) <= 0 {
		return nil
	}
	return found
}
