# Article 079: Advanced Peephole Optimization Strategy

**Author:** Claude Code Assistant  
**Date:** August 1, 2025  
**Version:** MinZ v0.7.0+  
**Status:** ARCHITECTURAL BRAINSTORMING 🧠

## Executive Summary

Analyzing inefficient instruction patterns in MinZ codegen and proposing a two-phase optimization strategy: **Pre-peephole Reordering** followed by **Peephole Pattern Matching**.

## Current Inefficiency: The EX DE, HL Problem

### The Pattern We Generate
```asm
; SMC parameter loading
LD HL, #0000   ; param x → HL
LD DE, #0000   ; param y → DE  
EX DE, HL      ; Swap! Now HL=y, DE=x
; Later arithmetic
LD D, H        ; Save HL to DE
LD E, L
ADD HL, DE     ; y + x (works but confusing)
```

### The Optimization Target
```asm
; Optimized version
LD HL, #0000   ; param x → HL
LD DE, #0000   ; param y → DE
ADD HL, DE     ; x + y (direct, efficient)
```

**Savings:** 1 instruction (EX DE, HL), clearer semantics

## Phase 1: Pre-Peephole Reordering

### The Challenge
Peephole optimizers work on **local instruction windows** (3-5 instructions). But some patterns require **global analysis** across larger scopes.

### Pre-Reordering Strategy
Before peephole optimization, analyze and reorder instructions to create **optimization-friendly patterns**.

#### Example 1: Register Value Tracking
```asm
; Before reordering
LD HL, #0000   ; x → HL
LD DE, #0000   ; y → DE
EX DE, HL      ; HL=y, DE=x
PUSH BC        ; Unrelated instruction
LD D, H        ; DE = HL
LD E, L
ADD HL, DE     ; HL + DE

; After reordering (track register contents)
LD HL, #0000   ; x → HL  
LD DE, #0000   ; y → DE
PUSH BC        ; Move unrelated instructions
; Skip EX DE, HL since we can use registers directly
ADD HL, DE     ; x + y (direct!)
```

#### Example 2: Load-Store Elimination
```asm
; Before reordering
LD HL, value1
LD (temp), HL   ; Store to temp
LD BC, value2
LD HL, (temp)   ; Load from temp
ADD HL, BC

; After reordering
LD DE, value1   ; Keep value1 in register
LD HL, value2
ADD HL, DE      ; Direct operation
```

### Pre-Reordering Techniques

#### 1. **Register Lifetime Analysis**
Track when registers are:
- **Defined** (written to)
- **Used** (read from)  
- **Dead** (no longer needed)

```go
type RegisterLifetime struct {
    DefInstr   int    // Instruction that defines the register
    LastUse    int    // Last instruction that uses the register
    LiveRange  []int  // All instructions where register is live
}
```

#### 2. **Dependency Graph Construction**
Build data dependency graph:
```
LD HL, #0000  →  EX DE, HL  →  ADD HL, DE
LD DE, #0000  →      ↑
```

#### 3. **Safe Reordering Rules**
- ✅ Move independent instructions past dependencies
- ✅ Eliminate redundant loads/stores within same basic block
- ❌ Never reorder across branches or calls
- ❌ Never violate data dependencies

## Phase 2: Enhanced Peephole Patterns

### Current Peephole Patterns
MinZ likely has basic patterns like:
```asm
; Pattern: Redundant load elimination
LD A, n
LD A, m    ; → LD A, m (first load eliminated)

; Pattern: Dead store elimination  
LD (addr), A
LD (addr), B  ; → LD (addr), B (first store eliminated)
```

### New Advanced Patterns

#### Pattern 1: EX Elimination with ADD
```go
type PeepholePattern struct {
    Name: "EX-ADD elimination",
    Pattern: []string{
        "EX DE, HL",
        "LD D, H",
        "LD E, L", 
        "ADD HL, DE",
    },
    Replacement: []string{
        "ADD HL, DE", // Direct addition without EX
    },
    Savings: 3, // instructions eliminated
}
```

#### Pattern 2: Register Shuffle Optimization
```asm
; Pattern: Unnecessary register moves
LD HL, DE    ; HL = DE
LD DE, BC    ; DE = BC  
ADD HL, DE   ; HL + BC

; Optimized:
LD HL, DE    ; HL = DE
ADD HL, BC   ; HL + BC (direct)
```

#### Pattern 3: SMC Parameter Optimization
```asm
; Pattern: SMC parameter inefficiency
LD HL, #0000  ; param x
LD DE, #0000  ; param y
EX DE, HL     ; swap
<arithmetic>

; Optimized: Generate different initial allocation
LD DE, #0000  ; param x → DE  
LD HL, #0000  ; param y → HL
<arithmetic>   ; Use registers as allocated
```

## Implementation Strategy

### Stage 1: Pre-Peephole Reorderer
```go
// pkg/optimizer/prereorder.go
type PreReorderer struct {
    cfg         *ControlFlowGraph
    liveness    *LivenessAnalysis
    dependencies *DependencyGraph
}

func (pr *PreReorderer) Reorder(instructions []ir.Instruction) []ir.Instruction {
    // 1. Build dependency graph
    deps := pr.buildDependencies(instructions)
    
    // 2. Analyze register lifetimes
    lifetimes := pr.analyzeLiveness(instructions)
    
    // 3. Find reordering opportunities
    opportunities := pr.findReorderOpportunities(deps, lifetimes)
    
    // 4. Apply safe reorderings
    return pr.applyReorderings(instructions, opportunities)
}
```

### Stage 2: Enhanced Peephole Optimizer
```go
// pkg/optimizer/peephole_advanced.go
type AdvancedPeephole struct {
    patterns []PeepholePattern
    window   int // Look-ahead window size
}

func (ap *AdvancedPeephole) OptimizeSequence(instrs []string) []string {
    // Apply patterns in priority order
    for _, pattern := range ap.patterns {
        instrs = ap.applyPattern(instrs, pattern)
    }
    return instrs
}
```

### Stage 3: Integration with Z80 Codegen
```go
// In pkg/codegen/z80.go
func (g *Z80Generator) optimizeInstructions(instrs []string) []string {
    // Phase 1: Pre-reordering
    reorderer := NewPreReorderer()
    instrs = reorderer.Reorder(instrs)
    
    // Phase 2: Peephole optimization
    peephole := NewAdvancedPeephole()
    instrs = peephole.OptimizeSequence(instrs)
    
    return instrs
}
```

## Expected Performance Gains

### Instruction Count Reduction
- **EX DE, HL elimination**: -1 instruction per SMC function call
- **Load-store elimination**: -2-4 instructions per function
- **Register shuffle optimization**: -1-3 instructions per arithmetic sequence

### Cycle Count Reduction
- **EX DE, HL**: 4 T-states saved
- **Redundant LD**: 7-10 T-states saved per elimination
- **Direct register operations**: 3-7 T-states saved

### Estimated Overall Impact
- **Code size reduction**: 5-15% for arithmetic-heavy functions
- **Performance improvement**: 3-8% T-state reduction
- **Readability improvement**: Significantly clearer assembly output

## Advanced Optimization Opportunities

### 1. Cross-Basic-Block Optimization
Extend reordering across basic block boundaries where safe:
```asm
; Block 1
LD HL, value
JP next

; Block 2  
next:
LD DE, HL    ; Could be eliminated if HL preserved
```

### 2. Loop-Aware Optimization
Detect loop patterns and optimize register allocation:
```asm
; Loop with register pressure
loop:
    LD HL, (counter)
    DEC HL
    LD (counter), HL
    JR NZ, loop

; Optimized: Keep counter in register
    LD HL, (counter)
loop:
    DEC HL
    JR NZ, loop
    LD (counter), HL  ; Store back once
```

### 3. Function Call Optimization
Optimize register allocation across function boundaries:
```asm
; Before function call
LD HL, param1
LD DE, param2
CALL function
; After call - registers may be preserved
```

## Implementation Phases

### Phase 1: Basic Peephole Enhancement (1 week)
- Implement EX DE, HL elimination pattern
- Add register shuffle detection
- Basic load-store elimination

### Phase 2: Pre-Reordering Framework (2 weeks)
- Dependency graph construction  
- Register lifetime analysis
- Safe reordering algorithms

### Phase 3: Advanced Patterns (1 week)
- Loop optimization patterns
- Cross-block analysis
- Function call optimization

### Phase 4: Integration and Testing (3 days)
- Integrate with existing optimizer
- Comprehensive test suite
- Performance benchmarking

## Conclusion: Layered Optimization Strategy

The combination of **Pre-Peephole Reordering** + **Advanced Peephole Patterns** creates a powerful optimization pipeline:

1. **Global Analysis**: Pre-reorderer creates optimization opportunities
2. **Local Optimization**: Peephole optimizer exploits those opportunities  
3. **Synergistic Effect**: Each phase amplifies the other's effectiveness

This approach transforms MinZ from generating "correct but suboptimal" code to producing "optimal and readable" Z80 assembly.

---

*The best optimizations happen when you can see both the forest (global instruction flow) and the trees (local instruction patterns). Pre-peephole reordering gives us the forest view, while peephole optimization perfects the trees.*