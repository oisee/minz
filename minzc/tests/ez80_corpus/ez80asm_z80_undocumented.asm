; Source: https://github.com/envenomator/agon-ez80asm/blob/master/tests/Opcodes/tests/z80_undocumented.s
; License: MIT (https://github.com/envenomator/agon-ez80asm)
; Description: Undocumented Z80 instructions (also valid on eZ80).
;              IXH/IXL/IYH/IYL operations, SLL, DDCB/FDCB prefix instructions.
;              Hex comments show expected encoding bytes.

; Testing only the undocumented Z80 instructions.
.cpu Z80
    inc ixh ; DD 24
    inc ixl ; DD 2C
    inc iyh ; FD 24
    inc iyl ; FD 2C

    sll b ; DDCB 30
    sll c ; DDCB 31
    sll d ; DDCB 32
    sll e ; DDCB 33
    sll h ; DDCB 34
    sll l ; DDCB 35
    sll a ; DDCB 37

    in (c) ; ED 70
    out (c),0 ; ED 71

    ; IXH register loads
    ld ixh,b ; DD 60
    ld ixh,c ; DD 61
    ld ixh,d ; DD 62
    ld ixh,e ; DD 63
    ld ixh,ixh ; DD 64
    ld ixh,ixl ; DD 65
    ld ixh,a ; DD 67

    ld ixl,b ; FD 60
    ld ixl,c ; FD 61
    ld ixl,d ; FD 62
    ld ixl,e ; FD 63
    ld ixl,ixh ; FD 64
    ld ixl,ixl ; FD 65
    ld ixl,a ; FD 67

    ; IXH/IXL as source
    ld b,ixh ; 44
    ld b,ixl ; 45
    ld c,ixh ; 4C
    ld c,ixl ; 4D
    ld d,ixh ; 54
    ld d,ixl ; 55
    ld e,ixh ; 5C
    ld e,ixl ; 5D

    ld ixh,0 ; 26
    ld ixl,0 ; 2E
    ld a,ixh ; 7C
    ld a,ixl ; 7D

    inc ixh ; 24
    inc ixl ; 2C
    dec ixh ; 25
    dec ixl ; 2D

    ; Arithmetic with IXH/IXL
    add a,ixh ; 84
    add a,ixl ; 85
    adc a,ixh ; 8C
    adc a,ixl ; 8D
    sub ixh ; 94
    sub ixl ; 95
    sbc a,ixh ; 9C
    sbc a,ixl ; 9D
    and ixh ; A4
    and ixl ; A5
    xor ixh ; AC
    xor ixl ; AD
    or ixh ; B4
    or ixl ; B5
    cp ixh ; BC
    cp ixl ; BD

    ; Same for IYH/IYL
    inc iyh ; 24
    dec iyh ; 25
    ld iyh,0 ; 26
    inc iyl ; 2C
    dec iyl ; 2D
    ld iyl,0 ; 2E

    ld b,iyh ; 44
    ld b,iyl ; 45
    ld c,iyh ; 4C
    ld c,iyl ; 4D
    ld d,iyh ; 54
    ld d,iyl ; 55
    ld e,iyh ; 5C
    ld e,iyl ; 5D

    ld iyh,b ; 60
    ld iyh,c ; 61
    ld iyh,d ; 62
    ld iyh,e ; 63
    ld iyh,iyh ; 64
    ld iyh,iyl ; 65
    ld iyh,a ; 67
    ld iyl,b ; 68
    ld iyl,c ; 69
    ld iyl,d ; 6A
    ld iyl,e ; 6B
    ld iyl,iyh ; 6C
    ld iyl,iyl ; 6D
    ld iyl,a ; 6F

    add a,iyh ; 84
    add a,iyl ; 85
    adc a,iyh ; 8C
    adc a,iyl ; 8D
    ld a,iyh ; 7C
    ld a,iyl ; 7D
    sub iyh ; 94
    sub iyl ; 95
    sbc a,iyh ; 9C
    sbc a,iyl ; 9D
    and iyh ; A4
    and iyl ; A5
    xor iyh ; AC
    xor iyl ; AD
    or iyh ; B4
    or iyl ; B5
    cp iyh ; BC
    cp iyl ; BD

    ; DDCB prefix: shift/rotate (IX+d) with result to register
    rlc (ix+0),b ; DDCB 00
    rlc (ix+0),c ; DDCB 01
    rlc (ix+0),d ; DDCB 02
    rlc (ix+0),e ; DDCB 03
    rlc (ix+0),h ; DDCB 04
    rlc (ix+0),l ; DDCB 05
    rlc (ix+0),a ; DDCB 07

    rrc (ix+0),b ; DDCB 08
    rrc (ix+0),c ; DDCB 09
    rrc (ix+0),d ; DDCB 0A
    rrc (ix+0),e ; DDCB 0B

    rl (ix+0),b ; DDCB 10
    rl (ix+0),c ; DDCB 11
    rl (ix+0),d ; DDCB 12
    rl (ix+0),e ; DDCB 13

    rr (ix+0),b ; DDCB 18
    rr (ix+0),c ; DDCB 19
    rr (ix+0),d ; DDCB 1A
    rr (ix+0),e ; DDCB 1B

    sla (ix+0),b ; DDCB 20
    sla (ix+0),c ; DDCB 21
    sla (ix+0),d ; DDCB 22
    sla (ix+0),e ; DDCB 23

    sra (ix+0),b ; DDCB 28
    sra (ix+0),c ; DDCB 29
    sra (ix+0),d ; DDCB 2A
    sra (ix+0),e ; DDCB 2B

    sll (ix+0),b ; DDCB 30
    sll (ix+0),c ; DDCB 31
    sll (ix+0),d ; DDCB 32
    sll (ix+0),e ; DDCB 33
    sll (ix+0)   ; DDCB 36

    srl (ix+0),b ; DDCB 38
    srl (ix+0),c ; DDCB 39
    srl (ix+0),d ; DDCB 3A
    srl (ix+0),e ; DDCB 3B

    ; DDCB: RES/SET with result to register
    res 0,(ix+0),b ; DDCB 80
    res 0,(ix+0),c ; DDCB 81
    res 0,(ix+0),d ; DDCB 82
    res 0,(ix+0),e ; DDCB 83
    set 0,(ix+0),b ; DDCB C0
    set 0,(ix+0),c ; DDCB C1
    set 0,(ix+0),d ; DDCB C2
    set 0,(ix+0),e ; DDCB C3
