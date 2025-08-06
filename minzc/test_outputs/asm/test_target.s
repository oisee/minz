; MinZ 6502 generated code
; Generated: 2025-08-05 11:22:53

    * = $0800

; Function: test_target.print_char
test_target.print_char:
    ; 6502: Platform-specific output
    LDA $00         ; Get parameter from zero page
    JSR $FFD2       ; C64 KERNAL CHROUT
    ; return
    rts        ; Return

; Function: test_target.main
test_target.main:
    lda #$41      ; r2 = 65
    sta local_2        ; store local_2
    lda #<str_0      ; Load string address (low)
    ldx #>str_0      ; Load string address (high)
    ; TODO: print string
    lda msg        ; r4 = msg
    jsr print_char        ; call print_char
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
