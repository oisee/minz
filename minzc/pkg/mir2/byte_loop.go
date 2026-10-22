package mir2

// ByteLoopExtra retains the scalar byte operations for VM authority checks and
// source identity. Registers in these retired instructions are historical IDs,
// not executable SSA uses. Calls carry all executable inputs explicitly.
type ByteLoopExtra struct {
	Load, Store, CountLoad, ValueLoad *Inst
	CountSymbol, ValueSymbol          string
	CountTy                           Ty
}

const scanZeroIntrinsic = "@mir.mem.scan_zero16"
const fillBytesIntrinsic = "@mir.mem.fill_bytes16"

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
		if c.scanZero() || c.fillBytes() {
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

// plainByteLoopBody accounts for every operation that can execute. Only total
// address/counter arithmetic may disappear with a recognized memory idiom.
func (c *byteLoopContext) plainBody() bool {
	for _, b := range c.f.Blocks {
		if c.l.Blocks[b.Label] {
			for _, x := range b.Insts {
				switch x.Op {
				case OpConst, OpMove, OpAdd, OpPtrAdd, OpPtrBump, OpField, OpExt, OpCmp, OpLoad, OpStore, OpAddrOf:
				default:
					return false
				}
			}
		}
	}
	return true
}
func (c *byteLoopContext) global(r Reg) *Global {
	r = c.root(r)
	x := c.defs[r]
	if x == nil || x.Op != OpAddrOf || c.f.mod == nil {
		return nil
	}
	for i := range c.f.mod.Globals {
		g := &c.f.mod.Globals[i]
		if g.Name == x.Sym && g.At == nil && !g.Volatile && g.Storage == StorageNormal {
			return g
		}
	}
	return nil
}

// A counted destination is contained in its ordinary linker object for every
// bound value. Thus loads from distinct ordinary globals cannot alias stores.
func (c *byteLoopContext) disjointLoad(r, base Reg, boundTy Ty) (*Inst, string) {
	x := c.defs[r]
	if x == nil || x.Op != OpLoad || x.Ty != TyU8 || x.Volatile {
		return nil, ""
	}
	source, dest := c.global(x.Src[0]), c.global(base)
	if source == nil || dest == nil || source == dest || dest.IsConst || boundTy == nil || boundTy != TyU8 || ByteWidth(dest.Ty) < 255 {
		return nil, ""
	}
	return x, source.Name
}
func (c *byteLoopContext) fillBytes() bool {
	l := c.l
	if len(l.Stores) != 1 || !c.plainBody() {
		return false
	}
	store := l.Stores[0]
	if store.Ty != TyU8 {
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
	if cmp == nil || cmp.Cond != CmpUlt {
		return false
	}
	exit, ok := c.continuation(cmp.Dst)
	if !ok {
		return false
	}
	for _, iv := range l.Inductions {
		if iv.Param.Dst != cmp.Src[0] || iv.Step != 1 || !c.constant(iv.Initial, 0) || (iv.Param.Ty != TyU8 && iv.Param.Ty != TyU16) {
			continue
		}
		count := c.root(cmp.Src[1])
		countTy := c.types[count]
		if countTy != TyU8 && countTy != TyU16 || countTy.Width() > iv.Param.Ty.Width() {
			continue
		}
		base := c.address(store.Src[0], iv)
		// Pointer-form loops retain an independent pointer recurrence.
		if base == NoReg {
			for _, p := range l.Inductions {
				if p.Param.Ty == TyPtr && p.Step == 1 {
					base = c.address(store.Src[0], p)
					if base != NoReg {
						break
					}
				}
			}
		}
		if base == NoReg {
			continue
		}
		extra := &ByteLoopExtra{Store: store, CountTy: countTy}
		value := c.root(store.Src[1])
		if _, ok := l.Invariant[value]; !ok {
			x, sym := c.disjointLoad(value, base, countTy)
			if x == nil {
				continue
			}
			extra.ValueLoad, extra.ValueSymbol = x, sym
			value = NoReg
		}
		if _, ok := l.Invariant[count]; !ok {
			x, sym := c.disjointLoad(count, base, countTy)
			if x == nil || l.LiveOut[iv.Param.Dst] {
				continue
			}
			extra.CountLoad, extra.CountSymbol = x, sym
			count = NoReg
		}
		// Every load must be the admitted invariant value or bound reload.
		legal := true
		for _, x := range l.Loads {
			if x != extra.ValueLoad && x != extra.CountLoad {
				legal = false
			}
		}
		for r := range l.LiveOut {
			if r != iv.Param.Dst {
				if _, ok := l.Invariant[r]; !ok {
					legal = false
				}
			}
		}
		var insts []*Inst
		seen := map[Reg]bool{}
		for _, r := range []Reg{base, value, count} {
			if r != NoReg && !c.materialize(r, &insts, seen) {
				legal = false
			}
		}
		if !legal {
			continue
		}
		call := &Inst{Op: OpCall, Sym: fillBytesIntrinsic, Args: []Reg{base, count, value}, Ty: TyVoid, ByteLoop: extra, SrcLine: store.SrcLine, SrcFile: store.SrcFile}
		insts = append(insts, call)
		mapping := map[Reg]Reg{iv.Param.Dst: count}
		for r, v := range l.Invariant {
			if r != v {
				mapping[r] = v
			}
		}
		remapTermRegs(exit, mapping)
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
		return x.Sym != scanZeroIntrinsic && x.Sym != fillBytesIntrinsic
	}
	if x.Op != OpCall || len(x.ExtraRets) != 0 {
		return false
	}
	e := x.ByteLoop
	switch x.Sym {
	case scanZeroIntrinsic:
		return len(x.Args) == 1 && info[x.Args[0]].Ty == TyPtr && x.Dst != NoReg && (x.Ty == TyU16 || x.Ty == TyPtr) && e.Store == nil && e.Load != nil && e.Load.Op == OpLoad && e.Load.Ty == TyU8 && !e.Load.Volatile
	case fillBytesIntrinsic:
		return len(x.Args) == 3 && info[x.Args[0]].Ty == TyPtr && (e.CountTy == TyU8 || e.CountTy == TyU16) && ((e.CountSymbol != "" && x.Args[1] == NoReg && e.CountLoad != nil) || (e.CountSymbol == "" && info[x.Args[1]].Ty == e.CountTy)) && ((e.ValueSymbol != "" && x.Args[2] == NoReg && e.ValueLoad != nil) || (e.ValueSymbol == "" && info[x.Args[2]].Ty == TyU8)) && x.Dst == NoReg && x.Ty == TyVoid && e.Store != nil && e.Store.Op == OpStore && e.Store.Ty == TyU8 && !e.Store.Volatile
	}
	return false
}
