# ADR-002: Generics Strategy - Crystal-Style + Zero-Cost Interfaces

**Status:** Accepted
**Date:** 2025-12-18
**Decision:** Park Rust-style `<T>` generics, implement Crystal-style `Type(T)` + Zero-Cost Interfaces

---

## Context

MinZ needs some form of generic programming for:
- Type-safe containers (`Array(u8)`, `Array(Point)`)
- Generic algorithms (`sort`, `find`, `max`)
- Polymorphic abstractions (`Drawable`, `Serializable`)

The question: **Full generics like Rust, or something Z80-friendly?**

---

## Decision

### Park Full Rust-Style Generics

**We will NOT implement:**
```minz
// NOT implementing this
fun max<T: Comparable>(a: T, b: T) -> T { ... }
struct Array<T> { data: *T, len: u8 }
```

**Rationale:**
- Requires complex type inference engine
- Trait bounds system adds significant complexity
- 2-4 weeks of implementation for marginal benefit
- Z80 target wants predictable, monomorphized code anyway

### Instead: Three-Pillar Approach

```
┌─────────────────────────────────────────────────────────┐
│                  MinZ Generic Programming               │
├───────────────────┬──────────────────┬─────────────────┤
│  Crystal-Style    │   Zero-Cost      │    Function     │
│  Type(T)          │   Interfaces     │    Overloading  │
├───────────────────┼──────────────────┼─────────────────┤
│  Containers       │  Polymorphism    │  Common Ops     │
│  Data structures  │  Abstractions    │  print, max...  │
└───────────────────┴──────────────────┴─────────────────┘
```

---

## Pillar 1: Crystal-Style Type Parameters

### Syntax
```minz
// Clean, Ruby-ish syntax
struct Array(T) {
    data: *T,
    len: u8,
    cap: u8
}

struct Pair(K, V) {
    key: K,
    value: V
}

// Usage - parentheses, not angle brackets
let numbers: Array(u8) = Array(u8).new(10);
let mapping: Pair(u8, str) = Pair(u8, str) { key: 1, value: "one" };
```

### How It Works
```
Source                    Monomorphization           Z80 Output
─────────────────────────────────────────────────────────────────
Array(u8).new(10)    →    Array_u8.new(10)      →   CALL Array_u8_new
Array(Point).get(i)  →    Array_Point.get(i)   →   CALL Array_Point_get
```

**Compile-time only** - no runtime cost, just syntax sugar over specialized types.

### Why Crystal-Style?

| Syntax | Example | Verdict |
|--------|---------|---------|
| Rust `<T>` | `Array<u8>` | Conflicts with comparison operators |
| C++ `<T>` | `Array<u8>` | Same issue, verbose |
| **Crystal `(T)`** | `Array(u8)` | Clean, no ambiguity, Ruby-ish |
| D `!T` | `Array!u8` | Unfamiliar, cryptic |

Crystal-style fits MinZ's Ruby aesthetic perfectly.

---

## Pillar 2: Zero-Cost Interfaces

### Syntax
```minz
// Define interface
interface Drawable {
    fun draw(self) -> void;
    fun get_bounds(self) -> Rect;
}

// Implement for concrete types
impl Drawable for Circle {
    fun draw(self) -> void {
        // Circle-specific drawing
        draw_circle(self.x, self.y, self.radius);
    }

    fun get_bounds(self) -> Rect {
        return Rect {
            x: self.x - self.radius,
            y: self.y - self.radius,
            w: self.radius * 2,
            h: self.radius * 2
        };
    }
}

impl Drawable for Sprite {
    fun draw(self) -> void {
        blit_sprite(self.data, self.x, self.y);
    }

    fun get_bounds(self) -> Rect {
        return Rect { x: self.x, y: self.y, w: self.w, h: self.h };
    }
}
```

### Zero-Cost Dispatch
```minz
// This function accepts any Drawable
fun render(item: impl Drawable) -> void {
    item.draw();        // Compiles to DIRECT call!
    let b = item.get_bounds();
}

// Usage
let circle = Circle { x: 10, y: 20, radius: 5 };
let sprite = Sprite { x: 0, y: 0, data: sprite_data };

render(circle);   // Compiles to: CALL Circle_draw
render(sprite);   // Compiles to: CALL Sprite_draw
```

### How Zero-Cost Works

**At compile time:**
1. Compiler sees `render(circle)` where `circle: Circle`
2. Knows `Circle` implements `Drawable`
3. Monomorphizes `render` for `Circle`
4. `item.draw()` becomes `CALL Circle_draw`

**Generated Z80:**
```asm
; render(circle) - monomorphized for Circle
render_Circle:
    ; item.draw() → direct call, NO vtable lookup!
    CALL Circle_draw
    ; item.get_bounds()
    CALL Circle_get_bounds
    RET

; render(sprite) - separate monomorphized version
render_Sprite:
    CALL Sprite_draw
    CALL Sprite_get_bounds
    RET
```

**Result:** Looks polymorphic, runs like hand-optimized code.

---

## Pillar 3: Function Overloading (Already Working!)

```minz
// Multiple functions, same name, different types
fun print(x: u8) -> void { /* u8 printing */ }
fun print(x: u16) -> void { /* u16 printing */ }
fun print(x: str) -> void { /* string printing */ }
fun print(x: bool) -> void { /* bool printing */ }

fun max(a: u8, b: u8) -> u8 { if a > b { return a; } return b; }
fun max(a: u16, b: u16) -> u16 { if a > b { return a; } return b; }

// Usage - compiler resolves correct overload
print(42);        // Calls print$u8
print("hello");   // Calls print$str
let m = max(x, y); // Calls max$u8 or max$u16 based on types
```

**This already works** and handles 80% of "generic" use cases.

---

## Comparison: MinZ vs Others

| Feature | Rust | Go | MinZ |
|---------|------|-----|------|
| Syntax | `<T>` | `[T any]` | `(T)` |
| Constraints | `T: Trait` | `T comparable` | `impl Interface` |
| Runtime cost | Zero (monomorphized) | Interface boxing | **Zero** |
| Complexity | High | Medium | **Low** |
| Compile time | Slow | Fast | **Fast** |

---

## Migration Path

### Phase 1: Now (v0.15.x)
- Function overloading ✅ Working
- @define macros ✅ Working
- Interface syntax 🟡 Parsing works

### Phase 2: Soon (v0.16.x)
- Zero-cost interface dispatch
- `impl Interface for Type` codegen
- `impl Trait` parameter syntax

### Phase 3: Later (v0.17.x)
- Crystal-style `Type(T)` syntax
- Automatic monomorphization
- Standard library containers

---

## Examples: Real-World Usage

### Generic Container
```minz
struct Stack(T) {
    data: [T; 32],
    top: u8
}

impl Stack(T) {
    fun push(self, item: T) -> void {
        self.data[self.top] = item;
        self.top = self.top + 1;
    }

    fun pop(self) -> T {
        self.top = self.top - 1;
        return self.data[self.top];
    }
}

// Usage
let byte_stack: Stack(u8) = Stack(u8).new();
byte_stack.push(42);
let val = byte_stack.pop();  // val: u8 = 42
```

### Interface-Based Game Objects
```minz
interface GameObject {
    fun update(self) -> void;
    fun draw(self) -> void;
    fun collides(self, other: impl GameObject) -> bool;
}

impl GameObject for Player { /* ... */ }
impl GameObject for Enemy { /* ... */ }
impl GameObject for Bullet { /* ... */ }

// Game loop - all calls are direct, no vtables!
fun game_update(objects: *impl GameObject, count: u8) -> void {
    for i in 0..count {
        objects[i].update();
        objects[i].draw();
    }
}
```

---

## What We're NOT Doing

| Feature | Reason |
|---------|--------|
| `<T>` angle bracket syntax | Ambiguous with `<` `>` operators |
| Trait bounds `T: Eq + Hash` | Too complex for Z80 use case |
| Associated types | Complexity without benefit |
| Higher-kinded types | Way overkill |
| Runtime polymorphism | No vtables on Z80! |

---

## Consequences

### Positive
- Simple, clean syntax fitting MinZ aesthetic
- Zero runtime overhead on Z80
- Predictable code generation
- Fast compile times
- Easy to understand and debug

### Negative
- Can't write truly generic libraries (must monomorphize)
- Some code duplication in generated output
- Not compatible with Rust/C++ generic patterns

### Neutral
- Different from mainstream - but that's MinZ's philosophy!

---

## References

- [Crystal Language Generics](https://crystal-lang.org/reference/syntax_and_semantics/generics.html)
- [Rust Monomorphization](https://doc.rust-lang.org/book/ch10-01-syntax.html)
- [ADR-001: Function Pointers Parked](./ADR_001_Function_Pointers_Parked.md)
- [MinZ Zero-Cost Abstractions Philosophy](./145_TSMC_Complete_Philosophy.md)

---

*"Write it like Ruby, run it like assembly."* - MinZ Philosophy
