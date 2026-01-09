# MinZ Shader Library Vision

**Date:** January 5, 2026
**Status:** Research & Planning
**Inspiration:** ZXSpeculator Experiments (DeanTheCoder)

## Executive Summary

After analyzing the stunning shader demos in ZXSpeculator's Experiments folder - raymarched 3D scenes, fire effects, procedural graphics - we're convinced that GLSL-style programming belongs in MinZ. This document outlines a path to bring shader-like capabilities to Z80.

## Source Material Analysis

### glsl.h - GLSL-Style Vector Math in C

The library provides familiar GLSL types and operations:

```c
// Types
typedef struct vec2_t { float x, y; } vec2_t;
typedef struct vec3_t { float x, y, z; } vec3_t;
typedef struct vec4_t { float x, y, z, w; } vec4_t;
typedef struct mat2_t { vec2_t c1, c2; } mat2_t;

// 50+ operations including:
// - Arithmetic: add, sub, mul, div (vector & scalar variants)
// - Geometric: dot, cross, length, normalize
// - Math: sin, cos, pow, floor, fract, abs
// - Utility: min, max, clamp, mix
// - Rotation: rot_xy, rot_xz, rot_yz
```

### OneSmallStep.c - Raymarched Lunar Scene

A complete raymarcher running on ZX Spectrum:

```c
// SDF for box primitive
static float box(vec3_t p, vec3_t b) {
    vec3_t q = *v3_sub(v3_abs(&p), &b);
    return v3_length(v3_maxf(&q, 0.0f)) + min(max(q.x, max(q.y, q.z)), 0.0f);
}

// Raymarching loop (15 iterations)
static float march(vec3_t ro, vec3_t rd) {
    vec3_t p;
    float d = .01f;
    for (float i = 0.0f; i < 15.0f; ++i) {
        p = *v3_add(&ro, v3_mulf(&rd, d));
        float h = map(p);
        if (abs(h) < .025f) break;
        d += h;
    }
    return lights(p) * exp(-d * .14);
}

// Floyd-Steinberg dithering for 1-bit output
for (uint16_t x = 1; x < 255; ++x) {
    err1[x + 1] += pixelError * 0.4375f;  // 7/16
    err2[x - 1] += pixelError * 0.1875f;  // 3/16
    err2[x] += pixelError * 0.3125f;      // 5/16
    err2[x + 1] += pixelError * 0.0625f;  // 1/16
}
```

## MinZ Port Strategy

### Phase 1: Fixed-Point Math Foundation

Replace floats with 8.8 or 12.4 fixed-point:

```minz
// Fixed-point type (8.8 format: 256 = 1.0)
type fixed = i16;

const FP_ONE: fixed = 256;
const FP_HALF: fixed = 128;

// Multiply two fixed-point numbers
fun fp_mul(a: fixed, b: fixed) -> fixed {
    let result: i32 = (a as i32) * (b as i32);
    return (result >> 8) as fixed;
}

// Divide with fixed-point
fun fp_div(a: fixed, b: fixed) -> fixed {
    let result: i32 = ((a as i32) << 8) / (b as i32);
    return result as fixed;
}
```

### Phase 2: Vector Types

```minz
struct vec2 { x: fixed, y: fixed }
struct vec3 { x: fixed, y: fixed, z: fixed }
struct vec4 { x: fixed, y: fixed, z: fixed, w: fixed }

// Vector operations
fun v2_add(a: vec2, b: vec2) -> vec2 {
    return vec2 { x: a.x + b.x, y: a.y + b.y };
}

fun v2_dot(a: vec2, b: vec2) -> fixed {
    return fp_mul(a.x, b.x) + fp_mul(a.y, b.y);
}

fun v2_length(v: vec2) -> fixed {
    let sq = v2_dot(v, v);
    return fp_sqrt(sq);  // Lookup table or Newton-Raphson
}
```

### Phase 3: Lookup Tables for Transcendentals

```minz
// 256-entry sine table (one quadrant, mirror for full circle)
global SIN_TABLE: [fixed; 64] = [
    0, 6, 13, 19, 25, 31, 38, 44,
    50, 56, 62, 68, 74, 80, 86, 92,
    // ... full table ...
];

fun fp_sin(angle: fixed) -> fixed {
    let idx = (angle >> 2) & 0x3F;  // Map to 0-63
    let quadrant = (angle >> 8) & 0x03;

    case quadrant {
        0 => return SIN_TABLE[idx],
        1 => return SIN_TABLE[63 - idx],
        2 => return -SIN_TABLE[idx],
        3 => return -SIN_TABLE[63 - idx]
    }
}

fun fp_cos(angle: fixed) -> fixed {
    return fp_sin(angle + 64);  // cos = sin(x + 90°)
}
```

### Phase 4: SDF Primitives

```minz
// Signed distance to sphere
fun sdSphere(p: vec3, r: fixed) -> fixed {
    return v3_length(p) - r;
}

// Signed distance to box
fun sdBox(p: vec3, b: vec3) -> fixed {
    let q = vec3 {
        x: fp_abs(p.x) - b.x,
        y: fp_abs(p.y) - b.y,
        z: fp_abs(p.z) - b.z
    };
    let outside = v3_length(v3_max(q, VEC3_ZERO));
    let inside = fp_min(fp_max(q.x, fp_max(q.y, q.z)), 0);
    return outside + inside;
}

// Union of two SDFs
fun opUnion(d1: fixed, d2: fixed) -> fixed {
    return fp_min(d1, d2);
}

// Smooth union
fun opSmoothUnion(d1: fixed, d2: fixed, k: fixed) -> fixed {
    let h = fp_clamp(FP_HALF + fp_div(d2 - d1, k << 1), 0, FP_ONE);
    return fp_mix(d2, d1, h) - fp_mul(fp_mul(k, h), FP_ONE - h);
}
```

### Phase 5: Raymarching Framework

```minz
const MAX_STEPS: u8 = 15;
const MAX_DIST: fixed = 2560;  // 10.0 in 8.8
const SURF_DIST: fixed = 2;    // 0.008 in 8.8

fun raymarch(ro: vec3, rd: vec3) -> fixed {
    let d: fixed = 0;

    for step in 0..MAX_STEPS {
        let p = v3_add(ro, v3_mulf(rd, d));
        let ds = map(p);  // Scene SDF
        d = d + ds;
        if ds < SURF_DIST { break; }
        if d > MAX_DIST { break; }
    }

    return d;
}
```

### Phase 6: Dithering

```minz
// Floyd-Steinberg error distribution weights (in 16ths)
const FS_RIGHT: u8 = 7;      // 7/16
const FS_DOWN_LEFT: u8 = 3;  // 3/16
const FS_DOWN: u8 = 5;       // 5/16
const FS_DOWN_RIGHT: u8 = 1; // 1/16

fun floyd_steinberg_line(pixels: *[u8; 256], errors: *[i8; 258]) {
    for x in 1..255 {
        let val = pixels[x] + errors[x];
        let out = if val >= 128 { 255 } else { 0 };
        let err = val - out;

        // Distribute error
        errors[x + 1] = errors[x + 1] + ((err * FS_RIGHT) >> 4);
        // ... next row errors stored separately

        if out > 0 { plot(x, current_y); }
    }
}
```

## Z80 Optimization Opportunities

### DJNZ for Iteration Loops
```asm
; Raymarching 15 iterations
    LD B, 15
.march_loop:
    ; ... march step ...
    DJNZ .march_loop
```

### Lookup Tables in ROM
```asm
; Sin table access
    LD L, A           ; Index
    LD H, HIGH(SIN_TABLE)
    LD A, (HL)        ; 7 T-states!
```

### Fixed-Point Multiply via Shift
```asm
; Multiply by 0.5 (>> 1)
    SRA H
    RR L
```

### Inline Screen Plotting
```asm
; Direct attribute memory access
    LD HL, $5800 + offset
    LD (HL), color
```

## Memory Budget (48K Spectrum)

| Component | Size | Notes |
|-----------|------|-------|
| Code | ~8KB | Raymarcher + dithering |
| Sin/Cos table | 256B | One quadrant |
| Sqrt table | 512B | For length calculations |
| Error buffers | 512B | Two lines for dithering |
| Screen | 6912B | Standard Spectrum |
| **Available** | ~32KB | Scene data, textures |

## Performance Targets

| Metric | Target | Notes |
|--------|--------|-------|
| Resolution | 256x192 | Full Spectrum screen |
| Raymarch steps | 8-15 | Adjustable quality |
| Frame time | 10-60s | Static image OK |
| Fixed-point | 8.8 | Good precision/speed |

## Example: MinZ Sphere Demo

```minz
import glsl;

const SCREEN_W: fixed = 256 << 8;  // 256.0
const SCREEN_H: fixed = 192 << 8;  // 192.0

fun map(p: vec3) -> fixed {
    return sdSphere(p, 128);  // Radius 0.5
}

fun main() -> void {
    let ro = vec3 { x: 0, y: 0, z: -1024 };  // Camera at z=-4

    for y in 0..192 {
        for x in 0..256 {
            let uv = vec2 {
                x: ((x << 8) - (SCREEN_W >> 1)) / 192,
                y: ((y << 8) - (SCREEN_H >> 1)) / 192
            };
            let rd = v3_normalize(vec3 { x: uv.x, y: uv.y, z: 256 });

            let d = raymarch(ro, rd);
            let brightness = 255 - (d >> 2);

            if brightness > 128 {
                plot(x, y);
            }
        }
    }

    loop { asm { EI; HALT } }
}
```

## Next Steps

1. **Implement fixed-point library** - Core math operations
2. **Create lookup tables** - Sin, cos, sqrt
3. **Port vec2/vec3 operations** - Start with 2D
4. **Simple raymarcher** - Sphere first
5. **Add dithering** - Floyd-Steinberg
6. **Optimize hotspots** - Profile and inline assembly
7. **Port OneSmallStep** - Ultimate goal!

## Acknowledgments

Massive thanks to DeanTheCoder for the ZXSpeculator experiments that prove sophisticated graphics are possible on Z80. The glsl.h library is a brilliant piece of work that directly inspired this vision.

---

*"Shaders on Z80 - because we can."*
