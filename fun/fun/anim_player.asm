; anim_player.asm — Animated LFSR-16 AND-cascade player for ZX Spectrum
;
; Binary format at ANIM_ADDR ($CC00):
;   u8[4]   "ANMZ"
;   u16 LE  n_frames
;   u8      fps       (1-50)
;   u8      _pad
;   × n_frames × 4 bytes: [u16 seed_count, u8 frame_type(0=kf,1=dt), u8 _pad]
;   × total_seeds × 7 bytes: [u16 seed, u8 ox,oy,blk,and_n,warmup]
;
; Build:
;   python3 gen_anim_bin.py input.json --bake anim_player.asm -o anim_player_baked.asm
;   mza anim_player_baked.asm -o anim_player.bin
;   mze --target spectrum anim_player.bin

        ORG     $8000

ANIM_ADDR   EQU $CC00
BUF         EQU $C000

; ── Entry point ───────────────────────────────────────────────

start:
        ; Read n_frames (u16 LE at +4)
        ld      hl, ANIM_ADDR + 4
        ld      c, (hl)
        inc     hl
        ld      b, (hl)         ; BC = n_frames

        ; Read fps (+6), compute halts_per_frame = 50 / fps
        inc     hl
        ld      a, 50
        ld      e, (hl)         ; E = fps
        call    div_a_by_e      ; A = 50 / fps
        ld      (halts_per_frame), a

        ; Store n_frames (as u16 via LD (nn),BC — use HL trick)
        ld      h, b
        ld      l, c
        ld      (var_nframes), hl   ; store n_frames as u16

        ; frame_table = ANIM_ADDR + 8
        ; seeds_base  = frame_table + n_frames * 4
        ; BC = n_frames, compute BC*4
        sla     c
        rl      b               ; BC <<= 1  (*2)
        sla     c
        rl      b               ; BC <<= 1  (*4)
        ld      hl, ANIM_ADDR + 8
        add     hl, bc          ; HL = seeds_base
        ld      (seeds_base), hl

; ── Animation loop (forever) ──────────────────────────────────

.anim_loop:
        ; Reset seeds pointer
        ld      hl, (seeds_base)
        ld      (seeds_ptr), hl

        ; fi = 0; frame table pointer at ANIM_ADDR+8
        ld      hl, ANIM_ADDR + 8       ; HL = frame table entry ptr
        ld      bc, (var_nframes)       ; BC = n_frames

.frame_loop:
        push    bc              ; save remaining frame count
        push    hl              ; save frame table pointer

        ; Read seed_count (u16 LE at HL)
        ld      e, (hl)
        inc     hl
        ld      d, (hl)         ; DE = seed_count
        inc     hl

        ; Read frame_type (0=kf, 1=dt)
        ld      a, (hl)
        inc     hl              ; skip frame_type byte
        inc     hl              ; skip pad byte

        or      a
        jr      nz, .no_clear
        call    clear_screen
.no_clear:

        ; Render DE seeds from seeds_ptr
        ld      a, d
        or      e
        jr      z, .seeds_done  ; 0 seeds in this frame

        ld      hl, (seeds_ptr)         ; HL = seeds_ptr
.seed_lp:
        push    de
        push    hl

        ; Read entry: seed(HL+0,+1), ox(+2), oy(+3), blk(+4), and_n(+5), warmup(+6)
        ld      e, (hl)
        inc     hl
        ld      d, (hl)         ; DE = seed
        inc     hl
        ld      a, (hl)
        ld      (var_ox), a
        inc     hl
        ld      a, (hl)
        ld      (var_oy), a
        inc     hl
        ld      a, (hl)
        ld      (var_blk), a
        inc     hl
        ld      a, (hl)
        ld      (var_and_n), a
        inc     hl
        ld      b, (hl)         ; B = warmup

        call    make_buf
        call    apply_buf

        pop     hl
        ld      bc, 7
        add     hl, bc          ; seeds_ptr += 7
        pop     de
        dec     de
        ld      a, d
        or      e
        jr      nz, .seed_lp

        ; Save updated seeds_ptr
        ld      (seeds_ptr), hl

.seeds_done:
        ; Wait halts_per_frame × HALT
        ld      a, (halts_per_frame)
        ld      b, a
.halt_lp:
        halt
        djnz    .halt_lp

        pop     hl              ; restore frame table pointer (already advanced)
        pop     bc
        dec     bc
        ld      a, b
        or      c
        jr      nz, .frame_loop

        jr      .anim_loop      ; loop forever

; ── lfsr16 ────────────────────────────────────────────────────
; In/Out: DE = state.  Clobbers: A, F

lfsr16:
        srl     d
        rr      e
        ret     nc
        ld      a, d
        xor     $B4
        ld      d, a
        ret

; ── make_buf ──────────────────────────────────────────────────
; In: DE=seed, B=warmup, (var_and_n)=and_n.  Clobbers: AF,BC,DE,HL

make_buf:
        ld      a, d
        or      e
        jr      nz, .s_ok
        inc     de
.s_ok:
        ld      a, b
        or      a
        jr      z, .wu_done
.wu:    call    lfsr16
        djnz    .wu
.wu_done:
        ld      hl, BUF
        ld      bc, 768
.blk:
        push    bc
        ld      a, (var_and_n)
        ld      b, a
        ld      c, 1
.and_lp:
        call    lfsr16
        ld      a, e
        and     1
        and     c
        ld      c, a
        djnz    .and_lp
        pop     bc
        ld      (hl), c
        inc     hl
        dec     bc
        ld      a, b
        or      c
        jr      nz, .blk
        ret

; ── apply_buf ─────────────────────────────────────────────────
; Clobbers: AF, BC, DE, HL

apply_buf:
        ld      hl, BUF
        xor     a
        ld      (cur_by), a
        ld      b, 24
.row:
        xor     a
        ld      (cur_bx), a
        ld      c, 32
.col:
        ld      a, (hl)
        or      a
        jr      z, .skip
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
; Clobbers: AF, BC, DE, HL

render_block:
        ld      a, (var_blk)
        ld      b, a
        ld      a, (cur_bx)
        call    small_mul
        ld      c, a
        ld      a, (var_ox)
        add     a, c
        ld      (px_base), a

        ld      a, (var_blk)
        ld      b, a
        ld      a, (cur_by)
        call    small_mul
        ld      c, a
        ld      a, (var_oy)
        add     a, c
        ld      (py_base), a

        xor     a
        ld      (cur_dy), a
.dy_lp:
        xor     a
        ld      (cur_dx), a
.dx_lp:
        ld      a, (py_base)
        ld      b, a
        ld      a, (cur_dy)
        add     a, b
        cp      96
        jr      nc, .skip_px
        ld      h, a

        ld      a, (px_base)
        ld      b, a
        ld      a, (cur_dx)
        add     a, b
        cp      128
        jr      nc, .skip_px
        ld      l, a

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
; A = A * B  (A≤31, B≤8).  Clobbers: A, B, C, F

small_mul:
        or      a
        ret     z
        ld      c, a
        xor     a
.mul_lp:
        add     a, c
        djnz    .mul_lp
        ret

; ── xor_pixel ─────────────────────────────────────────────────
; In: H=y (0..95), L=x (0..127).  Clobbers: AF, BC, DE, HL

xor_pixel:
        ld      b, h
        ld      c, l

        ld      a, b
        and     $C0
        rrca
        rrca
        rrca
        or      $40
        ld      d, a
        ld      a, b
        and     $07
        or      d
        ld      d, a

        ld      a, b
        and     $38
        add     a, a
        add     a, a
        ld      e, a
        ld      a, c
        rrca
        rrca
        rrca
        and     $1F
        or      e
        ld      e, a

        ld      a, c
        and     7
        ld      b, a
        ld      a, $80
        jr      z, .got_mask
.shr:   rrca
        djnz    .shr
.got_mask:
        ex      de, hl
        xor     (hl)
        ld      (hl), a
        ret

; ── clear_screen ──────────────────────────────────────────────
; Clobbers: AF, BC, DE, HL

clear_screen:
        ld      hl, $4000
        ld      de, $4001
        ld      bc, $17FF
        ld      (hl), 0
        ldir
        ret

; ── div_a_by_e ────────────────────────────────────────────────
; A = A / E  (unsigned, small values).  Clobbers: A, E, F

div_a_by_e:
        ld      d, 0
.div_lp:
        sub     e
        jr      c, .div_done
        inc     d
        jr      .div_lp
.div_done:
        ld      a, d
        ret

; ── Variables ─────────────────────────────────────────────────

var_nframes:    defw 0          ; u16 n_frames
seeds_base:     defw 0          ; u16 absolute address of seeds array
seeds_ptr:      defw 0          ; u16 current seeds pointer
halts_per_frame:defb 10
var_ox:         defb 0
var_oy:         defb 0
var_blk:        defb 4
var_and_n:      defb 3
cur_bx:         defb 0
cur_by:         defb 0
cur_dx:         defb 0
cur_dy:         defb 0
px_base:        defb 0
py_base:        defb 0

; ── Animation data ────────────────────────────────────────────
; Must assemble at ANIM_ADDR ($CC00).
; Generated by gen_anim_bin.py --bake anim_player.asm

        ORG     ANIM_ADDR
; ANIM_DATA_BEGIN
