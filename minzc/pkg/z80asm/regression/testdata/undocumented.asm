; Undocumented Z80 instruction test
    ORG $8000

; SLL (Shift Left Logical) - undocumented
    SLL A
    SLL B
    SLL C
    SLL D
    SLL E
    SLL H
    SLL L
    SLL (HL)
    
; SLL with index registers
    SLL (IX+0)
    SLL (IX+5)
    SLL (IX+127)
    SLL (IX-1)
    SLL (IX-128)
    
    SLL (IY+0)
    SLL (IY+10)
    SLL (IY-10)
    
; IX/IY half registers
    LD IXH, 0
    LD IXL, 255
    LD IYH, 128
    LD IYL, 64
    
    LD A, IXH
    LD B, IXL
    LD C, IYH
    LD D, IYL
    
    LD IXH, A
    LD IXL, B
    LD IYH, C
    LD IYL, D
    
    INC IXH
    INC IXL
    DEC IYH
    DEC IYL
    
; Arithmetic with IX/IY halves
    ADD A, IXH
    ADD A, IXL
    ADD A, IYH
    ADD A, IYL
    
    SUB IXH
    SUB IXL
    SBC A, IYH
    SBC A, IYL
    
    AND IXH
    AND IXL
    OR IYH
    OR IYL
    XOR IXH
    XOR IXL
    CP IYH
    CP IYL
    
; Undocumented ED instructions
    OUT (C), 0      ; ED 71
    
; Undocumented NEG opcodes
    DB $ED, $4C     ; NEG (alternate opcode)
    DB $ED, $54     ; NEG (alternate opcode)
    DB $ED, $5C     ; NEG (alternate opcode)
    DB $ED, $64     ; NEG (alternate opcode)
    DB $ED, $6C     ; NEG (alternate opcode)
    DB $ED, $74     ; NEG (alternate opcode)
    DB $ED, $7C     ; NEG (alternate opcode)