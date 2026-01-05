# MinZ GLSL Shader Library - v0.16.4

**Date:** January 5, 2026
**Codename:** "Shaders on Z80"

## Executive Summary

This release introduces a complete GLSL-style shader library for MinZ, bringing modern graphics programming paradigms to vintage Z80 hardware. Inspired by ZXSpeculator's stunning OneSmallStep lunar raymarcher, we've created a comprehensive toolkit for fixed-point math, vector operations, SDFs, and raymarching.

**Key Achievement:** A 410-line MinZ raymarched sphere demo compiles to 4,384 lines of optimized Z80 assembly with DJNZ loops, True SMC, and 95 inlined function calls!

## New Library: stdlib/glsl/

### Module Overview

| Module | Purpose | Key Functions |
|--------|---------|---------------|
| `fp.minz` | 8.8 Fixed-point math | `fp_mul`, `fp_div`, `fp_sqrt`, `fp_exp` |
| `trig.minz` | Trigonometry (lookup tables) | `fp_sin`, `fp_cos`, `fp_tan`, `fp_atan2` |
| `vec.minz` | GLSL-style vectors | `v3_dot`, `v3_cross`, `v3_normalize` |
| `sdf.minz` | Signed Distance Functions | `sdSphere`, `sdBox`, `opSmoothUnion` |
| `raymarch.minz` | Raymarching framework | `raymarch`, `calcLighting`, `applyFog` |
| `dither.minz` | Dithering algorithms | Floyd-Steinberg, Bayer, Blue Noise |
| `glsl.minz` | Main module (re-exports all) | - |

### Fixed-Point Math (8.8 Format)

The library uses 8.8 fixed-point where 256 = 1.0:

```minz
// Multiply: (a * b) >> 8 using Z80 assembly
fun fp_mul(a: i16, b: i16) -> i16 {
    asm {
        ; 16x16 signed multiply, return middle 16 bits
        ; Handles sign, uses shift-add algorithm
        ; ~200 T-states per multiply
    }
}

// Square root via Newton-Raphson (4 iterations)
fun fp_sqrt(x: i16) -> i16 {
    let guess: i16 = x >> 1;
    for i in 0..4 {
        guess = (guess + fp_div(x, guess)) >> 1;
    }
    return guess;
}
```

### Vector Operations

Full GLSL-style vec2/vec3/vec4 with all standard operations:

```minz
struct vec3 { x: i16, y: i16, z: i16 }

fun v3_dot(a: vec3, b: vec3) -> i16 {
    return fp_mul(a.x, b.x) + fp_mul(a.y, b.y) + fp_mul(a.z, b.z);
}

fun v3_normalize(v: vec3) -> vec3 {
    let len = v3_length(v);
    return vec3 {
        x: fp_div(v.x, len),
        y: fp_div(v.y, len),
        z: fp_div(v.z, len)
    };
}
```

### Signed Distance Functions

Complete SDF primitive library:

```minz
// Sphere SDF
fun sdSphere(p: vec3, r: i16) -> i16 {
    return v3_length(p) - r;
}

// Smooth union for organic shapes
fun opSmoothUnion(d1: i16, d2: i16, k: i16) -> i16 {
    let h = fp_clamp(FP_HALF + fp_div(fp_mul(FP_HALF, d2 - d1), k), 0, FP_ONE);
    return fp_mix(d2, d1, h) - fp_mul(fp_mul(k, h), FP_ONE - h);
}
```

### Raymarching

15-iteration raymarcher optimized for Z80:

```minz
fun raymarch(ro: vec3, rd: vec3) -> i16 {
    let d: i16 = 0;
    for step in 0..15 {  // DJNZ optimized!
        let p = v3_add(ro, v3_mulf(rd, d));
        let ds = map(p);
        if ds < SURF_DIST { return d; }
        d = d + ds;
    }
    return -1;
}
```

## Working Demo: glsl_sphere_demo.minz

A complete self-contained raymarched sphere with:
- Camera setup and ray generation
- 15-step raymarching
- Surface normal calculation
- Diffuse lighting
- Distance fog
- Bayer 4x4 dithering

### Compilation Results

| Metric | Value |
|--------|-------|
| Source lines | 410 |
| MIR instructions | 986 |
| Z80 assembly lines | 4,384 |
| Functions | 24 |
| DJNZ loops | 4 |
| True SMC calls | 30 |
| Inlined calls | 95 |
| Constant folding | 11 |
| Peephole patterns | 30 |

### Generated Code Quality

The compiler produces highly optimized Z80:

```asm
; Raymarching loop - DJNZ optimized
___raymarch_djnz_loop:
    CALL v3_mulf          ; p = ro + rd * d
    CALL v3_add
    CALL map              ; ds = scene(p)
    ; ... distance check ...
    DJNZ ___raymarch_djnz_loop

; Fixed-point multiply - pure Z80 assembly
.mul_loop:
    ADD HL, HL            ; Shift-add algorithm
    RL E
    RL D
    JR NC, .no_add
    ADD HL, BC
.no_add:
    DEC A
    JR NZ, .mul_loop
    LD L, H               ; Result >> 8
    LD H, E
```

## Performance Characteristics

| Operation | Estimated T-states |
|-----------|-------------------|
| `fp_mul` | ~200 |
| `fp_div` | ~300 |
| `fp_sqrt` (4 iter) | ~1,400 |
| `v3_dot` | ~700 |
| `v3_normalize` | ~2,500 |
| `map` (sphere) | ~800 |
| Full raymarch (15 steps) | ~50,000 |
| Per-pixel render | ~60,000 |

For a 64x48 quarter-resolution render: ~185M T-states (~53 seconds at 3.5MHz)

## Future Vision

This library opens the door to:

1. **Port OneSmallStep** - Full lunar scene with procedural terrain
2. **Fire effects** - Procedural flames using noise functions
3. **Plasma demos** - Classic demoscene effects
4. **Simple games** - 3D collision detection via SDFs
5. **Art tools** - Procedural texture generation

## Installation

The shader library is included in the MinZ stdlib:

```bash
cd minz/minzc
make build
```

To compile the demo:

```bash
./main examples/glsl_sphere_demo.minz -o sphere.a80
```

## Acknowledgments

Massive thanks to **DeanTheCoder** and the **ZXSpeculator** project for proving that sophisticated 3D graphics are possible on Z80. The OneSmallStep lunar raymarcher directly inspired this library.

---

*"Shaders on Z80 - because we can."*
