# MIR-Level Optimizations: Research & Brainstorm

**Status**: Research/Design
**Date**: 2025-12-22
**Author**: Claude + Alice

## Executive Summary

This document explores optimization opportunities at the MIR (Mid-level IR) level and designs a hint system to pass optimization metadata to lower levels (code generation, assembly peephole).

**Key Insight**: Many optimizations we do at assembly level could be done more effectively at MIR level, where we have:
- Structured control flow (basic blocks)
- Type information
- Virtual registers (SSA-like)
- No target-specific constraints yet

## Current Architecture

```
Source Code (.minz)
       │
       ▼
    Parser
       │
       ▼
      AST
       │
       ▼
   IR Builder ──────────► MIR (pkg/ir/ir.go)
       │                    │
       ▼                    │
  MIR Optimizer ◄──────────┘
  (pkg/optimizer/)
       │
       ├── RegisterAnalysisPass
       ├── ConstantFoldingPass
       ├── DeadCodeEliminationPass
       ├── SmartPeepholePass
       ├── RegisterAllocationPass
       ├── InliningPass
       ├── TrueSMCPass
       └── TailRecursionPass
       │
       ▼
   Code Generator (pkg/codegen/z80.go)
       │
       ▼
   Assembly (.a80)
       │
       ▼
   Assembly Peephole (pkg/optimizer/assembly_peephole.go)
       │
       ▼
   Final Assembly
```

### Current MIR Structure

```go
type Instruction struct {
    Op           Opcode      // Operation (OpLoadConst, OpAdd, etc.)
    Dest         Register    // Destination virtual register
    Src1         Register    // Source register 1
    Src2         Register    // Source register 2
    Imm          int64       // Immediate value
    Type         Type        // Type information
    Symbol       string      // Variable/function name
    Hint         RegisterHint // Hint for register allocator
    // ... more fields
}
```

### Existing MIR Optimizations

| Pass | What It Does |
|------|--------------|
| ConstantFolding | Evaluate `3 + 5` → `8` at compile time |
| AlgebraicSimplification | `x + 0` → `x`, `x * 1` → `x` |
| StrengthReduction | `x * 2` → `x << 1`, `x / 4` → `x >> 2` |
| CopyPropagation | `a = b; c = a` → `c = b` |
| DeadCodeElimination | Remove unused computations |
| RedundantLoadElimination | `load x; load x` → `load x; move` |

## New MIR-Level Optimization Opportunities

### 1. Value Range Analysis

Track possible value ranges for each register:

```
r1 = 0          ; r1 ∈ [0, 0]
r1 = r1 + 1     ; r1 ∈ [1, 1]
if r1 < 10:     ; r1 ∈ [1, 9] in true branch
    r2 = r1 + 1 ; r2 ∈ [2, 10]
```

**Benefits**:
- Eliminate bounds checks when array index is proven in-range
- Optimize comparisons (`if x < 256` always true for u8)
- Enable more constant folding

### 2. Constant Value Tracking with Delta Hints

Track not just "is constant" but "what is the constant" and "how does it relate to previous value":

```go
type ValueInfo struct {
    IsConstant   bool
    Value        int64       // If constant
    PrevValue    int64       // Previous constant (if known)
    Delta        int         // Difference from previous (+1, -1, 0)
    DeltaHint    DeltaHint   // Hint for codegen
}

type DeltaHint uint8
const (
    DeltaNone DeltaHint = iota
    DeltaPlus1          // Can use INC
    DeltaMinus1         // Can use DEC
    DeltaSame           // Can eliminate
)
```

**MIR Level**:
```
r1 = const 0xFE    ; Track: r1 = 0xFE
r1 = const 0xFF    ; Detect: delta = +1, add hint DeltaPlus1
```

**Code Generation sees**:
```
Instruction{Op: OpLoadConst, Dest: r1, Imm: 0xFF, Hint: DeltaPlus1}
```

**Emits**:
```asm
INC B        ; Instead of LD B, $FF
```

### 3. Register Lifetime Overlap Analysis

At MIR level, analyze which virtual registers can share physical registers:

```
r1 = ...        ; r1 live: [1, 5]
r2 = ...        ; r2 live: [2, 3]
use r2          ;
r3 = ...        ; r3 live: [4, 7]  -- can reuse r2's physical reg!
use r1
use r3
```

**Hint to codegen**: "r2 and r3 can share same physical register"

### 4. Loop-Invariant Code Motion (LICM)

Move computations out of loops:

```
; Before
loop:
    r1 = load global_x    ; Invariant - same every iteration
    r2 = r2 + r1
    djnz loop

; After
    r1 = load global_x    ; Hoisted
loop:
    r2 = r2 + r1
    djnz loop
```

### 5. Common Subexpression Elimination (CSE)

```
; Before
r1 = a + b
r2 = c * d
r3 = a + b      ; Same as r1!

; After
r1 = a + b
r2 = c * d
r3 = r1         ; Reuse
```

### 6. Load/Store Forwarding

```
; Before
store x, r1
r2 = load x     ; We just stored r1 here!

; After
store x, r1
r2 = r1         ; Forward the value
```

### 7. Branch Prediction Hints

```go
type Instruction struct {
    // ...
    BranchHint  BranchHint
}

type BranchHint uint8
const (
    BranchUnknown BranchHint = iota
    BranchLikely            // Probably taken
    BranchUnlikely          // Probably not taken
)
```

**Usage in codegen**: Layout blocks to minimize jumps for likely paths.

### 8. Zero-Value Tracking

Special case for zero (most common constant):

```go
type Instruction struct {
    // ...
    IsZero      bool  // Result is known to be zero
    CanUseXorA  bool  // This zero can be produced via XOR A
}
```

**Propagation**:
```
r1 = const 0      ; IsZero = true
r2 = r1 + r3      ; If r3 is also zero, result IsZero
r4 = r1 * r5      ; r1 is zero, so result IsZero
```

### 9. Increment/Decrement Chain Detection

Detect patterns like loop counters:

```
r1 = 0
loop:
    r1 = r1 + 1    ; INC pattern
    if r1 < 10: goto loop
```

**Hint**: "r1 is a counter, always incrementing"

### 10. Memory Access Pattern Analysis

```go
type MemoryAccessHint uint8
const (
    MemAccessRandom MemoryAccessHint = iota
    MemAccessSequential     // Array traversal
    MemAccessStrided        // Every Nth element
    MemAccessConstant       // Same address repeatedly
)
```

**For Z80**: Sequential access can use `LD (HL), A / INC HL` patterns.

## Hint System Design

### Instruction-Level Hints

```go
type Instruction struct {
    Op           Opcode
    Dest         Register
    Src1         Register
    Src2         Register
    Imm          int64

    // === NEW: Optimization Hints ===
    Hints        InstructionHints
}

type InstructionHints struct {
    // Value hints
    IsConstant     bool
    ConstValue     int64
    ValueDelta     int8      // -1, 0, +1 from previous

    // Register hints
    PreferReg      Z80Register  // Preferred physical register
    AvoidReg       Z80Register  // Registers to avoid
    ShareWith      Register     // Can share physical reg with

    // Code generation hints
    UseINC         bool         // Use INC instead of LD
    UseDEC         bool         // Use DEC instead of LD
    UseXOR         bool         // Use XOR A for zero
    Eliminate      bool         // Can be eliminated entirely

    // Control flow hints
    BranchLikely   bool
    LoopHeader     bool
    LoopExit       bool

    // SMC hints
    IsSMCTarget    bool         // Don't optimize - will be patched

    // Flags hints
    FlagsNeeded    bool         // Next instruction uses flags
    FlagsSet       Z80Flags     // Which flags this sets
}
```

### Function-Level Hints

```go
type Function struct {
    // ... existing fields ...

    // === NEW: Optimization Metadata ===
    OptHints       FunctionHints
}

type FunctionHints struct {
    IsPure          bool     // No side effects
    IsLeaf          bool     // Doesn't call other functions
    MaxStackDepth   int      // Maximum stack usage
    HotPath         []int    // Indices of hot basic blocks
    ColdPath        []int    // Indices of cold basic blocks

    // Register usage summary
    NeedsA          bool     // Uses accumulator heavily
    NeedsHL         bool     // Uses HL for pointers
    NeedsBC         bool     // Uses BC for counters

    // Inlining hints
    InlineCandidate bool
    InlineCost      int      // Estimated cost if inlined
}
```

### Basic Block Hints

```go
type BasicBlock struct {
    ID           int
    Instructions []int        // Indices into function instructions
    Predecessors []int        // Block IDs
    Successors   []int        // Block IDs

    // Hints
    IsLoopHeader bool
    IsLoopExit   bool
    Frequency    float64      // Estimated execution frequency
    Alignment    int          // Preferred alignment (for hot loops)
}
```

## Implementation Phases

### Phase 1: Value Tracking Infrastructure

Add value tracking to MIR:

```go
// In mir_peephole.go
type ValueTracker struct {
    values map[ir.Register]*ValueInfo
}

type ValueInfo struct {
    IsConstant bool
    Value      int64
    Source     ir.Register  // If copy, source register
    Delta      int8         // Change from previous value
}
```

**Detect delta patterns**:
```go
func (t *ValueTracker) trackLoadConst(inst *ir.Instruction) {
    prev, hasPrev := t.values[inst.Dest]

    info := &ValueInfo{
        IsConstant: true,
        Value:      inst.Imm,
    }

    if hasPrev && prev.IsConstant {
        delta := inst.Imm - prev.Value
        if delta == 1 {
            info.Delta = 1
            inst.Hints.UseINC = true
        } else if delta == -1 {
            info.Delta = -1
            inst.Hints.UseDEC = true
        } else if delta == 0 {
            inst.Hints.Eliminate = true
        }
    }

    t.values[inst.Dest] = info
}
```

### Phase 2: Pass Hints to Codegen

Modify code generator to check hints:

```go
// In z80.go
func (g *Z80Generator) generateLoadConst(inst *ir.Instruction) {
    if inst.Hints.Eliminate {
        // Skip - value already in register
        return
    }

    if inst.Hints.UseINC {
        g.emit("INC %s", g.regName(inst.Dest))
        return
    }

    if inst.Hints.UseDEC {
        g.emit("DEC %s", g.regName(inst.Dest))
        return
    }

    // Normal load
    g.emit("LD %s, $%02X", g.regName(inst.Dest), inst.Imm)
}
```

### Phase 3: Control Flow Hints

Add basic block analysis:

```go
func (p *MIRPeepholePass) analyzeControlFlow(fn *ir.Function) {
    // Build basic blocks
    blocks := p.buildBasicBlocks(fn)

    // Detect loops
    for _, block := range blocks {
        if p.isLoopHeader(block, blocks) {
            block.IsLoopHeader = true
            // Mark instructions in loop
            for _, instIdx := range block.Instructions {
                fn.Instructions[instIdx].Hints.InLoop = true
            }
        }
    }
}
```

### Phase 4: Cross-Function Analysis

```go
func (p *MIRPeepholePass) analyzeModule(module *ir.Module) {
    // Build call graph
    callGraph := p.buildCallGraph(module)

    // Identify leaf functions
    for _, fn := range module.Functions {
        if len(callGraph[fn.Name]) == 0 {
            fn.OptHints.IsLeaf = true
        }
    }

    // Propagate hints
    for _, fn := range module.Functions {
        if fn.OptHints.IsLeaf {
            // Leaf functions don't need to save as many registers
            fn.CalleeSavedRegs = 0
        }
    }
}
```

## Comparison with LLVM/GCC

### LLVM Approach

LLVM uses multiple IR levels:
1. **LLVM IR**: High-level, SSA form
2. **SelectionDAG**: Target-specific patterns
3. **MachineIR**: Near-final instructions

Each level has progressively more target-specific information.

**Applicable to MinZ**:
- We could add a "LowMIR" level between MIR and assembly
- LowMIR would have physical registers assigned but still structured

### GCC Approach

GCC uses:
1. **GIMPLE**: High-level tree representation
2. **RTL**: Register Transfer Language (low-level)

RTL is where most optimizations happen.

**Applicable to MinZ**:
- Our MIR is similar to GIMPLE
- We could add RTL-like representation for fine-grained optimization

## Z80-Specific Opportunities

### 1. Accumulator Routing

Z80 arithmetic heavily uses A register. Track when values flow through A:

```
r1 = r2 + r3    ; Must go through A: LD A,r2 / ADD A,r3 / LD r1,A
```

**Optimization**: If r1 will be used in A next, skip the final LD.

### 2. Register Pair Optimization

HL, DE, BC are pairs. Track 16-bit values:

```
r1 = const 0x1234    ; Can load to HL directly: LD HL, $1234
r2 = load_16bit ptr  ; Can use LD HL, (addr)
```

### 3. DJNZ Pattern Detection

```
r1 = N
loop:
    // ... body ...
    r1 = r1 - 1
    if r1 != 0: goto loop
```

**Hint**: "This is a DJNZ loop, use B register for r1"

### 4. Block Operations

Detect memcpy/memset patterns:

```
loop:
    r1 = load src
    store dst, r1
    src = src + 1
    dst = dst + 1
    count = count - 1
    if count != 0: goto loop
```

**Emit**: `LDIR` instruction (single Z80 instruction for block copy)

## Metrics & Validation

### Measure Optimization Impact

```go
type OptimizationStats struct {
    ConstantsFolded      int
    InstructionsRemoved  int
    INCsGenerated        int  // Instead of LD r, imm
    DECsGenerated        int
    BytesSaved           int
    CyclesSaved          int
}
```

### Test Cases

1. **Consecutive constants**: `a=1; a=2; a=3; a=4` → INC chain
2. **Zero patterns**: `a=0; b=0; c=0` → XOR A + LD chain
3. **Loop counters**: `for i in 0..10` → DJNZ with B
4. **Common subexpressions**: `x+y` computed twice → reuse

## Future Work

1. **Profile-Guided Optimization (PGO)**: Use runtime profiles for hot/cold hints
2. **Interprocedural Analysis**: Track values across function calls
3. **Alias Analysis**: Determine when pointers don't alias for better optimization
4. **Vectorization**: Unroll small loops for Z80
5. **Peephole at MIR**: More patterns at structured level

## Conclusion

Moving optimizations from assembly level to MIR level provides:

1. **Better analysis**: Structured control flow, type info
2. **Target independence**: Same optimization for Z80, 6502, WASM
3. **Composability**: Hints flow through pipeline
4. **Correctness**: Easier to prove optimizations are safe

The hint system allows MIR-level analysis to inform code generation without duplicating work at the assembly level.

## References

- LLVM Language Reference Manual
- GCC Internals
- Engineering a Compiler (Cooper & Torczon)
- ADR-294: Consecutive LD Zero Optimization
- ADR-295: Register Value Propagation
