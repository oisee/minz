package mir2

import "fmt"

// Inst is a single MIR2 instruction.
//
// Field usage by opcode — unlisted fields are zero/nil:
//
//	Binary (OpAdd…OpSar):       Dst, Src[0], Src[1], Ty, Cls
//	Unary (OpNeg, OpNot):       Dst, Src[0], Ty, Cls
//	Convert (OpExt/Sext/Trunc): Dst, Src[0], SrcTy, Ty(=dest), Cls
//	OpCmp:                      Dst, Src[0], Src[1], Cond, Ty(=TyBool), Cls
//	OpConst:                    Dst, Imm, Ty, Cls
//	OpMove:                     Dst, Src[0], Ty, Cls
//	OpLoad:                     Dst, Src[0](=ptr), Ty, Cls
//	OpStore:                    Src[0](=ptr), Src[1](=val), Ty   (Dst=NoReg)
//	OpAddrOf:                   Dst, Sym, Ty(=TyPtr), Cls
//	OpAlloca:                   Dst, Imm(=bytes), Ty(=TyPtr)
//	OpCall:                     Dst, Sym(=fn), Args, Ty, Cls, CallAttr
//	OpCallIndirect:             Dst, Src[0](=fn ptr), Args, Ty, Cls, CallAttr
//	OpPatchSlot:                Dst, Imm(=init val), Ty, Cls
//	OpLoadPatched:              Dst, Src[0](=slot reg), Ty, Cls
//	OpPatch:                    Src[0](=slot reg), Src[1](=new val)  (Dst=NoReg)
//	OpAsm:                      Asm field (all data inside AsmExtra)
//
// Note: MIR2 uses block arguments, not phi nodes.
// Join-point values are expressed as BlockParam entries on the target Block
// and as Args slices on TermJmp / TermBrIf.
type Inst struct {
	Op  Op
	Dst Reg    // result register; NoReg if the instruction produces no value
	Src [2]Reg // up to 2 operand registers

	Imm int64  // immediate value (OpConst, OpAlloca byte count, OpPatchSlot init)
	Sym string // symbol name (OpAddrOf, OpCall / intrinsic names)
	Ty  Ty     // type of Dst (or operand type for OpStore)

	// Register class constraint.
	//
	// Cls is the *preferred* class — the allocator tries to honour it but may
	// fall back to a broader class when the preferred physical register is
	// occupied (e.g. [counter] → B preferred, C/D/E allowed at small extra cost).
	//
	// ClsHard overrides this: the allocator MUST place Dst in a physical register
	// of class Cls, or spill.  Use ClsHard only for instructions that are
	// semantically tied to a specific register (e.g. the B operand of OpAsm
	// containing a DJNZ template, or an ABI boundary).
	Cls     RegClass
	ClsHard bool // if true, Cls is a hard constraint (see above)

	// Comparison predicate (OpCmp)
	Cond CmpCond

	// Source type for conversion ops (OpExt, OpSext, OpTrunc).
	// Ty holds the *destination* type.
	SrcTy Ty

	// Call metadata (OpCall, OpCallIndirect)
	CallAttr CallAttrs

	// Variadic argument list (OpCall, OpCallIndirect)
	Args []Reg

	// Inline assembly (OpAsm)
	Asm *AsmExtra

	// Source location (preserved for error messages and SLD generation)
	SrcLine int
	SrcFile string
}

// ── Register class helpers ────────────────────────────────────────────────────

// PreferClass returns the preferred (soft) class for this instruction's result.
func (inst *Inst) PreferClass() RegClass { return inst.Cls }

// RequiresClass reports whether the result *must* be in class Cls.
func (inst *Inst) RequiresClass() bool { return inst.ClsHard }

// ── Call attributes ───────────────────────────────────────────────────────────

// CallAttrs are modifiers on a call instruction.
type CallAttrs struct {
	// IsComptime: must be resolved at compile time.
	// The MIR2 VM executes the callee and folds the result.
	// Used for @-prefixed MinZ metafunctions.
	IsComptime bool

	// IsHost: calls into the compiler host environment.
	// Used for @emit and similar compile-time side-effects.
	// Only valid inside [comptime] functions; error at runtime.
	IsHost bool
}

// ── Inline assembly ───────────────────────────────────────────────────────────

// AsmExtra holds the extended fields for OpAsm.
type AsmExtra struct {
	Template string   // target-specific template string
	Ins      []Reg    // input operand registers (read by the asm block)
	Outs     []Reg    // output operand registers (written by the asm block)
	Clobbers ClassSet // classes clobbered but not listed in Outs
}

// ── Use/def analysis ─────────────────────────────────────────────────────────

// Uses returns all Reg values *read* by this instruction (Dst excluded).
func (inst *Inst) Uses() []Reg {
	switch inst.Op {
	case OpStore, OpPatch:
		return filterNoReg(inst.Src[0], inst.Src[1])
	case OpPush:
		if inst.Src[0] != NoReg {
			return []Reg{inst.Src[0]}
		}
		return nil
	case OpPop:
		// no uses (pops from implicit stack)
		return nil
	case OpCall:
		return inst.Args
	case OpCallIndirect:
		out := make([]Reg, 0, len(inst.Args)+1)
		if inst.Src[0] != NoReg {
			out = append(out, inst.Src[0])
		}
		return append(out, inst.Args...)
	case OpAsm:
		if inst.Asm != nil {
			return inst.Asm.Ins
		}
		return nil
	default:
		return filterNoReg(inst.Src[0], inst.Src[1])
	}
}

func filterNoReg(regs ...Reg) []Reg {
	var out []Reg
	for _, r := range regs {
		if r != NoReg {
			out = append(out, r)
		}
	}
	return out
}

// ── Terminators ───────────────────────────────────────────────────────────────

// Term is the mandatory terminator of a basic block.
// Terminators carry block-argument values to successor blocks, which
// eliminates the need for phi nodes.
//
// Concrete types: *TermJmp, *TermBrIf, *TermBrIf2, *TermDJNZ, *TermCondRet, *TermRet, *TermUnreachable.
type Term interface {
	isTerm()
	termUses() []Reg   // all Reg values read by this terminator
	Successors() []string // successor block labels
}

// TermJmp is an unconditional jump with optional block arguments.
//
//	jmp @target(%arg0, %arg1, ...)
//
// Args are matched positionally to the target block's Params.
type TermJmp struct {
	Target string
	Args   []Reg // values for target block's BlockParams (may be empty)
}

func (*TermJmp) isTerm()               {}
func (t *TermJmp) termUses() []Reg      { return t.Args }
func (t *TermJmp) Successors() []string { return []string{t.Target} }

// TermBrIf is a conditional branch with optional block arguments per edge.
//
//	br_if %cond, @then(%ta0, %ta1, ...), @else(%ea0, ...)
//
// Cond != 0 → Then with ThenArgs.
// Cond == 0 → Else with ElseArgs.
type TermBrIf struct {
	Cond     Reg
	Then     string
	ThenArgs []Reg
	Else     string
	ElseArgs []Reg
}

func (*TermBrIf) isTerm()               {}
func (t *TermBrIf) termUses() []Reg {
	all := make([]Reg, 0, 1+len(t.ThenArgs)+len(t.ElseArgs))
	all = append(all, t.Cond)
	all = append(all, t.ThenArgs...)
	all = append(all, t.ElseArgs...)
	return all
}
func (t *TermBrIf) Successors() []string { return []string{t.Then, t.Else} }

// TermBrIf2 is a three-way branch driven by a single unsigned comparison.
//
//	br_if2 %lhs, %rhs, @eq(...), @lt(...), @gt(...)
//
// On Z80: one CP instruction sets both Z (equal) and C (less-than) flags.
//   - Z set   → Eq branch  (lhs == rhs)
//   - C set   → Lt branch  (lhs <  rhs, unsigned)
//   - neither → Gt branch  (lhs >  rhs)
//
// This eliminates the redundant second CP in sequences like GCD where both
// the equality and ordering checks are needed after one comparison.
type TermBrIf2 struct {
	Lhs    Reg
	Rhs    Reg    // compared against Lhs; may alias a constant via OpConst
	Eq     string; EqArgs []Reg // Z flag set: lhs == rhs
	Lt     string; LtArgs []Reg // C flag set: lhs <  rhs (unsigned)
	Gt     string; GtArgs []Reg // no flag:    lhs >  rhs
}

func (*TermBrIf2) isTerm() {}
func (t *TermBrIf2) termUses() []Reg {
	all := make([]Reg, 0, 2+len(t.EqArgs)+len(t.LtArgs)+len(t.GtArgs))
	if t.Lhs != NoReg {
		all = append(all, t.Lhs)
	}
	if t.Rhs != NoReg {
		all = append(all, t.Rhs)
	}
	all = append(all, t.EqArgs...)
	all = append(all, t.LtArgs...)
	all = append(all, t.GtArgs...)
	return all
}
func (t *TermBrIf2) Successors() []string { return []string{t.Eq, t.Lt, t.Gt} }

// TermDJNZ is the Z80 DJNZ (Decrement B and Jump if Non-Zero) terminator.
//
// DJNZ atomically decrements the B register (ClassCounter) and branches to
// Body if B != 0, or falls through to Exit if B == 0.
//
//	Z80: DJNZ rel8  — 13T taken (B≠0), 8T not taken (B=0)
//
// Compared to the equivalent DEC B (4T) + JP NZ (10T) + pre-check AND A (4T) + JP Z (10T)
// = 28T, DJNZ saves 15T per iteration.
//
// Counter is the virtual register holding the current iteration count.
// It must be allocated to ClassCounter (B register).
// After DJNZ, B holds (Counter - 1); on the next iteration Body receives this value.
//
// BodyArgs: args for body block params EXCLUDING the counter (body.Params[0]).
// The body block's first param (ClassCounter) implicitly receives the decremented B.
// ExitArgs: args for exit block params.
type TermDJNZ struct {
	Counter  Reg
	Body     string
	BodyArgs []Reg
	Exit     string
	ExitArgs []Reg
}

func (*TermDJNZ) isTerm() {}
func (t *TermDJNZ) termUses() []Reg {
	all := make([]Reg, 0, 1+len(t.BodyArgs)+len(t.ExitArgs))
	if t.Counter != NoReg {
		all = append(all, t.Counter)
	}
	all = append(all, t.BodyArgs...)
	all = append(all, t.ExitArgs...)
	return all
}
func (t *TermDJNZ) Successors() []string { return []string{t.Body, t.Exit} }

// TermCondRet is a conditional return.
//
//	cond_ret %cond, [%v0, %v1, ...], @then(%ta0, ...)
//
// Cond == 0  → return Vals  (the "short-circuit" path)
// Cond != 0  → jump to Then with ThenArgs
//
// Z80: emit return-value moves, then RET CC (inverted condition).
//
//	Example (abs_diff, a≥b path):  SUB C / RET NC / NEG / RET
//
// ARM: RETNE / RETEQ / RETCS / etc.
//
// This terminator is created by CondRetSink from a BrIf whose else-branch
// is a trivial return block.
type TermCondRet struct {
	Cond     Reg
	Vals     []Reg   // return values when Cond == 0
	Then     string  // block to jump to when Cond != 0
	ThenArgs []Reg
}

func (*TermCondRet) isTerm() {}
func (t *TermCondRet) termUses() []Reg {
	all := make([]Reg, 0, 1+len(t.Vals)+len(t.ThenArgs))
	all = append(all, t.Cond)
	all = append(all, t.Vals...)
	all = append(all, t.ThenArgs...)
	return all
}
func (t *TermCondRet) Successors() []string { return []string{t.Then} }

// TermRet returns from the function.
// Vals is empty for void returns; multiple values for multi-return.
type TermRet struct{ Vals []Reg }

func (*TermRet) isTerm()               {}
func (t *TermRet) termUses() []Reg      { return t.Vals }
func (*TermRet) Successors() []string   { return nil }

// TermUnreachable marks dead code.
type TermUnreachable struct{}

func (*TermUnreachable) isTerm()               {}
func (*TermUnreachable) termUses() []Reg        { return nil }
func (*TermUnreachable) Successors() []string   { return nil }

// TermString formats a terminator for dump output.
func TermString(term Term) string {
	switch t := term.(type) {
	case *TermJmp:
		return fmt.Sprintf("jmp @%s(%s)", t.Target, regList(t.Args))
	case *TermBrIf:
		return fmt.Sprintf("br_if %%r%d, @%s(%s), @%s(%s)",
			t.Cond, t.Then, regList(t.ThenArgs), t.Else, regList(t.ElseArgs))
	case *TermBrIf2:
		return fmt.Sprintf("br_if2 %%r%d, %%r%d, @%s(%s), @%s(%s), @%s(%s)",
			t.Lhs, t.Rhs,
			t.Eq, regList(t.EqArgs),
			t.Lt, regList(t.LtArgs),
			t.Gt, regList(t.GtArgs))
	case *TermDJNZ:
		return fmt.Sprintf("djnz %%r%d, @%s(%s), @%s(%s)",
			t.Counter, t.Body, regList(t.BodyArgs), t.Exit, regList(t.ExitArgs))
	case *TermCondRet:
		return fmt.Sprintf("cond_ret %%r%d, [%s], @%s(%s)",
			t.Cond, regList(t.Vals), t.Then, regList(t.ThenArgs))
	case *TermRet:
		if len(t.Vals) == 0 {
			return "ret void"
		}
		return "ret " + regList(t.Vals)
	case *TermUnreachable:
		return "unreachable"
	}
	return "??term"
}

func regList(regs []Reg) string {
	if len(regs) == 0 {
		return ""
	}
	s := fmt.Sprintf("%%r%d", regs[0])
	for _, r := range regs[1:] {
		s += fmt.Sprintf(", %%r%d", r)
	}
	return s
}
