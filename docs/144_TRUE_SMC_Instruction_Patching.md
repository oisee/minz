# TRUE SMC Advanced: Instruction Opcode Patching

## Executive Summary

Beyond patching operands, we can patch the **instruction opcodes themselves** to dynamically change program behavior with zero runtime overhead.

---

## 1. The Revolutionary Insight

Instead of checking flags at runtime, we **patch the instructions** to change behavior:

### Traditional Approach (Runtime Check):
```asm
return_mode:
    LD B, #00        ; Check flag
    OR B
    JR Z, register_return
    ; ... wasteful branching
```

### Advanced TRUE SMC (Instruction Patching):
```asm
func_return.op:
    NOP              ; This opcode gets patched!
    ; Could become:
    ; - RET (C9)     for immediate return
    ; - LD (nn), A   for store operation
    ; - XOR A        for clearing
    ; - OR A         for flag setting
```

---

## 2. Instruction Patching Patterns

### 2.1 Early Return Patching
```asm
; Function with patchable return point
compute:
    LD A, 42         ; Compute result
    
compute_store.op:
    NOP              ; Patched to either:
                     ; - RET (C9) for early return
                     ; - LD (addr), A (32 xx xx) for store
    NOP
    NOP
    RET              ; Fallback return
```

**Caller patches the behavior:**
```asm
; For immediate use (patch to RET):
LD A, #C9            ; RET opcode
LD (compute_store.op), A
CALL compute         ; Returns immediately with value in A

; For storage (patch to LD):
LD A, #32            ; LD (nn), A opcode
LD (compute_store.op), A
LD HL, storage_addr
LD (compute_store.op+1), HL
CALL compute         ; Stores then returns
```

### 2.2 XOR/LD Patching Pattern
```asm
; Patchable operation on accumulator
process_value:
process_op.op:
    XOR A            ; Could be patched to:
                     ; - LD A, n (3E xx)
                     ; - OR A (B7)
                     ; - AND n (E6 xx)
                     ; - XOR A (AF)
    RET
```

**Dynamic behavior change:**
```asm
; Make it load a value:
LD HL, #3E42         ; LD A, 42
LD (process_op.op), HL
CALL process_value   ; Returns with A=42

; Make it clear:
LD A, #AF            ; XOR A
LD (process_op.op), A
CALL process_value   ; Returns with A=0
```

---

## 3. Register Restoration Patching

### 3.1 Conditional Register Save
```asm
func_entry:
func_save.op:
    NOP              ; Patched to PUSH AF if needed
    NOP
    ; ... function body ...
func_restore.op:
    NOP              ; Patched to POP AF if needed
    NOP
    RET
```

**Caller decides preservation:**
```asm
; Need to preserve A:
LD HL, #F5C9         ; PUSH AF (F5) + filler
LD (func_save.op), HL
LD HL, #F1C9         ; POP AF (F1) + filler
LD (func_restore.op), HL

; Don't need preservation:
LD HL, #0000         ; NOPs
LD (func_save.op), HL
LD (func_restore.op), HL
```

---

## 4. Multi-Mode Return via Opcode Patching

### 4.1 The Ultimate Pattern
```asm
add_numbers:
    ; Compute result in A
    ADD A, B
    
add_return_sequence:
    ; 6 bytes of patchable space
    NOP              ; Byte 0-1: Could be LD (nn), A
    NOP              ; Byte 2-3: Address for LD
    NOP              ; Byte 4: Could be RET
    NOP              ; Byte 5: Padding
    
    ; This sequence can be patched to:
    ; 1. "RET + 5 NOPs" - Immediate return
    ; 2. "LD (addr), A + RET" - Store and return
    ; 3. "LD B, A + RET" - Transfer to B
    ; 4. "PUSH AF + RET" - Stack return
```

### 4.2 Patching Templates
```asm
; Template 1: Immediate return
IMMEDIATE_RETURN_PATCH:
    DB #C9, #00, #00, #00, #00, #00  ; RET + padding

; Template 2: Store to address
STORE_RETURN_PATCH:
    DB #32, LOW(addr), HIGH(addr), #C9, #00, #00  ; LD (addr), A + RET

; Template 3: Register transfer
REGISTER_B_PATCH:
    DB #47, #C9, #00, #00, #00, #00  ; LD B, A + RET

; Apply template:
LD HL, IMMEDIATE_RETURN_PATCH
LD DE, add_return_sequence
LD BC, 6
LDIR                 ; Copy template to function
```

---

## 5. Conditional Execution via Patching

### 5.1 Skip Computation Pattern
```asm
expensive_calc:
calc_enabled.op:
    NOP              ; Patched to RET to skip
    ; ... expensive computation ...
    RET
```

**Enable/Disable at runtime:**
```asm
; Disable computation:
LD A, #C9            ; RET opcode
LD (calc_enabled.op), A

; Enable computation:
LD A, #00            ; NOP opcode
LD (calc_enabled.op), A
```

### 5.2 Dynamic Jump Tables
```asm
dispatch:
dispatch_vector.op:
    JP 0000          ; Patched to different targets
```

**Change behavior:**
```asm
; Route to handler A:
LD HL, handler_a
LD (dispatch_vector.op+1), HL

; Route to handler B:
LD HL, handler_b
LD (dispatch_vector.op+1), HL
```

---

## 6. Performance Analysis

### Traditional Dual-Mode (Flag Check):
```
LD B, flag     - 7 T-states
OR B           - 4 T-states  
JR Z, label    - 12/7 T-states
Total: 23-18 T-states overhead
```

### Instruction Patching:
```
Patching: One-time cost
Execution: 0 T-states overhead!
```

**Benefits:**
- Zero runtime overhead after patching
- No branch misprediction
- Smaller code in hot path
- Dynamic optimization possible

---

## 7. Implementation Strategy

### 7.1 Compiler Support
```minz
@patchpoint(6) {
    // Reserve 6 bytes for patching
    @default: RET
    @template store_return: LD (addr), A; RET
    @template reg_return: RET
}
```

### 7.2 MIR Representation
```mir
patchpoint func_return, size=6 {
    default: ret
    template.store: store [r1], a; ret
    template.immediate: ret
}
```

### 7.3 Assembly Generation
```asm
func_return_patch_point:
    DS 6, #00        ; Reserve 6 bytes
func_return_templates:
    ; Template definitions follow
```

---

## 8. Advanced Patterns

### 8.1 Self-Modifying Loop
```asm
loop_start:
loop_body.op:
    INC A            ; This instruction morphs!
    DJNZ loop_start
    
    ; After first iteration, patch to DEC:
    LD A, #3D        ; DEC A opcode
    LD (loop_body.op), A
```

### 8.2 Progressive Optimization
```asm
; Function that optimizes itself
compute:
compute_counter.op:
    LD B, #10        ; Counts calls
    DEC B
    LD (compute_counter.op+1), B
    JR NZ, slow_path
    
    ; After 10 calls, patch to fast path:
    LD HL, fast_compute
    LD (compute+1), HL
    LD A, #C3        ; JP opcode
    LD (compute), A
    
slow_path:
    ; ... general computation ...
    RET
```

---

## 9. Safety Considerations

### 9.1 Patch Validation
```asm
; Verify patch before applying
verify_patch:
    LD A, (patch_source)
    CP #C9           ; Is it RET?
    JR Z, valid
    CP #00           ; Is it NOP?
    JR Z, valid
    ; Invalid patch - abort
    RET
valid:
    ; Apply patch
```

### 9.2 Atomic Patching
For multi-byte patches, ensure atomicity:
```asm
    DI               ; Disable interrupts
    ; Apply multi-byte patch
    LD (target), HL
    EI               ; Re-enable
```

---

## 10. Conclusion

Instruction opcode patching represents the **ultimate form of TRUE SMC**:

1. **Zero overhead** - No runtime checks
2. **Maximum flexibility** - Change behavior completely
3. **Self-optimizing** - Code improves itself
4. **Minimal size** - No branching code

This technique transforms the Z80 from a simple processor into a **dynamically reconfigurable computing fabric** where the program literally rewrites its own logic during execution.

---

## 11. Complete Example: Two Different Call Patterns

### MinZ Source Code
```minz
fn add_numbers(a: u8, b: u8) -> u8 {
    return a + b;
}

fn main() -> u8 {
    // Call 1: Immediate use - patch for register return
    let temp = add_numbers(10, 20) + 5;
    
    // Call 2: Storage - patch for memory store
    let stored_result = add_numbers(30, 40);
    
    return temp + stored_result;
}
```

### Generated Assembly with Instruction Patching

```asm
; Function with patchable return sequence
add_numbers:
    ; Load parameters (patched by caller)
add_param_a.op:
add_param_a equ add_param_a.op + 1
    LD A, #00           ; Parameter a

add_param_b.op:
add_param_b equ add_param_b.op + 1
    LD B, #00           ; Parameter b

    ; Compute result
    ADD A, B            ; Result in A
    
    ; 6-byte patchable return sequence
add_return_patch.op:
    NOP                 ; Byte 0: Patched opcode
    NOP                 ; Byte 1: Operand/padding
    NOP                 ; Byte 2: Operand/padding  
    NOP                 ; Byte 3: RET or padding
    NOP                 ; Byte 4: Padding
    NOP                 ; Byte 5: Padding
    RET                 ; Fallback (should never reach)

main:
    ; === CALL 1: Immediate Use Pattern ===
    ; Patch for register return (just RET)
    LD HL, immediate_return_template
    LD DE, add_return_patch.op
    LD BC, 6
    LDIR                ; Copy template
    
    ; Set up parameters
    LD A, 10
    LD (add_param_a), A
    LD A, 20
    LD (add_param_b), A
    
    CALL add_numbers    ; Returns with result in A (30)
    ADD A, 5            ; temp = 30 + 5 = 35
    LD (temp_value), A  ; Store temp
    
    ; === CALL 2: Storage Pattern ===
    ; Patch for memory store
    LD HL, store_return_template
    LD DE, add_return_patch.op
    LD BC, 6
    LDIR                ; Copy template
    
    ; Patch the storage address into the template
    LD HL, stored_result
    LD (add_return_patch.op + 1), HL
    
    ; Set up parameters
    LD A, 30
    LD (add_param_a), A
    LD A, 40
    LD (add_param_b), A
    
    CALL add_numbers    ; Stores result to stored_result, returns
    
    ; Final computation
    LD A, (temp_value)      ; A = 35
    LD B, (stored_result)   ; B = 70
    ADD A, B                ; A = 105
    RET                     ; Return final result

; Patch Templates
immediate_return_template:
    DB #C9, #00, #00, #00, #00, #00    ; RET + 5 NOPs

store_return_template:
    DB #32, #00, #00, #C9, #00, #00    ; LD (nn), A + RET + 2 NOPs
    ; The #00, #00 gets patched with actual address

; Variables
temp_value:        DS 1
stored_result:     DS 1

    END main
```

### Runtime Behavior Analysis

#### Call 1 (Immediate Use):
1. **Patch applied**: `add_return_patch.op` becomes `RET + padding`
2. **Function execution**:
   - Load A=10, B=20
   - ADD A, B → A=30
   - Execute patched RET → **immediate return with A=30**
   - No memory store overhead!
3. **Caller continues**: ADD A, 5 → temp=35

#### Call 2 (Storage):
1. **Patch applied**: `add_return_patch.op` becomes `LD (stored_result), A + RET`
2. **Address patched**: The `#00, #00` becomes address of `stored_result`
3. **Function execution**:
   - Load A=30, B=40  
   - ADD A, B → A=70
   - Execute patched `LD (stored_result), A` → **store to memory**
   - Execute `RET` → return
4. **Result**: Value 70 automatically stored, no register return needed

### Performance Comparison

#### Traditional Approach:
```asm
; Call 1: 
CALL add_numbers     ; 17 T-states
; Check return mode  ; 18-23 T-states
; Return in register ; 0 T-states
Total: 35-40 T-states

; Call 2:
CALL add_numbers     ; 17 T-states  
; Check return mode  ; 18-23 T-states
; Store to memory    ; 13 T-states
Total: 48-53 T-states
```

#### Instruction Patching:
```asm
; Call 1:
; Apply patch        ; 44 T-states (one-time)
CALL add_numbers     ; 17 T-states
; Execute RET        ; 10 T-states
Total: 27 T-states execution + 44 T-states setup

; Call 2:
; Apply patch        ; 44 T-states (one-time)
CALL add_numbers     ; 17 T-states
; Execute LD+RET     ; 23 T-states  
Total: 40 T-states execution + 44 T-states setup
```

**Net Savings**: 8-13 T-states per call after patching overhead!

---

## 12. Optimized Template Management

### Pre-computed Templates
```asm
; Keep templates in memory for fast copying
patch_templates:
immediate_template:   DB #C9, #00, #00, #00, #00, #00
store_template:       DB #32, #00, #00, #C9, #00, #00  
reg_b_template:       DB #47, #C9, #00, #00, #00, #00  ; LD B, A + RET
reg_c_template:       DB #4F, #C9, #00, #00, #00, #00  ; LD C, A + RET

; Fast template application
apply_immediate_patch:
    LD HL, immediate_template
    JP copy_patch

apply_store_patch:
    LD HL, store_template
    ; Fall through to copy_patch

copy_patch:
    LD DE, add_return_patch.op
    LD BC, 6
    LDIR
    RET
```

### Template Selection Based on Usage
```asm
main:
    ; Smart patch selection
    CALL apply_immediate_patch  ; For call 1
    ; ... first call ...
    
    CALL apply_store_patch      ; For call 2  
    ; Patch storage address
    LD HL, stored_result
    LD (add_return_patch.op + 1), HL
    ; ... second call ...
```

This demonstrates the **ultimate TRUE SMC pattern** where the same function behaves completely differently based on how it's patched, with zero runtime overhead for the choice!

---

## Examples Priority

1. **HIGH**: Return mode patching (RET vs LD)
2. **HIGH**: Register preservation patching
3. **MEDIUM**: Conditional execution patching
4. **LOW**: Progressive self-optimization

---

*Document Version: 1.0*
*Last Updated: August 2025*