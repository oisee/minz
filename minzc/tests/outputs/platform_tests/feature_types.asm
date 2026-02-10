; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.gMHHwXtMgU.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r2 = 42
    LD A, 42
    ; Register 2 already in A
    ; r4 = 1000
    LD HL, 1000
    ; Register 4 already in HL
    ; r6 = 1
    LD A, 1
    LD B, A         ; Store to physical register B
    ; store b, r6
    LD HL, 1      ; Constant
    LD ($F00A), HL
    ; store x, r2
    LD A, 42       ; Constant
    LD ($F002), A
    ; store y, r4
    LD HL, 1000      ; Constant
    LD ($F006), HL
    ; return
    RET

    END


; Assembly peephole optimization: no patterns matched