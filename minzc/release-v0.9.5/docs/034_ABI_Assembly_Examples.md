# ABI Assembly Code Examples

**Visual comparison of different calling conventions for the same function**

## Simple Function: `add(a: u8, b: u8) -> u8`

### 1. Register-based ABI (Fastest)
```asm
; Caller side:
LD A, 10        ; First param in A
LD E, 20        ; Second param in E  
CALL add_register

; Function implementation:
add_register:
    ADD A, E    ; Direct register operation (4 T-states)
    RET         ; Result already in A

; Total overhead: 0 cycles
; Parameter access: 4 T-states
```

### 2. SMC (Self-Modifying Code) ABI
```asm
; Caller side:
LD A, 10
LD (add_smc_param_a+1), A    ; Patch the immediate value
LD A, 20
LD (add_smc_param_b+1), A    ; Patch the immediate value
CALL add_smc

; Function implementation:
add_smc:
add_smc_param_a:
    LD A, #00           ; This byte is patched (was 10)
add_smc_param_b:
    ADD A, #00          ; This byte is patched (was 20)
    RET

; Total overhead: 14 T-states (patching)
; Parameter access: Immediate (7 T-states)
```

### 3. Stack-based ABI (Traditional)
```asm
; Caller side:
LD A, 20
PUSH AF         ; Push second param
LD A, 10  
PUSH AF         ; Push first param
CALL add_stack
POP AF          ; Clean stack
POP AF

; Function implementation:
add_stack:
    PUSH IX
    LD IX, SP       ; Setup frame pointer
    
    LD A, (IX+4)    ; Load first param (19 T-states)
    LD B, (IX+6)    ; Load second param (19 T-states)
    ADD A, B
    
    POP IX
    RET

; Total overhead: ~40 T-states (push/pop/frame setup)
; Parameter access: 38 T-states
```

### 4. Virtual Register ABI (Current Default)
```asm
; Caller side:
LD A, 10
LD ($F000), A   ; Store first param
LD A, 20
LD ($F002), A   ; Store second param
CALL add_virtual

; Function implementation:
add_virtual:
    LD A, ($F000)   ; Load first param (13 T-states)
    LD B, ($F002)   ; Load second param (13 T-states)
    ADD A, B
    RET

; Total overhead: 26 T-states (memory stores)
; Parameter access: 26 T-states
```

### 5. Shadow Register ABI (For Interrupts)
```asm
; Caller side (or interrupt entry):
LD A, 10
EX AF, AF'      ; Move to shadow A (4 T-states)
LD A, 20
LD B, A
EXX             ; Move B to shadow B' (4 T-states)
CALL add_shadow

; Function implementation:
add_shadow:
    EX AF, AF'      ; Get shadow A (4 T-states)
    EXX             ; Get shadow B (4 T-states)
    ADD A, B        ; In shadow register context
    EXX             ; Back to main (4 T-states)
    EX AF, AF'      ; Result in main A (4 T-states)
    RET

; Total overhead: 16 T-states (register bank switching)
; Parameter access: 4 T-states (once switched)
```

## Complex Function: Many Parameters

### Hybrid ABI (Optimal for many params)
```asm
; For: complex_calc(a: u8, b: u8, c: u16, d: u16, e: u8, f: u8) -> u16

; Caller side:
LD A, 1         ; a in A register
LD E, 2         ; b in E register  
LD HL, 300      ; c in HL register pair
; d, e, f go on stack
LD BC, 6
PUSH BC         ; Push f
LD BC, 5  
PUSH BC         ; Push e
LD BC, 400
PUSH BC         ; Push d
CALL complex_calc
; Clean stack
LD HL, 6
ADD HL, SP
LD SP, HL

; Function uses mix of register and stack access
complex_calc:
    ; A = param a (register - fast)
    ; E = param b (register - fast)
    ; HL = param c (register - fast)
    PUSH IX
    LD IX, SP
    ; (IX+4) = param d (stack - slower)
    ; (IX+6) = param e (stack - slower)
    ; (IX+8) = param f (stack - slower)
```

## Performance Comparison Table

| ABI Type | Call Overhead | Param Access | Total Cycles | Best Use Case |
|----------|--------------|--------------|--------------|---------------|
| Register | 0 | 4T | 4T | Small, hot functions |
| SMC | 14T | 7T | 21T | RAM-only, stable params |
| Shadow | 16T | 4T | 20T | Interrupts, parallel |
| Virtual | 26T | 26T | 52T | General purpose |
| Stack | 40T | 38T | 78T | Recursive, many params |
| Hybrid | 10-20T | 4-19T | Variable | Default optimal |

## Real-World Example: String Length

### With Register ABI
```asm
; strlen(str: *u8) -> u8
; Using register ABI: HL = string pointer

strlen_register:
    LD A, (HL)      ; Length byte (7 T-states)
    RET             ; Total: 7 T-states
```

### With Stack ABI
```asm
strlen_stack:
    PUSH IX
    LD IX, SP
    LD L, (IX+4)    ; Load pointer low
    LD H, (IX+5)    ; Load pointer high  
    LD A, (HL)      ; Length byte
    POP IX
    RET             ; Total: ~50 T-states (7x slower!)
```

## Memory Layout Comparison

### Stack-based Layout
```
High Memory
+-----------+
| param n   |  <- IX+8
| param 2   |  <- IX+6  
| param 1   |  <- IX+4
| ret addr  |  <- IX+2
| saved IX  |  <- IX
| local 1   |  <- IX-2
| local 2   |  <- IX-4
+-----------+
Low Memory     <- SP
```

### SMC Layout
```
Code Segment (RAM)
+-----------+
| LD A, XX  |  <- Patched with param 1
| ADD A, YY |  <- Patched with param 2
| RET       |
+-----------+
```

### Virtual Register Layout
```
Fixed Memory Region ($F000+)
+-----------+
| param 1   |  $F000
| param 2   |  $F002
| param 3   |  $F004
| temp 1    |  $F006
| temp 2    |  $F008
+-----------+
```

## Guidelines for ABI Selection

1. **Leaf Functions** (no calls): Use registers
2. **Recursive Functions**: Must use stack
3. **Interrupt Handlers**: Use shadow registers
4. **Hot Functions**: Prioritize registers
5. **Many Parameters**: Use hybrid approach
6. **ROM-based Code**: Cannot use SMC
7. **Coroutines**: Consider dual-stack approach

## Future: Adaptive ABI Example

```minz
// Compiler analyzes and chooses optimal ABI
fun process_data(data: *u8, len: u16) -> u16 {
    // Analysis:
    // - 2 parameters (fits in registers)
    // - Not recursive
    // - Called frequently (hot path)
    // Decision: Use register ABI (HL=data, DE=len)
    
    let sum: u16 = 0;
    for i in 0..len {
        sum = sum + data[i];
    }
    return sum;
}

// Generates:
process_data:  ; HL=data, DE=len
    PUSH BC
    LD BC, 0        ; sum = 0
loop:
    LD A, (HL)      ; data[i]
    ADD A, C
    LD C, A
    JR NC, no_carry
    INC B           ; Handle carry
no_carry:
    INC HL          ; data++
    DEC DE          ; len--
    LD A, D
    OR E
    JR NZ, loop
    
    LD H, B         ; Return sum in HL
    LD L, C
    POP BC
    RET
```

The adaptive ABI system will make MinZ generate code that's as efficient as hand-written assembly while maintaining high-level language benefits!