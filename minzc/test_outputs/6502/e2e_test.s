; MinZ 6502 generated code
; Generated: 2025-08-06 12:38:56

    * = $0800

; Function: tests.minz.e2e_test.main
tests.minz.e2e_test.main:
    lda #$2A      ; r2 = 42
    sta x        ; store x
    lda #$0A      ; r4 = 10
    sta y        ; store y
    lda x        ; r6 = x
    lda y        ; r7 = y
    ; r8 = r6 + r7 (needs register allocation)
    clc
    adc $00        ; placeholder
    sta sum        ; store sum
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
