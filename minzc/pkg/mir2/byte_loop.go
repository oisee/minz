package mir2

// ByteLoopExtra retains the scalar byte operations for VM authority checks and
// source identity. Registers in these retired instructions are historical IDs,
// not executable SSA uses. Calls carry all executable inputs explicitly.
type ByteLoopExtra struct{ Load, Store *Inst }

const scanZeroIntrinsic = "@mir.mem.scan_zero16"

// RecognizeByteLoops replaces only proven single-body natural loops. Each
// intrinsic specifies sequential byte semantics; target lowering owns register
// adapters and saves, rather than imposing hidden physical operands on SSA.
func RecognizeByteLoops(f *Func) int {
	if f == nil || f.Attrs.FallibleCY || f.Attrs.HasAsm || verifyFunc(f) != nil {
		return 0
	}
	changed := 0
	for _, l := range AnalyzeNaturalLoops(f) {
		if l.Opaque || len(l.Blocks) != 2 || l.Back == nil || l.Back.Target != l.Header.Label || len(l.Exits) != 1 || l.Exits[0] != l.Header {
			continue
		}
		c := newByteLoopContext(f, l)
		if c.scanZero() {
			changed++
		}
	}
	if changed > 0 {
		EliminateDeadBlocks(f)
		PropagateCopies(f)
		DeadStoreElim(f)
		DeadBlockArgElim(f)
	}
	return changed
}

type byteLoopContext struct {
	f     *Func
	l     *NaturalLoop
	defs  map[Reg]*Inst
	types map[Reg]Ty
}

func newByteLoopContext(f *Func, l *NaturalLoop) *byteLoopContext {
	c := &byteLoopContext{f: f, l: l, defs: map[Reg]*Inst{}, types: map[Reg]Ty{}}
	for _, p := range f.Contract.Params {
		c.types[p.Reg] = p.Ty
	}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			c.types[p.Dst] = p.Ty
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				c.defs[x.Dst] = x
				c.types[x.Dst] = x.Ty
			}
		}
	}
	return c
}
func (c *byteLoopContext) constant(r Reg, n int64) bool {
	x := c.defs[r]
	return x != nil && x.Op == OpConst && !x.ClsHard && x.Imm == n
}
func (c *byteLoopContext) root(r Reg) Reg {
	if v, ok := c.l.Invariant[r]; ok {
		return v
	}
	return r
}

// address recognizes base + a zero-based unit induction, or a pointer induction.
func (c *byteLoopContext) address(r Reg, iv LoopInduction) Reg {
	if r == iv.Param.Dst && iv.Param.Ty == TyPtr {
		return iv.Initial
	}
	x := c.defs[r]
	if x == nil {
		return NoReg
	}
	if x.Op == OpMove {
		return c.address(x.Src[0], iv)
	}
	if x.Op != OpPtrAdd || x.Ty != TyPtr || !plainCountedPointerInst(x) {
		return NoReg
	}
	idx := x.Src[1]
	if e := c.defs[idx]; e != nil && e.Op == OpExt && e.SrcTy == TyU8 && e.Ty == TyU16 {
		idx = e.Src[0]
	}
	if idx != iv.Param.Dst || !c.constant(iv.Initial, 0) {
		return NoReg
	}
	return c.root(x.Src[0])
}

// materialize copies a total invariant expression DAG into the replacement
// header. Ordinary loads are deliberately excluded without an alias proof.
func (c *byteLoopContext) materialize(r Reg, out *[]*Inst, seen map[Reg]bool) bool {
	if seen[r] {
		return true
	}
	seen[r] = true
	if _, ok := c.l.Invariant[r]; ok {
		return true
	}
	x := c.defs[r]
	if x == nil || x.ClsHard {
		return false
	}
	switch x.Op {
	case OpAddrOf:
		if c.f.mod == nil || !invariantGlobalSymbol(c.f.mod, x.Sym) {
			return false
		}
	case OpConst:
	case OpMove, OpExt, OpPtrAdd, OpField:
		for _, s := range x.Uses() {
			if !c.materialize(s, out, seen) {
				return false
			}
		}
	default:
		return false
	}
	*out = append(*out, x)
	return true
}
func (c *byteLoopContext) continuation(cond Reg) (Term, bool) {
	head, body := c.l.Header, c.l.Latch
	switch t := head.Term.(type) {
	case *TermBrIf:
		if t.Cond != cond || t.Then != body.Label || len(t.ThenArgs) != 0 || c.l.Blocks[t.Else] {
			return nil, false
		}
		return &TermJmp{Target: t.Else, Args: append([]Reg(nil), t.ElseArgs...)}, true
	case *TermCondRet:
		if t.Cond != cond || t.Then != body.Label || len(t.ThenArgs) != 0 {
			return nil, false
		}
		return &TermRet{Vals: append([]Reg(nil), t.Vals...)}, true
	}
	return nil, false
}
func (c *byteLoopContext) scanZero() bool {
	l := c.l
	if len(l.Loads) != 1 || len(l.Stores) != 0 {
		return false
	}
	load := l.Loads[0]
	if load.Ty != TyU8 || load.Volatile || load.ClsHard {
		return false
	}
	var cmp *Inst
	for _, x := range l.Header.Insts {
		if x.Op == OpCmp {
			if cmp != nil {
				return false
			}
			cmp = x
		}
	}
	if cmp == nil || cmp.Cond != CmpNe || cmp.Src[0] != load.Dst || !c.constant(cmp.Src[1], 0) {
		return false
	}
	exit, ok := c.continuation(cmp.Dst)
	if !ok {
		return false
	}
	for _, iv := range l.Inductions {
		if iv.Step != 1 || (iv.Param.Ty != TyU16 && iv.Param.Ty != TyPtr) {
			continue
		}
		base := c.address(load.Src[0], iv)
		if base == NoReg {
			continue
		}
		legal := true
		for r := range l.LiveOut {
			if r != iv.Param.Dst {
				if _, ok := l.Invariant[r]; !ok {
					legal = false
				}
			}
		}
		for _, b := range c.f.Blocks {
			if l.Blocks[b.Label] {
				for _, x := range b.Insts {
					switch x.Op {
					case OpConst, OpMove, OpAdd, OpPtrAdd, OpPtrBump, OpField, OpExt, OpCmp, OpLoad, OpAddrOf:
					default:
						legal = false
					}
				}
			}
		}
		var insts []*Inst
		if !legal || !c.materialize(base, &insts, map[Reg]bool{}) {
			continue
		}
		result := c.f.AllocReg()
		call := &Inst{Op: OpCall, Sym: scanZeroIntrinsic, Dst: result, Args: []Reg{base}, Ty: iv.Param.Ty, Cls: ClassPointer, ByteLoop: &ByteLoopExtra{Load: load}, SrcLine: load.SrcLine, SrcFile: load.SrcFile}
		insts = append(insts, call)
		mapping := map[Reg]Reg{iv.Param.Dst: result}
		for r, v := range l.Invariant {
			if r != v {
				mapping[r] = v
			}
		}
		remapTermRegs(exit, mapping)
		// Escaping values are rewritten only outside the replaced loop.
		for _, b := range c.f.Blocks {
			if !l.Blocks[b.Label] {
				for _, x := range b.Insts {
					x.Src[0] = remapReg(x.Src[0], mapping)
					x.Src[1] = remapReg(x.Src[1], mapping)
					for i, r := range x.Args {
						x.Args[i] = remapReg(r, mapping)
					}
				}
				remapTermRegs(b.Term, mapping)
			}
		}
		l.Header.Insts = insts
		l.Header.Term = exit
		return true
	}
	return false
}

// Validate semantic shapes before either VM or native expansion can see them.
func validByteLoop(x *Inst, info map[Reg]RegInfo) bool {
	if x.ByteLoop == nil {
		return x.Sym != scanZeroIntrinsic
	}
	return x.Op == OpCall && x.Sym == scanZeroIntrinsic && len(x.Args) == 1 && info[x.Args[0]].Ty == TyPtr && x.Dst != NoReg && (x.Ty == TyU16 || x.Ty == TyPtr) && len(x.ExtraRets) == 0 && x.ByteLoop.Store == nil && x.ByteLoop.Load != nil && x.ByteLoop.Load.Op == OpLoad && x.ByteLoop.Load.Ty == TyU8 && !x.ByteLoop.Load.Volatile
}
