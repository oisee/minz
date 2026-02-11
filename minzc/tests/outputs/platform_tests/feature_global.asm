; MinZ generated code
; Generated: 2026-02-10 22:21:03


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.CvFovp8TZK.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; Inlined from tmp.CvFovp8TZK.inc
    LD HL, ($F000)
    ; Inlined from tmp.CvFovp8TZK.inc
    LD ($F000), HL
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.CvFovp8TZK.inc
inc:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; r1 = load tmp.CvFovp8TZK.g
    LD HL, ($F000)
    ; store g, r1
    LD ($F000), HL
    ; return
    RET

; Data section (follows code contiguously)

tmp.CvFovp8TZK.g:
    DW 0

    END


; Assembly peephole optimization: no patterns matched