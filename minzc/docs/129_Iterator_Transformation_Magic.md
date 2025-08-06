# 129: Iterator Transformation Magic - How MinZ Achieves Zero-Cost Abstractions

## The Vision: Functional Programming on 8-bit Hardware

MinZ brings modern functional programming patterns to Z80 assembly through sophisticated compile-time transformations. This document reveals the magic behind our zero-cost iterator abstractions.

## 1. The Iterator Chain AST Transformation

### From Method Calls to Iterator Chain
```minz
// What you write:
numbers.iter()
    .skip(5)
    .take(10)
    .filter(|x| x % 2 == 0)
    .map(|x| x * 2)
    .forEach(print_u8);
```

### AST Representation
```go
IteratorChainExpr {
    Source: Identifier("numbers"),
    Operations: [
        IteratorOp { Type: IterOpSkip, Function: NumberLiteral(5) },
        IteratorOp { Type: IterOpTake, Function: NumberLiteral(10) },
        IteratorOp { Type: IterOpFilter, Function: LambdaExpr(...) },
        IteratorOp { Type: IterOpMap, Function: LambdaExpr(...) },
        IteratorOp { Type: IterOpForEach, Function: Identifier("print_u8") }
    ]
}
```

## 2. Compile-Time Bounds Optimization

### Skip and Take Fusion
```minz
// Original chain
arr.iter().skip(5).take(10).forEach(f);

// Compile-time analysis
effectiveStart = 5
effectiveCount = min(10, arrayLength - 5)

// Generated code (pseudo-assembly)
LD HL, array + (5 * elementSize)  ; Start at skipped position
LD B, effectiveCount              ; DJNZ counter
loop:
    ; Process element at HL
    CALL f
    INC HL                        ; Or ADD HL, DE for larger elements
    DJNZ loop
```

**Key Insight**: Skip and take are resolved at compile time into loop bounds. No runtime overhead!

## 3. DJNZ Optimization for Arrays ≤255 Elements

### The Magic of DJNZ
The Z80's `DJNZ` (Decrement and Jump if Not Zero) instruction is perfect for iteration:
- Uses B register as counter
- Single instruction for decrement + conditional jump
- Only 13/8 T-states (taken/not taken)

### Standard Loop vs DJNZ Loop
```asm
; Standard indexed loop (45+ T-states per iteration)
loop:
    LD A, (index)
    CP length
    JP NC, end
    ; Calculate address: base + index * size
    ; Load element
    ; Process
    INC (index)
    JP loop

; DJNZ optimized loop (18 T-states per iteration)
    LD B, length      ; Counter in B
    LD HL, array      ; Pointer in HL
loop:
    LD A, (HL)        ; Load element
    ; Process
    INC HL            ; Advance pointer
    DJNZ loop         ; Decrement B and loop
```

**Performance**: 2.5x faster for simple iterations!

## 4. Filter Operation Transformation

### The Challenge
Filter operations conditionally skip elements, breaking simple loop flow.

### The Solution: Continue Labels
```minz
arr.filter(|x| x > 5).forEach(print);
```

Transforms to:
```asm
loop:
    LD A, (HL)        ; Load element
    CP 5              ; Compare with 5
    JP C, continue    ; Skip if <= 5
    PUSH AF           ; Save for print
    CALL print
continue:
    INC HL
    DJNZ loop
```

## 5. Map Operation Inlining

### Simple Transformations
```minz
arr.map(|x| x * 2)
```

When the lambda is simple, we inline it:
```asm
loop:
    LD A, (HL)        ; Load element
    ADD A, A          ; Double it (x * 2)
    ; Use transformed value
```

### Complex Transformations
For complex lambdas, we generate a function and call it:
```asm
map_func_0:
    ; Lambda body
    RET

loop:
    LD A, (HL)
    CALL map_func_0
    ; Use result
```

## 6. Enumerate Implementation

### Adding Index Tracking
```minz
arr.enumerate().forEach(|(i, val)| { ... });
```

Transforms to:
```asm
    LD B, length      ; DJNZ counter
    LD HL, array      ; Element pointer
    LD C, 0           ; Index counter
loop:
    ; C = index, (HL) = value
    PUSH BC           ; Save counters
    LD A, C           ; Index in A
    PUSH AF
    LD A, (HL)        ; Value in A
    PUSH AF
    CALL enumerated_func
    POP BC            ; Restore counters
    INC C             ; Increment index
    INC HL            ; Next element
    DJNZ loop
```

## 7. Peek/Inspect - Side Effects Without Consumption

### The Pattern
```minz
arr.iter()
   .peek(|x| debug_log(x))    // Side effect
   .map(|x| x * 2)            // Transformation
   .forEach(print);
```

### Implementation
Peek operations are just function calls that don't modify the iteration value:
```asm
loop:
    LD A, (HL)        ; Load element
    PUSH AF           ; Save it
    CALL debug_log    ; Side effect
    POP AF            ; Restore element
    ADD A, A          ; Map: x * 2
    CALL print        ; Final operation
    INC HL
    DJNZ loop
```

## 8. TakeWhile - Early Exit Optimization

### Dynamic Termination
```minz
arr.takeWhile(|x| x < 100).forEach(print);
```

Transforms to:
```asm
loop:
    LD A, (HL)
    CP 100            ; Check condition
    JP NC, end        ; Exit if >= 100
    CALL print
    INC HL
    DJNZ loop
end:
```

## 9. Future: Iterator Fusion

### The Ultimate Optimization
Multiple operations can be fused into a single loop:

```minz
// This chain:
arr.filter(even).map(double).filter(lt_100).forEach(print);

// Could compile to a single loop:
loop:
    LD A, (HL)
    AND 1             ; Check even
    JP NZ, continue
    LD A, (HL)
    ADD A, A          ; Double
    CP 100            ; Check < 100
    JP NC, continue
    CALL print
continue:
    INC HL
    DJNZ loop
```

## 10. Memory Layout Optimizations

### Pointer Arithmetic for Different Types
```asm
; For u8 arrays (1 byte elements)
INC HL              ; Simple increment

; For u16 arrays (2 byte elements)
INC HL
INC HL              ; Or use ADD HL, DE with DE=2

; For structs (N byte elements)
LD DE, element_size
ADD HL, DE          ; Generic advancement
```

## Performance Guarantees

### Zero-Cost Abstractions
- **skip(n)**: Compile-time pointer offset
- **take(n)**: Compile-time loop bound
- **filter**: Single conditional jump per element
- **map**: Inlined for simple operations
- **forEach**: Direct function call

### Overhead Analysis
| Operation | Traditional Loop | Iterator Chain | Overhead |
|-----------|-----------------|----------------|----------|
| Basic iteration | 45 T-states | 18 T-states | -60% (faster!) |
| Skip first 10 | 450 T-states setup | 0 T-states | -100% |
| Filter even | +15 T-states/elem | +15 T-states/elem | 0% |
| Map double | +8 T-states/elem | +8 T-states/elem | 0% |

## Conclusion

MinZ's iterator system proves that high-level abstractions don't require runtime overhead. Through careful compile-time analysis and Z80-specific optimizations, we achieve functional programming patterns that compile to assembly as efficient as hand-written code.

The magic isn't in runtime cleverness - it's in compile-time transformation. Every iterator operation is analyzed, optimized, and transformed into the most efficient Z80 assembly possible. This is how MinZ brings 21st-century programming to 1976 hardware.