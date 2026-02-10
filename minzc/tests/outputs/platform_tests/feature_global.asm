; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.eRN7Q1GdDp.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; Inlined from tmp.eRN7Q1GdDp.inc
    LD HL, ($F000)
    ; Inlined from tmp.eRN7Q1GdDp.inc
    LD ($F000), HL
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.eRN7Q1GdDp.inc
inc:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r1 = load tmp.eRN7Q1GdDp.g
    LD HL, ($F000)
    ; store g, r1
    LD ($F000), HL
    ; return
    RET

; Data section (follows code contiguously)

tmp.eRN7Q1GdDp.g:
    DW 0

    END


; Assembly peephole optimization: no patterns matched