# Type Casting in MinZ: Static vs Interface-Based Design

## Current State

MinZ currently has explicit type casting with the `as` operator:
```minz
let small: u8 = 42;
let big: u16 = small as u16;  // Explicit cast
```

But implicit casting is missing, causing compilation failures like:
- `no matching overload for safe_multiply(u16, u8)` 
- Functions expecting u8 fail when passed u16

## Design Options

### 1. Static Compile-Time Type Casting (Traditional)

**Implementation:**
```minz
// Compiler automatically inserts casts
let a: u8 = 100;
let b: u16 = 200;
let c = a + b;  // Compiler inserts: (a as u16) + b
```

**Pros:**
- Zero runtime overhead
- Works like C/Rust/Go
- Simple mental model
- Deterministic behavior

**Cons:**
- Can hide precision loss (u16 → u8)
- May cause unexpected behavior
- Requires complex type inference rules

### 2. Interface-Based Casting (Ruby/Swift Style)

**Implementation:**
```minz
interface ToU8 {
    fun to_u8(self) -> u8;
}

interface ToU16 {
    fun to_u16(self) -> u16;
}

// Compiler generates these implementations
impl ToU8 for u16 {
    fun to_u8(self) -> u8 {
        // Truncate to lower 8 bits
        @asm { "LD A, L" }
    }
}

impl ToU16 for u8 {
    fun to_u16(self) -> u16 {
        // Zero-extend to 16 bits
        @asm { "LD H, 0" }
    }
}
```

**Usage:**
```minz
let big: u16 = 300;
let small: u8 = big.to_u8();  // Explicit, clear truncation
```

**Pros:**
- Explicit about conversions
- Can add custom logic (saturation, clamping)
- Extensible to user types
- Clear API

**Cons:**
- More verbose
- Requires method call syntax
- May confuse users expecting automatic casting

### 3. Hybrid Approach (Recommended)

**Safe Implicit Widening + Explicit Narrowing:**
```minz
// Implicit widening (always safe)
let small: u8 = 42;
let big: u16 = small;  // OK - no data loss

// Explicit narrowing (may lose data)
let big: u16 = 300;
let small: u8 = big as u8;  // Explicit cast required
// OR
let small: u8 = big.to_u8();  // Interface method
```

**Compile-Time Interface Resolution:**
```minz
// Compiler can resolve these at compile time
interface NumericConversion<T> {
    fun to<T>(self) -> T;
}

// Usage becomes:
let x: u16 = y.to<u16>();  // Resolved at compile time to zero-cost operation
```

### 4. Compile-Time Interfaces (Novel Approach)

**Concept:** Interfaces that exist only at compile time, generating inline code:

```minz
@compile_interface Convertible<From, To> {
    @inline fun convert(value: From) -> To;
}

// Compiler generates specialized versions
@compile_impl Convertible<u8, u16> {
    @inline fun convert(value: u8) -> u16 {
        // Generates: LD H, 0; LD L, value
        @zero_extend(value)
    }
}

// Usage - resolved entirely at compile time
let small: u8 = 42;
let big: u16 = convert<u8, u16>(small);  // Zero overhead
```

**Benefits:**
- Type safety at compile time
- Zero runtime overhead
- Can specialize for each conversion
- Clear semantics

## Recommendation for MinZ

Given MinZ's philosophy of "modern abstractions with zero cost", I recommend:

### Immediate Implementation (Phase 1)
1. **Implicit widening** for safe conversions (u8→u16, i8→i16)
2. **Explicit `as` operator** for narrowing (u16→u8)
3. **Built-in conversion functions** for common cases

### Future Enhancement (Phase 2)
Add compile-time interfaces for custom conversions:
```minz
// User-defined conversions
@compile_interface FixedPointConvertible {
    fun to_fixed(self) -> f8.8;
}

impl FixedPointConvertible for u8 {
    @inline fun to_fixed(self) -> f8.8 {
        // Shift left by 8 bits for fractional part
        @asm { "LD H, A; LD L, 0" }
    }
}
```

## Implementation Strategy

### 1. Semantic Analyzer Changes
```go
// In analyzer.go
func (a *Analyzer) checkTypeCompatibility(expected, actual Type) bool {
    // Allow implicit widening
    if isWideningConversion(actual, expected) {
        return true
    }
    // Require explicit cast for narrowing
    return typesEqual(expected, actual)
}

func isWideningConversion(from, to Type) bool {
    // u8 → u16, u8 → u24, u16 → u24
    // i8 → i16, i8 → i24, i16 → i24
    // Never lose precision
    return from.Size() < to.Size() && 
           sameSignedness(from, to)
}
```

### 2. IR Generation
```go
// Automatically insert widening conversions
if isWideningConversion(actualType, expectedType) {
    // Insert zero-extend or sign-extend instruction
    inst := ir.Instruction{
        Op:   ir.OpCast,
        Src:  actualReg,
        Dest: newReg,
        Type: expectedType,
    }
}
```

### 3. Code Generation
```go
case ir.OpCast:
    if inst.Type.Size() > srcType.Size() {
        // Widening - zero extend or sign extend
        g.generateWidening(inst)
    } else {
        // Narrowing - truncate
        g.generateNarrowing(inst)
    }
```

## Questions for Consideration

1. **Should we allow implicit narrowing with warning?**
   - Pro: More convenient
   - Con: Can hide bugs

2. **Should interfaces be runtime or compile-time?**
   - Runtime: More flexible, allows polymorphism
   - Compile-time: Zero overhead, fits Z80 constraints

3. **How to handle signed/unsigned conversions?**
   - Always explicit?
   - Allow implicit if safe?

4. **Custom conversion operators?**
   ```minz
   operator as(self: FixedPoint) -> u8 {
       return self.integer_part;
   }
   ```

## Conclusion

MinZ should adopt a pragmatic approach:
- **Implicit widening** (safe, zero cost)
- **Explicit narrowing** (clear intent)
- **Compile-time interfaces** for extensibility
- **Zero runtime overhead** always

This balances safety, usability, and performance - perfect for Z80 development.

## Next Steps

1. Implement implicit widening in semantic analyzer
2. Add type casting to overload resolution
3. Generate appropriate IR opcodes
4. Optimize in peephole patterns
5. Document conversion rules clearly

The goal: Make common cases easy, make wrong code hard to write, maintain zero overhead.