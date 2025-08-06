; MinZ 6502 generated code
; Generated: 2025-08-05 11:33:24

    * = $0800

; Function: test_backend.main
test_backend.main:
    lda #$2A      ; r2 = 42
    sta local_2        ; store local_2
    lda #$E8      ; r4 = 1000 (low)
    ldx #$03      ; r4 = 1000 (high)
    sta local_4        ; store local_4
    lda #$00      ; r6 = 1048576 (low)
    ldx #$00      ; r6 = 1048576 (high)
    sta local_6        ; store local_6
    lda #$00      ; r8 = 0
    sta local_8        ; store local_8
    lda #<str_0      ; Load string address (low)
    ldx #>str_0      ; Load string address (high)
    ; TODO: print string
    lda x        ; r10 = x
    ; TODO: print u8 as decimal
    jsr print_u8
    ; TODO: print string direct
    lda y        ; r11 = y
    ; TODO: print u16 as decimal
    jsr print_u16
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
