# ADR-0028: ObjC Inheritance & Monomorphization on Z80

**Status:** Accepted
**Date:** 2026-03-16
**Context:** ObjC static dispatch frontend needs inheritance, protocols, and mixins — all without runtime overhead on Z80.

## Decision

All ObjC dispatch remains static. Inheritance uses struct embedding + method copying. Protocols are compile-time constraints. Mixins are protocol default implementations copied into conforming classes. Monomorphization is the default strategy for polymorphic code.

## Design

### 1. Inheritance: Struct Embedding + Method Resolution

```objc
@interface Shape { int x; int y; }
-(void)moveTo:(int)nx :(int)ny;
@end

@interface Circle : Shape { int radius; }
-(int)area;
@end
```

Lowers to:

```
struct Shape  { x: i16, y: i16 }              // 4 bytes
struct Circle { x: i16, y: i16, radius: i16 } // 6 bytes — parent fields FIRST
```

**Key invariant:** child struct starts with parent fields (C struct layout). A `Circle*` is always a valid `Shape*` — field offsets are compatible.

**Method resolution order (MRO):**
1. Check class own methods
2. Walk superclass chain upward
3. First match wins (override = define in child)

**Inherited methods:** If `Circle` doesn't define `moveTo:`, it gets `Circle_moveTo` as an alias or trampoline to `Shape_moveTo`. Both share the same calling convention since child struct layout is parent-compatible.

### 2. `super` Calls

```objc
@implementation Circle
-(void)reset {
    [super moveTo:0 :0];  // → Shape_moveTo(self, 0, 0)
    self->radius = 0;
}
@end
```

`[super msg]` resolves to the parent class method at compile time. No dynamic lookup.

### 3. Protocols: Compile-Time Checking (Zero Codegen)

```objc
@protocol Drawable
-(void)draw;
-(int)area;
@end

@interface Circle : Shape <Drawable>
```

At compile time: verify `Circle` implements all `@required` methods from `Drawable`. Missing method → compile error. **No vtable, no runtime cost.**

### 4. Protocol Default Methods (Mixins)

```objc
@protocol Printable
-(void)print;  // default implementation in @implementation Printable
@end

@implementation Printable  // protocol default implementations
-(void)print {
    // default body — copied into conforming classes
}
@end
```

Any class conforming to `Printable` that doesn't override `print` gets a monomorphized copy: `Circle_print`, `Square_print`, etc. Each copy has the correct `self` type.

### 5. Polymorphism Strategy

| Scenario | Strategy | Runtime Cost |
|----------|----------|-------------|
| `[circle draw]` — concrete type known | Direct CALL | Zero |
| Inherited method, not overridden | Alias to parent method | Zero (JP trampoline if needed) |
| Protocol conformance | Compile-time check | Zero |
| Protocol default impl (mixin) | Copy method body per class | Zero |
| `[super method]` | Direct CALL to parent | Zero |

**No vtables.** If truly heterogeneous collections are needed later, a thin dispatch table (2 bytes per method pointer) can be added as explicit opt-in.

## Implementation Order

1. **Inheritance** — struct field embedding from superclass in `lowerObjCInterface`
2. **Method inheritance** — copy/alias unoverridden methods in `lowerObjCImplementation`
3. **`super` calls** — resolve `[super msg]` to parent mangled name in `lowerObjCMessage`
4. **Protocol conformance** — verify required methods at end of `lowerObjCImplementation`
5. **Protocol default methods** — copy bodies from protocol impl into conforming classes

## Consequences

- Zero runtime overhead — all dispatch static
- Code size may grow with mixins (method bodies copied per class)
- No support for truly dynamic dispatch (heterogeneous `id` collections) — acceptable for Z80
- Parent-first struct layout enables safe upcasting without cost
