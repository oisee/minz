; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.fitN3fJAzw.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH DE
    ; Allocate struct P
    LD HL, -2
    ADD HL, SP
    LD SP, HL
    EX DE, HL
    LD HL, SP
    ; Register 2 already in HL
    ; r3 = 1
    LD A, 1
    ; Register 3 already in A
    ; r4 = 2
    INC A        ; Was LD A, $02 (val prop: $01+1)
    LD B, A         ; Store to physical register B
    ; store p, r2
    ; Register 2 already in HL
    LD ($F002), HL
    ; Store to P.x
    ; Register 2 already in HL
    PUSH HL
    LD HL, 1      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; Store to P.y
    ; Register 2 already in HL
    INC HL       ; Optimized: ADD HL,1 -> INC HL
    PUSH HL
    LD HL, 2      ; Constant
    POP DE
    LD A, L
    LD (DE), A
    INC DE
    LD A, H
    LD (DE), A
    ; return
    POP DE
    RET

    END


; Assembly peephole optimization: 2 patterns applied