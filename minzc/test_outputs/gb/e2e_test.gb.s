; MinZ Game Boy generated code
; Generated: 2025-08-06 12:38:56
; Target: Sharp LR35902 (Game Boy CPU)
; Note: No shadow registers or IX/IY on GB

; Using RGBDS assembler syntax


; Code section
SECTION "Code", ROM0[$0150]


; Function: tests.minz.e2e_test.main
; SMC enabled
tests.minz.e2e_test.main:
    LD A, 42
    ; Store to r2
    ; Store r2 to var x
    LD A, 10
    ; Store to r4
    ; Store r4 to var y
    ; Load var x to r6
    ; Load var y to r7
    ; ADD r6 + r7 -> r8
    ; TODO: Implement register allocation
    ; Store r8 to var sum
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
