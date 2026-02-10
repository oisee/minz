; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.gaqxP4Baeb.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r2 = 65
    LD A, 65
    ; Register 2 already in A
    ; r3 = call tmp.gaqxP4Baeb.mos_putchar$u8(r2)
    ; Call to tmp.gaqxP4Baeb.mos_putchar$u8 (args: 1)
    ; Explicit register mapping for extern function
    ; c in A
    RST $10    ; extern tmp.gaqxP4Baeb.mos_putchar$u8 (optimized from CALL)
    ; return
    POP DE
    POP BC
    RET

    END


; Assembly peephole optimization: 1 patterns applied