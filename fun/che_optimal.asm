; che_optimal.asm — Optimal Che decoder (seg0 only, 576 points)
;
; 32-bit Galois LFSR, poly 0xB4BCD35C
; seg0: seed=0x8165, init=0x8165BEEF, blk=8, pts=576
; LFSR step: 72T avg. Full render: ~50K T = 14ms @3.5MHz
;
; Run:  mza fun/che_optimal.asm -o build/che_opt.bin
;       mze build/che_opt.bin

    ORG $8000

main:
    ; Clear screen
    ld  hl, $4000
    ld  de, $4001
    ld  bc, $17FF
    ld  (hl), 0
    ldir

    ; Init LFSR: DEHL = (0x8165 << 16) | 0xBEEF
    ld  d, $81
    ld  e, $65
    ld  h, $BE
    ld  l, $EF

    ; Warmup: 8 steps
    ld  a, 8
.warmup:
    push af
    call lfsr_step
    pop  af
    dec  a
    jr   nz, .warmup

    ; Render 576 points (IX = counter)
    ld  ix, 576

.render_loop:
    call lfsr_step

    push de
    push hl

    ; X = L AND $78 (aligned to 8, range 0..120)
    ld  a, l
    and $78
    rrca
    rrca
    rrca            ; A = byte column (0..15)
    ld  c, a        ; C = xbyte

    ; Y = E % 96, aligned to 8
    ld  a, e
.mod96:
    cp  96
    jr  c, .mod96_ok
    sub 96
    jr  .mod96
.mod96_ok:
    and $F8         ; align to 8 pixel rows
    ; Convert to screen address
    ; pixel_y = A (0,8,...88), char_row = A/8 (0..11)
    rrca
    rrca
    rrca            ; A = char_row (0..11)
    ld  b, a
    and $18         ; third bits
    or  $40         ; H = $40 | third
    ld  h, a
    ld  a, b
    and $07         ; row within third
    rrca
    rrca
    rrca            ; bits 7:5 = row * 32
    or  c           ; + xbyte
    ld  l, a
    ; HL = screen address

    ; XOR 8x8 solid block
    ld  b, 8
.xor8:
    ld  a, (hl)
    cpl
    ld  (hl), a
    inc h
    djnz .xor8

    pop hl
    pop de

    ; Decrement 16-bit counter
    dec ix
    ld  a, ixh
    or  ixl
    jr  nz, .render_loop

    ; Done — infinite loop (or halt)
    di
    halt

; ── LFSR-32 Galois (72T avg) ────────────────────────────
lfsr_step:
    srl d
    rr  e
    rr  h
    rr  l
    ret nc          ; no XOR if bit=0
    ld  a, d
    xor $B4
    ld  d, a
    ld  a, e
    xor $BC
    ld  e, a
    ld  a, h
    xor $D3
    ld  h, a
    ld  a, l
    xor $5C
    ld  l, a
    ret
