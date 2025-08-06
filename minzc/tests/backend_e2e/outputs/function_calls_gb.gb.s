; MinZ Game Boy generated code
; Generated: 2025-08-06 16:51:44
; Target: Sharp LR35902 (Game Boy CPU)
; Note: No shadow registers or IX/IY on GB

; Using RGBDS assembler syntax


; Code section
SECTION "Code", ROM0[$0150]


; Function: tests.backend_e2e.sources.function_calls.add$u8$u8
; SMC enabled
tests.backend_e2e.sources.function_calls.add$u8$u8:
    ; TODO: LOAD_PARAM
    ; TODO: LOAD_PARAM
    ; ADD r3 + r4 -> r5
    ; TODO: Implement register allocation
    RET

; Function: tests.backend_e2e.sources.function_calls.main
; SMC enabled
tests.backend_e2e.sources.function_calls.main:
    LD A, 5
    ; Store to r2
    LD A, 3
    ; Store to r3
    LD A, 5
    ; Store to r4
    LD A, 3
    ; Store to r5
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    ; Store r6 to var result
    ; Load var result to r8
    ; Load var result to r9
    ; Load var result to r10
    ; Load var result to r11
    CALL tests.backend_e2e.sources.function_calls.add$u8$u8
    ; Store r12 to var doubled
    RET

; Print helpers for Game Boy
print_char:
    ; Wait for VBlank
    LD HL, $FF44  ; LY register
.wait_vblank:
    LD A, [HL]
    CP 144
    JR C, .wait_vblank
    ; Character in A, write to tile map
    ; This is a simplified version
    RET

print_hex:
    PUSH AF
    SWAP A
    CALL print_nibble
    POP AF
    CALL print_nibble
    RET

print_nibble:
    AND $0F
    CP 10
    JR C, .digit
    ADD A, 'A' - 10
    JR print_char
.digit:
    ADD A, '0'
    JR print_char
