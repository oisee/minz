; MinZ 6502 generated code
; Generated: 2025-08-06 12:14:00

    * = $0800

; Function: tests.minz.e2e_test.main
tests.minz.e2e_test.main:
    lda #$2A      ; r2 = 42
    sta local_2        ; store local_2
    lda #$0A      ; r4 = 10
    sta local_4        ; store local_4
    lda x        ; r6 = x
    lda y        ; r7 = y
    ; r8 = r6 + r7 (needs register allocation)
    clc
    adc $00        ; placeholder
    sta local_8        ; store local_8
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
