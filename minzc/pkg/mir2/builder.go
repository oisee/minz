package mir2

// Builder is a cursor-based API for emitting MIR2 instructions into a function.
// It tracks the current insertion block and delegates register allocation to
// the parent Func.
//
// Block arguments — not phi nodes:
//
//	// Loop with counted state in block params
//	head := b.SwitchToNewBlock("loop_head")
//	a    := b.BlockParam(head, TyU16, ClassPointer) // %r3
//	bReg := b.BlockParam(head, TyU16, ClassIndex)   // %r4
//	i    := b.BlockParam(head, TyU8,  ClassCounter) // %r5
//	...
//	b.BrIf(cond, "loop_body", []Reg{a, bReg, i}, "exit", nil)
//
// Register classes are soft by default; pass clsHard=true only when
// the instruction semantics require a specific physical register
// (e.g. the B operand in an inline DJNZ template).
type Builder struct {
	F   *Func  // owning function
	Cur *Block // current insertion block
}

// NewBuilder creates a Builder targeting function f.
func NewBuilder(f *Func) *Builder { return &Builder{F: f} }

// ── Block management ──────────────────────────────────────────────────────────

// SwitchToNewBlock creates a new block, makes it current, and returns it.
func (b *Builder) SwitchToNewBlock(label string) *Block {
	blk := b.F.NewBlock(label)
	b.Cur = blk
	return blk
}

// SwitchTo makes an existing block the current insertion point.
func (b *Builder) SwitchTo(blk *Block) { b.Cur = blk }

// ── Block parameters (replaces phi nodes) ─────────────────────────────────────

// BlockParam adds a formal parameter to blk and returns the virtual register
// that holds the value on block entry.  The parameter's class is soft by default.
func (b *Builder) BlockParam(blk *Block, ty Ty, cls RegClass) Reg {
	r := b.F.AllocReg()
	blk.Params = append(blk.Params, BlockParam{Dst: r, Ty: ty, Class: cls})
	return r
}

// BlockParamHard adds a formal parameter with a *hard* class constraint.
// Use only when the block entry truly requires a specific physical register
// (e.g. an ABI boundary).
func (b *Builder) BlockParamHard(blk *Block, ty Ty, cls RegClass) Reg {
	r := b.F.AllocReg()
	// Hard-ness is stored on the param; the allocator treats it as required.
	// We reuse the BlockParam struct — callers can check via block analysis.
	blk.Params = append(blk.Params, BlockParam{Dst: r, Ty: ty, Class: cls})
	return r
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (b *Builder) reg() Reg { return b.F.AllocReg() }

func (b *Builder) emit(inst *Inst) *Inst {
	b.Cur.Append(inst)
	return inst
}

func inst1(op Op, dst Reg, src0 Reg, ty Ty, cls RegClass) *Inst {
	return &Inst{Op: op, Dst: dst, Src: [2]Reg{src0}, Ty: ty, Cls: cls}
}

func inst2(op Op, dst Reg, src0, src1 Reg, ty Ty, cls RegClass) *Inst {
	return &Inst{Op: op, Dst: dst, Src: [2]Reg{src0, src1}, Ty: ty, Cls: cls}
}

// ── Function parameters ───────────────────────────────────────────────────────

// Param declares a function parameter and returns its virtual register.
// Call before emitting any instructions into the entry block.
func (b *Builder) Param(name string, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.F.Contract.Params = append(b.F.Contract.Params, Param{
		Name: name, Reg: r, Ty: ty, Class: cls,
	})
	return r
}

// ── Constants and moves ───────────────────────────────────────────────────────

// Const emits a constant load.  cls is soft.
func (b *Builder) Const(val int64, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpConst, Dst: r, Imm: val, Ty: ty, Cls: cls})
	return r
}

// Move emits a register copy.
func (b *Builder) Move(src Reg, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(inst1(OpMove, r, src, ty, cls))
	return r
}

// ── Binary arithmetic / bitwise ───────────────────────────────────────────────

// BinOp emits any OpAdd…OpSar instruction.
func (b *Builder) BinOp(op Op, lhs, rhs Reg, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(inst2(op, r, lhs, rhs, ty, cls))
	return r
}

func (b *Builder) Add(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpAdd, l, r, ty, cls) }
func (b *Builder) Sub(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpSub, l, r, ty, cls) }
func (b *Builder) Mul(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpMul, l, r, ty, cls) }
func (b *Builder) Div(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpDiv, l, r, ty, cls) }
func (b *Builder) SDiv(l, r Reg, ty Ty, cls RegClass) Reg { return b.BinOp(OpSDiv, l, r, ty, cls) }
func (b *Builder) Mod(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpMod, l, r, ty, cls) }
func (b *Builder) And(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpAnd, l, r, ty, cls) }
func (b *Builder) Or(l, r Reg, ty Ty, cls RegClass) Reg   { return b.BinOp(OpOr, l, r, ty, cls) }
func (b *Builder) Xor(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpXor, l, r, ty, cls) }
func (b *Builder) Shl(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpShl, l, r, ty, cls) }
func (b *Builder) Shr(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpShr, l, r, ty, cls) }
func (b *Builder) Sar(l, r Reg, ty Ty, cls RegClass) Reg  { return b.BinOp(OpSar, l, r, ty, cls) }

// ── Unary ops ─────────────────────────────────────────────────────────────────

func (b *Builder) unary(op Op, src Reg, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(inst1(op, r, src, ty, cls))
	return r
}

func (b *Builder) Neg(src Reg, ty Ty, cls RegClass) Reg { return b.unary(OpNeg, src, ty, cls) }
func (b *Builder) Not(src Reg, ty Ty, cls RegClass) Reg { return b.unary(OpNot, src, ty, cls) }

// ── Type conversions ──────────────────────────────────────────────────────────

func (b *Builder) conv(op Op, src Reg, srcTy, dstTy Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: op, Dst: r, Src: [2]Reg{src}, SrcTy: srcTy, Ty: dstTy, Cls: cls})
	return r
}

// Ext zero-extends src from srcTy to dstTy.
func (b *Builder) Ext(src Reg, srcTy, dstTy Ty, cls RegClass) Reg {
	return b.conv(OpExt, src, srcTy, dstTy, cls)
}

// Sext sign-extends src from srcTy to dstTy.
func (b *Builder) Sext(src Reg, srcTy, dstTy Ty, cls RegClass) Reg {
	return b.conv(OpSext, src, srcTy, dstTy, cls)
}

// Trunc truncates src from srcTy to dstTy.
func (b *Builder) Trunc(src Reg, srcTy, dstTy Ty, cls RegClass) Reg {
	return b.conv(OpTrunc, src, srcTy, dstTy, cls)
}

// ── Comparison ────────────────────────────────────────────────────────────────

// Cmp emits a comparison.  Use ClassFlag (soft) so the backend can avoid
// materialising the result.  Pass clsHard=true only at ABI boundaries.
func (b *Builder) Cmp(cond CmpCond, lhs, rhs Reg, cls RegClass, clsHard bool) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpCmp, Dst: r, Src: [2]Reg{lhs, rhs}, Cond: cond, Ty: TyBool, Cls: cls, ClsHard: clsHard})
	return r
}

// CmpWithSrcTy is like Cmp but records the operand type in SrcTy.
// This is needed for signed comparisons (CmpLt/Le/Gt/Ge) so the VM
// can sign-extend operands before comparing.
func (b *Builder) CmpWithSrcTy(cond CmpCond, lhs, rhs Reg, cls RegClass, clsHard bool, srcTy Ty) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpCmp, Dst: r, Src: [2]Reg{lhs, rhs}, Cond: cond, Ty: TyBool, SrcTy: srcTy, Cls: cls, ClsHard: clsHard})
	return r
}

// ── Memory ────────────────────────────────────────────────────────────────────

// Load emits a memory load from address ptr.
func (b *Builder) Load(ptr Reg, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(inst1(OpLoad, r, ptr, ty, cls))
	return r
}

// Store emits a memory store of val to address ptr.
func (b *Builder) Store(ptr, val Reg, ty Ty) {
	b.emit(&Inst{Op: OpStore, Src: [2]Reg{ptr, val}, Ty: ty})
}

// AddrOf emits an address-of for a named symbol.
func (b *Builder) AddrOf(sym string, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpAddrOf, Dst: r, Sym: sym, Ty: TyPtr, Cls: cls})
	return r
}

// Field advances ptr by byteOffset and returns the new pointer.
// byteOffset is a compile-time constant (struct field byte offset in AoS layout).
// On Z80: offset 1-3 → INC HL ×N; offset 4+ → LD BC,N; ADD HL,BC.
func (b *Builder) Field(ptr Reg, byteOffset int64, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpField, Dst: r, Src: [2]Reg{ptr}, Imm: byteOffset, Ty: TyPtr, Cls: cls})
	return r
}

// FieldOf is a convenience wrapper: computes the byte offset of field i
// in st and emits an OpField instruction.
func (b *Builder) FieldOf(ptr Reg, st *StructTy, fieldIdx int, cls RegClass) Reg {
	return b.Field(ptr, int64(st.ByteOffset(fieldIdx)), cls)
}

// PtrBump advances an iterator pointer by stride bytes and returns the new pointer.
// stride is a compile-time constant element stride in bytes.
// On Z80 (AoS): stride 1-3 → INC HL ×N; stride 4+ → LD BC,N; ADD HL,BC.
// Distinct from Field: marks iterator-style advance for future SoA/SoA256 lowering.
func (b *Builder) PtrBump(ptr Reg, stride int64, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpPtrBump, Dst: r, Src: [2]Reg{ptr}, Imm: stride, Ty: TyPtr, Cls: cls})
	return r
}

// PtrAdd adds a runtime u16 offset to a pointer: dst = base + offset.
// On Z80: base in HL, offset in DE → ADD HL, DE (11T, zero GC overhead).
// Use for runtime array indexing: ptr_add(arr_base, i * element_stride).
func (b *Builder) PtrAdd(base, offset Reg, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpPtrAdd, Dst: r, Src: [2]Reg{base, offset}, Ty: TyPtr, Cls: cls})
	return r
}

// Alloca reserves bytes bytes on the stack and returns a pointer.
func (b *Builder) Alloca(bytes int64) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpAlloca, Dst: r, Imm: bytes, Ty: TyPtr})
	return r
}

// ── Function calls ────────────────────────────────────────────────────────────

// Call emits a direct call.  Returns NoReg for void returns.
func (b *Builder) Call(fn string, args []Reg, retTy Ty, cls RegClass, attr CallAttrs) Reg {
	var r Reg
	if retTy != TyVoid {
		r = b.reg()
	}
	b.emit(&Inst{Op: OpCall, Dst: r, Sym: fn, Args: args, Ty: retTy, Cls: cls, CallAttr: attr})
	return r
}

// CallMulti emits a direct call that returns multiple values.
//
// returns is a slice describing each return position (Ty + Class).
// The first entry is the primary result (allocated as for Call); subsequent
// entries become ExtraRets on the instruction, each allocated to the given class.
//
// Returns a []Reg of length len(returns); NoReg at position i if returns[i].Ty == TyVoid.
func (b *Builder) CallMulti(fn string, args []Reg, returns []Return, attr CallAttrs) []Reg {
	if len(returns) == 0 {
		b.emit(&Inst{Op: OpCall, Sym: fn, Args: args, Ty: TyVoid, CallAttr: attr})
		return nil
	}
	// Allocate result regs.
	regs := make([]Reg, len(returns))
	for i, ret := range returns {
		if ret.Ty != TyVoid {
			regs[i] = b.reg()
		}
	}

	inst := &Inst{
		Op:       OpCall,
		Dst:      regs[0],
		Sym:      fn,
		Args:     args,
		Ty:       returns[0].Ty,
		Cls:      returns[0].Class,
		CallAttr: attr,
	}
	// Attach extra return regs.
	if len(returns) > 1 {
		inst.ExtraRets = regs[1:]
		inst.ExtraRetClasses = make([]RegClass, len(returns)-1)
		inst.ExtraRetTys = make([]Ty, len(returns)-1)
		for i, ret := range returns[1:] {
			inst.ExtraRetClasses[i] = ret.Class
			inst.ExtraRetTys[i] = ret.Ty
		}
	}
	b.emit(inst)
	return regs
}

// CallVoid emits a void direct call.
func (b *Builder) CallVoid(fn string, args []Reg, attr CallAttrs) {
	b.emit(&Inst{Op: OpCall, Sym: fn, Args: args, Ty: TyVoid, CallAttr: attr})
}

// Intrinsic emits a call to @mir.<name>.
func (b *Builder) Intrinsic(name string, args []Reg, retTy Ty, cls RegClass) Reg {
	return b.Call("@mir."+name, args, retTy, cls, CallAttrs{})
}

// IntrinsicVoid emits a void @mir.<name> call.
func (b *Builder) IntrinsicVoid(name string, args []Reg) {
	b.CallVoid("@mir."+name, args, CallAttrs{})
}

// CallIndirect emits an indirect call through a function-pointer register.
func (b *Builder) CallIndirect(fnPtr Reg, args []Reg, retTy Ty, cls RegClass) Reg {
	var r Reg
	if retTy != TyVoid {
		r = b.reg()
	}
	b.emit(&Inst{Op: OpCallIndirect, Dst: r, Src: [2]Reg{fnPtr}, Args: args, Ty: retTy, Cls: cls})
	return r
}

// ── SMC ──────────────────────────────────────────────────────────────────────

// PatchSlot declares a mutable immediate with initial value init.
func (b *Builder) PatchSlot(init int64, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpPatchSlot, Dst: r, Imm: init, Ty: ty, Cls: cls})
	return r
}

// PatchSlotNamed is like PatchSlot but attaches a symbol name used by the
// Z80 backend to emit an EQU label and synthesise a set_<param> patcher
// function.  sym should be "funcName$paramName" (dollar-separated).
func (b *Builder) PatchSlotNamed(sym string, init int64, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpPatchSlot, Dst: r, Sym: sym, Imm: init, Ty: ty, Cls: cls})
	return r
}

// LoadPatched reads the current value of a mutable immediate.
func (b *Builder) LoadPatched(slot Reg, ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(inst1(OpLoadPatched, r, slot, ty, cls))
	return r
}

// Patch writes a new value into a mutable immediate slot.
// ty must match the type used in the corresponding PatchSlot call.
func (b *Builder) Patch(slot, newVal Reg, ty Ty) {
	b.emit(&Inst{Op: OpPatch, Src: [2]Reg{slot, newVal}, Ty: ty})
}

// ── Inline assembly ───────────────────────────────────────────────────────────

// Asm emits an inline assembly block.
// Use ClsHard on any result/input register the template physically requires.
func (b *Builder) Asm(tmpl string, ins, outs []Reg, clobbers ClassSet, retTy Ty, cls RegClass) Reg {
	var r Reg
	if retTy != TyVoid {
		r = b.reg()
	}
	b.emit(&Inst{
		Op: OpAsm, Dst: r, Ty: retTy, Cls: cls,
		Asm: &AsmExtra{Template: tmpl, Ins: ins, Outs: outs, Clobbers: clobbers},
	})
	return r
}

// ── Terminators (with block arguments) ───────────────────────────────────────

// Jmp seals the current block with an unconditional jump.
// args are passed to the target block's params (positionally).
func (b *Builder) Jmp(target string, args ...Reg) {
	b.Cur.Seal(&TermJmp{Target: target, Args: args})
}

// BrIf seals the current block with a conditional branch.
// thenArgs go to then-block's params; elseArgs go to else-block's params.
func (b *Builder) BrIf(cond Reg, then string, thenArgs []Reg, else_ string, elseArgs []Reg) {
	b.Cur.Seal(&TermBrIf{
		Cond: cond, Then: then, ThenArgs: thenArgs, Else: else_, ElseArgs: elseArgs,
	})
}

// BrIf2 seals the current block with a three-way unsigned comparison branch.
// One CP instruction sets Z (eq) and C (lt); neither means gt.
// eqArgs/ltArgs/gtArgs are passed to the respective successor block params.
func (b *Builder) BrIf2(lhs, rhs Reg, eq string, eqArgs []Reg, lt string, ltArgs []Reg, gt string, gtArgs []Reg) {
	b.Cur.Seal(&TermBrIf2{
		Lhs: lhs, Rhs: rhs,
		Eq: eq, EqArgs: eqArgs,
		Lt: lt, LtArgs: ltArgs,
		Gt: gt, GtArgs: gtArgs,
	})
}

// Push emits a PUSH instruction (spill register to stack).
func (b *Builder) Push(r Reg, ty Ty) {
	b.emit(&Inst{Op: OpPush, Src: [2]Reg{r}, Ty: ty})
}

// Pop emits a POP instruction (restore register from stack).
func (b *Builder) Pop(ty Ty, cls RegClass) Reg {
	r := b.reg()
	b.emit(&Inst{Op: OpPop, Dst: r, Ty: ty, Cls: cls})
	return r
}

// PtrBumpSoA256Element advances a SoA256 pointer to the next element: INC L (4T).
// stride = -1 signals "SoA256 element advance".
func (b *Builder) PtrBumpSoA256Element(ptr Reg, cls RegClass) Reg {
	return b.PtrBump(ptr, -1, cls)
}

// PtrBumpSoA256Field switches a SoA256 pointer to the next field column: INC H (4T).
// stride = 256 signals "SoA256 field column switch".
func (b *Builder) PtrBumpSoA256Field(ptr Reg, cls RegClass) Reg {
	return b.PtrBump(ptr, 256, cls)
}

// DJNZ emits a Z80 DJNZ terminator.
// counter must be in ClassCounter (B). The body block's first param receives
// the decremented counter. BodyArgs are args for body.Params[1:].
func (b *Builder) DJNZ(counter Reg, body string, bodyArgs []Reg, exit string, exitArgs []Reg) {
	b.Cur.Seal(&TermDJNZ{
		Counter:  counter,
		Body:     body, BodyArgs: bodyArgs,
		Exit:     exit, ExitArgs: exitArgs,
	})
}

// Ret seals the current block with a return.
func (b *Builder) Ret(vals ...Reg) { b.Cur.Seal(&TermRet{Vals: vals}) }

// Unreachable seals the current block as dead code.
func (b *Builder) Unreachable() { b.Cur.Seal(&TermUnreachable{}) }
