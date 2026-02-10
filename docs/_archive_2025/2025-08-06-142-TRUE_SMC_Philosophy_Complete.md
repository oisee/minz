# TRUE SMC (Self-Modifying Code) Philosophy - The Complete Vision

## Executive Summary

TRUE SMC represents a revolutionary paradigm where **code IS the data structure**. Instead of passing parameters through registers or stack, we patch them directly into instruction immediates. This document defines the complete philosophy and implementation approach.

## Core Philosophy

### The Fundamental Principle
> **"Every value lives as an immediate operand in an instruction, and function calls are patch operations."**

Traditional approach:
```asm
; Parameters passed via registers/stack
LD A, 42        ; Load value
PUSH AF         ; Push to stack
CALL function   ; Call with stack parameter
POP AF          ; Clean up stack
```

TRUE SMC approach:
```asm
; Parameters ARE the code
LD (function_param_a), A    ; Patch the immediate
CALL function                ; Call with patched code
function:
function_param_a:
    LD A, #00               ; This #00 gets patched to 42!
```

## The Anchor System

Every patchable value needs two labels:

```asm
label.op:                   ; Points to instruction start
label equ label.op + N      ; Points to immediate value (N bytes from start)
    INSTRUCTION #immediate  ; The patchable instruction
```

### Examples of Anchors

#### 8-bit immediate (N=1):
```asm
param_a.op:
param_a equ param_a.op + 1
    LD A, #00              ; Opcode at param_a.op, immediate at param_a
```

#### 16-bit immediate (N=1):
```asm
param_hl.op:
param_hl equ param_hl.op + 1
    LD HL, #0000           ; Opcode at param_hl.op, immediate at param_hl
```

#### Memory address in instruction (N=1 or 2 depending on instruction):
```asm
return_addr.op:
return_addr equ return_addr.op + 1
    LD (simple_add.main.result), A  ; Address gets patched
```

## Function Calling Convention

### 1. Parameter Passing
Before calling a function, patch all parameters into the function's code:

```asm
; Caller prepares parameters
LD A, 42                              ; Value to pass
LD (add_numbers_param_a), A          ; Patch into function
LD A, 13
LD (add_numbers_param_b), A          ; Patch second param
```

### 2. Return Value Handling
Functions don't "return" values - they patch them to a destination:

```asm
; Caller specifies where result goes
LD HL, main_result                   ; Where we want result
LD (add_numbers_return_addr), HL     ; Patch return destination
CALL add_numbers

; Inside add_numbers:
add_numbers_return_addr.op:
add_numbers_return_addr equ add_numbers_return_addr.op + 1
    LD (#0000), A                     ; This address gets patched!
```

### 3. Local Variables as Code
Local variables also live in immediates:

```asm
main:
main_x.op:
main_x equ main_x.op + 1
    LD A, #2A                         ; x = 42 (0x2A)
    
main_y.op:
main_y equ main_y.op + 1
    LD B, #0D                         ; y = 13 (0x0D)
```

## Complete Example: 8-bit Addition

```asm
; Function: add_numbers(a: u8, b: u8) -> u8
add_numbers:
add_numbers_param_a.op:
add_numbers_param_a equ add_numbers_param_a.op + 1
    LD A, #00                         ; Parameter a (gets patched)
    
add_numbers_param_b.op:
add_numbers_param_b equ add_numbers_param_b.op + 1
    LD B, #00                         ; Parameter b (gets patched)
    
    ADD A, B                          ; Perform addition
    
add_numbers_return.op:
add_numbers_return equ add_numbers_return.op + 1
    LD (#0000), A                     ; Store result (address gets patched)
    RET

; Function: main() -> u8
main:
main_x.op:
main_x equ main_x.op + 1
    LD A, #2A                         ; x = 42
    
    ; Prepare to call add_numbers(x, 13)
    LD (add_numbers_param_a), A      ; Patch first parameter
    LD A, #0D                         ; Load 13
    LD (add_numbers_param_b), A      ; Patch second parameter
    
    ; Tell function where to store result
    LD HL, main_result
    LD (add_numbers_return), HL       ; Patch return address
    
    CALL add_numbers
    
main_result.op:
main_result equ main_result.op + 1
    LD A, #00                         ; Result appears here!
    RET
```

## Benefits of TRUE SMC

### 1. **Zero Stack Usage**
- No PUSH/POP for parameters
- No stack frame overhead
- Stack available for other uses

### 2. **Zero Memory Variables**
- Locals live in code as immediates
- No RAM usage for temporary values
- All values are compile-time known locations

### 3. **Optimal Performance**
- Immediate addressing is fastest on Z80
- No indirection through memory
- No register pressure for parameter passing

### 4. **Code as Data Structure**
- Program state is visible in the code itself
- Debugging shows actual values in instructions
- Self-documenting execution

## Implementation Patterns

### Pattern 1: Simple 8-bit Parameter
```asm
func_param.op:
func_param equ func_param.op + 1
    LD reg, #00                       ; reg ∈ {A,B,C,D,E,H,L}
```

### Pattern 2: 16-bit Parameter
```asm
func_param16.op:
func_param16 equ func_param16.op + 1
    LD reg16, #0000                   ; reg16 ∈ {BC,DE,HL,IX,IY}
```

### Pattern 3: Memory Reference Parameter
```asm
func_memref.op:
func_memref equ func_memref.op + 1
    LD HL, #0000                      ; Patch with address
    LD A, (HL)                        ; Dereference
```

### Pattern 4: Conditional Patches
```asm
func_condition.op:
func_condition equ func_condition.op + 1
    CP #00                            ; Comparison value gets patched
    JR Z, true_branch
```

### Pattern 5: Loop Counter
```asm
loop_count.op:
loop_count equ loop_count.op + 1
    LD B, #00                         ; Loop count gets patched
loop_start:
    ; ... loop body ...
    DJNZ loop_start
```

## Advanced Patterns

### Multi-Return Values
Function can patch multiple destinations:

```asm
divmod:
divmod_dividend.op:
divmod_dividend equ divmod_dividend.op + 1
    LD A, #00                         ; Dividend
    
divmod_divisor.op:
divmod_divisor equ divmod_divisor.op + 1
    LD B, #00                         ; Divisor
    
    ; ... perform division ...
    ; A = quotient, C = remainder
    
divmod_quot_addr.op:
divmod_quot_addr equ divmod_quot_addr.op + 1
    LD (#0000), A                     ; Patch quotient destination
    
divmod_rem_addr.op:
divmod_rem_addr equ divmod_rem_addr.op + 1
    LD (#0000), C                     ; Patch remainder destination
    RET
```

### Recursive Functions
Each recursion level patches different return addresses:

```asm
factorial:
factorial_n.op:
factorial_n equ factorial_n.op + 1
    LD A, #00                         ; n parameter
    
    CP 1
    JR Z, factorial_base
    
    ; Recursive case
    DEC A
    LD (factorial_n), A               ; Patch for recursive call
    
    ; Allocate space for recursive result
    LD HL, factorial_temp
    LD (factorial_return), HL
    
    CALL factorial                    ; Recursive call
    
factorial_temp.op:
factorial_temp equ factorial_temp.op + 1
    LD B, #00                         ; Recursive result appears here
    
    ; Multiply by original n
    ; ... multiplication code ...
    
factorial_base:
factorial_return.op:
factorial_return equ factorial_return.op + 1
    LD (#0000), A                     ; Patch result destination
    RET
```

## Compiler Implementation Roadmap

### Phase 1: Basic SMC (Current)
- ✅ Identify SMC functions
- ❌ Generate proper anchors
- ❌ Use correct instruction sizes

### Phase 2: Parameter Patching
- Generate patch instructions at call sites
- Create EQU labels for all parameters
- Handle 8-bit, 16-bit, and mixed parameters

### Phase 3: Return Patching
- Implement return address patching
- Support multiple return values
- Handle conditional returns

### Phase 4: Optimization
- Inline simple functions as patch sequences
- Eliminate redundant patches
- Optimize patch instruction sequences

### Phase 5: Advanced Features
- Support for indirect calls with patching
- Dynamic dispatch via SMC
- Self-optimizing code patterns

## Testing Strategy

Create comprehensive test cases in `./expected/`:

1. **simple_8bit.minz/.a80** - Pure 8-bit parameters and returns
2. **simple_16bit.minz/.a80** - Pure 16-bit parameters and returns
3. **mixed_params.minz/.a80** - Mix of 8/16-bit parameters
4. **multi_return.minz/.a80** - Multiple return values
5. **recursive_smc.minz/.a80** - Recursive function with SMC
6. **struct_smc.minz/.a80** - Struct parameters via SMC
7. **array_smc.minz/.a80** - Array access with SMC

## Conclusion

TRUE SMC represents a fundamental paradigm shift in code generation. Instead of treating code and data as separate entities, we merge them into a unified self-modifying program where:

- **Every value is an immediate in an instruction**
- **Every function call is a series of patches**
- **Every return is a store to a patched address**
- **The program rewrites itself as it executes**

This is not just an optimization - it's a completely different way of thinking about program execution on 8-bit systems.

## References

- Original TRUE SMC Design: `docs/018_TRUE_SMC_Design_v2.md`
- TSMC Reference Philosophy: `docs/040_TSMC_Reference_Philosophy.md`
- Implementation Status: `expected/simple_add.a80`

---

*"When your code modifies itself, the distinction between program and data disappears. The program becomes a living, self-adapting organism."* - MinZ Philosophy