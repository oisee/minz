; Basic Z80 instruction test
    ORG $8000

; Simple instructions
    NOP
    LD A, 42
    LD B, A
    LD HL, $1234
    ADD A, B
    SUB 10
    AND $0F
    OR B
    XOR A
    CP 0
    INC A
    DEC B
    INC HL
    DEC HL
    
; Jumps and calls
start:
    JP start
    JR $+2
    CALL subroutine
    RET
    
subroutine:
    PUSH AF
    PUSH BC
    PUSH DE
    PUSH HL
    ; Do something
    POP HL
    POP DE
    POP BC
    POP AF
    RET
    
; Conditional jumps
    JP Z, start
    JP NZ, start
    JP C, start
    JP NC, start
    JR Z, $+2
    JR NZ, $+2
    RET Z
    RET NZ
    
; I/O
    IN A, ($FE)
    OUT ($FE), A
    
; Block operations
    LDIR
    LDDR
    CPIR
    
; End
    HALT