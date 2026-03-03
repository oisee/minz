# MinZ MIR Architecture Guide

A technical guide to the MinZ Intermediate Representation — what it is, how it works, how it compares to other IRs, and how to evaluate it as a compilation target for 8-bit languages.

---

## What is MIR?

MIR (MinZ Intermediate Representation) is a register-based, flat-list IR designed for compiling high-level languages to Z80 and other 8-bit processors. It sits between the AST (from parsing) and the backend (code generation):

```
MinZ source → Parser → AST → Semantic Analyzer → MIR → Optimizer → Backend → Z80 asm
```

MIR is defined in `minzc/pkg/ir/ir.go` (1,433 LOC) and consists of:
- **118 opcodes** in 13 categories
- **24 types** (14 basic + 10 compound)
- **Virtual registers** (infinite, allocated by semantic analyzer)
- **Functions** and **Modules** as top-level containers
- **Register hints** for backend optimization

---

## Core Design Decisions

### Flat Instruction List (Not SSA, Not CFG)

MIR uses a flat list of instructions per function, with explicit labels and jumps:

```
Function add(a: u8, b: u8) -> u8
  0: r1 = load_param "a"
  1: r2 = load_param "b"
  2: r3 = add r1, r2
  3: return r3
```

**Why not SSA?** SSA (Static Single Assignment) requires phi nodes at control-flow merge points. On Z80, with 7 usable 8-bit registers, the phi-resolution overhead exceeds the optimization benefit. A flat list with copy propagation achieves similar results with simpler code.

**Why not basic blocks?** The optimizer uses label-based analysis instead of explicit basic block structures. `OpLabel` instructions serve as block boundaries. This is simpler to generate from the semantic analyzer and avoids the bookkeeping of CFG construction.

### Register-Based (Not Stack-Based)

MIR uses virtual registers (`r0`, `r1`, ...), not a stack:

```
r3 = add r1, r2     ← register-based (MIR)
push r1; push r2; add; pop r3  ← stack-based (hypothetical)
```

Register-based IR maps naturally to Z80's register file and simplifies register allocation. The MIR VM also executes register IR directly without stack simulation.

### Hardware-Aware (Not Hardware-Abstracted)

MIR deliberately encodes Z80-specific knowledge:

- **OpDJNZ** — Z80's "decrement B, jump if not zero" as a first-class opcode
- **RegisterHint** — hints like `RegHintB` (use B for loops) and `RegHintHL` (use HL for pointers)
- **SMC opcodes** — 10+ opcodes for self-modifying code, unique to Z80/6502
- **Flag conditions** — `FlagCY`, `FlagNC`, `FlagZ`, `FlagNZ` for conditional jumps

This coupling is intentional: it allows the semantic analyzer to emit IR that the backend can translate efficiently without expensive pattern matching.

---

## Opcode Reference

### Control Flow (11 opcodes)

| Opcode | Operands | Description |
|--------|----------|-------------|
| `OpNop` | — | No operation |
| `OpLabel` | label | Define a branch target |
| `OpJump` | label | Unconditional jump |
| `OpJumpIf` | src1, label | Jump if src1 != 0 |
| `OpJumpIfNot` | src1, label | Jump if src1 == 0 |
| `OpJumpIfZero` | src1, label | Jump if src1 == 0 (alias) |
| `OpJumpIfNotZero` | src1, label | Jump if src1 != 0 (alias) |
| `OpJumpIfFlag` | flag, label | Jump based on CPU flag (CY/NC/Z/NZ) |
| `OpCall` | symbol, args[] | Call function |
| `OpCallIndirect` | src1 | Call through function pointer |
| `OpReturn` | src1 | Return value |

### Data Movement (16 opcodes)

| Opcode | Operands | Description |
|--------|----------|-------------|
| `OpLoadConst` | dest, imm | Load immediate value |
| `OpLoadVar` | dest, symbol | Load named variable |
| `OpStoreVar` | src1, symbol | Store to named variable |
| `OpLoadParam` | dest, symbol | Load function parameter |
| `OpLoadField` | dest, src1, imm | Load struct field at offset |
| `OpStoreField` | src1, src2, imm | Store struct field at offset |
| `OpLoadIndex` | dest, src1, src2 | Load array[dynamic index] |
| `OpStoreIndex` | src1, src2, dest | Store array[dynamic index] |
| `OpLoadElement` | dest, src1, imm | Load array[constant index] |
| `OpStoreElement` | src1, dest, imm | Store array[constant index] |
| `OpLoadBitField` | dest, src1 | Load bit field |
| `OpStoreBitField` | src1, src2 | Store bit field |
| `OpMove` | dest, src1 | Register-to-register copy |
| `OpLoadLabel` | dest, label | Load address of label |
| `OpLoadDirect` | dest, imm | Load from absolute address |
| `OpStoreDirect` | src1, imm | Store to absolute address |

### Arithmetic (8 opcodes)

| Opcode | Operands | Description |
|--------|----------|-------------|
| `OpAdd` | dest, src1, src2 | dest = src1 + src2 |
| `OpSub` | dest, src1, src2 | dest = src1 - src2 |
| `OpMul` | dest, src1, src2 | dest = src1 × src2 |
| `OpDiv` | dest, src1, src2 | dest = src1 / src2 |
| `OpMod` | dest, src1, src2 | dest = src1 % src2 |
| `OpNeg` | dest, src1 | dest = -src1 |
| `OpInc` | dest, src1 | dest = src1 + 1 |
| `OpDec` | dest, src1 | dest = src1 - 1 |

### Bitwise (6 opcodes)

| Opcode | Operands | Description |
|--------|----------|-------------|
| `OpAnd` | dest, src1, src2 | Bitwise AND |
| `OpOr` | dest, src1, src2 | Bitwise OR |
| `OpXor` | dest, src1, src2 | Bitwise XOR |
| `OpNot` | dest, src1 | Bitwise NOT |
| `OpShl` | dest, src1, src2 | Shift left |
| `OpShr` | dest, src1, src2 | Shift right |

### Comparison (8 opcodes)

| Opcode | Operands | Description |
|--------|----------|-------------|
| `OpCmp` | dest, src1, src2 | Generic compare (sets flags) |
| `OpTest` | dest, src1 | Test register (sets flags) |
| `OpEq` | dest, src1, src2 | dest = (src1 == src2) |
| `OpNe` | dest, src1, src2 | dest = (src1 != src2) |
| `OpLt` | dest, src1, src2 | dest = (src1 < src2) |
| `OpGt` | dest, src1, src2 | dest = (src1 > src2) |
| `OpLe` | dest, src1, src2 | dest = (src1 <= src2) |
| `OpGe` | dest, src1, src2 | dest = (src1 >= src2) |

### Memory (8 opcodes)

| Opcode | Operands | Description |
|--------|----------|-------------|
| `OpAlloc` | dest, imm | Allocate memory |
| `OpFree` | src1 | Free memory |
| `OpLoadPtr` | dest, src1 | Dereference pointer |
| `OpStorePtr` | src1, src2 | Store through pointer |
| `OpAddr` | dest, src1 | Address-of (&) |
| `OpLoad` | dest, src1 | Load from address |
| `OpStore` | src1, src2 | Store to address |
| `OpPush`/`OpPop` | — | Stack operations |

### Z80-Specific: SMC (14 opcodes)

Self-Modifying Code — programs rewrite their own instructions at runtime for optimization.

| Opcode | Description |
|--------|-------------|
| `OpSMCLoadConst` | Load SMC constant |
| `OpSMCStoreConst` | Store SMC constant |
| `OpSMCParam` | Define SMC parameter slot |
| `OpSMCSave/Restore` | Save/restore SMC state |
| `OpSMCUpdate` | Update SMC parameter |
| `OpTrueSMCLoad` | Load from anchor (patchable immediate) |
| `OpTrueSMCPatch` | Patch anchor at call site |
| `OpTSMCRefAnchor/Load/Patch` | TSMC reference passing |
| `OpStoreTSMCRef` | Store to TSMC reference |
| `OpPatchPoint/Template/Target/Param` | Advanced instruction patching |

### Z80-Specific: Loop (1 opcode)

| Opcode | Operands | Description |
|--------|----------|-------------|
| `OpDJNZ` | src1, label | Decrement B register, jump if not zero |

This maps directly to Z80's `DJNZ rel8` instruction (opcode 0x10). The semantic analyzer emits it for iterator loops with known bounds.

### Built-in Functions (12 opcodes)

| Opcode | Description |
|--------|-------------|
| `OpPrint` | Print u8 as character |
| `OpPrintU8/U16/I8/I16` | Print numeric values |
| `OpPrintBool` | Print "true"/"false" |
| `OpPrintString/StringDirect` | Print strings |
| `OpLoadString` | Load string literal address |
| `OpLen` | Array/string length |
| `OpMemcpy/Memset` | Memory block operations |

### I/O Abstraction (3 opcodes)

| Opcode | Description |
|--------|-------------|
| `OpPortIn` | Read from I/O port |
| `OpPortOut` | Write to I/O port |
| `OpSyscall` | Platform system call |

---

## Type System

### Basic Types (14)

```
void  — 0 bytes    bool  — 1 byte
u8    — 1 byte     i8    — 1 byte
u16   — 2 bytes    i16   — 2 bytes
u24   — 3 bytes    i24   — 3 bytes  (eZ80)
u32   — 4 bytes    i32   — 4 bytes  (intermediate calculations)
f8.8  — 2 bytes    f.8   — 1 byte   (fixed-point)
f.16  — 2 bytes    f16.8 — 3 bytes
f8.16 — 3 bytes
```

The fixed-point types are a distinctive feature. Most IRs ignore fixed-point, requiring software library solutions. MIR models them natively, enabling the optimizer to reason about fixed-point arithmetic directly.

### Compound Types (10)

```go
*T          PointerType     — 2 bytes (16-bit address)
*mut T      PointerType     — 2 bytes, mutable
[N]T        ArrayType       — N × element size
struct{}    StructType      — sum of field sizes, ordered
enum{}      EnumType        — 1 byte (≤256 variants) or 2 bytes
bits<u8>    BitStructType   — underlying type size
|p| -> r    LambdaType      — 2 bytes (function pointer)
Iterator<T> IteratorType    — 4 bytes (pointer + state)
fun() -> r  FunctionType    — 2 bytes (function pointer)
String      StringType      — 2 bytes (pointer to length-prefixed data)
LString     LStringType     — 2 bytes (pointer, u16 length prefix)
T: Bound    GenericType     — placeholder for monomorphization
```

---

## The Instruction Struct

Every MIR instruction is a single Go struct:

```go
type Instruction struct {
    // Core fields (all opcodes)
    Op           Opcode        // What operation
    Dest         Register      // Destination virtual register
    Src1         Register      // First source register
    Src2         Register      // Second source register
    Imm          int64         // Immediate value
    Label        string        // Branch target / label name
    Symbol       string        // Variable / function name
    Type         Type          // Type information
    Comment      string        // Debug comment

    // Register allocation
    Hint         RegisterHint  // Preferred physical register
    CodegenHint  *CodegenHints // Optimizer → codegen communication

    // Source tracking
    SourceLine   int           // Original .minz line number
    SourceFile   string        // Original .minz file path

    // SMC fields
    SMCLabel     string
    SMCTarget    string
    SMCAnnotations []SMCAnnotation

    // Function call fields
    Args         []Register    // Call arguments
    FuncName     string        // Call target

    // Inline assembly
    AsmCode      string        // Raw assembly for OpAsm
    ...
}
```

This is a "tagged union" approach — all fields exist on every instruction, but only relevant fields are populated per opcode. This is simple and avoids Go interface dispatch overhead, at the cost of larger instruction size (~400 bytes per instruction).

---

## Register Hints: Semantic Knowledge → Backend

Register hints are how the semantic analyzer communicates Z80-specific knowledge to the backend:

```go
type RegisterHint uint8
const (
    RegHintNone  // No preference
    RegHintA     // Use accumulator (arithmetic results)
    RegHintB     // Use B register (DJNZ counter)
    RegHintC     // Use C register
    RegHintD     // Use D register
    RegHintE     // Use E register
    RegHintH     // Use H register
    RegHintL     // Use L register
    RegHintHL    // Use HL pair (memory addressing)
    RegHintDE    // Use DE pair
    RegHintBC    // Use BC pair
)
```

### Where Hints Originate

Hints are set in the **semantic analyzer**, not the codegen:

```
iterator.go:324  →  counter register  →  RegHintB   (for DJNZ)
iterator.go:334  →  array pointer     →  RegHintHL  (for (HL) addressing)
iterator.go:347  →  current element   →  RegHintA   (for accumulator ops)
iterator.go:482  →  OpDJNZ itself     →  RegHintB   (counter must be B)
```

The semantic analyzer has **domain knowledge** about how iterators map to Z80 patterns. By encoding this as hints, it avoids hardcoding Z80 instructions while still enabling optimal code generation.

### How Hints Are Consumed

The register allocator (`register_allocator.go:170-198`) checks hints during allocation:

```go
func (ra *Z80RegisterAllocator) preferredRegFor(inst ir.Instruction) PhysicalRegister {
    switch inst.Hint {
    case ir.RegHintA:  return RegA
    case ir.RegHintB:  return RegB
    case ir.RegHintHL: return RegHL
    ...
    }
}
```

If the preferred register is available, it's used. Otherwise, the allocator falls back to its normal strategy (linear scan with spilling).

### CodegenHints: Optimizer → Backend

A richer hint mechanism for the MIR optimizer to communicate patterns to the backend:

```go
type CodegenHints struct {
    CanUseINC     bool   // Value is previous + 1
    CanUseDEC     bool   // Value is previous - 1
    CanEliminate  bool   // Value unchanged
    CanUseXOR     bool   // Value is 0 (XOR A is faster)
    IsConstant    bool   // Compile-time constant
    ConstValue    int64  // The constant value
    NextUsesFlags bool   // Next instruction reads flags
    BareDJNZ      bool   // B is safe, use bare DJNZ
}
```

These hints are computed by the MIR value tracking pass (`mir_value_tracking.go`) and consumed by the Z80 codegen to emit optimal instructions without assembly-level peephole.

---

## Optimization Pipeline

MIR is optimized at two levels:

### MIR-Level (pkg/optimizer/)

| Pass | File | LOC | Description |
|------|------|-----|-------------|
| Constant folding | `constant_folding.go` | ~200 | Evaluate constant expressions |
| Dead code elimination | `dead_code_elimination.go` | ~300 | Remove unused computations |
| Copy propagation | `mir_peephole.go` | ~680 | Track and substitute copies |
| Algebraic simplification | `mir_peephole.go` | — | x+0=x, x*1=x, x*0=0 |
| Inlining | `inlining.go` | ~400 | Inline small functions |
| Iterator fusion | `fusion.go` | ~500 | Fuse map+filter+forEach into single loop |
| Tail recursion | `tail_recursion.go` | ~300 | Convert tail calls to jumps |
| Loop rerolling | `loop_reroll.go` | ~400 | Unrolled sequences → loops |
| Value tracking | `mir_value_tracking.go` | ~300 | Compute codegen hints |
| MIR combined | `mir_combined.go` | ~200 | Fixed-point iteration (max 10 rounds) |

### Assembly-Level (pkg/optimizer/)

| Pass | File | LOC | Description |
|------|------|-----|-------------|
| Assembly peephole | `assembly_peephole.go` | ~1,371 | 67 regex patterns |
| Assembly reordering | `assembly_reordering.go` | ~400 | Instruction scheduling |
| SMC optimization | `smc_optimization.go` | ~300 | SMC-specific patterns |
| TRUE SMC | `true_smc.go` | ~200 | Anchor optimization |
| Superopt peephole | `superopt_peephole.go` | ~400 | Exhaustive search |
| INC/DEC optimizer | `inc_dec_optimizer.go` | ~200 | LD A,n → INC/DEC |
| Layout optimizer | `layout_optimizer.go` | ~200 | Hot path placement |

---

## Module and Function Model

### Module

```go
type Module struct {
    Name      string
    Functions []*Function
    Globals   []Global
    Strings   []*String
    PatchTable []PatchEntry  // TRUE SMC
}
```

A module is a compilation unit — one `.minz` file produces one module. Modules contain functions, global variables, string literals, and an SMC patch table.

### Function

```go
type Function struct {
    Name         string
    Params       []Parameter
    ReturnType   Type
    Locals       []Local
    Instructions []Instruction
    NextReg      Register

    // Z80-specific metadata
    IsSMCEnabled      bool
    UsesTrueSMC       bool
    UsedRegisters     RegisterSet
    ModifiedRegisters RegisterSet
    CalleeSavedRegs   RegisterSet
    CPUMode           string  // "adl" or "z80"
    IsAsm             bool    // Body is raw assembly
    IsExtern          bool    // @extern (no body)
    ExternAddress     uint16  // @extern(0xC000)
    ...
}
```

Functions carry both portable metadata (params, return type, locals) and Z80-specific metadata (register sets, SMC info, CPU mode).

---

## Backend Interface

All backends implement a minimal interface:

```go
type Backend interface {
    Generate(module *ir.Module) (string, error)
}
```

Existing backends:

| Backend | File | LOC | Status |
|---------|------|-----|--------|
| Z80 | `z80.go` | 5,822 | Production |
| C | `c.go` | 721 | Testing |
| 6502 | `m6502_gen.go` | 527 | Basic |
| i8080 | `i8080.go` | 663 | Basic |
| Game Boy (SM83) | `gb.go` | 330 | Experimental |
| Crystal | `crystal.go` | 388 | Experimental |
| LLVM | `llvm_backend.go` | 493 | Proof-of-concept |

The Z80 backend is 8x larger than any other, reflecting its production maturity.

---

## MIR VM

The MIR VM (`pkg/mirvm/`, 4,604 LOC) is a register-based interpreter that executes MIR directly:

```go
type VM struct {
    registers []int64     // Virtual register file (256 max)
    memory    []int64     // Flat memory
    stack     []int64     // Call stack
    module    *ir.Module  // Loaded module
    ...
}
```

Used for:
- **CTIE (Compile-Time Interface Execution)** — trait monomorphization
- **@minz[[[...]]]** — compile-time metaprogramming
- **Testing** — 75+ unit tests verify MIR semantics independent of any backend

---

## Evaluating MIR as a Multi-Language Target

### What's Portable (works for any 8-bit language)

- Core arithmetic, bitwise, comparison: 22 opcodes
- Memory load/store/pointer: 8 opcodes
- Control flow: 7 opcodes (excluding flag-specific jumps)
- Data movement: 10 opcodes
- Array/struct access: 7 opcodes
- I/O abstraction: 3 opcodes
- **Total: ~57 portable opcodes**

### What's Z80-Specific (ignore or abstract for other targets)

- SMC/TSMC/Patching: 18 opcodes — Z80/6502 optimization, no equivalent on modern CPUs
- OpDJNZ: 1 opcode — Z80 instruction, translate to generic counted loop
- Flag conditions: 1 opcode — Z80 flag semantics, abstract to target flags
- Print built-ins: 8 opcodes — should be function calls
- Register hints: enum values — Z80 register names, remap per target
- **Total: ~28 Z80-specific items**

### Language Compatibility

| Language | Fit | Key Issue |
|----------|-----|-----------|
| **PL/M-80** | 9/10 | Near-perfect match (same target, similar types) |
| **BASIC (integer)** | 7/10 | Good fit; print opcodes actually useful |
| **Forth** | 5/10 | Stack→register translation needed |
| **Ada subset** | 4/10 | Missing: exceptions, range types, tasking |
| **Pascal** | 6/10 | Good fit for subset; needs string/set support |
| **C (SDCC-like)** | 7/10 | Good; MIR has all the ops, C backend proves it |

### Refactoring for Portability

To use MIR for other languages, the minimum changes are:

1. **Move Z80Register to a target package** — don't import physical regs in core IR
2. **Parameterize pointer/function size** — 2 bytes for Z80, 3 for eZ80, variable
3. **Make OpDJNZ generic** — `OpCountedLoop` with backend lowering
4. **Promote print to function calls** — `OpCall("__print_u8", r1)` instead of `OpPrintU8`
5. **Separate SMC extension** — optional module, not in core opcode enum

Estimated effort: 4-6 weeks. See the [MIR Analysis Report](../reports/2026-03-02-020-MIR_Analysis_Multi_Language_IR_Feasibility.md) for the detailed refactoring plan.

---

## Key Files Reference

| File | LOC | Role |
|------|-----|------|
| `pkg/ir/ir.go` | 1,433 | IR definition (opcodes, types, instruction struct) |
| `pkg/semantic/analyzer.go` | 12,353 | AST → MIR translation |
| `pkg/semantic/iterator.go` | 1,118 | Iterator chain → MIR (DJNZ patterns) |
| `pkg/codegen/z80.go` | 5,822 | MIR → Z80 assembly |
| `pkg/codegen/register_allocator.go` | 491 | Physical register allocation |
| `pkg/optimizer/` | 13,114 | 39 optimization pass files |
| `pkg/mirvm/` | 4,604 | MIR interpreter/VM |
| `pkg/codegen/backend.go` | 111 | Backend interface |

---

## Further Reading

- [MIR vs LLVM IR Comparison](MIR_vs_LLVM_IR_Comparison.md) — detailed three-way comparison with LLVM IR and Rust MIR
- [MIR Visualization Guide](MIR_VISUALIZATION_GUIDE.md) — Graphviz output for MIR
- [Internal Architecture](../minzc/docs/INTERNAL_ARCHITECTURE.md) — full compiler internals
- [MIR Analysis Report](../reports/2026-03-02-020-MIR_Analysis_Multi_Language_IR_Feasibility.md) — multi-language feasibility analysis
