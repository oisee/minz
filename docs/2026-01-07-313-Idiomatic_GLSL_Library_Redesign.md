# Idiomatic GLSL Library Redesign

**Status**: IN PROGRESS (January 2026)
**Version**: v0.17.0
**Prerequisite**: ADR-312 (Operator Overloading)

## Overview

Redesign the GLSL shader library to leverage MinZ's new operator overloading and UFCS features for truly idiomatic, GLSL-like syntax.

## Before vs After

### Vector Math

**Before (v0.16):**
```minz
let result = v3_add(v3_mulf(normal, intensity), ambient);
let reflected = v3_sub(dir, v3_mulf(normal, fp_mul(FP_TWO, v3_dot(dir, normal))));
```

**After (v0.17):**
```minz
let result = normal * intensity + ambient;
let reflected = dir - normal * (2.0 * dir.dot(normal));
```

### SDF Operations

**Before (v0.16):**
```minz
let dist = opSubtraction(sdSphere(p, r), sdBox(p, size));
let scene = opSmoothUnion(opUnion(sphere, box), cylinder, k);
```

**After (v0.17):**
```minz
let dist = sphere(p, r) - box(p, size);     // CSG subtraction
let scene = (sphere | box).smooth(cylinder, k);  // Smooth union
```

## Type Design

### 1. vec3 with Operators

```minz
struct vec3 { x: i16, y: i16, z: i16 }

impl vec3 {
    // Arithmetic operators
    fun add(self, other: vec3) -> vec3 { ... }      // v + v
    fun sub(self, other: vec3) -> vec3 { ... }      // v - v
    fun mul(self, other: vec3) -> vec3 { ... }      // v * v (component-wise)
    fun mul(self, scalar: i16) -> vec3 { ... }      // v * s (scaling)
    fun div(self, scalar: i16) -> vec3 { ... }      // v / s

    // Methods (via UFCS)
    fun dot(self, other: vec3) -> i16 { ... }       // v.dot(w)
    fun cross(self, other: vec3) -> vec3 { ... }    // v.cross(w)
    fun length(self) -> i16 { ... }                 // v.length()
    fun normalize(self) -> vec3 { ... }             // v.normalize()
    fun reflect(self, normal: vec3) -> vec3 { ... } // v.reflect(n)
}
```

### 2. Dist (SDF Distance) with CSG Operators

```minz
struct Dist { value: i16 }

impl Dist {
    // CSG operators via operator overloading
    fun sub(self, other: Dist) -> Dist {        // a - b = subtraction
        return Dist { value: fp_max(self.value, -other.value) };
    }

    fun bitor(self, other: Dist) -> Dist {      // a | b = union
        return Dist { value: fp_min(self.value, other.value) };
    }

    fun bitand(self, other: Dist) -> Dist {     // a & b = intersection
        return Dist { value: fp_max(self.value, other.value) };
    }

    // Smooth operations as methods
    fun smooth_union(self, other: Dist, k: i16) -> Dist { ... }
    fun smooth_sub(self, other: Dist, k: i16) -> Dist { ... }

    // Shell/onion
    fun onion(self, thickness: i16) -> Dist { ... }
}
```

### 3. SDF Primitives Return Dist

```minz
fun sphere(p: vec3, r: i16) -> Dist {
    return Dist { value: p.length() - r };
}

fun box(p: vec3, b: vec3) -> Dist {
    let q = p.abs() - b;
    return Dist { value: q.max(0).length() + fp_min(q.x.max(q.y).max(q.z), 0) };
}

fun cylinder(p: vec3, r: i16) -> Dist {
    return Dist { value: p.xz().length() - r };
}
```

## Idiomatic Raymarching Example

```minz
import glsl;

// Define the scene using natural operators
fun scene(p: vec3) -> Dist {
    // Sphere with box carved out, plus cylinder
    let body = sphere(p, 1.0) - box(p, vec3(0.6, 0.6, 0.6));
    let hole = cylinder(p, 0.3);

    return body | hole;  // Union
}

// Render with natural vector math
fun render(ro: vec3, rd: vec3) -> u8 {
    let t = 0;

    for i in 0..64 {
        let p = ro + rd * t;           // Natural!
        let d = scene(p);

        if d.value < 0.01 { break; }
        t = t + d.value;
    }

    // Calculate lighting
    let hit = ro + rd * t;
    let normal = calcNormal(hit);
    let light = vec3(1, 1, -1).normalize();

    let diffuse = normal.dot(light).max(0);  // Natural!
    return (diffuse * 255) as u8;
}
```

## CSG Operator Semantics

| Operator | Method | SDF Meaning | Formula |
|----------|--------|-------------|---------|
| `a - b` | `sub` | Subtraction (carve b from a) | `max(a, -b)` |
| `a \| b` | `bitor` | Union (combine shapes) | `min(a, b)` |
| `a & b` | `bitand` | Intersection (overlap only) | `max(a, b)` |

### Why These Operators?

- **`-` for subtraction**: Intuitive "remove" operation
- **`|` for union**: Like boolean OR - "this OR that"
- **`&` for intersection**: Like boolean AND - "this AND that"

## Migration Path

The new library will be **opt-in** via different import:

```minz
// Old style (still works)
import glsl.vec;
import glsl.sdf;
let result = v3_add(a, b);

// New idiomatic style
import glsl2;  // or glsl.idiomatic
let result = a + b;
```

## Implementation Modules

### `stdlib/glsl2/` (New Idiomatic Library)

| File | Contents |
|------|----------|
| `vec.minz` | vec2, vec3, vec4 with full operator overloading |
| `dist.minz` | Dist type with CSG operators |
| `sdf.minz` | Primitive functions returning Dist |
| `fp.minz` | Fixed-point (re-export or copy) |
| `trig.minz` | Trig functions (re-export) |
| `glsl2.minz` | Re-exports everything |

## Benefits

1. **Readability**: Code looks like GLSL/math
2. **Zero overhead**: Operators compile to same assembly
3. **Type safety**: Can't accidentally mix Dist and raw i16
4. **Chainable**: `sphere(p, r).onion(0.1) | box(p, b)`
5. **IDE-friendly**: Method completion for vec3, Dist

## Future Enhancements

- **Swizzling**: `v.xy`, `v.zyx` (requires language support)
- **Dot operator for dot product**: `v . w` (needs parser change)
- **Negation operator**: `-v` for `v.neg()` (unary operators)
- **Matrix types**: `mat3 * vec3` for transforms

---

*Part of MinZ v0.17.0 - Making shaders beautiful on Z80*
