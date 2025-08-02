# Lambda Design Rationale: Why Compile-Time Transformation

## The Problem with Runtime Lambdas

Traditional lambda implementations require:
1. **Closure allocation** - Runtime memory for captured variables
2. **Indirect dispatch** - Function pointers with complex calling conventions  
3. **Parameter passing overhead** - Stack manipulation for lambda calls
4. **GC pressure** - Lambdas as heap objects need management

On Z80 with 64KB RAM and no hardware stack manipulation, these costs are prohibitive.

## The MinZ Solution: Compile-Time Transformation

### Core Insight
Lambdas in MinZ are **syntactic sugar** for function definitions and references. They exist only at compile time, transforming into regular functions.

### Why This Works

#### 1. Most Lambda Uses Are Static
```minz
// 90% of lambda usage:
let double = |x| { x * 2 };
map(array, double);  // Known at compile time!
```

The compiler can transform this to:
```minz
fun generated_double(x: u8) -> u8 { x * 2 }
map(array, generated_double);  // Direct function reference
```

#### 2. Z80 Architecture Favors Static Code
- **Fixed memory layout** - No virtual memory or relocation
- **Limited registers** - Can't waste them on closure pointers
- **Direct calls are fast** - 17 cycles vs 24+ for indirect
- **SMC is natural** - Self-modifying code for specialization

#### 3. MinZ Philosophy: "Code IS Data"
Instead of runtime data structures representing functions, the functions themselves ARE the data:
```asm
; Traditional: Function pointer in HL, parameters on stack
LD HL, (lambda_ptr)
PUSH BC      ; Parameter
CALL (HL)    ; Indirect call

; MinZ: Direct call or SMC-specialized function
CALL generated_lambda   ; Direct - 17 cycles
; OR for curried:
specialized_func:       ; Code IS the closure!
    LD A, 5            ; Captured value baked in
    ADD A, L           ; Parameter in L
    RET
```

## Design Decisions

### 1. No Variable Capture (Initially)
**Rationale**: Simplifies implementation and forces clean design
```minz
fun bad(n: u8) -> fn() -> u8 {
    |x| { x + n }  // ERROR: Captures 'n'
}

fun good(n: u8) -> fn(u8) -> u8 {
    @curry(|x, m| { x + m }, n)  // OK: Explicit curry
}
```

### 2. Only Fully-Curried Lambdas Can Escape
**Rationale**: Returned lambdas must be self-contained
```minz
fun make_adder(n: u8) -> fn(u8) -> u8 {
    // Must curry to "bake in" n:
    @curry(|x: u8| => u8 { x + n }, n)
}
```

The curry operation generates a new function with `n` as an immediate value.

### 3. Local Lambdas Become Module Functions  
**Rationale**: Enables direct calls and debugging
```minz
fun process() {
    let f = |x| { x + 1 };  // Becomes process$f_0
    f(10);                   // Direct call
}
```

Benefits:
- Visible in disassembly as `process$f_0`
- Can set breakpoints
- No runtime allocation

### 4. Lambda Variables Are Compile-Time References
**Rationale**: Zero runtime cost
```minz
let f = |x| { x * 2 };
let g = f;        // Just copying a reference
let h = g;        // Still the same function
h(10);            // Calls the original generated function
```

## Implementation Strategy

### Phase 1: Basic Transformation (Current)
- Transform lambda assignments to functions
- Error on captures
- Direct calls only

### Phase 2: Curry Support
- Add `@curry` intrinsic
- Generate SMC-specialized functions
- Return addresses only

### Phase 3: Optimization
- Inline tiny lambdas
- Specialize hot paths
- Partial evaluation

### Phase 4: Advanced Features (Future)
- Capture support via hidden parameters
- Lambda lifting optimization
- Compile-time partial application

## Comparison with Other Approaches

### Traditional (Heap-Allocated Closures)
```rust
// Rust-style
let n = 5;
let f = move |x| x + n;  // Heap allocation
Box::new(f)               // Return boxed closure
```
**Cost**: Allocation, indirection, GC

### MinZ (Compile-Time Transformation)
```minz
let n = 5;
let f = @curry(|x, m| { x + m }, n);  // Generate specialized function
f  // Just return address
```
**Cost**: Zero runtime overhead

### C Function Pointers
```c
int (*f)(int) = &my_function;
f(10);  // Indirect call
```
**Limitation**: No captures, manual management

### MinZ Advantage
- **Syntax** of high-level lambdas
- **Performance** of direct calls  
- **Flexibility** of SMC specialization

## Real-World Example: Event System

```minz
// Traditional: Runtime dispatch table
struct EventHandler {
    handlers: [fn(u8); 256]  // Function pointer array
}

// MinZ: Compile-time generated jump table
fun on_keypress(key: u8) {
    match key {
        'A' => handle_a(),   // Direct calls!
        'B' => handle_b(),
        _ => handle_default()
    }
}

// Or with lambdas:
fun setup_handlers() {
    on_key('A', |k| { print("A pressed") });  // Generates handler function
    on_key('B', |k| { print("B pressed") });  // Another function
}
```

## Performance Impact

### Memory Usage
- **Traditional**: ~6-12 bytes per closure (pointer + captured vars)
- **MinZ**: 0 bytes (functions exist in code segment)

### Call Performance
- **Direct call**: 17 cycles
- **Indirect call**: 24+ cycles  
- **MinZ curry call**: 17 cycles (direct to specialized function)

### Code Size
- **Slight increase**: Each lambda becomes a function
- **Mitigated by**: Deduplication of identical lambdas
- **Offset by**: No runtime support code needed

## Conclusion

MinZ's compile-time lambda transformation is not a limitation but a feature. By embracing static compilation and Z80's architecture, we get:

1. **High-level syntax** - Write expressive functional code
2. **Zero-cost abstraction** - No runtime overhead
3. **Perfect SMC integration** - Currying via code specialization
4. **Predictable performance** - No hidden allocations or indirection

This design embodies MinZ's philosophy: elegant abstractions that compile to efficient machine code, where "the code IS the data structure."