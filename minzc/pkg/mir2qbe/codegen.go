// Package mir2qbe translates MIR2 modules to QBE IL text.
//
// QBE (https://c9x.me/compile/) is a small compiler backend that accepts
// a simple SSA IR and emits x86_64 / arm64 / rv64 assembly.
//
// Primary use: correctness oracle for MIR2 semantics.
// If a program produces the same result when run via:
//   (a) MIR2 → Z80 codegen → MZE emulator, and
//   (b) MIR2 → QBE → native binary
// then the MIR2 IR and HIR→MIR2 lowering are correct.
// Any divergence points directly at the Z80 codegen.
//
// Type mapping
//   TyBool, TyU8, TyI8, TyU16, TyI16  →  w  (QBE 32-bit word)
//   TyPtr                               →  l  (QBE 64-bit long, native ptr)
//   TyVoid                              →  (no type, omitted)
//
// Block params → phi nodes
//   MIR2 uses block arguments (Cranelift-style).
//   QBE uses phi nodes (SSA-style).
//   We build a predecessor map per function and emit phi nodes at block entry.
package mir2qbe

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
)

// Compile translates a MIR2 module to QBE IL source text.
func Compile(m *mir2.Module) (string, error) {
	g := &gen{sb: &strings.Builder{}, mod: m}
	g.emitModule()
	return g.sb.String(), nil
}

// ── internal generator ────────────────────────────────────────────────────────

type gen struct {
	sb  *strings.Builder
	mod *mir2.Module

	// per-function: preds[blockLabel][paramIndex] = list of (predLabel, argReg)
	preds map[string][][]predEdge

	// per-function: QBE type ("w" or "l") for each virtual register.
	// Built by scanRegTypes before emitting a function.
	// Used to decide when to zero-extend a "w" pointer to "l" for memory ops.
	regTy map[mir2.Reg]string
}

type predEdge struct {
	from string   // predecessor block label
	arg  mir2.Reg // value passed for this param slot
}

// ── module ────────────────────────────────────────────────────────────────────

func (g *gen) emitModule() {
	// Globals (data section)
	for _, gl := range g.mod.Globals {
		g.emitGlobal(gl)
	}
	if len(g.mod.Globals) > 0 {
		g.line("")
	}
	// Functions
	for _, f := range g.mod.Funcs {
		g.emitFunc(f)
		g.line("")
	}
}

func (g *gen) emitGlobal(gl mir2.Global) {
	// data $name = { b 0, b 1, ... }
	if len(gl.Init) == 0 {
		// zeroed — determine size from type
		sz := byteWidth(gl.Ty)
		g.printf("data $%s = { z %d }\n", gl.Name, sz)
		return
	}
	parts := make([]string, len(gl.Init))
	for i, b := range gl.Init {
		parts[i] = fmt.Sprintf("b %d", b)
	}
	g.printf("data $%s = { %s }\n", gl.Name, strings.Join(parts, ", "))
}

// ── function ──────────────────────────────────────────────────────────────────

func (g *gen) emitFunc(f *mir2.Func) {
	if f.Attrs.IsExtern {
		return // declaration only — QBE doesn't need extern decls
	}

	// Build predecessor map for phi synthesis
	g.buildPreds(f)
	// Build reg→qbeType map so pointer coercions work correctly
	g.scanRegTypes(f)

	// Signature
	retTy := ""
	if len(f.Contract.Returns) > 0 && f.Contract.Returns[0].Ty != mir2.TyVoid {
		retTy = qbeTy(f.Contract.Returns[0].Ty) + " "
	}

	params := make([]string, len(f.Contract.Params))
	for i, p := range f.Contract.Params {
		params[i] = fmt.Sprintf("%s %%r%d", g.regTy[p.Reg], p.Reg)
	}
	g.printf("export function %s$%s(%s) {\n", retTy, f.Name, strings.Join(params, ", "))

	for _, b := range f.Blocks {
		g.emitBlock(f, b)
	}
	g.line("}")
}

// ── block ─────────────────────────────────────────────────────────────────────

func (g *gen) emitBlock(f *mir2.Func, b *mir2.Block) {
	g.printf("@%s\n", b.Label)

	// Emit phi nodes for block params
	paramPreds := g.preds[b.Label]
	for i, bp := range b.Params {
		if i >= len(paramPreds) || len(paramPreds[i]) == 0 {
			continue
		}
		edges := paramPreds[i]
		parts := make([]string, len(edges))
		for j, e := range edges {
			parts[j] = fmt.Sprintf("@%s %%r%d", e.from, e.arg)
		}
		g.printf("\t%%r%d =%s phi %s\n", bp.Dst, g.regTy[bp.Dst], strings.Join(parts, ", "))
	}

	// Instructions
	for _, inst := range b.Insts {
		g.emitInst(inst)
	}

	// Terminator
	g.emitTerm(b.Term)
}

// ── instructions ──────────────────────────────────────────────────────────────

func (g *gen) emitInst(inst *mir2.Inst) {
	dst := inst.Dst
	ty := qbeTy(inst.Ty)
	a, b2 := inst.Src[0], inst.Src[1]

	def := func(op string, args ...string) {
		g.printf("\t%%r%d =%s %s %s\n", dst, ty, op, strings.Join(args, ", "))
	}
	reg := func(r mir2.Reg) string { return fmt.Sprintf("%%r%d", r) }

	switch inst.Op {
	// Binary arithmetic
	case mir2.OpAdd:
		def("add", reg(a), reg(b2))
	case mir2.OpSub:
		def("sub", reg(a), reg(b2))
	case mir2.OpMul:
		def("mul", reg(a), reg(b2))
	case mir2.OpDiv:
		def("udiv", reg(a), reg(b2))
	case mir2.OpSDiv:
		def("div", reg(a), reg(b2))
	case mir2.OpMod:
		def("urem", reg(a), reg(b2))

	// Bitwise
	case mir2.OpAnd:
		def("and", reg(a), reg(b2))
	case mir2.OpOr:
		def("or", reg(a), reg(b2))
	case mir2.OpXor:
		def("xor", reg(a), reg(b2))
	case mir2.OpShl:
		def("shl", reg(a), reg(b2))
	case mir2.OpShr:
		def("shr", reg(a), reg(b2))
	case mir2.OpSar:
		def("sar", reg(a), reg(b2))

	// Unary
	case mir2.OpNeg:
		def("neg", reg(a))
	case mir2.OpNot:
		// QBE has no bitwise-not; use xor with -1
		def("xor", reg(a), "-1")

	// Conversions — QBE's w is 32-bit; zero/sign extend within w is implicit
	case mir2.OpExt:
		def("copy", reg(a)) // zero-extend: already wider in w
	case mir2.OpSext:
		srcBits := typeBits(inst.SrcTy)
		if srcBits == 8 {
			def("extsb", reg(a))
		} else {
			def("extsh", reg(a))
		}
	case mir2.OpTrunc:
		bits := typeBits(inst.Ty)
		mask := (int64(1) << bits) - 1
		def("and", reg(a), fmt.Sprintf("%d", mask))

	// Comparison → produces 0 or 1
	case mir2.OpCmp:
		// Signed if the condition is an ordered comparison (lt/le/gt/ge).
		// MIR2 CmpLt/Le/Gt/Ge are always signed (see ops.go comments).
		signed := inst.Cond == mir2.CmpLt || inst.Cond == mir2.CmpLe ||
			inst.Cond == mir2.CmpGt || inst.Cond == mir2.CmpGe
		pred := qbeCmp(inst.Cond, signed)
		g.printf("\t%%r%d =w c%sw %s, %s\n", dst, pred, reg(a), reg(b2))

	// Const / move
	case mir2.OpConst:
		g.printf("\t%%r%d =%s copy %d\n", dst, ty, inst.Imm)
	case mir2.OpMove:
		// Use g.regTy[dst] so pointer-promoted regs get "l", not "w"
		dstTy := ty
		if t := g.regTy[dst]; t != "" {
			dstTy = t
		}
		g.printf("\t%%r%d =%s copy %%r%d\n", dst, dstTy, a)

	// Memory
	case mir2.OpLoad:
		bits := typeBits(inst.Ty)
		var loadOp string
		switch {
		case bits <= 8:
			loadOp = "loadub"
		case bits <= 16:
			loadOp = "loaduh"
		default:
			loadOp = "loadw"
		}
		g.printf("\t%%r%d =%s %s %s\n", dst, ty, loadOp, g.ptrReg(a, dst, "ld"))

	case mir2.OpStore:
		ptr, val := inst.Src[0], inst.Src[1]
		bits := typeBits(inst.Ty)
		var storeOp string
		switch {
		case bits <= 8:
			storeOp = "storeb"
		case bits <= 16:
			storeOp = "storeh"
		default:
			storeOp = "storew"
		}
		g.printf("\t%s %s, %s\n", storeOp, reg(val), g.ptrReg(ptr, dst, "st"))

	case mir2.OpAddrOf:
		g.printf("\t%%r%d =l copy $%s\n", dst, inst.Sym)

	case mir2.OpAlloca:
		// alloc4 = 4-byte aligned; alloc8 = 8-byte aligned
		g.printf("\t%%r%d =l alloc4 %d\n", dst, inst.Imm)

	case mir2.OpField, mir2.OpPtrBump:
		// pointer + compile-time offset — base may be w (u16 addr on Z80)
		g.printf("\t%%r%d =l add %s, %d\n", dst, g.ptrReg(a, dst, "fld"), inst.Imm)

	case mir2.OpPtrAdd:
		// pointer + runtime offset. Both may be "w" when ptr is typed u16 on Z80.
		// Zero-extend both to "l" for a valid QBE 64-bit add.
		baseL := g.ptrReg(a, dst, "pa")
		idxL := fmt.Sprintf("%%rpaidx%d", dst)
		g.printf("\t%s =l extsw %s\n", idxL, reg(b2))
		g.printf("\t%%r%d =l add %s, %s\n", dst, baseL, idxL)

	// Function calls
	case mir2.OpCall:
		g.emitCall(inst)

	case mir2.OpCallIndirect:
		g.emitCallIndirect(inst)

	// Z80-specific SMC ops — model as simple values for QBE
	case mir2.OpPatchSlot:
		g.printf("\t%%r%d =%s copy %d\n", dst, ty, inst.Imm)
	case mir2.OpLoadPatched:
		def("copy", reg(a))
	case mir2.OpPatch:
		// side-effect only, no result — nothing to emit in QBE

	// Z80-specific stack / asm — skip
	case mir2.OpPush, mir2.OpPop, mir2.OpAsm:
		// not representable in QBE; safe to omit for correctness testing

	default:
		g.printf("\t# unhandled op %v\n", inst.Op)
	}
}

func (g *gen) emitCall(inst *mir2.Inst) {
	callExpr := g.buildCallExpr("$"+inst.Sym, inst.Args, inst.Ty)
	if inst.Dst != mir2.NoReg {
		g.printf("\t%%r%d =%s %s\n", inst.Dst, qbeTy(inst.Ty), callExpr)
	} else {
		g.printf("\t%s\n", callExpr)
	}
}

func (g *gen) emitCallIndirect(inst *mir2.Inst) {
	fnPtr := fmt.Sprintf("%%r%d", inst.Src[0])
	callExpr := g.buildCallExpr(fnPtr, inst.Args, inst.Ty)
	if inst.Dst != mir2.NoReg {
		g.printf("\t%%r%d =%s %s\n", inst.Dst, qbeTy(inst.Ty), callExpr)
	} else {
		g.printf("\t%s\n", callExpr)
	}
}

func (g *gen) buildCallExpr(fn string, args []mir2.Reg, retTy mir2.Ty) string {
	// Use g.regTy for each arg so pointer regs are passed as "l", matching
	// the callee's parameter declaration (also promoted to "l" if pointer).
	argParts := make([]string, len(args))
	for i, a := range args {
		ty := g.regTy[a]
		if ty == "" {
			ty = "w"
		}
		argParts[i] = fmt.Sprintf("%s %%r%d", ty, a)
	}
	return fmt.Sprintf("call %s(%s)", fn, strings.Join(argParts, ", "))
}

// ── terminators ───────────────────────────────────────────────────────────────

func (g *gen) emitTerm(t mir2.Term) {
	switch t := t.(type) {
	case *mir2.TermJmp:
		g.printf("\tjmp @%s\n", t.Target)

	case *mir2.TermBrIf:
		g.printf("\tjnz %%r%d, @%s, @%s\n", t.Cond, t.Then, t.Else)

	case *mir2.TermBrIf2:
		// Three-way: eq / lt / gt. Expand to two branches.
		// Synthetic intermediate block name is deterministic: {Eq}_ltcheck
		// so buildPreds can record the correct predecessor for Lt and Gt edges.
		ltCheck := t.Eq + "_ltcheck"
		lhs, rhs := fmt.Sprintf("%%r%d", t.Lhs), fmt.Sprintf("%%r%d", t.Rhs)
		teq := g.freshTmp()
		tlt := g.freshTmp()
		g.printf("\t%s =w ceqw %s, %s\n", teq, lhs, rhs)
		g.printf("\tjnz %s, @%s, @%s\n", teq, t.Eq, ltCheck)
		g.printf("@%s\n", ltCheck)
		g.printf("\t%s =w cultw %s, %s\n", tlt, lhs, rhs)
		g.printf("\tjnz %s, @%s, @%s\n", tlt, t.Lt, t.Gt)

	case *mir2.TermDJNZ:
		// DJNZ = decrement counter, branch if non-zero
		// counter - 1; if != 0 → body, else → exit
		ctr := fmt.Sprintf("%%r%d", t.Counter)
		dec := g.freshTmp()
		nz := g.freshTmp()
		g.printf("\t%s =w sub %s, 1\n", dec, ctr)
		g.printf("\t%s =w cnew %s, 0\n", nz, dec)
		g.printf("\tjnz %s, @%s, @%s\n", nz, t.Body, t.Exit)

	case *mir2.TermRet:
		if len(t.Vals) == 0 {
			g.line("\tret")
		} else {
			g.printf("\tret %%r%d\n", t.Vals[0])
		}

	case *mir2.TermUnreachable:
		g.line("\thlt")

	case nil:
		g.line("\t# missing terminator")
	}
}

// ── predecessor map ───────────────────────────────────────────────────────────

// buildPreds constructs g.preds for function f.
// preds[label][paramIndex] = []predEdge{(from, argReg), ...}
func (g *gen) buildPreds(f *mir2.Func) {
	// Index blocks by label
	blockByLabel := make(map[string]*mir2.Block, len(f.Blocks))
	for _, b := range f.Blocks {
		blockByLabel[b.Label] = b
	}

	g.preds = make(map[string][][]predEdge)

	addEdge := func(from, to string, args []mir2.Reg) {
		dst, ok := blockByLabel[to]
		if !ok {
			return
		}
		slots := g.preds[to]
		if slots == nil {
			slots = make([][]predEdge, len(dst.Params))
			g.preds[to] = slots
		}
		for i := range args {
			if i < len(slots) {
				slots[i] = append(slots[i], predEdge{from: from, arg: args[i]})
			}
		}
	}

	for _, b := range f.Blocks {
		switch t := b.Term.(type) {
		case *mir2.TermJmp:
			addEdge(b.Label, t.Target, t.Args)
		case *mir2.TermBrIf:
			addEdge(b.Label, t.Then, t.ThenArgs)
			addEdge(b.Label, t.Else, t.ElseArgs)
		case *mir2.TermBrIf2:
			// Eq is reached directly from b; Lt and Gt are reached from the
			// synthetic {Eq}_ltcheck block (see emitTerm TermBrIf2 case).
			ltCheck := t.Eq + "_ltcheck"
			addEdge(b.Label, t.Eq, t.EqArgs)
			addEdge(ltCheck, t.Lt, t.LtArgs)
			addEdge(ltCheck, t.Gt, t.GtArgs)
		case *mir2.TermDJNZ:
			// Body block's first param implicitly gets the decremented counter.
			// We synthesize it: the decremented value is produced inline in emitTerm.
			// For now, pass an empty args slice (DJNZ body param 0 comes from dec).
			addEdge(b.Label, t.Body, t.BodyArgs)
			addEdge(b.Label, t.Exit, t.ExitArgs)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// findPointerRegs returns the set of virtual registers used as memory addresses.
// On Z80, these are u16 (MIR2 type TyU16 or TyPtr), but on native 64-bit targets
// they must be "l" (64-bit). We propagate bidirectionally through moves and
// phi edges so that all regs that carry the same address value get "l" type.
func findPointerRegs(f *mir2.Func) map[mir2.Reg]bool {
	ptrs := make(map[mir2.Reg]bool)

	// Seed: registers directly used as pointer operands, or produced by
	// OpAddrOf/OpAlloca/OpPtrAdd/OpField/OpPtrBump (which produce TyPtr).
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			switch inst.Op {
			case mir2.OpPtrAdd, mir2.OpField, mir2.OpPtrBump,
				mir2.OpAddrOf, mir2.OpAlloca:
				ptrs[inst.Dst] = true
				ptrs[inst.Src[0]] = true // base is also a ptr
			case mir2.OpLoad:
				ptrs[inst.Src[0]] = true
			case mir2.OpStore:
				ptrs[inst.Src[0]] = true // Src[0] = ptr
			}
			// Any reg with MIR2 type TyPtr is always a pointer.
			if inst.Ty == mir2.TyPtr && inst.Dst != mir2.NoReg {
				ptrs[inst.Dst] = true
			}
		}
	}
	// Function params declared as TyPtr are pointers.
	for _, p := range f.Contract.Params {
		if p.Ty == mir2.TyPtr {
			ptrs[p.Reg] = true
		}
	}

	// Propagate bidirectionally to fixed point.
	// Two rules:
	//   (1) move/phi edge: if either side is ptr → both sides are ptr
	//   (2) terminator arg → block param: pointer-ness flows both ways
	mark := func(r mir2.Reg) bool {
		if ptrs[r] {
			return false
		}
		ptrs[r] = true
		return true
	}

	changed := true
	for changed {
		changed = false
		// Propagate through OpMove: ptr ↔ Src[0]
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op == mir2.OpMove && inst.Dst != mir2.NoReg {
					if ptrs[inst.Src[0]] {
						changed = mark(inst.Dst) || changed
					}
					if ptrs[inst.Dst] {
						changed = mark(inst.Src[0]) || changed
					}
				}
			}
		}
		// Propagate through terminator args ↔ block params (bidirectional).
		for _, b := range f.Blocks {
			propagateArgs := func(target string, args []mir2.Reg) {
				tb := blockByLabel(f, target)
				if tb == nil {
					return
				}
				for i, arg := range args {
					if i >= len(tb.Params) {
						break
					}
					param := tb.Params[i].Dst
					if ptrs[arg] {
						changed = mark(param) || changed
					}
					if ptrs[param] {
						changed = mark(arg) || changed
					}
				}
			}
			switch t := b.Term.(type) {
			case *mir2.TermJmp:
				propagateArgs(t.Target, t.Args)
			case *mir2.TermBrIf:
				propagateArgs(t.Then, t.ThenArgs)
				propagateArgs(t.Else, t.ElseArgs)
			case *mir2.TermBrIf2:
				propagateArgs(t.Eq, t.EqArgs)
				propagateArgs(t.Lt, t.LtArgs)
				propagateArgs(t.Gt, t.GtArgs)
			case *mir2.TermDJNZ:
				propagateArgs(t.Body, t.BodyArgs)
				propagateArgs(t.Exit, t.ExitArgs)
			}
		}
	}
	return ptrs
}

func blockByLabel(f *mir2.Func, label string) *mir2.Block {
	for _, b := range f.Blocks {
		if b.Label == label {
			return b
		}
	}
	return nil
}

// scanRegTypes builds g.regTy mapping each virtual register to its QBE type.
// u16 registers used as memory addresses are promoted to "l" (64-bit pointer).
func (g *gen) scanRegTypes(f *mir2.Func) {
	ptrRegs := findPointerRegs(f)
	g.regTy = make(map[mir2.Reg]string)
	setTy := func(r mir2.Reg, ty mir2.Ty) {
		t := qbeTy(ty)
		if ptrRegs[r] {
			t = "l"
		}
		g.regTy[r] = t
	}
	for _, p := range f.Contract.Params {
		setTy(p.Reg, p.Ty)
	}
	for _, b := range f.Blocks {
		for _, bp := range b.Params {
			setTy(bp.Dst, bp.Ty)
		}
		for _, inst := range b.Insts {
			if inst.Dst != mir2.NoReg {
				setTy(inst.Dst, inst.Ty)
			}
		}
	}
}

// ptrReg returns reg(r) if r is already "l"-typed, otherwise emits a
// zero-extension to "l" and returns the extended tmp name.
// tag is a short label for the tmp name (for readability).
func (g *gen) ptrReg(r mir2.Reg, ctx mir2.Reg, tag string) string {
	rStr := fmt.Sprintf("%%r%d", r)
	if g.regTy[r] == "l" {
		return rStr
	}
	// r is "w" (e.g. u16 address) — zero-extend to l
	tmp := fmt.Sprintf("%%rp%s%d", tag, ctx)
	g.printf("\t%s =l extsw %s\n", tmp, rStr)
	return tmp
}

var tmpCounter int

func (g *gen) freshTmp() string {
	tmpCounter++
	return fmt.Sprintf("%%qtmp%d", tmpCounter)
}

func (g *gen) printf(format string, args ...any) {
	fmt.Fprintf(g.sb, format, args...)
}

func (g *gen) line(s string) {
	g.sb.WriteString(s)
	g.sb.WriteByte('\n')
}

// qbeTy maps a MIR2 type to a QBE base type letter.
func qbeTy(ty mir2.Ty) string {
	if ty == nil || ty == mir2.TyVoid {
		return ""
	}
	if ty == mir2.TyPtr {
		return "l"
	}
	return "w" // u8, u16, i8, i16, bool all fit in QBE word
}

// qbeCmp maps a MIR2 CmpCond to a QBE comparison prefix.
// signed=true → use signed variants (slt, sle, sgt, sge).
func qbeCmp(c mir2.CmpCond, signed bool) string {
	switch c {
	case mir2.CmpEq:
		return "eq"
	case mir2.CmpNe:
		return "ne"
	case mir2.CmpLt:
		if signed {
			return "slt"
		}
		return "ult"
	case mir2.CmpLe:
		if signed {
			return "sle"
		}
		return "ule"
	case mir2.CmpGt:
		if signed {
			return "sgt"
		}
		return "ugt"
	case mir2.CmpGe:
		if signed {
			return "sge"
		}
		return "uge"
	}
	return "eq"
}

func isSigned(ty mir2.Ty) bool {
	if ty == nil {
		return false
	}
	switch ty {
	case mir2.TyI8, mir2.TyI16:
		return true
	}
	return false
}

func typeBits(ty mir2.Ty) int {
	if ty == nil {
		return 32
	}
	switch ty {
	case mir2.TyBool:
		return 1
	case mir2.TyU8, mir2.TyI8:
		return 8
	case mir2.TyU16, mir2.TyI16:
		return 16
	case mir2.TyPtr:
		return 64
	}
	return 32
}

func byteWidth(ty mir2.Ty) int {
	// StructTy uses its own Width() (sum of field widths in bits).
	if _, isStruct := ty.(*mir2.StructTy); isStruct {
		bw := ty.Width() / 8
		if bw < 1 {
			return 1
		}
		return bw
	}
	bits := typeBits(ty)
	if bits < 8 {
		return 1
	}
	return bits / 8
}
