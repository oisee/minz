# Lambda Compile-Time Transformation Design

## Core Insight

Instead of runtime lambda values, transform lambdas at compile time:
1. Local lambda assignments become named functions
2. Only fully-curried lambdas can be returned (just function addresses)
3. Partial application generates specialized functions

## Transformation Rules

### 1. Local Lambda Assignment
```minz
let f = |x: u8| => u8 { x + 5 };
f(10)
```
Becomes:
```minz
fun scope_f_0(x: u8) -> u8 { x + 5 }
scope_f_0(10)  // Direct call!
```

### 2. Lambda with Captures
```minz
fun outer(n: u8) -> u8 {
    let add = |x: u8| => u8 { x + n };
    add(10)
}
```
Becomes:
```minz
fun outer_add_0(x: u8, n: u8) -> u8 { x + n }
fun outer(n: u8) -> u8 {
    outer_add_0(10, n)  // Pass captured variables as extra params
}
```

### 3. Returning Lambdas (Must be Fully Curried)
```minz
fun make_adder(n: u8) -> fn(u8) -> u8 {
    // Generate specialized function with n baked in
    @generate_smc_function(|x: u8| => u8 { x + n }, n)
}
```
Compiler generates:
```asm
make_adder_generated_XXX:
    ; TRUE SMC with n patched in
    LD A, [n_value]  ; This immediate gets patched
    ADD A, L         ; x in L
    RET
```

## Benefits

1. **Zero runtime overhead** - All lambdas compile to regular functions
2. **Perfect SMC integration** - Currying naturally uses SMC patching
3. **Simple indirect calls** - Returned lambdas are just addresses
4. **Type safety** - Compiler ensures only fully-applied lambdas escape

## Implementation Strategy

### Phase 1: Transform Local Lambdas
- Detect lambda assignments in semantic analyzer
- Generate module-level functions instead of lambda values
- Replace lambda calls with direct function calls

### Phase 2: Capture Analysis
- Track free variables in lambda bodies
- Pass captured variables as additional parameters
- Error on attempts to return lambdas with captures

### Phase 3: Curry Support
- Add `@curry` intrinsic for manual currying
- Generate SMC functions with patched parameters
- Return just the function address

### Phase 4: Partial Application
- Detect partial application patterns
- Generate wrapper functions
- Use SMC to patch known arguments

## Example: Full Curry Implementation

```minz
fun curry2(f: fn(u8, u8) -> u8, a: u8) -> fn(u8) -> u8 {
    // Compiler generates a new function:
    // curry2_f_a_XXX:
    //     LD A, [a_immediate]  ; SMC patched
    //     LD H, A
    //     LD A, L              ; Second param in L
    //     CALL f
    //     RET
    @generate_curry_wrapper(f, a)
}
```

## Migration Path

1. Keep current lambda syntax
2. Add compiler pass to transform lambdas
3. Gradually optimize common patterns
4. Eventually remove runtime lambda support

This design perfectly aligns with MinZ's philosophy: "The code IS the data structure!"