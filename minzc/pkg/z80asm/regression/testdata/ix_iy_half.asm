; IX/IY half register test - comprehensive coverage
    ORG $8000

; Load immediate to IX/IY halves
    LD IXH, 0
    LD IXH, 1
    LD IXH, 127
    LD IXH, 128
    LD IXH, 255
    
    LD IXL, 0
    LD IXL, 1
    LD IXL, 127
    LD IXL, 128
    LD IXL, 255
    
    LD IYH, 0
    LD IYH, 1
    LD IYH, 127
    LD IYH, 128
    LD IYH, 255
    
    LD IYL, 0
    LD IYL, 1
    LD IYL, 127
    LD IYL, 128
    LD IYL, 255
    
; Register to register moves with IX halves
    LD A, IXH
    LD A, IXL
    LD B, IXH
    LD B, IXL
    LD C, IXH
    LD C, IXL
    LD D, IXH
    LD D, IXL
    LD E, IXH
    LD E, IXL
    LD H, IXH   ; Note: These are valid!
    LD H, IXL
    LD L, IXH
    LD L, IXL
    
; Register to register moves with IY halves
    LD A, IYH
    LD A, IYL
    LD B, IYH
    LD B, IYL
    LD C, IYH
    LD C, IYL
    LD D, IYH
    LD D, IYL
    LD E, IYH
    LD E, IYL
    LD H, IYH
    LD H, IYL
    LD L, IYH
    LD L, IYL
    
; Reverse direction
    LD IXH, A
    LD IXL, A
    LD IXH, B
    LD IXL, B
    LD IXH, C
    LD IXL, C
    LD IXH, D
    LD IXL, D
    LD IXH, E
    LD IXL, E
    LD IXH, H
    LD IXL, H
    LD IXH, L
    LD IXL, L
    
    LD IYH, A
    LD IYL, A
    LD IYH, B
    LD IYL, B
    LD IYH, C
    LD IYL, C
    LD IYH, D
    LD IYL, D
    LD IYH, E
    LD IYL, E
    LD IYH, H
    LD IYL, H
    LD IYH, L
    LD IYL, L
    
; IX half to IX half (yes, these work!)
    LD IXH, IXL
    LD IXL, IXH
    
; IY half to IY half
    LD IYH, IYL
    LD IYL, IYH
    
; Cross register moves (these should fail in a real assembler)
; But some assemblers might accept them
;   LD IXH, IYH   ; Usually not allowed
;   LD IXL, IYL   ; Usually not allowed
    
; INC/DEC operations
    INC IXH
    INC IXL
    INC IYH
    INC IYL
    DEC IXH
    DEC IXL
    DEC IYH
    DEC IYL
    
; All arithmetic operations
    ADD A, IXH
    ADD A, IXL
    ADD A, IYH
    ADD A, IYL
    
    ADC A, IXH
    ADC A, IXL
    ADC A, IYH
    ADC A, IYL
    
    SUB IXH
    SUB IXL
    SUB IYH
    SUB IYL
    
    SBC A, IXH
    SBC A, IXL
    SBC A, IYH
    SBC A, IYL
    
    AND IXH
    AND IXL
    AND IYH
    AND IYL
    
    XOR IXH
    XOR IXL
    XOR IYH
    XOR IYL
    
    OR IXH
    OR IXL
    OR IYH
    OR IYL
    
    CP IXH
    CP IXL
    CP IYH
    CP IYL