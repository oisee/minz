# MinZ Optimization Notes

## No IX Usage - Pure SMC Style

The MinZ compiler has been optimized to completely eliminate IX register usage, even for recursive functions!

### All SMC Functions (Recursive and Non-Recursive)
- Use **absolute addressing** for all locals (e.g., `LD HL, ($F000)`)
- Local variables stored at fixed addresses starting at 0xF000
- Benefits:
  - `LD (addr), HL` - 16 T-states (vs 38 for IX-indexed)
  - `LD HL, (addr)` - 16 T-states (vs 38 for IX-indexed)
  - No IX register setup/teardown overhead
  - Smaller and faster code

### Recursive Function Handling
- SMC parameters saved/restored via simple PUSH/POP
- No stack frames needed - just parameter context
- Much more efficient than traditional stack frames

### Example Comparison

**Before optimization (IX-based):**
```asm
sum_tail:
    PUSH IX           ; 15 T-states
    LD IX, SP         ; 10 T-states
    LD L, (IX-6)      ; 19 T-states
    LD H, (IX-5)      ; 19 T-states
    ; ... function body ...
    LD SP, IX         ; 10 T-states
    POP IX            ; 14 T-states
    RET
```

**After optimization (pure SMC):**
```asm
sum_tail:
; Using absolute addressing for locals (SMC style)
    LD HL, ($F006)    ; 16 T-states
    ; ... function body ...
    ; For recursive calls:
    LD HL, (sum_tail_param_n)    ; 16 T-states
    PUSH HL                       ; 11 T-states
    CALL sum_tail
    POP HL                        ; 10 T-states
    LD (sum_tail_param_n), HL     ; 16 T-states
    RET
```

### Performance Impact
- **All functions** now use fast absolute addressing
- **58% faster** local variable access
- **No IX overhead** - saves 87 T-states per function call/return
- **Recursive functions** are now as fast as non-recursive for local access
- Only overhead for recursion is parameter save/restore (much lighter than full stack frames)

This optimization is automatically applied by the compiler to all SMC functions.