package mir2

// localInlineCSE shares ordinary global reads and pure expressions exposed by
// inlining. All stores, calls and opaque effects invalidate the load table.
// Loads through arbitrary pointers and volatile objects remain independent.
func localInlineCSE(m *Module, f *Func) {
	if !wordMaskFlagsUnobserved(f, NoReg) {
		return
	}
	globals := map[string]bool{}
	for _, g := range m.Globals {
		globals[g.Name] = !g.Volatile && g.At == nil
	}
	for _, b := range f.Blocks {
		type key struct {
			op        Op
			ty, srcTy Ty
			src       [2]Reg
			imm       int64
			sym       string
		}
		pure := map[key]Reg{}
		loads := map[key]Reg{}
		addresses := map[Reg]string{}
		copies := map[Reg]Reg{}
		resolve := func(r Reg) Reg {
			for n := 0; n < len(copies); n++ {
				s, ok := copies[r]
				if !ok {
					break
				}
				r = s
			}
			return r
		}
		for _, x := range b.Insts {
			for i, r := range x.Src {
				x.Src[i] = resolve(r)
			}
			for i, r := range x.Args {
				x.Args[i] = resolve(r)
			}
			// Asm.Ins needs no substitution here: wordMaskFlagsUnobserved
			// above rejects any function containing OpAsm before this loop.
			if x.Op == OpMove && !x.ClsHard {
				copies[x.Dst] = x.Src[0]
				continue
			}
			if x.Op == OpAddrOf {
				addresses[x.Dst] = x.Sym
			}
			if x.SequentialBytes || x.ByteLoop != nil || x.ClsHard || x.FallibleCY {
				loads = map[key]Reg{}
				continue
			}
			if x.Op == OpLoad {
				if x.Volatile {
					loads = map[key]Reg{}
					continue
				}
				if !x.Volatile && globals[addresses[x.Src[0]]] {
					k := key{op: OpLoad, ty: x.Ty, src: x.Src}
					if old, ok := loads[k]; ok {
						x.Op = OpMove
						x.Src = [2]Reg{old}
						copies[x.Dst] = old
					} else {
						loads[k] = x.Dst
					}
				}
				continue
			}
			switch x.Op {
			case OpConst, OpAddrOf, OpAdd, OpSub, OpMul, OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar, OpNeg, OpNot, OpExt, OpSext, OpTrunc, OpPackBytes:
				k := key{x.Op, x.Ty, x.SrcTy, x.Src, x.Imm, x.Sym}
				if r, ok := pure[k]; ok {
					x.Op = OpMove
					x.Src = [2]Reg{r}
					copies[x.Dst] = r
				} else {
					pure[k] = x.Dst
				}
			case OpCmp: // value comparison has no memory effects
			default:
				loads = map[key]Reg{}
			}
		}
	}
	PropagateCopies(f)
	DeadStoreElim(f)
}
