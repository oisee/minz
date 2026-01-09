# GLSL Library with Generics & Interface Detection

## Current State: Verbose and Repetitive

The current GLSL library requires explicit function naming for every type:

```minz
// Current: Explicit functions for each type
fun v2_add(a: vec2, b: vec2) -> vec2 { ... }
fun v3_add(a: vec3, b: vec3) -> vec3 { ... }
fun v4_add(a: vec4, b: vec4) -> vec4 { ... }

fun v2_dot(a: vec2, b: vec2) -> i16 { ... }
fun v3_dot(a: vec3, b: vec3) -> i16 { ... }
fun v4_dot(a: vec4, b: vec4) -> i16 { ... }

// Usage - verbose
let result = v3_add(v3_sub(a, b), v3_mulf(c, 2));
let d = v3_dot(v3_normalize(a), v3_normalize(b));
```

**Problems:**
1. Repetitive code for each dimension (vec2, vec3, vec4)
2. Long function names reduce readability
3. No operator overloading - math looks awkward
4. Hard to add new vector types

---

## Proposed: Generics + Compile-Time Interface Detection

### 1. Generic Vector Type

```minz
// Generic vector with compile-time dimension
struct Vec<const N: usize> {
    data: [i16; N]
}

// Type aliases for convenience
type vec2 = Vec<2>;
type vec3 = Vec<3>;
type vec4 = Vec<4>;
```

### 2. Operator Overloading via Traits

```minz
// Numeric trait for math operations
trait Numeric {
    fun add(self, other: Self) -> Self;
    fun sub(self, other: Self) -> Self;
    fun mul(self, other: Self) -> Self;
    fun neg(self) -> Self;
}

// Implement for i16 (fixed-point)
impl Numeric for i16 {
    fun add(self, other: i16) -> i16 { self + other }
    fun sub(self, other: i16) -> i16 { self - other }
    fun mul(self, other: i16) -> i16 { fp_mul(self, other) }
    fun neg(self) -> i16 { -self }
}

// Generic vector operations
impl<const N: usize> Numeric for Vec<N> {
    fun add(self, other: Vec<N>) -> Vec<N> {
        // Monomorphized at compile-time for each N
        let mut result = Vec<N> { data: [0; N] };
        for i in 0..N {
            result.data[i] = self.data[i] + other.data[i];
        }
        return result;
    }
    // ...
}
```

### 3. Method Syntax

```minz
// Methods on vec3
impl vec3 {
    fun new(x: i16, y: i16, z: i16) -> vec3 {
        return vec3 { data: [x, y, z] };
    }

    fun x(self) -> i16 { self.data[0] }
    fun y(self) -> i16 { self.data[1] }
    fun z(self) -> i16 { self.data[2] }

    fun dot(self, other: vec3) -> i16 {
        return fp_mul(self.x(), other.x())
             + fp_mul(self.y(), other.y())
             + fp_mul(self.z(), other.z());
    }

    fun length(self) -> i16 {
        return fp_sqrt(self.dot(self));
    }

    fun normalize(self) -> vec3 {
        let len = self.length();
        if len == 0 { return vec3::ZERO; }
        return self / len;  // Operator overloading!
    }
}
```

### 4. Usage Comparison

**Current:**
```minz
fun calcNormal(p: vec3) -> vec3 {
    let eps: i16 = 2;

    let px_plus = vec3 { x: p.x + eps, y: p.y, z: p.z };
    let px_minus = vec3 { x: p.x - eps, y: p.y, z: p.z };

    let dx = map(px_plus) - map(px_minus);
    let dy = map(py_plus) - map(py_minus);
    let dz = map(pz_plus) - map(pz_minus);

    return v3_normalize(vec3 { x: dx, y: dy, z: dz });
}
```

**With Generics:**
```minz
fun calcNormal(p: vec3) -> vec3 {
    let eps = 2;

    let n = vec3::new(
        map(p + vec3::X * eps) - map(p - vec3::X * eps),
        map(p + vec3::Y * eps) - map(p - vec3::Y * eps),
        map(p + vec3::Z * eps) - map(p - vec3::Z * eps)
    );

    return n.normalize();
}
```

---

## Implementation Strategy

### Phase 1: Compile-Time Interface Detection (Zero-Cost)

MinZ already has lambda-to-function transform. Extend this for traits:

```minz
// Compiler detects this pattern:
let a: vec3 = ...;
let b: vec3 = ...;
let c = a + b;

// Transforms to direct function call (no vtable):
let c = vec3_add(a, b);
```

**Zero-Cost Abstraction**: At compile time, trait method calls are resolved to direct function calls. No runtime dispatch.

### Phase 2: Generic Monomorphization

```minz
// Single generic implementation
fun dot<V: Vector>(a: V, b: V) -> i16 { ... }

// Compiler generates specialized versions:
fun dot_vec2(a: vec2, b: vec2) -> i16 { ... }
fun dot_vec3(a: vec3, b: vec3) -> i16 { ... }
fun dot_vec4(a: vec4, b: vec4) -> i16 { ... }
```

### Phase 3: Const Generics for Loop Unrolling

```minz
// Compiler unrolls at compile-time
impl<const N: usize> Vec<N> {
    fun dot(self, other: Vec<N>) -> i16 {
        let sum: i16 = 0;
        for i in 0..N {  // N known at compile-time
            sum = sum + fp_mul(self.data[i], other.data[i]);
        }
        return sum;
    }
}

// For vec3, compiler generates:
fun vec3_dot(self: vec3, other: vec3) -> i16 {
    return fp_mul(self.data[0], other.data[0])
         + fp_mul(self.data[1], other.data[1])
         + fp_mul(self.data[2], other.data[2]);
}
```

---

## Example: Complete Raymarcher with New Syntax

```minz
import glsl;

// SDF for a sphere
fun sdSphere(p: vec3, r: fp) -> fp {
    return p.length() - r;
}

// Scene composition
fun map(p: vec3) -> fp {
    return sdSphere(p, fp::ONE);
}

// Calculate normal using gradient
fun calcNormal(p: vec3) -> vec3 {
    let e = fp::from_int(2);
    return vec3::new(
        map(p + vec3::X * e) - map(p - vec3::X * e),
        map(p + vec3::Y * e) - map(p - vec3::Y * e),
        map(p + vec3::Z * e) - map(p - vec3::Z * e)
    ).normalize();
}

// Raymarch!
fun raymarch(ro: vec3, rd: vec3) -> fp {
    let t = fp::ZERO;

    for _ in 0..64 {
        let p = ro + rd * t;
        let d = map(p);

        if d < fp::EPSILON {
            return t;  // Hit!
        }

        t = t + d;

        if t > fp::from_int(10) {
            break;  // Too far
        }
    }

    return fp::NEG_ONE;  // No hit
}

// Main render loop
fun main() {
    for y in 0..48 {
        for x in 0..64 {
            // Convert screen to UV
            let uv = vec2::new(
                fp::from_int(x - 32) / fp::from_int(32),
                fp::from_int(y - 24) / fp::from_int(24)
            );

            // Camera setup
            let ro = vec3::new(fp::ZERO, fp::ZERO, fp::from_int(-3));
            let rd = vec3::new(uv.x(), uv.y(), fp::ONE).normalize();

            // Raymarch
            let t = raymarch(ro, rd);

            if t > fp::ZERO {
                let p = ro + rd * t;
                let n = calcNormal(p);
                let light = vec3::new(fp::ONE, fp::ONE, -fp::ONE).normalize();
                let brightness = (n.dot(light) + fp::ONE) / fp::from_int(2);
                plot(x as u8, y as u8, brightness.to_u8());
            }
        }
    }
}
```

---

## Benefits Summary

| Aspect | Current | With Generics |
|--------|---------|---------------|
| **Code Volume** | ~800 lines for vec2/3/4 | ~200 lines generic |
| **Readability** | `v3_add(a, v3_mulf(b, 2))` | `a + b * 2` |
| **Type Safety** | Manual | Compile-time checked |
| **Performance** | Direct calls | Same (monomorphized) |
| **Extensibility** | Copy-paste | Just `type vecN = Vec<N>` |

---

## Implementation Roadmap

1. **v0.16**: Method syntax `obj.method()` → `Type_method(obj)`
2. **v0.17**: Trait definitions and `impl` blocks
3. **v0.18**: Compile-time interface detection (duck typing)
4. **v0.19**: Generic types `struct Foo<T>`
5. **v0.20**: Const generics `struct Vec<const N: usize>`
6. **v0.21**: Operator overloading via traits

---

*MinZ: Making Z80 programming feel modern while staying zero-cost.*
