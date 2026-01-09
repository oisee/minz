# MinZ Generators: Zero-Cost Iteration Vision

## The Dream

Imagine writing this elegant, functional code:

```minz
range(100)
    .map(|x| x * x)
    .filter(|x| x % 2 == 0)
    .take(10)
    .forEach(print_u8);
```

And having it compile to this tight Z80 assembly:

```asm
    LD B, 10          ; take(10) counter
    XOR A             ; x = 0
.loop:
    LD C, A           ; save x
    ; x * x (inline)
    LD D, A
    CALL mul8         ; or inline multiply
    ; filter: x % 2 == 0
    BIT 0, A
    JR NZ, .skip
    ; forEach: print
    CALL print_u8
    DEC B
    JR Z, .done
.skip:
    LD A, C
    INC A
    JR .loop
.done:
```

**No heap. No closures. No runtime. Just DJNZ.**

---

## Generator Syntax

### Basic Generator

```minz
gen range(n: u8) -> u8 {
    for i in 0..n {
        yield i;
    }
}
```

### Infinite Generator

```minz
gen naturals() -> u16 {
    let n: u16 = 0;
    loop {
        yield n;
        n = n + 1;
    }
}

// Usage - take() provides termination
naturals().take(100).forEach(print_u16);
```

### Parameterized Generator

```minz
gen fibonacci() -> u16 {
    let a: u16 = 0;
    let b: u16 = 1;
    loop {
        yield a;
        let tmp = a + b;
        a = b;
        b = tmp;
    }
}

// First 20 Fibonacci numbers
fibonacci().take(20).forEach(print_u16);
```

### Generator with State

```minz
gen counter(start: u8, step: u8) -> u8 {
    let current = start;
    loop {
        yield current;
        current = current + step;
    }
}

// 0, 2, 4, 6, 8...
counter(0, 2).take(10).forEach(print_u8);
```

---

## Iterator Methods (Complete API)

### Transformations

```minz
.map(|x| expr)           // Transform each element
.flatMap(|x| gen)        // Transform and flatten
.scan(init, |acc, x| expr) // Running accumulation
```

### Filters

```minz
.filter(|x| pred)        // Keep if predicate true
.takeWhile(|x| pred)     // Take until predicate false
.skipWhile(|x| pred)     // Skip until predicate false
.take(n)                 // Take first n elements
.skip(n)                 // Skip first n elements
.step(n)                 // Take every nth element
```

### Combinators

```minz
.zip(other)              // Pair with another iterator
.chain(other)            // Concatenate iterators
.interleave(other)       // Alternate elements
.enumerate()             // Add index: (i, x)
```

### Terminators

```minz
.forEach(|x| action)     // Execute for each
.collect()               // Collect into array
.reduce(init, |acc, x| expr)  // Fold to single value
.find(|x| pred)          // First matching element
.any(|x| pred)           // True if any match
.all(|x| pred)           // True if all match
.count()                 // Count elements
.sum()                   // Sum all elements
.min() / .max()          // Find extremes
```

### Inspection

```minz
.peek(|x| action)        // Side effect without consuming
.inspect(|x| action)     // Alias for peek
```

---

## Zero-Cost Compilation Strategy

### Phase 1: Fusion Analysis

The compiler analyzes iterator chains and fuses operations:

```minz
range(100).map(f).filter(g).take(10).forEach(h)
```

Becomes a single fused loop:

```
FusedLoop {
    source: Range(0, 100),
    transforms: [Map(f), Filter(g)],
    limit: Take(10),
    terminator: ForEach(h)
}
```

### Phase 2: DJNZ Pattern Detection

Identify loops that can use DJNZ:
- Known constant bounds ≤ 255
- `take(n)` with n ≤ 255
- Countdown patterns

### Phase 3: Code Generation

Generate optimal Z80:

1. **Counter in B** - for DJNZ
2. **Iterator in A/C** - for element access
3. **Pointer in HL** - for array traversal
4. **Inline lambdas** - no CALL overhead

### Example Compilation

```minz
gen squares() -> u8 {
    for i in 0..16 {
        yield i * i;
    }
}

squares().filter(|x| x > 10).forEach(print_u8);
```

Compiles to:

```asm
; squares().filter(|x| x > 10).forEach(print_u8)
    LD B, 16          ; Generator: 0..16
    XOR A             ; i = 0
.loop:
    ; yield i * i (inlined)
    LD C, A           ; Save i
    LD D, A
    CALL mul8_inline  ; A = i * i (or unroll for small)

    ; filter: x > 10
    CP 11             ; Compare with 10+1
    JR C, .skip       ; Skip if <= 10

    ; forEach: print_u8 (inlined or call)
    CALL print_u8

.skip:
    LD A, C           ; Restore i
    INC A             ; i++
    DJNZ .loop        ; B--, loop if B != 0
```

**Total overhead: ZERO** - the abstraction compiles away completely.

---

## Advanced: Coroutine-Style Generators

For more complex state machines, generators can suspend and resume:

```minz
gen state_machine() -> Event {
    // Initial state
    yield Event::Started;

    // Wait for input
    let input = receive();

    if input == COMMAND_A {
        yield Event::ProcessingA;
        // ... do work ...
        yield Event::CompletedA;
    } else {
        yield Event::ProcessingB;
        // ... do work ...
        yield Event::CompletedB;
    }

    yield Event::Finished;
}
```

This compiles to a state machine with SMC for state storage:

```asm
state_machine:
    LD A, (state_var)     ; Load current state
    CP STATE_INITIAL
    JR Z, .state_initial
    CP STATE_PROCESSING_A
    JR Z, .state_processing_a
    ; ... etc
```

---

## Memory Model: Stack vs Stackless

### Stackless Generators (Default)

For simple generators, state lives in registers/globals:

```minz
gen counter(n: u8) -> u8 {
    for i in 0..n { yield i; }
}
```

State: Just `i` in a register. No allocation.

### Stack Generators (Complex)

For recursive or deeply nested generators:

```minz
gen tree_walk(node: *Node) -> u8 {
    if node.left != null {
        for x in tree_walk(node.left) {
            yield x;
        }
    }
    yield node.value;
    if node.right != null {
        for x in tree_walk(node.right) {
            yield x;
        }
    }
}
```

Uses minimal stack frames, but still optimizes leaf operations.

---

## Integration with SMC

Generators + Self-Modifying Code = Ultimate Performance

```minz
@smc
gen adaptive_counter(n: u8) -> u8 {
    // The loop bound is patched at runtime!
    for i in 0..n {
        yield i;
    }
}

fun process(count: u8) -> void {
    // Patches the generator's loop bound
    adaptive_counter(count).forEach(process_item);
}
```

The `n` parameter becomes an immediate value patched directly into:

```asm
adaptive_counter:
.bound_patch:
    LD B, 0           ; <-- Patched with actual count
.loop:
    ; ...
    DJNZ .loop
```

---

## Implementation Roadmap

### Phase 1: Basic Generators (Week 1-2)
- [ ] `gen` keyword in grammar
- [ ] `yield` statement parsing
- [ ] Single-yield generators (simple loops)
- [ ] Fusion with existing iterator chains

### Phase 2: Iterator Method Expansion (Week 2-3)
- [ ] `scan`, `flatMap`, `step`
- [ ] `zip`, `interleave`
- [ ] `find`, `any`, `all`
- [ ] Improved fusion analysis

### Phase 3: Infinite Generators (Week 3-4)
- [ ] `loop { yield }` pattern
- [ ] Mandatory `take()` for termination
- [ ] Compile-time infinite loop detection

### Phase 4: Advanced Features (Week 4+)
- [ ] Multi-yield generators
- [ ] Generator state machines
- [ ] SMC-optimized generators
- [ ] Recursive generators (tree walking)

---

## Philosophy: Why This Matters

### The Z80 Paradox

The Z80 is a **register-starved, memory-constrained** 8-bit CPU from 1976.

Yet we want to write **modern, expressive, functional code**.

Traditional approach: "You can't have nice things on Z80."

MinZ approach: **"Nice things that compile to nothing."**

### Zero-Cost Abstraction Hierarchy

```
High-Level Code          What You Write
        ↓
Iterator Chains          .map().filter().forEach()
        ↓
Generator Fusion         Single fused loop
        ↓
DJNZ Optimization        Tight Z80 loop
        ↓
Machine Code             Just the bytes you need
```

Every layer **compiles away**. The abstraction is free.

### The MinZ Promise

> Write like you have a modern CPU.
> Run like you're hand-coding assembly.
> No compromises.

---

## Example: Complete Demo

```minz
// Generate prime numbers using Sieve of Eratosthenes pattern
gen primes() -> u8 {
    let sieve: [bool; 256];

    // Initialize
    for i in 0..256 {
        sieve[i] = true;
    }

    for n in 2..256 {
        if sieve[n] {
            yield n;

            // Mark multiples as not prime
            let multiple = n + n;
            while multiple < 256 {
                sieve[multiple] = false;
                multiple = multiple + n;
            }
        }
    }
}

fun main() -> void {
    // Print first 20 primes
    primes()
        .take(20)
        .enumerate()
        .forEach(|(i, p)| {
            print_u8(i);
            print_str(": ");
            print_u8(p);
            print_newline();
        });
}
```

Output:
```
0: 2
1: 3
2: 5
3: 7
4: 11
...
19: 71
```

All with **zero heap allocation**, **DJNZ loops**, and **inlined lambdas**.

---

## Conclusion

MinZ generators represent the ultimate fusion of:
- **Elegance**: Write functional, declarative code
- **Performance**: Compile to optimal Z80 assembly
- **Zero-cost**: Abstractions that disappear at compile time

This isn't about making Z80 "good enough."
This is about making Z80 **sing**.

---

*"The best abstraction is one you can't see in the generated code."*
