; MinZ 6502 generated code with zero-page optimization
; SMC/TSMC optimizations enabled

    * = $0800

; Zero Page Allocation Map:
; $00-$7F: Virtual Registers
;   $03: r6
;   $04: r3
;   $00: r2
;   $01: r4
;   $02: r5
; $80-$9F: SMC Parameters
;   $80: n
; $A0-$BF: TSMC Anchors

; Function: simple_tail_test.countdown
; SMC enabled - parameters in zero page
simple_tail_test_countdown:
simple_tail_test_countdown_param_n = $80  ; SMC parameter in zero page

    ; Load from anchor n$imm0
    ; TODO: TRUE_SMC_LOAD
    ; Tail recursion loop start
simple_tail_test.countdown_tail_loop:
    ; TODO: TEST
    lda $01        ; load r4
    beq else_1         ; jump if false
    lda #$00
    sta $02        ; r5 = 0
    lda $02        ; load r5
    ; return
else_1:
    ; Load from anchor n$imm0
    ; TODO: TRUE_SMC_LOAD
    ; Tail recursion optimized to loop
    jmp simple_tail_test.countdown_tail_loop
    rts

; Function: simple_tail_test.main
; SMC enabled - parameters in zero page
simple_tail_test_main:

    jsr countdown
    sta $04        ; r3 = result
    ; return
    brk        ; End program

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64:
    jsr $FFD2      ; CHROUT
    rts

    ; For Apple II:
    ; jsr $FDED    ; COUT
    ; rts
