; MinZ generated code
; Generated: 2026-02-10 22:21:03


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.wO77fsrWvv.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; @sizeof = 2 bytes
    LD A, 2
    LD L, A         ; Store to HL (low byte)
    ; store s, r2
    LD HL, 2      ; Constant
    LD ($F002), HL
    ; return
    RET

    END


; Assembly peephole optimization: no patterns matched