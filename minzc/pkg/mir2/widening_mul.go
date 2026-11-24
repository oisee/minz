package mir2

// NarrowWideningByteMul rewrites a word multiply of two zero-extended
// unsigned bytes so that it reads the bytes themselves:
//
//	%a = ext %x : u8 -> u16
//	%b = ext %y : u8 -> u16          =>   %p = mul %x, %y : u16
//	%p = mul %a, %b : u16
//
// The product of two bytes is below 65536, so the word multiply of the
// zero-extended operands and the byte-operand form are the same value. An
// unsigned narrow operand of word arithmetic is already implicitly
// zero-extended in MIR (the VM keeps byte values in 0..255, the verifier only
// requires explicit extension of signed narrow operands), so the result is
// ordinary MIR. Z80 codegen multiplies such operands through its byte paths.
//
// The point is register allocation: without it each extension is a 16-bit
// value that needs a pair home (or IX, or a word spill) for bits the multiply
// never reads. The rewrite runs only for Z80 emission, just before allocation;
// other MIR consumers (C/QBE/LLVM/WASM/6502 emitters) never see it.
//
// Admission is deliberately narrow: a 16-bit multiply, both operands defined
// by `ext` from u8 in the same block, each extension used only by this
// multiply, the byte source unsigned u8 in every definition, and no
// redefinition of the byte source between its extension and the multiply.
// Signed bytes (sext, or i8 sources) never qualify. Returns the number of
// multiplies rewritten.
func NarrowWideningByteMul(f *Func) int {
	if f == nil {
		return 0
	}
	var uses map[Reg]int
	byteDefs := map[Reg]bool{}
	unsignedByte := func(r Reg) bool {
		if v, ok := byteDefs[r]; ok {
			return v
		}
		byteDefs[r] = unsignedByteEveryDefinition(f, r)
		return byteDefs[r]
	}
	n := 0
	for _, b := range f.Blocks {
		defAt := map[Reg]int{}
		retired := map[*Inst]bool{}
		for i, inst := range b.Insts {
			if inst.Dst != NoReg {
				defAt[inst.Dst] = i
			}
			if inst.Op != OpMul || inst.Ty == nil || inst.Ty.Width() != 16 || inst.ClsHard {
				continue
			}
			var exts [2]*Inst
			ok := true
			for k := 0; k < 2 && ok; k++ {
				at, found := defAt[inst.Src[k]]
				if !found {
					ok = false
					break
				}
				e := b.Insts[at]
				if retired[e] || e.Op != OpExt || e.SrcTy != TyU8 || e.Ty == nil || e.Ty.Width() != 16 || e.Src[0] == NoReg {
					ok = false
					break
				}
				if !unsignedByte(e.Src[0]) || redefinedBetween(b, at, i, e.Src[0]) {
					ok = false
					break
				}
				exts[k] = e
			}
			if !ok {
				continue
			}
			if uses == nil {
				uses = countRegUses(f)
			}
			// Each extension's only readers are this multiply's operand slots
			// (one extension may feed both: u16(x) * u16(x)).
			want := map[Reg]int{}
			want[inst.Src[0]]++
			want[inst.Src[1]]++
			for r, c := range want {
				if uses[r] != c {
					ok = false
				}
			}
			if !ok {
				continue
			}
			inst.Src[0], inst.Src[1] = exts[0].Src[0], exts[1].Src[0]
			retired[exts[0]], retired[exts[1]] = true, true
			n++
		}
		if len(retired) > 0 {
			kept := b.Insts[:0]
			for _, inst := range b.Insts {
				if !retired[inst] {
					kept = append(kept, inst)
				}
			}
			b.Insts = kept
		}
	}
	return n
}

// redefinedBetween reports whether r is written by b.Insts[from+1 .. to-1].
func redefinedBetween(b *Block, from, to int, r Reg) bool {
	for _, inst := range b.Insts[from+1 : to] {
		if inst.Dst == r {
			return true
		}
		for _, x := range inst.ExtraRets {
			if x == r {
				return true
			}
		}
		if inst.Asm != nil {
			for _, x := range inst.Asm.Outs {
				if x == r {
					return true
				}
			}
		}
	}
	return false
}

// unsignedByteEveryDefinition reports whether every definition of r has the
// unsigned 8-bit type. Definitions without a type of their own (extra call
// results, inline-asm outputs) do not qualify.
func unsignedByteEveryDefinition(f *Func, r Reg) bool {
	isU8 := func(t Ty) bool { return t != nil && t.Width() == 8 && !IsSigned(t) }
	defined := false
	for _, p := range f.Contract.Params {
		if p.Reg == r {
			if !isU8(p.Ty) {
				return false
			}
			defined = true
		}
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			if p.Dst == r {
				if !isU8(p.Ty) {
					return false
				}
				defined = true
			}
		}
		for _, inst := range b.Insts {
			for _, x := range inst.ExtraRets {
				if x == r {
					return false
				}
			}
			if inst.Asm != nil {
				for _, x := range inst.Asm.Outs {
					if x == r {
						return false
					}
				}
			}
			if inst.Dst == r {
				if !isU8(inst.Ty) {
					return false
				}
				defined = true
			}
		}
	}
	return defined
}
