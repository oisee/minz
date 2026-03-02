# DJNZ Loop Optimization

## Overview

MinZ now automatically optimizes `for-range` loops and iterator chains to use the Z80's DJNZ instruction when possible, providing significant performance improvements.

## What is DJNZ?

DJNZ (Decrement and Jump if Not Zero) is a Z80 instruction that:
- Decrements the B register
- Jumps to a label if B != 0
- Takes only 13 T-states (vs ~25+ for compare-jump sequences)

## Automatic Optimization

### For-Range Loops

Loops of the form `for i in 0..N` where N ≤ 255 automatically use DJNZ:

```minz
fun fill_buffer() -> void {
    for i in 0..10 {
        buffer[i] = i * 2;
    }
}
```

Generates:
```asm
; DJNZ OPTIMIZED: for i in 0..10
    LD B, 10           ; Counter in B
    XOR A              ; i = 0
.loop:
    ; ... loop body ...
    INC A              ; i++
    DJNZ .loop         ; B--, jump if B != 0
```

### Iterator Chains

Iterator operations on arrays ≤ 255 elements also use DJNZ:

```minz
numbers.iter()
    .map(|x| x * 2)
    .filter(|x| x > 5)
    .forEach(print_u8);
```

Generates:
```asm
; DJNZ OPTIMIZED LOOP for array[N]
    LD B, N            ; Counter = array length
    LD HL, numbers     ; Pointer to array
.loop:
    ; ... map, filter, forEach inlined ...
    INC HL             ; Next element
    DJNZ .loop
```

## Conditions for Optimization

DJNZ optimization is applied when:
1. Loop start is 0 (constant)
2. Loop end is constant and ≤ 255
3. For iterator chains: array length is known and ≤ 255

## Performance Impact

| Pattern | Before | After | Speedup |
|---------|--------|-------|---------|
| Simple loop 0..100 | ~30 T/iter | ~17 T/iter | 1.7x |
| Iterator forEach | ~35 T/iter | ~20 T/iter | 1.75x |

## Implementation

The optimization is implemented in:
- `semantic/analyzer.go`: `analyzeForStmtDJNZ()` for for-range loops
- `semantic/iterator.go`: `generateDJNZIteration()` for iterator chains

Both paths:
1. Detect constant bounds at compile time
2. Allocate B register for DJNZ counter
3. Maintain user-visible iterator if needed
4. Emit single DJNZ instruction at loop end

## Future: Generator Support

Planned syntax for generators that compile to DJNZ:

```minz
gen range(n: u8) -> u8 {
    for i in 0..n {
        yield i;
    }
}

// Usage - compiles to DJNZ loop!
range(10).forEach(print_u8);
```

This would enable:
- Lazy evaluation
- Infinite sequences (with take)
- Zero-cost abstraction over iteration patterns
