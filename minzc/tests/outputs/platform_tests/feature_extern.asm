; MinZ generated code
; Generated: 2026-02-10 22:21:03


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.RCRfmaFA5S.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    PUSH BC
    PUSH DE
    ; r2 = 65
    LD A, 65
    ; Register 2 already in A
    ; r3 = call tmp.RCRfmaFA5S.putc$u8(r2)
    ; Call to tmp.RCRfmaFA5S.putc$u8 (args: 1)
    ; Default register passing for extern function
    ; c in A (default)
    RST $10    ; extern tmp.RCRfmaFA5S.putc$u8 (optimized from CALL)
    ; return
    POP DE
    POP BC
    RET

    END


; Assembly peephole optimization: 1 patterns applied