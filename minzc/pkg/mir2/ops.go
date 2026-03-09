package mir2

// Op is a MIR2 core opcode (~28 values total).
//
// Design: "structurally modern, semantically modest, backend-pragmatic."
// — target-specific ops go through intrinsic calls (@mir.z80.*, @mir.x86.*, …)
// — high-level built-ins go through intrinsic calls (@mir.io.print.u8, …)
// — join points use block arguments on Block.Params, not phi nodes
//
// Field layout per opcode is documented on the Inst struct.
type Op uint8

const (
	// ── Binary arithmetic ──────────────────────────────────────────────────────
	// Inst: Dst = Src[0] op Src[1] : Ty [Cls]
	OpAdd  Op = iota // add  %a, %b : ty
	OpSub            // sub  %a, %b : ty
	OpMul            // mul  %a, %b : ty  (unsigned)
	OpDiv            // div  %a, %b : ty  (unsigned)
	OpSDiv           // sdiv %a, %b : ty  (signed)
	OpMod            // mod  %a, %b : ty  (unsigned remainder)

	// ── Binary bitwise ─────────────────────────────────────────────────────────
	OpAnd // and %a, %b : ty
	OpOr  // or  %a, %b : ty
	OpXor // xor %a, %b : ty
	OpShl // shl %a, %b : ty  (logical left shift)
	OpShr // shr %a, %b : ty  (logical right shift, zero-fill)
	OpSar // sar %a, %b : ty  (arithmetic right shift, sign-extend)

	// ── Unary ──────────────────────────────────────────────────────────────────
	// Inst: Dst = op Src[0] : Ty [Cls]
	OpNeg   // neg   %a : ty            (arithmetic negate)
	OpNot   // not   %a : ty            (bitwise complement)
	OpExt   // ext   %a : srcTy -> ty   (zero-extend)
	OpSext  // sext  %a : srcTy -> ty   (sign-extend)
	OpTrunc // trunc %a : srcTy -> ty   (truncate)

	// ── Comparison ─────────────────────────────────────────────────────────────
	// Inst: Dst = cmp.<Cond> Src[0], Src[1] : bool [Cls]
	// Cls=ClassFlag → backend uses CPU flag; no register materialisation.
	// Cls soft by default; ClsHard when the flag is consumed immediately.
	OpCmp

	// ── Data movement ──────────────────────────────────────────────────────────
	OpConst // const <Imm>    : ty [cls]  — immediate constant
	OpMove  // move  %src     : ty [cls]  — register copy

	// ── Memory ─────────────────────────────────────────────────────────────────
	OpLoad   // load  %ptr        : ty [cls]  — load from memory address
	OpStore  // store %ptr, %val  : ty        — store to memory (Dst=NoReg)
	OpAddrOf // addr_of <Sym>     : ptr [cls] — address of global/func/string
	OpAlloca // alloca <Imm>      : ptr       — reserve Imm bytes on stack frame

	// OpField advances a pointer by a compile-time byte offset (struct field access).
	// Inst: Dst = field %Src[0], Imm=byte_offset : ptr [cls]
	//
	// Z80 codegen:
	//   offset 0      → dst = src (copy only if different register)
	//   offset 1..3   → INC dst  ×N       (1B/6T each, no flag change)
	//   offset 4+     → LD BC,N; ADD HL,BC  (4B/21T, but faster than 4×INC)
	//
	// The caller is responsible for keeping the base pointer alive if needed
	// again after the field access (assign it a different register class).
	OpField // field %base, imm=byte_offset : ptr [cls]
	// OpPtrAdd adds a runtime u16 offset to a pointer.
	// Inst: Dst = Src[0](=base_ptr) + Src[1](=offset_u16) : TyPtr [cls]
	// Z80: base in HL, offset in DE → ADD HL, DE  (or appropriate pair)
	OpPtrAdd // ptr_add %base, %offset : ptr [cls]

	// OpPtrBump advances an iterator pointer by a compile-time stride.
	// Inst: Dst = ptr_bump %Src[0], Imm=stride : ptr [cls]
	//
	// Semantically equivalent to OpField but marks iterator-style advancement
	// (not struct field access), enabling future layout-aware lowering:
	//   AoS:     INC HL  ×stride  (or ADD HL,BC for stride≥4)
	//   SoA:     INC L            (advance within column, H stays = field page)
	//   SoA256:  INC L  or  INC H (element vs column switch)
	//
	// For Phase 3c, the Z80 codegen is identical to OpField.
	// SoA256-specific lowering is deferred to Phase 3c/5c.
	OpPtrBump // ptr_bump %ptr, imm=stride : ptr [cls]

	// ── Calls ──────────────────────────────────────────────────────────────────
	// Intrinsic calls: OpCall with Sym = "@mir.<subsystem>.<op>"
	// Example: "@mir.io.print.u8", "@mir.z80.port_out", "@mir.mem.memcpy"
	OpCall         // call @Sym(%Args...)          : ty [cls]
	OpCallIndirect // call_indirect %Src[0](%Args...) : ty [cls]

	// ── Platform-independent SMC ───────────────────────────────────────────────
	//
	// A mutable immediate is a value embedded inside an instruction stream that
	// can be overwritten at runtime.  Three primitives cover the full model;
	// the backend chooses the physical representation:
	//   Z80:  byte(s) inside the instruction encoding  (LD A, <imm>)
	//   x86:  immediate field in MOV/ADD instruction
	//   C:    static local variable
	//   WASM: module-level global
	//
	// On "safe" backends (C, WASM, interpreter) patch_slot lowers to a plain
	// mutable variable — semantically identical, zero overhead.
	OpPatchSlot   // %slot = patch_slot <Imm_init> : ty [cls]   — declare mutable imm
	OpLoadPatched // %v    = load_patched %slot     : ty [cls]   — read current value
	OpPatch       //         patch %slot, %newval               — write new value (Dst=NoReg)

	// ── Stack operations ───────────────────────────────────────────────────────
	// Callee-save scaffold for preserving register pairs across calls.
	OpPush // push register pair to stack (Z80: PUSH rr)
	OpPop  // pop from stack into register pair (Z80: POP rr)

	// ── Inline assembly ────────────────────────────────────────────────────────
	// Template syntax is target-specific; see AsmExtra for fields.
	// Use ClsHard=true on result/input regs that the template hard-requires.
	OpAsm

	opCount // sentinel — must remain last
)

func (op Op) String() string {
	if int(op) < len(opNames) {
		return opNames[op]
	}
	return "??op"
}

var opNames = [opCount]string{
	OpAdd: "add", OpSub: "sub", OpMul: "mul", OpDiv: "div", OpSDiv: "sdiv", OpMod: "mod",
	OpAnd: "and", OpOr: "or", OpXor: "xor", OpShl: "shl", OpShr: "shr", OpSar: "sar",
	OpNeg: "neg", OpNot: "not", OpExt: "ext", OpSext: "sext", OpTrunc: "trunc",
	OpCmp:          "cmp",
	OpConst:        "const",
	OpMove:         "move",
	OpLoad:         "load",
	OpStore:        "store",
	OpAddrOf:       "addr_of",
	OpAlloca:       "alloca",
	OpField:        "field",
	OpPtrBump:      "ptr_bump",
	OpPtrAdd:       "ptr_add",
	OpCall:         "call",
	OpCallIndirect: "call_indirect",
	OpPatchSlot:    "patch_slot",
	OpLoadPatched:  "load_patched",
	OpPatch:        "patch",
	OpPush:         "push",
	OpPop:          "pop",
	OpAsm:          "asm",
}

// Predicate helpers

// IsBinary reports whether op takes two value operands.
func IsBinary(op Op) bool { return op >= OpAdd && op <= OpSar }

// IsUnary reports whether op takes one value operand (not conversions).
func IsUnary(op Op) bool { return op == OpNeg || op == OpNot }

// IsConversion reports whether op changes the type of a value.
func IsConversion(op Op) bool { return op == OpExt || op == OpSext || op == OpTrunc }

// IsSideEffect reports whether op has no result (Dst must be NoReg).
// OpPush and OpPop are side-effecting (modify stack pointer).
func IsSideEffect(op Op) bool { return op == OpStore || op == OpPatch || op == OpPush }

// ── Comparison predicate ──────────────────────────────────────────────────────

// CmpCond is the predicate for OpCmp.
type CmpCond uint8

const (
	CmpEq  CmpCond = iota // ==   (signed and unsigned identical)
	CmpNe                 // !=
	CmpLt                 // <  signed
	CmpLe                 // <= signed
	CmpGt                 // >  signed
	CmpGe                 // >= signed
	CmpUlt                // <  unsigned
	CmpUle                // <= unsigned
	CmpUgt                // >  unsigned
	CmpUge                // >= unsigned
)

func (c CmpCond) String() string {
	names := [...]string{"eq", "ne", "lt", "le", "gt", "ge", "ult", "ule", "ugt", "uge"}
	if int(c) < len(names) {
		return names[c]
	}
	return "??"
}

// Negate returns the logical negation (eq↔ne, lt↔ge, …).
func (c CmpCond) Negate() CmpCond {
	switch c {
	case CmpEq:
		return CmpNe
	case CmpNe:
		return CmpEq
	case CmpLt:
		return CmpGe
	case CmpLe:
		return CmpGt
	case CmpGt:
		return CmpLe
	case CmpGe:
		return CmpLt
	case CmpUlt:
		return CmpUge
	case CmpUle:
		return CmpUgt
	case CmpUgt:
		return CmpUle
	case CmpUge:
		return CmpUlt
	}
	return c
}

// Swap returns the condition with operands swapped (a op b → b op' a).
func (c CmpCond) Swap() CmpCond {
	switch c {
	case CmpLt:
		return CmpGt
	case CmpLe:
		return CmpGe
	case CmpGt:
		return CmpLt
	case CmpGe:
		return CmpLe
	case CmpUlt:
		return CmpUgt
	case CmpUle:
		return CmpUge
	case CmpUgt:
		return CmpUlt
	case CmpUge:
		return CmpUle
	}
	return c // eq/ne are symmetric
}
