; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.yIRfFa4DIj.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r1 = call tmp.yIRfFa4DIj.rst8
    ; Call to tmp.yIRfFa4DIj.rst8 (args: 0)
    RST $08    ; extern tmp.yIRfFa4DIj.rst8 (optimized from CALL)
    ; return
    POP DE
    POP BC
    RET

    END


; Assembly peephole optimization: no patterns matched