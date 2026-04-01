; replay.asm — LFSR-16 AND-cascade XOR renderer for ZX Spectrum
;
; Reads seeds binary table at SEEDS_ADDR, renders onto 0x4000 screen.
;
; Build:
;   python3 gen_seeds_bin.py ~/dev/z80-optimizer/data/foveal_cascade_seeds.json seeds_lfsr.bin
;   mza replay.asm -o replay.bin
;   mze --target spectrum replay.bin
;
; Binary table format at SEEDS_ADDR:
;   u16 LE   n_seeds
;   × n_seeds (7 bytes each):
;     u16 LE  seed
;     u8      ox, oy, blk, and_n, warmup

        ORG     $8000

SEEDS_ADDR  EQU $CC00       ; seeds binary table (incbin at end)
BUF         EQU $C000       ; 768-byte block scratch buffer

; ── Entry point ───────────────────────────────────────────────

start:
        ; Clear ZX Spectrum screen ($4000..$57FF = 6144 bytes)
        ld      hl, $4000
        ld      de, $4001
        ld      bc, $17FF
        ld      (hl), 0
        ldir

        ; BC = n_seeds  (little-endian u16 at SEEDS_ADDR)
        ld      hl, SEEDS_ADDR
        ld      c, (hl)
        inc     hl
        ld      b, (hl)
        inc     hl              ; HL now points to first entry

        ld      a, b
        or      c
        ret     z               ; nothing to render

.seed_loop:
        push    bc              ; save remaining count
        push    hl              ; save entry pointer

        ; Read entry fields sequentially
        ld      e, (hl)         ; seed lo
        inc     hl
        ld      d, (hl)         ; seed hi  → DE = seed
        inc     hl
        ld      a, (hl)
        ld      (var_ox),    a
        inc     hl
        ld      a, (hl)
        ld      (var_oy),    a
        inc     hl
        ld      a, (hl)
        ld      (var_blk),   a
        inc     hl
        ld      a, (hl)
        ld      (var_and_n), a
        inc     hl
        ld      b, (hl)         ; warmup → B (consumed by make_buf)

        call    make_buf        ; fills BUF[0..767]
        call    apply_buf       ; XORs active blocks onto screen

        pop     hl
        ld      bc, 7
        add     hl, bc          ; advance to next entry
        pop     bc
        dec     bc
        ld      a, b
        or      c
        jr      nz, .seed_loop
        ret

; ── lfsr16 ────────────────────────────────────────────────────
; state = (state >> 1) ^ (0xB400 if state & 1 else 0)
; In/Out: DE = LFSR state
; Clobbers: A, F

lfsr16:
        srl     d
        rr      e               ; DE >>= 1; carry = old bit 0 of E (LSB of DE)
        ret     nc              ; LSB was 0: no XOR
        ld      a, d
        xor     $B4
        ld      d, a            ; D ^= $B4  ($B400 high byte)
        ; E ^= $00 — no-op (low byte of $B400 is 0)
        ret

; ── make_buf ──────────────────────────────────────────────────
; Fill BUF[768]: each byte = AND of and_n consecutive LFSR LSBs.
; In:  DE = seed (0 → auto-corrected to 1), B = warmup
;      (var_and_n) = and_n
; Out: BUF[0..767] filled with 0 or 1
; Clobbers: AF, BC, DE, HL

make_buf:
        ; guard: seed 0 → use 1 (LFSR dead state)
        ld      a, d
        or      e
        jr      nz, .s_ok
        inc     de
.s_ok:
        ; warmup: discard B LFSR steps
        ld      a, b
        or      a
        jr      z, .wu_done
.wu:    call    lfsr16
        djnz    .wu
.wu_done:
        ld      hl, BUF
        ld      bc, 768         ; outer counter: 768 blocks
.blk:
        push    bc              ; save outer counter across inner loop
        ld      a, (var_and_n)
        ld      b, a            ; B = and_n (DJNZ counter)
        ld      c, 1            ; C = accumulator = 1
.and_lp:
        call    lfsr16          ; advance LFSR; clobbers A, F
        ld      a, e
        and     1               ; LSB of new state
        and     c               ; acc &= bit
        ld      c, a
        djnz    .and_lp
        pop     bc              ; restore outer counter
        ld      (hl), c         ; BUF[i] = acc
        inc     hl
        dec     bc
        ld      a, b
        or      c
        jr      nz, .blk
        ret

; ── apply_buf ─────────────────────────────────────────────────
; XOR each set block in BUF onto screen.
; Reads: (var_ox, var_oy, var_blk)
; Clobbers: AF, BC, DE, HL

apply_buf:
        ld      hl, BUF
        xor     a
        ld      (cur_by), a
        ld      b, 24           ; 24 block rows
.row:
        xor     a
        ld      (cur_bx), a
        ld      c, 32           ; 32 block cols
.col:
        ld      a, (hl)
        or      a
        jr      z, .skip        ; block inactive
        push    bc
        push    hl
        call    render_block
        pop     hl
        pop     bc
.skip:
        inc     hl
        ld      a, (cur_bx)
        inc     a
        ld      (cur_bx), a
        dec     c
        jr      nz, .col
        ld      a, (cur_by)
        inc     a
        ld      (cur_by), a
        dec     b
        jr      nz, .row
        ret

; ── render_block ──────────────────────────────────────────────
; XOR a blk×blk tile at screen coords (ox + bx*blk, oy + by*blk).
; Reads: (cur_bx, cur_by, var_ox, var_oy, var_blk)
; Clobbers: AF, BC, DE, HL

render_block:
        ; px_base = ox + cur_bx * blk
        ld      a, (var_blk)
        ld      b, a
        ld      a, (cur_bx)
        call    small_mul
        ld      c, a
        ld      a, (var_ox)
        add     a, c
        ld      (px_base), a

        ; py_base = oy + cur_by * blk
        ld      a, (var_blk)
        ld      b, a
        ld      a, (cur_by)
        call    small_mul
        ld      c, a
        ld      a, (var_oy)
        add     a, c
        ld      (py_base), a

        ; iterate dy = 0..blk-1
        xor     a
        ld      (cur_dy), a
.dy_lp:
        ; iterate dx = 0..blk-1
        xor     a
        ld      (cur_dx), a
.dx_lp:
        ; py = py_base + dy; skip if >= 96
        ld      a, (py_base)
        ld      b, a
        ld      a, (cur_dy)
        add     a, b
        cp      96
        jr      nc, .skip_px
        ld      h, a            ; H = py

        ; px = px_base + dx; skip if >= 128
        ld      a, (px_base)
        ld      b, a
        ld      a, (cur_dx)
        add     a, b
        cp      128
        jr      nc, .skip_px
        ld      l, a            ; L = px

        call    xor_pixel

.skip_px:
        ld      a, (cur_dx)
        inc     a
        ld      (cur_dx), a
        ld      b, a
        ld      a, (var_blk)
        cp      b
        jr      nz, .dx_lp

        ld      a, (cur_dy)
        inc     a
        ld      (cur_dy), a
        ld      b, a
        ld      a, (var_blk)
        cp      b
        jr      nz, .dy_lp
        ret

; ── small_mul ─────────────────────────────────────────────────
; A = A * B  (A ≤ 31, B = blk ≤ 8, result ≤ 248 — fits u8)
; Clobbers: A, B, C, F

small_mul:
        or      a
        ret     z               ; 0 * anything = 0
        ld      c, a
        xor     a
.mul_lp:
        add     a, c
        djnz    .mul_lp
        ret

; ── xor_pixel ─────────────────────────────────────────────────
; XOR one pixel at ZX Spectrum screen address.
; In: H = y (0..95), L = x (0..127)
; Clobbers: AF, BC, DE, HL

xor_pixel:
        ; ZX Spectrum pixel address:
        ;   high = $40 | ((y & $C0) >> 3) | (y & $07)
        ;   low  = (y & $38) << 2 | (x >> 3)
        ld      b, h            ; B = y (preserved across computation)
        ld      c, l            ; C = x

        ; high byte
        ld      a, b
        and     $C0             ; third (bits 7:6) → positions 7:6
        rrca
        rrca
        rrca                    ; → positions 4:3
        or      $40
        ld      d, a
        ld      a, b
        and     $07             ; pixel row (bits 2:0)
        or      d
        ld      d, a            ; D = high byte of address

        ; low byte
        ld      a, b
        and     $38             ; char row (bits 5:3) at positions 5:3
        add     a, a
        add     a, a            ; shift left 2 → positions 7:5
        ld      e, a
        ld      a, c
        rrca
        rrca
        rrca
        and     $1F             ; x >> 3 (column)
        or      e
        ld      e, a            ; E = low byte of address

        ; bit mask: $80 >> (x & 7)
        ld      a, c
        and     7               ; Z set if x aligned to byte
        ld      b, a
        ld      a, $80
        jr      z, .got_mask
.shr:   rrca
        djnz    .shr
.got_mask:
        ex      de, hl          ; HL = pixel byte address
        xor     (hl)
        ld      (hl), a
        ret

; ── Variables ─────────────────────────────────────────────────

var_ox:     defb 0
var_oy:     defb 0
var_blk:    defb 4
var_and_n:  defb 3
cur_bx:     defb 0
cur_by:     defb 0
cur_dx:     defb 0
cur_dy:     defb 0
px_base:    defb 0
py_base:    defb 0

; ── Seeds binary table ────────────────────────────────────────
; Must assemble at SEEDS_ADDR ($CC00).
; Generated by bake_replay.py — do not edit manually.

        ORG     SEEDS_ADDR
; SEEDS_DATA_BEGIN
