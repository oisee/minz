; MinZ 6502 generated code
; Generated: 2025-08-05 18:05:42

    * = $0800

; Function: test_6502_smc.add_smc
test_6502_smc.add_smc:
    ; TODO: TRUE_SMC_LOAD
    ; TODO: TRUE_SMC_LOAD
    ; r5 = r3 + r4 (needs register allocation)
    clc
    adc $00        ; placeholder
    ; return
    rts        ; Return

; Function: test_6502_smc.loop_test
test_6502_smc.loop_test:
loop_1:
    lda i        ; r6 = i
    ; TODO: TRUE_SMC_LOAD
    ; TODO: LT
    ; conditional jump (needs implementation)
    beq end_loop_2        ; if zero
    jsr add_smc        ; call add_smc
    jmp loop_1
end_loop_2:
    lda sum        ; r15 = sum
    ; return
    rts        ; Return

; Function: test_6502_smc.main
test_6502_smc.main:
    jsr loop_test        ; call loop_test
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
