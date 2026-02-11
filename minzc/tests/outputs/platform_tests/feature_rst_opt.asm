; MinZ generated code
; Generated: 2026-02-10 22:21:03


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.0dt0W1MyM4.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r1 = call tmp.0dt0W1MyM4.rst8
    ; Call to tmp.0dt0W1MyM4.rst8 (args: 0)
    RST $08    ; extern tmp.0dt0W1MyM4.rst8 (optimized from CALL)
    ; return
    POP DE
    POP BC
    RET

    END


; Assembly peephole optimization: no patterns matched