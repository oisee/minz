; MinZ 6502 generated code
; Generated: 2025-08-05 11:41:24

    * = $0800

; Function: test_mir_compilation.main
test_mir_compilation.main:
    rts        ; Return

; Function: test_mir_compilation.add
test_mir_compilation.add:
    ; TODO: LOAD_PARAM
    ; TODO: LOAD_PARAM
    ; r5 = r3 + r4 (needs register allocation)
    clc
    adc $00        ; placeholder
    ; return
    rts        ; Return

; Helper routines
print_char:
    ; Platform-specific character output
    ; For C64: sta $FFD2
    ; For Apple II: jsr $FDED
    rts
