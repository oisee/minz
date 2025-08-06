; MinZ Game Boy generated code
; Generated: 2025-08-06 16:51:44
; Target: Sharp LR35902 (Game Boy CPU)
; Note: No shadow registers or IX/IY on GB

; Using RGBDS assembler syntax


; Code section
SECTION "Code", ROM0[$0150]


; Function: tests.backend_e2e.sources.control_flow.main
; SMC enabled
tests.backend_e2e.sources.control_flow.main:
    LD A, 0
    ; Store to r2
    ; Store r2 to var i
    ; TODO: LABEL
    ; Load var i to r3
    LD A, 10
    ; Store to r4
    ; TODO: LT
    ; TODO: JUMP_IF_NOT
    ; Load var i to r6
    LD A, 1
    ; Store to r7
    ; ADD r6 + r7 -> r8
    ; TODO: Implement register allocation
    ; Store r8 to var i
    ; TODO: JUMP
    ; TODO: LABEL
    ; Load var i to r9
    LD A, 10
    ; Store to r10
    ; TODO: EQ
    ; TODO: JUMP_IF_NOT
    LD A, 1
    ; Store to r13
    ; Store r13 to var result
    ; TODO: JUMP
    ; TODO: LABEL
    LD A, 0
    ; Store to r15
    ; Store r15 to var result
    ; TODO: LABEL
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
