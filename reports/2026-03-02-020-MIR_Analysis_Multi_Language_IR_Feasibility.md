# MinZ MIR Analysis — Multi-Language IR Feasibility Report

**Date:** 2026-03-02
**Version:** v0.19.5+
**Status:** Analysis complete

---

## Executive Summary

MinZ MIR is a **mature, hardware-aware intermediate representation** purpose-built for Z80 compilation. With 118 opcodes, 14 type variants, 13+ optimization passes, and a register-based VM, it is one of the most complete 8-bit IRs in existence. However, its deep Z80 coupling makes it **unsuitable as a portable multi-language IR** in its current form. This report evaluates what's excellent, what's Z80-specific, and what would need to change to support languages like PL/M, Ada, or Forth.

---

## 1. MIR at a Glance

| Metric | Value |
|--------|-------|
| Opcodes | 118 (13 categories) |
| Type system | 14 basic types + 10 compound types |
| IR definition | `pkg/ir/ir.go` — 1,433 LOC |
| Semantic layer | `pkg/semantic/analyzer.go` — 12,353 LOC |
| Z80 backend | `pkg/codegen/z80.go` — 5,822 LOC |
| Register allocator | `pkg/codegen/register_allocator.go` — 491 LOC |
| Optimizer pipeline | `pkg/optimizer/` — 13,114 LOC (39 files) |
| MIR VM | `pkg/mirvm/` — 4,604 LOC |
| Secondary backends | C (721), 6502 (527), Crystal (388), i8080 (663), GB (330), LLVM (493) |
| Total codegen | 14,559 LOC (30 files) |

### Opcode Categories

```
Control Flow (11):  OpNop, OpLabel, OpJump, OpJumpIf, OpJumpIfNot,
                    OpJumpIfZero, OpJumpIfNotZero, OpJumpIfFlag,
                    OpCall, OpCallIndirect, OpReturn

Data Movement (13): OpLoadConst, OpLoadVar, OpStoreVar, OpLoadParam,
                    OpLoadField, OpStoreField, OpLoadIndex, OpStoreIndex,
                    OpLoadElement, OpStoreElement, OpLoadBitField,
                    OpStoreBitField, OpMove, OpLoadLabel, OpLoadDirect,
                    OpStoreDirect

SMC (10):           OpSMCLoadConst, OpSMCStoreConst, OpSMCParam,
                    OpSMCSave, OpSMCRestore, OpSMCUpdate, OpStoreTSMCRef,
                    OpTrueSMCLoad, OpTrueSMCPatch, OpTSMCRefAnchor,
                    OpTSMCRefLoad, OpTSMCRefPatch

Patching (4):       OpPatchPoint, OpPatchTemplate, OpPatchTarget, OpPatchParam

Error (2):          OpSetError, OpCheckError

Array (3):          OpArrayInit, OpArrayElement, OpArrayLiteral

Arithmetic (8):     OpAdd, OpSub, OpMul, OpDiv, OpMod, OpNeg, OpInc, OpDec

Bitwise (6):        OpAnd, OpOr, OpXor, OpNot, OpShl, OpShr

Logic (2):          OpLogicalAnd, OpLogicalOr

Comparison (8):     OpCmp, OpTest, OpEq, OpNe, OpLt, OpGt, OpLe, OpGe

Memory (6):         OpAlloc, OpFree, OpLoadPtr, OpStorePtr, OpAddr,
                    OpLoad, OpStore

Stack (2):          OpPush, OpPop

Inline Asm (1):     OpAsm

Loop/Low-level (11): OpLoadAddr, OpCopyToBuffer, OpCopyFromBuffer, OpDJNZ,
                     OpLoadImm, OpLoadReg, OpLoadMem, OpStoreMem, OpAddImm,
                     OpJmp, OpJmpIf, OpJmpIfNot, OpPrintChar, OpHalt

Built-in (12):      OpPrint, OpPrintU8, OpPrintU16, OpPrintI8, OpPrintI16,
                    OpPrintBool, OpPrintString, OpPrintStringDirect,
                    OpLoadString, OpLen, OpMemcpy, OpMemset

Meta (1):           OpEmit

Cast (4):           OpCastInterface, OpCheckCast, OpMethodDispatch,
                    OpInterfaceCall (DEPRECATED)

I/O (3):            OpPortIn, OpPortOut, OpSyscall
```

### Type System

```
Basic (14):     void, bool, u8, u16, u24, u32, i8, i16, i24, i32,
                f8.8, f.8, f.16, f16.8, f8.16

Compound (10):  *T (PointerType), *mut T (mutable pointer),
                [N]T (ArrayType), String, LString,
                struct { fields } (StructType),
                enum { variants } (EnumType),
                bits<u8> (BitStructType),
                |params| -> ret (LambdaType),
                Iterator<T> (IteratorType),
                fun(params) -> ret (FunctionType),
                T: Bound (GenericType)
```

---

## 2. What's Excellent (Language-Independent)

### 2.1 Core Instruction Set

The arithmetic, bitwise, comparison, memory, and control flow opcodes are **fully portable** — any language targeting 8-bit machines needs exactly these operations:

```
OpAdd/Sub/Mul/Div/Mod/Neg/Inc/Dec     ← universal arithmetic
OpAnd/Or/Xor/Not/Shl/Shr              ← universal bitwise
OpEq/Ne/Lt/Gt/Le/Ge/Cmp/Test          ← universal comparison
OpLoad/Store/LoadPtr/StorePtr/Addr     ← universal memory
OpJump/JumpIf/JumpIfNot/Call/Return    ← universal control flow
OpAlloc/Free                           ← dynamic memory
OpPush/Pop                             ← stack operations
```

This core (~55 opcodes) would work unchanged for PL/M, Ada, Forth, BASIC, or any other language.

### 2.2 Type System Width

The 14 basic types cover the full spectrum needed for 8-bit systems:

| Type | Size | Use Case |
|------|------|----------|
| `u8`, `i8` | 1B | Registers, characters, small counters |
| `u16`, `i16` | 2B | Addresses, large counters, math |
| `u24`, `i24` | 3B | eZ80 24-bit addressing |
| `u32`, `i32` | 4B | Intermediate calculations |
| `f8.8`, `f.8`, `f.16`, `f16.8`, `f8.16` | 1-3B | Fixed-point math (graphics, DSP) |
| `bool` | 1B | Flags |
| `void` | 0B | No-return |

The **fixed-point types** are a standout — most IRs ignore fixed-point, forcing library solutions. MIR has native support, which is perfect for 8-bit graphics and DSP (sine tables, raymarchers, audio).

### 2.3 Compound Types

Structs, enums, arrays, pointers, function types, and bit-structs are all well-modeled:

- **StructType** with ordered fields and byte-level layout
- **EnumType** auto-sized (u8 for ≤256 variants, u16 otherwise)
- **BitStructType** for hardware register modeling (bit offsets + widths)
- **ArrayType** with static length × element size
- **PointerType** with mutability tracking (`*T` vs `*mut T`)
- **FunctionType** for function pointers (16-bit addresses)
- **GenericType** with bounds (for monomorphization)

### 2.4 Optimization Pipeline

13,114 LOC across 39 files — a serious optimization infrastructure:

| Pass | Category | Level |
|------|----------|-------|
| Constant folding | Value propagation | MIR |
| Dead code elimination | Cleanup | MIR |
| Copy propagation | Value propagation | MIR |
| Algebraic simplification | Strength reduction | MIR |
| Inlining | Function | MIR |
| Tail recursion | Loop transform | MIR |
| Iterator fusion | Domain-specific | MIR |
| Loop rerolling | Pattern recognition | MIR |
| INC/DEC optimization | Strength reduction | MIR→ASM |
| Assembly peephole (67 patterns) | Pattern matching | ASM |
| Assembly reordering | Instruction scheduling | ASM |
| SMC optimization | Domain-specific | MIR |
| TRUE SMC patching | Domain-specific | MIR→ASM |
| PGO (basic) | Profile-guided | MIR |
| Layout optimizer | Code placement | ASM |
| Calling convention heuristics | ABI | MIR |

Most of these are **target-independent** at the MIR level. Constant folding, DCE, copy propagation, inlining, and tail recursion would work for any backend.

### 2.5 MIR VM (Interpreter)

A 4,604-LOC register-based VM that can execute MIR directly. Used for:
- Compile-time evaluation (CTIE)
- Testing (75+ unit tests)
- `@minz[[[...]]]` metaprogramming

This is rare — most IRs don't have a standalone VM. Having one makes MIR debuggable and testable independently of any backend.

### 2.6 Source Position Tracking

Every instruction carries `SourceLine` and `SourceFile` metadata, propagated from AST through analysis. This enables:
- SLD generation for source-level debugging
- Error messages with precise locations
- PGO with instruction-level attribution

### 2.7 I/O Abstraction

`OpPortIn`, `OpPortOut`, `OpSyscall` — platform-abstracted I/O that works for Z80, 6502, or any I/O-mapped architecture. This is genuinely portable.

---

## 3. What's Z80-Specific

### 3.1 Physical Register Definitions (Lines 8-45)

```go
type Z80Register uint32
const (
    Z80_A Z80Register = 1 << iota
    Z80_F, Z80_B, Z80_C, Z80_D, Z80_E, Z80_H, Z80_L
    Z80_AF, Z80_BC, Z80_DE, Z80_HL, Z80_IX, Z80_IY, Z80_SP
    Z80_A_SHADOW ... Z80_HL_SHADOW  // Shadow registers
)
```

**Problem**: Physical register names baked into the IR definition. Every tool that touches `ir.go` — VM, optimizer, every backend — sees `Z80_A`, `Z80_HL`, etc.

**Impact**: A 6502 backend must work around Z80 register names. An ARM backend would need a parallel definition.

### 3.2 Register Hints (Lines 237-251)

```go
type RegisterHint uint8
const (
    RegHintA    // Prefer A register (accumulator)
    RegHintB    // Prefer B register (for DJNZ loops)
    RegHintHL   // Prefer HL register pair (for pointers)
    ...
)
```

These hints flow from the MinZ semantic layer (`iterator.go:324`) through MIR to the Z80 codegen. They encode **Z80-specific knowledge** at the IR level:
- `RegHintB` exists because Z80 has `DJNZ` (decrement B, jump if not zero)
- `RegHintHL` exists because Z80 uses HL for memory addressing
- `RegHintA` exists because Z80's accumulator is special

A 6502 backend doesn't have DJNZ. An ARM backend doesn't have HL. These hints would be noise.

### 3.3 OpDJNZ (Line 194)

```go
OpDJNZ  // Decrement and jump if not zero
```

This is a **Z80 instruction** (`DJNZ rel8` — opcode 0x10) elevated to IR level. No other architecture has this exact instruction. On 6502 you'd use `DEX/BNE` or `DEY/BNE`. On ARM, `SUBS Rn, #1 / BNE`.

The semantic analyzer (`iterator.go:482`) emits `OpDJNZ` directly from iterator chain analysis. This means the IR **requires Z80 semantics to express loops correctly**.

### 3.4 Self-Modifying Code Opcodes (10 opcodes)

```go
OpSMCLoadConst, OpSMCStoreConst, OpSMCParam, OpSMCSave,
OpSMCRestore, OpSMCUpdate, OpStoreTSMCRef, OpTrueSMCLoad,
OpTrueSMCPatch, OpTSMCRefAnchor, OpTSMCRefLoad, OpTSMCRefPatch
```

SMC is a Z80/6502 optimization paradigm — programs rewrite their own instructions at runtime (e.g., patching `LD A, 0` to `LD A, 42` by modifying the immediate byte in RAM). This is:
- **Illegal** on modern CPUs (NX bit, W^X)
- **Invalid** on ROM-based systems
- **Unsupported** on Harvard-architecture CPUs (separate instruction/data memory)

10 opcodes (~8.5% of total) are dedicated to this Z80-specific concept.

### 3.5 Instruction Patching Opcodes (4 opcodes)

```go
OpPatchPoint, OpPatchTemplate, OpPatchTarget, OpPatchParam
```

Advanced TRUE SMC support for runtime function specialization. Even more Z80-specific than basic SMC.

### 3.6 OpJumpIfFlag with CPU Flag Conditions

```go
type FlagCondition uint8
const (
    FlagCY   // Carry set (CF=1)
    FlagNC   // Carry not set (CF=0)
    FlagZ    // Zero set (ZF=1)
    FlagNZ   // Zero not set (ZF=0)
)
```

While carry/zero flags exist on most CPUs, encoding **specific flag conditions** in the IR ties it to Z80/8080/6502 flag semantics. ARM has condition codes (EQ, NE, CS, CC, MI, PL, VS, VC, HI, LS, GE, LT, GT, LE) — a superset. RISC-V has no flags at all.

### 3.7 Built-in Print Opcodes (8 opcodes)

```go
OpPrint, OpPrintU8, OpPrintU16, OpPrintI8, OpPrintI16,
OpPrintBool, OpPrintString, OpPrintStringDirect
```

These should be function calls, not IR opcodes. Every other IR (LLVM, GCC RTL, Rust MIR) models `print` as a library call. Hardcoding print at the IR level:
- Couples the IR to a specific I/O model
- Makes the opcodes meaningless on targets without a console
- Prevents customization (different print backends)

### 3.8 Error Handling via Carry Flag

```go
OpSetError    // Set carry flag and error code in A
OpCheckError  // Check carry flag for error
```

MinZ's error model uses Z80's carry flag as a boolean error indicator with error code in the A register. This is elegant for Z80 but:
- Ada requires exception handling (raise/handle)
- PL/M has structured error handling (CONDITION/ON)
- Rust uses `Result<T, E>` (algebraic types)

The carry-flag ABI is Z80-specific and insufficient for languages with richer error models.

### 3.9 Pointer/Function Size Hardcoded to 16-bit

```go
func (t *PointerType) Size() int { return 2 }     // 16-bit pointers
func (t *FunctionType) Size() int { return 2 }     // 16-bit addresses
func (t *LambdaType) Size() int { return 2 }       // 16-bit function pointer
```

eZ80 has 24-bit addresses. 6502 has 16-bit addresses but 8-bit data bus. These should be parameterized by target.

### 3.10 Function-Level Z80 Metadata

```go
type Function struct {
    UsedRegisters     RegisterSet   // Z80 registers used
    ModifiedRegisters RegisterSet   // Z80 registers modified
    CalleeSavedRegs   RegisterSet   // Z80 registers to preserve
    UsesTrueSMC       bool          // Z80 SMC feature
    SMCAnchors        map[string]*SMCAnchorInfo
    IsSMCDefault      bool          // SMC on by default!
    CPUMode           string        // "adl" or "z80"
    ...
}
```

The `Function` struct carries 15+ Z80-specific fields. Every function in MIR assumes Z80 semantics.

---

## 4. Portability Scorecard

### What Works As-Is (Portable Core)

| Component | Opcodes | Portable? |
|-----------|---------|-----------|
| Arithmetic | 8 | 100% |
| Bitwise | 6 | 100% |
| Comparison | 8 | 100% |
| Memory | 6 | 100% |
| Control flow (basic) | 7 | 100% |
| Data movement (basic) | 10 | 100% |
| Array ops | 3 | 100% |
| Stack ops | 2 | 100% |
| I/O ops | 3 | 100% |
| **Subtotal** | **53** | **100%** |

### What Needs Abstraction

| Component | Opcodes | Issue |
|-----------|---------|-------|
| Conditional jumps | 2 | FlagCondition is Z80-specific |
| Print built-ins | 8 | Should be function calls |
| Error handling | 2 | Carry-flag ABI too narrow |
| Loop ops | 1 (DJNZ) | Z80-specific instruction |
| **Subtotal** | **13** | **Abstractable** |

### What's Z80-Only

| Component | Opcodes | Issue |
|-----------|---------|-------|
| SMC ops | 10 | Paradigm doesn't exist elsewhere |
| TRUE SMC / Patching | 8 | Advanced Z80 optimization |
| Cast interface (deprecated) | 4 | Dead code |
| **Subtotal** | **22** | **Not portable** |

### Summary

```
Portable core:      53 opcodes (45%)  — works for any backend
Abstractable:       13 opcodes (11%)  — needs refactoring
Z80-only:           22 opcodes (19%)  — non-portable
Meta/low-level:     30 opcodes (25%)  — mixed (some portable, some Z80)
```

**Portability score: 4/10** — almost half the opcodes are universally useful, but the Z80 coupling is deep and structural (register types, function metadata, size assumptions).

---

## 5. Language Feasibility Assessment

### 5.1 PL/M-80 (Intel 8080/8085)

PL/M is the closest match — it was designed for Intel 8080, which Z80 is backward-compatible with.

| PL/M Feature | MIR Support | Gap |
|--------------|-------------|-----|
| BYTE (u8) | `TypeU8` | None |
| ADDRESS (u16) | `TypeU16` | None |
| Arrays | `ArrayType` + ops | None |
| Structures | `StructType` + field ops | None |
| Procedures | `Function` + `OpCall` | None |
| Typed procedures | `FunctionType` | None |
| DO/END blocks | `OpLabel` + `OpJump` | None |
| DO WHILE | `OpJumpIf` + labels | None |
| DO CASE | Multiple `OpJumpIf` | None |
| INTERRUPT | `Function.IsInterrupt` | None |
| AT (address overlay) | `OpLoadDirect`/`OpStoreDirect` | None |
| BASED variables | `PointerType` + `OpLoadPtr` | None |
| LITERALLY (macro) | N/A (preprocessor) | None (separate layer) |
| Module system | `Module` | Weak (no EXTERNAL/PUBLIC) |
| Reentrant procedures | Stack allocation | Partial (Z80 default is SMC) |

**Verdict: 9/10** — PL/M is almost a direct match. The IR has everything PL/M needs. SMC opcodes would be ignored, but the core is perfect. The main work is writing a PL/M parser (relatively simple language) and a thin backend adapter.

**Estimated effort**: 2-3 weeks for basic PL/M → MIR → Z80 pipeline.

### 5.2 Ada (GNAT-style subset for 8-bit)

Ada is a much harder target due to its rich type system and runtime requirements.

| Ada Feature | MIR Support | Gap |
|-------------|-------------|-----|
| Integer types | `TypeU8`–`TypeI32` | None |
| Range types (`type X is range 1..100`) | None | **Major** — needs runtime range checks |
| Enumeration types | `EnumType` | Partial — no character enums |
| Record types | `StructType` | None |
| Array types (bounded) | `ArrayType` | None |
| Array types (unbounded) | None | **Major** — dynamic sizing |
| Access types (pointers) | `PointerType` | Partial — no null check |
| Task types (concurrency) | None | **Major** — no tasking model |
| Exception handling | `OpSetError`/`OpCheckError` | **Major** — needs raise/propagate/handle |
| Generic packages | `GenericType` | Partial — needs instantiation |
| Operator overloading | OpCall dispatch | Semantic layer |
| Packages | `Module` | Weak — no elaboration |
| Subprograms | `Function` | Good |
| Fixed-point | `f8.8` etc. | **Excellent** — rare advantage |

**Verdict: 4/10** — Ada's range types, exceptions, and tasking are fundamental and have no MIR support. A "Safe Ada subset" (no tasking, limited exceptions, static arrays) might work, but that's not really Ada anymore. The fixed-point support is a genuine advantage — Ada is one of few languages with native fixed-point, and MIR already models it.

**Estimated effort**: 3-6 months for a meaningful Ada subset.

### 5.3 Forth

Forth is a stack machine, and MIR is a register machine — fundamental mismatch.

| Forth Feature | MIR Support | Gap |
|---------------|-------------|-----|
| Data stack | `OpPush`/`OpPop` | **Weak** — needs TOS caching |
| Return stack | Stack ops | Partial — need R>, >R |
| Arithmetic words | All arithmetic ops | Good |
| Memory (@, !) | `OpLoad`/`OpStore` | Good |
| Control flow (IF, THEN, ELSE) | Jump ops | Good |
| DO LOOP | `OpDJNZ` or labels | Works |
| Colon definitions (:, ;) | `Function` | Good |
| CREATE DOES> | None | **Major** — runtime word creation |
| IMMEDIATE words | None | **Major** — compile-time execution |

**Verdict: 5/10** — Forth's stack-based execution model requires a translation layer (stack → register mapping). Basic Forth would work; the metacircular parts (CREATE DOES>, IMMEDIATE) are hard.

### 5.4 BASIC (BBC BASIC style)

| BASIC Feature | MIR Support | Gap |
|---------------|-------------|-----|
| Numeric variables | `TypeU8`–`TypeI16` | None |
| Strings | `StringType` | Good |
| Arrays | `ArrayType` | Good |
| PRINT | `OpPrint*` | **Perfect fit** |
| Subroutines | `Function` | Good |
| Line numbers / GOTO | `OpLabel` + `OpJump` | Good |
| INPUT | None | Need I/O |
| Floating point | None | **Major** — no float support |

**Verdict: 6/10** — BASIC is a good fit except for floating-point. For integer BASIC (common on 8-bit), MIR works well. The Print opcodes, usually a weakness, are actually perfect for BASIC.

---

## 6. Refactoring Path: Portable MIR

If we wanted to make MIR a multi-language IR, here's the layered refactoring:

### Layer 1: Separate Target Definition (1-2 weeks)

```go
// Before (hardcoded):
type Z80Register uint32  // In ir.go, visible everywhere

// After (parameterized):
type TargetRegister interface {
    String() string
    Class() RegisterClass  // GP, Accumulator, Index, Pair, Stack
}

type TargetInfo struct {
    Name           string
    PointerSize    int              // 2 for Z80, 3 for eZ80, 2 for 6502
    AddressSpace   int              // 16 for Z80, 24 for eZ80
    Registers      []TargetRegister
    HasFlags       bool
    FlagConditions []string
}
```

### Layer 2: Promote Print to Calls (1 day)

```go
// Before:
OpPrintU8   // Hardcoded opcode

// After:
OpCall with Symbol="__builtin_print_u8"  // Standard function call
```

Keep the print opcodes as optional sugar in the VM, but remove them from the IR spec.

### Layer 3: Generic Loop Opcode (1 week)

```go
// Before:
OpDJNZ  // Z80-specific

// After:
OpCountedLoop  // Generic: decrement counter register, branch if != 0
// Backend decides: DJNZ on Z80, DEX/BNE on 6502, SUBS/BNE on ARM
```

### Layer 4: Separate SMC into Extension (2 days)

Move all SMC/TSMC/Patching opcodes into a `pkg/ir/smc_ext.go` extension module. The core IR doesn't reference them. Backends that support SMC import the extension.

### Layer 5: Enrich Error Model (1 week)

```go
// Add generic exception/error support:
OpThrow     // Raise error (Ada: raise, Rust: panic)
OpCatch     // Begin catch block
OpFinally   // Cleanup block
OpPropagate // Propagate error to caller

// Keep OpSetError/OpCheckError as Z80-optimized variants
```

### Total Effort: ~4-6 weeks

This would yield a MIR with:
- **Portable core** (~70 opcodes) — any language, any backend
- **SMC extension** (~18 opcodes) — Z80/6502 only
- **Target-parameterized** sizes, registers, and flags
- Existing Z80 backend works unchanged (uses full MIR + SMC extension)

---

## 7. The Practical Question

Should we refactor MIR for portability?

### Arguments For

1. **6502 backend already exists** — and struggles with Z80-specific IR
2. **i8080 backend** would benefit (same ISA family but different register semantics)
3. **Game Boy (SM83)** backend exists — similar but distinct register set
4. **Educational value** — "one IR, many targets" is compelling for teaching
5. **Future-proofing** — if someone wants to write BASIC or Forth for Z80, they can reuse MIR

### Arguments Against

1. **MinZ is the only real user** — no other language frontend exists
2. **Z80 coupling is a feature** — it enables SMC, register hints, DJNZ, and the entire optimization pipeline that makes MinZ code fast
3. **Premature generalization** — abstracting before there's a second consumer is YAGNI
4. **Backend quality varies** — Z80 is 5,822 LOC, production-grade; 6502 is 527 LOC, basic; abstracting would help 6502 but might hurt Z80
5. **4-6 weeks is significant** — better spent on register allocator fixes or new features

### Recommendation

**Don't refactor now. Extract naturally.**

The current MIR is excellent at what it does. When/if a second language frontend appears (PL/M is the obvious candidate), that's the time to extract the portable core. Until then:

1. **Document** the portable/Z80 boundary clearly (this report)
2. **Keep secondary backends working** (C, 6502, i8080) as existence proofs
3. **Fix the Z80 backend** (register allocator, enumerate/reduce) — this has more immediate value
4. If PL/M interest materializes, do Layer 1 (target definition) first — smallest change, biggest unlock

---

## 8. Comparison with Existing 8-bit IRs

| IR | Language | Target | Opcodes | SSA? | VM? | Optimization |
|----|----------|--------|---------|------|-----|-------------|
| **MinZ MIR** | MinZ | Z80/6502/i8080/GB | 118 | No | Yes (4.6K LOC) | 13+ passes (13K LOC) |
| **SDCC iCode** | C | Z80/8051/STM8 | ~80 | Partial | No | ~10 passes |
| **cc65 IR** | C | 6502 | ~60 | No | No | Basic peephole |
| **z88dk** | C | Z80 | ~50 | No | No | Basic peephole |
| **LLVM MIR** | Many | Many | ~200 | Yes | No | 100+ passes |
| **QBE IL** | C | x86/ARM | ~40 | Yes | No | ~5 passes |

MinZ MIR has **more opcodes and richer optimization than any comparable 8-bit IR**. The VM is unique. The optimization pipeline (13K LOC) rivals IRs targeting much larger platforms.

---

## 9. Conclusion

MinZ MIR is a **world-class 8-bit IR** — rich type system, comprehensive opcodes, mature optimization, standalone VM, source-level debugging support. It is deeply Z80-coupled by design, and that coupling is what makes it excellent for its purpose.

For multi-language use, the portable core exists (~53 opcodes, ~45%) and is solid. A 4-6 week refactoring could unlock PL/M and other languages. But the right time to do that is when there's a real consumer, not in advance.

The MIR's greatest achievement is proving that an IR for an 8-bit CPU can be as sophisticated as IRs for modern architectures — with optimization passes, a VM, source-level debugging, and self-modifying code support that has no equivalent anywhere else.
