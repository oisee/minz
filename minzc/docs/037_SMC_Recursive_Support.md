# SMC with Recursive Support

## The Insight

SMC (Self-Modifying Code) CAN support recursion efficiently by saving just the immediate parameter values on entry and restoring them on exit!

## How It Works

### Traditional Recursive Function (Stack-based)
```asm
factorial:
    PUSH IX
    LD IX, SP
    ; Load parameter from stack
    LD A, (IX+4)    ; 19 T-states
    ; ... work ...
    POP IX
    RET
```

### SMC Recursive Function (Optimized)
```asm
factorial:
    ; Save SMC parameters
    LD HL, (factorial_n_imm)
    PUSH HL         ; Save just 2 bytes!
    
factorial_n:
    LD A, #00       ; SMC parameter (7 T-states)
factorial_n_imm EQU factorial_n+1
    
    ; ... work ...
    
    ; Before recursive call, patch new value
    LD A, new_value
    LD (factorial_n_imm), A
    CALL factorial
    
    ; Restore SMC parameters
    POP HL
    LD (factorial_n_imm), HL
    RET
```

## Advantages

1. **Faster parameter access** - 7 T-states (immediate) vs 19 T-states (stack)
2. **Minimal stack usage** - Only save actual parameter bytes
3. **No register pressure** - Parameters load directly to destination
4. **Cache friendly** - Code and data in same location

## When to Use

### Perfect for:
- Functions with 1-3 small parameters
- Moderate recursion depth
- Performance-critical recursive algorithms

### Not suitable for:
- Many parameters (stack overhead grows)
- Very deep recursion (still uses stack)
- ROM execution (can't modify code)

## Implementation Strategy

```minz
@smc_recursive
fun factorial(n: u8) -> u16 {
    // Compiler generates:
    // 1. Save immediate values on entry
    // 2. Normal SMC parameter access
    // 3. Restore immediates on exit
    
    if n <= 1 {
        return 1;
    }
    return n * factorial(n - 1);
}
```

## Performance Comparison

| Method | Parameter Access | Stack Usage | Setup Overhead |
|--------|-----------------|-------------|----------------|
| Traditional | 19 T-states | 2 bytes + frame | ~40 T-states |
| SMC Recursive | 7 T-states | 2 bytes only | ~20 T-states |
| Pure SMC | 7 T-states | 0 bytes | 0 T-states |

## Real Example: Fibonacci

```asm
fib:
    ; Save both SMC parameters (4 bytes total)
    LD HL, (fib_n_imm)
    PUSH HL
    LD HL, (fib_cache_imm)
    PUSH HL
    
fib_n:
    LD A, #00       ; n parameter
fib_n_imm EQU fib_n+1

fib_cache:
    LD HL, #0000    ; cache parameter  
fib_cache_imm EQU fib_cache+1
    
    ; Check base cases
    CP 2
    JR NC, recurse
    ; Base case: return n
    LD L, A
    LD H, 0
    JR done
    
recurse:
    ; Calculate fib(n-1)
    DEC A
    LD (fib_n_imm), A
    CALL fib
    PUSH HL         ; Save fib(n-1)
    
    ; Calculate fib(n-2)
    LD A, (fib_n_imm)
    DEC A
    LD (fib_n_imm), A
    CALL fib
    
    ; Add results
    POP DE
    ADD HL, DE
    
done:
    ; Restore SMC parameters
    POP DE
    LD (fib_cache_imm), DE
    POP DE
    LD (fib_n_imm), DE
    RET
```

This combines the **speed of SMC** with **recursion support** - truly the best of both worlds!