# 030: TRUE SMC-Powered Lambdas/Closures Design

## Brilliant Insight!

You're absolutely right - TRUE SMC provides the perfect mechanism for ultra-efficient lambdas with captured variables! Instead of heap allocation or stack frames, we can patch captured values directly into the lambda's code.

## Core Concept

```minz
fn make_adder(x: u8) -> fn(u8) -> u8 {
    return |y| x + y  // x is captured and patched into the lambda!
}

// Usage:
let add5 = make_adder(5)
let result = add5(3)  // Returns 8
```

## Implementation Strategy

### 1. Lambda as SMC Template

```asm
; Lambda template for |y| x + y
lambda_add_template:
    ; TRUE SMC anchor for captured 'x'
    LD B, 0        ; lambda$x anchor - will be patched
    ADD A, B       ; A already contains 'y' parameter
    RET
```

### 2. Creating Lambda Instance

```asm
make_adder:
    ; Input: A = x value to capture
    LD B, A        ; Save x
    
    ; Allocate lambda instance (copy template)
    LD HL, lambda_add_template
    LD DE, lambda_instance_addr
    LD BC, lambda_size
    LDIR           ; Copy template
    
    ; Patch captured value using TRUE SMC
    LD A, B        ; Get x back
    LD (lambda_instance_addr + 1), A  ; Patch the immediate!
    
    ; Return lambda address
    LD HL, lambda_instance_addr
    RET
```

## Advantages Over Traditional Closures

1. **Zero Allocation Overhead**: No heap, no GC
2. **Direct Code Patching**: Values are literal immediates
3. **Optimal Performance**: No indirection, no lookups
4. **Cache Friendly**: Code and data together

## Multiple Captures

```minz
fn make_rect_area(width: u8, height: u8) -> fn() -> u16 {
    return || width * height
}
```

Generated:
```asm
lambda_rect_template:
width$imm:
    LD A, 0        ; Patched with width
height$imm:
    LD B, 0        ; Patched with height
    ; Multiply A * B
    RET
```

## Design Decisions

### 1. Lambda Allocation Strategy

**Option A: Static Pool**
- Pre-allocate N lambda slots
- Simple, predictable
- Limited number of active lambdas

**Option B: Stack Allocation**
- Lambdas live on stack
- Automatic cleanup
- Can't outlive creator

**Option C: Arena Allocation**
- Dedicated lambda memory region
- Manual management
- Most flexible

**Recommendation**: Start with Option A for simplicity

### 2. Syntax Options

```minz
// Rust-style
let f = |x| x + 1

// Arrow function style  
let f = (x) => x + 1

// Minimalist
let f = \x -> x + 1
```

**Recommendation**: Rust-style `|x|` fits MinZ aesthetic

### 3. Type Inference

Lambdas should infer types from context:
```minz
let nums: [u8; 10] = [1, 2, 3, ...]
let doubled = nums.map(|x| x * 2)  // x inferred as u8
```

## Implementation Phases

### Phase 1: Simple Lambdas
- Single parameter
- Single capture
- No recursion
- Stack allocated

### Phase 2: Full Closures
- Multiple captures
- Multiple parameters  
- Proper lifetime tracking
- Arena allocation

### Phase 3: Advanced Features
- Generic lambdas
- Recursive lambdas
- Lambda literals in data
- Optimization passes

## Code Generation Example

Input:
```minz
let multiplier = 5
let f = |x| x * multiplier
let result = f(3)
```

Generated:
```asm
; Set up multiplier
LD A, 5
LD (multiplier), A

; Create lambda instance
LD HL, lambda_template_1
LD DE, $F000        ; Lambda allocation area
LD BC, 10           ; Lambda size
LDIR

; Patch multiplier into lambda
LD A, (multiplier)
LD ($F001), A       ; Patch immediate at $F000+1

; Call lambda with 3
LD A, 3
CALL $F000
; Result in A = 15
```

## Memory Layout

```
Lambda Pool at $E000:
+0000: [Lambda 0 - 16 bytes]
+0010: [Lambda 1 - 16 bytes]
+0020: [Lambda 2 - 16 bytes]
...
+00F0: [Lambda 15 - 16 bytes]
```

## Challenges

1. **Lifetime Management**: When can we reuse lambda slots?
2. **Debugging**: Patched code is harder to debug
3. **Recursion**: Self-referential lambdas need special handling
4. **Size Limits**: Each lambda needs space for code + patches

## Conclusion

TRUE SMC makes MinZ lambdas potentially the most efficient closure implementation ever created for 8-bit systems. By patching captured values directly into code, we eliminate all the overhead traditionally associated with closures.

This feature would make MinZ truly unique - a retro language with modern functional programming features that run faster than imperative code!