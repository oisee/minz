# MinZ Optimization Notes

## Absolute vs IX-Relative Addressing

The MinZ compiler now implements an important optimization for local variable access:

### Non-Recursive Functions
- Use **absolute addressing** (e.g., `LD HL, ($F000)`)
- Local variables stored at fixed addresses starting at 0xF000
- Benefits:
  - `LD (addr), HL` - 16 T-states (vs 38 for IX-indexed)
  - `LD HL, (addr)` - 16 T-states (vs 38 for IX-indexed)
  - No IX register overhead
  - Smaller and faster code

### Recursive Functions  
- Use **IX-relative addressing** (e.g., `LD HL, (IX-6)`)
- Required for proper stack frame management
- Each recursive call gets its own stack frame
- Slightly slower but necessary for correctness

### Example Comparison

**Non-recursive (optimized):**
```asm
add:
; Using absolute addressing for locals (non-recursive)
    LD HL, ($F006)    ; 16 T-states
    LD ($F008), HL    ; 16 T-states
```

**Recursive (traditional):**
```asm
sum_tail:
; Using IX-relative addressing for locals (recursive)
    PUSH IX
    LD IX, SP
    LD L, (IX-6)      ; 19 T-states
    LD H, (IX-5)      ; 19 T-states
```

### Performance Impact
- Non-recursive functions see up to 58% faster local variable access
- No memory overhead - uses existing RAM
- Recursive functions maintain correctness with traditional approach

This optimization is automatically applied by the compiler based on function analysis.