# Multi-Level Optimization Design

## Overview

MinZ implements optimization at three distinct levels, each with multiple passes until fixpoint. This design maximizes optimization opportunities while avoiding infinite loops.

## Three-Level Architecture

```
Source Code
    ↓
[SEMANTIC LEVEL]
    ├─ Constant Folding
    ├─ Pattern Transformation  
    ├─ Lambda Lifting
    └─ Interface Resolution
    ↓
[MIR LEVEL]
    ├─ Instruction Reordering
    ├─ Register Pressure Reduction
    ├─ Dead Code Elimination
    └─ Peephole Patterns
    ↓
[ASM LEVEL]
    ├─ Instruction Reordering (ASM-specific)
    ├─ Peephole Patterns (Z80-aware)
    ├─ Z80-Specific Patterns
    ├─ Instruction Selection
    ├─ Register Allocation
    └─ Final Peephole
    ↓
Optimized Z80 Assembly
```

## Level 1: Semantic Optimization

### Purpose
Transform high-level constructs into more efficient forms before IR generation.

### Passes

#### 1.1 Constant Expression Evaluation
```minz
// Before
let x = 2 + 3 * 4;
let y = x + 1;

// After
let x = 14;
let y = 15;
```

#### 1.2 Pattern-Based Transformation
```minz
// Before
for i in 0..256 {
    array[i] = 0;
}

// After
@memset(array, 0, 256);  // Intrinsic function
```

#### 1.3 Compile-Time Function Evaluation
```minz
// Before
let size = calculate_size(10, 20);

// After (if calculate_size is pure)
let size = 200;
```

#### 1.4 Lambda Lifting and Specialization
```minz
// Before
let add5 = |x| { x + 5 };
let result = add5(10);

// After
fun add5_lifted(x: u8) -> u8 { x + 5 }
let result = 15;  // Inlined and folded!
```

### Semantic Optimizer Implementation

```minz
@optimizer("semantic")
class SemanticOptimizer {
    passes: [SemanticPass] = [
        ConstantFoldingPass(),
        PatternMatchingPass(),
        LambdaLiftingPass(),
        InterfaceResolutionPass(),
        CompileTimeEvaluationPass(),
    ];
    
    fun optimize(ast: AST) -> AST {
        let max_iterations = 10;
        let mut changed_total = false;
        
        for iteration in 0..max_iterations {
            let mut changed_this_iteration = false;
            
            for pass in self.passes {
                let (new_ast, changed) = pass.apply(ast);
                if changed {
                    ast = new_ast;
                    changed_this_iteration = true;
                    changed_total = true;
                }
            }
            
            if !changed_this_iteration {
                break;  // Fixpoint reached
            }
        }
        
        return ast;
    }
}
```

## Level 2: MIR Optimization

### Purpose
Optimize the machine-independent representation for better code generation.

### Passes

#### 2.1 Instruction Reordering
```mir
// Before
r1 = load x
r2 = add r1, 1
store x, r2
r3 = load y
r4 = add r3, 1
store y, r4

// After (loads clustered)
r1 = load x
r3 = load y
r2 = add r1, 1
r4 = add r3, 1
store x, r2
store y, r4
```

#### 2.2 Common Subexpression Elimination
```mir
// Before
r1 = mul a, b
r2 = add r1, c
r3 = mul a, b  // Same expression!
r4 = sub r3, d

// After
r1 = mul a, b
r2 = add r1, c
r4 = sub r1, d  // Reuse r1!
```

#### 2.3 Register Pressure Reduction
```mir
// Before (high pressure)
r1 = load a
r2 = load b
r3 = load c
r4 = load d
r5 = add r1, r2
r6 = add r3, r4
r7 = mul r5, r6

// After (reduced pressure)
r1 = load a
r2 = load b
r3 = add r1, r2  // Reuse r3
r1 = load c      // Reuse r1
r2 = load d      // Reuse r2
r4 = add r1, r2
r5 = mul r3, r4
```

### MIR Pattern Rules

```lua
-- MIR optimization patterns
mir_patterns = {
    -- Strength reduction
    {
        pattern = "mul %r, 2",
        replace = "shl %r, 1"
    },
    
    -- Algebraic simplification
    {
        pattern = "add %r, 0",
        replace = ""  -- Delete
    },
    
    -- Memory access optimization
    {
        pattern = "store %addr, %r1; load %r2, %addr",
        replace = "store %addr, %r1; move %r2, %r1"
    }
}
```

## Level 3: ASM Optimization

### Purpose
Apply final optimizations at the assembly level with full knowledge of Z80 instruction timings and register constraints.

### Passes

#### 3.1 ASM Instruction Reordering
```asm
; Before (poor scheduling)
LD A, (var1)
LD HL, array
ADD A, 5
LD (HL), A
LD B, (var2)

; After (better scheduling)
LD HL, array      ; Setup pointer early
LD A, (var1)      ; Load first
LD B, (var2)      ; Load second (parallel)
ADD A, 5          ; Compute
LD (HL), A        ; Store when ready
```

#### 3.2 Z80-Aware Peephole Patterns
```asm
; Before
LD A, B
LD C, A
LD A, C

; After
LD A, B
LD C, B    ; Skip intermediate copy
```

#### 3.3 Z80 Instruction Selection
```asm
; Before
LD A, B
ADD A, 1
LD B, A

; After
INC B  ; Single instruction!
```

#### 3.2 Register Allocation Optimization
```asm
; Before (using memory)
LD A, (temp1)
ADD A, (temp2)
LD (result), A

; After (using registers)
LD A, B
ADD A, C
LD D, A
```

#### 3.3 Flag Optimization
```asm
; Before
CP 0
JP Z, label

; After
OR A       ; Faster, same effect
JP Z, label
```

#### 3.4 Addressing Mode Selection
```asm
; Before
LD HL, array
LD A, (HL)
INC HL
LD B, (HL)

; After
LD IX, array
LD A, (IX+0)
LD B, (IX+1)  ; If accessed multiple times
```

## Cross-Level Optimization Opportunities

### Example 1: Loop Unrolling Cascade

```minz
// Semantic level detects unrollable loop
for i in 0..4 {
    sum += array[i];
}

// MIR level sees sequential adds
r1 = load array[0]
r2 = add sum, r1
r3 = load array[1]
r4 = add r2, r3
...

// ASM level uses 16-bit ops
LD HL, (array)
LD DE, (array+2)
ADD HL, DE
```

### Example 2: Constant Propagation Chain

```minz
// Semantic: Evaluate constant
const MASK = 0xFF;
let result = value & MASK;

// MIR: Recognize redundant op
r1 = and value, 0xFF  // No-op for u8!

// ASM: Eliminate instruction
; Nothing generated!
```

## Termination Strategies

### 1. Cost Model
Each optimization must reduce cost:
```
cost = cycles * weight_cycles + 
       size * weight_size + 
       registers * weight_registers
```

### 2. Pattern History
```rust
struct OptimizationHistory {
    applied_patterns: HashSet<(PassID, Location)>,
    
    fn can_apply(&self, pass: PassID, loc: Location) -> bool {
        !self.applied_patterns.contains(&(pass, loc))
    }
}
```

### 3. Monotonic Progress
```rust
enum Progress {
    Improved { metric: i32 },
    Neutral,
    Degraded,
}

fn should_apply(before: Metric, after: Metric) -> bool {
    match compare(before, after) {
        Progress::Improved(_) => true,
        Progress::Neutral => false,  // Avoid oscillation
        Progress::Degraded => false,
    }
}
```

## Implementation Architecture

```rust
trait OptimizationPass {
    type IR;  // AST, MIR, or ASM
    
    fn name(&self) -> &str;
    fn apply(&self, ir: Self::IR) -> (Self::IR, bool);
    fn cost(&self, ir: &Self::IR) -> Cost;
}

struct MultiLevelOptimizer {
    semantic_passes: Vec<Box<dyn OptimizationPass<IR=AST>>>,
    mir_passes: Vec<Box<dyn OptimizationPass<IR=MIR>>>,
    asm_passes: Vec<Box<dyn OptimizationPass<IR=ASM>>>,
    
    fn optimize(&self, ast: AST) -> ASM {
        // Phase 1: Semantic
        let ast = self.run_until_fixpoint(ast, &self.semantic_passes);
        
        // Phase 2: Generate MIR
        let mir = generate_mir(ast);
        let mir = self.run_until_fixpoint(mir, &self.mir_passes);
        
        // Phase 3: Generate ASM
        let asm = generate_asm(mir);
        let asm = self.run_until_fixpoint(asm, &self.asm_passes);
        
        asm
    }
    
    fn run_until_fixpoint<T>(&self, mut ir: T, passes: &[Box<dyn OptimizationPass<IR=T>>]) -> T {
        let mut iteration = 0;
        let max_iterations = 20;
        
        loop {
            let mut changed = false;
            let initial_cost = self.calculate_cost(&ir);
            
            for pass in passes {
                let (new_ir, pass_changed) = pass.apply(ir.clone());
                
                if pass_changed {
                    let new_cost = self.calculate_cost(&new_ir);
                    
                    // Only apply if it improves or maintains cost
                    if new_cost <= initial_cost {
                        ir = new_ir;
                        changed = true;
                    }
                }
            }
            
            iteration += 1;
            
            if !changed || iteration >= max_iterations {
                break;
            }
        }
        
        ir
    }
}
```

## Benefits

1. **Maximum Optimization**: Each level can expose opportunities for others
2. **Clean Separation**: Each level handles appropriate concerns
3. **Extensibility**: Easy to add new passes at any level
4. **Debugging**: Can disable levels independently
5. **Predictability**: Fixpoint guarantees termination

## Example: Complete Optimization Flow

```minz
// Original code
fun calculate(x: u8) -> u8 {
    let a = 2 * x;
    let b = a + 4;
    let c = b / 2;
    return c + 1;
}

// After semantic optimization
fun calculate(x: u8) -> u8 {
    return x + 3;  // Algebraically simplified!
}

// After MIR optimization
calculate:
    r1 = load_param 0
    r2 = add r1, 3
    ret r2

// After ASM optimization
calculate:
    LD A, (param_x)
    ADD A, 3
    RET
```

The multi-level approach found that `(2*x + 4)/2 + 1 = x + 3`, something that would be very difficult to discover at a single level!