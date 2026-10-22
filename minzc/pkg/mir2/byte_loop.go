package mir2

// ByteLoopExtra retains the scalar byte operations for VM authority checks and
// source identity. Registers in these retired instructions are historical IDs,
// not executable SSA uses. Calls carry all register inputs explicitly; named
// global inputs describe invariant ordinary byte reloads.
type ByteLoopExtra struct {
	Load, Store, CountLoad, ValueLoad *Inst
	CountSymbol, ValueSymbol          string
	CountTy                           Ty
}

// Semantic contracts (all address increments are modulo 65536):
// scan_zero16(p) reads until the first zero, returning the matching pointer or
// its distance from p. A complete address-space miss continues, never returns.
// fill_bytes16(dst,n,v) makes exactly n byte stores, including none for n=0.
// copy_forward16(dst,n,src) reads then writes each byte in ascending order;
// overlapping ranges intentionally propagate, unlike memcpy/memmove hosts.
const scanZeroIntrinsic = "@mir.mem.scan_zero16"
const fillBytesIntrinsic = "@mir.mem.fill_bytes16"
const copyBytesIntrinsic = "@mir.mem.copy_forward16"

// RecognizeByteLoops replaces only proven single-body natural loops. Each
// intrinsic specifies sequential byte semantics; target lowering owns register
// adapters and saves, rather than imposing hidden physical operands on SSA.
func RecognizeByteLoops(f *Func) int {
	if f == nil || f.Attrs.FallibleCY || funcHasAsm(f) || verifyFunc(f) != nil {
		return 0
	}
	changed := 0
	for _, l := range AnalyzeNaturalLoops(f) {
		if l.Opaque {
			continue
		}
		c := newByteLoopContext(f, l)
		if c.countdownCopy() {
			changed++
			continue
		}
		if l.Opaque || len(l.Blocks) != 2 || l.Back == nil || l.Back.Target != l.Header.Label || len(l.Exits) != 1 || l.Exits[0] != l.Header {
			continue
		}
		if c.scanZero() || c.fillBytes() || c.copyBytes() {
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
		if base == NoReg || c.types[base] != TyPtr {
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
		c.replaceLoop(insts, exit, mapping)
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
	if source == nil || source.Ty != TyU8 || dest == nil || source == dest || dest.IsConst || boundTy == nil || boundTy != TyU8 || ByteWidth(dest.Ty) < 255 {
		return nil, ""
	}
	return x, source.Name
}
func (c *byteLoopContext) fillBytes() bool { return c.countedBytes(false) }
func (c *byteLoopContext) copyBytes() bool { return c.countedBytes(true) }
func (c *byteLoopContext) initialAddress(r Reg, iv LoopInduction) Reg {
	base := c.address(r, iv)
	if base == NoReg {
		for _, p := range c.l.Inductions {
			if p.Param.Ty == TyPtr && p.Step == 1 {
				base = c.address(r, p)
				if base != NoReg {
					break
				}
			}
		}
	}
	return base
}
func (c *byteLoopContext) inBlock(x *Inst, b *Block) bool {
	for _, i := range b.Insts {
		if i == x {
			return true
		}
	}
	return false
}
func (c *byteLoopContext) countedBytes(copying bool) bool {
	l := c.l
	if len(l.Stores) != 1 || !c.plainBody() {
		return false
	}
	store := l.Stores[0]
	if store.Ty != TyU8 || !c.inBlock(store, l.Latch) {
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
		base := c.initialAddress(store.Src[0], iv)
		if base == NoReg || c.types[base] != TyPtr {
			continue
		}
		extra := &ByteLoopExtra{Store: store, CountTy: countTy}

		value := c.root(store.Src[1])
		sym := fillBytesIntrinsic
		if copying {
			if len(l.Loads) != 1 || l.Loads[0].Ty != TyU8 || store.Src[1] != l.Loads[0].Dst || !c.inBlock(l.Loads[0], l.Latch) {
				continue
			}
			extra.Load = l.Loads[0]
			value = c.initialAddress(extra.Load.Src[0], iv)
			if value == NoReg || c.types[value] != TyPtr {
				continue
			}
			sym = copyBytesIntrinsic
		} else if _, ok := l.Invariant[value]; !ok && !c.canMaterialize(value) {
			x, symbol := c.disjointLoad(value, base, countTy)
			if x == nil || !c.inBlock(x, l.Latch) {
				continue
			}
			extra.ValueLoad, extra.ValueSymbol = x, symbol
			value = NoReg
		}
		if _, ok := l.Invariant[count]; !ok && !c.canMaterialize(count) {
			x, sym := c.disjointLoad(count, base, countTy)
			if x == nil || !c.inBlock(x, l.Header) || l.LiveOut[iv.Param.Dst] {
				continue
			}
			extra.CountLoad, extra.CountSymbol = x, sym
			count = NoReg
		}
		if !copying && value != NoReg && c.types[value] != TyU8 {
			continue
		}
		// Every load must be the admitted invariant value or bound reload.
		legal := true
		for _, x := range l.Loads {
			if x != extra.ValueLoad && x != extra.CountLoad && x != extra.Load {
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
		call := &Inst{Op: OpCall, Sym: sym, Args: []Reg{base, count, value}, Ty: TyVoid, ByteLoop: extra, SrcLine: store.SrcLine, SrcFile: store.SrcFile}
		insts = append(insts, call)
		mapping := map[Reg]Reg{iv.Param.Dst: count}
		if l.LiveOut[iv.Param.Dst] && countTy != iv.Param.Ty {
			out := c.f.AllocReg()
			insts = append(insts, &Inst{Op: OpExt, Dst: out, Src: [2]Reg{count}, SrcTy: countTy, Ty: iv.Param.Ty, Cls: ClassPair})
			mapping[iv.Param.Dst] = out
		}
		c.replaceLoop(insts, exit, mapping)
		return true
	}
	return false
}

// Validate semantic shapes before either VM or native expansion can see them.
func validByteLoop(f *Func, x *Inst, info map[Reg]RegInfo) bool {
	if x.ByteLoop == nil {
		return x.Sym != scanZeroIntrinsic && x.Sym != fillBytesIntrinsic && x.Sym != copyBytesIntrinsic
	}
	if x.Op != OpCall || len(x.ExtraRets) != 0 {
		return false
	}
	e := x.ByteLoop
	if e.CountSymbol != "" || e.ValueSymbol != "" {
		// A symbolic reload is part of this operation's scalar semantics. Recheck
		// the disjoint-object proof so a manually constructed intrinsic cannot
		// claim an invariant load through an arbitrary/aliasing destination.
		if x.Sym != fillBytesIntrinsic || e.CountTy != TyU8 || len(x.Args) != 3 {
			return false
		}
		definitions := map[Reg]*Inst{}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				definitions[i.Dst] = i
			}
		}
		address := definitions[x.Args[0]]
		if address == nil || address.Op != OpAddrOf || f.mod == nil {
			return false
		}
		destinationValid := false
		for _, g := range f.mod.Globals {
			if g.Name == address.Sym {
				destinationValid = g.At == nil && !g.Volatile && !g.IsConst && g.Storage == StorageNormal && ByteWidth(g.Ty) >= 255
			}
		}
		if !destinationValid || e.CountSymbol == address.Sym || e.ValueSymbol == address.Sym {
			return false
		}
		validSymbol := func(name string, origin *Inst) bool {
			if name == "" {
				return origin == nil
			}
			if origin == nil || origin.Op != OpLoad || origin.Ty != TyU8 || origin.Volatile || f.mod == nil {
				return false
			}
			for _, g := range f.mod.Globals {
				if g.Name == name {
					return g.Ty == TyU8 && !g.Volatile && g.At == nil && g.Storage == StorageNormal
				}
			}
			return false
		}
		if !validSymbol(e.CountSymbol, e.CountLoad) || !validSymbol(e.ValueSymbol, e.ValueLoad) {
			return false
		}
	}
	switch x.Sym {
	case scanZeroIntrinsic:
		if e.CountLoad != nil || e.ValueLoad != nil || e.CountTy != nil {
			return false
		}
		return len(x.Args) == 1 && info[x.Args[0]].Ty == TyPtr && x.Dst != NoReg && (x.Ty == TyU16 || x.Ty == TyPtr) && e.Store == nil && e.Load != nil && e.Load.Op == OpLoad && e.Load.Ty == TyU8 && !e.Load.Volatile
	case fillBytesIntrinsic, copyBytesIntrinsic:
		if x.Sym == copyBytesIntrinsic && (e.CountSymbol != "" || e.ValueSymbol != "" || e.CountLoad != nil || e.ValueLoad != nil || e.Load == nil || e.Load.Volatile) {
			return false
		}
		if x.Sym == fillBytesIntrinsic && e.Load != nil {
			return false
		}
		if e.CountSymbol == "" && e.CountLoad != nil || e.ValueSymbol == "" && e.ValueLoad != nil {
			return false
		}
		return len(x.Args) == 3 && info[x.Args[0]].Ty == TyPtr && (e.CountTy == TyU8 || e.CountTy == TyU16) && ((e.CountSymbol != "" && x.Args[1] == NoReg && e.CountLoad != nil) || (e.CountSymbol == "" && info[x.Args[1]].Ty == e.CountTy)) && ((e.ValueSymbol != "" && x.Args[2] == NoReg && e.ValueLoad != nil) || (e.ValueSymbol == "" && ((x.Sym == fillBytesIntrinsic && info[x.Args[2]].Ty == TyU8) || (x.Sym == copyBytesIntrinsic && info[x.Args[2]].Ty == TyPtr && e.Load != nil && e.Load.Ty == TyU8 && e.Load.Op == OpLoad)))) && x.Dst == NoReg && x.Ty == TyVoid && e.Store != nil && e.Store.Op == OpStore && e.Store.Ty == TyU8 && !e.Store.Volatile
	}
	return false
}

// countdownCopy recognizes the guarded byte ForEach shape using the same
// recurrence and memory facts as counted loops. DJNZ's implicit argument is
// interpreted according to TermDJNZ.ForEachEdge, never as an ordinary phi.
func (c *byteLoopContext) countdownCopy() bool {
	l := c.l
	t, ok := l.Header.Term.(*TermDJNZ)
	if !ok || l.Header != l.Latch || len(l.Blocks) != 1 || t.Body != l.Header.Label || len(l.Header.Params) == 0 || t.Counter != l.Header.Params[0].Dst || l.Header.Params[0].Ty != TyU8 || len(l.Loads) != 1 || len(l.Stores) != 1 || !c.plainBody() {
		return false
	}
	pre, ok := l.Preheader.Term.(*TermBrIf)
	if !ok || pre.Then != l.Header.Label || pre.Else == l.Header.Label {
		return false
	}
	count := l.Incoming[0]
	cmp := c.defs[pre.Cond]
	if cmp == nil || cmp.Op != OpCmp || cmp.Cond != CmpNe || cmp.Src[0] != count || !c.constant(cmp.Src[1], 0) || c.types[count] != TyU8 {
		return false
	}
	load, store := l.Loads[0], l.Stores[0]
	if load.Ty != TyU8 || store.Ty != TyU8 || store.Src[1] != load.Dst {
		return false
	}
	for _, iv := range l.Inductions {
		if iv.Step != 1 || (iv.Param.Ty != TyU8 && iv.Param.Ty != TyU16) || !c.constant(iv.Initial, 0) {
			continue
		}
		dst, src := c.initialAddress(store.Src[0], iv), c.initialAddress(load.Src[0], iv)
		if dst == NoReg || src == NoReg || c.types[dst] != TyPtr || c.types[src] != TyPtr {
			continue
		}
		mapping := map[Reg]Reg{iv.Next: count}
		for r, v := range l.Invariant {
			if r != v {
				mapping[r] = v
			}
		}
		legal := true
		for r := range l.LiveOut {
			if _, ok := mapping[r]; !ok {
				legal = false
			}
		}
		var insts []*Inst
		seen := map[Reg]bool{}
		for _, r := range []Reg{dst, src, count} {
			if !c.materialize(r, &insts, seen) {
				legal = false
			}
		}
		if !legal {
			continue
		}
		insts = append(insts, &Inst{Op: OpCall, Sym: copyBytesIntrinsic, Args: []Reg{dst, count, src}, Ty: TyVoid, ByteLoop: &ByteLoopExtra{Load: load, Store: store, CountTy: TyU8}, SrcLine: store.SrcLine, SrcFile: store.SrcFile})
		if iv.Param.Ty == TyU16 && l.LiveOut[iv.Next] {
			out := c.f.AllocReg()
			insts = append(insts, &Inst{Op: OpExt, Dst: out, Src: [2]Reg{count}, SrcTy: TyU8, Ty: TyU16, Cls: ClassPair})
			mapping[iv.Next] = out
		}
		exit := &TermJmp{Target: t.Exit, Args: append([]Reg(nil), t.ExitArgs...)}
		c.replaceLoop(insts, exit, mapping)
		return true
	}
	return false
}

func (c *byteLoopContext) replaceLoop(insts []*Inst, exit Term, mapping map[Reg]Reg) {
	for r, v := range c.l.Invariant {
		if r != v {
			mapping[r] = v
		}
	}
	remapTermRegs(exit, mapping)
	for _, b := range c.f.Blocks {
		if !c.l.Blocks[b.Label] {
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
	c.l.Header.Insts = insts
	c.l.Header.Term = exit
}

func (c *byteLoopContext) canMaterialize(r Reg) bool {
	var instructions []*Inst
	return c.materialize(r, &instructions, map[Reg]bool{})
}

// IsByteLoopIntrinsic identifies a validated semantic memory operation, not a
// host call. Consumers still authorize each byte through the ordinary memory
// hooks; the intrinsic name alone grants no host or memory authority.
func IsByteLoopIntrinsic(f *Func, x *Inst) bool {
	return x != nil && x.ByteLoop != nil && validByteLoop(f, x, collectRegInfo(f))
}
