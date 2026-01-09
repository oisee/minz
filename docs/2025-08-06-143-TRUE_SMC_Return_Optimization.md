# TRUE SMC Return Value Optimization Strategy

## Executive Summary

How to optimize function return values in TRUE SMC, especially for immediate use cases where the result is consumed right away without storing.

---

## 1. Current TRUE SMC Return Pattern

### Traditional TRUE SMC (as documented):
```asm
; Function patches where to store result
func_return.op:
func_return equ func_return.op + 1
    LD (0000), A        ; Address gets patched by caller
    RET

; Caller:
LD HL, storage_location
LD (func_return), HL    ; Patch where to store
CALL func
; Result is now at storage_location
```

**Problem**: This always requires a memory write even if we want to use the value immediately!

---

## 2. Proposed Hybrid Return Optimization

### 2.1 Dual Return Modes

Functions should support TWO return modes:
1. **Register Return** (default) - For immediate use
2. **Patched Return** - For storing to specific location

### 2.2 Implementation Approach

```asm
; Function with dual return capability
add_numbers:
    ; ... computation ...
    ; Result in A register
    
    ; Check if caller wants patched return
add_numbers_return_mode.op:
add_numbers_return_mode equ add_numbers_return_mode.op + 1
    LD B, #00           ; 00 = register return, 01 = patch return
    
    OR B                ; Check mode
    JR Z, register_return
    
patch_return:
add_numbers_return_addr.op:
add_numbers_return_addr equ add_numbers_return_addr.op + 1
    LD (0000), A        ; Store to patched address
    RET
    
register_return:
    ; A already contains result
    RET
```

---

## 3. Optimization Opportunities

### 3.1 Immediate Use Pattern
```minz
// MinZ code:
let x = add(5, 3);
let y = multiply(x, 2);  // x used immediately

// Optimized assembly:
; First call - register return
LD A, 5
LD (add_param_a), A
LD A, 3
LD (add_param_b), A
LD A, 00                ; Set register return mode
LD (add_return_mode), A
CALL add
; Result in A, use directly!

; Second call - use A directly
LD (multiply_param_a), A
LD A, 2
LD (multiply_param_b), A
CALL multiply
```

### 3.2 Store Pattern
```minz
// MinZ code:
result = add(5, 3);  // Store for later

// Assembly with patch return:
LD A, 5
LD (add_param_a), A
LD A, 3  
LD (add_param_b), A
LD A, 01                ; Set patch return mode
LD (add_return_mode), A
LD HL, result_location
LD (add_return_addr), HL
CALL add
```

---

## 4. Where to Implement This Optimization

### 4.1 MIR Level (Recommended)
**Best place** for this optimization - MIR can track value usage:

```mir
; MIR detects immediate use
r1 = call add(5, 3)     ; Returns in register
r2 = mul r1, 2          ; Direct register use

; MIR detects storage need
r1 = call add(5, 3)
store result, r1        ; Needs memory write
```

MIR optimizer can:
- Analyze if return value is used immediately
- Choose register vs patch return mode
- Eliminate unnecessary stores

### 4.2 Assembly Peephole (Secondary)
Can catch patterns like:
```asm
; Pattern to detect:
CALL func
LD (temp), A        ; Store result
LD A, (temp)        ; Load it back immediately

; Optimize to:
CALL func
; A already has result!
```

### 4.3 Semantic Analysis (Tertiary)
Could annotate AST with usage hints:
```go
type CallExpr struct {
    // ...
    ReturnUsage enum {
        Immediate    // Used in next expression
        Stored       // Assigned to variable
        Discarded    // Return value ignored
    }
}
```

---

## 5. Advanced Return Patterns

### 5.1 Multiple Return Values
```asm
; Function returns two values
divmod:
    ; Compute quotient in A, remainder in B
    
divmod_return_mode.op:
divmod_return_mode equ divmod_return_mode.op + 1
    LD C, #00           ; Bit 0: quotient, Bit 1: remainder
    
    BIT 0, C
    JR Z, skip_quot
divmod_quot_addr.op:
divmod_quot_addr equ divmod_quot_addr.op + 1
    LD (0000), A
skip_quot:
    
    BIT 1, C
    JR Z, skip_rem
divmod_rem_addr.op:
divmod_rem_addr equ divmod_rem_addr.op + 1
    LD (0000), B
skip_rem:
    RET
```

### 5.2 Conditional Return Optimization
```minz
// MinZ:
let x = flag ? compute() : 0;

// Optimized: Only patch if needed
CP flag, 0
JR Z, skip_call
CALL compute        ; Register return
JR store_x
skip_call:
XOR A              ; Zero in register
store_x:
; A has result either way
```

---

## 6. Implementation Roadmap

### Phase 1: MIR Analysis
1. Add usage tracking to MIR
2. Identify immediate-use patterns
3. Mark functions for register return

### Phase 2: Code Generation
1. Generate dual-mode return code
2. Add return mode patching
3. Optimize call sites

### Phase 3: Peephole Enhancement
1. Detect store-load patterns
2. Remove redundant memory operations
3. Optimize register flow

---

## 7. Performance Impact

### Traditional TRUE SMC:
- Store: 13 T-states (LD (addr), A)
- Load: 13 T-states (LD A, (addr))
- Total: 26 T-states overhead

### Optimized Register Return:
- Store: 0 T-states (already in register)
- Load: 0 T-states (already in register)
- Total: 0 T-states overhead!

**Savings: 26 T-states per immediate-use function call**

---

## 8. Example: Complete Optimization

### Before (Current TRUE SMC):
```asm
; Computing: (a + b) * c
CALL add            ; Result to memory
LD A, (add_result)  ; Load from memory (wasteful!)
LD (mul_param_a), A
CALL multiply
LD A, (mul_result)  ; Another load
```

### After (Optimized):
```asm
; Computing: (a + b) * c  
CALL add            ; Result in A
LD (mul_param_a), A ; Direct use
CALL multiply       ; Result in A
; Final result already in A!
```

---

## 9. Conclusion

The best approach is a **hybrid system**:
1. **MIR tracks usage patterns** and decides return mode
2. **Functions support both modes** via patched flag
3. **Peephole optimization** catches remaining inefficiencies
4. **Default to register return** for better performance

This maintains TRUE SMC philosophy while eliminating unnecessary memory operations for common patterns.

---

## Implementation Priority

1. **HIGH**: MIR usage analysis
2. **HIGH**: Dual-mode return generation
3. **MEDIUM**: Call-site optimization
4. **LOW**: Peephole patterns for cleanup

---

*Document Version: 1.0*
*Last Updated: August 2025*