package mir2

// narrowTruncatedByteArithmetic computes demanded low bytes modulo 256.
// Add/sub/bitwise/left shift commute with truncation; right shifts require
// a proven zero-extended byte. Wide definitions retain all other consumers.
func narrowTruncatedByteArithmetic(f *Func) bool {
	if f.Attrs.FallibleCY {
		return false
	}
	for _, r := range f.Contract.Returns {
		if r.Class == ClassFlag {
			return false
		}
	}
	counts := map[Reg]int{}
	for _, p := range f.Contract.Params {
		counts[p.Reg]++
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			counts[p.Dst]++
		}
		for _, i := range b.Insts {
			if i.Op == OpAsm {
				return false
			}
			if i.Dst != NoReg {
				counts[i.Dst]++
			}
		}
	}
	// Restrict proofs to preceding, uniquely defined values in the same block.
	// Clone cheap operations rather than changing any surviving wide consumers.
	word := func(t Ty) bool { return BaseOf(t) == TyI16 || BaseOf(t) == TyU16 }
	for r := range counts {
		if f.nextReg <= r {
			f.nextReg = r + 1
		}
	}
	changed := false
	for _, b := range f.Blocks {
		defs := map[Reg]*Inst{}
		cache := map[Reg]Reg{}
		var out []*Inst
		for _, i := range b.Insts {
			var safe func(Reg, Ty, int) bool
			safe = func(r Reg, ty Ty, depth int) bool {
				v := defs[r]
				if depth > 12 || counts[r] != 1 || v == nil || v.ClsHard || v.Ty != ty || !word(ty) {
					return false
				}
				switch v.Op {
				case OpExt:
					return BaseOf(v.SrcTy) == TyU8 && counts[v.Src[0]] == 1
				case OpConst:
					return true
				case OpAdd, OpSub, OpAnd, OpOr, OpXor:
					return safe(v.Src[0], ty, depth+1) && safe(v.Src[1], ty, depth+1)
				case OpShl, OpShr, OpSar:
					k := defs[v.Src[1]]
					if k == nil || k.Op != OpConst || k.ClsHard || counts[k.Dst] != 1 || k.Imm < 0 || k.Imm >= 8 {
						return false
					}
					if v.Op != OpShl {
						x := defs[v.Src[0]]
						// A right shift can bring high bits into the low byte.
						if x == nil || x.Op != OpExt || BaseOf(x.SrcTy) != TyU8 {
							return false
						}
					}
					return safe(v.Src[0], ty, depth+1)
				}
				return false
			}
			var emit func(Reg) Reg
			emit = func(r Reg) Reg {
				if low, ok := cache[r]; ok {
					return low
				}
				v := defs[r]
				if v.Op == OpExt {
					return v.Src[0]
				}
				x := &Inst{Op: v.Op, Ty: TyU8, SrcTy: TyU8, Cls: ClassGeneral, SrcFile: i.SrcFile, SrcLine: i.SrcLine}
				if v.Op == OpConst {
					x.Imm = int64(uint8(v.Imm))
					x.SrcTy = nil
				} else {
					x.Src[0] = emit(v.Src[0])
					// Shift counts were proved constant in [0,7], so their byte
					// representation has exactly the same value.
					x.Src[1] = emit(v.Src[1])
					if x.Op == OpSar {
						x.Op = OpShr // zero extension proves a nonnegative word
					}
				}
				x.Dst = f.AllocReg()
				cache[r] = x.Dst
				out = append(out, x)
				return x.Dst
			}
			if !i.ClsHard {
				root := defs[i.Src[0]]
				// Extensions and constants are leaves of the arithmetic proof,
				// not rewrite roots. foldTruncOfExtension handles standalone
				// conversion identities; codegen also recognizes memory BIT tests.
				if i.Op == OpTrunc && counts[i.Dst] == 1 && root != nil && root.Op != OpExt && root.Op != OpConst && BaseOf(i.Ty) == TyU8 && word(i.SrcTy) && safe(i.Src[0], i.SrcTy, 0) {
					r := i.Src[0]
					before := len(out)
					low := emit(r)
					// Keep the destination and its class/source location. For a new
					// root, put the byte operation directly at the old truncation.
					if len(out) > before && out[len(out)-1].Dst == low && cache[r] == low {
						x := out[len(out)-1]
						out = out[:len(out)-1]
						i.Op, i.Src, i.Imm, i.SrcTy = x.Op, x.Src, x.Imm, x.SrcTy
						cache[r] = i.Dst
					} else {
						i.Op, i.Src, i.SrcTy = OpMove, [2]Reg{low}, TyU8
					}
					changed = true
				} else if (i.Op == OpStore && BaseOf(i.Ty) == TyU8) || (i.Op == OpCmp && BaseOf(i.SrcTy) == TyU8) {
					// These consumers explicitly observe byte operands. A word
					// comparison is never a low-byte demand.
					start := 0
					if i.Op == OpStore {
						start = 1
					}
					for n := start; n < 2; n++ {
						v := defs[i.Src[n]]
						// Even a byte-typed comparison must preserve the full
						// value: only strip a proven zero extension here.
						if v != nil && (i.Op != OpCmp || v.Op == OpExt) && safe(i.Src[n], v.Ty, 0) {
							i.Src[n] = emit(i.Src[n])
							changed = true
						}
					}
				}
			}
			out = append(out, i)
			if i.Dst != NoReg {
				defs[i.Dst] = i
			}
		}
		b.Insts = out
	}
	return changed
}
