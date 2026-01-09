# Super Math ROM - Brainstorm

## Vision

A compact ROM/library with lookup tables and fast algorithms for demo effects on Z80. Target: beautiful visuals with minimal CPU cycles.

---

## 1. Sin/Cos Tables

### Basic Approach: 256-entry table
- Full circle = 256 "brads" (binary radians)
- Each entry = signed 8-bit (-128 to +127) representing -1.0 to +0.996
- **cos(x) = sin(x + 64)** - reuse same table with offset!

```
sin_table: (256 bytes)
  0,   3,   6,   9,  12,  15,  18,  21,  ; 0-7
 24,  27,  30,  33,  36,  39,  42,  45,  ; 8-15
 ...
127, 127, 127, 126, 125, 124, 122, 120,  ; 60-67 (peak)
 ...
```

### Approximation: Parabola vs Real Sine

For x in [0, π], the parabola approximation is:
```
sin(x) ≈ 4x(π-x)/π²
```

**Error analysis:**
| x (degrees) | Real sin | Parabola | Error |
|-------------|----------|----------|-------|
| 0° | 0.000 | 0.000 | 0.000 |
| 30° | 0.500 | 0.556 | **0.056** |
| 45° | 0.707 | 0.750 | 0.043 |
| 60° | 0.866 | 0.889 | 0.023 |
| 90° | 1.000 | 1.000 | 0.000 |

**Maximum error: ~5.6%** at 30° and 150°

### Hybrid Approach: Parabola + Correction Table

Idea: Store only the **difference** between parabola and real sine!

```
; First compute parabola approximation (fast)
; Then add correction from small table

parabola_approx:
    ; x in A (0-64 for quarter circle)
    LD B, A
    LD C, 64
    SUB C           ; 64 - x
    ; Multiply A * B... (need fast mult)
    ; Scale by 4/π²...

correction_table: (64 bytes for quarter, mirror for rest)
    0, -1, -2, -3, -4, -4, -5, -5, ...  ; corrections
```

This saves ~192 bytes (256 → 64) but costs some cycles.

### Quarter-table with Symmetry
```
; Full sine from quarter table (64 bytes)
; sin(x):
;   0-63:   table[x]
;   64-127: table[127-x]
;   128-191: -table[x-128]
;   192-255: -table[255-x]
```

---

## 2. Fast Multiplication

Z80 has no MUL instruction. Options:

### A. Lookup Table: a*b = ((a+b)² - (a-b)²) / 4

```
; squares_table: 512 bytes (0-255 squared, low bytes)
; squares_hi:    512 bytes (high bytes)

; To multiply A * B:
fast_mult:
    LD H, squares_table >> 8
    ; sum = a + b
    LD C, A
    ADD B
    LD L, A
    LD E, (HL)      ; (a+b)² low
    INC H
    LD D, (HL)      ; (a+b)² high
    ; diff = a - b (handle sign!)
    LD A, C
    SUB B
    JR NC, positive
    NEG             ; absolute value
positive:
    DEC H
    LD L, A
    LD A, E
    SUB (HL)        ; subtract (a-b)² low
    INC H
    LD A, D
    SBC (HL)        ; subtract (a-b)² high
    ; Divide by 4 (shift right 2)
    SRL A
    RR E
    SRL A
    RR E
    ; Result in DE (or just E for 8-bit)
```

**Cost:** 512 bytes for 8×8→16 multiply

### B. Russian Peasant (shift-and-add)
```
; A * B using shifts
mult_8x8:
    LD C, 0         ; result high
    LD D, 0
    LD E, A         ; multiplicand
mult_loop:
    SRL B           ; shift multiplier right
    JR NC, no_add
    ADD E           ; add multiplicand
    ADC C, D        ; with carry
no_add:
    SLA E           ; shift multiplicand left
    RL D
    OR B
    JR NZ, mult_loop
    RET
```

**Cost:** 0 bytes table, but slower (~100-200 T-states)

### C. Hybrid: Small table for common multipliers
```
; Tables for ×2, ×3, ×5, ×7 etc.
mult_3_table:  ; 256 bytes
    0, 3, 6, 9, 12, ...
```

---

## 3. Fixed-Point Formats

### 8.8 Format (16-bit)
- High byte = integer part (-128 to 127)
- Low byte = fractional part (0.00390625 precision)
- Range: -128.0 to +127.996

### 4.4 Format (8-bit)
- High nibble = integer (-8 to 7)
- Low nibble = fraction (1/16 precision)
- Compact but limited range

### 1.7 Format for sin/cos
- 1 sign bit + 7 fraction bits
- Perfect for -1.0 to +0.992 range
- Direct use in calculations

---

## 4. Matrix Operations (3D effects)

### 2D Rotation Matrix
```
| cos(θ)  -sin(θ) |   | x |   | x' |
| sin(θ)   cos(θ) | × | y | = | y' |

x' = x*cos(θ) - y*sin(θ)
y' = x*sin(θ) + y*cos(θ)
```

With lookup tables:
```asm
rotate_point:
    ; Input: B=x, C=y, A=angle (0-255)
    ; Get sin and cos
    LD H, sin_table >> 8
    LD L, A
    LD D, (HL)      ; sin(θ)
    LD A, L
    ADD 64          ; cos = sin + 90°
    LD L, A
    LD E, (HL)      ; cos(θ)

    ; x' = x*cos - y*sin
    LD A, B
    CALL signed_mult_E  ; A*E = x*cos
    LD H, A         ; save
    LD A, C
    CALL signed_mult_D  ; A*D = y*sin
    LD L, A
    LD A, H
    SUB L           ; x' = x*cos - y*sin
    ; ... similar for y'
```

### 3D Rotation (simplified)
For demos, often use pre-computed rotation matrices at fixed angles (e.g., every 4°).

---

## 5. Other Useful Functions

### Square Root (8-bit)
```
; sqrt_table: 256 bytes
; sqrt_table[x] = floor(sqrt(x))
sqrt_table:
    0, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, ; 0-15
    4, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, ; 16-31
    ...
```

### Atan2 (for rotations)
Expensive to compute, better to use lookup table:
```
; atan2_table: 256 bytes (or 64 with symmetry)
; Returns angle 0-255 for ratio y/x
```

### Random Numbers (LFSR)
```asm
; 16-bit LFSR, period 65535
random:
    LD HL, (seed)
    LD A, H
    XOR L
    LD A, H
    RRA
    XOR H
    LD H, A
    LD A, L
    RRA
    XOR L
    LD L, A
    LD (seed), HL
    RET
```

### Plasma Effect
Uses sin table with multiple frequencies:
```
plasma(x, y) = sin(x) + sin(y) + sin(x+y) + sin(sqrt(x²+y²))
```

### Fade/Interpolation
```
; lerp(a, b, t) = a + (b-a)*t/256
lerp:
    LD C, A         ; save a
    SUB B           ; b - a (signed!)
    ; multiply by t
    CALL mult_signed
    ADD C           ; + a
    RET
```

---

## 6. ROM Layout Proposal

```
$0000-$00FF: sin_table (256 bytes) - also used for cos with +64 offset
$0100-$01FF: squares_lo (256 bytes)
$0200-$02FF: squares_hi (256 bytes)
$0300-$03FF: sqrt_table (256 bytes)
$0400-$043F: atan2_table (64 bytes, use symmetry)
$0440-$04FF: Reserved
$0500-$05FF: Fast routines
  - rotate_point
  - fast_mult_8x8
  - fast_mult_16x16
  - random
  - lerp
  - sqrt (uses table)
  - atan2 (uses table)
$0600-$07FF: Demo-specific tables (tunnel, plasma, etc.)

Total: 2KB
```

---

## 7. Optimization Tricks

### Unrolled inner loops
For fixed-size operations, unroll to avoid loop overhead:
```asm
; 8 pixels instead of loop
    LD A, (HL) : OUT (C), A : INC HL
    LD A, (HL) : OUT (C), A : INC HL
    LD A, (HL) : OUT (C), A : INC HL
    ...
```

### Self-modifying for speed
```asm
    LD A, value
    LD (smc_target+1), A  ; patch immediate
smc_target:
    LD B, 0               ; becomes LD B, value
```

### Pre-shifted tables
For sprites: store 8 versions pre-shifted by 0-7 pixels.

### Delta encoding
For animations: store differences, not absolute values.

---

## Questions to Resolve

1. **Sin table precision:** 7-bit vs 8-bit values?
2. **Multiply method:** Table (fast, big) vs Russian Peasant (slow, small)?
3. **3D:** Pre-computed rotation matrices or real-time calculation?
4. **Memory budget:** 1KB? 2KB? 4KB?
5. **Integration:** Inline assembly or MinZ stdlib?

---

## Next Steps

1. Implement sin_table generator in MinZ
2. Benchmark multiply methods on real Z80
3. Create rotate_point demo
4. Build plasma effect using tables
5. Package as MinZ stdlib module
