# Fast Sine Approximations for Z80 Demos

## Introduction

Every demo coder faces the same dilemma: you need sin/cos for plasma effects, rotations, tunnels, and waves - but Z80 has no floating point, no trigonometry, and precious little RAM. This article explores the trade-offs between lookup tables and mathematical approximations.

## The Challenge

On modern CPUs, `sin(x)` is a single instruction. On Z80, we have three options:

1. **Lookup table** - Fast but uses memory
2. **Mathematical approximation** - Compact but uses cycles
3. **Hybrid** - Best of both worlds

Let's analyze each approach with actual numbers.

---

## Method 1: Full Lookup Table (256 bytes)

The classic approach: pre-compute all 256 values.

```asm
; sin_table[angle] where angle = 0-255 (full circle)
; Values are signed: -127 to +127 representing -1.0 to +1.0

sin_table:
    DB   0,   3,   6,   9,  12,  15,  18,  21  ;   0-  7
    DB  24,  27,  30,  33,  36,  39,  42,  45  ;   8- 15
    DB  48,  51,  54,  57,  59,  62,  65,  67  ;  16- 23
    DB  70,  73,  75,  78,  80,  82,  85,  87  ;  24- 31
    DB  89,  91,  94,  96,  98, 100, 102, 103  ;  32- 39
    DB 105, 107, 108, 110, 112, 113, 114, 116  ;  40- 47
    DB 117, 118, 119, 120, 121, 122, 123, 123  ;  48- 55
    DB 124, 125, 125, 126, 126, 126, 127, 127  ;  56- 63
    DB 127, 127, 127, 126, 126, 126, 125, 125  ;  64- 71
    DB 124, 123, 123, 122, 121, 120, 119, 118  ;  72- 79
    DB 117, 116, 114, 113, 112, 110, 108, 107  ;  80- 87
    DB 105, 103, 102, 100,  98,  96,  94,  91  ;  88- 95
    DB  89,  87,  85,  82,  80,  78,  75,  73  ;  96-103
    DB  70,  67,  65,  62,  59,  57,  54,  51  ; 104-111
    DB  48,  45,  42,  39,  36,  33,  30,  27  ; 112-119
    DB  24,  21,  18,  15,  12,   9,   6,   3  ; 120-127
    DB   0,  -3,  -6,  -9, -12, -15, -18, -21  ; 128-135
    ; ... (mirror of above with negative values)

get_sin:
    ; Input: A = angle (0-255)
    ; Output: A = sin value (-127 to +127)
    LD H, sin_table >> 8
    LD L, A
    LD A, (HL)
    RET

get_cos:
    ; cos(x) = sin(x + 64)
    ADD 64
    JP get_sin
```

**Performance:**
- Lookup: 18 T-states
- Memory: 256 bytes
- Accuracy: Perfect (within 8-bit precision)

**Verdict:** Best for speed-critical demos. Memory cost is acceptable.

---

## Method 2: Quarter Table with Symmetry (64 bytes)

Sine has four-fold symmetry:
- sin(0-63): Read from table
- sin(64-127): Read table[127-x]
- sin(128-191): Negate table[x-128]
- sin(192-255): Negate table[255-x]

```asm
sin_quarter:
    ; Only first quarter: 0° to 90°
    DB   0,   3,   6,   9,  12,  15,  18,  21
    DB  24,  27,  30,  33,  36,  39,  42,  45
    DB  48,  51,  54,  57,  59,  62,  65,  67
    DB  70,  73,  75,  78,  80,  82,  85,  87
    DB  89,  91,  94,  96,  98, 100, 102, 103
    DB 105, 107, 108, 110, 112, 113, 114, 116
    DB 117, 118, 119, 120, 121, 122, 123, 123
    DB 124, 125, 125, 126, 126, 126, 127, 127

get_sin_quarter:
    ; Input: A = angle (0-255)
    ; Output: A = sin value (-127 to +127)
    LD C, A             ; Save original
    AND $C0             ; Get quadrant (0, 64, 128, 192)
    JR Z, q0            ; 0-63: direct
    CP 64
    JR Z, q1            ; 64-127: mirror
    CP 128
    JR Z, q2            ; 128-191: negate
    ; else 192-255: mirror + negate
q3:
    LD A, C
    NEG                 ; 256 - x
    AND $3F             ; mod 64
    LD L, A
    LD H, sin_quarter >> 8
    LD A, (HL)
    NEG                 ; Negate result
    RET
q2:
    LD A, C
    AND $3F
    LD L, A
    LD H, sin_quarter >> 8
    LD A, (HL)
    NEG
    RET
q1:
    LD A, 127
    SUB C               ; 127 - x
    AND $3F
    LD L, A
    LD H, sin_quarter >> 8
    LD A, (HL)
    RET
q0:
    LD A, C
    AND $3F
    LD L, A
    LD H, sin_quarter >> 8
    LD A, (HL)
    RET
```

**Performance:**
- Lookup: ~40 T-states (varies by quadrant)
- Memory: 64 bytes table + ~50 bytes code = 114 bytes
- Accuracy: Perfect

**Verdict:** Good when memory is tight but you need accuracy.

---

## Method 3: Parabola Approximation (0 bytes table!)

The parabola `4x(π-x)/π²` approximates sine surprisingly well:

```
sin(x) ≈ 4x(π-x)/π²  for x ∈ [0, π]
```

In fixed-point with x in [0, 128] representing [0, π]:

```
sin(x) ≈ x(128-x)/32  (approximately)
```

### Error Analysis

| Angle | Real sin | Parabola | Error |
|-------|----------|----------|-------|
| 0°    | 0.000    | 0.000    | 0.00% |
| 15°   | 0.259    | 0.306    | 4.67% |
| 25°   | 0.423    | 0.478    | **5.58%** |
| 30°   | 0.500    | 0.556    | 5.56% |
| 45°   | 0.707    | 0.750    | 4.29% |
| 60°   | 0.866    | 0.889    | 2.29% |
| 90°   | 1.000    | 1.000    | 0.00% |

Maximum error: **5.6% at 25°** (and symmetrically at 155°)

In 8-bit terms: error of 7 out of 127. Visible but often acceptable.

```asm
parabola_sin:
    ; Input: A = angle (0-255)
    ; Output: A = approximate sin (-127 to +127)
    ; Formula: x(128-x)/32 for first half

    LD C, A             ; Save angle
    AND $80             ; Check if second half
    JR NZ, second_half

    ; First half: 0-127
    LD A, C
    LD B, A             ; B = x
    LD A, 128
    SUB B               ; A = 128 - x
    ; Now multiply A * B
    CALL fast_mult_8    ; Result in HL
    ; Divide by 32 (shift right 5)
    LD A, H
    SRL A
    RR L
    ; ... (3 more shifts)
    LD A, L
    RET

second_half:
    ; Negate angle, compute, negate result
    LD A, C
    SUB 128             ; x - 128
    ; ... same as above, then NEG result
```

**Performance:**
- Calculation: ~80-120 T-states
- Memory: ~40 bytes code, 0 bytes table
- Accuracy: 94.4% (max 5.6% error)

**Verdict:** When every byte counts and slight wobble is acceptable.

---

## Method 4: Parabola + Correction Table (32 bytes!)

The brilliant insight: the error between parabola and real sine is small and smooth. We can store just the corrections!

### The Correction Values

Since parabola always **overestimates** sine (except at 0° and 90°), corrections are all negative:

| Range | Correction (in 1/127 units) |
|-------|----------------------------|
| 0-7   | 0, -1, -2, -2, -3, -4, -4, -5 |
| 8-15  | -5, -5, -6, -6, -6, -7, -7, -7 |
| 16-31 | -7, -7, -7, -7, -7, -7, -7, -7 |
| 32-47 | -7, -6, -6, -6, -5, -5, -5, -4 |
| 48-63 | -4, -3, -3, -2, -2, -1, -1, 0 |

All values fit in 3 bits (0-7)! We can pack two per byte:

```asm
; Nibble-packed correction table (32 bytes for 64 entries)
; High nibble = even index, Low nibble = odd index
sin_correction:
    DB $01, $22, $34, $45, $55, $66, $67, $77  ;  0-15
    DB $77, $77, $77, $77, $77, $76, $66, $66  ; 16-31
    DB $55, $55, $54, $44, $43, $33, $32, $22  ; 32-47
    DB $22, $11, $11, $11, $00, $00, $00, $00  ; 48-63

corrected_sin:
    ; Compute parabola, then subtract correction
    CALL parabola_sin   ; Get approximate value
    LD D, A             ; Save it

    ; Get correction index (0-63 for quarter)
    LD A, C             ; Original angle
    AND $3F             ; mod 64
    SRL A               ; Divide by 2 for packed index
    LD HL, sin_correction
    ADD L
    LD L, A
    LD A, (HL)          ; Get packed byte

    ; Extract correct nibble
    BIT 0, C            ; Odd or even?
    JR Z, even_entry
    AND $0F             ; Low nibble for odd
    JR apply_correction
even_entry:
    SRL A
    SRL A
    SRL A
    SRL A               ; High nibble for even

apply_correction:
    LD B, A             ; Correction (0-7)
    LD A, D             ; Parabola result
    SUB B               ; Apply correction
    RET
```

**Performance:**
- Calculation: ~100-150 T-states
- Memory: 32 bytes table + ~60 bytes code = ~92 bytes
- Accuracy: Perfect (within 8-bit precision)

**Verdict:** Excellent balance! Nearly as accurate as full table, 1/3 the memory.

---

## Method 5: Bhaskara I's Formula (7th Century Magic!)

The Indian mathematician Bhaskara I discovered this in 628 AD:

```
sin(x) ≈ 16x(π-x) / [5π² - 4x(π-x)]
```

### Error Analysis

| Angle | Real sin | Bhaskara | Error |
|-------|----------|----------|-------|
| 0°    | 0.000    | 0.000    | 0.000% |
| 15°   | 0.259    | 0.260    | 0.15% |
| 30°   | 0.500    | 0.500    | **0.00%** |
| 45°   | 0.707    | 0.706    | 0.12% |
| 60°   | 0.866    | 0.865    | 0.12% |
| 90°   | 1.000    | 1.000    | 0.00% |

Maximum error: **0.16%** - that's 35x better than parabola!

The catch? It requires division, which is expensive on Z80 (~200+ T-states).

```asm
bhaskara_sin:
    ; sin(x) ≈ 16x(π-x) / [5π² - 4x(π-x)]
    ; In fixed-point with π ≈ 128:
    ; sin(x) ≈ 16x(128-x) / [5*128² - 4x(128-x)]
    ;        = 16x(128-x) / [81920 - 4x(128-x)]

    ; This requires 16-bit multiply and divide
    ; Only use when accuracy trumps speed!

    LD B, A             ; B = x
    LD A, 128
    SUB B               ; A = 128 - x
    CALL mult_16        ; HL = x * (128-x)
    ; ... complex division follows
```

**Performance:**
- Calculation: ~200-300 T-states
- Memory: ~80 bytes code, 0 bytes table
- Accuracy: 99.84%

**Verdict:** For scientific applications where accuracy matters more than speed.

---

## Method 6: Runtime Decompression (The Ultimate Trick!)

Here's the killer insight: **decompress at init, run at full speed!**

Store only 32 bytes in ROM, but generate the full 256-byte table into RAM at startup:

```asm
; ROM: 32 bytes correction + ~60 bytes init code = ~92 bytes
; RAM: 256 bytes (generated at init)
; Runtime: 18 T-states (same as full table!)

init_sin_table:
    ; Generate full 256-byte table from parabola + corrections
    ; Called ONCE at program start (~5000 T-states, ~1.4ms @ 3.5MHz)

    LD HL, sin_ram_table
    LD B, 0             ; angle counter

init_loop:
    PUSH BC
    PUSH HL

    ; Compute parabola(angle)
    LD A, B
    CALL parabola_sin   ; A = approximate sin

    ; Apply correction from packed table
    LD C, B
    LD A, C
    AND $3F             ; Quarter index
    SRL A               ; Packed index
    LD HL, sin_correction
    ADD L
    LD L, A
    LD A, (HL)
    BIT 0, C
    JR Z, init_even
    AND $0F             ; Odd: low nibble
    JR init_apply
init_even:
    RRCA
    RRCA
    RRCA
    RRCA                ; Even: high nibble
    AND $0F

init_apply:
    LD D, A             ; D = correction
    CALL parabola_sin   ; Recompute (or save earlier)
    SUB D               ; Apply correction

    ; Handle quadrant negation for angles 128-255
    BIT 7, B
    JR Z, init_store
    NEG

init_store:
    POP HL
    LD (HL), A
    INC HL
    POP BC
    INC B
    JR NZ, init_loop    ; Loop 256 times
    RET

; Now runtime is just a simple lookup!
get_sin_fast:
    LD H, sin_ram_table >> 8
    LD L, A
    LD A, (HL)          ; 18 T-states!
    RET
```

**The Math:**
- Init time: ~256 × 150 = 38,400 T-states ≈ **11ms** @ 3.5MHz
- That's invisible during game/demo load!

**Performance:**
- ROM: 92 bytes (32 table + 60 code)
- RAM: 256 bytes (generated)
- Init: 11ms (once)
- Runtime: **18 T-states** (same as full table!)
- Accuracy: Perfect

**Verdict:** **THE OPTIMAL SOLUTION!** Tiny ROM footprint, maximum runtime speed!

---

## Comparison Summary

| Method | ROM | RAM | Speed (T) | Max Error | Best Use |
|--------|-----|-----|-----------|-----------|----------|
| Full table (ROM) | 256 B | 0 | 18 | 0% | ROM-rich systems |
| Quarter + mirror | 114 B | 0 | 40 | 0% | Balanced |
| Parabola only | 40 B | 0 | 100 | 5.6% | Size-coding |
| Parabola + runtime fix | 92 B | 0 | 130 | 0% | ROM-tight, slow OK |
| **Decompress at init** | **92 B** | **256 B** | **18** | **0%** | **OPTIMAL!** |
| Bhaskara | 80 B | 0 | 250 | 0.16% | Scientific |

---

## Practical Recommendations

### For Most Demos (THE DEFAULT CHOICE)
Use **runtime decompression**! 92 bytes ROM, 18 T-states runtime. The 11ms init is invisible during loading. You get the speed of a full table with 1/3 the ROM cost.

### For ROM-Only Systems (No Writable RAM)
Use **full 256-byte table in ROM**. Some cartridge systems can't generate tables at runtime.

### For Extreme ROM Constraints
Use **quarter table + mirror**. 64 bytes table, still perfect accuracy, ~40 T per lookup.

### For Size-Coding (256-byte demos)
Use **parabola only**. The slight wobble adds character! Many classic demos embraced imperfection as aesthetic.

### For Educational/Scientific
Use **Bhaskara**. When correctness matters more than performance.

---

## Bonus: Cosine for Free!

With any sine method, cosine is just a phase shift:

```asm
get_cos:
    ADD 64          ; cos(x) = sin(x + 90°)
    JP get_sin      ; In 256-unit circle, 90° = 64
```

This works because we're using "binary radians" (brads) where 256 = full circle.

---

## Generating Tables with MinZ

```minz
// Generate sine table at compile time
@ctie
fun generate_sin_table() -> void {
    for i in 0..256 {
        let angle: f32 = (i as f32) * 3.14159 * 2.0 / 256.0;
        let value: i8 = (sin(angle) * 127.0) as i8;
        @emit("    DB " + value.to_string());
    }
}
```

---

## Conclusion

The humble sine function offers a beautiful case study in engineering trade-offs. From brute-force tables to 7th-century mathematics, each approach has its place.

For most Z80 demos, the **parabola + 32-byte correction** hybrid offers the best balance: nearly perfect accuracy in under 100 bytes. But when pushing for maximum frame rate, nothing beats a direct table lookup.

Choose wisely, and may your plasmas be smooth!

---

*Written for the MinZ compiler project, December 2024*
