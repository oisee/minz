; ============================================================
; Che Guevara ZX Spectrum intro (BB-style layered LFSR)
; 64 layers × XOR random points on full 128x96 screen
; Each layer: LFSR seed (16-bit) + point count (8-bit)
;
; Assemble: mza che_intro.asm -o che_intro.bin
; Run:      mze --target spectrum che_intro.bin
; ============================================================

    ORG $8000

NLAYERS EQU 64

start:
    ; Clear screen
    ld  hl, $4000
    ld  de, $4001
    ld  bc, $17FF
    ld  (hl), 0
    ldir                    ; 13 bytes

    ; IX = layer table
    ld  ix, layer_table     ; 4 bytes
    ld  c, NLAYERS          ; 2 bytes = total 19

outer:
    ; Load seed → DE, points → B
    ld  d, (ix+1)           ; seed high
    ld  e, (ix+0)           ; seed low
    ld  b, (ix+2)           ; num points

    ; Init HL = $BEEF (fixed, CUDA uses seg_id*13+BEEF but for layer 0 it's BEEF)
    ; Actually for layered search CUDA uses: (seed<<16) | (layer_idx*7+0x1337)
    ; We need per-layer init. Store as 4th byte? Or compute.
    ; Simplest: HL = $1337 + layer_offset. But layer_offset varies.
    ; Let's use (ix+3) for init_lo, compute init from layer index.
    ; OR: just use fixed HL=$1337, and re-search with that init.

    ; For now use per-layer init from table (4 bytes per layer)
    ld  h, (ix+3)           ; init high
    ld  l, (ix+2)           ; wait, (ix+2) is points count!

    ; Restructure: table = seed_lo, seed_hi, init_lo, init_hi, points
    ; 5 bytes per layer... too much. 64*5=320 data + code = too big

    ; COMPROMISE: use fixed init HL for all layers, re-run CUDA search with same init
    ; For now: HL = (layer_index * 7 + $1337), compute from counter
    ;   layer_index = NLAYERS - C
    ;   HL = (NLAYERS - C) * 7 + $1337
    ; This is annoying to compute on Z80. Let's just use HL = $1337 for all.
    ; The images won't match but let's see if the LFSR produces anything recognizable.

    ld  hl, $1337           ; fixed init (we'll re-search with this)

    push bc                 ; save layer counter

    ; Warmup: 8 LFSR steps
    ld  a, 8
warmup:
    push af
    call lfsr_step
    pop af
    dec a
    jr  nz, warmup

    pop bc                  ; B = points, C = layer counter
    push bc                 ; save again

inner:
    call lfsr_step

    ; Save LFSR state
    push de
    push hl
    push bc

    ; Extract X = L & $7F (0..127), Y = H mod 96
    ld  a, l
    and $7F
    ld  e, a                ; E = X

    ld  a, h
    and $7F
    cp  96
    jr  c, y_ok
    sub 96
y_ok:
    ld  d, a                ; D = Y

    ; Screen address from (X=E, Y=D)
    ld  a, e
    srl a
    srl a
    srl a
    ld  l, a                ; L = X/8

    ld  a, d
    and 7
    or  $40
    ld  h, a                ; H = $40 | (Y&7)

    ld  a, d
    and $38
    rlca
    rlca
    or  l
    ld  l, a                ; L |= (Y&$38)<<2

    ld  a, d
    cp  64
    jr  c, no_third
    set 3, h                ; third of screen
no_third:

    ; Bit mask: $80 >> (X & 7)
    ld  a, e
    and 7
    jr  z, no_shift
    ld  c, a
    ld  a, $80
shift:
    srl a
    dec c
    jr  nz, shift
    jr  do_xor
no_shift:
    ld  a, $80
do_xor:
    xor (hl)
    ld  (hl), a

    ; Restore LFSR state
    pop bc
    pop hl
    pop de

    djnz inner

    ; Advance IX to next layer entry (3 bytes: seed_lo, seed_hi, npoints)
    ld  de, 3
    add ix, de

    pop bc                  ; C = layer counter
    dec c
    jr  nz, outer

    ; Done
    di
    halt

; ============================================================
; LFSR subroutine: 32-bit Galois LFSR in DEHL
; ============================================================
lfsr_step:
    srl d
    rr  e
    rr  h
    rr  l
    ret nc
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

; ============================================================
; Layer table: 64 × 3 bytes (seed_lo, seed_hi, npoints) = 192 bytes
; Re-searched with fixed init HL=$1337 for Z80 compatibility
; ============================================================
layer_table:
    DW $C6A1
    DB 128
    DW $DE22
    DB 126
    DW $A455
    DB 124
    DW $6CCA
    DB 122
    DW $6CCA
    DB 120
    DW $6CCA
    DB 118
    DW $6CCA
    DB 116
    DW $6CCA
    DB 114
    DW $43AD
    DB 112
    DW $2D28
    DB 110
    DW $2D28
    DB 108
    DW $2D28
    DB 106
    DW $CE7A
    DB 104
    DW $0C20
    DB 102
    DW $F88E
    DB 100
    DW $40B0
    DB 98
    DW $6370
    DB 48
    DW $3498
    DB 47
    DW $70C8
    DB 46
    DW $E447
    DB 45
    DW $D595
    DB 44
    DW $1840
    DB 43
    DW $1F56
    DB 42
    DW $5E92
    DB 41
    DW $3449
    DB 40
    DW $0BE0
    DB 39
    DW $2968
    DB 38
    DW $83C6
    DB 37
    DW $894A
    DB 36
    DW $5C3A
    DB 35
    DW $B662
    DB 34
    DW $D337
    DB 33
    DW $744F
    DB 24
    DW $99F1
    DB 23
    DW $C176
    DB 23
    DW $060D
    DB 22
    DW $56A7
    DB 22
    DW $3E4D
    DB 21
    DW $2FCD
    DB 21
    DW $4AB5
    DB 20
    DW $9475
    DB 20
    DW $3A0F
    DB 19
    DW $5E9C
    DB 19
    DW $2B1C
    DB 18
    DW $6EF7
    DB 18
    DW $0FD2
    DB 17
    DW $AED2
    DB 17
    DW $5EB5
    DB 16
    DW $7CE5
    DB 16
    DW $4BE5
    DB 15
    DW $0BEA
    DB 15
    DW $33EC
    DB 14
    DW $7D0D
    DB 14
    DW $0613
    DB 13
    DW $13F0
    DB 13
    DW $96CE
    DB 12
    DW $4CCE
    DB 12
    DW $091E
    DB 11
    DW $2A8D
    DB 11
    DW $5364
    DB 10
    DW $B494
    DB 10
    DW $15C5
    DB 9
    DW $0176
    DB 9
    DW $02EA
    DB 8
