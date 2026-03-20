; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Z_PRG_AgonBits_Lessons/tests/bitmap.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: Agon Light bitmap display program.
;              Demonstrates: RST.LIL $08/$18, VDP protocol, IX-based sysvars access.

    .assume adl=1                       ; ez80 ADL memory mode
    .org $40000                         ; load code here

    jp start_here                       ; jump to start of code

    .align 64                           ; MOS header
    .db "MOS",0,1

start_here:

    push af
    push bc
    push de
    push ix
    push iy

; Sending a VDU byte stream
    ld hl, VDUdata
    ld bc, endVDUdata - VDUdata
    rst.lil $18                         ; MOS API: send data to VDP

    ld a, $08
    rst.lil $08                         ; MOS API: get IX pointer to System Variables

WAIT_HERE:
    ld a, (ix + $05)                    ; get ASCII code of key pressed
    cp 27                               ; check for ESC
    jp z, EXIT_HERE

    jr WAIT_HERE

EXIT_HERE:
    pop iy
    pop ix
    pop de
    pop bc
    pop af
    ld hl,0
    ret

; VDP command data
crystal:    EQU     0
star:       EQU     1

VDUdata:
    .db 23, 0, 192, 0                   ; set to non-scaled graphics

    ; Load bitmap from data
    .db 23, 27, 0, crystal              ; select bitmap 0
    .db 23, 27, 1                       ; load bitmap data
    .dw 16, 16                          ; size 16x16

    .db 23, 27, 0, star                 ; select bitmap 1
    .db 23, 27, 1
    .dw 16, 16

    ; Draw bitmap
    .db 23, 27, 0, crystal
    .db 23, 27, 3                       ; draw at position
    .dw 80, 50

    ; Plot bitmap (VDP v2.1.0+)
    .db 23, 27, 0, star
    .db 25, $ED                         ; plot absolute
    .dw 150, 120

endVDUdata:
