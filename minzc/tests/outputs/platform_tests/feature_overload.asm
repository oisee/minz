; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.VUUZLqcoRj.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.VUUZLqcoRj.f$u8
tmp.VUUZLqcoRj.f$u8:
; TRUE SMC function with immediate anchors
f_u8_param_a$immOP:
    LD A, 0        ; a anchor (will be patched)
f_u8_param_a$imm0 EQU f_u8_param_a$immOP+1
    ; return r2
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.VUUZLqcoRj.f$u16
tmp.VUUZLqcoRj.f$u16:
; TRUE SMC function with immediate anchors
f_u16_param_a$immOP:
    LD HL, 0       ; a anchor (will be patched)
f_u16_param_a$imm0 EQU f_u16_param_a$immOP+1
    ; Register 2 already in HL
    ; return r2
    ; Register 2 already in HL
    RET

; TRUE SMC PATCH-TABLE
; Format: DW anchor_addr, DB size, DB param_tag
PATCH_TABLE:
    DW f_u8_param_a$imm0           ; tmp.VUUZLqcoRj.f$u8.a
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW f_u16_param_a$imm0           ; tmp.VUUZLqcoRj.f$u16.a
    DB 2              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW 0              ; End of table
PATCH_TABLE_END:

    END


; Assembly peephole optimization: no patterns matched