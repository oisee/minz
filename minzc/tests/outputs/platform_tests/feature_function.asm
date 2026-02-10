; MinZ generated code
; Generated: 2026-02-10 20:17:09


; Code section
    ORG $8000

; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.xQ5qkdCxgo.main
main:
; IsSMCDefault=true, IsSMCEnabled=true
; Using absolute addressing for locals (SMC style)
    ; return
    RET
; Using hierarchical register allocation (physical → shadow → memory)

; Function: tmp.xQ5qkdCxgo.add$u8$u8
tmp.xQ5qkdCxgo.add$u8$u8:
; TRUE SMC function with immediate anchors
add_u8_u8_param_a$immOP:
    LD A, 0        ; a anchor (will be patched)
add_u8_u8_param_a$imm0 EQU add_u8_u8_param_a$immOP+1
add_u8_u8_param_b$immOP:
    LD B, 0        ; b anchor (will be patched)
add_u8_u8_param_b$imm0 EQU add_u8_u8_param_b$immOP+1
    ; r5 = r3 + r4
    LD D, H
    LD E, L
    ADD HL, DE
    ; return r5
    RET

; TRUE SMC PATCH-TABLE
; Format: DW anchor_addr, DB size, DB param_tag
PATCH_TABLE:
    DW add_u8_u8_param_a$imm0           ; tmp.xQ5qkdCxgo.add$u8$u8.a
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW add_u8_u8_param_b$imm0           ; tmp.xQ5qkdCxgo.add$u8$u8.b
    DB 1              ; Size in bytes
    DB 0              ; Reserved for param tag
    DW 0              ; End of table
PATCH_TABLE_END:

    END


; Assembly peephole optimization: no patterns matched