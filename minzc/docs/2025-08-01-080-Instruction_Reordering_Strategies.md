# Article 080: Advanced Instruction Reordering Strategies for Optimization

**Author:** Claude Code Assistant  
**Date:** August 1, 2025  
**Version:** MinZ v0.7.0+  
**Status:** OPTIMIZATION ARCHITECTURE DESIGN 🔧

## Executive Summary

Comprehensive analysis of instruction reordering strategies across different compilation phases, from MIR-level semantic reordering to ASM-level micro-optimizations, with iterative convergence algorithms.

## The Multi-Level Reordering Pipeline

### Why Multiple Levels?

Different compilation phases offer different **visibility** and **optimization opportunities**:

```
MinZ Source → AST → MIR → Optimized MIR → ASM → Optimized ASM → Binary
                     ↑                        ↑
              High-level reordering    Low-level reordering
              (semantic analysis)      (register pressure)
```

## Level 1: MIR-Level Semantic Reordering

### Advantages of MIR Reordering
- **Rich semantic information**: Variable types, scopes, lifetimes
- **Control flow analysis**: Easy to identify basic blocks and loops
- **Alias analysis**: Understanding which memory accesses conflict
- **High-level patterns**: Loop invariants, common subexpressions

### MIR Reordering Strategies

#### Strategy 1: Data Dependency Scheduling
```go
// Before MIR reordering
r1 = load x        // Independent
r2 = load y        // Independent  
r3 = r1 + r2       // Depends on r1, r2
r4 = load z        // Independent (could be moved up!)
r5 = r3 * r4       // Depends on r3, r4

// After MIR reordering (parallelizable loads)
r1 = load x        // Load group
r2 = load y        
r4 = load z        // Moved up!
r3 = r1 + r2       // Computation group
r5 = r3 * r4
```

#### Strategy 2: Loop Invariant Code Motion
```go
// Before: Inefficient loop
for i in 0..100 {
    r1 = load base_addr    // Loop invariant!
    r2 = r1 + i
    store r2, array[i]
}

// After: Hoist invariant
r1 = load base_addr        // Moved outside loop
for i in 0..100 {
    r2 = r1 + i
    store r2, array[i]
}
```

#### Strategy 3: Common Subexpression Preparation
```go
// Before: Scattered computation
r1 = a + b
r2 = load x
r3 = a + b         // Common subexpression
r4 = load y

// After: Group common subexpressions
r1 = a + b         // Grouped
r3 = r1            // Reuse (will become move or eliminated)
r2 = load x        // Memory operations grouped
r4 = load y
```

### MIR Reordering Implementation
```go
// pkg/optimizer/mir_reorder.go
type MIRReorderer struct {
    cfg           *ControlFlowGraph
    dominators    *DominatorTree
    dependencies  *DataDependencyGraph
    aliasAnalysis *AliasAnalyzer
}

func (mr *MIRReorderer) ReorderBasicBlock(bb *BasicBlock) {
    // 1. Build dependency graph
    deps := mr.buildDependencies(bb.Instructions)
    
    // 2. Find scheduling constraints
    constraints := mr.findConstraints(deps)
    
    // 3. Apply scheduling algorithm
    scheduled := mr.scheduleInstructions(bb.Instructions, constraints)
    
    // 4. Update basic block
    bb.Instructions = scheduled
}
```

## Level 2: ASM-Level Micro-Reordering

### Why ASM-Level Reordering?
- **Register pressure visibility**: Know exactly which physical registers are used
- **Instruction timing**: Understand Z80 instruction cycle counts
- **Pipeline awareness**: Prepare for future pipeline optimizations
- **Pattern preparation**: Set up for peephole optimization

### ASM Reordering Strategies

#### Strategy 1: Register Pressure Reduction
```asm
; Before: High register pressure
LD HL, value1      ; HL busy
LD DE, value2      ; DE busy  
LD BC, value3      ; BC busy (all registers occupied!)
ADD HL, DE         ; HL free after this
LD A, (BC)         ; Could use HL instead of keeping BC

; After: Reordered to reduce pressure
LD HL, value1      ; HL busy
LD DE, value2      ; DE busy
ADD HL, DE         ; HL contains result, DE now free
LD DE, value3      ; Reuse DE instead of BC
LD A, (DE)         ; Better register utilization
```

#### Strategy 2: Instruction Fusion Preparation
```asm
; Before: Missed fusion opportunity
LD A, 5
LD B, 10
ADD A, B
LD HL, result_addr
LD (HL), A

; After: Prepared for fusion patterns
LD A, 5            ; Arithmetic group
ADD A, 10          ; Immediate fusion (if supported)
LD HL, result_addr ; Memory group  
LD (HL), A         ; Store fusion
```

#### Strategy 3: Memory Access Clustering
```asm
; Before: Scattered memory access
LD A, (addr1)      ; Memory access
LD B, 5            ; Register op
LD C, (addr2)      ; Memory access
ADD A, B           ; Register op
LD D, (addr3)      ; Memory access

; After: Clustered memory access (better cache behavior)
LD A, (addr1)      ; Memory cluster
LD C, (addr2)      
LD D, (addr3)
LD B, 5            ; Register cluster
ADD A, B
```

### ASM Reordering Implementation
```go
// pkg/optimizer/asm_reorder.go
type ASMReorderer struct {
    regTracker    *RegisterUsageTracker
    cycleAnalyzer *InstructionTimingAnalyzer
    patterns      []ReorderPattern
}

func (ar *ASMReorderer) ReorderInstructions(instrs []string) []string {
    // 1. Analyze register lifetimes
    lifetimes := ar.regTracker.AnalyzeLifetimes(instrs)
    
    // 2. Build dependency constraints
    constraints := ar.buildASMConstraints(instrs)
    
    // 3. Apply reordering patterns
    return ar.applyReorderPatterns(instrs, constraints, lifetimes)
}
```

## Iterative Reordering Process

### Why Iterative?
Reordering can **expose new opportunities**:

```asm
; Iteration 1: Move independent instruction
LD HL, value1
LD DE, value2      ; Independent, could move
ADD HL, BC         ; Depends on HL
LD A, (DE)         ; Depends on DE

; After iteration 1:
LD HL, value1
ADD HL, BC         ; Moved up
LD DE, value2      ; Now independent
LD A, (DE)

; Iteration 2: Now we can see DE/A can be optimized
; Maybe there's a pattern that can eliminate the LD DE step...
```

### Iterative Algorithm Design
```go
type IterativeReorderer struct {
    maxIterations int
    convergence   ConvergenceCriteria
    phases        []ReorderPhase
}

func (ir *IterativeReorderer) OptimizeIteratively(program *Program) {
    var prevMetrics OptimizationMetrics
    
    for iteration := 0; iteration < ir.maxIterations; iteration++ {
        // Apply all reordering phases
        for _, phase := range ir.phases {
            phase.Apply(program)
        }
        
        // Measure improvement
        currentMetrics := ir.measureProgram(program)
        
        // Check convergence
        if ir.hasConverged(prevMetrics, currentMetrics) {
            break
        }
        
        prevMetrics = currentMetrics
    }
}

type ConvergenceCriteria struct {
    MinImprovement  float64  // Stop if improvement < 1%
    StableIterations int     // Stop after N stable iterations
    MetricWeights   map[string]float64 // Instruction count, cycles, etc.
}
```

### Multi-Phase Iterative Strategy

#### Phase 1: Dependency-Driven Reordering
- **Goal**: Maximize instruction-level parallelism
- **Scope**: Within basic blocks
- **Iterations**: Usually converges in 2-3 iterations

#### Phase 2: Register Pressure Optimization  
- **Goal**: Minimize register spills and conflicts
- **Scope**: Across basic blocks in a function
- **Iterations**: May need 3-5 iterations

#### Phase 3: Pattern Preparation
- **Goal**: Arrange instructions for peephole optimization
- **Scope**: Local instruction windows
- **Iterations**: Quick convergence, 1-2 iterations

#### Phase 4: Global Code Motion
- **Goal**: Move code across larger scopes (loop hoisting, etc.)
- **Scope**: Entire function or program
- **Iterations**: Complex, may need 5-10 iterations

### Example Iterative Process
```go
// Iteration schedule
iterations := []ReorderPhase{
    // Iteration 1-3: Local optimizations
    {Type: DependencyReorder, Scope: BasicBlock, MaxIters: 3},
    {Type: RegisterPressure, Scope: BasicBlock, MaxIters: 2},
    
    // Iteration 4-6: Function-level
    {Type: RegisterPressure, Scope: Function, MaxIters: 3},
    {Type: PatternPrep, Scope: Function, MaxIters: 2},
    
    // Iteration 7-10: Global optimizations
    {Type: LoopOptimization, Scope: Function, MaxIters: 2},
    {Type: GlobalCodeMotion, Scope: Program, MaxIters: 2},
    
    // Final cleanup
    {Type: PatternPrep, Scope: BasicBlock, MaxIters: 1},
}
```

## Advanced Reordering Strategies

### Strategy 1: Speculative Reordering
**Concept**: Reorder instructions based on **probable** execution paths.

```asm
; Original: Equal probability branches
CMP A, 5
JP Z, likely_path
JP unlikely_path

likely_path:
    LD HL, common_value    ; Hot path
    ; ... more code

unlikely_path:
    LD HL, rare_value      ; Cold path

; Speculative reordering: Prefetch common case
LD HL, common_value        ; Speculatively load common case
CMP A, 5
JP Z, likely_path
LD HL, rare_value          ; Override if cold path taken
JP unlikely_path
```

### Strategy 2: Energy-Aware Reordering (Future Z80 Variants)
```asm
; Group similar operations to reduce switching activity
LD A, val1             ; Load cluster
LD B, val2
LD C, val3
ADD A, B               ; ALU cluster  
ADD A, C
LD (result), A         ; Store cluster
```

### Strategy 3: Profile-Guided Reordering
```go
type ProfileGuidedReorderer struct {
    executionCounts map[int]int64  // Instruction → execution count
    branchProbs     map[int]float64 // Branch → taken probability
}

func (pgr *ProfileGuidedReorderer) ReorderWithProfile(instrs []Instruction) {
    // Prioritize frequently executed instructions
    // Move cold code out of hot paths
    // Optimize for common execution patterns
}
```

## Safety and Correctness

### Reordering Constraints
1. **Data Dependencies**: Never violate RAW, WAR, WAW hazards
2. **Control Dependencies**: Don't move instructions across branches
3. **Memory Dependencies**: Preserve memory ordering semantics
4. **Exception Semantics**: Maintain precise exception behavior
5. **Volatile Operations**: Never reorder volatile memory access

### Verification Strategy
```go
type ReorderingVerifier struct {
    originalSemantics *SemanticModel
    reorderedSemantics *SemanticModel
}

func (rv *ReorderingVerifier) VerifyCorrectness(original, reordered []Instruction) error {
    // 1. Check data dependency preservation
    if !rv.verifyDataDeps(original, reordered) {
        return errors.New("data dependency violation")
    }
    
    // 2. Check control flow equivalence
    if !rv.verifyControlFlow(original, reordered) {
        return errors.New("control flow violation")
    }
    
    // 3. Check memory ordering
    if !rv.verifyMemoryOrdering(original, reordered) {
        return errors.New("memory ordering violation")
    }
    
    return nil
}
```

## Implementation Roadmap

### Phase 1: MIR-Level Foundation (2 weeks)
- Basic dependency analysis
- Simple scheduling within basic blocks
- Loop invariant code motion
- Verification framework

### Phase 2: ASM-Level Micro-Optimization (2 weeks)
- Register usage tracking
- Instruction pattern recognition
- Memory access clustering
- Integration with MIR reorderer

### Phase 3: Iterative Framework (1 week)
- Convergence detection
- Multi-phase coordination
- Performance metrics collection
- Regression testing

### Phase 4: Advanced Strategies (2 weeks)
- Speculative reordering
- Profile-guided optimization
- Global code motion
- Cross-function optimization

### Phase 5: Integration and Tuning (1 week)
- Integration with existing optimizer
- Performance benchmarking
- Parameter tuning
- Production hardening

## Expected Performance Impact

### Instruction Count Reduction
- **MIR-level reordering**: 3-8% reduction through better scheduling
- **ASM-level reordering**: 5-12% reduction through pattern preparation
- **Iterative refinement**: Additional 2-5% through multiple passes

### Execution Speed Improvement
- **Register pressure reduction**: 10-20% fewer memory operations
- **Instruction clustering**: 5-10% better cache behavior (future CPUs)
- **Dependency optimization**: 8-15% better instruction throughput

### Overall Impact Estimate
- **Code size**: 8-20% reduction in instruction count
- **Performance**: 15-35% improvement in execution speed
- **Compilation time**: 20-40% increase (worthwhile tradeoff)

## Conclusion: The Reordering Revolution

Instruction reordering represents a **paradigm shift** from reactive optimization to **proactive code arrangement**. By working at multiple levels with iterative refinement, we can achieve:

1. **Better code quality** through systematic optimization
2. **Exposed optimization opportunities** that single-pass optimizers miss
3. **Adaptable optimization** that improves as the compiler learns

The combination of MIR-level semantic analysis and ASM-level micro-optimization creates a powerful optimization pipeline that can rival much larger compilers.

---

*The art of instruction reordering is like conducting an orchestra - each instruction must play its part at exactly the right time to create beautiful, efficient code. The iterative approach ensures we find the perfect harmony.*