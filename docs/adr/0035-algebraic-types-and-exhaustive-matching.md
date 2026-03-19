# ADR-0035: Algebraic Data Types and Exhaustive Pattern Matching in Nanz

**Date:** 2026-03-19
**Status:** Proposed
**Related:** ADR-0006 (pattern matching), Report 093 (const/enum improvements)

## Context

Nanz currently has simple enums (`enum Color { RED, GREEN, BLUE }`) that compile to
integer constants. Pattern matching exists as `switch/case` over integer values only.
There is no way to attach data to enum variants, and no compile-time verification
that all variants are handled.

This limits expressiveness in real programs. In the Mini Norton Commander (`nc.nanz`),
directory entries are distinguished by manual bit-testing:

```nanz
let attr: u8 = dirent[11]
let is_dir: u8 = (attr >> 4) & 1
if is_dir != 0 {
    // directory path
} else {
    // file path — but what about deleted? volume label? LFN?
}
```

Forgetting a case is a silent bug. Adding a new variant requires finding every
`if`-chain manually.

Meanwhile, the Elm language demonstrates that algebraic data types (tagged unions)
and exhaustive pattern matching provide massive safety benefits while compiling to
simple tag+payload representations — zero overhead on Z80.

We evaluated adding Elm as a full frontend. Conclusion: **the paradigm gap is too
large** (~15K LOC for a restricted subset), but the best ideas — tagged unions and
exhaustive matching — can be absorbed into Nanz directly for ~10% of the effort.

## Decision

### Phase 1: Tagged Unions (ADTs)

Extend `type` declarations to support variants with optional payloads:

```nanz
type Shape =
    Circle(radius: u8)
    Rect(w: u8, h: u8)
    Point
```

Syntax rules:
- Keyword `type` (already exists for aliases, extended for ADTs)
- Variants are listed on separate lines (or separated by `|`)
- Payload fields are named, like struct fields
- Variants without payload are allowed (like today's enum values)
- Maximum payload size: 16 bytes (fits in Z80 registers / small stack frame)

#### Z80 Representation

```
[tag: u8] [payload: N bytes, padded to max variant size]
```

Example for `Shape`:
- Total size = 1 (tag) + 2 (max payload = Rect's w,h) = 3 bytes
- `Circle(5)` → `[0x00][0x05][0x00]`
- `Rect(3, 7)` → `[0x01][0x03][0x07]`
- `Point` → `[0x02][0x00][0x00]`

This is exactly what C programmers write by hand. We generate it.

#### Construction Syntax

```nanz
let s: Shape = Circle(5)
let r: Shape = Rect(3, 7)
let p: Shape = Point
```

Variants act as constructor functions. `Circle(5)` produces a `Shape` value.

### Phase 2: Exhaustive Pattern Matching

Extend `switch` (or add `match`) to destructure tagged unions:

```nanz
match shape {
    Circle(r) -> {
        // r is bound to the radius (u8)
        draw_circle(r)
    }
    Rect(w, h) -> {
        draw_rect(w, h)
    }
    Point -> {
        draw_pixel()
    }
}
```

Rules:
- `match` (preferred) or `switch` keyword
- Each arm binds payload fields to local variables
- Compiler error if not all variants are covered
- Optional `_` wildcard for "everything else" (suppresses exhaustiveness check)
- Nested patterns NOT in scope (keep it simple for v1)

#### Exhaustiveness Check

For `match` on a typed enum/union value:

1. Collect all defined variants of the type
2. Collect all variants mentioned in case arms
3. If `_` present → OK
4. If missing variants → **compile error**:
   ```
   error: line 42: match on Shape is not exhaustive
     missing variants: Point
   ```

This also works for today's simple enums:

```nanz
enum Direction { NORTH, SOUTH, EAST, WEST }

match dir {
    NORTH -> go_up()
    SOUTH -> go_down()
    // ERROR: match on Direction is not exhaustive
    //   missing variants: EAST, WEST
}
```

### Phase 3 (Optional, future): Pattern Matching Extensions

- **Guard clauses**: `Circle(r) if r > 10 -> { ... }`
- **Literal patterns**: `match x { 0 -> ..., 1 -> ..., _ -> ... }`
- **Nested patterns**: `Just(Circle(r)) -> ...`

These are NOT proposed for v1. They can be added incrementally later.

## HIR Representation

### New HIR nodes

```go
// Tagged union type (extends existing enum support)
type UnionType struct {
    Name     string
    Variants []UnionVariant
}

type UnionVariant struct {
    Name   string
    Tag    int           // auto-assigned: 0, 1, 2, ...
    Fields []StructField // empty for payload-less variants
}

// Match expression/statement
type MatchStmt struct {
    Val   Expr
    Arms  []MatchArm
    Pos   Position
}

type MatchArm struct {
    Variant  string   // variant name
    Bindings []string // local names for payload fields
    Body     *Block
}
```

### MIR2 Lowering

`match` lowers to a chain of comparisons on the tag byte:

```
%tag = load %val, offset=0, u8
brif (%tag == 0) -> arm_circle, (%tag == 1) -> arm_rect, else -> arm_point

arm_circle:
  %r = load %val, offset=1, u8
  // body with r bound
  jmp exit

arm_rect:
  %w = load %val, offset=1, u8
  %h = load %val, offset=2, u8
  // body with w, h bound
  jmp exit
```

On Z80, this becomes:

```z80
    LD A, (IX+0)    ; tag
    CP 0
    JR Z, .circle
    CP 1
    JR Z, .rect
    ; fall through to point
.point:
    CALL draw_pixel
    JR .exit
.circle:
    LD A, (IX+1)    ; radius
    CALL draw_circle
    JR .exit
.rect:
    LD A, (IX+1)    ; width
    LD C, (IX+2)    ; height
    CALL draw_rect
.exit:
```

For small enums (2-4 variants), this is ~20 bytes. Same as hand-written code.

## Implementation Plan

| Phase | What | Estimated LOC | Dependencies |
|-------|------|---------------|--------------|
| **1a** | Parse `type X = Variant(fields)` | ~150 | None |
| **1b** | HIR `UnionType` + `UnionVariant` | ~50 | None |
| **1c** | Constructor codegen (`Circle(5)` → tag+payload) | ~100 | 1a, 1b |
| **1d** | MIR2 lowering for union type storage | ~100 | 1b |
| **2a** | Parse `match val { Variant(bindings) -> body }` | ~200 | 1a |
| **2b** | HIR `MatchStmt` + lowering to tag comparisons | ~150 | 2a, 1d |
| **2c** | Exhaustiveness checker | ~100 | 1b, 2a |
| **2d** | Exhaustiveness for existing simple enums | ~50 | 2c |
| **3** | Extensions (guards, literals, nested) | ~300 | 2b |
| | **Total (Phases 1+2)** | **~900** | |

Total estimated effort: **~900 LOC for Phase 1+2**. Phase 3 is optional.

## Consequences

### Positive

- **Compile-time safety**: forgotten match arms caught at compile time, not runtime
- **Self-documenting**: `type DirEntry = File(...) | Dir(...) | Deleted` reads better than bit flags
- **Zero overhead**: tag byte + payload is exactly what you'd write by hand
- **Incremental**: works alongside existing enums (simple enums become unions without payload)
- **Foundation**: enables future `Option(T)` / `Result(T, E)` patterns (without generics — monomorphic)

### Negative

- **Slightly larger values**: 1 extra byte for tag (vs. implicit type knowledge). On Z80, every byte matters. Mitigation: for 2-variant types, tag can be a single bit or a boolean.
- **Parser complexity**: ~200 LOC for new `type` syntax and `match` — modest.
- **No generics**: `Option(u8)` and `Option(u16)` are separate type declarations. This is intentional — generics are rejected per CLAUDE.md.

### Neutral

- Existing simple enums continue to work unchanged
- Existing `switch` on integers continues to work unchanged
- `match` and `switch` coexist — `match` for ADTs, `switch` for raw integers

## Alternatives Considered

### 1. Full Elm Frontend

~15K LOC. Requires indentation parser, H-M inference, immutability lowering,
currying elimination. 10x the effort for marginal benefit over absorbing the
best ideas into Nanz.

**Rejected**: paradigm gap too large for Z80 target.

### 2. Haskell-style typeclasses

Would enable `Option<T>` generics. But generics are explicitly rejected
(CLAUDE.md: "Use function overloading instead"). Monomorphic tagged unions
give 90% of the value.

**Rejected**: too complex, generics are out of scope.

### 3. C-style tagged unions (manual)

```nanz
struct Shape {
    tag: u8
    data: [u8; 4]
}
```

This is what programmers do today. No safety, no exhaustiveness checking, easy
to forget a case, painful to read.

**Rejected**: this is the status quo we're trying to improve.

### 4. Rust-style `enum` with `impl`

Full Rust enums with methods. More powerful but significantly more complex
(trait resolution, lifetime concerns if we ever add references).

**Rejected**: over-engineering for Z80. Our proposal is simpler and sufficient.

## Example: Norton Commander DirEntry (Before/After)

### Before (current Nanz)

```nanz
// Manual bit-flag checking, constants inlined
let attr: u8 = dirent[11]
if attr == 0 { return }                    // empty
if dirent[0] == 0xE5 { return }           // deleted
let is_dir: u8 = (attr >> 4) & 1
if is_dir != 0 {
    puts_pad(c"<DIR>", 8)
} else {
    let ext0: u8 = dirent[8]
    if ext0 == 84 {
        puts_pad(c"Text file", 13)
    } else {
        if ext0 == 66 {
            puts_pad(c"Binary file", 13)
        } else {
            puts_pad(c"File", 13)
        }
    }
}
```

### After (proposed)

```nanz
type DirEntry =
    File(name: [u8;11], size: u16, cluster: u16)
    Dir(name: [u8;11], cluster: u16)
    Deleted
    Empty

fun describe_entry(entry: DirEntry) -> void {
    match entry {
        File(name, size, cluster) -> {
            puts_pad(&name, 14)
            puts_right(format_size(size), 8)
        }
        Dir(name, cluster) -> {
            puts_pad(&name, 14)
            puts_pad(c"<DIR>", 8)
        }
        Deleted -> { }
        Empty -> { }
    }
}
```

Add a new variant (e.g., `VolumeLabel(name: [u8;11])`) → compiler tells you
every `match` that needs updating.

## References

- [Elm Language Guide — Custom Types](https://guide.elm-lang.org/types/custom_types.html)
- [Rust Enum Reference](https://doc.rust-lang.org/reference/items/enumerations.html)
- CLAUDE.md: pattern matching status (WIP), generics (rejected)
- ADR-0006: register allocator pattern (related codegen)
- Report 093: `const` support, enum improvements
