; MinZ 6502 generated code
; Generated: 2025-08-06 16:51:44

    * = $0800

; Function: tests.backend_e2e.sources.control_flow.main
tests.backend_e2e.sources.control_flow.main:
    lda #$00      ; r2 = 0
    sta i        ; store i
loop_1:
    lda i        ; r3 = i
    lda #$0A      ; r4 = 10
    ; TODO: LT
    ; conditional jump (needs implementation)
    beq end_loop_2        ; if zero
    lda i        ; r6 = i
    lda #$01      ; r7 = 1
    ; r8 = r6 + r7 (needs register allocation)
    clc
    adc $00        ; placeholder
    sta i        ; store i
    jmp loop_1
end_loop_2:
    lda i        ; r9 = i
    lda #$0A      ; r10 = 10
    ; TODO: EQ
    ; conditional jump (needs implementation)
    beq else_3        ; if zero
    lda #$01      ; r13 = 1
    sta result        ; store result
    jmp end_if_4
else_3:
    lda #$00      ; r15 = 0
    sta result        ; store result
end_if_4:
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
