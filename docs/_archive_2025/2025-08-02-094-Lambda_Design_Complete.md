# MinZ Lambda Design: Compile-Time Transformation

## Overview

MinZ lambdas are not runtime values but compile-time constructs that transform into regular functions. This design eliminates runtime overhead and integrates perfectly with MinZ's SMC (Self-Modifying Code) philosophy.

## Core Principles

1. **Lambda assignments create named functions** - Not anonymous values
2. **Only fully-curried lambdas can be returned** - Just function addresses
3. **No runtime parameter passing complexity** - All resolved at compile time
4. **Perfect SMC integration** - Currying uses parameter patching

## Transformation Rules

### 1. Local Lambda Assignment

When you write:
```minz
fun calculate() -> u8 {
    let add = |x: u8, y: u8| => u8 { x + y };
    add(10, 20)
}
```

The compiler transforms it to:
```minz
fun calculate$add_0(x: u8, y: u8) -> u8 {
    x + y
}

fun calculate() -> u8 {
    calculate$add_0(10, 20)  // Direct call!
}
```

**Key insight**: The lambda becomes a module-level function with a generated name. The "variable" `add` is just a compile-time reference to this function.

### 2. Lambda References

```minz
fun process() -> u8 {
    let double = |x: u8| => u8 { x * 2 };
    let f = double;  // Just copying the reference
    f(10)           // Still a direct call to process$double_0
}
```

### 3. No Capture Support (Initial Design)

```minz
fun outer(n: u8) -> u8 {
    // ERROR: Lambda captures 'n' from outer scope
    let bad = |x: u8| => u8 { x + n };
    
    // OK: No captures
    let good = |x: u8| => u8 { x + 5 };
    good(n)
}
```

### 4. Returning Lambdas (Must Be Fully Curried)

Only fully-applied lambdas can be returned, as simple function addresses:

```minz
fun make_adder(n: u8) -> fn(u8) -> u8 {
    // This generates a new function with 'n' baked in
    @curry(|x: u8| => u8 { x + n }, n)
}

fun test() -> u8 {
    let add5 = make_adder(5);   // Returns function address
    let add10 = make_adder(10); // Different function address
    
    // Simple indirect calls - no parameter magic!
    add5(15) + add10(15)  // 20 + 25 = 45
}
```

## Implementation Details

### Phase 1: Basic Transformation

1. **Parser**: No changes - lambda syntax remains the same
2. **Semantic Analyzer**: 
   - Detect lambda assignments
   - Generate function names: `{scope}${var}_{counter}`
   - Create IR functions instead of lambda values
   - Replace lambda calls with direct function calls

3. **Code Generator**: No changes - just generates normal functions

### Phase 2: Curry Support

The `@curry` intrinsic generates SMC functions:

```minz
@curry(|x: u8| => u8 { x + n }, n=5)
```

Generates:
```asm
curry_generated_XXX:
    LD A, 5      ; <-- This 5 is from the curry argument
    ADD A, L     ; x parameter in L
    RET
```

### Phase 3: TRUE SMC Curry Implementation

For maximum efficiency, use SMC templates:

```asm
; Template function
make_adder$template:
n$immOP:
    LD A, 0      ; <-- Patch point for n
n$imm0 EQU n$immOP+1
    ADD A, L     ; x parameter in L
    RET

; Curry function patches and returns address
make_adder:
    ; Patch template with n value (in A)
    LD (n$imm0), A
    ; Return template address
    LD HL, make_adder$template
    RET
```

## Why This Works

### 1. No Runtime Overhead
- Local lambdas compile to direct calls
- No closure allocation
- No runtime dispatch

### 2. Perfect Z80 Fit
- Function addresses fit in 16-bit registers
- Indirect calls via `CALL (HL)` are natural
- SMC enables efficient currying

### 3. Type Safety
- Compiler ensures only fully-applied lambdas escape
- No partial application confusion
- Clear ownership of parameters

### 4. Simplicity
- Lambda values are just function addresses
- No complex calling conventions
- No hidden allocations

## Examples

### Example 1: Map Function
```minz
fun map_array(arr: [u8; 10], f: fn(u8) -> u8) -> void {
    for i in 0..10 {
        arr[i] = f(arr[i]);
    }
}

fun test() -> void {
    let data = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
    
    // Local lambda becomes function
    let double = |x: u8| => u8 { x * 2 };
    map_array(data, double);  // Pass function reference
}
```

### Example 2: Curry Chain
```minz
fun add3(a: u8, b: u8, c: u8) -> u8 {
    a + b + c
}

fun curry_add3() -> fn(u8) -> u8 {
    let add_5_10 = @curry(@curry(add3, 5), 10);
    add_5_10  // Returns fn(u8) -> u8 that adds 5+10+x
}
```

### Example 3: Function Composition
```minz
fun compose(f: fn(u8) -> u8, g: fn(u8) -> u8) -> fn(u8) -> u8 {
    // Generate new function that does g(f(x))
    @generate_composition(f, g)
}
```

## Migration Path

1. **Keep current syntax** - No breaking changes
2. **Add transformation pass** - Convert lambdas to functions
3. **Error on captures** - Initially disallow
4. **Add curry support** - Via intrinsics
5. **Optimize common patterns** - Partial application, etc.

## Future Extensions

### Capture Support (Future)
Could pass captured variables as hidden parameters:
```minz
fun outer(n: u8) -> u8 {
    let add = |x: u8| => u8 { x + n };  // Captures n
    add(10)  // Transforms to: outer$add_0(10, n)
}
```

### Inline Lambdas (Future)
For tiny lambdas, inline at call site:
```minz
let sum = fold(arr, 0, |a, b| => u8 { a + b });
// Could inline the lambda body into fold's loop
```

## Conclusion

This design transforms MinZ lambdas from a runtime feature into a compile-time construct, eliminating complexity while maintaining expressiveness. By embracing the "code IS the data" philosophy, we get efficient lambda support that feels natural on Z80 hardware.