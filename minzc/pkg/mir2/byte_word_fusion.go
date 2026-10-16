package mir2

// FuseSequentialByteWords reduces same-block little-endian byte assembly to an
// explicitly sequential word access. No side effect is crossed; removed values
// are exclusively used by the matched DAG. It runs on SSA before allocation.
func FuseSequentialByteWords(f *Func) int {
	if f == nil || verifyFunc(f) != nil {
		return 0
	}
	// Opaque assembly can observe physical flags or registers that are absent
	// from SSA use counts. Such functions stay outside this pass.
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.Op == OpAsm || x.Volatile {
				return 0
			}
		}
	}
	// Native word selection and the adjacency proof use 16-bit addresses.
	types := map[Reg]Ty{}
	for _, p := range f.Contract.Params {
		types[p.Reg] = p.Ty
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			types[p.Dst] = p.Ty
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				types[x.Dst] = x.Ty
			}
		}
	}
	changed := 0
	for _, b := range f.Blocks {
		again := true
		for again {
			again = false
			uses := countRegUses(f)
			defs := map[Reg]int{}
			for i, x := range b.Insts {
				if x.Dst != NoReg {
					defs[x.Dst] = i
				}
			}
			constant := func(r Reg, n int64) bool {
				i, ok := defs[r]
				return ok && b.Insts[i].Op == OpConst && truncate(Value{I: b.Insts[i].Imm}, b.Insts[i].Ty).I == n
			}
			// Pointer offset chains must dominate the access and consist of total
			// pointer/constant arithmetic, with the original pointer width retained.
			var pointer func(Reg, int) (Reg, int64, bool)
			pointer = func(r Reg, before int) (Reg, int64, bool) {
				if ty, ok := types[r]; !ok || ty.Width() != 16 {
					return 0, 0, false
				}
				i, ok := defs[r]
				if !ok {
					return r, 0, true
				}
				if i >= before {
					return 0, 0, false
				}
				x := b.Insts[i]
				if x.Op == OpMove {
					return pointer(x.Src[0], i)
				}
				if x.Op == OpPtrAdd {
					j, ok := defs[x.Src[1]]
					if !ok || j >= i {
						return 0, 0, false
					}
					c := b.Insts[j]
					if c.Op != OpConst {
						return 0, 0, false
					}
					root, off, ok := pointer(x.Src[0], i)
					return root, off + truncate(Value{I: c.Imm}, c.Ty).I, ok
				}
				return r, 0, true
			}
			for out, inst := range b.Insts {
				if inst.Op != OpOr || inst.Ty != TyU16 {
					continue
				}
				consumed := map[int]bool{}
				var low, high *Inst
				var li, hi int
				decode := func(r Reg, shifted bool) bool {
					i, ok := defs[r]
					if !ok || i >= out {
						return false
					}
					x := b.Insts[i]
					if shifted {
						if x.Op != OpShl || x.Ty != TyU16 || !constant(x.Src[1], 8) {
							return false
						}
						if uses[r] != 1 {
							return false
						}
						consumed[i] = true
						r = x.Src[0]
						i, ok = defs[r]
						if !ok {
							return false
						}
						x = b.Insts[i]
					}
					if x.Op != OpExt || x.Ty != TyU16 || x.SrcTy != TyU8 || uses[r] != 1 {
						return false
					}
					consumed[i] = true
					r = x.Src[0]
					i, ok = defs[r]
					if !ok {
						return false
					}
					x = b.Insts[i]
					if x.Op != OpLoad || x.Ty != TyU8 || uses[r] != 1 {
						return false
					}
					consumed[i] = true
					if shifted {
						high = x
						hi = i
					} else {
						low = x
						li = i
					}
					return true
				}
				if !(decode(inst.Src[0], true) && decode(inst.Src[1], false)) {
					consumed = map[int]bool{}
					if !(decode(inst.Src[1], true) && decode(inst.Src[0], false)) {
						continue
					}
				}
				if li >= hi {
					continue
				}
				root, lo, ok := pointer(low.Src[0], li)
				if !ok {
					continue
				}
				other, hio, ok := pointer(high.Src[0], hi)
				if !ok || root != other || hio != lo+1 {
					continue
				}
				safe := true
				for j := li; j <= out; j++ {
					x := b.Insts[j]
					if consumed[j] || j == out {
						continue
					}
					switch x.Op {
					case OpConst, OpMove, OpExt, OpTrunc, OpShl, OpPtrAdd:
					default:
						safe = false
					}
				}
				if !safe {
					continue
				}
				wide := *inst
				wide.Op = OpLoad
				wide.Src = [2]Reg{low.Src[0], NoReg}
				wide.SequentialBytes = true
				wide.SequentialByteOrigins = []*Inst{low, high}
				wide.SequentialAddressDefs = sequentialAddressDefs(b, low.Src[0], high.Src[0])
				// Keep the original low-access position and retain independent address
				// proof metadata rather than extra executable pointer computations.
				next := []*Inst{}
				for j, x := range b.Insts {
					if j == li {
						next = append(next, &wide)
					} else if j != out && !consumed[j] {
						next = append(next, x)
					}
				}
				b.Insts = next
				DeadStoreElim(f)
				changed++
				again = true
				break
			}
			if again {
				continue
			}
			// Store sequence: low trunc(val), then high trunc(val >> 8). Masks255
			// before trunc are redundant but retained unless exclusively consumed.
			for li, low := range b.Insts {
				if low.Op != OpStore || low.Ty != TyU8 {
					continue
				}
				consumed := map[int]bool{}
				var byteValue func(Reg, bool) (Reg, bool)
				byteValue = func(r Reg, high bool) (Reg, bool) {
					i, ok := defs[r]
					if !ok {
						return 0, false
					}
					x := b.Insts[i]
					if x.Op != OpTrunc || x.Ty != TyU8 || x.SrcTy != TyU16 || uses[r] != 1 {
						return 0, false
					}
					consumed[i] = true
					r = x.Src[0]
					i, ok = defs[r]
					if ok && b.Insts[i].Op == OpAnd {
						x = b.Insts[i]
						var other Reg
						if constant(x.Src[0], 255) {
							other = x.Src[1]
						} else if constant(x.Src[1], 255) {
							other = x.Src[0]
						} else {
							return 0, false
						}
						if uses[r] != 1 {
							return 0, false
						}
						consumed[i] = true
						r = other
						i, ok = defs[r]
					}
					if high {
						if !ok {
							return 0, false
						}
						x = b.Insts[i]
						if x.Op != OpShr || x.Ty != TyU16 || !constant(x.Src[1], 8) || uses[r] != 1 {
							return 0, false
						}
						consumed[i] = true
						r = x.Src[0]
					}
					return r, true
				}
				val, ok := byteValue(low.Src[1], false)
				if !ok {
					continue
				}
				root, lo, ok := pointer(low.Src[0], li)
				if !ok {
					continue
				}
				for hi := li + 1; hi < len(b.Insts); hi++ {
					x := b.Insts[hi]
					if x.Op == OpStore {
						if x.Ty != TyU8 {
							break
						}
						other, off, ok := pointer(x.Src[0], hi)
						if !ok || root != other || off != lo+1 {
							break
						}
						highVal, ok := byteValue(x.Src[1], true)
						if !ok || highVal != val {
							break
						}
						safe := true
						for j := li + 1; j < hi; j++ {
							switch b.Insts[j].Op {
							case OpConst, OpMove, OpExt, OpTrunc, OpShr, OpAnd, OpPtrAdd:
							default:
								safe = false
							}
						}
						if !safe {
							break
						}
						wide := *low
						wide.Ty = TyU16
						wide.Src[1] = val
						wide.SequentialBytes = true
						wide.SequentialByteOrigins = []*Inst{low, x}
						wide.SequentialAddressDefs = sequentialAddressDefs(b, low.Src[0], x.Src[0])
						next := []*Inst{}
						for j, x := range b.Insts {
							if j == li {
								next = append(next, &wide)
							} else if j != hi && !consumed[j] {
								next = append(next, x)
							}
						}
						b.Insts = next
						DeadStoreElim(f)
						changed++
						again = true
						break
					}
					switch x.Op {
					case OpConst, OpMove, OpExt, OpTrunc, OpShr, OpAnd, OpPtrAdd:
					default:
						hi = len(b.Insts)
					}
				}
				if again {
					break
				}
			}
		}
	}
	return changed
}

// sequentialByteOriginsValid proves byte-address adjacency from retained SSA,
// rather than treating provenance metadata as authority.
func sequentialByteOriginsValid(f *Func, x *Inst) bool {
	if !x.SequentialBytes {
		return len(x.SequentialByteOrigins) == 0 && len(x.SequentialAddressDefs) == 0
	}
	if (x.Op != OpLoad && x.Op != OpStore) || x.Ty != TyU16 || len(x.SequentialByteOrigins) != 2 {
		return false
	}
	defs := map[Reg]*Inst{}
	types := map[Reg]Ty{}
	for _, p := range f.Contract.Params {
		types[p.Reg] = p.Ty
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			types[p.Dst] = p.Ty
		}
		for _, i := range b.Insts {
			if i.Dst != NoReg {
				defs[i.Dst] = i
				types[i.Dst] = i.Ty
			}
		}
	}
	for _, i := range x.SequentialAddressDefs {
		if i == nil || i.Dst == NoReg || i.SequentialBytes || len(i.SequentialByteOrigins) > 0 || (i.Op != OpConst && i.Op != OpMove && i.Op != OpPtrAdd) {
			return false
		}
		if _, ok := defs[i.Dst]; !ok {
			defs[i.Dst] = i
			types[i.Dst] = i.Ty
		}
	}
	var address func(Reg, map[Reg]bool) (Reg, uint16, bool)
	address = func(r Reg, seen map[Reg]bool) (Reg, uint16, bool) {
		if seen[r] || types[r] == nil || types[r].Width() != 16 {
			return 0, 0, false
		}
		seen[r] = true
		i := defs[r]
		if i == nil {
			return r, 0, true
		}
		if i.Op == OpMove {
			return address(i.Src[0], seen)
		}
		if i.Op == OpPtrAdd {
			c := defs[i.Src[1]]
			if c == nil || c.Op != OpConst {
				return 0, 0, false
			}
			root, off, ok := address(i.Src[0], seen)
			return root, off + uint16(truncate(Value{I: c.Imm}, c.Ty).I), ok
		}
		return r, 0, true
	}
	root, off, ok := address(x.Src[0], map[Reg]bool{})
	if !ok {
		return false
	}
	for j, o := range x.SequentialByteOrigins {
		if o == nil || o.Op != x.Op || o.Ty != TyU8 || o.SequentialBytes || len(o.SequentialByteOrigins) != 0 || o.Sym != "" || o.Asm != nil || len(o.Args) != 0 {
			return false
		}
		r, a, ok := address(o.Src[0], map[Reg]bool{})
		if !ok || r != root || a != off+uint16(j) {
			return false
		}
	}
	return true
}

// Retain only total pure address definitions; they are proof metadata, not code.
func sequentialAddressDefs(b *Block, regs ...Reg) []*Inst {
	defs := map[Reg]*Inst{}
	for _, i := range b.Insts {
		defs[i.Dst] = i
	}
	seen := map[Reg]bool{}
	var out []*Inst
	var visit func(Reg)
	visit = func(r Reg) {
		if seen[r] {
			return
		}
		seen[r] = true
		i := defs[r]
		if i == nil {
			return
		}
		switch i.Op {
		case OpConst:
		case OpMove:
			visit(i.Src[0])
		case OpPtrAdd:
			visit(i.Src[0])
			visit(i.Src[1])
		default:
			return
		}
		copy := *i
		out = append(out, &copy)
	}
	for _, r := range regs {
		visit(r)
	}
	return out
}
