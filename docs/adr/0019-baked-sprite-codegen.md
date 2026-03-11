# ADR-0019: Baked Sprite Codegen and Phantom Global Storage

**Status**: Proposed
**Date**: 2026-03-11
**Context**: Nanz/MIR2 Z80 backend — ZX Spectrum target

---

## Context

On the ZX Spectrum, every sprite pixel must be written to video RAM by Z80 code.
The hardware provides no blitter, no sprite coprocessor, and no DMA.
The screen layout is deliberately hostile: pixel rows are interleaved across three
"thirds" of VRAM, so consecutive pixel rows are *not* at consecutive addresses.
Computing the next row address at runtime costs 27–62 T-states per row boundary.

The canonical high-performance technique — well-documented in demoscene literature
and handcrafted assembly archives (see antique-toy/ch16-sprites, Method 5) — is
the **compiled sprite**: a function where pixel data is baked as `LD (HL), n`
immediates and screen addresses are pre-computed once at load time.  The function
is called to draw; moving the sprite means patching the address bytes inside the
function.  This is an application of the TSMC (True Self-Modifying Code) paradigm
already present in MinZ, extended from function parameters to struct fields.

No existing high-level language can express this because C requires `&field` to
be a valid memory address, which forbids embedding field values inside instruction
opcodes.  Nanz has no such constraint.

This ADR also covers the broader **phantom global storage** family of which baked
sprites are one instance, and introduces `StorageClass` as the architectural
foundation for all SMC-oriented storage modes.

---

## The Problem in Numbers

For a 16×8 sprite (2 bytes wide, 8 rows):

| Method | Draw cost | Move cost | Code size |
|--------|-----------|-----------|-----------|
| Generic loop (LDIR) | ~1,344 T | 0 T | ~30 B |
| OR+AND masked loop  | ~1,040 T | 0 T | ~40 B |
| Compiled sprite (this ADR) | **~118 T** | ~320 T | ~48 B |
| Pre-shifted ×8 compiled | **~118 T** | ~640 T | ~384 B |

Draw is 9–11× faster than a generic loop.  Move cost is a one-time patch amortised
over all frames until the next movement.  At 50 fps, a sprite that moves once per
frame spends 118T drawing and 320T moving — both negligible fractions of a 3.5 MHz
frame budget (58,333 T per frame).

---

## Decision

### Part 1: `StorageClass` in `mir2.Global` (implemented now)

```go
type StorageClass uint8

const (
    StorageNormal      StorageClass = iota  // DB data section; &field valid
    StorageSMCGetter                        // fields in getter fn immediates
    StoragePhantom                          // fields at each read site
    StorageBakedSprite                      // ZX Spectrum compiled sprite
)

type Global struct {
    ...
    Storage StorageClass  // ← added; zero value = StorageNormal (no behaviour change)
}
```

This is the only code change today.  All four modes share the same architectural
concept: **field values live in instruction immediates, not in a data section**.

---

### Part 2: Storage mode taxonomy

#### StorageNormal (current, always on)
```
palette: DB 1, 2, 3
LD A, (palette)     ; 13T per field read
LD (palette), A     ; 13T per field write
&palette.r          ; valid address
```

#### StorageSMCGetter (next sprint)
Fields baked as immediates in a synthesised getter function.
```nanz
@smc global palette: Color = Color{ r: 1, g: 2, b: 3 }
```
```z80
get_palette:
    LD L, 1        ; palette.r — 7T
palette$r  EQU get_palette + 1
    LD H, 2        ; palette.g — 7T
palette$g  EQU get_palette + 3
    LD E, 3        ; palette.b — 7T
palette$b  EQU get_palette + 5
    RET            ; 10T
; Write: LD A, val; LD (palette$r), A — 20T, same as normal write
```
Read 3 fields: `CALL get_palette` = 17+10+21 = 48T vs 39T (slower for 3+ u8 fields
unless packed two-per-LD-HL, see below).  Better packing with `LD HL, g<<8|r`:
read r+g together = 17+10+10 = 37T vs 26T (worse for pair reads; better for
structured 3+ field returns via HL+E registers filling calling convention slots).

**Forbidden**: `&palette.r` — no data-section address exists.
**Target restriction**: RAM targets only (ZX Spectrum, CP/M, Agon).

#### StoragePhantom (post-PBQP contracts)
Fields baked at every individual read site.  No getter function.
```nanz
@phantom global palette: Color = Color{ r: 1, g: 2, b: 3 }
```
```z80
; at read site A:
read_r_siteA:  LD A, 1       ; palette.r — 7T
palette$r$A    EQU read_r_siteA + 1

; at read site B:
read_r_siteB:  LD B, 1       ; palette.r — 7T
palette$r$B    EQU read_r_siteB + 1

; write palette.r = 42:
    LD A, 42
    LD (palette$r$A), A      ; 13T — patch site A
    LD (palette$r$B), A      ; 13T — patch site B
```
Read: always 7T (optimal).  Write: 13T × N_read_sites.
Break-even vs normal: read count > write count (very common for config/palette).
Requires two-pass codegen: collect all read sites before emitting any writes.

#### StorageBakedSprite (this ADR's primary focus)
See Part 3 below.

---

### Part 3: Baked Sprite Codegen

#### 3.1 Annotation and struct declaration

```nanz
@target(zxspectrum)

@baked_sprite(width_bytes: 2, height: 8)
struct Sprite {
    x:    u8<0..31>    // byte column
    y:    u8<0..191>   // pixel row
    data: [16]u8       // width_bytes × height, row-major
}

global player: Sprite = Sprite{
    x: 10, y: 20,
    data: [
        0x3C, 0x0F,   // row 0: left byte, right byte
        0x7E, 0x1F,   // row 1
        0xFF, 0xFF,   // row 2
        0xFF, 0xFF,   // row 3
        0xFF, 0xFF,   // row 4
        0x7E, 0x1F,   // row 5
        0x3C, 0x0F,   // row 6
        0x00, 0x00,   // row 7 (transparent)
    ]
}
```

The compiler synthesises three functions automatically:
- `player_draw()` — the baked sprite printer
- `player_set_pos(x: u8, y: u8)` — patches address immediates
- `player_set_frame(frame_idx: u8)` — patches data immediates (multi-frame sprites)

#### 3.2 Compile-time ZX screen address

```go
// pkg/spectrum/screen.go (or pkg/mir2/zxaddr.go — to be decided)
func ZXPixelAddr(byteCol, pixelRow int) uint16 {
    return 0x4000 |
        uint16(pixelRow&0x07)<<8 |  // fine-y → addr[10:8]
        uint16(pixelRow&0x38)<<2 |  // char row → addr[7:5]
        uint16(pixelRow&0xC0)<<5 |  // third → addr[12:11]
        uint16(byteCol)             // byte column → addr[4:0]
}

func ZXAttrAddr(charCol, charRow int) uint16 {
    return 0x5800 + uint16(charRow*32+charCol)
}
```

This function runs in the compiler, not in the generated Z80 code.

#### 3.3 Generated `draw_player`

For `Sprite{ x:10, y:20, width_bytes:2, height:8 }`:

```z80
; fun player_draw()  ; clobbers: HL
; Generated by BakedSpriteGen. Edit via player_set_pos / player_set_frame.
player_draw:
    LD HL, 0x4152        ; ZXPixelAddr(10, 20) = 0x4152  — 10T
player$row0$addr  EQU player_draw + 1
    LD (HL), 0x3C        ;  data[0] — 10T
player$row0$b0    EQU player_draw + 4
    INC L                ;  col+1   —  4T
    LD (HL), 0x0F        ;  data[1] — 10T
player$row0$b1    EQU player_draw + 7
    DEC L                ;  restore —  4T  (only if not last col)
    INC H                ;  row+1   —  4T

    LD HL, 0x4252        ; ZXPixelAddr(10, 21) — 10T
player$row1$addr  EQU player_draw + 13
    LD (HL), 0x7E        ; 10T
player$row1$b0    EQU player_draw + 16
    INC L
    LD (HL), 0x1F        ; 10T
player$row1$b1    EQU player_draw + 19
    DEC L
    INC H
    ; ... rows 2–6 same pattern ...

    ; Row 7 (character boundary at y=24, third boundary handled in addr):
    LD HL, 0x40D2        ; ZXPixelAddr(10, 27) — pre-computed, handles boundary
player$row7$addr  EQU player_draw + N
    LD (HL), 0x00
player$row7$b0    EQU player_draw + N+3
    INC L
    LD (HL), 0x00
player$row7$b1    EQU player_draw + N+6
    RET                  ; 10T

; Total for 8 rows × 2 bytes: 8 × (10+10+4+10+4+4) = 8 × 42 = 336T + 10T RET = 346T
; (vs ~1,344T generic LDIR loop)
```

Note: `INC H` works within a character block (fine-y 0–7).  At character
boundaries the address jumps non-linearly — the compiler bakes a fresh `LD HL`
for each row so the boundary is handled automatically, at the cost of 10T per row
instead of 4T for `INC H`.  The pre-computation eliminates ALL boundary arithmetic
from the runtime path.

**Optimisation**: for rows within the same character block (fine-y 0→1→2→...→7),
the compiler CAN emit `INC H` instead of a full `LD HL` (saves 6T per row):

```z80
    LD HL, 0x4152        ; row 0 — 10T (base LD HL for block start)
    LD (HL), 0x3C
    INC H                ; row 1 — 4T  (fine-y+1, same char block)
    LD (HL), 0x7E
    INC H                ; row 2 — 4T
    ; ...
    INC H                ; row 7 (fine-y=7→8, char boundary — needs new LD HL next row)
    LD HL, 0x4252        ; row 8 in next char block — new LD HL
```

The compiler detects character block boundaries from the pre-computed addresses and
emits `INC H` vs `LD HL` accordingly.

#### 3.4 Generated `player_set_pos`

```z80
; fun player_set_pos(x: u8<B>, y: u8<C>)  ; clobbers: A, BC, HL
player_set_pos:
    ; Patch row 0 address
    CALL zx_pixel_addr        ; B=x, C=y → HL = screen addr  (~30T)
    LD A, L
    LD (player$row0$addr),   A   ; 13T
    LD A, H
    LD (player$row0$addr+1), A   ; 13T
    INC C                         ;  4T  y+1
    ; Patch row 1 address
    CALL zx_pixel_addr
    LD A, L
    LD (player$row1$addr),   A
    LD A, H
    LD (player$row1$addr+1), A
    INC C
    ; ... repeat for all 8 rows ...
    RET

; Cost: 8 rows × (CALL 17 + compute ~30 + RET 10 + 4×13 = 109T) ≈ 872T per set_pos call
; Called only when sprite moves — amortised over many draw() calls
```

`zx_pixel_addr` is a shared stdlib function (16B, ~30T):
```z80
; Input: B=byte_col (0..31), C=pixel_row (0..191)
; Output: HL = ZX pixel address
; Clobbers: A
zx_pixel_addr:
    LD A, C          ;  4T  pixel_row
    AND 0x07         ;  7T  fine-y (bits 0-2)
    RLCA ; RLCA ; RLCA ; RLCA ; RLCA ; RLCA ; RLCA ; RLCA  ; → bits 8-10 of addr
    ; ... standard ZX addr calc ...
    OR B             ; add byte column
    LD L, A
    ; compute H from char-row and third
    LD H, ...
    RET
```

#### 3.5 Generated `player_set_frame` (animated sprites)

For multi-frame sprites:
```nanz
@baked_sprite(width_bytes: 2, height: 8, frames: 4)
global enemy: AnimSprite = AnimSprite{
    x: 20, y: 40,
    frames: [
        [0x3C, 0x0F, ...],  // frame 0
        [0x7E, 0x1F, ...],  // frame 1
        [0x3C, 0x0F, ...],  // frame 2
        [0x18, 0x0F, ...],  // frame 3
    ]
}
```

```z80
enemy_set_frame:   ; A = frame index
    ; multiply A by frame stride to get frame data offset
    ; then patch each data byte EQU from the frame table
    ; Cost: N_rows × width_bytes × 20T = 8×2×20 = 320T per frame change
```

#### 3.6 Three levels of baked sprites

```
Level 1: @baked_sprite (overwrite)
  LD (HL), n per byte — fastest, no masking
  data[row][col] baked; addresses baked
  14T per byte-row (LD (HL),n=10T + INC H=4T)

Level 2: @masked_sprite (AND+OR compiled)
  LD A,(HL) ; AND mask ; OR data ; LD (HL),A per byte
  mask AND data baked; addresses baked
  38T per byte-row (vs 134T in masked loop)

Level 3: @preshift_sprite (8 compiled variants)
  8 versions of draw function, one per X pixel offset (0..7)
  set_pos() selects correct version via JMP table
  14T per byte-row in ALL of the 8 variants
  8× code size; ultimate speed for fast-moving sprites
```

#### 3.7 Attribute table integration

For colour, the compiler can optionally bake attribute writes too:

```z80
player_draw_attrs:
    LD HL, 0x5862            ; ZXAttrAddr(char_col, char_row) — pre-computed
player$attr$r0c0  EQU player_draw_attrs + 1
    LD (HL), BRIGHT|PAPER_CYAN|INK_WHITE    ; baked colour — 10T
    INC L
    LD (HL), BRIGHT|PAPER_CYAN|INK_WHITE    ; second attr cell — 10T
    LD HL, 0x5882            ; next char row attrs
    LD (HL), BRIGHT|PAPER_CYAN|INK_WHITE
    INC L
    LD (HL), BRIGHT|PAPER_CYAN|INK_WHITE
    RET
; 4 attr cells: 4 × 10T = 40T total
```

`set_pos()` patches both pixel addresses AND attr addresses.

---

### Part 4: Patch table architecture

The compiler maintains a `PatchTable` during code generation:

```go
type PatchSite struct {
    Symbol string   // EQU label in generated asm, e.g. "player$row0$addr"
    Offset int      // byte offset from function start
    Width  int      // 1 = lo byte, 2 = lo+hi, etc.
}

type PatchTable struct {
    // GlobalName → FieldKey → []PatchSite
    Sites map[string]map[string][]PatchSite
}
```

The table is populated in Pass 1 (read site collection) and consumed in Pass 2
(write site emission).  This is analogous to how MZA (the assembler) handles
forward references — the same mechanism, one layer higher.

---

### Part 5: Conflict detection

The following must be compile errors for `StorageBakedSprite` / `StoragePhantom`:

| Forbidden operation | Reason |
|---|---|
| `&sprite.x` (address-of field) | No data section address exists |
| `sprite.x` as lvalue in a loop body | Write cost = 13T × N_sites |
| Multiple instances of same baked type | Requires separate getter/draw per instance |
| ROM target | SMC writes would trap |
| Passing baked field to generic pointer param | Cannot take address |

---

## Implementation Plan

```
Now (done):
  StorageClass field in mir2.Global
  ADR-0019

After copy-coalescing stabilises:
  StorageSMCGetter:
    - parser: @smc annotation
    - mir2: OpSMCRead, OpSMCWrite ops (or reuse OpPatchSlot)
    - codegen: synthesise getter function with EQU labels
    - conflict detection: AddrOf on @smc global → error

After PBQP contracts:
  StoragePhantom:
    - two-pass codegen (read collection → write emission)
    - cost oracle integration with GPU superoptimizer

After StoragePhantom:
  StorageBakedSprite:
    - @baked_sprite parser annotation
    - BakedSpriteGen pass (new pkg/mir2/bakedsprite.go)
    - ZXPixelAddr / ZXAttrAddr compile-time functions
    - synth draw() / set_pos() / set_frame()
    - patch table architecture
    - three levels: overwrite / masked / preshift
    - test corpus: antique-toy/ch16-sprites/draft.md Method 5 examples
```

---

## Consequences

### Positive
- Draw performance: 9–11× faster than generic sprite loops
- No special knowledge required: `player.draw()` just works
- ZX Spectrum address interleaving handled entirely by compiler
- Composable with existing TSMC infrastructure (OpPatchSlot reuse)
- First high-level language to generate demoscene-quality sprite code
- Test corpus exists: antique-toy ch16 provides ground-truth assembly

### Negative
- `@baked_sprite` / `@phantom` globals cannot be used as regular pointers
- Animation requires one compiled routine per frame (code size tradeoff)
- `set_pos()` cost (~870T for 8-row sprite) is non-trivial if called every frame
- Pre-shifted sprites (level 3) use 8× code space per sprite
- ROM targets excluded entirely

### Neutral
- `StorageNormal` (default) is completely unaffected — zero behaviour change today
- The patch mechanism reuses MZA's existing EQU/forward-reference infrastructure
- `zx_pixel_addr` stdlib function is useful independently of baked sprites

---

## Alternatives Considered

### Runtime address computation with cached base
Cache the computed base address in a global and only recompute on `set_pos()`.
Draw function loads cached HL and uses `INC H` / boundary correction.
**Rejected**: still requires boundary arithmetic; draw function is ~3× slower than
baked version; doesn't eliminate the data-pointer management.

### LDIR-based compiled sprite
Use `LD DE, screen_addr; LD HL, data; LD BC, row_len; LDIR` per row.
**Rejected**: 21T per byte (LDIR loop), no masking support, requires data table,
addresses still computed at runtime.

### External code generator (Python/Lua)
Generate the compiled sprite in a separate tool and `@include` it.
**Rejected**: breaks the single-source-of-truth property of Nanz; requires a
separate build step; doesn't integrate with `set_pos()` / `set_frame()`.

### C-style struct with __attribute__((smc))
**Rejected**: C requires `&field` to be a valid memory address.  This is a
fundamental constraint of the C abstract machine that Nanz does not share.

---

## References

- antique-toy/ch16-sprites/draft.md — Method 5: Compiled Sprites (ground-truth)
- antique-toy/ch16-sprites/draft.md — Method 3: Pre-Shifted Sprites
- ADR-0010: TSMC (True Self-Modifying Code) — the foundational mechanism
- TSMC docs: `docs/145_TSMC_Complete_Philosophy.md`
- ZX Spectrum Screen Memory Layout: `0x4000 | fine-y<<8 | char-row<<5 | third<<11 | col`
- `pkg/spectrum/screen.go` — ZX screen utilities (ZXPixelAddr to be added here)
- `pkg/mir2/module.go` — `StorageClass` field (added in this ADR)
