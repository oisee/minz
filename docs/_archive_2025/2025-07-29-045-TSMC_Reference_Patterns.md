# TSMC Reference Patterns Documentation

## Overview

TSMC (Tree-Structured Machine Code) references are a revolutionary MinZ feature where pointer parameters become addresses to immediate operands within the instruction stream. This enables self-modifying code patterns that eliminate pointer indirection overhead.

## Core Concept

In traditional architectures:
```minz
// Traditional: ptr contains address of data in memory
fun process(ptr: *u8) {
    *ptr = 42;  // Loads ptr, then stores to memory address
}
```

With TSMC references:
```minz
// TSMC: ptr IS the address of immediate operand
@abi("smc")
fun process(ptr: *u8) {
    *ptr = 42;  // Modifies immediate in the dereference instruction itself!
}
```

## How TSMC References Work

### 1. Declaration
Only pointer parameters in SMC functions become TSMC references:
```minz
@abi("smc")
fun example(p: *u8, q: *u16, val: u8) {
    // p and q are TSMC references
    // val is a regular parameter
}
```

### 2. Code Generation
The compiler generates special anchors for TSMC references:
```asm
; TSMC anchor for parameter 'str'
str$immOP:
    LD HL, 0       ; str anchor (will be patched)
str$imm0 EQU str$immOP+1
```

### 3. Self-Modification
Updates to TSMC references modify the immediate field:
```minz
str = str + 1;  // Generates: LD (str$imm0), HL
```

## Common TSMC Patterns

### Pattern 1: String Processing
TSMC strlen - the pointer advances through the string by self-modification:
```minz
@abi("smc")
fun strlen_tsmc(str: *u8) -> u16 {
    var len: u16 = 0;
    while *str != 0 {
        len = len + 1;
        str = str + 1;  // Self-modifies the dereference above!
    }
    return len;
}
```

### Pattern 2: Array Iteration
Process array elements with zero-overhead iteration:
```minz
@abi("smc")
fun sum_array(arr: *u8, count: u16) -> u16 {
    var sum: u16 = 0;
    for i in 0..count {
        sum = sum + *arr;
        arr = arr + 1;  // Modifies the dereference in next iteration
    }
    return sum;
}
```

### Pattern 3: Memory Fill
Ultra-fast memory filling:
```minz
@abi("smc")
fun memset_tsmc(dest: *u8, val: u8, count: u16) {
    for i in 0..count {
        *dest = val;
        dest = dest + 1;  // Next iteration writes to new address
    }
}
```

### Pattern 4: Search Operations
Find character in string with minimal overhead:
```minz
@abi("smc")
fun strchr_tsmc(str: *u8, ch: u8) -> *u8 {
    while *str != 0 {
        if *str == ch {
            return str;
        }
        str = str + 1;
    }
    return null;
}
```

### Pattern 5: Conditional Updates
TSMC references can be updated conditionally:
```minz
@abi("smc")
fun skip_whitespace(str: *u8) -> *u8 {
    while *str == ' ' || *str == '\t' {
        str = str + 1;  // Skip whitespace characters
    }
    return str;
}
```

## Performance Benefits

### Traditional Pointer Access (6-10 T-states per access)
```asm
LD HL, (ptr_addr)  ; 16 T-states: Load pointer
LD A, (HL)         ; 7 T-states: Dereference
; Total: 23 T-states
```

### TSMC Reference Access (7 T-states)
```asm
LD HL, immediate   ; 10 T-states: Direct immediate (but patched once)
LD A, (HL)         ; 7 T-states: Dereference
; Total: 7 T-states after first access
```

### Iteration Comparison
For a loop processing 100 bytes:
- Traditional: 100 × 23 = 2,300 T-states
- TSMC: 10 + 99 × 7 = 703 T-states
- **Performance gain: 3.3x faster**

## Implementation Details

### Semantic Analysis
1. Parameters marked as TSMC refs during function analysis
2. Assignment to TSMC refs generates OpStoreTSMCRef
3. Auto-deref disabled for TSMC references

### Code Generation
1. TSMC anchors created for each reference parameter
2. Updates use special immediate labels (param$imm0)
3. TRUE SMC calling convention patches immediates atomically

### Optimization Opportunities
1. **Loop unrolling**: TSMC refs make unrolling more effective
2. **Instruction fusion**: Combine increment with comparison
3. **Pattern matching**: Detect common TSMC patterns for specialized codegen

## Best Practices

### DO:
- Use TSMC for tight loops over memory
- Combine with other optimizations (unrolling, fusion)
- Consider memory access patterns
- Use for string/array processing

### DON'T:
- Use TSMC for random access patterns
- Mix TSMC and regular pointers carelessly
- Forget that TSMC refs modify code, not data
- Use for rarely-accessed data

## Advanced Patterns

### Multi-Level TSMC
Process 2D arrays with nested TSMC:
```minz
@abi("smc")
fun process_matrix(row: **u8, rows: u16, cols: u16) {
    for r in 0..rows {
        var col: *u8 = *row;
        for c in 0..cols {
            *col = transform(*col);
            col = col + 1;
        }
        row = row + 1;
    }
}
```

### TSMC with Callbacks
Pass TSMC refs to other SMC functions:
```minz
@abi("smc")
fun foreach_byte(ptr: *u8, count: u16, fn: fun(*u8)) {
    for i in 0..count {
        fn(ptr);  // Callback receives current TSMC ref
        ptr = ptr + 1;
    }
}
```

## Debugging TSMC Code

### Common Issues:
1. **Forgetting @abi("smc")**: Parameters won't be TSMC refs
2. **Mixing conventions**: Can't pass TSMC ref to non-SMC function
3. **Code/data confusion**: TSMC modifies code, not data memory

### Debugging Tips:
1. Check generated assembly for TSMC anchors
2. Verify OpStoreTSMCRef in IR output
3. Use -O flag to see optimization effects
4. Test with small examples first

## Future Enhancements

1. **TSMC Arrays**: Multiple TSMC refs in parallel
2. **TSMC Structs**: Self-modifying field access
3. **TSMC Chains**: Linked TSMC references
4. **Hardware TSMC**: CPU support for immediate patching

## Conclusion

TSMC references represent a paradigm shift in pointer handling, turning the instruction stream itself into a data structure. By eliminating indirection and leveraging self-modification, TSMC enables performance levels impossible with traditional architectures.