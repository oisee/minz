package mir2

import "reflect"

// StrengthReduceCountedPointers threads base+i through loop arguments as a
// pointer recurrence. The admitted loop starts at zero, increments an unsigned
// byte index by one, and enters its single body only while index<count (u8).
// Thus each executed index is at most 254 and never wraps before termination.
// Pointer arithmetic remains modulo 65536, including the final unused bump.
// Memory operations keep their original order: overlapping copies are allowed.
func StrengthReduceCountedPointers(f *Func) int {
	if f == nil || verifyFunc(f) != nil || f.Attrs.FallibleCY || f.Attrs.HasAsm {
		return 0
	}
	types := map[Reg]Ty{}
	params := map[Reg]bool{}
	for _, p := range f.Contract.Params {
		types[p.Reg] = p.Ty
		params[p.Reg] = !p.ClassHard
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			types[p.Dst] = p.Ty
		}
		for _, x := range b.Insts {
			// Physical state and calls cannot be inferred from the SSA loop alone.
			if x.SequentialBytes || len(x.SequentialAddressDefs) != 0 || len(x.SequentialByteOrigins) != 0 || x.Op == OpPatch || x.Op == OpPatchSlot || x.Op == OpLoadPatched || x.Op == OpAsm || x.Op == OpCall || x.Op == OpCallIndirect || x.Op == OpCallCond || x.Volatile || x.FallibleCY {
				return 0
			}
			if x.Dst != NoReg {
				types[x.Dst] = x.Ty
			}
		}
	}
	changed := 0
	for _, head := range f.Blocks {
		br, ok := head.Term.(*TermCondRet)
		if !ok || len(br.ThenArgs) != 0 || len(head.Insts) != 1 {
			continue
		}
		cmp := head.Insts[0]
		if cmp.Op != OpCmp || cmp.Dst != br.Cond || cmp.Cond != CmpUlt || cmp.ClsHard || cmp.SrcTy != nil && cmp.SrcTy != TyU8 {
			continue
		}
		idx, bound := cmp.Src[0], cmp.Src[1]
		if types[idx] != TyU8 || types[bound] != TyU8 {
			continue
		}
		body := f.BlockByLabel(br.Then)
		if body == nil || body == head || len(body.Params) != 0 || countPredecessors(f, body.Label) != 1 {
			continue
		}
		back, ok := body.Term.(*TermJmp)
		if !ok || back.Target != head.Label || len(back.Args) != len(head.Params) {
			continue
		}
		var entry *Block
		var incoming *TermJmp
		bad := false
		for _, b := range f.Blocks {
			for _, target := range b.Successors() {
				if target != head.Label || b == body {
					continue
				}
				j, ok := b.Term.(*TermJmp)
				if !ok || entry != nil || b != f.Blocks[0] || len(j.Args) != len(head.Params) {
					bad = true
					break
				}
				entry, incoming = b, j
			}
		}
		if bad || entry == nil {
			continue
		}
		entryDefs, bodyDefs := map[Reg]*Inst{}, map[Reg]*Inst{}
		for _, x := range entry.Insts {
			entryDefs[x.Dst] = x
		}
		for _, x := range body.Insts {
			bodyDefs[x.Dst] = x
		}
		constant := func(d map[Reg]*Inst, r Reg, n int64) bool {
			x := d[r]
			return x != nil && x.Op == OpConst && !x.ClsHard && x.Ty == TyU8 && x.Imm == n
		}
		indexSlot := -1
		for i, p := range head.Params {
			if p.Dst == idx {
				indexSlot = i
			}
		}
		if indexSlot < 0 || !constant(entryDefs, incoming.Args[indexSlot], 0) {
			continue
		}
		next := bodyDefs[back.Args[indexSlot]]
		if next == nil || next.Op != OpAdd || next.ClsHard || next.Ty != TyU8 || next.Src[0] != idx || !constant(bodyDefs, next.Src[1], 1) {
			continue
		}
		// The bound and pointer bases must be invariant function parameters,
		// optionally threaded unchanged through the header.
		root := func(r Reg) Reg {
			if params[r] {
				return r
			}
			for i, p := range head.Params {
				if p.Dst == r && back.Args[i] == r && params[incoming.Args[i]] && p.Ty == types[incoming.Args[i]] {
					return incoming.Args[i]
				}
			}
			return NoReg
		}
		if root(bound) == NoReg {
			continue
		}
		type candidate struct {
			x    *Inst
			base Reg
		}
		var candidates []candidate
		for _, x := range body.Insts {
			if x.Op != OpPtrAdd || x.Ty != TyPtr || !plainCountedPointerInst(x) {
				continue
			}
			offset := x.Src[1]
			if ext := bodyDefs[offset]; ext != nil && ext.Op == OpExt && ext.SrcTy == TyU8 && ext.Ty == TyU16 && !ext.ClsHard {
				offset = ext.Src[0]
			}
			base := root(x.Src[0])
			if offset == idx && base != NoReg && types[base] == TyPtr {
				candidates = append(candidates, candidate{x, base})
			}
		}
		if len(candidates) == 0 {
			continue
		}
		// Collapse invariant header forwarding as well: otherwise dead cycles
		// keep original bases live and cause unnecessary physical parallel copies.
		invariants := map[Reg]Reg{}
		needed := map[Reg]bool{bound: true}
		for _, c := range candidates {
			needed[c.x.Src[0]] = true
		}
		for _, p := range head.Params {
			if !needed[p.Dst] {
				continue
			}
			if r := root(p.Dst); r != NoReg && r != p.Dst {
				invariants[p.Dst] = r
			}
		}
		for _, b := range f.Blocks {
			for _, x := range b.Insts {
				x.Src[0] = remapReg(x.Src[0], invariants)
				x.Src[1] = remapReg(x.Src[1], invariants)
			}
			remapTermRegs(b.Term, invariants)
		}
		walks := map[Reg]Reg{}
		for _, c := range candidates {
			walk, ok := walks[c.base]
			if !ok {
				walk = f.AllocReg()
				bump := f.AllocReg()
				head.Params = append(head.Params, BlockParam{Dst: walk, Ty: TyPtr, Class: ClassPointer})
				incoming.Args = append(incoming.Args, c.base)
				body.Insts = append(body.Insts, &Inst{Op: OpPtrBump, Dst: bump, Src: [2]Reg{walk}, Imm: 1, Ty: TyPtr, Cls: ClassPointer, SrcLine: next.SrcLine, SrcFile: next.SrcFile})
				back.Args = append(back.Args, bump)
				walks[c.base] = walk
			}
			c.x.Op = OpMove
			c.x.Src = [2]Reg{walk}
			changed++
		}
	}
	if changed != 0 {
		PropagateCopies(f)
		DeadStoreElim(f)
		DeadBlockArgElim(f)
	}
	return changed
}

// A rewrite must not silently discard hard constraints or unrecognized opcode
// metadata. Source locations are preserved by mutating only Op and Src.
func plainCountedPointerInst(x *Inst) bool {
	y := *x
	y.Op, y.Dst, y.Src, y.Ty, y.Cls = 0, 0, [2]Reg{}, nil, 0
	y.SrcLine, y.SrcFile = 0, ""
	return reflect.DeepEqual(y, Inst{})
}
