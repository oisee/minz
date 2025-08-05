; MinZ Game Boy generated code
; Generated: 2025-08-05 12:01:19
; Target: Sharp LR35902 (Game Boy CPU)
; Note: No shadow registers or IX/IY on GB

; Using RGBDS assembler syntax


; Code section
SECTION "Code", ROM0[$0150]


; Function: test_gb.main
; SMC enabled
test_gb.main:
    LD A, 42
    ; Store to r2
    ; Store r2 to var 
    LD A, 10
    ; Store to r4
    ; Store r4 to var 
    ; Load var x to r6
    ; Load var y to r7
    ; ADD r6 + r7 -> r8
    ; TODO: Implement register allocation
    ; Store r8 to var 
    ; Print string direct
    ; Load var sum to r9
    ; Print u8 as hex
    CALL print_hex
    ; Print string direct
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
