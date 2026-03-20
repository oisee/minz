; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Z_PRG_AgonBits_Lessons/tests/sprite.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: Agon Light sprite management program.
;              Demonstrates: MOSCALL macro, keyboard matrix polling via IX,
;              RST.LIL calls, VDP sprite protocol, subroutine calls.

    .assume adl=1
    .org $40000

    jp start_here

    .align 64
    .db "MOS",0,1

start_here:
    push af
    push bc
    push de
    push ix
    push iy

    ld hl, VDUdata
    ld bc, endVDUdata - VDUdata
    rst.lil $18                         ; Send VDP data

    ld a, $08
    rst.lil $08                         ; Get sysvars in IX

WAIT_HERE:
    MOSCALL $1E                         ; Load IX with keymap address

    ld a, (ix + $0E)
    bit 0, a                            ; ESC key check
    jp nz, EXIT_HERE

    ld a, (ix + $06)
    bit 0, a                            ; '1' key
    call nz, setFrame0

    ld a, (ix + $06)
    bit 1, a                            ; '2' key
    call nz, setFrame1

    ld a, (ix + $0A)
    bit 4, a                            ; 'h' key
    call nz, hideSprite

    ld a, (ix + $0A)
    bit 1, a                            ; 's' key
    call nz, showSprite

    jp WAIT_HERE

setFrame0:
    ld hl, set0
    ld bc, endset0 - set0
    rst.lil $18
    ret

set0:
    .db 23, 27, 4, 0                    ; select sprite 0
    .db 23, 27, 10, 0                   ; select frame 0
endset0:

setFrame1:
    ld hl, set1
    ld bc, endset1 - set1
    rst.lil $18
    ret

set1:
    .db 23, 27, 4, 0
    .db 23, 27, 10, 1                   ; select frame 1
endset1:

hideSprite:
    ld hl, hide
    ld bc, endhide - hide
    rst.lil $18
    ret

hide:
    .db 23, 27, 4, 0
    .db 23, 27, 12                      ; hide current sprite
endhide:

showSprite:
    ld hl, show
    ld bc, endshow - show
    rst.lil $18
    ret

show:
    .db 23, 27, 4, 0
    .db 23, 27, 11                      ; show current sprite
endshow:

EXIT_HERE:
    pop iy
    pop ix
    pop de
    pop bc
    pop af
    ld hl,0
    ret

; VDP setup data
crystal:    EQU     0
star:       EQU     1
our_sprite: EQU     0

VDUdata:
    .db 23, 0, 192, 0                   ; non-scaled graphics

    .db 23, 27, 0, crystal
    .db 23, 27, 1
    .dw 16, 16

    .db 23, 27, 0, star
    .db 23, 27, 1
    .dw 16, 16

    ; Sprite setup
    .db 23, 27, 4, our_sprite           ; select sprite 0
    .db 23, 27, 5                       ; clear frames
    .db 23, 27, 6, crystal              ; add frame
    .db 23, 27, 6, star                 ; add frame
    .db 23, 27, 7, 1                    ; activate 1 sprite
    .db 23, 27, 11                      ; show sprite

    ; Move sprite
    .db 23, 27, 4, our_sprite
    .db 23, 27, 13
    .dw 150, 100

    .db 23, 27, 15                      ; update sprites in GPU

    ; Draw rectangle
    .db 18, 0, 45                       ; set colour
    .db 25, 69
    .dw 80,80
    .db 25, 101
    .dw 190,130

endVDUdata:
