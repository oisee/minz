; MinZ 6502 generated code with zero-page optimization
; SMC/TSMC optimizations enabled

    * = $0800

; Zero Page Allocation Map:
; $00-$7F: Virtual Registers
;   $03: r6
;   $04: r7
;   $05: r8
;   $06: r11
;   $07: r15
;   $00: r3
;   $01: r4
;   $02: r5
; $80-$9F: SMC Parameters
;   $81: b
;   $82: count
;   $80: a
; $A0-$BF: TSMC Anchors

; Function: test_6502_smc.add_smc
; SMC enabled - parameters in zero page
test_6502_smc_add_smc:
test_6502_smc_add_smc_param_a = $80  ; SMC parameter in zero page
test_6502_smc_add_smc_param_b = $81  ; SMC parameter in zero page

    ; Load from anchor a$imm0
    ; TODO: TRUE_SMC_LOAD
    ; Load from anchor b$imm0
    ; TODO: TRUE_SMC_LOAD
    lda $00        ; load r3
    clc
    adc $01        ; + r4 (from zero page)
    sta $02        ; r5 = result
    ; return
    rts

; Function: test_6502_smc.loop_test
; SMC enabled - parameters in zero page
test_6502_smc_loop_test:
test_6502_smc_loop_test_param_count = $82  ; SMC parameter in zero page

loop_1:
    lda i
    sta $03        ; r6 = i
    ; Load from anchor count$imm0
    ; TODO: TRUE_SMC_LOAD
    cmp $04        ; compare with r7 (zero page)
    lda #0         ; assume false
    bcs +3         ; skip if >=
    lda #1         ; true if <
    sta $05        ; r8 = comparison result
    beq end_loop_2         ; jump if false
    jsr add_smc
    sta $06        ; r11 = result
    jmp loop_1
end_loop_2:
    lda sum
    sta $07        ; r15 = sum
    ; return
    rts

; Function: test_6502_smc.main
; SMC enabled - parameters in zero page
test_6502_smc_main:

    jsr loop_test
    sta $00        ; r3 = result
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
