package mir2

// Pointer-offset reassociation.
//
//	t = add a, b          (u16/i16, t used only here, flags unobserved)
//	p = ptr_add base, t
//
// becomes
//
//	t = ptr_add base, a
//	p = ptr_add t, b
//
// Both forms compute base + a + b modulo 2^16 (ptr_add and a 16-bit add both
// wrap), so the rewrite is exact for every input, including sums that wrap
// past 0xFFFF.  On the Z80 the second form keeps the pointer in HL for two
// `ADD HL,rr` steps; the first needs the sum a+b in HL while the base already
// lives there, which costs a PUSH/POP round trip per step (fat12_decode:
// `cluster + cluster/2` indexed from a pointer parameter).
//
// The rewrite is deliberately narrow, so that it cannot hide a shape a later
// consumer folds better:
//
//   - a constant operand is left alone: `ptr_add @sym, (add i, k)` is folded
//     to `(sym+k) + i` by the codegen (ptrAddConstantFolds), and `field`
//     offsets already carry constants;
//   - a link-time base (addr_of) is left alone for the same reason: the
//     ptr_add names the symbol as an immediate (deadAddressConstants);
//   - the add and the ptr_add must be in one block, the add's only use the
//     ptr_add, and the base defined before the add (so the rewritten first
//     step can sit at the add's position);
//   - the base must die at the ptr_add (not read later, not live out of the
//     block), so the first step can take the base's home.
//
// It runs after the affine pointer-walk, counted-pointer and adjacent-pointer
// passes (pipeline.go), so the shapes those recognise are never reassociated
// before they look.
func ReassociatePointerOffsets(f *Func) int {
	if f == nil || f.Attrs.HasAsm {
		return 0
	}
	defs := map[Reg]*Inst{}
	defCount := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defCount[p.Dst] += 2
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst] = x
				defCount[x.Dst]++
			}
			for _, r := range x.ExtraRets {
				defCount[r] += 2
			}
		}
	}
	for _, p := range f.Contract.Params {
		defCount[p.Reg] += 2
	}
	constant := func(r Reg) bool {
		d := defs[r]
		return d != nil && (d.Op == OpConst || d.Op == OpAddrOf)
	}
	uses := countRegUses(f)
	lr := ComputeLiveness(f)
	changed := 0
	for _, b := range f.Blocks {
		liveOut := lr.LiveOutOf(f, b)
		pos := map[*Inst]int{}
		for i, x := range b.Insts {
			pos[x] = i
		}
		for _, x := range b.Insts {
			if x.Op != OpPtrAdd || x.Dst == NoReg || x.Ty == nil || x.Ty.Width() != 16 {
				continue
			}
			base, t := x.Src[0], x.Src[1]
			add := defs[t]
			if add == nil || add.Op != OpAdd || defCount[t] != 1 || uses[t] != 1 || add.ClsHard {
				continue
			}
			ai, inBlock := pos[add]
			if !inBlock || ai >= pos[x] || add.Ty == nil || add.Ty.Width() != 16 {
				continue
			}
			a, c := add.Src[0], add.Src[1]
			if constant(a) || constant(c) || constant(base) || base == t {
				continue
			}
			// The first step is placed where the add was, so the base must
			// already be defined there.
			if bd := defs[base]; bd != nil {
				if bi, same := pos[bd]; same && bi > ai {
					continue
				}
			}
			// The base must die here: the gain is that the first step takes
			// the base's own home and both steps advance it in place.  A
			// base that stays live (above all one carried around a loop,
			// grace_intern_checked region_eq) would keep its pair and add a
			// second pointer temporary next to it.
			if baseLiveAfter(b, x, base, liveOut) {
				continue
			}
			if !wordMaskFlagsUnobserved(f, t) {
				continue
			}
			add.Op = OpPtrAdd
			add.Src = [2]Reg{base, a}
			add.Ty = x.Ty
			add.Cls = x.Cls
			add.SrcTy = nil
			add.InBounds = false
			x.Src = [2]Reg{t, c}
			// Neither step is the source subscript any more (inst.go).
			x.InBounds = false
			changed++
		}
	}
	return changed
}

// baseLiveAfter reports whether r is read after inst in b or live out of b.
func baseLiveAfter(b *Block, inst *Inst, r Reg, liveOut *RegSet) bool {
	if liveOut != nil && liveOut.Has(r) {
		return true
	}
	after := false
	for _, x := range b.Insts {
		if after {
			for _, u := range x.Uses() {
				if u == r {
					return true
				}
			}
		}
		if x == inst {
			after = true
		}
	}
	if b.Term != nil {
		for _, u := range b.Term.termUses() {
			if u == r {
				return true
			}
		}
	}
	return false
}
