; ============================================================
; Che Guevara ZX Spectrum intro — incbin seeds variant
;
; Seeds table loaded from binary file instead of init_layers() code.
; 64 layers × 3 bytes = 192 bytes at 0xC000.
; Saves ~350 bytes of init code vs Nanz-compiled version.
;
; Assemble: mza che_intro_incbin.asm -o che_incbin.bin
; Run:      mze --target spectrum che_incbin.bin
; ============================================================

    ORG $8000

NLAYERS  EQU 64
INIT_VAL EQU $1337      ; warmup LFSR init value
SEEDS    EQU $C000      ; layer table: 64 × (seed_lo, seed_hi, points)

start:
    ; Clear screen ($4000..$57FF)
    ld  hl, $4000
    ld  de, $4001
    ld  bc, $17FF
    ld  (hl), 0
    ldir

    ; Copy seeds table to $C000
    ld  hl, seeds_data
    ld  de, SEEDS
    ld  bc, NLAYERS * 3
    ldir

    ; Render all 64 layers
    ld  ix, SEEDS
    ld  c,  NLAYERS
outer:
    ld  d, (ix+1)       ; seed_hi
    ld  e, (ix+0)       ; seed_lo
    ld  b, (ix+2)       ; points

    ; Warmup: 8 LFSR steps
    push bc
    ld  b, 8
warmup:
    call lfsr_step
    djnz warmup
    pop  bc

    ; Plot B points
plot_loop:
    call lfsr_step
    ; x = E % 128, y = D % 96
    ld  a, e
    and $7F             ; x = sl & 127
    ld  l, a            ; save x
    ld  a, d
    ; y % 96: subtract 96 while >= 96
.mod96:
    cp  96
    jr  c, .mod96_done
    sub 96
    jr  .mod96
.mod96_done:
    ld  h, a            ; save y
    call xor_pixel      ; H=y, L=x
    djnz plot_loop

    ld  bc, 3
    add ix, bc
    dec c
    jr  nz, outer

    ret

; ── LFSR step: DE = next state ────────────────────────
; In: DE = seed (D=hi, E=lo), init=$1337 hardcoded
; Out: DE = next seed
; Clobbers: A, F
lfsr_step:
    ld  a, d
    rlca                ; feedback = bit7 of D
    and 1               ; A = feedback (0 or 1)
    push af             ; save feedback

    ; sh = (sh+sh) + (sl>>7): shift DE left, MSB of E into bit0 of D
    ld  a, e
    rlca
    and 1               ; MSB of E (old bit7)
    ld  l, a
    ld  a, d
    add a, d
    add a, l            ; D = D*2 + E>>7
    ld  d, a
    ld  a, e
    add a, e            ; E = E*2
    ld  e, a

    pop af
    jr  z, .no_xor
    ; XOR with init $1337: D ^= $13, E ^= $37
    ld  a, d
    xor $13
    ld  d, a
    ld  a, e
    xor $37
    ld  e, a
.no_xor:
    ret

; ── XOR pixel: H=y, L=x ───────────────────────────────
; Clobbers: A, F, HL, BC, DE
xor_pixel:
    ; Compute ZX Spectrum pixel address:
    ; addr = $4000 | (y & 7)<<8 | (y>>3 & 7)<<5 | (y>>6)<<11 | (x>>3)
    ld  a, h            ; A = y
    ld  c, l            ; C = x

    ; y contribution
    ld  b, a
    and 7               ; y & 7 (pixel row in cell)
    rrca
    rrca
    rrca                ; → bits [10:8] of addr (need it at D high byte)
    ld  d, a
    ld  a, b
    rra
    rra
    rra
    rra                 ; y >> 3... actually rebuild cleanly:

    ; Standard ZX Spectrum address formula (from reference):
    ; D = $40 | (y>>6)<<3 | (y&7)
    ; E = (y>>3&7)<<5 | (x>>3)
    ld  a, h            ; y
    and $C0
    rlca
    rlca
    rlca                ; (y>>6) << 3... into bits 5:3
    or  $40
    ld  d, a
    ld  a, h
    and 7               ; y & 7
    or  d
    ld  d, a

    ld  a, h
    rra
    rra
    rra
    and $E0             ; (y>>3 & 7) << 5
    ld  e, a
    ld  a, c
    rra
    rra
    rra
    and $1F             ; x >> 3
    or  e
    ld  e, a            ; DE = pixel byte address

    ; Bit mask: bit 7 - (x & 7)
    ld  a, c
    and 7
    ld  b, a
    ld  a, $80
    jr  z, .got_mask
.shift_mask:
    rrca
    djnz .shift_mask
.got_mask:
    ex  de, hl          ; HL = address
    xor (hl)
    ld  (hl), a
    ret

; ── GPU-found seeds: 64 × (seed_lo, seed_hi, points) ─
seeds_data:
    incbin "seeds.bin"

