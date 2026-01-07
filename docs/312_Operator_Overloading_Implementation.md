# Operator Overloading Implementation

**Status**: IMPLEMENTED (January 2026)
**Version**: v0.17.0
**Prerequisite**: ADR-311 (UFCS Implementation)

## Overview

Operator overloading in MinZ transforms binary operators into method calls when the left operand's type has a corresponding method defined. This builds on the UFCS infrastructure.

```minz
v1 + v2      // Transforms to: v1.add(v2) -> Vec2_add(v1, v2)
v1 == v2     // Transforms to: v1.eq(v2)  -> Vec2_eq(v1, v2)
```

## Supported Operators

| Operator | Method Name | Example |
|----------|-------------|---------|
| `+` | `add` | `v1 + v2` -> `v1.add(v2)` |
| `-` | `sub` | `v1 - v2` -> `v1.sub(v2)` |
| `*` | `mul` | `v1 * v2` -> `v1.mul(v2)` |
| `/` | `div` | `v1 / v2` -> `v1.div(v2)` |
| `%` | `mod` | `v1 % v2` -> `v1.mod(v2)` |
| `==` | `eq` | `v1 == v2` -> `v1.eq(v2)` |
| `!=` | `ne` | `v1 != v2` -> `v1.ne(v2)` |
| `<` | `lt` | `v1 < v2` -> `v1.lt(v2)` |
| `<=` | `le` | `v1 <= v2` -> `v1.le(v2)` |
| `>` | `gt` | `v1 > v2` -> `v1.gt(v2)` |
| `>=` | `ge` | `v1 >= v2` -> `v1.ge(v2)` |
| `&` | `bitand` | `v1 & v2` -> `v1.bitand(v2)` |
| `\|` | `bitor` | `v1 \| v2` -> `v1.bitor(v2)` |
| `^` | `bitxor` | `v1 ^ v2` -> `v1.bitxor(v2)` |
| `<<` | `shl` | `v1 << v2` -> `v1.shl(v2)` |
| `>>` | `shr` | `v1 >> v2` -> `v1.shr(v2)` |

## Auto-Derivation

MinZ automatically derives missing comparison operators from related ones. This means you only need to implement `eq` and `lt` to get all 6 comparison operators!

### Derivation Rules

| Operator | Derives From | Transformation |
|----------|--------------|----------------|
| `!=` | `eq` | `a != b` = `!(a.eq(b))` |
| `>` | `lt` | `a > b` = `b.lt(a)` (swap operands) |
| `>=` | `lt` | `a >= b` = `!(a.lt(b))` |
| `<=` | `gt` or `lt` | `a <= b` = `!(a.gt(b))` or `!(b.lt(a))` |

### Priority

1. **Explicit method always wins** - If you define `ne`, it will be used instead of deriving from `eq`
2. **Derivation as fallback** - Only used when explicit method is not found

### Minimal Implementation Example

```minz
struct Vec2 { x: i16, y: i16 }

impl Vec2 {
    // Only define eq and lt - get !=, >, >=, <= for free!
    fun eq(self, other: Vec2) -> bool {
        return self.x == other.x;
    }

    fun lt(self, other: Vec2) -> bool {
        return self.x < other.x;
    }
}

fun main() -> void {
    let v1 = Vec2 { x: 3, y: 4 };
    let v2 = Vec2 { x: 1, y: 2 };

    if v1 == v2 { }  // Direct: eq
    if v1 != v2 { }  // Derived: !eq
    if v1 <  v2 { }  // Direct: lt
    if v1 >  v2 { }  // Derived: swap lt
    if v1 >= v2 { }  // Derived: !lt
    if v1 <= v2 { }  // Derived: swap+negate lt
}
```

### Generated Assembly Comments

The compiler adds comments showing derivation:
```asm
; Operator == via Vec2.eq
; Operator != via Vec2.eq (negate)
; Operator <  via Vec2.lt
; Operator >  via Vec2.lt (swap)
; Operator >= via Vec2.lt (negate)
; Operator <= via Vec2.lt (swap+negate)
```

## Usage Example

```minz
struct Vec2 { x: i16, y: i16 }

impl Vec2 {
    fun add(self, other: Vec2) -> Vec2 {
        return Vec2 { x: self.x + other.x, y: self.y + other.y };
    }

    fun eq(self, other: Vec2) -> bool {
        return self.x == other.x && self.y == other.y;
    }
}

fun main() -> void {
    let v1 = Vec2 { x: 3, y: 4 };
    let v2 = Vec2 { x: 1, y: 2 };

    // Operator overloading: + uses add method
    let v3 = v1 + v2;  // Compiles to: CALL Vec2.add

    // Operator overloading: == uses eq method
    if v1 == v2 {      // Compiles to: CALL Vec2.eq
        @asm("NOP");
    }
}
```

## Implementation Details

### Resolution Order

When the analyzer encounters a binary expression:

1. **Check for operator method**: Look up method name for the operator (e.g., `+` -> `add`)
2. **Infer left operand type**: Determine the type without full analysis
3. **Find method**: Search for the method on the inferred type
4. **Generate method call**: If found, transform to method call IR
5. **Fall back**: If no method found, use primitive operator semantics

### Key Functions

Located in `pkg/semantic/analyzer.go`:

- **`operatorMethodName(op string) string`**: Maps operators to method names
- **`inferExpressionType(expr ast.Expr) *ir.Type`**: Determines expression type for lookup
- **`analyzeOperatorMethodCall(bin, funcSym, irFunc)`**: Generates the method call IR
- **`analyzeBinaryExpr()`**: Modified to check for operator methods first

### Generated Assembly

```asm
; v1 + v2 with operator overloading
; Operator + via test_operators.Vec2.add
; Call to test_operators.Vec2.add (args: 2)
; Stack-based parameter passing
    LD HL, ($F016)    ; Virtual register 11 from memory
    PUSH HL           ; Argument 1
    LD HL, ($F014)    ; Virtual register 10 from memory
    PUSH HL           ; Argument 0
    CALL test_operators.Vec2.add
```

## Design Decisions

### Why Method Dispatch over Operator Functions

We chose method-based dispatch (`v1.add(v2)`) over standalone functions (`add(v1, v2)`) because:

1. **Consistency with UFCS**: Leverages existing infrastructure
2. **Type-directed**: Method is always on the left operand's type
3. **Natural syntax**: `impl Type { fun add(...) }` feels natural
4. **No special syntax**: No need for `operator+` or similar

### Type Inference

Operator resolution uses lightweight type inference that only examines:
- Identifiers: Look up in scope
- Struct literals: Use the type name
- Member expressions: Look up struct and field type

This avoids recursive full analysis while providing enough information for method lookup.

### Primitive Fallback

If no operator method is found, the expression falls through to primitive operator handling. This means:
- `3 + 4` still works (primitive i16 addition)
- `v1 + v2` uses method (if Vec2 has `add`)
- No implicit conversion or coercion

## Testing

E2E test added: `TestE2EOperatorOverloading` in `pkg/semantic/e2e_test.go`

Test file: `/tmp/test_operators.minz` (see example above)

Verification:
```bash
./minzc/main /tmp/test_operators.minz -o /tmp/test_operators.a80
grep "CALL.*add\|CALL.*eq" /tmp/test_operators.a80
```

## Future Enhancements

- **Compound assignment**: `v1 += v2` -> `v1 = v1.add(v2)`
- **Unary operators**: `-v` -> `v.neg()`, `!v` -> `v.not()`
- **Index operators**: `v[i]` -> `v.index(i)`, `v[i] = x` -> `v.index_set(i, x)`
- **Call operator**: `f(x)` -> `f.call(x)` for callable structs

---

*Part of the Compile-Time Interfaces implementation (Phase 1)*
