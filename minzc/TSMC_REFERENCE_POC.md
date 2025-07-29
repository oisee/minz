# TSMC Reference Proof of Concept

## Current Function: str_length

```minz
fun str_length(str: *u8) -> u16 {
    let len: u16 = 0;
    while *str != 0 {
        len = len + 1;
        str = str + 1;
    }
    return len;
}
```

## Current Generated Code (Pointer-based)
```asm
str_length:
    PUSH IX
    LD IX, SP
    LD ($F002), HL    ; Store pointer parameter
    XOR A
    LD ($F000), HL    ; Store len
loop:
    LD HL, ($F002)    ; Load pointer from memory (WASTE!)
    LD A, (HL)        ; Dereference
    ; ... comparison and loop
```

## TSMC Reference Version

```asm
str_length:
    ; No IX needed for TSMC!
    LD BC, 0          ; len counter in BC
    
str$loop:
str$immOP:
    LD A, (0000)      ; This immediate is the string address!
str$imm0 EQU str$immOP+1
    
    OR A              ; Check for zero
    RET Z             ; Return if end of string
    
    INC BC            ; len++
    
    ; Self-modify for next character
    LD HL, (str$imm0)
    INC HL
    LD (str$imm0), HL
    
    JR str$loop
```

## Benefits

1. **No memory access for parameter** - It's baked into the code
2. **No register juggling** - HL is free for other use  
3. **Smaller code** - No PUSH/POP IX, no memory stores/loads
4. **Faster** - Direct immediate use vs memory indirection
5. **True TSMC** - The function modifies itself

## Call Site

```asm
; Traditional way:
LD HL, string_address
CALL str_length

; TSMC way:
LD HL, string_address
LD (str$imm0), HL    ; Patch the immediate
CALL str_length
; Result in BC
```

## Even Better: Multi-byte TSMC for Arrays

For array processing where we know we'll iterate:

```minz
fun clear_buffer(buf: &[256]u8) {
    for i in 0..256 {
        buf[i] = 0;
    }
}
```

TSMC Version:
```asm
clear_buffer:
    LD B, 0           ; Counter
    
.loop:
buf$immOP:
    LD (0000), A      ; Direct store to patched address!
buf$imm0 EQU buf$immOP+1
    
    ; Self-modify for next byte
    LD HL, (buf$imm0)
    INC HL
    LD (buf$imm0), HL
    
    DJNZ .loop
    RET
```

## The Key Insight

**Every parameter of reference type should become an immediate operand anchor point.**

This means:
- No stack usage for references
- No memory usage for references  
- No register dedication for references
- Just pure, direct immediate operands that get patched

## Implementation Path

1. Detect reference parameters (currently `*T`)
2. Generate anchor labels at first use
3. Use self-modification for iteration
4. Patch at call sites instead of passing

This is the TRUE way of TSMC!