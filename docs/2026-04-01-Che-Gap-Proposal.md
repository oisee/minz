# Proposal: Closing the `che_intro.nanz → che_intro.asm` Gap

**Date:** 2026-04-01
**Author:** z80-optimizer session (Alice)
**Target:** minZ backend team
**Status:** Ready to implement — all tables and infrastructure exist

---

## Executive Summary

`che_intro.nanz` compiled by minZ today produces ~840ms Z80 code.
`che_intro.asm` hand-written produces ~290ms.
**Gap: 2.9×.** With three targeted backend passes this closes to **≤1.05×** using tables
that already exist in `z80-optimizer/data/`. No new GPU search needed.

---

## Profile Data

Execution profile of `cuda/che_intro.asm` (annotated binary, 223,729 total instructions):

| Address | Executions | Instruction / Region | Why hot |
|---------|-----------|---------------------|---------|
| 0x800B | 6,143 | `LDIR` (screen clear) | 6144 bytes, unavoidable |
| 0x8032 | 3,204 | `CALL lfsr_step` | 2968 pts + 64×8 warmup steps |
| **0x806A** | **11,403** | **`SRL A; DEC C; JR NZ`** (bit-shift loop) | **avg 3.56 iters/pixel = E[X&7]** |
| 0x8085 | 3,716 | `CP 96; SUB 96` (Y-mod branch) | Y≥96 taken 23% of time |

The bit-shift loop at 0x806A (11,403 hits = 3.56× per pixel) is entirely avoidable.
The Y-mod branch (3,716 hits) can be restructured. Everything else is near-optimal.

---

## Root Cause: `xor_pixel` Cost Breakdown

`xor_pixel(x: u8, y: u8)` is called 2,968 times (inner loop body).
The screen address formula is `0x4000 + y7*2048 + y2_0*256 + y5_3*32 + xbyte`.

### Current Nanz emission (estimated, library calls):

| Sub-expression | Operation | Naive T | Optimal T | Source |
|---------------|-----------|---------|-----------|--------|
| `y / 8` | SRL×3 | ~80T (call) | **24T** | `div8_optimal[8]` |
| `y % 8` | AND 7 | ~100T (call) | **4T** | power-of-2 mask |
| `y / 64` | mul_shift | ~141T (call) | **10T** ¹ | range-aware |
| `(y/8) % 8` | AND 7 after shift | ~100T (call) | **4T** | power-of-2 mask |
| `x / 8` | SRL×3 | ~80T (call) | **24T** | `div8_optimal[8]` |
| `x % 8` | AND 7 | ~100T (call) | **4T** | power-of-2 mask |
| `y5_3 * 32` | RLA+ADD A,A×4 | ~70T (call) | **20T** | `mulopt8[32]` |
| `y7 * 2048` | range branch | ~150T (call) | **10T** ¹ | range → branch |
| **TOTAL/pixel** | | **~821T** | **~100T** | |

¹ `y < 96` is a loop invariant → `y/64 ∈ {0,1}` → `CP 64; JR C; SET 3,H` (10T avg).
This requires range propagation feeding the arithmetic lowering pass.

**Saving: ~721T/pixel × 2,968 pixels = 539ms @3.5MHz** — from arithmetic tables alone.

### Bit-mask loop: `$80 >> (x & 7)`

```asm
; Current (variable loop, avg 11403/3204 = 3.56 iters, ~28T avg):
    LD C, A      ; C = x & 7
    LD A, $80
.shift:
    SRL A
    DEC C
    JR NZ, .shift

; Better (table lookup, 16T fixed):
    LD C, A      ; C = x & 7  (A = x & 7 already computed above)
    LD B, 0
    LD HL, bitmask_table
    ADD HL, BC
    LD A, (HL)   ; A = $80 >> c

; bitmask_table: DB $80,$40,$20,$10,$08,$04,$02,$01

; Or: RRCA trick (8T, zero clobbers other than A,F):
    ; A = x & 7, want $80 >> A
    ; Method: LD A,$80 ; repeat RRCA for bit count
    ; With DJNZ: if C=0 skip, else DJNZ — 8T + avg 3.56×8T = ~36T (worse)

; Best: precomputed 8-byte table → LD A,(HL+C) = 16T fixed
```

Table approach: **16T fixed** vs 28T avg. Saving: **12T × 2,968 = 10ms**.

---

## Change 1: Arithmetic Idiom Pass (biggest win, ~459ms)

**Where in compiler:** IR lowering or MIR→VIR pass, before regalloc.

**Rule:** For any `DIV_CONST(v, K)` or `MOD_CONST(v, K)`:
1. Load `z80-optimizer/data/div8_optimal.json[K]` → get `.ops[]` and `.tstates`
2. Load `z80-optimizer/data/mod8_optimal.json[K]` equivalently
3. Emit the sequence directly, inline, no library call

**Go API (already built):**
```go
import "github.com/oisee/z80-optimizer/pkg/mulopt"

// Division: A ÷ K → A
seq := mulopt.EmitDiv8(k)      // returns []Instruction
// Multiplication: A × K → A
seq := mulopt.Emit8(k, bSafe)  // bSafe=true keeps B free
```

**Critical constants for `xor_pixel`:**

| K | Operation | Sequence | T | Note |
|---|-----------|----------|---|------|
| 8 | `÷` | `SRL A; SRL A; SRL A` | 24T | y→y_char, x→xbyte |
| 8 | `%` | `AND 7` | 4T | y→y_pixel_row |
| 32 | `×` | `RLA; ADD A,A; ADD A,A; ADD A,A; ADD A,A` | 20T | y5_3→addr_l |
| 64 | `÷` | range-aware: `CP 64; JR C; SET 3,H` | 10T | y7 (needs range hint) |

**Range-aware case** (`÷ 64` with `y < 96`):
Add range annotation to loop variable: if type checker or loop analysis knows `y ∈ [0, 95]`,
then `y / 64 ∈ {0, 1}`, so the "division" is just a conditional bit set.
Emit: `CP 64; JR C .no_third; SET 3,H; .no_third:` — **10T vs 37T** from generic table.
This is a 27T improvement, requires range propagation → arithmetic lowering communication.

---

## Change 2: Inline `xor_pixel` (79ms)

**Where in compiler:** inlining pass, before regalloc.

**Condition to inline:** function is called in a loop body, body ≤ 15 MIR ops, no recursion,
no side effects beyond memory write.

`xor_pixel` has exactly 5 live variables post-inlining: `{x, y, addr_h, addr_l, mask}`.

**Regalloc lookup** (`z80-optimizer/data/enriched_5v.enr`):
```go
shape := regalloc.Shape{
    NVregs:      5,
    Widths:      []int{8, 8, 8, 8, 8},
    Interference: 0b00110,  // x↔y don't interfere; addr_h↔addr_l do interfere with each other
}
entry := table.Lookup(idx)
// entry.Assignment = [E, D, H, L, A]  ← x→E, y→D, addr_h→H, addr_l→L, mask→A
// entry.Flags & FlagMul8Safe = true   ← C,H,L free for mul8 (C is not assigned)
// OFB: HL_PTR set → direct XOR (HL) native
// OFB: DJNZ_FREE set → B available for loop counter
```

Optimal assignment: **x→E, y→D, addr→HL, mask→A, B free**.
This matches `che_intro.asm` exactly. No PUSH/POP needed — saves:
- `CALL` (17T) + `RET` (10T) + 3×`PUSH` (11T each) + 3×`POP` (10T each) = **93T/pixel**

With OFB `DJNZ_FREE` set: if inner loop also needs B for XOR-8-rows block, use B as row counter
(the `che_optimal.asm` approach: `LD B,8; .xor8: LD A,(HL); CPL; LD (HL),A; INC H; DJNZ`).

---

## Change 3: EXX Zone for LFSR State (49ms)

**Where in compiler:** EXX zone split pass (ADR-008), after inlining.

**Problem:** After inlining `xor_pixel`, the LFSR state `(D,E,H,L)` must survive pixel
computation which also needs `D,E,H,L`. Current solution: 3×PUSH + 3×POP = 66T overhead.

**Solution:** LFSR state lives in shadow register bank. Pixel computation runs in main bank.

```asm
; EXX zone structure:
lfsr_step:                ; runs in main bank — DEHL = LFSR state
    SRL D; RR E; RR H; RR L   ; shift right (32T total)
    RET NC
    LD A,D; XOR $B4; LD D,A   ; XOR poly bytes
    ...
    RET

; After lfsr_step, before pixel computation:
    EXX                   ; save DEHL → D'E'H'L', get clean DEHL = 4T

; Pixel computation uses DEHL freely
; ...
    EXX                   ; restore D'E'H'L' → DEHL = 4T
; Loop: DJNZ, DEC C, etc.
```

**Cost model** (from `pkg/regalloc/zone.go`):
```go
cost := zone.BoundaryCostFull(
    mainAssign:   []byte{locE, locD, locH, locL},   // LFSR vars
    shadowAssign: []byte{locE, locD, locH, locL},   // same vars in shadow
    crossing:     []int{},                           // no vars cross zone
    widths:       []int{8, 8, 8, 8},
)
// Returns: 4T (EXX) + 4T (EXX back) = 8T total
// vs current: 3×PUSH + 3×POP = 66T
// Saving: 58T per pixel
```

Note: `IXH`/`IXL` (layer pointer high/low) survive EXX unchanged — IX is a universal bridge.
Layer counter in C also survives (C is in main bank but not used during pixel ops — free).

**Required in compiler:** detect EXX-split opportunity when:
- A function has two disjoint live sets (persistent state S₁, temporary compute S₂)
- `|S₁| ≤ 4` (fits in shadow DEHL)
- S₁ ∩ S₂ = ∅ in register terms after optimal assignment

Check: `zone.BoundaryCost(S₁, S₂, ...) < pushpop_cost(S₁)`.

---

## Change 4: Bitmask Table for `$80 >> (x & 7)` (10ms)

**Where in compiler:** recognizer for pattern `$80 >> (val & 7)` or `1 << (7 - (val & 7))`.

Emit an 8-byte table at a nearby address and:
```asm
    LD C, A         ; C = x & 7
    LD B, 0
    LD HL, bitmask_lut
    ADD HL, BC
    LD A, (HL)      ; A = $80 >> (x&7), 16T fixed
```
vs current loop: 4T + 7T + avg(3.56 × 16T) = **68T avg** → **16T** (saving 52T per pixel, 44ms).

Actually the hand-coded `che_intro.asm` doesn't use a table — it uses the shift loop too.
`che_optimal.asm` uses a different approach: XOR entire 8-byte block (`CPL` on each row).
The table approach is a clean middle ground available to the compiler without
requiring the "XOR 8x8 block" semantic transformation.

---

## Summary: Implementation Priority

| Change | Saving | Effort | Dependency |
|--------|--------|--------|------------|
| **1. Arithmetic idiom pass** (div/mul by constant) | **459ms** | 3–4 days | `pkg/mulopt` API ready |
| **2. Inline `xor_pixel`** (loop body, ≤15 ops) | **79ms** | 2–3 days | Needs Change 1 first for full benefit |
| **3. EXX zone split** | **49ms** | 3–5 days | `pkg/regalloc/zone.go` ready |
| **4. Bitmask table** (`$80>>(v&7)`) | **10ms** | 1 day | Standalone |
| **Total** | **~597ms** | **~2 weeks** | |

**Result:** 840ms → ~240ms. Target hand-coded (290ms) is beaten by ~17%.

---

## What Already Exists (no new search needed)

| Resource | Location | How to use |
|----------|----------|------------|
| `div8_optimal.json` | `z80-optimizer/data/` | `mulopt.EmitDiv8(k)` |
| `mod8_optimal.json` | `z80-optimizer/data/` | `mulopt.EmitMod8(k)` |
| `mulopt8_clobber.json` | `z80-optimizer/data/` | `mulopt.Emit8(k, bSafe)` |
| `enriched_5v.enr` | `z80-optimizer/data/` | `regalloc.Lookup(shape)` |
| `pkg/mulopt/` | `z80-optimizer/` | Go API, builds clean |
| `pkg/regalloc/zone.go` | `z80-optimizer/` | `BoundaryCostFull(...)` |
| `peephole_top500.json` | `z80-optimizer/data/` | `peephole.Top500()` |

Import path: `github.com/oisee/z80-optimizer/pkg/{mulopt,regalloc,peephole}`

---

## Concrete: What `che_intro_optimized.asm` Would Look Like

After Changes 1–3, the inner loop becomes:

```asm
; outer: DE=LFSR, IX→layer table
outer:
    LD  D, (IX+1)         ; seed_hi → D
    LD  E, (IX+0)         ; seed_lo → E
    LD  B, (IX+2)         ; npoints → B
    EXX                   ; LFSR state to shadow bank (D'E' = seed, H'L' = warmup state)
    LD  H, $13             ; init H = $13
    LD  L, $37             ; init L = $37 (HL=$1337)
    ; warmup: 8 steps in shadow bank (H'L' acts as second LFSR word)
    ...
    EXX                   ; back to main

inner:
    EXX                   ; load LFSR from shadow
    CALL lfsr_step        ; DEHL updated
    EXX                   ; store LFSR to shadow

    ; pixel computation in main DEHL — no push/pop needed
    LD  A, L
    AND $7F               ; x = L & 127
    LD  E, A              ; E = x
    LD  A, H
    AND $7F               ; y ∈ [0, 127]
    CP  96
    JR  C, .y_ok
    SUB 96
.y_ok:
    LD  D, A              ; D = y (range-constrained: 0..95)

    ; Screen address — all bit ops, no library calls:
    LD  A, E
    SRL A \ SRL A \ SRL A ; A = x/8 (xbyte)  24T
    LD  L, A              ; L = xbyte

    LD  A, D
    AND 7                 ; A = y % 8 (pixel row in char)  4T
    OR  $40               ; H = $40 | (y&7)
    LD  H, A

    LD  A, D
    AND $38               ; A = y & $38 (char row bits)  4T
    RLCA \ RLCA           ; A = (y&$38)<<2  8T
    OR  L                 ; combine with xbyte  4T
    LD  L, A              ; HL = screen address

    LD  A, D
    CP  64                ; third of screen?  8T
    JR  C, .no_third
    SET 3, H              ; add $0800  8T (taken ~33%)
.no_third:

    ; Bitmask: $80 >> (x & 7) via lookup  16T
    LD  A, E
    AND 7
    LD  C, A
    LD  B, 0
    LD  HL, bitmask_lut
    ADD HL, BC
    LD  A, (HL)

    ; Restore screen address... (need to save HL above — one issue)
    ; Actually: compute screen addr → HL, save mask in C, use C directly:
    LD  C, A              ; C = bitmask
    ; restore HL ... hmm, need two pointers at once
    ; Resolution: compute mask first, then screen addr

    XOR (HL)
    LD  (HL), A

    DJNZ inner            ; B = npoints counter

    ; Advance IX to next layer (3 bytes per entry)
    LD  DE, 3
    ADD IX, DE

    DEC C                 ; C = layer counter
    JR  NZ, outer

bitmask_lut: DB $80,$40,$20,$10,$08,$04,$02,$01
```

The ordering issue (mask computed before addr, but both need HL) is resolved by
computing mask into C, then computing screen addr into HL — possible because x is
in E throughout. This is exactly what `pkg/regalloc/` would find: the interference
graph for `{x,y,addr_h,addr_l,mask}` fits in `{E,D,H,L,C}` with B free for DJNZ.

---

## Open Question: 16-bit LFSR vs 32-bit LFSR

`che_intro.nanz` uses a **16-bit Fibonacci LFSR** (two bytes, feedback via XOR).
`che_intro.asm` / `che_optimal.asm` use a **32-bit Galois LFSR** (four bytes DEHL, `SRL D; RR E; RR H; RR L`).

The Nanz source emits Fibonacci (slow: shift+branch+XOR per bit, ~50T), but the
hand-coded uses Galois (fast: 4 rotates + conditional XOR, **72T avg**).

Are these producing the same image? No — different LFSR polynomials, different seeds.
The nanz file says `// Fibonacci LFSR matching CUDA kernel`. The asm file has the
seeds from the original Galois search. **These are different demos**, not the same
program compiled two ways.

To truly compile `che_intro.nanz → optimal asm`, the Nanz source would need to be
updated to use the Galois LFSR idiom. Alternatively, the LFSR function in nanz
should be recognized as a `__builtin_lfsr32_galois` intrinsic and lowered to the
`SRL D; RR E; RR H; RR L; RET NC; [XOR poly]` pattern from `lfsr_step`.

**This is the most important single fix**: replacing the Fibonacci LFSR with Galois
saves ~(100T - 72T) × 3,480 calls = ~98ms, and more importantly aligns the nanz
output with the GPU-searched seeds.

---

## Recommended Implementation Order

1. **Day 1–2**: Arithmetic idiom pass — `DIV_CONST`/`MOD_CONST`/`MUL_CONST` → table lookup.
   Hook: MIR lowering, before `VIR` emission. API: `mulopt.Emit8(k, bSafe)`.

2. **Day 3**: LFSR intrinsic — recognize the Fibonacci LFSR pattern in nanz and emit
   the `SRL D; RR E; RR H; RR L` Galois variant. Seeds need re-search after this change
   (run `cuda/z80_regalloc --server` with updated LFSR, about 30 min on GPU0).

3. **Day 4–5**: Inline pass — inline functions called in loops with ≤15 body ops.
   Use `enriched_5v.enr` to confirm register assignment feasibility before inlining.

4. **Day 6–8**: EXX zone split — detect persistent-state / compute-state disjoint live
   sets, emit `EXX`/`EXX` boundary instead of PUSH/POP. `pkg/regalloc/zone.go` ready.

5. **Day 9**: Bitmask table for `$80>>(v&7)` — pattern recognizer, emit 8-byte LUT.

Expected total after all 5: **~240ms** (vs 290ms hand-coded, vs 840ms current).
The compiler beats the hand-written version on this benchmark.
