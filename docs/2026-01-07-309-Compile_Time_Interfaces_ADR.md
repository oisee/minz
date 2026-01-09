# ADR-309: Compile-Time Interfaces for MinZ

**Status**: Accepted
**Date**: 2026-01-07
**Decision**: Implement structural compile-time interfaces with UFCS and operator overloading

## Context

MinZ needs a way to write generic, reusable code (like the GLSL shader library) without:
- Runtime overhead (vtables, indirect calls)
- Verbose boilerplate (`impl Trait for Type`)
- Code bloat from excessive monomorphization

The Z80's constraints make this critical:
- 16KB-64KB ROM total
- No heap for trait objects
- Direct CALL (17 T) vs indirect JP (HL) (4 T but setup overhead)

## Decision

Implement **Compile-Time Interfaces** with the following characteristics:

### 1. UFCS (Universal Function Call Syntax)

Method calls transform to direct function calls at compile-time:

```minz
v.add(other)     →  Vec2_add(v, other)
v.length()       →  Vec2_length(v)
v.normalize()    →  Vec2_normalize(v)
```

**Z80 Output:**
```asm
; v.add(other)
    ; Load v and other into registers
    CALL Vec2_add    ; Direct call - 17 T-states
```

### 2. Optional Interface Declarations (Structural Matching)

Interfaces are **optional documentation** - the compiler validates structurally:

```minz
// OPTIONAL: Declare interface for documentation & better errors
interface Numeric {
    fun add(self, other: Self) -> Self;
    fun mul(self, other: Self) -> Self;
}

// NO "impl Numeric for Vec2" required!
// Just define the methods:
impl Vec2 {
    fun add(self, other: Vec2) -> Vec2 {
        Vec2 { x: self.x + other.x, y: self.y + other.y }
    }
    fun mul(self, other: Vec2) -> Vec2 {
        Vec2 { x: fp_mul(self.x, other.x), y: fp_mul(self.y, other.y) }
    }
}

// Generic function with explicit constraint
fun double<T: Numeric>(x: T) -> T {
    return x.add(x);
}

// OR implicit constraint (inferred from usage)
fun double<T>(x: T) -> T {
    return x.add(x);  // Compiler infers: T must have add(Self) -> Self
}
```

**Structural Matching Rules:**
- Type satisfies interface if it has ALL required methods
- Method signatures must match exactly (name, params, return type)
- No explicit `impl Interface for Type` declaration needed
- Compiler checks at call site, not at definition

### 3. Operator Overloading via Standard Interfaces

Built-in operator interfaces (in stdlib prelude):

```minz
// Arithmetic operators
interface Add { fun add(self, other: Self) -> Self; }  // +
interface Sub { fun sub(self, other: Self) -> Self; }  // -
interface Mul { fun mul(self, other: Self) -> Self; }  // *
interface Div { fun div(self, other: Self) -> Self; }  // /
interface Mod { fun mod(self, other: Self) -> Self; }  // %
interface Neg { fun neg(self) -> Self; }               // - (unary)

// Comparison operators
interface Eq  { fun eq(self, other: Self) -> bool; }   // ==, !=
interface Ord { fun cmp(self, other: Self) -> i8; }    // <, >, <=, >=

// Bitwise operators
interface BitAnd { fun bitand(self, other: Self) -> Self; }  // &
interface BitOr  { fun bitor(self, other: Self) -> Self; }   // |
interface BitXor { fun bitxor(self, other: Self) -> Self; }  // ^
interface Shl    { fun shl(self, n: u8) -> Self; }           // <<
interface Shr    { fun shr(self, n: u8) -> Self; }           // >>
```

**Usage:**
```minz
impl Vec2 {
    fun add(self, other: Vec2) -> Vec2 { ... }
    fun neg(self) -> Vec2 { Vec2 { x: -self.x, y: -self.y } }
    fun eq(self, other: Vec2) -> bool { self.x == other.x && self.y == other.y }
}

// Operators now work!
let c = a + b;       // Vec2_add(a, b)
let d = -a;          // Vec2_neg(a)
let e = a + b * c;   // Vec2_add(a, Vec2_mul(b, c))
if a == b { ... }    // Vec2_eq(a, b)
```

### 4. Monomorphization Strategy

**Generate specialized code for each concrete type:**

```minz
fun dot<T: Numeric>(a: T, b: T) -> T {
    return a.x.mul(b.x).add(a.y.mul(b.y));
}

// Called with Vec2 and Vec3:
dot(v2a, v2b);  // Generates: dot_Vec2
dot(v3a, v3b);  // Generates: dot_Vec3
```

**Z80 Optimizations:**

| Optimization | Description | Savings |
|--------------|-------------|---------|
| **Inline small methods** | Methods < 8 bytes inline automatically | Eliminates CALL/RET |
| **Deduplicate identical** | Same bytecode = share code | ROM space |
| **Dead code elimination** | Unused monomorphizations removed | ROM space |
| **Const propagation** | Known values at compile-time | Runtime cycles |

### 5. Error Messages

With optional interface declarations, errors are clearer:

```
// Without interface declaration:
error: Type 'MyType' doesn't have method 'add(self, Self) -> Self'
       required by function 'double' at line 42

// With interface declaration:
error: Type 'MyType' doesn't satisfy interface 'Numeric'
       missing method: fun add(self, other: Self) -> Self
       required by function 'double<T: Numeric>' at line 42
```

## Implementation Plan

### Phase 1: UFCS (v0.17)
- Transform `x.method(args)` → `Type_method(x, args)`
- Works with existing `impl` blocks
- No generics yet - concrete types only

### Phase 2: Basic Generics (v0.18)
- Generic functions: `fun foo<T>(x: T) -> T`
- Monomorphization at call sites
- Implicit constraints from usage

### Phase 3: Interface Declarations (v0.19)
- Optional `interface` declarations
- Explicit constraints: `<T: Interface>`
- Structural matching validation

### Phase 4: Operator Overloading (v0.20)
- Standard operator interfaces in prelude
- Operator → method desugaring
- Precedence handling

### Phase 5: Advanced Features (v0.21+)
- Associated types
- Default method implementations
- Const generics for vectors: `Vec<const N: usize>`

## Consequences

### Positive

- **Zero runtime overhead** - All dispatch at compile-time
- **Clean syntax** - `a + b` instead of `v2_add(a, b)`
- **No boilerplate** - No `impl Interface for Type`
- **Self-documenting** - Interfaces describe capabilities
- **Z80 optimal** - Direct CALLs, inlining, deduplication

### Negative

- **Code size** - Monomorphization can bloat ROM
- **Compile time** - More work at compile-time
- **Complexity** - Structural matching is subtle

### Mitigations

- Aggressive inlining for small methods
- Deduplication of identical monomorphizations
- Lazy generation (only generate what's used)
- Size warnings when monomorphization exceeds threshold

## Examples

### GLSL Library with Compile-Time Interfaces

```minz
// Vector interface (optional, for documentation)
interface Vector {
    fun add(self, other: Self) -> Self;
    fun sub(self, other: Self) -> Self;
    fun mul(self, s: i16) -> Self;        // Scalar multiply
    fun dot(self, other: Self) -> i16;
    fun length(self) -> i16;
    fun normalize(self) -> Self;
}

// Vec2 implementation
struct Vec2 { x: i16, y: i16 }

impl Vec2 {
    fun new(x: i16, y: i16) -> Vec2 { Vec2 { x, y } }

    fun add(self, other: Vec2) -> Vec2 {
        Vec2 { x: self.x + other.x, y: self.y + other.y }
    }

    fun dot(self, other: Vec2) -> i16 {
        fp_mul(self.x, other.x) + fp_mul(self.y, other.y)
    }

    fun length(self) -> i16 {
        fp_sqrt(self.dot(self))
    }

    fun normalize(self) -> Vec2 {
        let len = self.length();
        if len == 0 { return Vec2::ZERO; }
        Vec2 { x: fp_div(self.x, len), y: fp_div(self.y, len) }
    }
}

// Vec3 - same interface, different implementation
struct Vec3 { x: i16, y: i16, z: i16 }

impl Vec3 {
    // ... similar methods
}

// Generic raymarching works with any Vector!
fun raymarch<V: Vector>(ro: V, rd: V, max_steps: u8) -> i16 {
    var t: i16 = 0;
    for _ in 0..max_steps {
        let p = ro + rd * t;  // Operators work!
        let d = sdf_scene(p);
        if d < EPSILON { return t; }
        t = t + d;
    }
    return -1;
}
```

### Generated Z80 for Vec2

```asm
; fun add(self, other: Vec2) -> Vec2
Vec2_add:
    ; self in HL (pointer), other in DE (pointer)
    ; Result returned in HL (pointer to stack space)

    LD A, (HL)      ; self.x low
    INC HL
    LD B, (HL)      ; self.x high
    INC HL

    EX DE, HL
    LD C, (HL)      ; other.x low
    INC HL
    LD D, (HL)      ; other.x high

    ; Add x components
    ADD A, C
    LD E, A
    LD A, B
    ADC A, D
    LD D, A
    ; DE = result.x

    ; ... similar for y

    RET
```

## Comparison with Other Languages

| Language | Approach | Runtime Cost |
|----------|----------|--------------|
| **Rust** | Traits with static/dynamic dispatch | Zero (static) or vtable (dynamic) |
| **Go** | Structural interfaces, always dynamic | Interface wrapper + indirect call |
| **C++** | Templates, concepts (C++20) | Zero (all static) |
| **Swift** | Protocols with witness tables | Witness table lookup |
| **MinZ** | Structural compile-time interfaces | **Zero - always direct calls** |

## References

- [Zero-Cost Interfaces Design](132_Zero_Cost_Interfaces_Design.md)
- [GLSL Library Vision](308_GLSL_Library_With_Generics_Vision.md)
- [Rust Traits](https://doc.rust-lang.org/book/ch10-02-traits.html)
- [Go Interfaces](https://go.dev/tour/methods/9)
- [C++ Concepts](https://en.cppreference.com/w/cpp/language/constraints)

---

*"Compile-time interfaces: Ruby's duck typing meets Rust's zero-cost abstractions, optimized for Z80."*
